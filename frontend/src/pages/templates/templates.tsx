import { useEffect, useState } from 'react';
import { toast } from 'sonner';
import { FileCode, Loader2, Plus, Trash2 } from 'lucide-react';
import {
  useCreateTemplate,
  useDeleteTemplate,
  usePreviewTemplate,
  useTemplates,
  useUpdateTemplate,
} from '@/api/hooks';
import type { Template, TemplateType } from '@/api/types';
import { useDebounce } from '@/hooks/use-debounce';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Skeleton } from '@/components/ui/skeleton';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog';
import { EmptyState, ErrorState, PageHeader } from '@/pages/shared';

interface EditorState {
  id?: string;
  name: string;
  type: TemplateType;
  page_template: string;
}

const blankEditor: EditorState = {
  name: '',
  type: 'book',
  page_template: '# {{title}}\n\nby {{author}}\n\n{{#each highlights}}\n> {{text}}\n{{/each}}\n',
};

function TemplateEditor({
  editor,
  onChange,
  onSaved,
}: {
  editor: EditorState;
  onChange: (e: EditorState) => void;
  onSaved: (t: Template) => void;
}) {
  const create = useCreateTemplate();
  const update = useUpdateTemplate();
  const preview = usePreviewTemplate();
  const debouncedTemplate = useDebounce(editor.page_template, 500);

  useEffect(() => {
    if (!debouncedTemplate) return;
    preview.mutate({ page_template: debouncedTemplate, type: editor.type });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [debouncedTemplate, editor.type]);

  const saving = create.isPending || update.isPending;

  const save = () => {
    if (!editor.name.trim()) {
      toast.error('Give the template a name');
      return;
    }
    if (editor.id) {
      update.mutate(
        {
          id: editor.id,
          name: editor.name,
          type: editor.type,
          page_template: editor.page_template,
        },
        {
          onSuccess: (t) => {
            toast.success('Template saved');
            onSaved(t);
          },
          onError: () => toast.error('Could not save template'),
        },
      );
    } else {
      create.mutate(
        {
          name: editor.name,
          type: editor.type,
          page_template: editor.page_template,
        },
        {
          onSuccess: (t) => {
            toast.success('Template created');
            onSaved(t);
          },
          onError: () => toast.error('Could not create template'),
        },
      );
    }
  };

  return (
    <Card>
      <CardContent className="p-5">
        <div className="mb-4 flex flex-col gap-4 sm:flex-row sm:items-end">
          <div className="flex-1">
            <Label htmlFor="tpl-name" className="mb-2 block">
              Name
            </Label>
            <Input
              id="tpl-name"
              value={editor.name}
              onChange={(e) => onChange({ ...editor, name: e.target.value })}
              placeholder="My book template"
            />
          </div>
          <div className="w-40">
            <Label className="mb-2 block">Type</Label>
            <Select
              value={editor.type}
              onValueChange={(v) =>
                onChange({ ...editor, type: v as TemplateType })
              }
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="book">Book</SelectItem>
                <SelectItem value="article">Article</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <Button onClick={save} disabled={saving}>
            {saving && <Loader2 className="size-4 animate-spin" />}
            {editor.id ? 'Save' : 'Create'}
          </Button>
        </div>

        <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
          <div>
            <Label className="mb-2 block">Template</Label>
            <textarea
              value={editor.page_template}
              onChange={(e) =>
                onChange({ ...editor, page_template: e.target.value })
              }
              spellCheck={false}
              className="h-96 w-full resize-none rounded-md border border-input bg-background p-3 font-mono text-xs text-foreground outline-none focus-visible:ring-2 focus-visible:ring-ring"
            />
          </div>
          <div>
            <Label className="mb-2 block">Preview</Label>
            <div className="h-96 overflow-auto rounded-md border border-border bg-muted p-3">
              {preview.isPending ? (
                <div className="flex h-full items-center justify-center">
                  <Loader2 className="size-4 animate-spin text-muted-foreground" />
                </div>
              ) : preview.isError ? (
                <p className="text-xs text-destructive">
                  Preview unavailable.
                </p>
              ) : (
                <pre className="whitespace-pre-wrap text-xs text-foreground">
                  {preview.data?.content || 'Start typing to see a preview…'}
                </pre>
              )}
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}

export function TemplatesPage() {
  const { data, isLoading, isError, error } = useTemplates();
  const remove = useDeleteTemplate();
  const [editor, setEditor] = useState<EditorState | null>(null);

  const openNew = () => setEditor({ ...blankEditor });
  const openEdit = (t: Template) =>
    setEditor({
      id: t.id,
      name: t.name,
      type: t.type,
      page_template: t.page_template,
    });

  return (
    <div>
      <PageHeader
        title="Templates"
        description="Customize how documents export to markdown"
        actions={
          <Button onClick={openNew}>
            <Plus className="size-4" />
            New template
          </Button>
        }
      />

      {editor && (
        <div className="mb-6">
          <TemplateEditor
            editor={editor}
            onChange={setEditor}
            onSaved={(t) => openEdit(t)}
          />
          <Button
            variant="ghost"
            size="sm"
            className="mt-2"
            onClick={() => setEditor(null)}
          >
            Close editor
          </Button>
        </div>
      )}

      {isError ? (
        <ErrorState message={(error as Error)?.message} />
      ) : isLoading ? (
        <div className="flex flex-col gap-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-16 w-full rounded-lg" />
          ))}
        </div>
      ) : !data || data.length === 0 ? (
        <EmptyState
          icon={<FileCode className="size-8" />}
          title="No templates yet"
          description="Create a template to control your markdown exports."
          action={
            <Button onClick={openNew}>
              <Plus className="size-4" />
              New template
            </Button>
          }
        />
      ) : (
        <div className="flex flex-col gap-3">
          {data.map((t) => (
            <Card
              key={t.id}
              className={cn(
                'transition-colors',
                editor?.id === t.id && 'ring-2 ring-primary',
              )}
            >
              <CardContent className="flex items-center justify-between gap-3 p-4">
                <button
                  type="button"
                  className="flex min-w-0 flex-1 items-center gap-3 text-left"
                  onClick={() => openEdit(t)}
                >
                  <FileCode className="size-5 shrink-0 text-muted-foreground" />
                  <div className="min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="truncate font-medium text-foreground">
                        {t.name}
                      </span>
                      {t.is_default && (
                        <Badge variant="primary" appearance="light" size="sm">
                          Default
                        </Badge>
                      )}
                    </div>
                    <Badge
                      variant="secondary"
                      size="sm"
                      className="mt-1 capitalize"
                    >
                      {t.type}
                    </Badge>
                  </div>
                </button>
                <AlertDialog>
                  <AlertDialogTrigger asChild>
                    <Button
                      mode="icon"
                      variant="ghost"
                      size="sm"
                      aria-label="Delete template"
                    >
                      <Trash2 className="size-4" />
                    </Button>
                  </AlertDialogTrigger>
                  <AlertDialogContent>
                    <AlertDialogHeader>
                      <AlertDialogTitle>Delete template?</AlertDialogTitle>
                      <AlertDialogDescription>
                        &ldquo;{t.name}&rdquo; will be permanently removed.
                      </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                      <AlertDialogCancel>Cancel</AlertDialogCancel>
                      <AlertDialogAction
                        onClick={() =>
                          remove.mutate(t.id, {
                            onSuccess: () => {
                              toast.success('Template deleted');
                              if (editor?.id === t.id) setEditor(null);
                            },
                          })
                        }
                      >
                        Delete
                      </AlertDialogAction>
                    </AlertDialogFooter>
                  </AlertDialogContent>
                </AlertDialog>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}

import { useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { toast } from 'sonner';
import {
  ArrowLeft,
  Check,
  Download,
  ExternalLink,
  Loader2,
  Pencil,
  Star,
  Trash2,
  X,
} from 'lucide-react';
import {
  useDeleteDocument,
  useDeleteHighlight,
  useDocument,
  useExportDocument,
  useFavoriteHighlight,
  useUpdateDocument,
  useUpdateHighlight,
} from '@/api/hooks';
import type { Highlight } from '@/api/types';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Textarea } from '@/components/ui/textarea';
import { Skeleton } from '@/components/ui/skeleton';
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
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { ErrorState, PageHeader, TypeBadge } from '@/pages/shared';

function HighlightItem({
  highlight,
  documentId,
}: {
  highlight: Highlight;
  documentId: string;
}) {
  const [editing, setEditing] = useState(false);
  const [text, setText] = useState(highlight.text);
  const [note, setNote] = useState(highlight.note);

  const updateHighlight = useUpdateHighlight(documentId);
  const deleteHighlight = useDeleteHighlight(documentId);
  const favorite = useFavoriteHighlight(documentId);

  const save = () => {
    updateHighlight.mutate(
      { id: highlight.id, text, note },
      {
        onSuccess: () => {
          toast.success('Highlight updated');
          setEditing(false);
        },
        onError: () => toast.error('Could not update highlight'),
      },
    );
  };

  const accent = highlight.color || 'var(--color-zinc-300)';

  return (
    <Card>
      <CardContent className="flex gap-3 p-4">
        <div
          className="mt-1 w-1 shrink-0 self-stretch rounded-full"
          style={{ backgroundColor: accent }}
        />
        <div className="min-w-0 flex-1">
          {editing ? (
            <div className="flex flex-col gap-3">
              <Textarea
                value={text}
                onChange={(e) => setText(e.target.value)}
                placeholder="Highlight text"
                rows={3}
              />
              <Textarea
                value={note}
                onChange={(e) => setNote(e.target.value)}
                placeholder="Add a note…"
                rows={2}
              />
              <div className="flex gap-2">
                <Button size="sm" onClick={save} disabled={updateHighlight.isPending}>
                  {updateHighlight.isPending ? (
                    <Loader2 className="size-3.5 animate-spin" />
                  ) : (
                    <Check className="size-3.5" />
                  )}
                  Save
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => {
                    setText(highlight.text);
                    setNote(highlight.note);
                    setEditing(false);
                  }}
                >
                  <X className="size-3.5" />
                  Cancel
                </Button>
              </div>
            </div>
          ) : (
            <>
              <p className="text-sm leading-relaxed text-foreground">
                {highlight.text}
              </p>
              {highlight.note && (
                <p className="mt-2 rounded-md bg-muted px-3 py-2 text-sm text-muted-foreground">
                  {highlight.note}
                </p>
              )}
              <div className="mt-2 flex flex-wrap items-center gap-2">
                {highlight.chapter && (
                  <span className="text-xs text-muted-foreground">
                    {highlight.chapter}
                  </span>
                )}
                {highlight.location && (
                  <span className="text-xs text-muted-foreground">
                    · {highlight.location_type || 'loc'} {highlight.location}
                  </span>
                )}
                {highlight.tags?.map((t) => (
                  <Badge key={t} variant="secondary" size="sm">
                    {t}
                  </Badge>
                ))}
              </div>
            </>
          )}
        </div>
        {!editing && (
          <div className="flex shrink-0 flex-col gap-1">
            <Button
              size="sm"
              mode="icon"
              variant="ghost"
              aria-label="Favorite"
              onClick={() => favorite.mutate(highlight.id)}
            >
              <Star
                className={cn(
                  'size-4',
                  highlight.favorite && 'fill-yellow-400 text-yellow-400',
                )}
              />
            </Button>
            <Button
              size="sm"
              mode="icon"
              variant="ghost"
              aria-label="Edit"
              onClick={() => setEditing(true)}
            >
              <Pencil className="size-4" />
            </Button>
            <AlertDialog>
              <AlertDialogTrigger asChild>
                <Button size="sm" mode="icon" variant="ghost" aria-label="Delete">
                  <Trash2 className="size-4" />
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>Delete highlight?</AlertDialogTitle>
                  <AlertDialogDescription>
                    This permanently removes the highlight and its note.
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>Cancel</AlertDialogCancel>
                  <AlertDialogAction
                    onClick={() =>
                      deleteHighlight.mutate(highlight.id, {
                        onSuccess: () => toast.success('Highlight deleted'),
                      })
                    }
                  >
                    Delete
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

export function DocumentPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { data: doc, isLoading, isError, error } = useDocument(id);
  const updateDoc = useUpdateDocument(id ?? '');
  const deleteDoc = useDeleteDocument();
  const exportDoc = useExportDocument();
  const [exportOpen, setExportOpen] = useState(false);

  if (isLoading) {
    return (
      <div className="flex flex-col gap-4">
        <Skeleton className="h-40 w-full rounded-lg" />
        <Skeleton className="h-24 w-full rounded-lg" />
        <Skeleton className="h-24 w-full rounded-lg" />
      </div>
    );
  }

  if (isError || !doc) {
    return <ErrorState message={(error as Error)?.message} />;
  }

  const highlights = doc.highlights ?? [];

  const onExport = () => {
    setExportOpen(true);
    exportDoc.mutate(doc.id, {
      onError: () => toast.error('Export failed'),
    });
  };

  return (
    <div>
      <PageHeader title={doc.title || 'Document'} />

      <Button
        variant="ghost"
        size="sm"
        className="mb-4 -ml-2"
        onClick={() => navigate('/')}
      >
        <ArrowLeft className="size-4" />
        Back to library
      </Button>

      <Card className="mb-6">
        <CardContent className="flex flex-col gap-4 p-6 sm:flex-row">
          {doc.image_url && (
            <img
              src={doc.image_url}
              alt={doc.title}
              className="h-44 w-32 shrink-0 rounded-md object-cover"
            />
          )}
          <div className="min-w-0 flex-1">
            <div className="mb-2 flex flex-wrap items-center gap-2">
              <TypeBadge type={doc.type} />
              {doc.category && (
                <Badge variant="secondary" size="sm">
                  {doc.category}
                </Badge>
              )}
            </div>
            <h2 className="text-xl font-semibold text-foreground">{doc.title}</h2>
            {doc.author && (
              <p className="mt-1 text-sm text-muted-foreground">{doc.author}</p>
            )}
            {doc.tags?.length > 0 && (
              <div className="mt-3 flex flex-wrap gap-1.5">
                {doc.tags.map((t) => (
                  <Badge key={t} variant="primary" appearance="light" size="sm">
                    {t}
                  </Badge>
                ))}
              </div>
            )}
            <div className="mt-4 flex flex-wrap items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => updateDoc.mutate({ favorite: !doc.favorite })}
              >
                <Star
                  className={cn(
                    'size-4',
                    doc.favorite && 'fill-yellow-400 text-yellow-400',
                  )}
                />
                {doc.favorite ? 'Favorited' : 'Favorite'}
              </Button>
              <Button variant="outline" size="sm" onClick={onExport}>
                <Download className="size-4" />
                Export
              </Button>
              {doc.url && (
                <Button variant="outline" size="sm" asChild>
                  <a href={doc.url} target="_blank" rel="noreferrer">
                    <ExternalLink className="size-4" />
                    Source
                  </a>
                </Button>
              )}
              <AlertDialog>
                <AlertDialogTrigger asChild>
                  <Button variant="outline" size="sm">
                    <Trash2 className="size-4" />
                    Delete
                  </Button>
                </AlertDialogTrigger>
                <AlertDialogContent>
                  <AlertDialogHeader>
                    <AlertDialogTitle>Delete document?</AlertDialogTitle>
                    <AlertDialogDescription>
                      This permanently removes &ldquo;{doc.title}&rdquo; and all of
                      its highlights.
                    </AlertDialogDescription>
                  </AlertDialogHeader>
                  <AlertDialogFooter>
                    <AlertDialogCancel>Cancel</AlertDialogCancel>
                    <AlertDialogAction
                      onClick={() =>
                        deleteDoc.mutate(doc.id, {
                          onSuccess: () => {
                            toast.success('Document deleted');
                            navigate('/');
                          },
                        })
                      }
                    >
                      Delete
                    </AlertDialogAction>
                  </AlertDialogFooter>
                </AlertDialogContent>
              </AlertDialog>
            </div>
          </div>
        </CardContent>
      </Card>

      <h2 className="mb-3 text-sm font-semibold text-foreground">
        Highlights ({highlights.length})
      </h2>
      {highlights.length === 0 ? (
        <Card>
          <CardContent className="p-8 text-center text-sm text-muted-foreground">
            No highlights for this document yet.
          </CardContent>
        </Card>
      ) : (
        <div className="flex flex-col gap-3">
          {highlights.map((h) => (
            <HighlightItem key={h.id} highlight={h} documentId={doc.id} />
          ))}
        </div>
      )}

      <Dialog open={exportOpen} onOpenChange={setExportOpen}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>Export · {doc.title}</DialogTitle>
            <DialogDescription>Rendered markdown export</DialogDescription>
          </DialogHeader>
          <DialogBody>
            {exportDoc.isPending ? (
              <div className="flex items-center justify-center py-10">
                <Loader2 className="size-5 animate-spin text-muted-foreground" />
              </div>
            ) : exportDoc.isError ? (
              <p className="text-sm text-destructive">
                Could not generate export.
              </p>
            ) : (
              <pre className="max-h-[60vh] overflow-auto whitespace-pre-wrap rounded-md bg-muted p-4 text-xs text-foreground">
                {exportDoc.data?.content}
              </pre>
            )}
          </DialogBody>
        </DialogContent>
      </Dialog>
    </div>
  );
}

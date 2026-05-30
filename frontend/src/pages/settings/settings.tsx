import { useEffect, useState } from 'react';
import { toast } from 'sonner';
import {
  Check,
  Copy,
  KeyRound,
  Loader2,
  Plus,
  RefreshCw,
  Trash2,
} from 'lucide-react';
import { useAuth } from '@/auth/auth-context';
import {
  useCreateToken,
  useReadeckIntegration,
  useRevokeToken,
  useSync,
  useSyncStatus,
  useTokens,
  useUpdateReadeckIntegration,
} from '@/api/hooks';
import type { ApiTokenCreated } from '@/api/types';
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Skeleton } from '@/components/ui/skeleton';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog';
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
import { EmptyState, formatDateTime, PageHeader } from '@/pages/shared';

function ProfileTab() {
  const { user } = useAuth();
  return (
    <Card>
      <CardContent className="flex max-w-md flex-col gap-4 p-6">
        <div className="flex flex-col gap-2">
          <Label htmlFor="profile-name">Name</Label>
          <Input id="profile-name" value={user?.name ?? ''} readOnly />
        </div>
        <div className="flex flex-col gap-2">
          <Label htmlFor="profile-email">Email</Label>
          <Input id="profile-email" value={user?.email ?? ''} readOnly />
        </div>
        {user?.is_admin && (
          <Badge variant="primary" appearance="light" size="sm" className="w-fit">
            Administrator
          </Badge>
        )}
      </CardContent>
    </Card>
  );
}

function CreateTokenDialog() {
  const create = useCreateToken();
  const [name, setName] = useState('');
  const [open, setOpen] = useState(false);
  const [created, setCreated] = useState<ApiTokenCreated | null>(null);
  const { isCopied, copyToClipboard } = useCopyToClipboard();

  const reset = () => {
    setName('');
    setCreated(null);
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        setOpen(o);
        if (!o) reset();
      }}
    >
      <DialogTrigger asChild>
        <Button>
          <Plus className="size-4" />
          Create token
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Create API token</DialogTitle>
          <DialogDescription>
            {created
              ? 'Copy this token now — it will not be shown again.'
              : 'Name your token to identify where it is used.'}
          </DialogDescription>
        </DialogHeader>
        <DialogBody>
          {created ? (
            <div className="flex items-center gap-2">
              <code className="flex-1 truncate rounded-md bg-muted px-3 py-2 font-mono text-xs">
                {created.token}
              </code>
              <Button
                variant="outline"
                mode="icon"
                onClick={() => copyToClipboard(created.token)}
                aria-label="Copy token"
              >
                {isCopied ? (
                  <Check className="size-4 text-green-600" />
                ) : (
                  <Copy className="size-4" />
                )}
              </Button>
            </div>
          ) : (
            <div className="flex flex-col gap-2">
              <Label htmlFor="token-name">Token name</Label>
              <Input
                id="token-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="My laptop"
              />
            </div>
          )}
        </DialogBody>
        <DialogFooter>
          {created ? (
            <Button onClick={() => setOpen(false)}>Done</Button>
          ) : (
            <Button
              disabled={!name.trim() || create.isPending}
              onClick={() =>
                create.mutate(
                  { name: name.trim() },
                  {
                    onSuccess: (t) => setCreated(t),
                    onError: () => toast.error('Could not create token'),
                  },
                )
              }
            >
              {create.isPending && <Loader2 className="size-4 animate-spin" />}
              Create
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function TokensTab() {
  const { data, isLoading } = useTokens();
  const revoke = useRevokeToken();

  return (
    <Card>
      <CardContent className="p-5">
        <div className="mb-4 flex items-center justify-between">
          <div>
            <h3 className="text-sm font-semibold text-foreground">API tokens</h3>
            <p className="text-xs text-muted-foreground">
              Use tokens for programmatic access via Bearer auth.
            </p>
          </div>
          <CreateTokenDialog />
        </div>

        {isLoading ? (
          <Skeleton className="h-32 w-full" />
        ) : !data || data.length === 0 ? (
          <EmptyState
            icon={<KeyRound className="size-7" />}
            title="No tokens yet"
            description="Create a token to access the API programmatically."
          />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Prefix</TableHead>
                <TableHead>Last used</TableHead>
                <TableHead>Created</TableHead>
                <TableHead />
              </TableRow>
            </TableHeader>
            <TableBody>
              {data.map((t) => (
                <TableRow key={t.id}>
                  <TableCell className="font-medium">{t.name}</TableCell>
                  <TableCell>
                    <code className="font-mono text-xs">{t.prefix}…</code>
                  </TableCell>
                  <TableCell>{formatDateTime(t.last_used_at)}</TableCell>
                  <TableCell>{formatDateTime(t.created_at)}</TableCell>
                  <TableCell className="text-right">
                    <AlertDialog>
                      <AlertDialogTrigger asChild>
                        <Button
                          mode="icon"
                          variant="ghost"
                          size="sm"
                          aria-label="Revoke token"
                        >
                          <Trash2 className="size-4" />
                        </Button>
                      </AlertDialogTrigger>
                      <AlertDialogContent>
                        <AlertDialogHeader>
                          <AlertDialogTitle>Revoke token?</AlertDialogTitle>
                          <AlertDialogDescription>
                            Apps using &ldquo;{t.name}&rdquo; will immediately
                            lose access.
                          </AlertDialogDescription>
                        </AlertDialogHeader>
                        <AlertDialogFooter>
                          <AlertDialogCancel>Cancel</AlertDialogCancel>
                          <AlertDialogAction
                            onClick={() =>
                              revoke.mutate(t.id, {
                                onSuccess: () => toast.success('Token revoked'),
                              })
                            }
                          >
                            Revoke
                          </AlertDialogAction>
                        </AlertDialogFooter>
                      </AlertDialogContent>
                    </AlertDialog>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}

function IntegrationsTab() {
  const { data, isLoading } = useReadeckIntegration();
  const update = useUpdateReadeckIntegration();
  const sync = useSync();
  const [url, setUrl] = useState('');
  const [token, setToken] = useState('');

  useEffect(() => {
    if (data) {
      setUrl(data.url ?? '');
      setToken(data.token ?? '');
    }
  }, [data]);

  return (
    <Card>
      <CardContent className="flex max-w-md flex-col gap-4 p-6">
        <div className="flex items-center justify-between">
          <h3 className="text-sm font-semibold text-foreground">Readeck</h3>
          {data?.configured && (
            <Badge variant="success" appearance="light" size="sm">
              Configured
            </Badge>
          )}
        </div>

        {isLoading ? (
          <Skeleton className="h-24 w-full" />
        ) : (
          <>
            <div className="flex flex-col gap-2">
              <Label htmlFor="readeck-url">Server URL</Label>
              <Input
                id="readeck-url"
                value={url}
                onChange={(e) => setUrl(e.target.value)}
                placeholder="https://readeck.example.com"
              />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="readeck-token">API token</Label>
              <Input
                id="readeck-token"
                type="password"
                value={token}
                onChange={(e) => setToken(e.target.value)}
                placeholder="••••••••"
              />
            </div>
            <div className="flex gap-2">
              <Button
                disabled={update.isPending}
                onClick={() =>
                  update.mutate(
                    { url, token },
                    {
                      onSuccess: () => toast.success('Integration saved'),
                      onError: () => toast.error('Could not save integration'),
                    },
                  )
                }
              >
                {update.isPending && <Loader2 className="size-4 animate-spin" />}
                Save
              </Button>
              <Button
                variant="outline"
                disabled={sync.isPending}
                onClick={() =>
                  sync.mutate(undefined, {
                    onSuccess: () => toast.success('Sync complete'),
                    onError: () => toast.error('Sync failed'),
                  })
                }
              >
                {sync.isPending ? (
                  <Loader2 className="size-4 animate-spin" />
                ) : (
                  <RefreshCw className="size-4" />
                )}
                Sync now
              </Button>
            </div>
          </>
        )}
      </CardContent>
    </Card>
  );
}

const syncStatusVariant: Record<
  string,
  'success' | 'destructive' | 'warning'
> = {
  completed: 'success',
  failed: 'destructive',
  started: 'warning',
};

function SyncHistoryTab() {
  const { data, isLoading } = useSyncStatus();

  return (
    <Card>
      <CardContent className="p-5">
        <h3 className="mb-4 text-sm font-semibold text-foreground">
          Sync history
        </h3>
        {isLoading ? (
          <Skeleton className="h-32 w-full" />
        ) : !data || data.length === 0 ? (
          <EmptyState
            icon={<RefreshCw className="size-7" />}
            title="No syncs yet"
            description="Run a sync to populate your library."
          />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Status</TableHead>
                <TableHead>Documents</TableHead>
                <TableHead>Highlights</TableHead>
                <TableHead>Started</TableHead>
                <TableHead>Completed</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {data.map((log) => (
                <TableRow key={log.id}>
                  <TableCell>
                    <Badge
                      variant={syncStatusVariant[log.status] ?? 'secondary'}
                      appearance="light"
                      size="sm"
                      className="capitalize"
                    >
                      {log.status}
                    </Badge>
                    {log.error && (
                      <p className="mt-1 text-xs text-destructive">{log.error}</p>
                    )}
                  </TableCell>
                  <TableCell>{log.documents_synced}</TableCell>
                  <TableCell>{log.highlights_synced}</TableCell>
                  <TableCell>{formatDateTime(log.started_at)}</TableCell>
                  <TableCell>{formatDateTime(log.completed_at)}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}

export function SettingsPage() {
  return (
    <div>
      <PageHeader
        title="Settings"
        description="Manage your account, tokens, and integrations"
      />
      <Tabs defaultValue="profile">
        <TabsList className="mb-6">
          <TabsTrigger value="profile">Profile</TabsTrigger>
          <TabsTrigger value="tokens">API Tokens</TabsTrigger>
          <TabsTrigger value="integrations">Integrations</TabsTrigger>
          <TabsTrigger value="history">Sync History</TabsTrigger>
        </TabsList>
        <TabsContent value="profile">
          <ProfileTab />
        </TabsContent>
        <TabsContent value="tokens">
          <TokensTab />
        </TabsContent>
        <TabsContent value="integrations">
          <IntegrationsTab />
        </TabsContent>
        <TabsContent value="history">
          <SyncHistoryTab />
        </TabsContent>
      </Tabs>
    </div>
  );
}

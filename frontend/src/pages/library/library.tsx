import { useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { toast } from 'sonner';
import {
  BookOpen,
  FileText,
  Highlighter,
  Loader2,
  RefreshCw,
  Search,
  Star,
} from 'lucide-react';
import {
  useDocuments,
  useFavoriteHighlight,
  useHighlights,
  useStats,
  useSync,
  useUpdateDocument,
} from '@/api/hooks';
import type { Document } from '@/api/types';
import { useDebounce } from '@/hooks/use-debounce';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Skeleton } from '@/components/ui/skeleton';
import { EmptyState, ErrorState, PageHeader, TypeBadge } from '@/pages/shared';

type TabValue = 'all' | 'books' | 'articles';

function StatCard({
  label,
  value,
  icon,
  loading,
}: {
  label: string;
  value?: number;
  icon: React.ReactNode;
  loading: boolean;
}) {
  return (
    <Card>
      <CardContent className="flex items-center gap-4 p-5">
        <div className="flex size-10 items-center justify-center rounded-md bg-accent text-accent-foreground">
          {icon}
        </div>
        <div>
          <div className="text-2xl font-semibold text-foreground">
            {loading ? <Skeleton className="h-7 w-12" /> : (value ?? 0)}
          </div>
          <div className="text-xs font-medium text-muted-foreground">{label}</div>
        </div>
      </CardContent>
    </Card>
  );
}

function DocumentCard({ doc }: { doc: Document }) {
  const updateDoc = useUpdateDocument(doc.id);
  const highlightCount = doc.highlights?.length ?? 0;

  return (
    <Card className="group relative overflow-hidden transition-shadow hover:shadow-md">
      <Link to={`/documents/${doc.id}`} className="block">
        <div className="flex aspect-video items-center justify-center overflow-hidden bg-muted">
          {doc.image_url ? (
            <img
              src={doc.image_url}
              alt={doc.title}
              className="h-full w-full object-cover"
              loading="lazy"
            />
          ) : (
            <FileText className="size-10 text-muted-foreground/40" />
          )}
        </div>
        <CardContent className="flex flex-col gap-2 p-4">
          <div className="flex items-center justify-between gap-2">
            <TypeBadge type={doc.type} />
            <span className="text-xs text-muted-foreground">
              {highlightCount} highlight{highlightCount === 1 ? '' : 's'}
            </span>
          </div>
          <h3 className="line-clamp-2 text-sm font-semibold leading-snug text-foreground">
            {doc.title || 'Untitled'}
          </h3>
          {doc.author && (
            <p className="line-clamp-1 text-xs text-muted-foreground">
              {doc.author}
            </p>
          )}
        </CardContent>
      </Link>
      <button
        type="button"
        aria-label={doc.favorite ? 'Unfavorite' : 'Favorite'}
        onClick={(e) => {
          e.preventDefault();
          updateDoc.mutate({ favorite: !doc.favorite });
        }}
        className="absolute right-2 top-2 flex size-8 items-center justify-center rounded-full bg-background/80 text-muted-foreground backdrop-blur transition-colors hover:text-foreground"
      >
        <Star
          className={cn(
            'size-4',
            doc.favorite && 'fill-yellow-400 text-yellow-400',
          )}
        />
      </button>
    </Card>
  );
}

function HighlightResults({ query }: { query: string }) {
  const { data, isLoading } = useHighlights(
    { q: query },
    { enabled: query.length > 0 },
  );
  const favorite = useFavoriteHighlight();

  if (!query) return null;
  if (isLoading) {
    return <Skeleton className="h-24 w-full" />;
  }
  if (!data || data.length === 0) return null;

  return (
    <div className="mb-8">
      <h2 className="mb-3 text-sm font-semibold text-foreground">
        Matching highlights ({data.length})
      </h2>
      <div className="flex flex-col gap-3">
        {data.map((h) => (
          <Card key={h.id}>
            <CardContent className="flex items-start justify-between gap-3 p-4">
              <Link
                to={`/documents/${h.document_id}`}
                className="min-w-0 flex-1"
              >
                <p className="text-sm text-foreground">{h.text}</p>
                {h.note && (
                  <p className="mt-1 text-xs text-muted-foreground">{h.note}</p>
                )}
                {h.chapter && (
                  <p className="mt-1 text-xs text-muted-foreground">
                    {h.chapter}
                  </p>
                )}
              </Link>
              <button
                type="button"
                aria-label="Favorite highlight"
                onClick={() => favorite.mutate(h.id)}
                className="shrink-0 text-muted-foreground hover:text-foreground"
              >
                <Star
                  className={cn(
                    'size-4',
                    h.favorite && 'fill-yellow-400 text-yellow-400',
                  )}
                />
              </button>
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  );
}

export function LibraryPage() {
  const [tab, setTab] = useState<TabValue>('all');
  const [rawQuery, setRawQuery] = useState('');
  const query = useDebounce(rawQuery, 300);

  const typeParam =
    tab === 'books' ? 'book' : tab === 'articles' ? 'article' : undefined;

  const stats = useStats();
  const documents = useDocuments({
    type: typeParam,
    q: query || undefined,
  });
  const sync = useSync();

  const docs = useMemo(() => documents.data ?? [], [documents.data]);

  const onSync = () => {
    sync.mutate(undefined, {
      onSuccess: (res) => {
        const r = res?.readeck;
        toast.success(
          r
            ? `Synced ${r.documents} documents and ${r.highlights} highlights`
            : 'Sync complete',
        );
      },
      onError: () => toast.error('Sync failed. Check your integration settings.'),
    });
  };

  return (
    <div>
      <PageHeader
        title="Library"
        description="Your books, articles, and highlights"
        actions={
          <Button onClick={onSync} disabled={sync.isPending}>
            {sync.isPending ? (
              <Loader2 className="size-4 animate-spin" />
            ) : (
              <RefreshCw className="size-4" />
            )}
            Sync
          </Button>
        }
      />

      <div className="mb-6 grid grid-cols-2 gap-4 lg:grid-cols-4">
        <StatCard
          label="Books"
          value={stats.data?.books}
          loading={stats.isLoading}
          icon={<BookOpen className="size-5" />}
        />
        <StatCard
          label="Articles"
          value={stats.data?.articles}
          loading={stats.isLoading}
          icon={<FileText className="size-5" />}
        />
        <StatCard
          label="Highlights"
          value={stats.data?.highlights}
          loading={stats.isLoading}
          icon={<Highlighter className="size-5" />}
        />
        <StatCard
          label="Due reviews"
          value={stats.data?.due_reviews}
          loading={stats.isLoading}
          icon={<Star className="size-5" />}
        />
      </div>

      <div className="mb-6 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <Tabs value={tab} onValueChange={(v) => setTab(v as TabValue)}>
          <TabsList>
            <TabsTrigger value="all">All</TabsTrigger>
            <TabsTrigger value="books">Books</TabsTrigger>
            <TabsTrigger value="articles">Articles</TabsTrigger>
          </TabsList>
        </Tabs>
        <div className="relative w-full sm:w-72">
          <Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={rawQuery}
            onChange={(e) => setRawQuery(e.target.value)}
            placeholder="Search title, author, or highlights…"
            className="pl-9"
          />
        </div>
      </div>

      <HighlightResults query={query} />

      {documents.isError ? (
        <ErrorState message={(documents.error as Error)?.message} />
      ) : documents.isLoading ? (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
          {Array.from({ length: 8 }).map((_, i) => (
            <Skeleton key={i} className="h-64 w-full rounded-lg" />
          ))}
        </div>
      ) : docs.length === 0 ? (
        <EmptyState
          icon={<BookOpen className="size-8" />}
          title={query ? 'No documents match your search' : 'No documents yet'}
          description={
            query
              ? 'Try a different search term.'
              : 'Sync your sources to import your reading and highlights.'
          }
          action={
            !query && (
              <Button onClick={onSync} disabled={sync.isPending}>
                <RefreshCw className="size-4" />
                Sync now
              </Button>
            )
          }
        />
      ) : (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
          {docs.map((doc) => (
            <DocumentCard key={doc.id} doc={doc} />
          ))}
        </div>
      )}
    </div>
  );
}

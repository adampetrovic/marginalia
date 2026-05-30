import { useState } from 'react';
import { Link } from 'react-router-dom';
import { toast } from 'sonner';
import { CheckCircle2, Loader2, PartyPopper } from 'lucide-react';
import { useRateReview, useReview } from '@/api/hooks';
import type { ReviewRating, ReviewResponse } from '@/api/types';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { ErrorState, PageHeader, TypeBadge } from '@/pages/shared';

const ratings: { value: ReviewRating; label: string; className: string }[] = [
  {
    value: 'again',
    label: 'Again',
    className: 'border-red-300 text-red-700 hover:bg-red-50 dark:text-red-400',
  },
  {
    value: 'hard',
    label: 'Hard',
    className:
      'border-orange-300 text-orange-700 hover:bg-orange-50 dark:text-orange-400',
  },
  {
    value: 'good',
    label: 'Good',
    className:
      'border-green-300 text-green-700 hover:bg-green-50 dark:text-green-400',
  },
  {
    value: 'easy',
    label: 'Easy',
    className:
      'border-blue-300 text-blue-700 hover:bg-blue-50 dark:text-blue-400',
  },
];

function isDone(res: ReviewResponse | undefined): boolean {
  return !!res && 'done' in res && res.done === true;
}

export function ReviewPage() {
  const { data, isLoading, isError, error } = useReview();
  const rate = useRateReview();
  const [revealed, setRevealed] = useState(false);

  if (isLoading) {
    return (
      <div>
        <PageHeader title="Daily Review" />
        <Skeleton className="mx-auto h-72 w-full max-w-2xl rounded-lg" />
      </div>
    );
  }

  if (isError) {
    return (
      <div>
        <PageHeader title="Daily Review" />
        <ErrorState message={(error as Error)?.message} />
      </div>
    );
  }

  if (isDone(data) || !data || !('highlight' in data)) {
    return (
      <div>
        <PageHeader title="Daily Review" />
        <Card className="mx-auto max-w-2xl">
          <CardContent className="flex flex-col items-center justify-center gap-3 py-16 text-center">
            <PartyPopper className="size-10 text-primary" />
            <h2 className="text-lg font-semibold text-foreground">All done!</h2>
            <p className="max-w-sm text-sm text-muted-foreground">
              You have reviewed everything due today. Come back tomorrow for more.
            </p>
            <Button asChild variant="outline" className="mt-2">
              <Link to="/">Back to library</Link>
            </Button>
          </CardContent>
        </Card>
      </div>
    );
  }

  const { highlight, document, stats } = data;

  const onRate = (rating: ReviewRating) => {
    rate.mutate(
      { id: highlight.id, rating },
      {
        onSuccess: () => setRevealed(false),
        onError: () => toast.error('Could not save your review'),
      },
    );
  };

  return (
    <div>
      <PageHeader
        title="Daily Review"
        description="Strengthen your memory of past highlights"
      />

      <div className="mx-auto flex max-w-2xl items-center justify-center gap-6 pb-4">
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <span className="font-semibold text-foreground">{stats.due}</span> due
        </div>
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <CheckCircle2 className="size-4 text-green-600" />
          <span className="font-semibold text-foreground">
            {stats.reviewed_today}
          </span>{' '}
          reviewed today
        </div>
      </div>

      <Card className="mx-auto max-w-2xl">
        <CardContent className="flex flex-col gap-6 p-8">
          <div className="flex items-center justify-between gap-2">
            <Link
              to={`/documents/${document.id}`}
              className="min-w-0 truncate text-sm font-medium text-muted-foreground hover:text-foreground"
            >
              {document.title}
              {document.author && ` · ${document.author}`}
            </Link>
            <TypeBadge type={document.type} />
          </div>

          <button
            type="button"
            onClick={() => setRevealed((r) => !r)}
            className="rounded-lg bg-muted/50 p-6 text-left transition-colors hover:bg-muted"
          >
            <p className="text-lg leading-relaxed text-foreground">
              {highlight.text}
            </p>
            {highlight.chapter && (
              <p className="mt-3 text-xs text-muted-foreground">
                {highlight.chapter}
              </p>
            )}
          </button>

          {revealed ? (
            highlight.note ? (
              <div className="rounded-md border border-border bg-background p-4">
                <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                  Note
                </p>
                <p className="mt-1 text-sm text-foreground">{highlight.note}</p>
              </div>
            ) : (
              <p className="text-center text-sm text-muted-foreground">
                No note on this highlight.
              </p>
            )
          ) : (
            <p className="text-center text-sm text-muted-foreground">
              Click the highlight to reveal its note.
            </p>
          )}

          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
            {ratings.map((r) => (
              <Button
                key={r.value}
                variant="outline"
                disabled={rate.isPending}
                onClick={() => onRate(r.value)}
                className={r.className}
              >
                {rate.isPending ? (
                  <Loader2 className="size-4 animate-spin" />
                ) : (
                  r.label
                )}
              </Button>
            ))}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

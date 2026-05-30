import type { ReactNode } from 'react';
import { Helmet } from 'react-helmet-async';
import type { DocumentType } from '@/api/types';
import { Badge } from '@/components/ui/badge';

export function PageHeader({
  title,
  description,
  actions,
}: {
  title: string;
  description?: string;
  actions?: ReactNode;
}) {
  return (
    <>
      <Helmet>
        <title>{title} · Marginalia</title>
      </Helmet>
      <div className="mb-6 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-xl font-semibold tracking-tight text-foreground">
            {title}
          </h1>
          {description && (
            <p className="mt-1 text-sm text-muted-foreground">{description}</p>
          )}
        </div>
        {actions && <div className="flex items-center gap-2">{actions}</div>}
      </div>
    </>
  );
}

const typeVariant: Record<
  string,
  'primary' | 'secondary' | 'success' | 'warning' | 'info' | 'destructive'
> = {
  book: 'info',
  article: 'success',
  podcast: 'warning',
  tweet: 'primary',
};

export function TypeBadge({ type }: { type: DocumentType | string }) {
  return (
    <Badge
      variant={typeVariant[type] ?? 'secondary'}
      appearance="light"
      size="sm"
      className="capitalize"
    >
      {type}
    </Badge>
  );
}

export function formatDate(value: string | null | undefined): string {
  if (!value) return '—';
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return '—';
  return d.toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  });
}

export function formatDateTime(value: string | null | undefined): string {
  if (!value) return '—';
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return '—';
  return d.toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

export function ErrorState({ message }: { message?: string }) {
  return (
    <div className="flex flex-col items-center justify-center rounded-lg border border-dashed border-border py-16 text-center">
      <p className="text-sm font-medium text-foreground">Something went wrong</p>
      <p className="mt-1 max-w-sm text-sm text-muted-foreground">
        {message || 'We could not load this data. The backend may be unavailable.'}
      </p>
    </div>
  );
}

export function EmptyState({
  title,
  description,
  icon,
  action,
}: {
  title: string;
  description?: string;
  icon?: ReactNode;
  action?: ReactNode;
}) {
  return (
    <div className="flex flex-col items-center justify-center rounded-lg border border-dashed border-border py-16 text-center">
      {icon && <div className="mb-3 text-muted-foreground">{icon}</div>}
      <p className="text-sm font-medium text-foreground">{title}</p>
      {description && (
        <p className="mt-1 max-w-sm text-sm text-muted-foreground">{description}</p>
      )}
      {action && <div className="mt-4">{action}</div>}
    </div>
  );
}

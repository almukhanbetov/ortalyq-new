'use client';

import type { ReactNode } from 'react';
import { AlertCircle, Inbox, RotateCw } from 'lucide-react';
import { cn } from './cn';
import { useT } from '@/lib/i18n';
import { errorMessage } from '@/lib/errors';

export function Spinner({ size = 20, className }: { size?: number; className?: string }) {
  return (
    <svg className={cn('animate-spin', className)} width={size} height={size} viewBox="0 0 24 24" fill="none" aria-hidden>
      <circle cx="12" cy="12" r="9" stroke="currentColor" strokeOpacity=".2" strokeWidth="3" />
      <path d="M21 12a9 9 0 0 0-9-9" stroke="currentColor" strokeWidth="3" strokeLinecap="round" />
    </svg>
  );
}

export function Loading({ label, className }: { label?: string; className?: string }) {
  const t = useT();
  return (
    <div role="status" className={cn('flex items-center justify-center gap-3 py-12 text-ink-muted', className)}>
      <Spinner className="text-primary" />
      <span>{label ?? t('common.loading')}</span>
    </div>
  );
}

export function ErrorState({ error, onRetry, className }: { error: unknown; onRetry?: () => void; className?: string }) {
  const t = useT();
  return (
    <div role="alert" className={cn('flex flex-col items-center gap-3 py-12 text-center', className)}>
      <span className="flex h-12 w-12 items-center justify-center rounded-full bg-danger-soft text-danger-text">
        <AlertCircle size={24} />
      </span>
      <div>
        <p className="text-md font-medium text-ink">{t('common.errorTitle')}</p>
        <p className="mt-1 text-sm text-ink-muted">{errorMessage(error, t)}</p>
      </div>
      {onRetry && (
        <button type="button" onClick={onRetry} className="inline-flex h-9 items-center gap-2 rounded-md border border-line-strong px-4 text-base font-medium text-ink hover:bg-surface-hover">
          <RotateCw size={16} /> {t('common.retry')}
        </button>
      )}
    </div>
  );
}

export function EmptyState({ title, text, action, icon, className }: { title: ReactNode; text?: ReactNode; action?: ReactNode; icon?: ReactNode; className?: string }) {
  return (
    <div className={cn('flex flex-col items-center gap-3 px-4 py-12 text-center', className)}>
      <span className="flex h-14 w-14 items-center justify-center rounded-full bg-primary-soft text-primary-text">{icon ?? <Inbox size={26} />}</span>
      <p className="max-w-md text-md font-medium text-ink">{title}</p>
      {text && <p className="max-w-md text-sm text-ink-muted">{text}</p>}
      {action}
    </div>
  );
}

/** Renders loading / error+retry / content for a react-query result. */
export function QueryState<T>({ query, children, loading }: { query: { data: T | undefined; isLoading: boolean; error: unknown; refetch: () => unknown }; children: (data: T) => ReactNode; loading?: ReactNode }) {
  if (query.isLoading) return <>{loading ?? <Loading />}</>;
  if (query.error || query.data === undefined) return <ErrorState error={query.error} onRetry={() => query.refetch()} />;
  return <>{children(query.data)}</>;
}

export function ProgressBar({ value, max, label, tone = 'primary' }: { value: number; max: number; label?: string; tone?: 'primary' | 'success' }) {
  const pct = max > 0 ? Math.round((value / max) * 100) : 0;
  return (
    <div className="h-2 w-full overflow-hidden rounded-full bg-surface-hover" role="progressbar" aria-valuemin={0} aria-valuemax={max} aria-valuenow={value} aria-label={label}>
      <div className={cn('h-full rounded-full transition-all', tone === 'success' ? 'bg-success' : 'bg-primary')} style={{ width: `${pct}%` }} />
    </div>
  );
}

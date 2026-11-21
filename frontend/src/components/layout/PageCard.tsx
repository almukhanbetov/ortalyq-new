import type { ReactNode } from 'react';
import { cn } from '@/components/ui';

/** White content card used by cabinet list pages (Edvibe-like). */
export function PageCard({ title, actions, children, className, maxWidth = 1120 }: { title?: ReactNode; actions?: ReactNode; children: ReactNode; className?: string; maxWidth?: number }) {
  return (
    <section className={cn('mx-auto w-full rounded-2xl bg-surface p-5 shadow-card sm:p-8', className)} style={{ maxWidth }}>
      {(title || actions) && (
        <div className="mb-6 flex flex-wrap items-center justify-between gap-3">
          {title && <h1 className="text-2xl font-bold text-ink">{title}</h1>}
          {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
        </div>
      )}
      {children}
    </section>
  );
}

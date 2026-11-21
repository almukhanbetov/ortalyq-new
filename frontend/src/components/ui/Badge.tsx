import type { ReactNode } from 'react';
import { cn } from './cn';

export type Tone = 'neutral' | 'primary' | 'success' | 'danger' | 'warning' | 'pink' | 'info';

const tones: Record<Tone, string> = {
  neutral: 'bg-surface-hover text-ink-body',
  primary: 'bg-primary-soft text-primary-text',
  success: 'bg-success-soft text-success-text',
  danger: 'bg-danger-soft text-danger-text',
  warning: 'bg-warning-soft text-warning-text',
  pink: 'bg-pink-soft text-pink-text',
  info: 'bg-info-soft text-info-text',
};

export function Badge({ tone = 'neutral', children, icon, className }: { tone?: Tone; children: ReactNode; icon?: ReactNode; className?: string }) {
  return (
    <span className={cn('inline-flex h-6 items-center gap-1 whitespace-nowrap rounded-full px-2.5 text-xs font-medium', tones[tone], className)}>
      {icon}
      {children}
    </span>
  );
}

export function CountBubble({ n, tone = 'danger' }: { n: number; tone?: 'danger' | 'primary' }) {
  if (n <= 0) return null;
  return (
    <span className={cn('inline-flex h-5 min-w-5 items-center justify-center rounded-full px-1.5 text-xs font-bold text-white', tone === 'danger' ? 'bg-danger' : 'bg-primary')}>{n}</span>
  );
}

/** Status notice banner with an icon (meaning never conveyed by colour alone). */
export function Notice({ tone, icon, children, action, className }: { tone: Tone; icon?: ReactNode; children: ReactNode; action?: ReactNode; className?: string }) {
  return (
    <div role="status" className={cn('flex flex-wrap items-center gap-3 rounded-xl px-4 py-3', tones[tone], className)}>
      {icon && <span className="shrink-0" aria-hidden>{icon}</span>}
      <div className="min-w-0 flex-1 text-base font-medium">{children}</div>
      {action}
    </div>
  );
}

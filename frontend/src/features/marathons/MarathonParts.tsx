'use client';

import type { ReactNode } from 'react';
import { AlertCircle, CalendarClock, CircleCheck, Footprints, Hourglass, Info, RotateCcw, Search } from 'lucide-react';
import { Input, Notice, cn } from '@/components/ui';
import { useI18n } from '@/lib/i18n';
import type { MarathonInfo, MarathonLessonStatus, MarathonRating } from '@/types/api';
import { formatMarathonMoment } from './marathonTime';

/**
 * End-date banner under the marathon header. Before the end: blue notice "Марафон завершится…";
 * after it (finished is computed by the server): red notice "Марафон завершен… Доступ к марафону закрыт".
 */
export function MarathonDatesBanner({ info, className }: { info: MarathonInfo; className?: string }) {
  const { t, locale } = useI18n();
  const starts = !info.started && info.start_at ? (
    <Notice tone="info" icon={<CalendarClock size={20} />}>
      <span data-testid="marathon-starts-banner">{t('marathons.startsBanner', formatMarathonMoment(info.start_at, info.timezone, locale))}</span>
    </Notice>
  ) : null;
  const v = info.end_at ? formatMarathonMoment(info.end_at, info.timezone, locale) : null;
  const ends = !v ? null : info.finished ? (
    <Notice tone="danger" icon={<AlertCircle size={20} />}>
      <span data-testid="marathon-finished-banner">{t('marathons.finishedBanner', v)}</span>
    </Notice>
  ) : (
    <Notice tone="info" icon={<Info size={20} />}>
      <span data-testid="marathon-ends-banner">{t('marathons.endsBanner', v)}</span>
    </Notice>
  );
  if (!starts && !ends) return null;
  return <div className={cn('flex flex-col gap-2', className)}>{starts}{ends}</div>;
}

/** Search field of the marathon tabs. */
export function SearchField({ value, onChange, placeholder }: { value: string; onChange: (v: string) => void; placeholder: string }) {
  return (
    <div className="relative min-w-0 flex-1">
      <Search size={18} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-ink-faint" aria-hidden />
      <Input type="search" value={value} onChange={(e) => onChange(e.target.value)} placeholder={placeholder} aria-label={placeholder}
        className="border-line-soft pl-10" />
    </div>
  );
}

/** "Кол-во учеников N ·········" separator line. */
export function CountLine({ children }: { children: ReactNode }) {
  return (
    <div className="mt-4 flex items-center gap-2 text-base text-ink-soft">
      <span className="shrink-0">{children}</span>
      <span className="h-0 flex-1 border-t border-dotted border-line-strong" aria-hidden />
    </div>
  );
}

/** "Пройдено X из Y" with a progress bar; grey when the student is inactive. */
export function PassedBar({ done, total, muted, compact }: { done: number; total: number; muted?: boolean; compact?: boolean }) {
  const { t } = useI18n();
  const label = t('marathons.passed', { a: done, b: total });
  const pct = total > 0 ? Math.round((done / total) * 100) : 0;
  return (
    <div className={cn('flex h-6 min-w-0 items-center gap-3 rounded-md border border-line-soft px-2', compact ? 'w-auto' : 'flex-1')}>
      <span className="inline-flex shrink-0 items-center gap-1 whitespace-nowrap text-xs font-medium text-ink-body">
        <Footprints size={12} aria-hidden />{label}
      </span>
      {!compact && (
        <div className="h-1.5 min-w-[48px] flex-1 overflow-hidden rounded-full bg-surface-muted" role="progressbar" aria-label={label}
          aria-valuemin={0} aria-valuemax={total} aria-valuenow={done}>
          <div className={cn('h-full rounded-full transition-all', muted ? 'bg-ink-faint' : 'bg-success')} style={{ width: `${pct}%` }} />
        </div>
      )}
    </div>
  );
}

/**
 * Review status of a lesson (stage 6): not completed, awaiting review (hourglass), returned for revision,
 * completed. Icon and text together, so the meaning never relies on colour alone.
 */
export function LessonStatusChip({ status, className }: { status: MarathonLessonStatus; className?: string }) {
  const { t } = useI18n();
  const view = {
    none: { cls: 'bg-surface-muted text-ink-soft', icon: null, label: t('marathons.lessonNotDone') },
    submitted: { cls: 'bg-warning-soft text-warning-text', icon: <Hourglass size={12} aria-hidden />, label: t('marathons.lessonStatus.submitted') },
    revision: { cls: 'bg-danger-soft text-danger-text', icon: <RotateCcw size={12} aria-hidden />, label: t('marathons.lessonStatus.revision') },
    completed: { cls: 'bg-success-soft text-success-text', icon: <CircleCheck size={12} aria-hidden />, label: t('marathons.lessonDone') },
  }[status];
  return (
    <span data-testid="lesson-status" data-status={status}
      className={cn('inline-flex h-5 items-center gap-1 rounded px-1.5 text-xs font-medium', view.cls, className)}>
      {view.icon}{view.label}
    </span>
  );
}

export const RATINGS: { value: MarathonRating; emoji: string }[] = [
  { value: 'not_bad', emoji: '🙂' },
  { value: 'nice', emoji: '😊' },
  { value: 'amazing', emoji: '🤩' },
];

/** "Оценка за урок": the three ratings with the chosen one highlighted (read-only), and the teacher's comment. */
export function RatingView({ rating, comment, commentLabel }: { rating: MarathonRating; comment?: string | null; commentLabel: string }) {
  const { t } = useI18n();
  return (
    <div className="flex flex-col gap-3" data-testid="lesson-rating" data-rating={rating}>
      <p className="text-sm font-medium text-ink-body">{t('marathons.ratingLabel')}</p>
      <div className="flex flex-wrap gap-2">
        {RATINGS.map((r) => (
          <span key={r.value} aria-current={r.value === rating ? 'true' : undefined}
            className={cn('inline-flex h-10 items-center gap-2 rounded-lg border px-3 text-sm font-medium',
              r.value === rating ? 'border-primary bg-primary-soft text-primary-text' : 'border-line-soft text-ink-faint')}>
            <span aria-hidden>{r.emoji}</span>{t(`marathons.rating.${r.value}`)}
          </span>
        ))}
      </div>
      {comment && (
        <div className="rounded-lg border border-line-soft px-3 py-2">
          <p className="text-xs font-medium text-ink-soft">{commentLabel}</p>
          <p className="mt-1 whitespace-pre-wrap break-words text-sm text-ink-body" data-testid="lesson-comment">{comment}</p>
        </div>
      )}
    </div>
  );
}

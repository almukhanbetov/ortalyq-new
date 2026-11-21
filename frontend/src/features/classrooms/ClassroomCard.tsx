'use client';

import Link from 'next/link';
import { ChevronRight, ClipboardList, Footprints, Hourglass, Plus, User, Users } from 'lucide-react';
import { AvatarStack, cn } from '@/components/ui';
import { useT } from '@/lib/i18n';
import { classUrl } from '@/lib/routes';
import type { Classroom } from '@/types/api';
import { Cover } from './Cover';

const TypeIcon = ({ c, size }: { c: Classroom; size: number }) =>
  c.lesson_type === 'individual' ? <User size={size} strokeWidth={2.5} aria-hidden />
    : c.lesson_type === 'marathon' ? <Footprints size={size} strokeWidth={2.5} aria-hidden />
      : <Users size={size} strokeWidth={2.5} aria-hidden />;

/**
 * Shared bits so tiles and list rows show exactly the same data.
 * A marathon shows "Марафон" in mixed lists and its passing mode on the marathons tab (as in the original).
 */
function TypeChip({ c, showMode }: { c: Classroom; showMode?: boolean }) {
  const t = useT();
  const label = c.lesson_type === 'marathon'
    ? (showMode && c.marathon ? t(`marathons.mode.${c.marathon.mode}`) : t('marathons.typeBadge'))
    : c.lesson_type === 'individual' ? t('classes.typeIndividual') : t('classes.typeGroup');
  return (
    <span className="inline-flex h-[19px] items-center gap-1 rounded-full border border-line-soft px-1.5 text-xs font-medium text-ink-soft">
      <TypeIcon c={c} size={11} />
      {label}
    </span>
  );
}

/** Marathon counters: students and lessons (real counts from the server). */
function MarathonMetrics({ c }: { c: Classroom }) {
  const t = useT();
  const pill = (n: number) => cn('inline-flex h-5 items-center gap-0.5 rounded px-1 text-xs font-medium', n > 0 ? 'bg-success-soft text-success' : 'bg-surface-muted text-ink-soft');
  return (
    <span className="flex items-center gap-2">
      <span className={pill(c.students_count)} role="img" title={t('marathons.studentsCount', { n: c.students_count })} aria-label={t('marathons.studentsCount', { n: c.students_count })}>
        <Users size={12} aria-hidden /> {c.students_count}
      </span>
      <span className={pill(c.lessons_count)} role="img" title={t('marathons.lessonsCount', { n: c.lessons_count })} aria-label={t('marathons.lessonsCount', { n: c.lessons_count })}>
        <ClipboardList size={12} aria-hidden /> {c.lessons_count}
      </span>
      {c.my_role === 'teacher' && (c.homework_badge > 0 || c.marathon?.teacher_check) && (
        <span role="img" title={t('classes.badgeTeacher', { n: c.homework_badge })} aria-label={t('classes.badgeTeacher', { n: c.homework_badge })} data-testid="marathon-pending"
          className={cn('inline-flex h-5 items-center gap-0.5 rounded px-1 text-xs font-medium', c.homework_badge > 0 ? 'bg-warning-soft text-warning-text' : 'bg-surface-muted text-ink-soft')}>
          <Hourglass size={12} aria-hidden /> {c.homework_badge}
        </span>
      )}
    </span>
  );
}

function Metrics({ c }: { c: Classroom }) {
  const t = useT();
  if (c.lesson_type === 'marathon') return <MarathonMetrics c={c} />;
  const hwLabel = c.my_role === 'teacher' ? t('classes.badgeTeacher', { n: c.homework_badge }) : t('classes.badgeStudent', { n: c.homework_badge });
  return (
    <span className="flex items-center gap-2">
      <span title={hwLabel} aria-label={hwLabel} role="img"
        className={cn('inline-flex h-5 items-center gap-0.5 rounded px-1 text-xs font-medium',
          c.homework_badge > 0 ? 'bg-success-soft text-success' : 'bg-surface-muted text-ink-soft')}>
        <ClipboardList size={12} aria-hidden /> {c.homework_badge}
      </span>
      <span className="sr-only">{t('classes.members')}:</span>
      <AvatarStack people={c.members.map((m) => ({ name: m.full_name, color: m.avatar_color }))} size={20} />
    </span>
  );
}

/** Grid tile: square cover, title, type chip, homework counter and members. The whole tile opens the class. */
export function ClassroomCard({ c, showMode }: { c: Classroom; showMode?: boolean }) {
  const t = useT();
  return (
    <article className="min-w-0">
      <Link href={classUrl(c)} aria-label={t('classes.enterAria', { name: c.name })} className="group block rounded-lg focus-visible:shadow-focus">
        <div className="relative overflow-hidden rounded-lg">
          <Cover variant="tile" color={c.cover_color} imageId={c.cover_image_id} title={c.name} className="aspect-square w-full transition-transform duration-300 group-hover:scale-[1.03]" />
          <span className="absolute right-2 top-2 flex h-6 w-6 items-center justify-center rounded-md bg-white/70 text-ink-soft" aria-hidden>
            <TypeIcon c={c} size={13} />
          </span>
          <span className="absolute inset-x-3 bottom-3 flex h-9 items-center justify-center rounded-lg bg-surface text-sm font-medium text-primary-text opacity-0 shadow-card transition-opacity group-hover:opacity-100 group-focus-visible:opacity-100 [@media(hover:none)]:opacity-100" aria-hidden>
            {t('classes.enter')}
          </span>
        </div>
        <h2 className="mt-2 truncate text-lg font-medium text-ink-body group-hover:text-primary-text" title={c.name}>{c.name}</h2>
        <div className="mt-1"><TypeChip c={c} showMode={showMode} /></div>
        <div className="mt-2"><Metrics c={c} /></div>
      </Link>
    </article>
  );
}

/** List row with the same data as the tile. */
export function ClassroomRow({ c, showMode }: { c: Classroom; showMode?: boolean }) {
  const t = useT();
  return (
    <article className="min-w-0">
      <Link href={classUrl(c)} aria-label={t('classes.enterAria', { name: c.name })}
        className="group flex items-center gap-4 rounded-lg border border-line-soft p-3 transition-colors hover:border-line-strong hover:bg-surface-alt focus-visible:shadow-focus">
        <Cover variant="tile" color={c.cover_color} imageId={c.cover_image_id} title={c.name} className="h-16 w-16 shrink-0 rounded-lg [&>span]:text-xl" />
        <div className="min-w-0 flex-1">
          <h2 className="truncate text-lg font-medium text-ink-body group-hover:text-primary-text">{c.name}</h2>
          <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1.5">
            <TypeChip c={c} showMode={showMode} />
            <Metrics c={c} />
          </div>
        </div>
        <span className="hidden items-center gap-1 text-sm font-medium text-primary-text sm:inline-flex" aria-hidden>
          {t('classes.enter')} <ChevronRight size={16} />
        </span>
      </Link>
    </article>
  );
}

export function CreateClassTile({ onClick, list }: { onClick: () => void; list?: boolean }) {
  const t = useT();
  return (
    <button type="button" onClick={onClick}
      className={cn('flex items-center justify-center rounded-lg border border-line-soft bg-surface text-primary transition-colors hover:border-primary hover:bg-primary-soft/40',
        list ? 'h-[92px] w-full gap-3' : 'aspect-square w-full flex-col gap-[14px]')}>
      <span className="flex h-[18px] w-[18px] items-center justify-center rounded-[3px] bg-primary text-white" aria-hidden><Plus size={14} strokeWidth={3} /></span>
      <span className="text-lg font-bold">{t('classes.create')}</span>
    </button>
  );
}

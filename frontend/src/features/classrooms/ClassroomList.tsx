'use client';

import { useEffect, useRef, useState } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { classesUrl, typeFromParam, typeToParam, type ClassTypeFilter } from '@/lib/routes';
import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { ChevronDown, LayoutGrid, ListFilter, Rows3, Search, SearchX } from 'lucide-react';
import { Button, EmptyState, ErrorState, Loading, Pagination, cn } from '@/components/ui';
import { PageCard } from '@/components/layout/PageCard';
import { classroomsApi } from '@/lib/api/endpoints';
import { qk } from '@/lib/queryKeys';
import { useT } from '@/lib/i18n';
import { useDisclosure } from '@/hooks/useDisclosure';
import type { Me } from '@/types/api';
import { ClassroomCard, ClassroomRow, CreateClassTile } from './ClassroomCard';
import { CreateClassroomModal } from './CreateClassroomModal';
import { isTeaching } from '@/lib/roles';

const LIMIT = 24;
const VIEW_KEY = 'ortalyq_classes_view';
const SORT_KEY = 'ortalyq_classes_sort';

type TypeFilter = ClassTypeFilter;
type View = 'grid' | 'list';
// Sorting = field ("По умолчанию" = creation date, or name) × direction. The four combined values are
// what we store, so a choice saved by the previous version of the page still applies.
type SortOption = 'newest' | 'oldest' | 'name_asc' | 'name_desc';
type SortField = 'created' | 'name';
type SortOrder = 'asc' | 'desc';
const SORTS: Record<SortOption, { sort: SortField; order: SortOrder }> = {
  newest: { sort: 'created', order: 'desc' },
  oldest: { sort: 'created', order: 'asc' },
  name_asc: { sort: 'name', order: 'asc' },
  name_desc: { sort: 'name', order: 'desc' },
};
const sortOption = (sort: SortField, order: SortOrder) =>
  (Object.keys(SORTS) as SortOption[]).find((k) => SORTS[k].sort === sort && SORTS[k].order === order)!;
const FIELD_LABEL: Record<SortField, string> = { created: 'classes.sortDefault', name: 'classes.sortName' };
const HINTS: Partial<Record<TypeFilter, string>> = { individual: 'classes.hintIndividual', group: 'classes.hintGroup', marathon: 'classes.hintMarathon' };

function stored<T extends string>(key: string, allowed: readonly T[], fallback: T): T {
  try {
    const v = localStorage.getItem(key) as T | null;
    return v && allowed.includes(v) ? v : fallback;
  } catch {
    return fallback;
  }
}
function store(key: string, v: string) {
  try {
    localStorage.setItem(key, v);
  } catch {
    /* ignore */
  }
}

export function ClassroomList({ me }: { me: Me }) {
  const t = useT();
  const teacher = isTeaching(me.role);
  // The URL is the single source of truth for the applied filter and search (?type=1|2|group&q=…).
  // The input keeps its own text and pushes it to the URL after a 250 ms pause.
  const params = useSearchParams();
  const router = useRouter();
  const rawType = params.get('type');
  const type = typeFromParam(rawType);
  const urlQ = params.get('q') ?? '';
  const dq = urlQ.trim();
  const [q, setQ] = useState(urlQ);
  const pushed = useRef(urlQ);
  const go = (next: { type?: TypeFilter; q?: string }) => {
    const nq = next.q ?? dq;
    pushed.current = nq;
    router.replace(classesUrl({ type: next.type ?? type, q: nq }), { scroll: false });
  };
  const setType = (t: TypeFilter) => go({ type: t });
  const [view, setView] = useState<View>(() => stored(VIEW_KEY, ['grid', 'list'] as const, 'grid'));
  const [sortOpt, setSortOpt] = useState<SortOption>(() => stored(SORT_KEY, Object.keys(SORTS) as SortOption[], 'newest'));
  const [offset, setOffset] = useState(0);
  const create = useDisclosure();

  // Typing → URL (debounced).
  useEffect(() => {
    if (q.trim() === dq) return;
    const id = setTimeout(() => go({ q: q.trim() }), 250);
    return () => clearTimeout(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [q]);
  // URL → input, for back/forward and reset (ignore echoes of our own debounced pushes).
  useEffect(() => {
    if (urlQ !== pushed.current) {
      pushed.current = urlQ;
      setQ(urlQ);
    }
  }, [urlQ]);
  // Unknown ?type values fall back to "Все" with a clean URL; the legacy ?type=group becomes ?type=0.
  useEffect(() => {
    if (rawType !== null && rawType !== typeToParam(type)) router.replace(classesUrl({ type, q: dq }), { scroll: false });
  }, [rawType, type, dq, router]);
  useEffect(() => setOffset(0), [dq, type, sortOpt]);

  const { sort, order } = SORTS[sortOpt];
  const query = useQuery({
    queryKey: [...qk.classrooms(dq), type, sort, order, offset],
    queryFn: () => classroomsApi.list({ q: dq, type, sort, order, limit: LIMIT, offset }),
    placeholderData: keepPreviousData,
  });
  const items = query.data?.items ?? [];

  const chips: { value: TypeFilter; label: string }[] = [
    { value: '', label: t('classes.filterAll') },
    { value: 'individual', label: t('classes.filterIndividual') },
    { value: 'group', label: t('classes.filterGroup') },
    { value: 'marathon', label: t('classes.filterMarathon') },
  ];
  const changeView = (v: View) => { setView(v); store(VIEW_KEY, v); };
  const changeSort = (v: SortOption) => { setSortOpt(v); store(SORT_KEY, v); };
  const reset = () => go({ type: '', q: '' });
  // Group classes cannot be created yet, so the group tab has no create tile; its empty state still opens the type chooser.
  const canCreate = teacher;
  const showCreate = teacher && offset === 0 && !dq && type !== 'group';
  const empty = !query.isLoading && !query.error && items.length === 0;
  // Like the original: an empty list shows one framed placeholder instead of the counter and tiles.
  const blank = empty && !dq;

  return (
    <PageCard maxWidth={1024} className="pt-[18px] sm:pt-[18px] md:min-h-[calc(100dvh-64px-32px)]">
      <h1 className="text-[30px] font-bold leading-9 text-ink-body">{t('classes.title')}</h1>

      <div className="relative mt-6">
        <Search size={20} className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-ink-faint" aria-hidden />
        <input type="search" value={q} onChange={(e) => setQ(e.target.value)} placeholder={t('classes.searchPlaceholder')} aria-label={t('classes.searchPlaceholder')}
          className="h-10 w-full rounded-lg border border-line-soft bg-surface pl-10 pr-3 text-md text-ink-body placeholder:text-ink-faint focus:border-primary focus:outline-none focus:shadow-focus" />
      </div>

      <div role="radiogroup" aria-label={t('classes.typeFilter')} className="mt-2.5 flex flex-wrap gap-2">
        {chips.map((c) => {
          const hint = HINTS[c.value];
          const hintId = hint ? `classes-hint-${c.value}` : undefined;
          return (
            <span key={c.value || 'all'} className="group/chip relative">
              <button type="button" role="radio" aria-checked={type === c.value} aria-describedby={hintId} onClick={() => setType(c.value)}
                className={cn('h-8 rounded-full border-2 px-1.5 text-md font-medium transition-colors focus-visible:shadow-focus focus-visible:outline-none',
                  type === c.value ? 'border-ink-faint bg-ink-faint text-white' : 'border-surface-muted bg-surface text-ink-faint hover:text-ink-soft')}>
                {c.label}
              </button>
              {hint && (
                <span id={hintId} role="tooltip"
                  className="pointer-events-none invisible absolute bottom-full left-1/2 z-20 mb-[15px] w-[146px] -translate-x-1/2 rounded-md bg-tooltip px-3 py-2 text-center text-md leading-6 text-white opacity-0 shadow-pop transition-opacity [@media(hover:hover)]:group-hover/chip:visible [@media(hover:hover)]:group-hover/chip:opacity-100 group-has-[:focus-visible]/chip:visible group-has-[:focus-visible]/chip:opacity-100">
                  {t(hint)}
                </span>
              )}
            </span>
          );
        })}
      </div>

      <div className="mt-[18px] flex flex-wrap items-center gap-2">
        <div role="radiogroup" aria-label={t('classes.view')} className="inline-flex rounded-lg bg-surface-muted p-0.5">
          {([['grid', LayoutGrid, t('classes.viewGrid')], ['list', Rows3, t('classes.viewList')]] as const).map(([v, Icon, label]) => (
            <button key={v} type="button" role="radio" aria-checked={view === v} aria-label={label} title={label} onClick={() => changeView(v)}
              className={cn('flex h-7 w-7 items-center justify-center rounded-md transition-colors', view === v ? 'bg-surface text-ink-body' : 'text-ink-faint hover:text-ink-body')}>
              <Icon size={17} />
            </button>
          ))}
        </div>
        <span className="h-6 w-px bg-line-soft" aria-hidden />
        <div role="radiogroup" aria-label={t('classes.sortDirection')} className="inline-flex rounded-lg bg-surface-muted p-0.5">
          {(['desc', 'asc'] as const).map((o) => {
            const label = o === 'desc' ? t('classes.sortDesc') : t('classes.sortAsc');
            return (
              <button key={o} type="button" role="radio" aria-checked={order === o} aria-label={label} title={label} onClick={() => changeSort(sortOption(sort, o))}
                className={cn('flex h-7 w-7 items-center justify-center rounded-md transition-colors', order === o ? 'bg-surface text-ink-body' : 'text-ink-faint hover:text-ink-body')}>
                <ListFilter size={17} className={o === 'asc' ? 'rotate-180' : undefined} />
              </button>
            );
          })}
        </div>
        <label className="relative ml-2.5 inline-flex items-center">
          <span className="sr-only">{t('classes.sortField')}</span>
          <select value={sort} onChange={(e) => changeSort(sortOption(e.target.value as SortField, order))}
            className="h-8 cursor-pointer appearance-none rounded-md bg-transparent pr-6 text-md text-ink-body hover:text-ink focus:outline-none focus-visible:shadow-focus">
            {(Object.keys(FIELD_LABEL) as SortField[]).map((k) => <option key={k} value={k}>{t(FIELD_LABEL[k])}</option>)}
          </select>
          <ChevronDown size={16} className="pointer-events-none absolute right-0 text-ink-faint" aria-hidden />
        </label>
      </div>

      {!blank && !query.isLoading && !query.error && (
        <div className="mb-[21px] mt-[13px] flex items-center gap-3">
          <p className="shrink-0 text-md text-ink-faint" aria-live="polite">{t('classes.countLabel', { n: query.data?.total ?? 0 })}</p>
          <span className="h-0 flex-1 border-t-2 border-dotted border-line-soft" aria-hidden />
        </div>
      )}

      {query.isLoading ? <Loading /> : query.error ? <ErrorState error={query.error} onRetry={() => query.refetch()} /> : blank ? (
        <div className="mt-4 flex min-h-[546px] flex-col items-center justify-center rounded-lg border border-surface-muted px-4 py-10 text-center">
          <span className="text-[64px] leading-none" aria-hidden>🧑‍🏫</span>
          <p className="mt-[22px] text-[20px] leading-7 text-ink-soft">
            {teacher ? t('classes.emptyCreate') : t('classes.emptyStudent')}
          </p>
          {canCreate && (
            <button type="button" onClick={create.onOpen}
              className="mt-[22px] h-10 rounded-lg bg-primary px-4 text-md font-bold text-white transition-colors hover:bg-primary-hover focus-visible:shadow-focus focus-visible:outline-none">
              {t('classes.create')}
            </button>
          )}
        </div>
      ) : (
        <>
          {empty ? (
            <EmptyState icon={<SearchX size={26} />} title={t('classes.noResults')} text={t('classes.noResultsHint')}
              action={<Button variant="secondary" onClick={reset}>{t('classes.resetFilters')}</Button>} />
          ) : view === 'grid' ? (
            <div className={cn('grid grid-cols-2 gap-x-4 gap-y-6 sm:grid-cols-3 sm:gap-x-8 lg:grid-cols-4', query.isFetching && 'opacity-70')}>
              {showCreate && <CreateClassTile onClick={create.onOpen} />}
              {items.map((c) => <ClassroomCard key={c.id} c={c} showMode={type === 'marathon'} />)}
            </div>
          ) : (
            <div className={cn('flex flex-col gap-2.5', query.isFetching && 'opacity-70')}>
              {showCreate && <CreateClassTile onClick={create.onOpen} list />}
              {items.map((c) => <ClassroomRow key={c.id} c={c} showMode={type === 'marathon'} />)}
            </div>
          )}
          <Pagination total={query.data!.total} limit={LIMIT} offset={offset} onChange={setOffset} />
        </>
      )}
      {teacher && <CreateClassroomModal open={create.open} onClose={create.onClose} />}
    </PageCard>
  );
}

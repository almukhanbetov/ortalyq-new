'use client';

import { useEffect, useRef, useState } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { Check, Search, SearchX } from 'lucide-react';
import { Badge, Button, EmptyState, ErrorState, Loading, cn } from '@/components/ui';
import { PageCard } from '@/components/layout/PageCard';
import { libraryApi } from '@/lib/api/endpoints';
import { qk } from '@/lib/queryKeys';
import { useT } from '@/lib/i18n';
import { materialsCatalogUrl } from '@/lib/routes';
import { Cover } from '@/features/classrooms/Cover';
import type { CatalogCourse } from '@/types/api';
import { MaterialsTabs } from './MaterialsTabs';
import { CatalogCourseModal } from './CatalogCourseModal';

/**
 * «Материалы» → «Каталог»: ready-made author courses. The search is kept in the address (?q=…), the open course too
 * (?course=…). «Добавить в мои материалы» gives the teacher an own, editable copy in «Личные материалы».
 */
export function CatalogList() {
  const t = useT();
  const router = useRouter();
  const params = useSearchParams();
  const urlQ = params.get('q') ?? '';
  const dq = urlQ.trim();
  const openKey = params.get('course');
  const [q, setQ] = useState(urlQ);
  const pushed = useRef(urlQ);

  const go = (next: { q?: string; course?: string | null }) => {
    const sp = new URLSearchParams(materialsCatalogUrl({ q: next.q ?? dq }).split('?')[1] ?? '');
    const course = next.course === undefined ? openKey : next.course;
    if (course) sp.set('course', course);
    const s = sp.toString();
    router.replace(s ? `${materialsCatalogUrl()}?${s}` : materialsCatalogUrl(), { scroll: false });
  };

  useEffect(() => {
    if (q.trim() === dq) return;
    const id = setTimeout(() => { pushed.current = q.trim(); go({ q: q.trim() }); }, 250);
    return () => clearTimeout(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [q]);
  useEffect(() => {
    if (urlQ !== pushed.current) { pushed.current = urlQ; setQ(urlQ); }
  }, [urlQ]);

  const query = useQuery({ queryKey: qk.library(dq), queryFn: () => libraryApi.list(dq), placeholderData: keepPreviousData });
  const items = query.data?.items ?? [];

  return (
    <PageCard maxWidth={1024} className="pt-[18px] sm:pt-[18px] md:min-h-[calc(100dvh-64px-32px)]">
      <h1 className="text-[30px] font-bold leading-9 text-ink-body">{t('materials.title')}</h1>
      <MaterialsTabs current="catalog" />

      <div className="relative mt-5">
        <Search size={20} className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-ink-faint" aria-hidden />
        <input type="search" value={q} onChange={(e) => setQ(e.target.value)} placeholder={t('materials.searchPlaceholder')} aria-label={t('materials.catalogSearch')}
          className="h-10 w-full rounded-lg border border-line-soft bg-surface pl-10 pr-3 text-md text-ink-body placeholder:text-ink-faint focus:border-primary focus:outline-none focus:shadow-focus" />
      </div>

      {!query.isLoading && !query.error && (
        <div className="mb-[21px] mt-[18px] flex items-center gap-3">
          <p className="shrink-0 text-md text-ink-faint" aria-live="polite">{t('materials.catalogCount', { n: query.data?.total ?? 0 })}</p>
          <span className="h-0 flex-1 border-t-2 border-dotted border-line-soft" aria-hidden />
        </div>
      )}

      {query.isLoading ? <Loading /> : query.error ? <ErrorState error={query.error} onRetry={() => query.refetch()} /> : items.length === 0 ? (
        <EmptyState icon={<SearchX size={26} />} title={t('materials.noResults')} text={t('materials.noResultsHint')}
          action={<Button variant="secondary" onClick={() => setQ('')}>{t('materials.resetSearch')}</Button>} />
      ) : (
        <ul className={cn('grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-3', query.isFetching && 'opacity-70')}>
          {items.map((c) => <CatalogCard key={c.key} c={c} onOpen={() => go({ course: c.key })} />)}
        </ul>
      )}

      <CatalogCourseModal courseKey={openKey} onClose={() => go({ course: null })} />
    </PageCard>
  );
}

function CatalogCard({ c, onOpen }: { c: CatalogCourse; onOpen: () => void }) {
  const t = useT();
  return (
    <li className="relative flex flex-col overflow-hidden rounded-xl border border-line-soft bg-surface transition-shadow hover:shadow-card">
      <Cover color={c.cover_color} title={c.title} className="aspect-[16/9] w-full" />
      <div className="flex flex-1 flex-col gap-1 p-4">
        <h2 className="line-clamp-2 text-md font-bold text-ink-body">
          <button type="button" onClick={onOpen} className="text-left after:absolute after:inset-0 focus-visible:outline-none focus-visible:after:rounded-xl focus-visible:after:shadow-focus">{c.title}</button>
        </h2>
        <p className="line-clamp-2 text-sm text-ink-muted">{c.description}</p>
        <div className="mt-auto flex flex-wrap items-center gap-2 pt-2">
          <Badge tone="info">{t('materials.level', { level: c.level })}</Badge>
          <span className="text-sm text-ink-faint">{t('picker.lessonsCount', { n: c.lesson_count })}</span>
          {c.owned_course_id && <Badge tone="success" icon={<Check size={12} />}>{t('materials.inMyMaterials')}</Badge>}
        </div>
      </div>
    </li>
  );
}

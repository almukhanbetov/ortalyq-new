'use client';

import { useEffect, useRef, useState } from 'react';
import Link from 'next/link';
import { useRouter, useSearchParams } from 'next/navigation';
import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, ArrowRight, FileText, Plus, Search, SearchX } from 'lucide-react';
import { Badge, Button, EmptyState, ErrorState, IconButton, Loading, cn } from '@/components/ui';
import { PageCard } from '@/components/layout/PageCard';
import { documentsApi, materialsApi } from '@/lib/api/endpoints';
import { qk } from '@/lib/queryKeys';
import { useT } from '@/lib/i18n';
import { documentUrl, materialUrl, materialsUrl } from '@/lib/routes';
import { formatBytes } from '@/lib/format';
import { useDisclosure } from '@/hooks/useDisclosure';
import { Cover } from '@/features/classrooms/Cover';
import type { Course, MaterialDocument } from '@/types/api';
import { MaterialModal, type MaterialFields } from './MaterialModal';
import { MaterialsTabs } from './MaterialsTabs';

// The API returns at most 100 per page; reordering needs the whole list, so personal materials load in one page.
const LIMIT = 100;

/**
 * «Материалы» → «Личные материалы»: the teacher's own materials — courses with lessons and PDF documents.
 * The address is the single source of truth for the search (?q=…); the server returns only the teacher's own materials.
 */
export function CourseList() {
  const t = useT();
  const qc = useQueryClient();
  const router = useRouter();
  const params = useSearchParams();
  const urlQ = params.get('q') ?? '';
  const dq = urlQ.trim();
  const [q, setQ] = useState(urlQ);
  const pushed = useRef(urlQ);
  const create = useDisclosure();

  // Typing → URL (debounced); URL → input for back/forward (ignoring echoes of our own pushes).
  useEffect(() => {
    if (q.trim() === dq) return;
    const id = setTimeout(() => { pushed.current = q.trim(); router.replace(materialsUrl({ q: q.trim() }), { scroll: false }); }, 250);
    return () => clearTimeout(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [q]);
  useEffect(() => {
    if (urlQ !== pushed.current) { pushed.current = urlQ; setQ(urlQ); }
  }, [urlQ]);

  const query = useQuery({
    queryKey: qk.courses({ q: dq }),
    queryFn: () => materialsApi.courses({ q: dq, limit: LIMIT }),
    placeholderData: keepPreviousData,
  });
  const docs = useQuery({
    queryKey: qk.documents(dq),
    queryFn: () => documentsApi.list(dq),
    placeholderData: keepPreviousData,
  });
  const createM = useMutation({
    mutationFn: (v: MaterialFields) => materialsApi.createCourse(v),
    onSuccess: (c) => { qc.invalidateQueries({ queryKey: ['courses'] }); create.onClose(); router.push(materialUrl(c.id)); },
  });
  const reorder = useMutation({
    mutationFn: (ids: string[]) => materialsApi.reorderCourses(ids),
    onSettled: () => qc.invalidateQueries({ queryKey: ['courses'] }),
  });

  const items = query.data?.items ?? [];
  const documents = docs.data?.items ?? [];
  const courseTotal = query.data?.total ?? 0;
  const total = courseTotal + (docs.data?.total ?? 0);
  const loading = query.isLoading || docs.isLoading;
  const error = query.error ?? docs.error;
  const empty = !loading && !error && items.length === 0 && documents.length === 0;
  const blank = empty && !dq;
  // Courses keep their own order; it can be changed only on the full, unfiltered list (the server checks that every
  // course is listed once). PDF documents follow the courses, newest first.
  const canReorder = !dq && items.length === courseTotal && items.length > 1;
  const move = (i: number, d: -1 | 1) => {
    const ids = items.map((x) => x.id);
    [ids[i], ids[i + d]] = [ids[i + d], ids[i]];
    reorder.mutate(ids);
  };

  return (
    <PageCard maxWidth={1024} className="pt-[18px] sm:pt-[18px] md:min-h-[calc(100dvh-64px-32px)]">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-[30px] font-bold leading-9 text-ink-body">{t('materials.title')}</h1>
        {!blank && <Button icon={<Plus size={18} />} onClick={create.onOpen}>{t('materials.createMaterial')}</Button>}
      </div>

      <MaterialsTabs current="personal" />

      <div className="relative mt-5">
        <Search size={20} className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-ink-faint" aria-hidden />
        <input type="search" value={q} onChange={(e) => setQ(e.target.value)} placeholder={t('materials.searchPlaceholder')} aria-label={t('materials.searchPlaceholder')}
          className="h-10 w-full rounded-lg border border-line-soft bg-surface pl-10 pr-3 text-md text-ink-body placeholder:text-ink-faint focus:border-primary focus:outline-none focus:shadow-focus" />
      </div>

      {!blank && !loading && !error && (
        <div className="mb-[21px] mt-[18px] flex items-center gap-3">
          <p className="shrink-0 text-md text-ink-faint" aria-live="polite">{t('materials.count', { n: total })}</p>
          <span className="h-0 flex-1 border-t-2 border-dotted border-line-soft" aria-hidden />
        </div>
      )}

      {loading ? <Loading /> : error ? <ErrorState error={error} onRetry={() => { query.refetch(); docs.refetch(); }} /> : blank ? (
        <div className="mt-4 flex min-h-[420px] flex-col items-center justify-center rounded-lg border border-surface-muted px-4 py-10 text-center">
          <span className="text-[64px] leading-none" aria-hidden>📚</span>
          <p className="mt-[22px] max-w-md text-[20px] leading-7 text-ink-soft">{t('materials.empty')}</p>
          <Button className="mt-[22px]" icon={<Plus size={18} />} onClick={create.onOpen}>{t('materials.createMaterial')}</Button>
        </div>
      ) : empty ? (
        <EmptyState icon={<SearchX size={26} />} title={t('materials.noResults')} text={t('materials.noResultsHint')}
          action={<Button variant="secondary" onClick={() => setQ('')}>{t('materials.resetSearch')}</Button>} />
      ) : (
        <ul className={cn('grid grid-cols-1 gap-x-6 gap-y-6 sm:grid-cols-2 lg:grid-cols-3', (query.isFetching || docs.isFetching) && 'opacity-70')}>
          {items.map((c, i) => (
            <MaterialCard key={c.id} c={c}
              order={canReorder ? { first: i === 0, last: i === items.length - 1, busy: reorder.isPending, move: (d) => move(i, d) } : undefined} />
          ))}
          {documents.map((d) => <DocumentCard key={d.id} d={d} />)}
        </ul>
      )}

      <MaterialModal open={create.open} onClose={create.onClose} heading={t('materials.createMaterial')}
        loading={createM.isPending} error={createM.error} onSubmit={(v) => createM.mutate(v)} />
    </PageCard>
  );
}

/** A PDF document: opens the viewer. */
function DocumentCard({ d }: { d: MaterialDocument }) {
  const t = useT();
  return (
    <li className="group relative flex flex-col overflow-hidden rounded-xl border border-line-soft bg-surface transition-shadow hover:shadow-card" data-testid="document-card">
      <div className="flex aspect-[16/9] w-full items-center justify-center bg-danger-soft text-danger-text" aria-hidden>
        <FileText size={48} strokeWidth={1.5} />
      </div>
      <div className="flex flex-1 flex-col gap-1 p-4">
        <h2 className="line-clamp-2 text-md font-bold text-ink-body">
          <Link href={documentUrl(d.id)} className="after:absolute after:inset-0 focus-visible:outline-none focus-visible:after:rounded-xl focus-visible:after:shadow-focus">{d.title}</Link>
        </h2>
        <p className="truncate text-sm text-ink-muted">{d.file_name}</p>
        <div className="mt-auto flex items-center justify-between pt-2">
          <span className="text-sm text-ink-faint">{formatBytes(d.size_bytes)}</span>
          <Badge tone="danger" icon={<FileText size={12} />}>{t('materials.pdfBadge')}</Badge>
        </div>
      </div>
    </li>
  );
}

function MaterialCard({ c, order }: { c: Course; order?: { first: boolean; last: boolean; busy: boolean; move: (d: -1 | 1) => void } }) {
  const t = useT();
  return (
    <li className="group relative flex flex-col overflow-hidden rounded-xl border border-line-soft bg-surface transition-shadow hover:shadow-card">
      <Cover color={c.cover_color} title={c.title} className="aspect-[16/9] w-full" />
      <div className="flex flex-1 flex-col gap-1 p-4">
        <h2 className="line-clamp-2 text-md font-bold text-ink-body">
          {/* The stretched link makes the whole card clickable; the order buttons sit above it. */}
          <Link href={materialUrl(c.id)} className="after:absolute after:inset-0 focus-visible:outline-none focus-visible:after:rounded-xl focus-visible:after:shadow-focus">{c.title}</Link>
        </h2>
        {c.description && <p className="line-clamp-2 text-sm text-ink-muted">{c.description}</p>}
        <div className="mt-auto flex items-center justify-between pt-2">
          <span className="text-sm text-ink-faint">{t('picker.lessonsCount', { n: c.lesson_count })}</span>
          {order && (
            <span className="relative z-10 flex">
              <IconButton label={t('materials.moveEarlier')} disabled={order.first || order.busy} onClick={() => order.move(-1)}><ArrowLeft size={16} /></IconButton>
              <IconButton label={t('materials.moveLater')} disabled={order.last || order.busy} onClick={() => order.move(1)}><ArrowRight size={16} /></IconButton>
            </span>
          )}
        </div>
      </div>
    </li>
  );
}

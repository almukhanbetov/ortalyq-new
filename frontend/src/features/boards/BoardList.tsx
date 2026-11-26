'use client';

import { useEffect, useRef, useState } from 'react';
import Link from 'next/link';
import { useRouter, useSearchParams } from 'next/navigation';
import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { PenTool, Plus, Search, SearchX } from 'lucide-react';
import { Button, EmptyState, ErrorState, Loading, cn } from '@/components/ui';
import { PageCard } from '@/components/layout/PageCard';
import { boardsApi } from '@/lib/api/endpoints';
import { qk } from '@/lib/queryKeys';
import { useI18n } from '@/lib/i18n';
import { formatDateTime } from '@/lib/format';
import { errorMessage } from '@/lib/errors';
import { boardUrl, boardsUrl } from '@/lib/routes';
import { useDisclosure } from '@/hooks/useDisclosure';
import { useMe } from '@/hooks/useMe';
import { MaterialsTabs } from '@/features/materials/MaterialsTabs';
import { TitleModal } from '@/features/materials/TitleModal';
import type { BoardSummary } from '@/types/api';

/**
 * «Материалы» → «Интерактивные доски»: the teacher's own boards, newest first. The search is kept in the address
 * (?q=…); the server returns only the caller's boards.
 */
export function BoardList() {
  const { t } = useI18n();
  const qc = useQueryClient();
  const router = useRouter();
  const params = useSearchParams();
  const urlQ = params.get('q') ?? '';
  const dq = urlQ.trim();
  const [q, setQ] = useState(urlQ);
  const pushed = useRef(urlQ);
  const create = useDisclosure();

  useEffect(() => {
    if (q.trim() === dq) return;
    const id = setTimeout(() => { pushed.current = q.trim(); router.replace(boardsUrl({ q: q.trim() }), { scroll: false }); }, 250);
    return () => clearTimeout(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [q]);
  useEffect(() => {
    if (urlQ !== pushed.current) { pushed.current = urlQ; setQ(urlQ); }
  }, [urlQ]);

  const query = useQuery({ queryKey: qk.boards(dq), queryFn: () => boardsApi.list(dq), placeholderData: keepPreviousData });
  const createM = useMutation({
    mutationFn: (title: string) => boardsApi.create(title),
    onSuccess: (b) => { qc.invalidateQueries({ queryKey: ['boards'] }); create.onClose(); router.push(boardUrl(b.id)); },
  });

  const items = query.data?.items ?? [];
  const total = query.data?.total ?? 0;
  const empty = !query.isLoading && !query.error && items.length === 0;
  const blank = empty && !dq;

  return (
    <PageCard maxWidth={1024} className="pt-[18px] sm:pt-[18px] md:min-h-[calc(100dvh-64px-32px)]">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-[30px] font-bold leading-9 text-ink-body">{t('materials.title')}</h1>
        {!blank && <Button icon={<Plus size={18} />} onClick={create.onOpen}>{t('boards.create')}</Button>}
      </div>
      <MaterialsTabs current="boards" />

      <div className="relative mt-5">
        <Search size={20} className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-ink-faint" aria-hidden />
        <input type="search" value={q} onChange={(e) => setQ(e.target.value)} placeholder={t('materials.searchPlaceholder')} aria-label={t('boards.search')}
          className="h-10 w-full rounded-lg border border-line-soft bg-surface pl-10 pr-3 text-md text-ink-body placeholder:text-ink-faint focus:border-primary focus:outline-none focus:shadow-focus" />
      </div>

      {!blank && !query.isLoading && !query.error && (
        <div className="mb-[21px] mt-[18px] flex items-center gap-3">
          <p className="shrink-0 text-md text-ink-faint" aria-live="polite">{t('boards.count', { n: total })}</p>
          <span className="h-0 flex-1 border-t-2 border-dotted border-line-soft" aria-hidden />
        </div>
      )}

      {query.isLoading ? <Loading /> : query.error ? <ErrorState error={query.error} onRetry={() => query.refetch()} /> : blank ? (
        <div className="mt-4 flex min-h-[420px] flex-col items-center justify-center rounded-lg border border-surface-muted px-4 py-10 text-center">
          <span className="flex h-16 w-16 items-center justify-center rounded-2xl bg-primary-soft text-primary-text" aria-hidden><PenTool size={30} /></span>
          <p className="mt-[22px] max-w-md text-[20px] leading-7 text-ink-soft">{t('boards.empty')}</p>
          <Button className="mt-[22px]" icon={<Plus size={18} />} onClick={create.onOpen}>{t('boards.create')}</Button>
        </div>
      ) : empty ? (
        <EmptyState icon={<SearchX size={26} />} title={t('boards.noResults')} text={t('materials.noResultsHint')}
          action={<Button variant="secondary" onClick={() => setQ('')}>{t('materials.resetSearch')}</Button>} />
      ) : (
        <ul className={cn('grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-3', query.isFetching && 'opacity-70')}>
          {items.map((b) => <BoardCard key={b.id} b={b} />)}
        </ul>
      )}

      <TitleModal open={create.open} onClose={create.onClose} title={t('boards.create')} label={t('boards.name')}
        loading={createM.isPending} error={createM.error ? errorMessage(createM.error, t) : undefined} onSubmit={(v) => createM.mutate(v)} />
    </PageCard>
  );
}

function BoardCard({ b }: { b: BoardSummary }) {
  const { t, locale } = useI18n();
  const me = useMe().data;
  return (
    <li className="relative flex flex-col overflow-hidden rounded-xl border border-line-soft bg-surface transition-shadow hover:shadow-card">
      {/* A plain sheet as the cover: boards have no picture of their own yet. */}
      <div className="flex aspect-[16/9] w-full items-center justify-center bg-surface-alt text-ink-faint" aria-hidden>
        <PenTool size={36} />
      </div>
      <div className="flex flex-1 flex-col gap-1 p-4">
        <h2 className="line-clamp-2 text-md font-bold text-ink-body">
          <Link href={boardUrl(b.id)} className="after:absolute after:inset-0 focus-visible:outline-none focus-visible:after:rounded-xl focus-visible:after:shadow-focus">{b.title}</Link>
        </h2>
        <p className="mt-auto pt-2 text-sm text-ink-faint">{t('boards.updated', { date: formatDateTime(b.updated_at, locale, me?.timezone) })}</p>
      </div>
    </li>
  );
}

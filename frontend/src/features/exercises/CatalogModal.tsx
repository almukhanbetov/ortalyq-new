'use client';

import { useQuery } from '@tanstack/react-query';
import { AlignLeft, ArrowDownUp, CheckSquare, Image as ImageIcon, Layers, Link2, ListChecks, Minus, Plug, PencilLine, Shuffle, StickyNote, ToggleLeft, Lock } from 'lucide-react';
import type { ReactNode } from 'react';
import { Modal, QueryState, cn } from '@/components/ui';
import { exercisesApi } from '@/lib/api/endpoints';
import { qk } from '@/lib/queryKeys';
import { useT } from '@/lib/i18n';

const icons: Record<string, ReactNode> = {
  image: <ImageIcon size={22} />, audio_video: <Layers size={22} />, gaps: <PencilLine size={22} />, test: <ListChecks size={22} />,
  true_false: <ToggleLeft size={22} />, ordering: <ArrowDownUp size={22} />, matching: <Shuffle size={22} />, text: <AlignLeft size={22} />,
  word_set: <CheckSquare size={22} />, note: <StickyNote size={22} />, link: <Link2 size={22} />, divider: <Minus size={22} />, integrations: <Plug size={22} />,
};

/** Kinds that cannot go to homework (read-only blocks have no answer to send). */
const LESSON_ONLY = new Set(['text']);

export function CatalogModal({ open, onClose, onPick, homework }: { open: boolean; onClose: () => void; onPick: (kind: string) => void; homework?: boolean }) {
  const t = useT();
  const q = useQuery({ queryKey: qk.catalog, queryFn: exercisesApi.catalog, enabled: open, staleTime: Infinity });
  return (
    <Modal open={open} onClose={onClose} title={t('catalog.title')} size="lg">
      <p className="mb-4 rounded-lg bg-info-soft px-3 py-2 text-sm text-info-text">{t('catalog.hint')}</p>
      <QueryState query={q}>
        {(data) => (
          <ul className="grid grid-cols-2 gap-3 pb-4 sm:grid-cols-3 md:grid-cols-4">
            {data.items.map((x) => ({ ...x, available: x.available && !(homework && LESSON_ONLY.has(x.kind)) })).map((it) => (
              <li key={it.kind}>
                <button type="button" disabled={!it.available} onClick={() => onPick(it.kind)}
                  aria-disabled={!it.available}
                  className={cn('flex h-full w-full flex-col items-center gap-2 rounded-xl border p-4 text-center transition-colors',
                    it.available ? 'border-line hover:border-primary hover:bg-primary-soft' : 'cursor-not-allowed border-dashed border-line bg-surface-alt')}>
                  <span className={cn('flex h-11 w-11 items-center justify-center rounded-lg', it.available ? 'bg-primary-soft text-primary-text' : 'bg-surface-hover text-ink-faint')}>{icons[it.kind]}</span>
                  <span className={cn('text-sm font-medium', it.available ? 'text-ink' : 'text-ink-muted')}>{t(`catalog.types.${it.kind}`)}</span>
                  {!it.available && <span className="inline-flex items-center gap-1 text-xs text-ink-muted"><Lock size={11} /> {t('catalog.unavailable')}</span>}
                </button>
              </li>
            ))}
          </ul>
        )}
      </QueryState>
    </Modal>
  );
}

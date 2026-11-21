'use client';

import { useEffect, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Search, X } from 'lucide-react';
import { Button, ErrorState, IconButton, Input, Loading, Notice, cn } from '@/components/ui';
import { chatApi } from '@/lib/api/endpoints';
import { qk } from '@/lib/queryKeys';
import { useT } from '@/lib/i18n';
import type { Gif } from '@/types/api';

/** "Reactions" panel: GIPHY search → pick → optional comment → send. */
export function GifPicker({ onClose, onSend, sending }: { onClose: () => void; onSend: (gif: Gif, caption: string) => void; sending: boolean }) {
  const t = useT();
  const [q, setQ] = useState('');
  const [dq, setDq] = useState('');
  const [picked, setPicked] = useState<Gif | null>(null);
  const [caption, setCaption] = useState('');
  useEffect(() => {
    const id = setTimeout(() => setDq(q.trim()), 350);
    return () => clearTimeout(id);
  }, [q]);
  const status = useQuery({ queryKey: qk.gifStatus, queryFn: chatApi.gifStatus, staleTime: 60_000 });
  const gifs = useQuery({ queryKey: ['gifs', dq], queryFn: () => chatApi.searchGifs(dq), enabled: !!status.data?.enabled, staleTime: 60_000 });

  return (
    <div className="absolute inset-x-3 bottom-full z-20 mb-2 flex max-h-[420px] flex-col rounded-2xl border border-line bg-surface shadow-pop animate-pop-in" role="dialog" aria-label={t('chat.reactions')}>
      <div className="flex items-center justify-between px-4 pb-1 pt-3">
        <h3 className="text-base font-bold text-ink">{t('chat.reactions')}</h3>
        <IconButton size="sm" label={t('common.close')} onClick={onClose}><X size={16} /></IconButton>
      </div>
      {status.isLoading ? <Loading /> : !status.data?.enabled ? (
        <Notice tone="warning" className="m-4">{t('chat.gifUnavailable')}</Notice>
      ) : picked ? (
        <div className="flex flex-col gap-3 p-4">
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img src={picked.url} alt={t('chat.gifAlt', { title: picked.title })} className="max-h-[200px] self-start rounded-lg" />
          <Input autoFocus value={caption} maxLength={500} onChange={(e) => setCaption(e.target.value)} placeholder={t('chat.captionPlaceholder')} aria-label={t('chat.caption')}
            onKeyDown={(e) => e.key === 'Enter' && onSend(picked, caption)} />
          <div className="flex justify-end gap-2">
            <Button variant="secondary" onClick={() => setPicked(null)}>{t('common.back')}</Button>
            <Button onClick={() => onSend(picked, caption)} loading={sending}>{t('chat.sendGif')}</Button>
          </div>
        </div>
      ) : (
        <>
          <div className="relative px-4 py-2">
            <Search size={16} className="pointer-events-none absolute left-7 top-1/2 -translate-y-1/2 text-ink-faint" aria-hidden />
            <Input autoFocus value={q} onChange={(e) => setQ(e.target.value)} placeholder={t('chat.gifSearch')} aria-label={t('chat.gifSearch')} className="h-9 pl-9" />
          </div>
          <div className="min-h-[160px] flex-1 overflow-y-auto px-4 pb-2 scrollbar-thin">
            {gifs.isLoading ? <Loading /> : gifs.error ? <ErrorState error={gifs.error} onRetry={() => gifs.refetch()} /> : gifs.data!.items.length === 0 ? (
              <p className="py-8 text-center text-sm text-ink-muted">{t('chat.gifEmpty')}</p>
            ) : (
              <ul className="grid grid-cols-3 gap-2" aria-label={t('chat.gifSelect')}>
                {gifs.data!.items.map((g) => (
                  <li key={g.id}>
                    <button type="button" onClick={() => setPicked(g)} className={cn('block w-full overflow-hidden rounded-lg bg-surface-hover focus-visible:shadow-focus')}>
                      {/* eslint-disable-next-line @next/next/no-img-element */}
                      <img src={g.preview_url} alt={g.title} loading="lazy" className="h-20 w-full object-cover" />
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </div>
          <p className="px-4 pb-2 text-right text-[10px] uppercase tracking-wide text-ink-faint">{t('chat.poweredBy')}</p>
        </>
      )}
    </div>
  );
}

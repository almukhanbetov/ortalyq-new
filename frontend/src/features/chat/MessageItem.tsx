'use client';

import { Download, FileText, MoreHorizontal, Pencil, RotateCw, Trash2 } from 'lucide-react';
import { Avatar, IconButton, Menu, cn } from '@/components/ui';
import { attachmentUrl } from '@/lib/api/client';
import { formatBytes, formatTime } from '@/lib/format';
import { useI18n } from '@/lib/i18n';
import type { ChatMessage, Me } from '@/types/api';

export interface PendingMessage {
  client_id: string;
  kind: 'text' | 'gif' | 'file';
  body: string;
  status: 'sending' | 'failed';
  preview?: string;
  created_at: string;
}

export function MessageItem({ m, me, grouped, onEdit, onDelete }: { m: ChatMessage; me: Me; grouped: boolean; onEdit: () => void; onDelete: () => void }) {
  const { t, locale } = useI18n();
  const own = m.author.id === me.id;
  return (
    <li className={cn('group flex gap-2', own ? 'flex-row-reverse' : 'flex-row', grouped ? 'mt-1' : 'mt-3')}>
      <div className="w-8 shrink-0">{!grouped && <Avatar name={m.author.full_name} color={m.author.avatar_color} size={32} />}</div>
      <div className={cn('flex max-w-[78%] flex-col', own ? 'items-end' : 'items-start')}>
        {!grouped && (
          <p className={cn('mb-1 flex items-baseline gap-2 text-xs', own && 'flex-row-reverse')}>
            <span className="font-medium text-ink">{own ? t('classroom.you') : m.author.full_name}</span>
            <time dateTime={m.created_at} className="text-ink-muted">{formatTime(m.created_at, locale, me.timezone)}</time>
          </p>
        )}
        <div className={cn('flex items-start gap-1', own && 'flex-row-reverse')}>
          <div className={cn('overflow-hidden rounded-2xl text-base', own ? 'rounded-tr-md bg-primary-soft text-ink' : 'rounded-tl-md bg-surface-hover text-ink')}>
            {m.kind === 'gif' && m.gif && (
              // eslint-disable-next-line @next/next/no-img-element
              <img src={m.gif.url} alt={t('chat.gifAlt', { title: m.gif.title })} className="block max-h-[220px] max-w-full" />
            )}
            {m.kind === 'file' && m.attachment && (
              m.attachment.mime_type.startsWith('image/') ? (
                <a href={attachmentUrl(m.attachment.id)} className="block" aria-label={`${t('chat.download')}: ${m.attachment.name}`}>
                  {/* eslint-disable-next-line @next/next/no-img-element */}
                  <img src={attachmentUrl(m.attachment.id, true)} alt={m.attachment.name} className="block max-h-[220px] max-w-full" />
                </a>
              ) : (
                <a href={attachmentUrl(m.attachment.id)} className="flex items-center gap-3 px-3 py-2.5 hover:bg-black/5">
                  <span className="flex h-9 w-9 items-center justify-center rounded-lg bg-surface text-primary-text"><FileText size={18} /></span>
                  <span className="min-w-0">
                    <span className="block truncate font-medium">{m.attachment.name}</span>
                    <span className="block text-xs text-ink-muted">{formatBytes(m.attachment.size_bytes)}</span>
                  </span>
                  <Download size={16} className="text-ink-muted" aria-label={t('chat.download')} />
                </a>
              )
            )}
            {m.body && <p className="whitespace-pre-wrap break-words px-3 py-2">{m.body}</p>}
          </div>
          {own && (
            <Menu label={t('chat.menu')} align={own ? 'right' : 'left'} items={[
              { key: 'edit', label: t('chat.edit'), icon: <Pencil size={15} />, onSelect: onEdit },
              { key: 'delete', label: t('chat.delete'), icon: <Trash2 size={15} />, tone: 'danger', onSelect: onDelete },
            ]} trigger={(p) => (
              <IconButton {...p} size="sm" label={t('chat.menu')} className="opacity-60 focus:opacity-100 group-hover:opacity-100"><MoreHorizontal size={16} /></IconButton>
            )} />
          )}
        </div>
        {m.edited_at && <span className="mt-0.5 text-[11px] text-ink-muted">{t('chat.edited')}</span>}
      </div>
    </li>
  );
}

export function PendingItem({ p, onRetry }: { p: PendingMessage; onRetry: () => void }) {
  const { t } = useI18n();
  return (
    <li className="mt-1 flex flex-row-reverse gap-2">
      <div className="w-8 shrink-0" />
      <div className="flex max-w-[78%] flex-col items-end">
        <div className={cn('rounded-2xl rounded-tr-md px-3 py-2 text-base', p.status === 'failed' ? 'bg-danger-soft text-danger-text' : 'bg-primary-soft/60 text-ink-body')}>
          {p.preview ?? p.body}
        </div>
        <span className="mt-0.5 flex items-center gap-1 text-[11px] text-ink-muted" role="status">
          {p.status === 'sending' ? t('chat.sending') : (
            <>
              <span className="text-danger-text">{t('chat.failed')}</span>
              <button type="button" onClick={onRetry} className="inline-flex items-center gap-0.5 font-medium text-primary-text hover:underline"><RotateCw size={11} />{t('chat.retry')}</button>
            </>
          )}
        </span>
      </div>
    </li>
  );
}

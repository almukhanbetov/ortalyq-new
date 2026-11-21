'use client';

import { Fragment, useEffect, useLayoutEffect, useRef, useState, type KeyboardEvent } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Paperclip, Send, Smile, X } from 'lucide-react';
import { Button, ConfirmDialog, EmptyState, ErrorState, IconButton, Loading, useToast } from '@/components/ui';
import { chatApi } from '@/lib/api/endpoints';
import { qk } from '@/lib/queryKeys';
import { dayKey, formatDay, newClientId } from '@/lib/format';
import { useI18n } from '@/lib/i18n';
import { errorMessage } from '@/lib/errors';
import type { ChatMessage, Gif, Me } from '@/types/api';
import { removeMessage, upsertMessage, type ChatPage } from './chatCache';
import { GifPicker } from './GifPicker';
import { MessageItem, PendingItem, type PendingMessage } from './MessageItem';

const MAX_MB = 10;

type SendBody = { client_id: string; kind: 'text' | 'gif' | 'file'; body?: string; gif_id?: string; attachment_id?: string };

export function ChatPanel({ classroomId, me, onClose }: { classroomId: string; me: Me; onClose: () => void }) {
  const { t, locale } = useI18n();
  const qc = useQueryClient();
  const toast = useToast();
  const list = useRef<HTMLDivElement>(null);
  const fileInput = useRef<HTMLInputElement>(null);
  const [text, setText] = useState('');
  const [editing, setEditing] = useState<ChatMessage | null>(null);
  const [deleting, setDeleting] = useState<ChatMessage | null>(null);
  const [gifOpen, setGifOpen] = useState(false);
  const [pending, setPending] = useState<(PendingMessage & { req: SendBody })[]>([]);
  const [uploading, setUploading] = useState(false);
  const [loadingOlder, setLoadingOlder] = useState(false);
  const stickBottom = useRef(true);

  const q = useQuery({ queryKey: qk.messages(classroomId), queryFn: () => chatApi.list(classroomId) });

  // After a reconnect the latest page is re-read from the API; pending items are
  // matched by client_id so a message confirmed meanwhile is never shown twice.
  const items = q.data?.items ?? [];
  const confirmedIds = new Set(items.map((m) => m.client_id));
  const visiblePending = pending.filter((p) => !confirmedIds.has(p.client_id));

  useLayoutEffect(() => {
    const el = list.current;
    if (el && stickBottom.current) el.scrollTop = el.scrollHeight;
  }, [items.length, visiblePending.length]);

  const send = async (req: SendBody, preview?: string) => {
    setPending((p) => [...p.filter((x) => x.client_id !== req.client_id), { client_id: req.client_id, kind: req.kind, body: req.body ?? '', preview, status: 'sending', created_at: new Date().toISOString(), req }]);
    stickBottom.current = true;
    try {
      const m = await chatApi.send(classroomId, req);
      upsertMessage(qc, classroomId, m);
      setPending((p) => p.filter((x) => x.client_id !== req.client_id));
      return true;
    } catch (e) {
      setPending((p) => p.map((x) => (x.client_id === req.client_id ? { ...x, status: 'failed' } : x)));
      toast(errorMessage(e, t), 'error');
      return false;
    }
  };

  const edit = useMutation({
    mutationFn: ({ id, body }: { id: string; body: string }) => chatApi.edit(id, body),
    onSuccess: (m) => { upsertMessage(qc, classroomId, m); setEditing(null); setText(''); },
    onError: (e) => toast(errorMessage(e, t), 'error'),
  });
  const del = useMutation({
    mutationFn: (id: string) => chatApi.remove(id),
    onSuccess: (_, id) => { removeMessage(qc, classroomId, id); setDeleting(null); },
    onError: (e) => toast(errorMessage(e, t), 'error'),
  });

  const submit = () => {
    const body = text.trim();
    if (editing) {
      if (editing.kind === 'text' && !body) return;
      edit.mutate({ id: editing.id, body });
      return;
    }
    if (!body) return;
    setText('');
    send({ client_id: newClientId(), kind: 'text', body });
  };

  const onKey = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) {
      e.preventDefault();
      submit();
    }
    if (e.key === 'Escape' && editing) {
      setEditing(null);
      setText('');
    }
  };

  const onFile = async (file: File | undefined) => {
    if (!file) return;
    if (file.size > MAX_MB * 1024 * 1024) {
      toast(t('chat.fileTooLarge', { n: MAX_MB }), 'error');
      return;
    }
    setUploading(true);
    try {
      const a = await chatApi.upload(classroomId, file);
      const caption = text.trim();
      setText('');
      await send({ client_id: newClientId(), kind: 'file', attachment_id: a.id, body: caption }, `📎 ${a.name}`);
    } catch (e) {
      toast(errorMessage(e, t), 'error');
    } finally {
      setUploading(false);
      if (fileInput.current) fileInput.current.value = '';
    }
  };

  const sendGif = async (gif: Gif, caption: string) => {
    const ok = await send({ client_id: newClientId(), kind: 'gif', gif_id: gif.id, body: caption.trim() }, `GIF · ${gif.title}`);
    if (ok) setGifOpen(false);
  };

  const loadOlder = async () => {
    const first = items[0];
    if (!first) return;
    setLoadingOlder(true);
    const el = list.current;
    const prevHeight = el?.scrollHeight ?? 0;
    try {
      const older = await chatApi.list(classroomId, { at: first.created_at, id: first.id });
      qc.setQueryData<ChatPage>(qk.messages(classroomId), (old) => {
        if (!old) return old;
        const ids = new Set(old.items.map((m) => m.id));
        return { items: [...older.items.filter((m) => !ids.has(m.id)), ...old.items], has_more: older.has_more };
      });
      stickBottom.current = false;
      requestAnimationFrame(() => el && (el.scrollTop = el.scrollHeight - prevHeight));
    } catch (e) {
      toast(errorMessage(e, t), 'error');
    } finally {
      setLoadingOlder(false);
    }
  };

  useEffect(() => {
    if (editing) setText(editing.body);
  }, [editing]);

  return (
    <aside className="flex h-full w-full flex-col bg-surface" aria-label={t('chat.title')}>
      <header className="flex h-14 shrink-0 items-center justify-between border-b border-line px-4">
        <h2 className="text-md font-bold text-ink">{t('chat.title')}</h2>
        <IconButton label={t('chat.close')} onClick={onClose}><X size={18} /></IconButton>
      </header>

      <div ref={list} className="flex-1 overflow-y-auto px-3 pb-3 scrollbar-thin" aria-live="polite"
        onScroll={(e) => { const el = e.currentTarget; stickBottom.current = el.scrollHeight - el.scrollTop - el.clientHeight < 80; }}>
        {q.isLoading ? <Loading /> : q.error ? <ErrorState error={q.error} onRetry={() => q.refetch()} /> : (
          <>
            {q.data!.has_more && (
              <div className="flex justify-center pt-3">
                <Button size="sm" variant="ghost" loading={loadingOlder} onClick={loadOlder}>{t('chat.loadOlder')}</Button>
              </div>
            )}
            {items.length === 0 && !visiblePending.length && <EmptyState title={t('chat.empty')} className="py-16" />}
            <ul>
              {items.map((m, i) => {
                const prev = items[i - 1];
                const day = dayKey(m.created_at, me.timezone);
                const newDay = !prev || dayKey(prev.created_at, me.timezone) !== day;
                const grouped = !newDay && prev.author.id === m.author.id && new Date(m.created_at).getTime() - new Date(prev.created_at).getTime() < 5 * 60_000;
                return (
                  <Fragment key={m.id}>
                    {newDay && <DateSeparator iso={m.created_at} />}
                    <MessageItem m={m} me={me} grouped={grouped} onEdit={() => setEditing(m)} onDelete={() => setDeleting(m)} />
                  </Fragment>
                );
              })}
              {visiblePending.map((p) => <PendingItem key={p.client_id} p={p} onRetry={() => send(p.req, p.preview)} />)}
            </ul>
          </>
        )}
      </div>

      <div className="relative shrink-0 border-t border-line p-3">
        {gifOpen && <GifPicker onClose={() => setGifOpen(false)} onSend={sendGif} sending={visiblePending.some((p) => p.kind === 'gif' && p.status === 'sending')} />}
        {editing && (
          <div className="mb-2 flex items-center justify-between rounded-lg bg-primary-soft px-3 py-1.5 text-sm text-primary-text">
            <span>{t('chat.editing')}</span>
            <IconButton size="sm" label={t('chat.cancelEdit')} onClick={() => { setEditing(null); setText(''); }}><X size={14} /></IconButton>
          </div>
        )}
        {uploading && <p className="mb-2 text-sm text-ink-muted" role="status">{t('chat.uploading')}</p>}
        <div className="flex items-end gap-1 rounded-xl border border-line-strong bg-surface p-1.5 focus-within:border-primary focus-within:shadow-focus">
          {!editing && (
            <>
              <IconButton label={t('chat.attach')} onClick={() => fileInput.current?.click()} disabled={uploading}><Paperclip size={18} /></IconButton>
              <input ref={fileInput} type="file" className="hidden" onChange={(e) => onFile(e.target.files?.[0])}
                accept="image/png,image/jpeg,image/gif,image/webp,application/pdf,text/plain,audio/mpeg,video/mp4,.docx,.xlsx,.pptx" />
              <IconButton label={t('chat.reactions')} active={gifOpen} onClick={() => setGifOpen((o) => !o)}><Smile size={18} /></IconButton>
            </>
          )}
          <textarea value={text} onChange={(e) => setText(e.target.value)} onKeyDown={onKey} rows={1} maxLength={4000}
            placeholder={editing && editing.kind !== 'text' ? t('chat.captionPlaceholder') : t('chat.placeholder')} aria-label={t('chat.placeholder')}
            className="max-h-32 min-h-[36px] min-w-0 flex-1 resize-none bg-transparent px-2 py-2 text-base text-ink placeholder:text-ink-faint focus:outline-none focus-visible:shadow-none"
            style={{ fieldSizing: 'content' } as React.CSSProperties} />
          <IconButton label={editing ? t('common.save') : t('chat.send')} onClick={submit} disabled={edit.isPending || (!text.trim() && (!editing || editing.kind === 'text'))}
            className="bg-primary text-white hover:bg-primary-hover hover:text-white disabled:bg-line disabled:opacity-100"><Send size={16} /></IconButton>
        </div>
      </div>

      <ConfirmDialog open={!!deleting} onClose={() => setDeleting(null)} onConfirm={() => deleting && del.mutate(deleting.id)} loading={del.isPending} tone="danger"
        title={t('chat.deleteConfirmTitle')} text={t('chat.deleteConfirm')} confirmLabel={t('chat.delete')} />
    </aside>
  );

  function DateSeparator({ iso }: { iso: string }) {
    const today = dayKey(new Date().toISOString(), me.timezone);
    const yesterday = dayKey(new Date(Date.now() - 86_400_000).toISOString(), me.timezone);
    const d = dayKey(iso, me.timezone);
    const label = d === today ? t('chat.today') : d === yesterday ? t('chat.yesterday') : formatDay(iso, locale, me.timezone);
    return (
      <li className="my-4 flex items-center gap-3" role="separator" aria-label={label}>
        <span className="h-px flex-1 bg-line" />
        <span className="text-xs font-medium text-ink-muted">{label}</span>
        <span className="h-px flex-1 bg-line" />
      </li>
    );
  }
}

import type { QueryClient } from '@tanstack/react-query';
import type { ChatMessage, RealtimeEvent } from '@/types/api';

export interface ChatPage { items: ChatMessage[]; has_more: boolean }

const key = (cid: string) => ['messages', cid];

/** Insert or replace by id; keeps chronological order; never duplicates. */
export function upsertMessage(qc: QueryClient, cid: string, m: ChatMessage) {
  qc.setQueryData<ChatPage>(key(cid), (old) => {
    if (!old) return old;
    const exists = old.items.some((x) => x.id === m.id);
    const items = exists ? old.items.map((x) => (x.id === m.id ? m : x)) : [...old.items, m];
    items.sort((a, b) => a.created_at.localeCompare(b.created_at) || a.id.localeCompare(b.id));
    return { ...old, items };
  });
}

export function removeMessage(qc: QueryClient, cid: string, id: string) {
  qc.setQueryData<ChatPage>(key(cid), (old) => (old ? { ...old, items: old.items.filter((x) => x.id !== id) } : old));
}

export function applyChatEvent(qc: QueryClient, cid: string, ev: RealtimeEvent) {
  if (ev.type === 'chat.message.created' || ev.type === 'chat.message.updated') upsertMessage(qc, cid, ev.payload as ChatMessage);
  else if (ev.type === 'chat.message.deleted') removeMessage(qc, cid, (ev.payload as { id: string }).id);
}

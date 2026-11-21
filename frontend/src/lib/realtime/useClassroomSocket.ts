'use client';

import { useEffect, useRef, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { wsUrl } from '@/lib/api/client';
import type { RealtimeEvent } from '@/types/api';

export type SocketState = 'connecting' | 'open' | 'reconnecting';

/**
 * Subscribes to classroom events. On every (re)connect all classroom-scoped
 * queries are invalidated, so state is re-read from the API (no lost updates).
 */
export function useClassroomSocket(classroomId: string | undefined, onEvent: (e: RealtimeEvent) => void) {
  const qc = useQueryClient();
  const [state, setState] = useState<SocketState>('connecting');
  const handler = useRef(onEvent);
  handler.current = onEvent;

  useEffect(() => {
    if (!classroomId) return;
    let ws: WebSocket | null = null;
    let attempt = 0;
    let timer: ReturnType<typeof setTimeout> | undefined;
    let closed = false;
    let everOpened = false;
    let watchdog: ReturnType<typeof setTimeout> | undefined;
    // The server sends an event at least every 25 s; silence means a dead link.
    const arm = () => {
      clearTimeout(watchdog);
      watchdog = setTimeout(() => ws?.close(), 60_000);
    };

    const resync = () => {
      for (const key of ['classroom', 'progress', 'lesson-exercises', 'homework', 'messages']) {
        qc.invalidateQueries({ queryKey: [key], predicate: (q) => key === 'classroom' || q.queryKey.includes(classroomId) });
      }
    };

    const connect = () => {
      setState(everOpened ? 'reconnecting' : 'connecting');
      ws = new WebSocket(wsUrl(classroomId));
      ws.onopen = () => {
        attempt = 0;
        arm();
        if (everOpened) resync();
        everOpened = true;
        setState('open');
      };
      ws.onmessage = (msg) => {
        arm();
        try {
          const ev = JSON.parse(msg.data) as RealtimeEvent;
          if (ev.type !== 'hello' && ev.type !== 'ping') handler.current(ev);
        } catch {
          /* ignore malformed */
        }
      };
      ws.onclose = () => {
        clearTimeout(watchdog);
        if (closed) return;
        setState('reconnecting');
        const delay = Math.min(10_000, 500 * 2 ** attempt++);
        timer = setTimeout(connect, delay);
      };
    };
    connect();

    const onOnline = () => {
      if (ws && ws.readyState !== WebSocket.OPEN) {
        clearTimeout(timer);
        ws.close();
        connect();
      }
    };
    const onOffline = () => ws?.close();
    window.addEventListener('online', onOnline);
    window.addEventListener('offline', onOffline);
    return () => {
      closed = true;
      clearTimeout(timer);
      clearTimeout(watchdog);
      window.removeEventListener('online', onOnline);
      window.removeEventListener('offline', onOffline);
      ws?.close();
    };
  }, [classroomId, qc]);

  return state;
}

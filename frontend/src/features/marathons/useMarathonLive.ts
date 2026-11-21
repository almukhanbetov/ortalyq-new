'use client';

import { useCallback } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { qk } from '@/lib/queryKeys';
import { useClassroomSocket } from '@/lib/realtime/useClassroomSocket';
import type { Classroom, RealtimeEvent } from '@/types/api';
import { useServerMoment } from './marathonTime';

/**
 * Keeps a marathon page current: realtime notifications re-read lessons, students and the header,
 * and when the server clock reaches the end date everything is re-read so the "finished" state comes
 * from the server, not from the browser clock.
 */
export function useMarathonLive(c: Classroom | undefined) {
  const qc = useQueryClient();
  const id = c?.id;
  const refresh = useCallback(() => {
    if (!id) return;
    qc.invalidateQueries({ queryKey: qk.classroom(id) });
    qc.invalidateQueries({ queryKey: ['marathon', id] });
    qc.invalidateQueries({ queryKey: ['progress', id] });
    qc.invalidateQueries({ queryKey: ['lesson-exercises', id] });
  }, [qc, id]);
  const onEvent = useCallback(
    (ev: RealtimeEvent) => {
      if (ev.type === 'marathon.changed' || ev.type === 'attempt.updated' || ev.type === 'materials.changed') refresh();
    },
    [refresh],
  );
  useClassroomSocket(c?.lesson_type === 'marathon' ? id : undefined, onEvent);
  const m = c?.marathon;
  useServerMoment(m && !m.finished ? m.end_at : null, m?.server_now, refresh);
  // At the delayed start the server opens the lessons; re-read so the page follows its decision.
  useServerMoment(m && !m.started ? m.start_at : null, m?.server_now, refresh);
}

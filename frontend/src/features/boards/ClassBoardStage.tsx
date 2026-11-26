'use client';

import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, Eye, PenLine, Pencil, UserCheck, UserX } from 'lucide-react';
import { Badge, Button, Select, cn, useToast } from '@/components/ui';
import { classBoardApi } from '@/lib/api/endpoints';
import { ApiError } from '@/lib/api/client';
import { qk } from '@/lib/queryKeys';
import { useI18n } from '@/lib/i18n';
import { errorMessage } from '@/lib/errors';
import type { BoardItem, ClassBoardView, Member } from '@/types/api';
import { BoardViewer } from './BoardItems';
import { BoardWorkspace } from './BoardEditor';
import { useCloseInClass } from './BoardClassControls';
import { INKS, PEN_WIDTH } from './model';

/**
 * The board open in the lesson. The teacher (its owner) draws with every tool, saved by itself, and lets a chosen
 * student draw. A student sees the saved board (re-read after every change; events only notify) and, with the
 * teacher's permission, draws pen strokes: each stroke is sent as its own action with its own id.
 */
export function ClassBoardStage({ classroomId, view, teacher, members, onBack }: {
  classroomId: string; view: ClassBoardView; teacher: boolean; members: Member[]; onBack: () => void;
}) {
  const { t } = useI18n();
  const close = useCloseInClass(view.board!.id);
  const board = view.board!;
  return (
    <div className="mx-auto max-w-[1180px]">
      <Button variant="ghost" size="sm" className="mb-3" icon={<ArrowLeft size={16} />} onClick={onBack}>{t('boards.backToLesson')}</Button>
      {teacher ? (
        <>
          <DrawersPanel classroomId={classroomId} drawers={view.drawers ?? []} students={members.filter((m) => m.role === 'student')} />
          <BoardWorkspace key={board.id} board={board} variant="class" closing={close.isPending} onCloseInClass={() => close.mutate(classroomId)} />
        </>
      ) : (
        <StudentBoard classroomId={classroomId} view={view} />
      )}
    </div>
  );
}

/** «Разрешить рисовать»: the teacher picks a student of the class; «Отозвать разрешение» returns them to view only. */
function DrawersPanel({ classroomId, drawers, students }: { classroomId: string; drawers: string[]; students: Member[] }) {
  const { t } = useI18n();
  const toast = useToast();
  const qc = useQueryClient();
  const viewers = students.filter((s) => !drawers.includes(s.user_id));
  const [picked, setPicked] = useState('');
  const choice = viewers.some((s) => s.user_id === picked) ? picked : viewers[0]?.user_id ?? '';
  const set = useMutation({
    mutationFn: ({ uid, allow }: { uid: string; allow: boolean }) => (allow ? classBoardApi.allowDrawing(classroomId, uid) : classBoardApi.revokeDrawing(classroomId, uid)),
    onSuccess: (v) => qc.setQueryData(qk.classBoard(classroomId), v),
    onError: (e) => toast(errorMessage(e, t), 'error'),
  });
  const name = (uid: string) => students.find((s) => s.user_id === uid)?.full_name ?? '';
  return (
    <section aria-label={t('boards.drawersTitle')} className="mb-4 flex flex-wrap items-center gap-3 rounded-xl border border-line bg-surface-alt px-4 py-3">
      <span className="inline-flex items-center gap-2 text-sm font-medium text-ink-body"><PenLine size={16} />{t('boards.drawersTitle')}</span>
      {drawers.map((uid) => (
        <span key={uid} className="inline-flex items-center gap-2 rounded-lg bg-success-soft px-2.5 py-1 text-sm text-success-text" data-testid="board-drawer">
          <UserCheck size={14} />{t('boards.drawerCanDraw', { name: name(uid) })}
          <Button size="sm" variant="secondary" icon={<UserX size={14} />} loading={set.isPending && set.variables?.uid === uid}
            onClick={() => set.mutate({ uid, allow: false })}>{t('boards.revokeDrawing')}</Button>
        </span>
      ))}
      {viewers.length > 0 ? (
        <span className="inline-flex flex-wrap items-center gap-2">
          <label className="sr-only" htmlFor="board-drawer-pick">{t('boards.drawerStudent')}</label>
          <Select id="board-drawer-pick" value={choice} onChange={(e) => setPicked(e.target.value)} className="h-9 min-w-[200px]">
            {viewers.map((s) => <option key={s.user_id} value={s.user_id}>{s.full_name}</option>)}
          </Select>
          <Button size="sm" icon={<Pencil size={14} />} disabled={!choice} loading={set.isPending && set.variables?.allow === true}
            onClick={() => set.mutate({ uid: choice, allow: true })}>{t('boards.allowDrawing')}</Button>
        </span>
      ) : drawers.length === 0 && <span className="text-sm text-ink-muted">{t('boards.noStudents')}</span>}
      <span className="basis-full text-xs text-ink-muted">{t('boards.drawersHint')}</span>
    </section>
  );
}

const uuid = () => (typeof crypto !== 'undefined' && 'randomUUID' in crypto ? crypto.randomUUID()
  : '10000000-1000-4000-8000-100000000000'.replace(/[018]/g, (c) => (Number(c) ^ (Math.random() * 16) >> (Number(c) / 4)).toString(16)));

/** The student's board: view only, or the pen and the colours when the teacher allowed drawing. */
function StudentBoard({ classroomId, view }: { classroomId: string; view: ClassBoardView }) {
  const { t } = useI18n();
  const toast = useToast();
  const qc = useQueryClient();
  const board = view.board!;
  const canDraw = !!view.can_draw;
  const [ink, setInk] = useState<string>(INKS[0].value);
  // Strokes sent but not yet in the board read from the server: shown at once, dropped when the board has them.
  const [pending, setPending] = useState<(BoardItem & { id: string })[]>([]);
  const saved = board.content.items;
  useEffect(() => {
    const ids = new Set(saved.map((x) => x.id));
    setPending((ps) => (ps.some((p) => ids.has(p.id)) ? ps.filter((p) => !ids.has(p.id)) : ps));
  }, [saved]);

  const send = useMutation({
    mutationFn: (body: { board_id: string; client_id: string; color: string; width: number; points: [number, number][] }) => classBoardApi.stroke(classroomId, body),
    // A lost connection resends the same stroke (same client_id): the server stores it once.
    retry: (n, e) => !(e instanceof ApiError && e.status < 500) && n < 3,
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.classBoard(classroomId) }),
    onError: (e, body) => {
      setPending((ps) => ps.filter((p) => p.id !== `s-${body.client_id}`));
      toast(e instanceof ApiError && e.status === 403 ? t('boards.drawingRevoked') : errorMessage(e, t), 'error');
      qc.invalidateQueries({ queryKey: qk.classBoard(classroomId) });
    },
  });
  const items = useMemo(() => [...saved, ...pending], [saved, pending]);
  const onStroke = (points: [number, number][]) => {
    const client = uuid();
    setPending((ps) => [...ps, { id: `s-${client}`, type: 'stroke', color: ink, width: PEN_WIDTH, points }]);
    send.mutate({ board_id: board.id, client_id: client, color: ink, width: PEN_WIDTH, points });
  };

  return (
    <section aria-label={board.title} data-testid="class-board-student">
      <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
        <h2 className="truncate text-xl font-bold text-ink-body">{t('boards.inClassTitle', { title: board.title })}</h2>
        {canDraw
          ? <Badge tone="success" icon={<Pencil size={12} />}>{t('boards.canDraw')}</Badge>
          : <Badge tone="info" icon={<Eye size={12} />}>{t('boards.viewOnly')}</Badge>}
      </div>
      {canDraw && (
        <div role="toolbar" aria-label={t('boards.tools')} data-testid="student-board-toolbar" className="mb-3 flex flex-wrap items-center gap-1.5 rounded-xl border border-line bg-surface-alt p-2">
          <span aria-pressed="true" role="button" aria-label={t('boards.pen')} title={t('boards.pen')}
            className="inline-flex h-9 w-9 items-center justify-center rounded-lg bg-primary text-white"><Pencil size={18} /></span>
          <span className="mx-0.5 h-6 w-px bg-line" aria-hidden />
          <div role="radiogroup" aria-label={t('boards.color')} className="flex items-center gap-1">
            {INKS.map((c) => (
              <button key={c.value} type="button" role="radio" aria-checked={ink === c.value} aria-label={t(`boards.colors.${c.name}`)} title={t(`boards.colors.${c.name}`)}
                onClick={() => setInk(c.value)}
                className={cn('h-7 w-7 rounded-full border-2 transition-transform focus-visible:outline-none focus-visible:shadow-focus',
                  ink === c.value ? 'scale-110 border-ink' : 'border-surface hover:scale-105')}
                style={{ backgroundColor: c.value }} />
            ))}
          </div>
        </div>
      )}
      <BoardViewer items={items} imageUrl={(imageId) => classBoardApi.imageUrl(classroomId, imageId)} label={t('boards.canvas')}
        pen={canDraw ? { color: ink, width: PEN_WIDTH, onStroke } : undefined} />
      <p className="mt-2 text-xs text-ink-muted">{canDraw ? t('boards.canDrawHint') : t('boards.viewOnlyHint')}</p>
    </section>
  );
}

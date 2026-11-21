'use client';

import { useState, type ReactNode } from 'react';
import Link from 'next/link';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { CheckCircle2, ClipboardList, History, Hourglass, RotateCcw, Send } from 'lucide-react';
import { Button, ConfirmDialog, EmptyState, Modal, Notice, QueryState, RadioCard, useToast, type Tone } from '@/components/ui';
import { attemptsApi, homeworkApi } from '@/lib/api/endpoints';
import { qk } from '@/lib/queryKeys';
import { formatDateTime } from '@/lib/format';
import { useI18n } from '@/lib/i18n';
import { errorMessage } from '@/lib/errors';
import type { ExerciseView, HomeworkStatus, HomeworkView, Me, SnapshotExercise } from '@/types/api';
import { TestPlayer } from '@/features/exercises/TestPlayer';

const STATUS_TONE: Record<HomeworkStatus, Tone> = { not_submitted: 'info', pending: 'warning', revision: 'pink', done: 'success' };
const STATUS_ICON: Record<HomeworkStatus, ReactNode> = {
  not_submitted: <ClipboardList size={20} />,
  pending: <Hourglass size={20} />,
  revision: <RotateCcw size={20} />,
  done: <CheckCircle2 size={20} />,
};

export function HomeworkStatusPill({ status }: { status: HomeworkStatus }) {
  const { t } = useI18n();
  const cls = { not_submitted: 'bg-info-soft text-info-text', pending: 'bg-warning-soft text-warning-text', revision: 'bg-pink-soft text-pink-text', done: 'bg-success-soft text-success-text' }[status];
  return <span className={`inline-flex h-6 items-center gap-1 rounded-full px-2.5 text-xs font-medium ${cls}`}>{t(`homework.status.${status}`)}</span>;
}

type Dialog = null | 'submit' | 'revision' | 'complete' | 'change';

export function HomeworkPanel({ classroomId, lessonId, role, studentId, studentName, me }: {
  classroomId: string;
  lessonId: string;
  role: 'teacher' | 'student';
  studentId: string;
  studentName: string;
  me: Me;
}) {
  const { t, locale } = useI18n();
  const qc = useQueryClient();
  const toast = useToast();
  const teacher = role === 'teacher';
  const sid = teacher ? studentId : undefined;
  const hw = useQuery({ queryKey: qk.homework(classroomId, lessonId, sid), queryFn: () => homeworkApi.get(classroomId, lessonId, sid) });
  const ex = useQuery({ queryKey: qk.lessonExercises(classroomId, lessonId, 'homework', sid), queryFn: () => attemptsApi.lessonExercises(classroomId, lessonId, 'homework', sid) });
  const [dialog, setDialog] = useState<Dialog>(null);
  const [newStatus, setNewStatus] = useState<HomeworkStatus>('revision');

  const after = (v: HomeworkView) => {
    qc.setQueryData(qk.homework(classroomId, lessonId, sid), v);
    qc.invalidateQueries({ queryKey: ['lesson-exercises', classroomId, lessonId, 'homework'] });
    qc.invalidateQueries({ queryKey: ['classrooms'] });
    setDialog(null);
  };
  const onError = (e: unknown) => {
    toast(errorMessage(e, t), 'error');
    hw.refetch();
  };
  const submit = useMutation({ mutationFn: () => homeworkApi.submit(classroomId, lessonId), onSuccess: after, onError });
  const setStatus = useMutation({
    mutationFn: (to: HomeworkStatus) => homeworkApi.setStatus(classroomId, lessonId, { student_id: studentId, to_status: to, from_status: hw.data?.status }),
    onSuccess: after,
    onError,
  });

  return (
    <QueryState query={hw}>
      {(h) => {
        const status = h.status;
        const visible = (ex.data?.items ?? []).filter((e) => teacher || !e.hidden);
        if (!h.has_exercises && !visible.length) {
          return (
            <EmptyState title={teacher ? t('homework.emptyTeacher') : t('homework.empty')}
              action={teacher ? <Link href={`/materials/lessons/${lessonId}`} className="text-base font-medium text-primary-text underline">{t('homework.addInEditor')}</Link> : undefined} />
          );
        }
        const actions: ReactNode = teacher ? (
          <div className="flex flex-wrap gap-2">
            {status === 'pending' && <Button variant="warning" onClick={() => setDialog('revision')}>{t('homework.sendToRevision')}</Button>}
            {(status === 'pending' || status === 'revision' || status === 'not_submitted') && <Button variant="success" onClick={() => setDialog('complete')}>{t('homework.complete')}</Button>}
            {status === 'done' && <Button variant="secondary" onClick={() => { setNewStatus('revision'); setDialog('change'); }}>{t('homework.changeStatus')}</Button>}
          </div>
        ) : status === 'not_submitted' || status === 'revision' ? (
          <Button icon={<Send size={16} />} onClick={() => setDialog('submit')}>{status === 'revision' ? t('homework.resubmit') : t('homework.submit')}</Button>
        ) : null;

        const snap = teacher && h.latest_submission && status !== 'not_submitted' ? h.latest_submission : null;
        return (
          <div className="flex flex-col gap-4">
            <Notice tone={STATUS_TONE[status]} icon={STATUS_ICON[status]} action={actions}>
              <span className="block">{t(`homework.${teacher ? 'teacher' : 'student'}.${status}`)}</span>
              {teacher && <span className="mt-0.5 block text-sm font-normal opacity-80">{studentName} · {t('homework.statusLabel')}: {t(`homework.status.${status}`)}</span>}
            </Notice>

            {snap && (
              <p className="text-sm text-ink-muted">
                {t('homework.snapshotNote', { n: snap.number })} · {t('homework.submission', { n: snap.number, date: formatDateTime(snap.submitted_at, locale, me.timezone) })}
              </p>
            )}

            <QueryState query={ex}>
              {() =>
                snap ? (
                  <div className="flex flex-col gap-4">
                    {snap.snapshot.map((s, i) => (
                      <TestPlayer key={s.exercise_id} classroomId={classroomId} view={snapshotView(s)} role="teacher" studentId={studentId} studentName={studentName} index={i} snapshot />
                    ))}
                  </div>
                ) : (
                  <div className="flex flex-col gap-4">
                    {visible.map((v, i) => (
                      <TestPlayer key={v.exercise_id} classroomId={classroomId} view={v} role={role} studentId={studentId} studentName={studentName} index={i} />
                    ))}
                  </div>
                )
              }
            </QueryState>

            {h.events.length > 0 && (
              <details className="rounded-xl border border-line bg-surface p-4">
                <summary className="flex cursor-pointer items-center gap-2 text-base font-medium text-ink"><History size={16} />{t('homework.history')}</summary>
                <ol className="mt-3 flex flex-col gap-2 text-sm">
                  {h.events.map((e, i) => (
                    <li key={i} className="flex flex-wrap items-center gap-2 text-ink-body">
                      <span className="text-ink-muted">{formatDateTime(e.created_at, locale, me.timezone)}</span>
                      <span className="font-medium text-ink">{e.actor_name}</span>
                      <HomeworkStatusPill status={e.from_status} /> → <HomeworkStatusPill status={e.to_status} />
                    </li>
                  ))}
                </ol>
              </details>
            )}

            <ConfirmDialog open={dialog === 'submit'} onClose={() => setDialog(null)} onConfirm={() => submit.mutate()} loading={submit.isPending}
              title={t('homework.submitConfirmTitle')} text={t('homework.submitConfirmText')} confirmLabel={t('homework.submit')} />
            <ConfirmDialog open={dialog === 'revision'} onClose={() => setDialog(null)} onConfirm={() => setStatus.mutate('revision')} loading={setStatus.isPending} tone="warning"
              title={t('homework.revisionConfirmTitle')} text={t('homework.revisionConfirmText')} confirmLabel={t('homework.sendToRevision')} />
            <ConfirmDialog open={dialog === 'complete'} onClose={() => setDialog(null)} onConfirm={() => setStatus.mutate('done')} loading={setStatus.isPending} tone="success"
              title={t('homework.completeConfirmTitle')} text={status === 'not_submitted' ? t('homework.completeEarlyText') : t('homework.completeConfirmText')} confirmLabel={t('homework.complete')} />
            <Modal open={dialog === 'change'} onClose={() => setDialog(null)} title={t('homework.changeStatusTitle')} size="sm"
              footer={<>
                <Button variant="secondary" onClick={() => setDialog(null)}>{t('common.cancel')}</Button>
                <Button onClick={() => setStatus.mutate(newStatus)} loading={setStatus.isPending}>{t('homework.apply')}</Button>
              </>}>
              <p className="mb-3 text-base text-ink-body">{t('homework.changeStatusText')}</p>
              <div className="flex flex-col gap-2 pb-2">
                {(['not_submitted', 'revision'] as HomeworkStatus[]).map((s) => (
                  <RadioCard key={s} name="hw-status" value={s} checked={newStatus === s} onChange={() => setNewStatus(s)} title={t(`homework.status.${s}`)} />
                ))}
              </div>
            </Modal>
          </div>
        );
      }}
    </QueryState>
  );
}

/** Adapts a submitted snapshot into a read-only finished exercise view. */
function snapshotView(s: SnapshotExercise): ExerciseView {
  return {
    exercise_id: s.exercise_id,
    scope: 'homework',
    section_id: null,
    kind: 'test',
    position: 0,
    hidden: false,
    locked: true,
    reset_count: 0,
    version: s.version,
    attempt: s.attempt_id
      ? { id: s.attempt_id, status: 'finished', started_at: '', deadline_at: null, finished_at: null, finish_reason: null, server_now: '', answers: s.answers, summary: s.summary, revealed: true }
      : null,
  };
}

'use client';

import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { CheckCircle2, Pencil, RotateCcw, Shuffle } from 'lucide-react';
import { Button, ConfirmDialog, IconButton, ProgressBar, cn, useToast } from '@/components/ui';
import { gradingApi } from '@/lib/api/endpoints';
import { formatDateTime } from '@/lib/format';
import { useI18n } from '@/lib/i18n';
import { errorMessage } from '@/lib/errors';
import type { Grade, Me, Points, Progress } from '@/types/api';
import { GradeFace } from './GradeFace';

const GRADES: Grade[] = ['fine', 'good', 'amazing'];

export function LessonFinish({ classroomId, progress, role, me, onOtherLesson }: {
  classroomId: string;
  progress: Progress;
  role: 'teacher' | 'student';
  me: Me;
  onOtherLesson: () => void;
}) {
  const { t, locale } = useI18n();
  const qc = useQueryClient();
  const toast = useToast();
  const c = progress.completion;
  const [choice, setChoice] = useState<Grade | null>(null);
  const [editing, setEditing] = useState(false);
  const [confirmUndo, setConfirmUndo] = useState(false);
  const teacher = role === 'teacher';
  const refresh = () => qc.invalidateQueries({ queryKey: ['progress', classroomId] });
  const onError = (e: unknown) => toast(errorMessage(e, t), 'error');

  const complete = useMutation({ mutationFn: (g: Grade) => gradingApi.setCompletion(classroomId, progress.lesson_id, true, g), onSuccess: () => { refresh(); setChoice(null); }, onError });
  const setGrade = useMutation({ mutationFn: (g: Grade) => gradingApi.setGrade(classroomId, progress.lesson_id, g), onSuccess: () => { refresh(); setEditing(false); }, onError });
  const undo = useMutation({ mutationFn: () => gradingApi.setCompletion(classroomId, progress.lesson_id, false), onSuccess: () => { refresh(); setConfirmUndo(false); }, onError });

  const picker = (value: Grade | null, onPick: (g: Grade) => void, disabled?: boolean) => (
    <div role="radiogroup" aria-label={t('grading.selectGrade')} className="grid grid-cols-3 gap-3">
      {GRADES.map((g) => (
        <button key={g} type="button" role="radio" aria-checked={value === g} disabled={disabled} onClick={() => onPick(g)}
          className={cn('flex flex-col items-center gap-2 rounded-2xl border-2 px-2 py-4 transition-all disabled:cursor-not-allowed',
            value === g ? 'border-primary bg-primary-soft' : 'border-line hover:border-line-strong hover:bg-surface-alt')}>
          <GradeFace grade={g} />
          <span className={cn('text-base font-medium', value === g ? 'text-primary-text' : 'text-ink')}>{t(`grading.grades.${g}`)}</span>
        </button>
      ))}
    </div>
  );

  return (
    <div className="mx-auto flex w-full max-w-[720px] flex-col gap-5">
      <section className="rounded-2xl border border-line bg-surface p-6 text-center shadow-card sm:p-8">
        {c.completed && c.grade ? (
          <>
            <span className="mx-auto mb-3 inline-flex items-center gap-2 rounded-full bg-success-soft px-3 py-1 text-sm font-medium text-success-text"><CheckCircle2 size={16} /> {t('grading.completed')}</span>
            <h2 className="text-xl font-bold text-ink">{progress.lesson_title}</h2>
            <p className="mt-1 text-sm text-ink-muted">{t('grading.completedBy', { name: c.completed_by_name, date: formatDateTime(c.completed_at!, locale, me.timezone) })}</p>
            {editing ? (
              <div className="mt-6">
                {picker(c.grade, (g) => setGrade.mutate(g), setGrade.isPending)}
                <Button variant="ghost" className="mt-3" onClick={() => setEditing(false)}>{t('common.cancel')}</Button>
              </div>
            ) : (
              <div className="mt-6 flex flex-col items-center gap-2">
                <p className="text-sm text-ink-muted">{t('grading.yourGrade')}</p>
                <div className="flex items-center gap-2">
                  <GradeFace grade={c.grade} size={72} />
                  {teacher && <IconButton label={t('grading.changeGrade')} onClick={() => setEditing(true)}><Pencil size={18} /></IconButton>}
                </div>
                <p className="text-lg font-bold text-ink">{t(`grading.grades.${c.grade}`)}</p>
              </div>
            )}
            {teacher && (
              <div className="mt-6 flex flex-wrap justify-center gap-2">
                <Button icon={<Shuffle size={16} />} onClick={onOtherLesson}>{t('classroom.otherLesson')}</Button>
                <Button variant="secondary" icon={<RotateCcw size={16} />} onClick={() => setConfirmUndo(true)}>{t('grading.uncomplete')}</Button>
              </div>
            )}
          </>
        ) : teacher ? (
          <>
            <h2 className="text-xl font-bold text-ink">{t('grading.title')}</h2>
            <p className="mb-6 mt-1 text-base text-ink-muted">{t('grading.rate')}</p>
            {picker(choice, setChoice)}
            <Button size="lg" className="mt-6" disabled={!choice} loading={complete.isPending} onClick={() => choice && complete.mutate(choice)}>{t('grading.complete')}</Button>
          </>
        ) : (
          <>
            <h2 className="text-xl font-bold text-ink">{t('grading.title')}</h2>
            <p className="mt-2 text-base text-ink-muted">{t('grading.notCompletedStudent')}</p>
          </>
        )}
      </section>

      <div className="grid gap-4 sm:grid-cols-2">
        <PointsCard title={t('grading.lessonPoints')} points={progress.lesson_points} />
        <PointsCard title={t('grading.homeworkPoints')} points={progress.homework_points} />
      </div>

      <ConfirmDialog open={confirmUndo} onClose={() => setConfirmUndo(false)} onConfirm={() => undo.mutate()} loading={undo.isPending} tone="danger"
        title={t('grading.uncompleteConfirmTitle')} text={t('grading.uncompleteConfirm')} confirmLabel={t('grading.uncomplete')} />
    </div>
  );
}

function PointsCard({ title, points }: { title: string; points: Points }) {
  const { t } = useI18n();
  return (
    <section className="rounded-2xl border border-line bg-surface p-5 shadow-card" aria-label={title}>
      <div className="mb-2 flex items-baseline justify-between gap-2">
        <h3 className="text-md font-bold text-ink">{title}</h3>
        <span className="text-sm text-ink-muted">{t('grading.exercisesDone', { a: points.finished, b: points.exercises })}</span>
      </div>
      {points.max === 0 ? (
        <p className="text-sm text-ink-muted">{t('grading.noScoring')}</p>
      ) : (
        <>
          <p className="mb-2 text-2xl font-bold text-ink">{t('grading.earnedOf', { a: points.earned, b: points.max })} <span className="text-base font-normal text-ink-muted">{t('grading.points').toLowerCase()}</span></p>
          <ProgressBar value={points.earned} max={points.max} label={t('grading.progress')} />
          <h4 className="mb-2 mt-4 text-sm font-medium text-ink-body">{t('grading.categories')}</h4>
          <ul className="flex flex-col gap-2">
            {points.categories.map((c) => (
              <li key={c.category_id ?? 'none'} className="flex items-center justify-between gap-2 text-sm">
                <span className="text-ink">{c.category_name || '—'}</span>
                <span className="font-medium text-ink">{c.earned}/{c.max}</span>
              </li>
            ))}
          </ul>
        </>
      )}
    </section>
  );
}

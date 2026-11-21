'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, CircleCheck, Eye, Hourglass, Info, Mail, RotateCcw, Undo2 } from 'lucide-react';
import { Avatar, Button, ConfirmDialog, EmptyState, Field, Modal, Notice, QueryState, Textarea, cn, useToast } from '@/components/ui';
import { PageCard } from '@/components/layout/PageCard';
import { TestPlayer } from '@/features/exercises/TestPlayer';
import { marathonsApi } from '@/lib/api/endpoints';
import { qk } from '@/lib/queryKeys';
import { useI18n, useT } from '@/lib/i18n';
import { errorMessage } from '@/lib/errors';
import { marathonStudentUrl } from '@/lib/routes';
import type { Classroom, MarathonLessonReview as Review, MarathonRating, MarathonReviewRequest } from '@/types/api';
import { LessonStatusChip, MarathonDatesBanner, RATINGS, RatingView } from './MarathonParts';
import { formatMarathonMoment } from './marathonTime';

/**
 * Owner's read-only view of one student's lesson: saved answers with the key, results and points.
 * The endpoint never writes, and the players run in snapshot mode (no reset, no live actions),
 * so it works the same after the marathon has ended.
 */
export function MarathonLessonReview({ marathon, lessonId, studentId }: { marathon: Classroom; lessonId: string; studentId: string }) {
  const t = useT();
  const id = marathon.id;
  const review = useQuery({ queryKey: qk.marathonReview(id, studentId, lessonId), queryFn: () => marathonsApi.reviewLesson(id, studentId, lessonId) });
  return (
    <PageCard maxWidth={1024} className="pt-[18px] sm:pt-8">
      <Link href={marathonStudentUrl(id, studentId)} className="inline-flex items-center gap-2 text-sm font-medium text-ink-muted hover:text-ink">
        <ArrowLeft size={16} aria-hidden />{t('marathons.backToProgress')} · {marathon.name}
      </Link>
      {marathon.marathon && <MarathonDatesBanner info={marathon.marathon} className="mt-4" />}
      <QueryState query={review}>
        {(r) => (
          <div className="mt-6" data-testid="lesson-review">
            <div className="flex items-center gap-3">
              <Avatar name={r.student.full_name} color={r.student.avatar_color} size={40} />
              <div className="min-w-0">
                <p className="truncate text-base font-medium text-ink-body">{r.student.full_name}</p>
                <p className="flex items-center gap-1.5 text-sm text-ink-soft"><Mail size={14} aria-hidden />{r.student.email}</p>
              </div>
            </div>
            <Notice tone="neutral" icon={<Eye size={20} />} className="mt-4 text-sm">{t('marathons.reviewNotice', { name: r.student.full_name })}</Notice>
            <p className="mt-6 text-sm text-ink-muted">{r.course_title}</p>
            <h1 className="text-[26px] font-bold leading-8 text-ink-body">{r.lesson_title}</h1>
            <div className="mt-2 flex flex-wrap items-center gap-2">
              <span data-testid="review-status" data-status={r.lesson.status}><LessonStatusChip status={r.lesson.status} /></span>
              {r.points.max > 0 && (
                <span className="inline-flex h-6 items-center rounded-md border border-line-soft px-2 text-xs font-medium text-ink-body" data-testid="review-points">
                  {t('marathons.reviewPoints', { a: r.points.earned, b: r.points.max })}
                </span>
              )}
            </div>
            {r.exercises.length === 0 && <EmptyState title={t('marathons.reviewNoExercises')} className="py-10" />}
            {r.sections.map((s, si) => {
              const items = r.exercises.filter((e) => e.section_id === s.id);
              if (!items.length) return null;
              return (
                <section key={s.id} aria-labelledby={`sec-${s.id}`} className="mt-8">
                  <p className="text-xs font-medium uppercase tracking-wide text-ink-muted">{t('classroom.sectionCounter', { a: si + 1, b: r.sections.length })}</p>
                  <h2 id={`sec-${s.id}`} className="mb-4 text-xl font-bold text-ink">{s.title}</h2>
                  <div className="flex flex-col gap-4">
                    {items.map((v, i) => (
                      <div key={v.exercise_id}>
                        {!v.attempt && <p className="mb-2 text-sm text-ink-muted" data-testid="not-started">{t('marathons.notStarted')}</p>}
                        <TestPlayer classroomId={id} view={v} role="teacher" studentId={studentId} studentName={r.student.full_name} index={i} snapshot />
                      </div>
                    ))}
                  </div>
                </section>
              );
            })}
            <ReviewPanel marathon={marathon} review={r} />
          </div>
        )}
      </QueryState>
    </PageCard>
  );
}

/**
 * «Проверка учителем» for the owner, at the end of the lesson (as the original's section=finish): the student's
 * points, the status and the decisions the server allows — «Отправить на доработку» and «Завершить урок» (rating
 * and comment) for a sent lesson, «Отменить завершение» for an accepted one. Every decision is checked again on
 * the server (from_status protects against a stale page); the page shows the state the server returns.
 */
function ReviewPanel({ marathon, review: r }: { marathon: Classroom; review: Review }) {
  const { t, locale } = useI18n();
  const qc = useQueryClient();
  const toast = useToast();
  const id = marathon.id;
  const l = r.lesson;
  const [rating, setRating] = useState<null | { value: MarathonRating | null; comment: string }>(null);
  const [confirm, setConfirm] = useState<null | 'revision' | 'cancel'>(null);
  // The server answered that the decision had already been taken (e.g. accepted in another tab): nothing changed.
  const [unchanged, setUnchanged] = useState<null | { to: MarathonReviewRequest['to_status']; rating: MarathonRating | null }>(null);
  const tz = marathon.marathon?.timezone ?? 'Asia/Almaty';
  const decide = useMutation({
    mutationFn: (body: MarathonReviewRequest) => marathonsApi.reviewDecision(id, r.lesson_id, body),
    onSuccess: (res, body) => {
      qc.setQueryData(qk.marathonReview(id, r.student.id, r.lesson_id), res);
      qc.invalidateQueries({ queryKey: ['marathon', id] });
      qc.invalidateQueries({ queryKey: qk.classroom(id) });
      qc.invalidateQueries({ queryKey: ['classrooms'] });
      setRating(null);
      setConfirm(null);
      if (res.unchanged) {
        // Idempotent repeat: the first decision stays. Say so instead of reporting a success that did not happen.
        setUnchanged({ to: body.to_status, rating: body.rating ?? null });
        toast(t('marathons.reviewUnchangedToast'), 'info');
        return;
      }
      setUnchanged(null);
      toast(t(body.to_status === 'revision' ? 'marathons.revisionToast' : body.to_status === 'completed' ? 'marathons.completedToast' : 'marathons.cancelToast'), 'success');
    },
    onError: (e) => {
      qc.invalidateQueries({ queryKey: qk.marathonReview(id, r.student.id, r.lesson_id) });
      setConfirm(null);
      toast(errorMessage(e, t), 'error');
    },
  });
  const send = (body: Omit<MarathonReviewRequest, 'student_id' | 'from_status'>) => decide.mutate({ ...body, student_id: r.student.id, from_status: l.status });
  const nextClosesOnCancel = marathon.marathon?.mode === 'progressive' && marathon.marathon.teacher_check && marathon.marathon.unlock_after === 'teacher_check';

  return (
    <section className="mt-10 border-t border-line-soft pt-6" aria-labelledby="review-title" data-testid="review-panel" data-status={l.status}>
      <h2 id="review-title" className="mb-3 text-xl font-bold text-ink">{t('marathons.completeLesson')}</h2>
      {!r.teacher_check && (l.status === 'submitted' || l.status === 'revision') && (
        <Notice tone="neutral" icon={<Info size={20} />} className="mb-3 text-sm">{t('marathons.checkOffNotice')}</Notice>
      )}
      {l.status === 'submitted' && (
        <div className="flex flex-col gap-3">
          <Notice tone="warning" icon={<Hourglass size={20} />}>
            <span className="block">{t('marathons.awaitingTitle')}</span>
            <span className="block text-sm font-normal">{t('marathons.awaitingTeacherText')}</span>
          </Notice>
          <div className="flex flex-wrap gap-2">
            <Button variant="secondary" icon={<RotateCcw size={16} />} disabled={decide.isPending} onClick={() => setConfirm('revision')}>{t('marathons.sendToRevision')}</Button>
            <Button icon={<CircleCheck size={16} />} disabled={decide.isPending}
              onClick={() => setRating({ value: l.previous_rating ?? null, comment: l.previous_comment ?? '' })}>{t('marathons.completeLesson')}</Button>
          </div>
        </div>
      )}
      {l.status === 'revision' && (
        <div className="flex flex-col gap-3">
          <Notice tone="danger" icon={<RotateCcw size={20} />}>
            <span className="block">{t('marathons.revisionTitle')}</span>
            <span className="block text-sm font-normal">{t('marathons.revisionTeacherText')}</span>
          </Notice>
          <div>
            <Button icon={<CircleCheck size={16} />} disabled={decide.isPending}
              onClick={() => setRating({ value: l.previous_rating ?? null, comment: l.previous_comment ?? '' })}>{t('marathons.completeLesson')}</Button>
          </div>
        </div>
      )}
      {unchanged && (
        <div data-testid="review-unchanged"><Notice tone="warning" icon={<Info size={20} />} className="mb-3 text-sm">
          {unchanged.to === 'completed' && l.status === 'completed' && l.rating
            ? t(unchanged.rating && unchanged.rating !== l.rating ? 'marathons.reviewUnchangedRating' : 'marathons.reviewUnchangedCompleted',
              { rating: t(`marathons.rating.${l.rating}`), requested: unchanged.rating ? t(`marathons.rating.${unchanged.rating}`) : '' })
            : t('marathons.reviewUnchangedOther')}
        </Notice></div>
      )}
      {l.status === 'completed' && (
        <div className="flex flex-col gap-4">
          <Notice tone="success" icon={<CircleCheck size={20} />}>
            <span className="block">{t('marathons.completedTitle')}</span>
            <span className="block text-sm font-normal">{l.accepted ? t('marathons.completedByTeacher') : t('marathons.selfCompletedText')}</span>
          </Notice>
          {l.accepted && l.rating && <RatingView rating={l.rating} comment={l.teacher_comment} commentLabel={t('marathons.commentToStudent')} />}
          {l.accepted && (
            <div><Button variant="secondary" icon={<Undo2 size={16} />} disabled={decide.isPending} onClick={() => setConfirm('cancel')}>{t('marathons.cancelCompletion')}</Button></div>
          )}
        </div>
      )}
      {l.status === 'none' && <Notice tone="neutral" icon={<Info size={20} />} className="text-sm">{t('marathons.noSubmissionText')}</Notice>}
      {l.submitted_at && (
        <p className="mt-3 text-xs text-ink-soft" data-testid="submitted-at">
          {t('marathons.submittedAt', formatMarathonMoment(l.submitted_at, tz, locale))} · {t('marathons.submissionsCount', { n: r.submissions_count })}
        </p>
      )}

      {r.points.max > 0 && (
        <div className="mt-6 rounded-lg border border-line-soft p-4" data-testid="student-stats">
          <h3 className="text-base font-bold text-ink-body">{t('marathons.studentStats')}</h3>
          <p className="mt-2 flex items-baseline justify-between gap-3 text-sm text-ink-body">
            <span>{t('marathons.earnedPoints')}</span><span className="text-lg font-bold">{r.points.earned} / {r.points.max}</span>
          </p>
          {r.points.categories.map((c) => (
            <div key={c.category_id ?? 'none'} className="mt-3">
              <p className="flex justify-between gap-3 text-xs text-ink-soft"><span>{c.category_name}</span><span>{c.earned} / {c.max}</span></p>
              <div className="mt-1 h-1.5 overflow-hidden rounded-full bg-surface-muted" role="progressbar" aria-label={c.category_name}
                aria-valuemin={0} aria-valuemax={c.max} aria-valuenow={c.earned}>
                <div className="h-full rounded-full bg-success" style={{ width: `${c.max ? Math.round((c.earned / c.max) * 100) : 0}%` }} />
              </div>
            </div>
          ))}
        </div>
      )}

      <Modal open={!!rating} onClose={() => setRating(null)} title={t('marathons.ratingTitle')} size="sm"
        footer={<>
          <Button variant="secondary" onClick={() => setRating(null)} disabled={decide.isPending}>{t('common.cancel')}</Button>
          <Button loading={decide.isPending} disabled={!rating?.value}
            onClick={() => rating?.value && send({ to_status: 'completed', rating: rating.value, comment: rating.comment })}>{t('marathons.rate')}</Button>
        </>}>
        {rating && (
          <div className="flex flex-col gap-4">
            <p className="text-sm text-ink-soft">{t('marathons.ratingHint')}</p>
            <div role="radiogroup" aria-label={t('marathons.ratingLabel')} className="grid grid-cols-3 gap-2">
              {RATINGS.map((x) => {
                const on = rating.value === x.value;
                return (
                  <button key={x.value} type="button" role="radio" aria-checked={on} onClick={() => setRating({ ...rating, value: x.value })}
                    className={cn('flex flex-col items-center gap-1 rounded-lg border px-2 py-3 text-sm font-medium transition-colors focus-visible:shadow-focus focus-visible:outline-none',
                      on ? 'border-primary bg-primary-soft text-primary-text' : 'border-line text-ink-body hover:bg-surface-hover')}>
                    <span className="text-2xl leading-none" aria-hidden>{x.emoji}</span>{t(`marathons.rating.${x.value}`)}
                  </button>
                );
              })}
            </div>
            <Field label={t('marathons.commentToStudent')}>
              {(p) => <Textarea {...p} rows={3} maxLength={2000} value={rating.comment} placeholder={t('marathons.commentPlaceholder')}
                onChange={(e) => setRating({ ...rating, comment: e.target.value })} />}
            </Field>
          </div>
        )}
      </Modal>
      <ConfirmDialog open={confirm === 'revision'} onClose={() => setConfirm(null)} onConfirm={() => send({ to_status: 'revision' })} loading={decide.isPending}
        tone="warning" title={t('marathons.sendToRevision')} text={t('marathons.revisionConfirmText')} confirmLabel={t('marathons.sendToRevision')} />
      <ConfirmDialog open={confirm === 'cancel'} onClose={() => setConfirm(null)} onConfirm={() => send({ to_status: 'submitted' })} loading={decide.isPending}
        tone="warning" title={t('marathons.cancelCompletion')} text={t('marathons.cancelText')} confirmLabel={t('marathons.cancelConfirm')}>
        {nextClosesOnCancel && <Notice tone="info" className="mt-3 text-sm">{t('marathons.cancelNextLocked')}</Notice>}
      </ConfirmDialog>
    </section>
  );
}

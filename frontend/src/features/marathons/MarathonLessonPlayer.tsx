'use client';

import { useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { useRouter, useSearchParams } from 'next/navigation';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, ArrowRight, CircleCheck, Flag, Hourglass, Info, Lock, RotateCcw, Send } from 'lucide-react';
import { Button, ConfirmDialog, EmptyState, ErrorState, Loading, Notice, QueryState, useToast } from '@/components/ui';
import { PageCard } from '@/components/layout/PageCard';
import { TestPlayer } from '@/features/exercises/TestPlayer';
import { attemptsApi, classroomsApi, gradingApi, marathonsApi } from '@/lib/api/endpoints';
import { ApiError } from '@/lib/api/client';
import { qk } from '@/lib/queryKeys';
import { useI18n } from '@/lib/i18n';
import { errorMessage } from '@/lib/errors';
import { marathonLessonUrl, marathonUrl } from '@/lib/routes';
import { useMe } from '@/hooks/useMe';
import { MarathonDatesBanner, RatingView } from './MarathonParts';
import { useServerMoment } from './marathonTime';
import { useMarathonLive } from './useMarathonLive';
import { MarathonLessonReview } from './MarathonLessonReview';
import { StudentLessonResults } from './MarathonStats';
import { lessonLockText } from './MarathonLessons';
import { LookBackground, ReactionBurst, SoundToggle, useAnswerReactions, useLessonLook } from './appearance/LessonLook';
import type { MarathonLessonLock, MarathonLessonStatus, MarathonRating } from '@/types/api';

const isClosed = (e: unknown) => e instanceof ApiError && e.code === 'marathon_closed';
const isDeactivated = (e: unknown) => e instanceof ApiError && e.code === 'marathon_deactivated';
const codeOf = (e: unknown) => (e instanceof ApiError ? e.code : '');
const noRetryOnClosed = (n: number, e: unknown) => !isClosed(e) && !(e instanceof ApiError && e.status < 500) && n < 2;

/**
 * A marathon lesson taken by the student on their own: sections with exercises, then "Завершить урок".
 * After the end date the server rejects every call with marathon_closed; the page then shows the closed state.
 */
export function MarathonLessonPlayer({ id, lessonId }: { id: string; lessonId: string }) {
  const { t, locale } = useI18n();
  const qc = useQueryClient();
  const router = useRouter();
  const toast = useToast();
  const me = useMe().data;
  const reviewStudent = useSearchParams().get('student');
  const cls = useQuery({ queryKey: qk.classroom(id), queryFn: () => classroomsApi.get(id) });
  const c = cls.data;
  useMarathonLive(c);
  const finished = !!c?.marathon?.finished;
  const student = c?.my_role === 'student';
  const enabled = !!c && student && !finished;
  const progress = useQuery({ queryKey: qk.progress(id, lessonId), queryFn: () => gradingApi.progress(id, lessonId), enabled, retry: noRetryOnClosed });
  const exercises = useQuery({ queryKey: qk.lessonExercises(id, lessonId, 'lesson'), queryFn: () => attemptsApi.lessonExercises(id, lessonId, 'lesson'), enabled, retry: noRetryOnClosed });
  // The student's own lessons with access and review status (GET /me): this lesson's status and the next lesson.
  const mine = useQuery({ queryKey: qk.marathonMe(id), queryFn: () => marathonsApi.me(id), enabled: !!c && student, retry: noRetryOnClosed });
  const items = mine.data?.items ?? [];
  const at = items.findIndex((l) => l.lesson_id === lessonId);
  const lesson = at >= 0 ? items[at] : undefined;
  const next = at >= 0 ? items[at + 1] : undefined;
  const [confirmSubmit, setConfirmSubmit] = useState(false);
  // Appearance (stage 10): background of the lesson and reactions to answers checked by the server.
  const look = useLessonLook(id, lessonId, !!c && student);
  const reactions = useAnswerReactions(look);
  const closedNow = isClosed(progress.error) || isClosed(exercises.error);
  // The teacher deactivated the student: the server refuses the lesson; show why instead of a generic error.
  const deactivatedNow = isDeactivated(progress.error) || isDeactivated(exercises.error);
  // Before the delayed start / after the individual access period the server refuses the lesson too.
  const lockCode = [codeOf(progress.error), codeOf(exercises.error)].find((c) => c === 'marathon_not_started' || c === 'marathon_auto_deactivated');
  // The passage mode keeps this lesson closed (opening date, previous lesson, not opened by the teacher).
  const lessonLock = [progress.error, exercises.error].find((e) => e instanceof ApiError && e.code === 'marathon_lesson_locked') as ApiError | undefined;
  // When the lock has a moment (opening date, schedule), the lesson opens by itself at that moment.
  useServerMoment(lessonLock?.fields.opens_at ?? null, c?.marathon?.server_now, () => {
    qc.invalidateQueries({ queryKey: qk.progress(id, lessonId) });
    qc.invalidateQueries({ queryKey: qk.lessonExercises(id, lessonId, 'lesson') });
  });

  useEffect(() => {
    // The teacher edits lessons in materials; marathons other than ours are not here.
    // A teacher sees a student's lesson only through the read-only review (?student=…).
    if (c && (c.lesson_type !== 'marathon' || (c.my_role === 'teacher' && !reviewStudent))) router.replace(marathonUrl(id, 'lessons'));
  }, [c, id, router, reviewStudent]);
  useEffect(() => {
    // The end date passed while the page was open: re-read the marathon so the red banner appears.
    if (closedNow && !finished) qc.invalidateQueries({ queryKey: qk.classroom(id) });
  }, [closedNow, finished, qc, id]);

  const reread = () => {
    qc.invalidateQueries({ queryKey: ['marathon', id] });
    qc.invalidateQueries({ queryKey: qk.lessonExercises(id, lessonId, 'lesson') });
    qc.invalidateQueries({ queryKey: qk.progress(id, lessonId) });
  };
  const onActionError = (e: unknown) => {
    if (isClosed(e)) qc.invalidateQueries({ queryKey: qk.classroom(id) });
    // The settings or the status changed meanwhile (check switched, teacher decided): show the server's state.
    reread();
    toast(errorMessage(e, t), 'error');
  };
  const complete = useMutation({
    mutationFn: () => marathonsApi.completeLesson(id, lessonId),
    onSuccess: () => { reread(); toast(t('marathons.lessonCompleted'), 'success'); },
    onError: onActionError,
  });
  // «Отправить на проверку»: the server finishes the attempts and locks the answers until the teacher decides.
  const submit = useMutation({
    mutationFn: () => marathonsApi.submitLesson(id, lessonId),
    onSuccess: () => { setConfirmSubmit(false); reread(); toast(t('marathons.submittedToast'), 'success'); },
    onError: (e) => { setConfirmSubmit(false); onActionError(e); },
  });

  if (cls.isLoading) return <PageCard maxWidth={1024}><Loading /></PageCard>;
  if (cls.error || !c) return <PageCard maxWidth={1024}><ErrorState error={cls.error} onRetry={() => cls.refetch()} /></PageCard>;
  if (c.my_role === 'teacher' && reviewStudent && c.lesson_type === 'marathon') {
    return <MarathonLessonReview marathon={c} lessonId={lessonId} studentId={reviewStudent} />;
  }
  if (!student || !c.marathon) return null;

  return (
    <PageCard maxWidth={1024} className="relative isolate overflow-clip pt-[18px] sm:pt-8">
      <LookBackground look={look} />
      <ReactionBurst burst={reactions.burst} onDone={reactions.clear} />
      <div className="flex flex-wrap items-center justify-between gap-3">
        <Link href={marathonUrl(id)} className="inline-flex items-center gap-2 text-sm font-medium text-ink-muted hover:text-ink">
          <ArrowLeft size={16} aria-hidden />{t('marathons.backToMarathon')} · {c.name}
        </Link>
        {reactions.hasSound && <SoundToggle on={reactions.soundOn} onChange={reactions.setSoundOn} />}
      </div>
      <MarathonDatesBanner info={c.marathon} className="mt-4" />
      {lessonLock ? (
        <EmptyState icon={<Lock size={26} />} className="min-h-[320px]" title={t('marathons.lessonLockedTitle')}
          text={lessonLockText(t, { open: false, reason: lessonLock.fields.reason as MarathonLessonLock['reason'], opens_at: lessonLock.fields.opens_at }, c.marathon?.timezone ?? 'Asia/Almaty', locale)}
          action={<Link href={marathonUrl(id)} className="text-sm font-medium text-primary-text hover:underline">{t('marathons.backToMarathon')}</Link>} />
      ) : lockCode ? (
        <EmptyState icon={<Lock size={26} />} className="min-h-[320px]"
          title={t(lockCode === 'marathon_not_started' ? 'marathons.notStartedLesson' : 'marathons.autoExpiredLesson')}
          action={<Link href={marathonUrl(id)} className="text-sm font-medium text-primary-text hover:underline">{t('marathons.backToMarathon')}</Link>} />
      ) : deactivatedNow ? (
        <EmptyState icon={<Lock size={26} />} title={t('marathons.deactivatedStudentTitle')} text={t('marathons.deactivatedStudentText')} className="min-h-[320px]"
          action={<Link href={marathonUrl(id)} className="text-sm font-medium text-primary-text hover:underline">{t('marathons.backToMarathon')}</Link>} />
      ) : finished || closedNow ? (
        <EmptyState icon={<Lock size={26} />} title={t('marathons.closedLesson')} className="min-h-[320px]"
          action={<Link href={marathonUrl(id)} className="text-sm font-medium text-primary-text hover:underline">{t('marathons.backToMarathon')}</Link>} />
      ) : (
        <QueryState query={progress}>
          {(p) => (
            <div className="mt-6">
              <p className="text-sm text-ink-muted">{p.course_title}</p>
              <h1 className="text-[26px] font-bold leading-8 text-ink-body">{p.lesson_title}</h1>
              {p.sections.length === 0 && <EmptyState title={t('classroom.noSections')} className="py-10" />}
              {p.sections.map((s, si) => (
                <section key={s.id} aria-labelledby={`sec-${s.id}`} className="mt-8">
                  <p className="text-xs font-medium uppercase tracking-wide text-ink-muted">{t('classroom.sectionCounter', { a: si + 1, b: p.sections.length })}</p>
                  <h2 id={`sec-${s.id}`} className="mb-4 text-xl font-bold text-ink">{s.title}</h2>
                  <QueryState query={exercises}>
                    {(data) => {
                      const items = data.items.filter((e) => e.section_id === s.id);
                      if (!items.length) return <EmptyState title={t('classroom.sectionEmpty')} className="rounded-2xl border border-dashed border-line-strong bg-surface" />;
                      return (
                        <div className="flex flex-col gap-4">
                          {items.map((v, i) => (
                            <TestPlayer key={v.exercise_id} classroomId={id} view={v} role="student" studentId={me?.id ?? ''} studentName={me?.full_name ?? ''} index={i} onChecked={reactions.react} />
                          ))}
                        </div>
                      );
                    }}
                  </QueryState>
                </section>
              ))}
              <div className="mt-10 border-t border-line-soft pt-6" data-testid="lesson-finish">
                <h2 className="mb-3 text-xl font-bold text-ink">{t('marathons.completeLesson')}</h2>
                {lesson && (
                  <LessonFinish status={lesson.status} accepted={lesson.accepted} rating={lesson.rating} comment={lesson.teacher_comment}
                    teacherCheck={!!c.marathon?.teacher_check} busy={complete.isPending || submit.isPending}
                    onComplete={() => complete.mutate()} onSubmit={() => setConfirmSubmit(true)}
                    next={next ? (next.access === 'open'
                      ? <Link href={marathonLessonUrl(id, next.lesson_id)} className={linkButton} data-testid="next-lesson"><ArrowRight size={18} aria-hidden />{t('marathons.nextLesson')}</Link>
                      : null)
                      : <Link href={marathonUrl(id)} className={linkButton} data-testid="next-lesson"><ArrowRight size={18} aria-hidden />{t('marathons.backToMarathon')}</Link>} />
                )}
              </div>
              {c.marathon?.stats_enabled && <StudentLessonResults marathonId={id} lessonId={lessonId} />}
            </div>
          )}
        </QueryState>
      )}
      <ConfirmDialog open={confirmSubmit} onClose={() => setConfirmSubmit(false)} onConfirm={() => submit.mutate()} loading={submit.isPending}
        title={t('marathons.submitTitle')} text={t('marathons.submitText')} confirmLabel={t('marathons.submitConfirm')} />
    </PageCard>
  );
}

const linkButton = 'inline-flex h-10 items-center gap-2 rounded-md bg-primary px-4 text-base font-medium text-white transition-colors hover:bg-primary-hover focus-visible:shadow-focus focus-visible:outline-none';

/**
 * End of a marathon lesson for the student. Without the teacher check: «Завершить урок» (stage 4).
 * With it: «Отправить на проверку» → «Урок ожидает проверку» (answers locked) → returned for revision (send again)
 * or «Урок завершён» with the teacher's rating and comment and «Следующий урок». Every state comes from the server.
 */
function LessonFinish({ status, accepted, rating, comment, teacherCheck, busy, onComplete, onSubmit, next }: {
  status: MarathonLessonStatus; accepted: boolean; rating: MarathonRating | null; comment: string | null;
  teacherCheck: boolean; busy: boolean; onComplete: () => void; onSubmit: () => void; next: ReactNode;
}) {
  const { t } = useI18n();
  if (status === 'completed') {
    return (
      <div className="flex flex-col gap-4" data-testid="finish-state" data-state="completed">
        <Notice tone="success" icon={<CircleCheck size={20} />}>
          <span className="block">{accepted ? t('marathons.completedTitle') : t('marathons.lessonCompleted')}</span>
          {accepted && <span className="block text-sm font-normal">{t('marathons.completedByTeacherStudent')}</span>}
        </Notice>
        {accepted && rating && <RatingView rating={rating} comment={comment} commentLabel={t('marathons.teacherCommentLabel')} />}
        {next && <div>{next}</div>}
      </div>
    );
  }
  if (status === 'submitted') {
    return (
      <div data-testid="finish-state" data-state="submitted">
        <Notice tone="warning" icon={<Hourglass size={20} />}>
          <span className="block">{t('marathons.awaitingTitle')}</span>
          <span className="block text-sm font-normal">{t('marathons.awaitingStudentText')}</span>
        </Notice>
      </div>
    );
  }
  if (!teacherCheck) {
    // Check off: the student completes the lesson themselves (also a lesson left on revision from before).
    return (
      <div data-testid="finish-state" data-state="self">
        <Button icon={<Flag size={16} />} loading={busy} onClick={onComplete}>{t('marathons.completeLesson')}</Button>
      </div>
    );
  }
  return (
    <div className="flex flex-col gap-4" data-testid="finish-state" data-state={status}>
      {status === 'revision' ? (
        <Notice tone="danger" icon={<RotateCcw size={20} />}>
          <span className="block">{t('marathons.revisionTitle')}</span>
          <span className="block text-sm font-normal">{t('marathons.revisionStudentText')}</span>
        </Notice>
      ) : (
        <Notice tone="info" icon={<Info size={20} />}>
          <span className="block">{t('marathons.notCompletedTitle')}</span>
          <span className="block text-sm font-normal">{t('marathons.notCompletedText')}</span>
        </Notice>
      )}
      <div><Button icon={<Send size={16} />} loading={busy} onClick={onSubmit}>{t('marathons.submitLesson')}</Button></div>
    </div>
  );
}

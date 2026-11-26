'use client';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Check, Clock, EyeOff, Lock, RotateCcw, Timer, X } from 'lucide-react';
import { Badge, Button, ChoiceMark, ConfirmDialog, Notice, cn, useToast } from '@/components/ui';
import { attemptsApi, exercisesApi, previewApi } from '@/lib/api/endpoints';
import { ApiError } from '@/lib/api/client';
import { formatDuration } from '@/lib/format';
import { useT } from '@/lib/i18n';
import { errorMessage } from '@/lib/errors';
import type { AnswerView, ExerciseView, Question } from '@/types/api';
import { GapQuestion } from './GapQuestion';
import { TextBlock } from './TextBlock';
import { PdfBlock } from './PdfBlock';

/** Replaces an exercise view in every cached lesson list of this classroom. */
export function useExerciseCache(classroomId: string) {
  const qc = useQueryClient();
  return useCallback(
    (view: ExerciseView) => {
      qc.setQueriesData<{ items: ExerciseView[] }>({ queryKey: ['lesson-exercises', classroomId] }, (old) =>
        old ? { items: old.items.map((x) => (x.exercise_id === view.exercise_id ? view : x)) } : old,
      );
      qc.invalidateQueries({ queryKey: ['progress', classroomId] });
    },
    [qc, classroomId],
  );
}

/** Replaces an exercise view in the cached preview list of a lesson. */
function usePreviewCache(lessonId: string | undefined) {
  const qc = useQueryClient();
  return useCallback(
    (view: ExerciseView) => {
      if (!lessonId) return;
      qc.setQueryData<{ items: ExerciseView[] }>(['preview', lessonId], (old) =>
        old ? { items: old.items.map((x) => (x.exercise_id === view.exercise_id ? view : x)) } : old);
    },
    [qc, lessonId],
  );
}

interface Props {
  classroomId: string;
  view: ExerciseView;
  role: 'teacher' | 'student';
  studentId: string;
  studentName: string;
  index: number;
  /** Read-only rendering of a submitted snapshot (no reset, no live actions). */
  snapshot?: boolean;
  /**
   * Teacher preview (stage 8): rendered as for a student, but every call goes to /preview with the teacher's
   * own preview attempts; «Сбросить попытку» archives only the preview attempt.
   */
  preview?: { lessonId: string };
  /**
   * Reaction to an answer (marathon appearance, stage 10): called only when the server's response marks the
   * question as checked, with the server's verdict — never for the client's own guess or an unchecked answer.
   */
  onChecked?: (correct: boolean) => void;
}

/** One exercise of an open lesson: a read-only text or PDF block, or a test / gaps exercise the student answers. */
export function TestPlayer(props: Props) {
  if (props.view.kind === 'text') return <TextBlock view={props.view} teacher={props.role === 'teacher'} index={props.index} />;
  if (props.view.kind === 'pdf') {
    // In the teacher preview the file comes through the lesson owner's route, in a class through the class.
    const url = props.preview ? exercisesApi.fileUrl(props.view.exercise_id) : attemptsApi.fileUrl(props.classroomId, props.view.exercise_id);
    return <PdfBlock view={props.view} teacher={props.role === 'teacher'} index={props.index} url={url} />;
  }
  return <QuestionPlayer {...props} />;
}

function QuestionPlayer({ classroomId, view, role, studentId, studentName, index, snapshot, preview, onChecked }: Props) {
  const t = useT();
  const toast = useToast();
  const putClass = useExerciseCache(classroomId);
  const putPreview = usePreviewCache(preview?.lessonId);
  const put = preview ? putPreview : putClass;
  const ops = preview
    ? {
      view: () => previewApi.view(view.exercise_id), start: () => previewApi.start(view.exercise_id), answer: previewApi.answer,
      finish: previewApi.finish, reset: (attemptId: string) => previewApi.reset(attemptId),
    }
    : {
      view: () => attemptsApi.view(classroomId, view.exercise_id), start: () => attemptsApi.start(classroomId, view.exercise_id), answer: attemptsApi.answer,
      finish: attemptsApi.finish, reset: () => attemptsApi.reset(classroomId, view.exercise_id, studentId),
    };
  const v = view.version;
  const a = view.attempt;
  const teacher = role === 'teacher';
  const timed = v.settings.timer_enabled && !!v.settings.timer_seconds;
  const instant = v.settings.check_mode === 'instant';
  const finished = a?.status === 'finished';
  // «Слова и пропуски»: the same attempt flow, but each question is a phrase with a typed word.
  const gaps = (v.kind ?? view.kind) === 'gaps';
  const [pending, setPending] = useState<Record<string, string[]>>({});
  const [texts, setTexts] = useState<Record<string, string>>({}); // typed words not yet saved
  const [confirmFinish, setConfirmFinish] = useState(false);
  const [confirmReset, setConfirmReset] = useState(false);

  useEffect(() => { setPending({}); setTexts({}); }, [a?.id]);

  const onErr = (e: unknown) => {
    if (e instanceof ApiError && (e.code === 'attempt_expired' || e.code === 'attempt_finished' || e.code === 'homework_locked' || e.code === 'lesson_review_locked')) {
      ops.view().then(put).catch(() => {});
    }
    toast(t('test.saveError', { msg: errorMessage(e, t) }), 'error');
  };

  const start = useMutation({ mutationFn: () => ops.start(), onSuccess: put, onError: onErr });
  const answer = useMutation({
    mutationFn: async ({ qid, ids, check, text }: { qid: string; ids: string[]; check: boolean; text?: string }) => {
      let attemptId = a?.id;
      if (!attemptId) attemptId = (await ops.start()).attempt!.id;
      return ops.answer(attemptId, qid, ids, check, text ?? '');
    },
    onSuccess: (res, vars) => {
      put(res);
      if (vars.check && !teacher && onChecked) {
        const checked = res.attempt?.answers.find((x) => x.question_id === vars.qid);
        if (checked?.checked && (checked.result === 'correct' || checked.result === 'wrong')) onChecked(checked.result === 'correct');
      }
      setPending((p) => {
        const n = { ...p };
        if (JSON.stringify(n[vars.qid]) === JSON.stringify(vars.ids)) delete n[vars.qid];
        return n;
      });
      if (vars.text !== undefined) {
        setTexts((m) => {
          if ((m[vars.qid] ?? '').trim() !== vars.text) return m;
          const n = { ...m };
          delete n[vars.qid];
          return n;
        });
      }
    },
    onError: (e, vars) => {
      setPending((p) => {
        const n = { ...p };
        delete n[vars.qid];
        return n;
      });
      onErr(e);
    },
  });
  const finish = useMutation({ mutationFn: () => ops.finish(a!.id), onSuccess: (r) => { put(r); setConfirmFinish(false); }, onError: onErr });
  const reset = useMutation({ mutationFn: () => ops.reset(a!.id), onSuccess: (r) => { put(r); setConfirmReset(false); }, onError: onErr });

  const answers = useMemo(() => {
    const m: Record<string, AnswerView> = {};
    a?.answers.forEach((x) => (m[x.question_id] = x));
    return m;
  }, [a]);

  const remaining = useCountdown(a?.status === 'in_progress' ? a.deadline_at : null, a?.server_now);
  const expiredRef = useRef(false);
  useEffect(() => {
    if (remaining !== null && remaining <= 0 && !expiredRef.current && !teacher && a) {
      expiredRef.current = true;
      finish.mutate(); // server records the deadline as finish time
    }
    if (remaining !== null && remaining > 0) expiredRef.current = false;
  }, [remaining, teacher, a, finish]);

  const locked = view.locked && !finished;
  const interactive = !teacher && !finished && !view.locked && !(timed && !a) && !(remaining !== null && remaining <= 0);
  const answeredCount = v.questions.filter((q) => gaps
    ? (texts[q.id] ?? answers[q.id]?.text ?? '').trim() !== ''
    : (pending[q.id] ?? answers[q.id]?.selected ?? []).length > 0).length;

  const select = (q: Question, optionId: string) => {
    if (!interactive) return;
    const cur = answers[q.id];
    if (cur?.checked) return;
    const prev = pending[q.id] ?? cur?.selected ?? [];
    const ids = q.multiple ? (prev.includes(optionId) ? prev.filter((x) => x !== optionId) : [...prev, optionId]) : [optionId];
    setPending((p) => ({ ...p, [q.id]: ids }));
    answer.mutate({ qid: q.id, ids, check: instant && !q.multiple });
  };

  return (
    <article className="rounded-2xl border border-line bg-surface p-5 shadow-card sm:p-6" aria-labelledby={`ex-${view.exercise_id}`}>
      <header className="mb-4 flex flex-wrap items-start justify-between gap-3">
        <div>
          <p className="text-xs font-medium uppercase tracking-wide text-ink-muted">{t(gaps ? 'catalog.types.gaps' : 'catalog.types.test')} · {index + 1}</p>
          <h3 id={`ex-${view.exercise_id}`} className="mt-1 text-lg font-bold text-ink">{v.title}</h3>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {view.hidden && <Badge tone="neutral" icon={<EyeOff size={12} />}>{t('classroom.hiddenBadge')}</Badge>}
          {timed && <Badge tone="info" icon={<Timer size={12} />}>{formatDuration(v.settings.timer_seconds!)}</Badge>}
          {remaining !== null && remaining > 0 && (
            <span role="timer" aria-live="off" aria-label={t('test.timeLeft')}
              className={cn('inline-flex h-8 items-center gap-1.5 rounded-full px-3 text-base font-bold tabular-nums', remaining <= 10_000 ? 'bg-danger-soft text-danger-text' : 'bg-primary-soft text-primary-text')}>
              <Clock size={15} /> {formatDuration(remaining / 1000)}
            </span>
          )}
          {(teacher || preview) && a && !snapshot && (
            <Button size="sm" variant="secondary" icon={<RotateCcw size={14} />} onClick={() => setConfirmReset(true)}>{t('test.reset')}</Button>
          )}
        </div>
      </header>

      {teacher && !snapshot && (
        <p className="mb-4 rounded-lg bg-surface-alt px-3 py-2 text-sm text-ink-muted">
          {a ? t('test.teacherView') : t('test.noAttempt')}
          {view.reset_count > 0 && ` · ${t('test.resets', { n: view.reset_count })}`}
        </p>
      )}
      {locked && !snapshot && <Notice tone="warning" icon={<Lock size={18} />} className="mb-4">{t('test.locked')}</Notice>}

      {!teacher && timed && !a ? (
        <div className="flex flex-col items-center gap-3 rounded-xl bg-surface-alt px-6 py-10 text-center">
          <span className="flex h-14 w-14 items-center justify-center rounded-full bg-primary-soft text-primary-text"><Timer size={28} /></span>
          <p className="text-md font-bold text-ink">{t('test.startTitle')}</p>
          <p className="max-w-md text-sm text-ink-muted">{t('test.startText', { time: formatDuration(v.settings.timer_seconds!) })}</p>
          <Button size="lg" onClick={() => start.mutate()} loading={start.isPending} disabled={view.locked}>{t('test.start')}</Button>
        </div>
      ) : (
        <ol className="flex flex-col gap-5">
          {gaps && v.questions.map((q, qi) => (
            <GapQuestion key={q.id} q={q} index={qi} answer={answers[q.id]} teacher={teacher} finished={finished} instant={instant}
              interactive={interactive} text={texts[q.id]} busy={answer.isPending}
              onText={(value) => setTexts((m) => ({ ...m, [q.id]: value }))}
              onSave={(text, check) => answer.mutate({ qid: q.id, ids: [], text, check })} />
          ))}
          {!gaps && v.questions.map((q, qi) => {
            const ans = answers[q.id];
            const selected = pending[q.id] ?? ans?.selected ?? [];
            const revealed = !!ans?.result && (teacher || finished || (instant && ans.checked));
            const correctIds = new Set(ans?.correct_option_ids ?? (teacher ? q.options.filter((o) => o.is_correct).map((o) => o.id) : []));
            const showKey = teacher || revealed;
            const groupRole = q.multiple ? 'group' : 'radiogroup';
            return (
              <li key={q.id}>
                <p id={`q-${q.id}`} className="mb-1 text-md font-medium text-ink">{qi + 1}. {q.prompt}</p>
                <p className="mb-2 text-xs text-ink-muted">{q.multiple ? t('test.multipleHint') : t('test.singleHint')}{!instant && !finished && !teacher ? ` · ${t('test.afterHint')}` : ''}</p>
                <div role={groupRole} aria-labelledby={`q-${q.id}`} className="flex flex-col gap-2">
                  {q.options.map((o) => {
                    const isSel = selected.includes(o.id);
                    const isCorrect = correctIds.has(o.id);
                    // Colour is never the only signal: right/wrong/missed also render an icon + text.
                    let state: 'idle' | 'selected' | 'right' | 'wrong' | 'missed' = isSel ? 'selected' : 'idle';
                    if (revealed) {
                      if (isSel) state = isCorrect ? 'right' : 'wrong';
                      else if (isCorrect) state = 'missed';
                    } else if (showKey && isCorrect && !isSel) {
                      state = 'missed'; // teacher sees the key for unanswered questions
                    }
                    const tone = state === 'right' ? 'success' : state === 'wrong' ? 'danger' : 'primary';
                    const Tag = interactive && !ans?.checked ? 'button' : 'div';
                    return (
                      <Tag key={o.id}
                        {...(Tag === 'button' ? { type: 'button' as const, onClick: () => select(q, o.id) } : {})}
                        role={q.multiple ? 'checkbox' : 'radio'}
                        aria-checked={isSel}
                        aria-disabled={Tag === 'div' || undefined}
                        className={cn(
                          'flex min-h-[48px] w-full items-center gap-3 rounded-xl border-2 px-4 py-2.5 text-left text-base transition-colors',
                          state === 'idle' && 'border-line bg-surface',
                          state === 'idle' && Tag === 'button' && 'hover:border-line-strong hover:bg-surface-alt',
                          state === 'selected' && 'border-primary bg-primary-soft',
                          state === 'right' && 'border-success bg-success-soft',
                          state === 'wrong' && 'border-danger bg-danger-soft',
                          state === 'missed' && 'border-dashed border-success bg-surface',
                          Tag === 'div' && 'cursor-default',
                        )}>
                        <ChoiceMark type={q.multiple ? 'checkbox' : 'radio'} checked={isSel} tone={tone} />
                        <span className="flex-1 text-ink">{o.label}</span>
                        {state === 'right' && <span className="inline-flex items-center gap-1 text-sm font-medium text-success-text"><Check size={16} />{t('test.correct')}</span>}
                        {state === 'wrong' && <span className="inline-flex items-center gap-1 text-sm font-medium text-danger-text"><X size={16} />{t('test.wrong')}</span>}
                        {state === 'missed' && <span className="inline-flex items-center gap-1 text-sm text-success-text"><Check size={16} />{t('test.correctAnswer')}</span>}
                        {teacher && isSel && state === 'selected' && <span className="text-xs text-ink-muted">{t('test.teacherSelected')}</span>}
                      </Tag>
                    );
                  })}
                </div>
                {!teacher && instant && q.multiple && interactive && !ans?.checked && (
                  <Button size="sm" variant="soft" className="mt-2" disabled={!selected.length || answer.isPending}
                    onClick={() => answer.mutate({ qid: q.id, ids: selected, check: true })}>{t('test.check')}</Button>
                )}
              </li>
            );
          })}
        </ol>
      )}

      {!teacher && a && !finished && !view.locked && (
        <footer className="mt-6 flex flex-wrap items-center justify-between gap-3 border-t border-line pt-4">
          <span className="text-sm text-ink-muted">{t('test.answered', { a: answeredCount, b: v.questions.length })}</span>
          <Button variant="secondary" onClick={() => setConfirmFinish(true)}>{t('test.finish')}</Button>
        </footer>
      )}

      {finished && a?.summary && (
        <section className="mt-6 rounded-xl bg-surface-alt p-4" aria-label={t('test.resultTitle')}>
          {a.finish_reason === 'timeout' && <p className="mb-3 flex items-center gap-2 text-sm font-medium text-warning-text"><Clock size={16} />{t('test.timeout')}</p>}
          <dl className="grid grid-cols-2 gap-3 sm:grid-cols-5">
            <Summary label={t('test.elapsed')} value={formatDuration(a.summary.elapsed_seconds)} />
            <Summary label={t('test.correctCount')} value={a.summary.correct} tone="success" />
            <Summary label={t('test.wrongCount')} value={a.summary.wrong} tone="danger" />
            <Summary label={t('test.unansweredCount')} value={a.summary.unanswered} />
            {v.settings.scoring_enabled && <Summary label={t('test.points')} value={`${a.summary.points_earned} / ${a.summary.points_max}`} tone="primary" />}
          </dl>
        </section>
      )}

      <ConfirmDialog open={confirmFinish} onClose={() => setConfirmFinish(false)} onConfirm={() => finish.mutate()} loading={finish.isPending}
        title={t('test.finishConfirmTitle')} confirmLabel={t('test.finish')}
        text={<>{t('test.finishConfirmText')} {v.questions.length - answeredCount > 0 && <strong>{t('test.unansweredWarning', { n: v.questions.length - answeredCount })}</strong>}</>} />
      <ConfirmDialog open={confirmReset} onClose={() => setConfirmReset(false)} onConfirm={() => reset.mutate()} loading={reset.isPending} tone="danger"
        title={t('test.resetConfirmTitle')} text={preview ? t('test.previewResetText') : t('test.resetConfirmText', { name: studentName })} confirmLabel={t('test.reset')} />
    </article>
  );
}

function Summary({ label, value, tone }: { label: string; value: string | number; tone?: 'success' | 'danger' | 'primary' }) {
  return (
    <div>
      <dt className="text-xs text-ink-muted">{label}</dt>
      <dd className={cn('text-lg font-bold', tone === 'success' ? 'text-success-text' : tone === 'danger' ? 'text-danger-text' : tone === 'primary' ? 'text-primary-text' : 'text-ink')}>{value}</dd>
    </div>
  );
}

/** Milliseconds left until deadline, corrected for client clock skew via server_now. */
function useCountdown(deadline: string | null | undefined, serverNow: string | undefined) {
  const skew = useMemo(() => (serverNow ? new Date(serverNow).getTime() - Date.now() : 0), [serverNow]);
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (!deadline) return;
    const id = setInterval(() => setNow(Date.now()), 250);
    return () => clearInterval(id);
  }, [deadline]);
  if (!deadline) return null;
  return new Date(deadline).getTime() - (now + skew);
}

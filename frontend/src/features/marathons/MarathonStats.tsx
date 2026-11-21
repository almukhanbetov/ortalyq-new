'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, ArrowRight, BarChart3, Download, Footprints, MessageSquare, PencilLine, Star, Trophy } from 'lucide-react';
import { Avatar, Button, EmptyState, ErrorState, Field, Loading, Modal, Notice, QueryState, Textarea, cn, useToast } from '@/components/ui';
import { PageCard } from '@/components/layout/PageCard';
import { classroomsApi, marathonsApi } from '@/lib/api/endpoints';
import { ApiError } from '@/lib/api/client';
import { qk } from '@/lib/queryKeys';
import { useI18n } from '@/lib/i18n';
import { errorMessage, fieldErrors } from '@/lib/errors';
import { marathonStudentUrl, marathonUrl, marathonResultsUrl } from '@/lib/routes';
import type { CategoryPoints, Classroom, Locale, MarathonLesson, MarathonStudentsRating, MarathonStats } from '@/types/api';
import { LessonStatusChip, MarathonDatesBanner, PassedBar } from './MarathonParts';
import { useMarathonLive } from './useMarathonLive';

type T = (k: string, v?: Record<string, string | number>) => string;

/** Number in the export: two decimals, comma for ru/kk (as the original's «5,00»), point for en. */
const csvNumber = (n: number, locale: Locale) => (locale === 'en' ? n.toFixed(2) : n.toFixed(2).replace('.', ','));

/** A CSV field: quoted when needed; a leading = + - @ is neutralised so spreadsheets never run it as a formula. */
const csvField = (v: string) => {
  let s = v;
  if (/^[=+\-@\t\r]/.test(s)) s = `'${s}`;
  return /[;"\r\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
};

/**
 * «Скачать данные»: the original's export format (UTF-8 with BOM, «;», CRLF, every row ending with «;»):
 * Email, name, lessons completed, points earned and a column per score category. Built from GET /stats, so the
 * file always matches the table on the page.
 */
export function statsCsv(stats: MarathonStats, t: T, locale: Locale) {
  const head = [t('marathons.csv.email'), t('marathons.csv.name'), t('marathons.csv.completed'), t('marathons.csv.points'),
    ...stats.categories.map((c) => c.category_name)];
  const lines = [head.map(csvField).join(';') + ';'];
  for (const r of stats.items) {
    const cats = stats.categories.map((c) => r.points.categories.find((x) => x.category_id === c.category_id)?.earned ?? 0);
    lines.push([csvField(r.email), csvField(r.full_name), String(r.completed_lessons), csvNumber(r.points.earned, locale),
      ...cats.map((n) => csvNumber(n, locale))].join(';') + ';');
  }
  return '﻿' + lines.join('\r\n') + '\r\n';
}

function download(name: string, text: string) {
  const url = URL.createObjectURL(new Blob([text], { type: 'text/csv;charset=utf-8' }));
  const a = document.createElement('a');
  a.href = url;
  a.download = name;
  document.body.appendChild(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

/** "Заработано баллов X / Y" with a bar per category. */
export function PointsCard({ earned, max, categories, title }: { earned: number; max: number; categories: CategoryPoints[]; title: string }) {
  const { t } = useI18n();
  return (
    <div className="rounded-lg border border-line-soft p-4" data-testid="points-card">
      <h3 className="text-base font-bold text-ink-body">{title}</h3>
      <p className="mt-2 flex items-baseline justify-between gap-3 text-sm text-ink-body">
        <span>{t('marathons.earnedPoints')}</span><span className="text-lg font-bold" data-testid="points-total">{earned} / {max}</span>
      </p>
      {categories.map((c) => (
        <div key={c.category_id ?? 'none'} className="mt-3">
          <p className="flex justify-between gap-3 text-xs text-ink-soft"><span>{c.category_name}</span><span>{c.earned} / {c.max}</span></p>
          <div className="mt-1 h-1.5 overflow-hidden rounded-full bg-surface-muted" role="progressbar" aria-label={c.category_name}
            aria-valuemin={0} aria-valuemax={c.max} aria-valuenow={c.earned}>
            <div className="h-full rounded-full bg-success" style={{ width: `${c.max ? Math.round((c.earned / c.max) * 100) : 0}%` }} />
          </div>
        </div>
      ))}
    </div>
  );
}

const MEDALS = ['🥇', '🥈', '🥉'];

/**
 * «Рейтинг учеников»: only real participants from the server (never sample rows). Places 1–3 get medals;
 * the viewer's row is marked; a student outside the shown places sees their own place below when allowed.
 */
export function RatingList({ rating, title }: { rating: MarathonStudentsRating; title?: string }) {
  const { t } = useI18n();
  const row = (e: MarathonStudentsRating['items'][number], key: string) => (
    <li key={key} data-testid="rating-row" data-place={e.place} data-me={e.me ? '1' : undefined}
      className={cn('flex items-center gap-3 border-b border-line-soft px-1 py-2.5 last:border-b-0', e.me && 'bg-primary-soft')}>
      <span className="w-7 shrink-0 text-center text-sm font-medium text-ink-soft">
        {e.place <= 3 ? <span role="img" aria-label={String(e.place)}>{MEDALS[e.place - 1]}</span> : e.place}
      </span>
      <Avatar name={e.full_name} color={e.avatar_color} size={32} />
      <span className="min-w-0 flex-1 truncate text-sm font-medium text-ink-body">
        {e.full_name}{e.me && <span className="ml-1.5 text-xs text-primary-text">({t('marathons.ratingYou')})</span>}
      </span>
      <span className="shrink-0 text-sm font-bold text-ink-body">{e.points}</span>
    </li>
  );
  return (
    <section className="rounded-lg border border-line-soft p-4" data-testid="rating" aria-label={title ?? t('marathons.studentsRating')}>
      <h3 className="mb-1 flex items-center gap-2 text-base font-bold text-ink-body"><Trophy size={18} aria-hidden />{title ?? t('marathons.studentsRating')}</h3>
      {rating.items.length === 0 ? (
        <p className="py-4 text-sm text-ink-soft">{t('marathons.ratingEmpty')}</p>
      ) : (
        <ol>{rating.items.map((e) => row(e, e.student_id))}</ol>
      )}
      {rating.limit && rating.total > rating.items.length && (
        <p className="mt-2 text-xs text-ink-soft">{t('marathons.ratingShown', { n: rating.limit, total: rating.total })}</p>
      )}
      {rating.me && (
        <div className="mt-3 border-t border-dashed border-line-strong pt-2" data-testid="rating-own">
          <p className="mb-1 text-xs font-medium text-ink-soft">{t('marathons.ratingYourPlace', { n: rating.me.place })}</p>
          <ol>{row(rating.me, 'me')}</ol>
        </div>
      )}
    </section>
  );
}

/** Statistics off: the original's notice with «Перейти в настройки» (opens the settings on «Дополнительно»). */
function StatsDisabled({ onOpenSettings }: { onOpenSettings: () => void }) {
  const { t } = useI18n();
  return (
    <div className="flex min-h-[320px] flex-col items-center justify-center rounded-lg border border-surface-muted px-4 py-10 text-center" data-testid="stats-disabled">
      <Star size={56} className="text-warning" aria-hidden />
      <p className="mt-4 max-w-xl text-[20px] leading-7 text-ink-soft">{t('marathons.statsDisabledText')}</p>
      <Button className="mt-4" onClick={onOpenSettings}>{t('marathons.goToSettings')}</Button>
    </div>
  );
}

/**
 * "Статистика" tab of the owner: every participant with lessons completed, points and score categories, sorted by
 * place; «Скачать данные» exports the same rows. Only the "all time" period exists (as in the original); there is
 * no filter to pretend otherwise.
 */
export function MarathonStatistics({ marathon, onOpenSettings }: { marathon: Classroom; onOpenSettings: () => void }) {
  const { t, locale } = useI18n();
  const on = !!marathon.marathon?.stats_enabled;
  const stats = useQuery({ queryKey: qk.marathonStats(marathon.id), queryFn: () => marathonsApi.stats(marathon.id), enabled: on });
  if (!on) return <StatsDisabled onOpenSettings={onOpenSettings} />;
  return (
    <QueryState query={stats}>
      {(d) => {
        const date = new Intl.DateTimeFormat('ru-RU', { day: '2-digit', month: '2-digit', year: '2-digit' }).format(new Date());
        return (
          <div data-testid="marathon-stats">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <span className="inline-flex h-9 items-center gap-2 rounded-md border border-line-soft px-3 text-sm text-ink-body">
                <BarChart3 size={16} aria-hidden />{t('marathons.statsAllTime')}
              </span>
              <Button variant="secondary" icon={<Download size={16} />}
                onClick={() => download(`${t('marathons.csv.file', { name: marathon.name, date })}.csv`, statsCsv(d, t, locale))}>
                {t('marathons.downloadData')}
              </Button>
            </div>
            {d.items.length === 0 ? (
              <EmptyState title={t('marathons.statsNoStudents')} className="py-12" />
            ) : (
              <div className="mt-4 overflow-x-auto rounded-lg border border-line-soft">
                <table className="w-full min-w-[640px] border-collapse text-sm" data-testid="stats-table">
                  <thead className="bg-surface-muted text-left text-xs font-medium text-ink-soft">
                    <tr>
                      <th scope="col" className="w-16 px-3 py-2">{t('marathons.statsPlace')}</th>
                      <th scope="col" className="px-3 py-2">{t('marathons.statsStudent')}</th>
                      <th scope="col" className="px-3 py-2">{t('marathons.statsCompleted')}</th>
                      <th scope="col" className="px-3 py-2 text-right">{t('marathons.earnedPoints')}</th>
                      {d.categories.map((c) => <th key={c.category_id ?? 'none'} scope="col" className="px-3 py-2 text-right">{c.category_name}</th>)}
                    </tr>
                  </thead>
                  <tbody>
                    {d.items.map((r) => (
                      <tr key={r.student_id} className="border-t border-line-soft" data-testid="stats-row" data-student={r.student_id} data-place={r.place}>
                        <td className="px-3 py-2.5 font-medium text-ink-body">
                          {r.place ? r.place : <span className="text-xs text-ink-soft">{t('marathons.statsNotRated')}</span>}
                        </td>
                        <td className="px-3 py-2.5">
                          <Link href={marathonStudentUrl(marathon.id, r.student_id)} className="flex min-w-0 items-center gap-2 hover:underline">
                            <Avatar name={r.full_name} color={r.avatar_color} size={28} />
                            <span className="min-w-0">
                              <span className="block truncate font-medium text-ink-body">{r.full_name}</span>
                              <span className="block truncate text-xs text-ink-soft">{r.email}</span>
                            </span>
                          </Link>
                        </td>
                        <td className="px-3 py-2.5"><PassedBar done={r.completed_lessons} total={r.total_lessons} muted={r.status === 'inactive'} compact /></td>
                        <td className="px-3 py-2.5 text-right font-bold text-ink-body" data-testid="stats-points">{r.points.earned} / {r.points.max}</td>
                        {d.categories.map((c) => {
                          const x = r.points.categories.find((y) => y.category_id === c.category_id);
                          return <td key={c.category_id ?? 'none'} className="px-3 py-2.5 text-right text-ink-body">{x ? `${x.earned} / ${x.max}` : '—'}</td>;
                        })}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
            <p className="mt-3 text-xs text-ink-soft">{t('marathons.statsNote')}</p>
          </div>
        );
      }}
    </QueryState>
  );
}

/** Student's «Результаты урока» under the lesson (statistics on): own points, the teacher's comment, the lesson rating. */
export function StudentLessonResults({ marathonId, lessonId }: { marathonId: string; lessonId: string }) {
  const { t } = useI18n();
  const res = useQuery({ queryKey: qk.marathonLessonResults(marathonId, lessonId), queryFn: () => marathonsApi.lessonResults(marathonId, lessonId), retry: false });
  if (!res.data) return null;
  const r = res.data;
  return (
    <section className="mt-10 flex flex-col gap-4 border-t border-line-soft pt-6" aria-labelledby="lesson-results" data-testid="lesson-results">
      <h2 id="lesson-results" className="text-xl font-bold text-ink">{t('marathons.lessonResultsTitle')}</h2>
      {r.points && <PointsCard earned={r.points.earned} max={r.points.max} categories={r.points.categories} title={t('marathons.studentStats')} />}
      {r.comment && (
        <div className="rounded-lg border border-line-soft p-4" data-testid="stats-comment">
          <p className="flex items-center gap-2 text-xs font-medium text-ink-soft"><MessageSquare size={14} aria-hidden />{t('marathons.statsComment')}</p>
          <p className="mt-1 whitespace-pre-wrap break-words text-sm text-ink-body">{r.comment.body}</p>
        </div>
      )}
      {r.rating && <RatingList rating={r.rating} />}
    </section>
  );
}

/**
 * Owner's «Результаты урока» from the lessons tab: every participant's points and completion for this lesson,
 * the lesson rating and «Комментарий к статистике» (this marathon only, Н7).
 */
export function LessonResultsModal({ marathon, lesson, onClose }: { marathon: Classroom; lesson: MarathonLesson; onClose: () => void }) {
  const { t } = useI18n();
  const qc = useQueryClient();
  const toast = useToast();
  const key = qk.marathonLessonResults(marathon.id, lesson.lesson_id);
  const res = useQuery({ queryKey: key, queryFn: () => marathonsApi.lessonResults(marathon.id, lesson.lesson_id), retry: false });
  const [draft, setDraft] = useState<string | null>(null);
  const save = useMutation({
    mutationFn: (body: string) => marathonsApi.setLessonComment(marathon.id, lesson.lesson_id, body),
    onSuccess: () => { qc.invalidateQueries({ queryKey: key }); setDraft(null); toast(t('marathons.statsCommentSaved'), 'success'); },
  });
  const disabled = res.error instanceof ApiError && res.error.code === 'marathon_stats_disabled';
  return (
    <Modal open onClose={onClose} title={t('marathons.lessonResultsTitle')} size="lg">
      <p className="-mt-2 mb-4 text-sm text-ink-soft">{lesson.title}</p>
      {disabled ? <Notice tone="neutral" className="text-sm">{t('marathons.statsDisabledText')}</Notice> : (
        <QueryState query={res}>
          {(r) => (
            <div className="flex flex-col gap-4" data-testid="teacher-lesson-results">
              {draft === null ? (
                r.comment ? (
                  <div className="rounded-lg border border-line-soft p-4" data-testid="stats-comment">
                    <p className="flex items-center gap-2 text-xs font-medium text-ink-soft"><MessageSquare size={14} aria-hidden />{t('marathons.statsComment')}</p>
                    <p className="mt-1 whitespace-pre-wrap break-words text-sm text-ink-body">{r.comment.body}</p>
                    <Button className="mt-2" size="sm" variant="secondary" icon={<PencilLine size={14} />} onClick={() => setDraft(r.comment?.body ?? '')}>{t('marathons.editStatsComment')}</Button>
                  </div>
                ) : (
                  <button type="button" onClick={() => setDraft('')}
                    className="flex flex-col items-center gap-1 rounded-lg border border-line-soft px-4 py-4 text-sm font-medium text-ink-soft transition-colors hover:bg-surface-hover focus-visible:shadow-focus focus-visible:outline-none">
                    <MessageSquare size={18} aria-hidden />{t('marathons.addStatsComment')}
                  </button>
                )
              ) : (
                <div className="flex flex-col gap-2">
                  <Field label={t('marathons.statsComment')} hint={t('marathons.statsCommentHint')} error={fieldErrors(save.error).body}>
                    {(p) => <Textarea {...p} rows={3} maxLength={2000} value={draft} onChange={(e) => setDraft(e.target.value)} />}
                  </Field>
                  {save.error && !fieldErrors(save.error).body && <p role="alert" className="text-sm text-danger-text">{errorMessage(save.error, t)}</p>}
                  <div className="flex gap-2">
                    <Button size="sm" loading={save.isPending} onClick={() => save.mutate(draft)}>{t('common.save')}</Button>
                    <Button size="sm" variant="secondary" onClick={() => setDraft(null)}>{t('common.cancel')}</Button>
                  </div>
                </div>
              )}
              {(r.students ?? []).length === 0 ? <EmptyState title={t('marathons.statsNoStudents')} className="py-6" /> : (
                <ol className="rounded-lg border border-line-soft" data-testid="lesson-students">
                  {(r.students ?? []).map((s) => (
                    <li key={s.student_id} className="flex flex-wrap items-center gap-3 border-b border-line-soft px-3 py-2.5 last:border-b-0">
                      <span className="w-12 shrink-0 text-sm font-medium text-ink-body">{s.place || '—'}</span>
                      <Avatar name={s.full_name} color={s.avatar_color} size={28} />
                      <span className="min-w-0 flex-1 truncate text-sm text-ink-body">{s.full_name}</span>
                      <LessonStatusChip status={s.completed ? 'completed' : 'none'} />
                      <span className="w-16 shrink-0 text-right text-sm font-bold text-ink-body">{s.points.earned} / {s.points.max}</span>
                    </li>
                  ))}
                </ol>
              )}
            </div>
          )}
        </QueryState>
      )}
    </Modal>
  );
}

/** Student's own results entry on the marathon page: a link to the results page and the rating when it is on. */
export function StudentStatsBlock({ marathon }: { marathon: Classroom }) {
  const { t } = useI18n();
  const ratingOn = !!marathon.marathon?.stats_enabled && !!marathon.marathon?.rating_enabled;
  const rating = useQuery({ queryKey: qk.marathonRating(marathon.id), queryFn: () => marathonsApi.rating(marathon.id), enabled: ratingOn, retry: false });
  return (
    <div className="mt-6 flex flex-col gap-4">
      <div>
        <Link href={marathonResultsUrl(marathon.id)} data-testid="show-results"
          className="inline-flex h-10 items-center gap-2 rounded-md bg-surface-muted px-4 text-base font-medium text-ink-body transition-colors hover:bg-surface-hover focus-visible:shadow-focus focus-visible:outline-none">
          <Footprints size={18} aria-hidden />{t('marathons.showResults')}
        </Link>
      </div>
      {ratingOn && rating.data && <RatingList rating={rating.data} />}
    </div>
  );
}

/** «Результаты» — «Успехи ученика в марафоне» (П12): lessons and exercises always, points and place with statistics. */
export function MyResultsPage({ id }: { id: string }) {
  const { t } = useI18n();
  const cls = useQuery({ queryKey: qk.classroom(id), queryFn: () => classroomsApi.get(id) });
  const c = cls.data;
  useMarathonLive(c);
  const res = useQuery({ queryKey: qk.marathonMyResults(id), queryFn: () => marathonsApi.myResults(id), enabled: !!c && c.my_role === 'student', retry: false });
  if (cls.isLoading) return <PageCard maxWidth={1024}><Loading /></PageCard>;
  if (cls.error || !c) return <PageCard maxWidth={1024}><ErrorState error={cls.error} onRetry={() => cls.refetch()} /></PageCard>;
  const tile = (emoji: string, label: string, value: number | string, testid: string) => (
    <div className="flex flex-col items-center gap-1 rounded-lg border border-line-soft px-4 py-5 text-center" data-testid={testid}>
      <span className="text-3xl leading-none" aria-hidden>{emoji}</span>
      <span className="text-sm text-ink-soft">{label}</span>
      <span className="text-xl font-bold text-ink-body">{value}</span>
    </div>
  );
  return (
    <PageCard maxWidth={1024} className="pt-[18px] sm:pt-8">
      <Link href={marathonUrl(id)} className="inline-flex items-center gap-2 text-sm font-medium text-ink-muted hover:text-ink">
        <ArrowLeft size={16} aria-hidden />{t('marathons.backToMarathon')} · {c.name}
      </Link>
      {c.marathon && <MarathonDatesBanner info={c.marathon} className="mt-4" />}
      <h1 className="mt-6 text-[26px] font-bold leading-8 text-ink-body">{t('marathons.myResultsTitle')}</h1>
      <p className="mt-1 text-base text-ink-soft">{t('marathons.myResultsSubtitle')}</p>
      {c.my_role !== 'student' ? (
        <EmptyState title={t('marathons.notFound')} className="py-10" />
      ) : res.error ? (
        <ErrorState error={res.error} onRetry={() => res.refetch()} />
      ) : !res.data ? <Loading /> : (
        <div className="mt-6 flex flex-col gap-4" data-testid="my-results">
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
            {tile('🏃', t('marathons.passedLessons'), `${res.data.completed_lessons} / ${res.data.total_lessons}`, 'my-lessons')}
            {tile('📝', t('marathons.finishedExercises'), `${res.data.finished_exercises} / ${res.data.exercises}`, 'my-exercises')}
            {res.data.points && tile('💪', t('marathons.earnedPoints'), `${res.data.points.earned} / ${res.data.points.max}`, 'my-points')}
          </div>
          {res.data.points ? (
            <>
              {res.data.points.categories.length > 0 && <PointsCard earned={res.data.points.earned} max={res.data.points.max} categories={res.data.points.categories} title={t('marathons.studentStats')} />}
              {res.data.rating && <RatingList rating={res.data.rating} />}
            </>
          ) : (
            <Notice tone="neutral" className="text-sm">{t('marathons.myResultsStatsOff')}</Notice>
          )}
          <div>
            <Link href={marathonUrl(id)} className="inline-flex h-10 items-center gap-2 rounded-md bg-primary px-4 text-base font-medium text-white transition-colors hover:bg-primary-hover focus-visible:shadow-focus focus-visible:outline-none">
              <ArrowRight size={18} aria-hidden />{t('marathons.backToMarathon')}
            </Link>
          </div>
        </div>
      )}
    </PageCard>
  );
}

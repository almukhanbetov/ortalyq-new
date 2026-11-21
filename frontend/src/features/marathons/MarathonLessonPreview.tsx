'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, Eye, Info, LogOut, RotateCcw } from 'lucide-react';
import { Button, ConfirmDialog, EmptyState, ErrorState, Loading, Notice, QueryState, useToast } from '@/components/ui';
import { PageCard } from '@/components/layout/PageCard';
import { TestPlayer } from '@/features/exercises/TestPlayer';
import { classroomsApi, materialsApi, previewApi } from '@/lib/api/endpoints';
import { qk } from '@/lib/queryKeys';
import { useT } from '@/lib/i18n';
import { errorMessage } from '@/lib/errors';
import { marathonUrl } from '@/lib/routes';
import { LookBackground, ReactionBurst, SoundToggle, useAnswerReactions, useLessonLook } from './appearance/LessonLook';

/**
 * «Режим предпросмотра» of a marathon lesson for its owner: the lesson as a student sees it (hidden exercises
 * left out, the key shown only when revealed, the timer running), with the teacher's own preview attempts. The
 * server keeps them apart from student attempts, so nothing here reaches students, statistics, the rating, the
 * review or the opening of lessons. «Сбросить предпросмотр» archives every preview attempt of the lesson.
 */
export function MarathonLessonPreview({ id, lessonId }: { id: string; lessonId: string }) {
  const t = useT();
  const router = useRouter();
  const qc = useQueryClient();
  const toast = useToast();
  const [confirmReset, setConfirmReset] = useState(false);
  const cls = useQuery({ queryKey: qk.classroom(id), queryFn: () => classroomsApi.get(id) });
  const c = cls.data;
  const owner = c?.my_role === 'teacher' && c.lesson_type === 'marathon';
  const lesson = useQuery({ queryKey: ['preview-lesson', lessonId], queryFn: () => materialsApi.lesson(lessonId), enabled: owner, retry: false });
  const exercises = useQuery({ queryKey: qk.preview(lessonId), queryFn: () => previewApi.lessonExercises(lessonId), enabled: owner, retry: false });
  // The preview shows the lesson with the appearance students get (background, reactions).
  const look = useLessonLook(id, lessonId, owner);
  const reactions = useAnswerReactions(look);
  const reset = useMutation({
    mutationFn: () => previewApi.resetLesson(lessonId),
    onSuccess: (res) => { qc.setQueryData(qk.preview(lessonId), res); setConfirmReset(false); toast(t('marathons.previewResetDone'), 'success'); },
    onError: (e) => toast(errorMessage(e, t), 'error'),
  });
  useEffect(() => {
    // Students (and other class types) have no preview: back to the marathon page.
    if (c && !owner) router.replace(marathonUrl(id));
  }, [c, owner, id, router]);

  if (cls.isLoading) return <PageCard maxWidth={1024}><Loading /></PageCard>;
  if (cls.error || !c) return <PageCard maxWidth={1024}><ErrorState error={cls.error} onRetry={() => cls.refetch()} /></PageCard>;
  if (!owner) return null;
  const back = marathonUrl(id, 'lessons');

  return (
    <PageCard maxWidth={1024} className="relative isolate overflow-clip pt-[18px] sm:pt-8">
      <LookBackground look={look} />
      <ReactionBurst burst={reactions.burst} onDone={reactions.clear} />
      <div className="flex flex-wrap items-center justify-between gap-3">
        <Link href={back} className="inline-flex items-center gap-2 text-sm font-medium text-ink-muted hover:text-ink">
          <ArrowLeft size={16} aria-hidden />{t('marathons.backToMarathon')} · {c.name}
        </Link>
        {reactions.hasSound && <SoundToggle on={reactions.soundOn} onChange={reactions.setSoundOn} />}
      </div>
      <div role="region" aria-label={t('marathons.previewTitle')} data-testid="preview-banner"
        className="sticky top-[calc(theme(spacing.cabinetbar)+8px)] z-10 mt-4 flex flex-wrap items-center gap-3 rounded-xl bg-primary px-4 py-3 text-white shadow-card">
        <Eye size={20} aria-hidden className="shrink-0" />
        <div className="min-w-0 flex-1">
          <p className="text-base font-bold">{t('marathons.previewTitle')}</p>
          <p className="text-sm">{t('marathons.previewBanner')}</p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button size="sm" variant="secondary" icon={<RotateCcw size={14} />} onClick={() => setConfirmReset(true)}>{t('marathons.previewReset')}</Button>
          <Link href={back} className="inline-flex h-8 items-center gap-1.5 rounded-md bg-white/15 px-3 text-sm font-medium text-white transition-colors hover:bg-white/25 focus-visible:shadow-focus focus-visible:outline-none">
            <LogOut size={14} aria-hidden />{t('marathons.previewExit')}
          </Link>
        </div>
      </div>
      <QueryState query={lesson}>
        {(l) => (
          <div className="mt-6" data-testid="preview-lesson">
            <h1 className="text-[26px] font-bold leading-8 text-ink-body">{l.title}</h1>
            {(l.sections ?? []).length === 0 && <EmptyState title={t('classroom.noSections')} className="py-10" />}
            {(l.sections ?? []).map((s, si) => (
              <section key={s.id} aria-labelledby={`sec-${s.id}`} className="mt-8">
                <p className="text-xs font-medium uppercase tracking-wide text-ink-muted">{t('classroom.sectionCounter', { a: si + 1, b: l.sections!.length })}</p>
                <h2 id={`sec-${s.id}`} className="mb-4 text-xl font-bold text-ink">{s.title}</h2>
                <QueryState query={exercises}>
                  {(data) => {
                    const items = data.items.filter((e) => e.section_id === s.id);
                    if (!items.length) return <EmptyState title={t('classroom.sectionEmpty')} className="rounded-2xl border border-dashed border-line-strong bg-surface" />;
                    return (
                      <div className="flex flex-col gap-4">
                        {items.map((v, i) => (
                          <TestPlayer key={v.exercise_id} classroomId={id} view={v} role="student" studentId="" studentName="" index={i} preview={{ lessonId }} onChecked={reactions.react} />
                        ))}
                      </div>
                    );
                  }}
                </QueryState>
              </section>
            ))}
            <div className="mt-10 border-t border-line-soft pt-6">
              <h2 className="mb-3 text-xl font-bold text-ink">{t('marathons.completeLesson')}</h2>
              <Notice tone="neutral" icon={<Info size={20} />} className="text-sm">{t('marathons.previewFinishNote')}</Notice>
            </div>
          </div>
        )}
      </QueryState>
      <ConfirmDialog open={confirmReset} onClose={() => setConfirmReset(false)} onConfirm={() => reset.mutate()} loading={reset.isPending} tone="warning"
        title={t('marathons.previewReset')} text={t('marathons.previewResetText')} confirmLabel={t('marathons.previewReset')} />
    </PageCard>
  );
}

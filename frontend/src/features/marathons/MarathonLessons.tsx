'use client';

import { useCallback, useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertCircle, ArrowDown, BarChart3, Eye, ArrowRight, CalendarClock, ArrowUp, Check, Copy, FilePlus, FolderOpen, Hourglass, Info, Lock, LockOpen, MoreHorizontal, Pencil, Settings2, Trash2 } from 'lucide-react';
import { Button, ConfirmDialog, EmptyState, ErrorState, Field, IconButton, Input, Loading, Menu, Modal, Notice, QueryState, Switch, Tabs, Textarea, cn, useToast } from '@/components/ui';
import { COVER_COLORS, Cover } from '@/features/classrooms/Cover';
import { LessonPicker } from '@/features/materials/LessonPicker';
import { marathonsApi } from '@/lib/api/endpoints';
import { ApiError } from '@/lib/api/client';
import { qk } from '@/lib/queryKeys';
import { useI18n, useT } from '@/lib/i18n';
import { errorMessage, fieldErrors } from '@/lib/errors';
import { lessonEditorUrl, marathonLessonUrl, marathonPreviewUrl } from '@/lib/routes';
import type { Classroom, Locale, MarathonLesson, MarathonLessonLock, MarathonLessons } from '@/types/api';
import { CountLine, LessonStatusChip, PassedBar, SearchField } from './MarathonParts';
import { formatMarathonMoment, fromZonedInputs, toZonedInputs, useServerMoment } from './marathonTime';
import { DateFields, type DateDraft } from './DateFields';
import { LessonResultsModal } from './MarathonStats';
import { AppearanceFields, draftOf, draftValid, type AppearanceDraft } from './appearance/AppearanceFields';
import { ImageField } from './appearance/ImageField';

const matches = (q: string, l: MarathonLesson) => {
  const s = q.trim().toLowerCase();
  return !s || l.title.toLowerCase().includes(s) || l.course_title.toLowerCase().includes(s);
};

function LessonCover({ l }: { l: MarathonLesson }) {
  return <Cover variant="tile" color={l.cover_color} imageId={l.cover_image_id} title={l.title} className="h-16 w-16 shrink-0 rounded-lg" />;
}

/** "Урок" tab of the owner: lessons in order with completion counts; add, create, settings, copy, order, remove. */
export function MarathonLessons({ marathon }: { marathon: Classroom }) {
  const t = useT();
  const qc = useQueryClient();
  const toast = useToast();
  const [q, setQ] = useState('');
  const [adding, setAdding] = useState<null | 'choose' | 'pick' | 'create'>(null);
  const [editing, setEditing] = useState<MarathonLesson | null>(null);
  const [removing, setRemoving] = useState<MarathonLesson | null>(null);
  const [unlocking, setUnlocking] = useState<MarathonLesson | null>(null);
  const [results, setResults] = useState<MarathonLesson | null>(null);
  const { locale } = useI18n();
  const mode = marathon.marathon?.mode ?? 'open';
  const tz = marathon.marathon?.timezone ?? 'Asia/Almaty';
  const key = qk.marathonLessons(marathon.id);
  const lessons = useQuery({ queryKey: key, queryFn: () => marathonsApi.lessons(marathon.id) });

  /** Every change returns the fresh list; counters on the card and in "Ученики" depend on it. */
  const applied = (res: MarathonLessons) => {
    qc.setQueryData(key, res);
    qc.invalidateQueries({ queryKey: qk.classroom(marathon.id) });
    qc.invalidateQueries({ queryKey: qk.marathonStudents(marathon.id) });
    qc.invalidateQueries({ queryKey: ['classrooms'] });
  };
  const onError = (e: unknown) => toast(errorMessage(e, t), 'error');
  const pick = async (lessonId: string) => applied(await marathonsApi.addLesson(marathon.id, lessonId));
  const reorder = useMutation({ mutationFn: (ids: string[]) => marathonsApi.reorderLessons(marathon.id, ids), onSuccess: applied, onError });
  const copy = useMutation({
    mutationFn: (l: MarathonLesson) => marathonsApi.copyLesson(marathon.id, l.lesson_id),
    onSuccess: (res) => { applied(res); toast(t('marathons.lessonCopied'), 'success'); },
    onError,
  });
  const remove = useMutation({
    mutationFn: (l: MarathonLesson) => marathonsApi.removeLesson(marathon.id, l.lesson_id),
    onSuccess: (res) => { applied(res); setRemoving(null); setEditing(null); toast(t('marathons.lessonRemoved'), 'success'); },
    onError,
  });
  const unlock = useMutation({
    mutationFn: (l: MarathonLesson) => marathonsApi.unlockLesson(marathon.id, l.lesson_id),
    onSuccess: (res) => { applied(res); setUnlocking(null); toast(t('marathons.lessonUnlockedAll'), 'success'); },
    onError,
  });
  const busy = reorder.isPending || copy.isPending || remove.isPending;

  return (
    <QueryState query={lessons}>
      {(data) => {
        const shown = data.items.filter((l) => matches(q, l));
        const filtered = q.trim() !== '';
        const move = (i: number, d: -1 | 1) => {
          const ids = data.items.map((l) => l.lesson_id);
          [ids[i], ids[i + d]] = [ids[i + d], ids[i]];
          reorder.mutate(ids);
        };
        return (
          <div>
            <SearchField value={q} onChange={setQ} placeholder={t('common.search')} />
            <CountLine>{t('marathons.lessonsTotal', { n: data.total_lessons })}</CountLine>
            {data.items.length === 0 ? (
              <div className="mt-4 flex min-h-[320px] flex-col items-center justify-center rounded-lg border border-surface-muted px-4 py-10 text-center">
                <span className="text-[64px] leading-none" aria-hidden>📖</span>
                <p className="mt-[22px] max-w-xl text-[20px] leading-7 text-ink-soft">{t('marathons.lessonsEmpty')}</p>
                <Button className="mt-4" onClick={() => setAdding('choose')}>{t('marathons.addLesson')}</Button>
              </div>
            ) : (
              <>
                {shown.length === 0 ? <EmptyState title={t('marathons.noMatches')} className="py-10" /> : (
                  <ol className="mt-4" data-testid="marathon-lessons">
                    {shown.map((l) => {
                      const i = data.items.indexOf(l);
                      return (
                        <li key={l.lesson_id} className="flex flex-wrap items-center gap-4 border-b border-line-soft py-4 first:border-t">
                          <LessonCover l={l} />
                          <div className="min-w-0 flex-1">
                            <p className="truncate text-base font-medium text-ink-body">{l.title}</p>
                            <p className="mt-0.5 truncate text-sm text-ink-soft">{l.course_title}</p>
                            <div className="mt-1.5 flex flex-wrap items-center gap-1.5">
                              <span title={t('marathons.lessonCompletedBy', { n: l.completed_count, m: marathon.students_count })}
                                className="inline-flex h-5 items-center gap-1 rounded bg-surface-muted px-1.5 text-xs font-medium text-ink-soft">
                                <Check size={12} aria-hidden />{l.completed_count}/{marathon.students_count}
                              </span>
                              {l.pending_count > 0 && (
                                <span data-testid="lesson-pending" title={t('marathons.pendingChip', { n: l.pending_count })}
                                  className="inline-flex h-5 items-center gap-1 rounded bg-warning-soft px-1.5 text-xs font-medium text-warning-text">
                                  <Hourglass size={12} aria-hidden />{t('marathons.pendingChip', { n: l.pending_count })}
                                </span>
                              )}
                              {(mode === 'open' || mode === 'progressive') && l.open_at && Date.parse(l.open_at) > Date.now() && (
                                <span data-testid="lesson-opens-at" className="inline-flex h-5 items-center gap-1 rounded bg-primary-soft px-1.5 text-xs font-medium text-primary-text">
                                  <CalendarClock size={12} aria-hidden />{t('marathons.lessonOpensChip', formatMarathonMoment(l.open_at, tz, locale))}
                                </span>
                              )}
                              {mode === 'closed' && (
                                <span data-testid="lesson-unlock-state" className={cn('inline-flex h-5 items-center gap-1 rounded px-1.5 text-xs font-medium',
                                  l.unlocked_for_all ? 'bg-success-soft text-success-text' : 'bg-surface-muted text-ink-soft')}>
                                  {l.unlocked_for_all ? <LockOpen size={12} aria-hidden /> : <Lock size={12} aria-hidden />}
                                  {l.unlocked_for_all ? t('marathons.lessonOpenForAll') : t('marathons.lessonClosedChip')}
                                </span>
                              )}
                            </div>
                          </div>
                          <Link href={marathonPreviewUrl(marathon.id, l.lesson_id)} data-testid="lesson-preview"
                            className="inline-flex h-10 items-center gap-2 rounded-md bg-surface-muted px-4 text-base font-medium text-ink-body transition-colors hover:bg-surface-hover focus-visible:shadow-focus focus-visible:outline-none">
                            <Eye size={16} aria-hidden />{t('marathons.previewButton')}
                          </Link>
                          <Link href={lessonEditorUrl(l.lesson_id, marathon.id)}
                            className="inline-flex h-10 items-center gap-2 rounded-md bg-surface-muted px-4 text-base font-medium text-ink-body transition-colors hover:bg-surface-hover focus-visible:shadow-focus focus-visible:outline-none">
                            <Pencil size={16} aria-hidden />{t('marathons.editLesson')}
                          </Link>
                          <Menu label={t('marathons.lessonMenu', { title: l.title })} items={[
                            { key: 'settings', label: t('marathons.lessonSettings'), icon: <Settings2 size={16} />, onSelect: () => setEditing(l) },
                            { key: 'results', label: t('marathons.lessonResultsTitle'), icon: <BarChart3 size={16} />, onSelect: () => setResults(l) },
                            ...(mode === 'closed' && !l.unlocked_for_all
                              ? [{ key: 'unlock', label: t('marathons.unlockForAll'), icon: <LockOpen size={16} />, onSelect: () => setUnlocking(l) }] : []),
                            { key: 'copy', label: t('marathons.copyLesson'), icon: <Copy size={16} />, disabled: busy, onSelect: () => copy.mutate(l) },
                            { key: 'up', label: t('common.moveUp'), icon: <ArrowUp size={16} />, disabled: busy || filtered || i === 0, onSelect: () => move(i, -1) },
                            { key: 'down', label: t('common.moveDown'), icon: <ArrowDown size={16} />, disabled: busy || filtered || i === data.items.length - 1, onSelect: () => move(i, 1) },
                            { key: 'remove', label: t('marathons.removeLesson'), icon: <Trash2 size={16} />, tone: 'danger', onSelect: () => setRemoving(l) },
                          ]} trigger={(p) => <IconButton {...p} label={t('marathons.lessonMenu', { title: l.title })}><MoreHorizontal size={18} /></IconButton>} />
                        </li>
                      );
                    })}
                  </ol>
                )}
                <div className="mt-4 flex justify-center">
                  <Button onClick={() => setAdding('choose')}>{t('marathons.addLesson')}</Button>
                </div>
              </>
            )}
            <AddLessonChooser open={adding === 'choose'} onClose={() => setAdding(null)} onPick={() => setAdding('pick')} onCreate={() => setAdding('create')} />
            <LessonPicker open={adding === 'pick'} onClose={() => setAdding(null)} classroomId={marathon.id} onOpened={() => {}}
              onPick={pick} pickedIds={data.items.map((l) => l.lesson_id)} />
            {adding === 'create' && (
              <LessonFieldsModal marathon={marathon} onClose={() => setAdding(null)}
                onSaved={(res) => { applied(res); setAdding(null); toast(t('marathons.lessonCreated'), 'success'); }} />
            )}
            {editing && (
              <LessonFieldsModal marathon={marathon} lesson={editing} onClose={() => setEditing(null)} onRemove={() => setRemoving(editing)}
                onSaved={(res) => { applied(res); setEditing(null); toast(t('marathons.saved'), 'success'); }} />
            )}
            {results && <LessonResultsModal marathon={marathon} lesson={results} onClose={() => setResults(null)} />}
            <ConfirmDialog open={!!unlocking} onClose={() => setUnlocking(null)} onConfirm={() => unlocking && unlock.mutate(unlocking)} loading={unlock.isPending}
              title={t('marathons.unlockForAllTitle')} text={unlocking ? t('marathons.unlockForAllText', { title: unlocking.title }) : ''}
              confirmLabel={t('marathons.unlockConfirm')} />
            <ConfirmDialog open={!!removing} onClose={() => setRemoving(null)} onConfirm={() => removing && remove.mutate(removing)} loading={remove.isPending}
              tone="danger" title={t('marathons.removeLessonTitle')} text={removing ? t('marathons.removeLessonText', { title: removing.title }) : ''}
              confirmLabel={t('marathons.removeLesson')} />
          </div>
        );
      }}
    </QueryState>
  );
}

/** "Добавить урок": from personal courses or a new lesson inside the marathon. */
function AddLessonChooser({ open, onClose, onPick, onCreate }: { open: boolean; onClose: () => void; onPick: () => void; onCreate: () => void }) {
  const t = useT();
  const option = 'flex flex-1 flex-col items-center gap-2 rounded-lg border border-line px-4 py-6 text-center text-base font-medium text-primary-text transition-colors hover:bg-primary-soft focus-visible:shadow-focus focus-visible:outline-none';
  return (
    <Modal open={open} onClose={onClose} title={t('marathons.addLesson')} size="sm">
      <p className="mb-4 text-sm text-ink-soft">{t('marathons.addLessonHint')}</p>
      <div className="flex flex-col gap-3 sm:flex-row">
        <button type="button" className={option} onClick={onPick} data-autofocus>
          <FolderOpen size={22} aria-hidden />{t('marathons.addFromCourses')}
        </button>
        <button type="button" className={option} onClick={onCreate}>
          <FilePlus size={22} aria-hidden />{t('marathons.createLesson')}
        </button>
      </div>
    </Modal>
  );
}

type LessonTab = 'main' | 'extra' | 'appearance';
type LessonLookDraft = { enabled: boolean; d: AppearanceDraft };

/** "Создать новый урок" (no lesson) and "Настройки урока" (with a lesson). */
function LessonFieldsModal({ marathon, lesson, onClose, onSaved, onRemove }: {
  marathon: Classroom;
  lesson?: MarathonLesson;
  onClose: () => void;
  onSaved: (res: MarathonLessons) => void;
  onRemove?: () => void;
}) {
  const t = useT();
  const [tab, setTab] = useState<LessonTab>('main');
  const marathonId = marathon.id;
  const tz = marathon.marathon?.timezone ?? 'Asia/Almaty';
  // Н3 = A: the opening date applies in the open and progressive modes (in progressive together with the previous lesson).
  const curMode = marathon.marathon?.mode ?? 'open';
  const openMode = curMode === 'open' || curMode === 'progressive';
  const [v, setV] = useState({ title: lesson?.title ?? '', description: lesson?.description ?? '', cover_color: lesson?.cover_color ?? COVER_COLORS[0] });
  // The cover picture takes effect at once; read it from the live list so the preview follows uploads.
  const live = useQuery({ queryKey: qk.marathonLessons(marathonId), queryFn: () => marathonsApi.lessons(marathonId), enabled: !!lesson });
  const coverImageId = live.data?.items.find((x) => x.lesson_id === lesson?.lesson_id)?.cover_image_id ?? lesson?.cover_image_id ?? null;
  // «Оформление урока» (stage 10): the lesson's own appearance in this marathon.
  const lookQ = useQuery({ queryKey: qk.marathonLessonAppearance(marathonId, lesson?.lesson_id ?? ''), queryFn: () => marathonsApi.lessonAppearance(marathonId, lesson!.lesson_id), enabled: !!lesson });
  const marathonLook = useQuery({ queryKey: qk.marathonAppearance(marathonId), queryFn: () => marathonsApi.appearance(marathonId), enabled: !!lesson });
  const [look, setLook] = useState<LessonLookDraft | null>(null);
  useEffect(() => {
    if (lookQ.data && !look) setLook({ enabled: lookQ.data.enabled, d: draftOf(lookQ.data) });
  }, [lookQ.data, look]);
  const setLookPart = useCallback((d: AppearanceDraft) => setLook((x) => (x ? { ...x, d } : x)), []);
  const lookChanged = !!look && !!lookQ.data && JSON.stringify(look) !== JSON.stringify({ enabled: lookQ.data.enabled, d: draftOf(lookQ.data) });
  const lookValid = !look || draftValid(look.d, !!lookQ.data?.background_image_id);
  const [openDate, setOpenDate] = useState<DateDraft>(() => (lesson?.open_at
    ? { on: true, ...toZonedInputs(lesson.open_at, tz) }
    : { on: false, ...toZonedInputs(new Date(Date.now() + 864e5).toISOString(), tz), time: '10:00' }));
  const openIso = openDate.on && openDate.date && openDate.time ? fromZonedInputs(openDate.date, openDate.time, tz) : null;
  const openChanged = !!lesson && openIso !== (lesson.open_at ? new Date(lesson.open_at).toISOString() : null);
  const save = useMutation({
    mutationFn: async () => {
      if (!lesson) return marathonsApi.createLesson(marathonId, v);
      let res = await marathonsApi.updateLesson(marathonId, lesson.lesson_id, v);
      if (lookChanged && look) await marathonsApi.setLessonAppearance(marathonId, lesson.lesson_id, { enabled: look.enabled, ...look.d });
      if (openChanged) res = await marathonsApi.setLessonOpenAt(marathonId, lesson.lesson_id, openIso);
      return res;
    },
    onSuccess: onSaved,
  });
  const errs = fieldErrors(save.error);
  const general = save.error && Object.keys(errs).length === 0 ? errorMessage(save.error, t) : null;
  const ready = v.title.trim() !== '' && (!openDate.on || !!openIso) && lookValid;
  const submit = () => { if (ready && !save.isPending) save.mutate(); };

  return (
    <Modal open onClose={onClose} title={lesson ? t('marathons.lessonSettings') : t('marathons.createLesson')}
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={save.isPending}>{t('common.cancel')}</Button>
          <Button onClick={submit} disabled={!ready} loading={save.isPending}>{lesson ? t('common.save') : t('common.create')}</Button>
        </>
      }>
      {lesson && (
        <div className="mb-4 border-b border-line">
          <Tabs<LessonTab> value={tab} onChange={setTab} items={[{ value: 'main', label: t('marathons.lessonTabMain') }, { value: 'extra', label: t('marathons.lessonTabExtra') }, { value: 'appearance', label: t('marathons.tabAppearance') }]} />
        </div>
      )}
      {tab === 'main' ? (
        <form className="flex flex-col gap-4" onSubmit={(e) => { e.preventDefault(); submit(); }}>
          <fieldset>
            <legend className="mb-1.5 text-sm font-medium text-ink-body">{t('marathons.cover')}</legend>
            <div className="flex flex-wrap items-center gap-3">
              <Cover variant="tile" color={v.cover_color} imageId={coverImageId} title={v.title || t('marathons.lessonTitle')} className="h-10 w-10 rounded-lg [&>span]:text-lg" />
              <div className="flex flex-wrap gap-2">
                {COVER_COLORS.map((c) => (
                  <button key={c} type="button" onClick={() => setV({ ...v, cover_color: c })} aria-label={t('marathons.coverColor', { color: c })} aria-pressed={c === v.cover_color}
                    className={cn('h-8 w-8 rounded-full ring-offset-2 transition', c === v.cover_color && 'ring-2 ring-ink')} style={{ background: c }} />
                ))}
              </div>
            </div>
            <p className="mt-1.5 text-xs text-ink-muted">{t('marathons.lessonCoverHint')}</p>
            {lesson
              ? <div className="mt-3"><ImageField marathonId={marathonId} lessonId={lesson.lesson_id} slot="cover" imageId={coverImageId} testId="lesson-cover-image" /></div>
              : <p className="mt-1 text-xs text-ink-muted">{t('marathons.look.lessonImageLater')}</p>}
            {errs.cover_color && <p role="alert" className="mt-1 text-sm text-danger-text">{errs.cover_color}</p>}
          </fieldset>
          <Field label={t('marathons.lessonTitle')} error={errs.title} required>
            {(p) => <Input {...p} data-autofocus value={v.title} onChange={(e) => setV({ ...v, title: e.target.value })} placeholder={t('marathons.lessonTitlePlaceholder')} maxLength={200} />}
          </Field>
          <Field label={t('marathons.lessonDescription')} error={errs.description}>
            {(p) => <Textarea {...p} rows={4} value={v.description} onChange={(e) => setV({ ...v, description: e.target.value })} placeholder={t('marathons.lessonDescriptionPlaceholder')} maxLength={2000} />}
          </Field>
          {lesson && <Notice tone="info">{t('marathons.lessonTemplateNotice')}</Notice>}
          {general && <p role="alert" className="text-sm text-danger-text">{general}</p>}
          {lesson && onRemove && (
            <div className="flex flex-wrap items-center justify-between gap-3 border-t border-line pt-4">
              <span className="text-sm text-ink-body">{t('marathons.removeLessonLabel')}</span>
              <Button variant="danger" size="sm" icon={<Trash2 size={16} />} onClick={onRemove}>{t('marathons.removeLesson')}</Button>
            </div>
          )}
          <button type="submit" hidden aria-hidden tabIndex={-1} />
        </form>
      ) : tab === 'appearance' ? (
        !look ? (lookQ.error ? <ErrorState error={lookQ.error} onRetry={() => lookQ.refetch()} /> : <Loading />) : (
          <div className="flex flex-col" data-testid="lesson-look">
            {marathonLook.data && !marathonLook.data.lesson_appearance && (
              <Notice tone="warning" className="mb-2 text-sm">{t('marathons.look.lessonAppearanceOffNotice')}</Notice>
            )}
            <div className="border-b border-line-soft pb-4">
              <Switch checked={look.enabled} onChange={(x) => setLook({ ...look, enabled: x })}
                label={t('marathons.look.lessonOwn')} hint={t('marathons.look.lessonOwnHint')} />
            </div>
            {look.enabled && (
              <AppearanceFields value={look.d} onChange={setLookPart} marathonId={marathonId} lessonId={lesson!.lesson_id}
                backgroundImageId={lookQ.data?.background_image_id ?? null} idPrefix="lesson" />
            )}
            {general && <p role="alert" className="text-sm text-danger-text">{general}</p>}
          </div>
        )
      ) : openMode ? (
        <div className="flex flex-col gap-3">
          <DateFields kind="lesson" value={openDate} onChange={setOpenDate} timezone={tz} iso={openIso} error={errs.open_at} />
          {general && <p role="alert" className="text-sm text-danger-text">{general}</p>}
        </div>
      ) : (
        <div className="flex items-center justify-between gap-3 py-2">
          <span className="text-sm font-medium text-ink-body">{t('marathons.lessonOpenDate')}</span>
          <span className="text-sm text-ink-muted">{t('marathons.lessonOpenDateNotUsed')}</span>
        </div>
      )}
    </Modal>
  );
}

/**
 * Student's own view of the marathon, from GET /me: the server's access decision and the lessons.
 * open → lessons can be opened; not_started → listed with a lock until the start; deactivated →
 * by the end date or the individual period: listed read-only; by the teacher: no lessons at all.
 * The server checks every action again; the page only explains its decision.
 */
export function StudentMarathon({ marathon }: { marathon: Classroom }) {
  const { t, locale } = useI18n();
  const qc = useQueryClient();
  const me = useQuery({
    queryKey: qk.marathonMe(marathon.id), queryFn: () => marathonsApi.me(marathon.id),
    retry: (n, e) => !(e instanceof ApiError && e.status < 500) && n < 2,
  });
  const tz = marathon.marathon?.timezone ?? 'Asia/Almaty';
  const acc = me.data?.access;
  // Re-read when the server clock reaches the start or the student's deadline.
  const reread = () => qc.invalidateQueries({ queryKey: ['marathon', marathon.id] });
  useServerMoment(acc?.status === 'not_started' ? acc.opens_at : null, me.data?.server_now, reread);
  useServerMoment(acc?.status === 'open' ? acc.deactivates_at : null, me.data?.server_now, reread);
  // A lesson with an opening date or a schedule opens by itself: re-read at the earliest such moment (server clock).
  const nextOpening = acc?.status === 'open'
    ? me.data?.items.map((l) => l.lock?.opens_at).filter((x): x is string => !!x).sort()[0] ?? null : null;
  useServerMoment(nextOpening, me.data?.server_now, reread);

  return (
    <QueryState query={me}>
      {(data) => {
        const a = data.access;
        if (a.status === 'deactivated' && a.reason === 'manual') {
          return (
            <div data-testid="student-deactivated">
              <EmptyState icon={<Lock size={26} />} title={t('marathons.deactivatedStudentTitle')} text={t('marathons.deactivatedStudentText')} className="min-h-[320px]" />
            </div>
          );
        }
        const open = a.status === 'open';
        let notice: ReactNode = null;
        // Before the start the header banner (Марафон начнётся…) already says when lessons open.
        if (a.status === 'deactivated' && a.reason === 'auto_deactivation' && a.deactivates_at) {
          notice = <Notice tone="danger" icon={<AlertCircle size={20} />} className="mb-4"><span data-testid="student-access">{t('marathons.studentAutoExpired', formatMarathonMoment(a.deactivates_at, tz, locale))}</span></Notice>;
        } else if (open && a.deactivation_reason === 'auto_deactivation' && a.deactivates_at) {
          notice = <Notice tone="info" icon={<Info size={20} />} className="mb-4"><span data-testid="student-access">{t('marathons.studentAutoUntil', formatMarathonMoment(a.deactivates_at, tz, locale))}</span></Notice>;
        }
        const lockTitle = a.status === 'not_started' ? t('marathons.notStartedLesson') : t('marathons.closedLesson');
        const lockText = (lock: MarathonLessonLock) => lessonLockText(t, lock, tz, locale);
        return (
          <div data-access={a.status}>
            {notice}
            <div className="flex">
              <PassedBar done={data.completed_lessons} total={data.total_lessons} muted={!open} />
            </div>
            <CountLine>{t('marathons.lessonsTotal', { n: data.total_lessons })}</CountLine>
            {data.items.length === 0 ? (
              <div className="flex flex-col items-center px-4 py-10 text-center">
                <span className="text-[64px] leading-none" aria-hidden>📖</span>
                <p className="mt-[22px] max-w-xl text-[20px] leading-7 text-ink-soft">{t('marathons.noLessonsStudent')}</p>
              </div>
            ) : (
              <ol className="mt-4" data-testid="student-lessons">
                {data.items.map((l) => (
                  <li key={l.lesson_id} className="flex flex-wrap items-center gap-4 border-b border-line-soft py-4 first:border-t">
                    <LessonCover l={l} />
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-base font-medium text-ink-body">{l.title}</p>
                      <LessonStatusChip status={l.status} className="mt-1.5" />
                      {l.access === 'locked' && l.lock && (
                        <p className="mt-1 text-xs text-ink-soft" data-testid="lesson-lock">{lockText(l.lock)}</p>
                      )}
                    </div>
                    {l.access !== 'open' ? (
                      <Button variant="secondary" disabled icon={<Lock size={16} />} title={l.lock ? lockText(l.lock) : lockTitle}>{t('marathons.openLesson')}</Button>
                    ) : (
                      <Link href={marathonLessonUrl(marathon.id, l.lesson_id)}
                        className="inline-flex h-10 items-center gap-2 rounded-md bg-primary px-4 text-base font-medium text-white transition-colors hover:bg-primary-hover focus-visible:shadow-focus focus-visible:outline-none">
                        <ArrowRight size={18} aria-hidden />{t('marathons.openLesson')}
                      </Link>
                    )}
                  </li>
                ))}
              </ol>
            )}
          </div>
        );
      }}
    </QueryState>
  );
}

/** Why the passage mode keeps a lesson closed, in words (shared by the student list and the player). */
export function lessonLockText(t: (k: string, v?: Record<string, string | number>) => string, lock: MarathonLessonLock, tz: string, locale: Locale) {
  if ((lock.reason === 'opens_at' || lock.reason === 'scheduled') && lock.opens_at) return t('marathons.lockOpensAt', formatMarathonMoment(lock.opens_at, tz, locale));
  if (lock.reason === 'previous_not_completed') return t('marathons.lockPrevious');
  if (lock.reason === 'previous_not_reviewed') return t('marathons.lockPreviousReview');
  if (lock.reason === 'previous_not_submitted') return t('marathons.lockPreviousSubmission');
  if (lock.reason === 'not_unlocked') return t('marathons.lockNotUnlocked');
  return t('marathons.closedLesson');
}

'use client';

import { useCallback, useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { useRouter, useSearchParams } from 'next/navigation';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, CalendarDays, CalendarRange, ClipboardList, Footprints, Lock, LockOpen, Settings, Users } from 'lucide-react';
import { Button, ConfirmDialog, EmptyState, ErrorState, Field, Input, Loading, Modal, Notice, Select, Switch, Tabs, cn, useToast } from '@/components/ui';
import { PageCard } from '@/components/layout/PageCard';
import { Cover } from '@/features/classrooms/Cover';
import { classroomsApi, marathonsApi } from '@/lib/api/endpoints';
import { ApiError } from '@/lib/api/client';
import { qk } from '@/lib/queryKeys';
import { useI18n, useT } from '@/lib/i18n';
import { errorMessage, fieldErrors } from '@/lib/errors';
import { classesUrl, classUrl, marathonUrl, type MarathonTab } from '@/lib/routes';
import type { MarathonAppearance, Classroom, MarathonInfo, MarathonMode, MarathonSaveRequest, MarathonUnlockAfter } from '@/types/api';
import { MarathonForm, marathonReady } from './MarathonForm';
import { MarathonDatesBanner } from './MarathonParts';
import { MarathonStudents } from './MarathonStudents';
import { MarathonLessons, StudentMarathon } from './MarathonLessons';
import { MarathonStatistics, StudentStatsBlock } from './MarathonStats';
import { useMarathonLive } from './useMarathonLive';
import { formatMarathonMoment, fromZonedInputs, toZonedInputs } from './marathonTime';
import { DateFields, type DateDraft } from './DateFields';
import { AppearanceFields, draftOf, draftValid, type AppearanceDraft } from './appearance/AppearanceFields';
import { ImageField } from './appearance/ImageField';

/** Marathon page inside the cabinet shell: header, tabs (students / lessons / statistics). */
export function MarathonPage({ id, tab }: { id: string; tab: MarathonTab }) {
  const t = useT();
  const router = useRouter();
  const q = useQuery({ queryKey: qk.classroom(id), queryFn: () => classroomsApi.get(id), retry: (n, e) => !(e instanceof ApiError && e.status === 404) && n < 2 });
  const [settings, setSettings] = useState<false | SettingsSection>(false);
  // Right after «Создать» the settings open by themselves (?setup=1) with «Настроить позже» instead of «Отмена».
  const [setup, setSetup] = useState(false);
  const setupParam = useSearchParams().get('setup') === '1';
  const m = q.data;
  const wrongType = !!m && m.lesson_type !== 'marathon';
  useMarathonLive(m && !wrongType ? m : undefined);
  useEffect(() => {
    // A regular class opened by a marathon URL goes to its own page.
    if (m && wrongType) router.replace(classUrl(m));
  }, [m, wrongType, router]);
  useEffect(() => {
    if (!setupParam || !m || wrongType) return;
    // The flag is consumed once: a reload or «Назад» does not open the dialog again.
    router.replace(marathonUrl(id, tab));
    if (m.my_role === 'teacher') {
      setSetup(true);
      setSettings('passage');
    }
  }, [setupParam, m, wrongType, router, id, tab]);

  if (q.isLoading) return <PageCard maxWidth={1024}><Loading /></PageCard>;
  if (q.error) {
    const notFound = q.error instanceof ApiError && q.error.status === 404;
    return (
      <PageCard maxWidth={1024}>
        {notFound
          ? <EmptyState title={t('marathons.notFound')} action={<Link href={classesUrl({ type: 'marathon' })} className="text-sm font-medium text-primary-text hover:underline">{t('marathons.back')}</Link>} />
          : <ErrorState error={q.error} onRetry={() => q.refetch()} />}
      </PageCard>
    );
  }
  if (!m || wrongType) return null;
  const teacher = m.my_role === 'teacher';
  const tabs: { value: MarathonTab; label: string }[] = [
    { value: 'students', label: t('marathons.tabStudents') },
    { value: 'lessons', label: t('marathons.tabLessons') },
    { value: 'statistics', label: t('marathons.tabStatistics') },
  ];

  return (
    <PageCard maxWidth={1024} className="pt-[18px] sm:pt-8 md:min-h-[calc(100dvh-64px-32px)]">
      <header className="flex flex-wrap items-start gap-4">
        <Link href={classesUrl({ type: 'marathon' })} aria-label={t('marathons.back')} title={t('marathons.back')}
          className="flex h-[120px] w-10 shrink-0 items-center justify-center rounded-lg bg-surface-muted text-ink-muted transition-colors hover:text-ink focus-visible:shadow-focus focus-visible:outline-none max-sm:h-10">
          <ArrowLeft size={20} />
        </Link>
        <Cover variant="tile" color={m.cover_color} imageId={m.cover_image_id} title={m.name} className="h-[120px] w-[120px] shrink-0 rounded-lg" />
        <div className="min-w-0 flex-1">
          <h1 className="break-words text-[30px] font-bold leading-9 text-ink-body">{m.name}</h1>
          <div className="mt-1 flex flex-wrap items-center gap-1.5">
            <Chip icon={<Users size={12} aria-hidden />} label={t('marathons.studentsCount', { n: m.students_count })} value={m.students_count} active={m.students_count > 0} />
            <Chip icon={<ClipboardList size={12} aria-hidden />} label={t('marathons.lessonsCount', { n: m.lessons_count })} value={m.lessons_count} active={m.lessons_count > 0} />
            {m.marathon && (
              <span className="inline-flex h-5 items-center gap-1 rounded-full border border-line-soft px-1.5 text-xs font-medium text-ink-soft">
                <Footprints size={11} aria-hidden />{t(`marathons.mode.${m.marathon.mode}`)}
              </span>
            )}
            {m.marathon?.target_language && (
              <span className="inline-flex h-5 items-center rounded-full border border-line-soft px-1.5 text-xs font-medium text-ink-soft">{t(`languages.${m.marathon.target_language}`)}</span>
            )}
          </div>
          {teacher && (
            <div className="mt-4">
              <Button variant="secondary" icon={<Settings size={18} />} onClick={() => setSettings('main')}>{t('marathons.settings')}</Button>
            </div>
          )}
        </div>
      </header>

      {m.marathon && <MarathonDatesBanner info={m.marathon} className="mt-4" />}

      {teacher ? (
        <>
          <Tabs className="mt-8 border-b border-line-soft" size="lg" value={tab} items={tabs} onChange={(v) => router.push(marathonUrl(id, v))} idPrefix="marathon" />
          <section role="tabpanel" aria-labelledby={`marathon-tab-${tab}`} className="mt-4">
            {tab === 'students' ? <MarathonStudents marathon={m} /> : tab === 'lessons' ? <MarathonLessons marathon={m} />
              : <MarathonStatistics marathon={m} onOpenSettings={() => setSettings('extra')} />}
          </section>
        </>
      ) : (
        <section className="mt-8" aria-label={t('marathons.tabLessons')}>
          <StudentMarathon marathon={m} />
          <StudentStatsBlock marathon={m} />
        </section>
      )}

      {teacher && <MarathonSettingsModal open={!!settings} initialSection={settings || 'main'} setup={setup}
        onClose={() => { setSettings(false); setSetup(false); }} marathon={m} />}
    </PageCard>
  );
}

function Chip({ icon, label, value, active }: { icon: React.ReactNode; label: string; value: number; active: boolean }) {
  return (
    <span title={label} aria-label={label} role="img"
      className={active ? 'inline-flex h-5 items-center gap-0.5 rounded bg-success-soft px-1 text-xs font-medium text-success' : 'inline-flex h-5 items-center gap-0.5 rounded bg-surface-muted px-1 text-xs font-medium text-ink-soft'}>
      {icon}{value}
    </span>
  );
}

/**
 * «Настройки марафона». With `setup` it is the dialog shown right after creation: without «Основные» (just filled in),
 * starting on «Прохождение», and «Настроить позже» closes it keeping the defaults (mode «Открытый»).
 */
function MarathonSettingsModal({ open, onClose, marathon, initialSection, setup = false }: {
  open: boolean; onClose: () => void; marathon: Classroom; initialSection: SettingsSection; setup?: boolean;
}) {
  const t = useT();
  const qc = useQueryClient();
  const router = useRouter();
  const toast = useToast();
  const initial = (): MarathonSaveRequest => ({
    name: marathon.name,
    cover_color: marathon.cover_color,
    direction: marathon.marathon?.direction ?? 'languages',
    target_language: marathon.marathon?.target_language ?? null,
  });
  const tz = marathon.marathon?.timezone ?? 'Asia/Almaty';
  const initialDate = (at: string | null | undefined, daysAhead: number, time: string): DateDraft => {
    if (at) return { on: true, ...toZonedInputs(at, tz) };
    // Default suggestion when switched on, in the marathon zone.
    const d = toZonedInputs(new Date(Date.now() + daysAhead * 864e5).toISOString(), tz);
    return { on: false, date: d.date, time };
  };
  const initialEnd = () => initialDate(marathon.marathon?.end_at, 7, '17:00');
  const initialStart = () => initialDate(marathon.marathon?.start_at, 1, '10:00');
  const initialAuto = (): AutoDraft => {
    const n = marathon.marathon?.auto_deactivate_days;
    return { on: n != null, days: String(n ?? 30) };
  };
  const initialReview = (): ReviewDraft => ({ on: !!marathon.marathon?.teacher_check, unlock: marathon.marathon?.unlock_after ?? 'teacher_check' });
  const [value, setValue] = useState(initial);
  const [review, setReview] = useState(initialReview);
  const [end, setEnd] = useState(initialEnd);
  const [start, setStart] = useState(initialStart);
  const [auto, setAuto] = useState(initialAuto);
  const [section, setSection] = useState<SettingsSection>(initialSection);
  const initialStats = (): StatsDraft => ({
    stats: !!marathon.marathon?.stats_enabled, rating: !!marathon.marathon?.rating_enabled,
    limit: marathon.marathon?.rating_limit ?? null, showPlace: !!marathon.marathon?.rating_show_place,
  });
  const [statsDraft, setStatsDraft] = useState(initialStats);
  // «Оформление» (stage 10): read when the dialog opens; the draft starts from the server values.
  const look = useQuery({ queryKey: qk.marathonAppearance(marathon.id), queryFn: () => marathonsApi.appearance(marathon.id), enabled: open });
  const [lookDraft, setLookDraft] = useState<LookDraft | null>(null);
  useEffect(() => {
    if (open && look.data && !lookDraft) setLookDraft(lookDraftOf(look.data));
  }, [open, look.data, lookDraft]);
  const setLookPart = useCallback((d: AppearanceDraft) => setLookDraft((x) => (x ? { ...x, d } : x)), []);
  const [confirmDelete, setConfirmDelete] = useState(false);
  // A mode change asks first about the progress reset; when lessons have opening dates that the new mode does
  // not use, the server answers lesson_dates_set and a second dialog asks to reset the dates (Н3 = A).
  const [pendingMode, setPendingMode] = useState<ModeChoice | null>(null);
  const [datesConfirm, setDatesConfirm] = useState<ModeChoice | null>(null);
  const changeMode = useMutation({
    mutationFn: ({ choice, resetDates }: { choice: ModeChoice; resetDates?: boolean }) => marathonsApi.changeMode(marathon.id, {
      mode: choice.mode, expected_epoch: marathon.marathon?.progress_epoch ?? 1, confirm_reset: true, confirm_reset_dates: !!resetDates, ...choice.schedule,
    }),
    onSuccess: (m, { choice }) => {
      qc.setQueryData(qk.classroom(m.id), m);
      qc.invalidateQueries({ queryKey: ['classrooms'] });
      qc.invalidateQueries({ queryKey: ['marathon', m.id] });
      setPendingMode(null);
      setDatesConfirm(null);
      toast(t(choice.mode === marathon.marathon?.mode ? 'marathons.scheduleSaved' : 'marathons.modeChanged'), 'success');
    },
    onError: (e, { choice }) => {
      setPendingMode(null);
      if (e instanceof ApiError && e.code === 'lesson_dates_set') setDatesConfirm(choice);
      else setDatesConfirm(null);
    },
  });
  const [lastOpen, setLastOpen] = useState(open);
  if (open !== lastOpen) {
    // Re-read the saved values every time the dialog opens.
    setLastOpen(open);
    if (open) {
      setValue(initial());
      setEnd(initialEnd());
      setStart(initialStart());
      setAuto(initialAuto());
      setReview(initialReview());
      setStatsDraft(initialStats());
      setLookDraft(null);
      setSection(initialSection);
    }
  }
  const endIso = end.on && end.date && end.time ? fromZonedInputs(end.date, end.time, tz) : null;
  const startIso = start.on && start.date && start.time ? fromZonedInputs(start.date, start.time, tz) : null;
  const autoDays = auto.on ? Number(auto.days) : null;
  const iso = (v: string | null | undefined) => (v ? new Date(v).toISOString() : null);
  const passageChanged = endIso !== iso(marathon.marathon?.end_at) || startIso !== iso(marathon.marathon?.start_at)
    || autoDays !== (marathon.marathon?.auto_deactivate_days ?? null);
  const reviewChanged = review.on !== !!marathon.marathon?.teacher_check || review.unlock !== (marathon.marathon?.unlock_after ?? 'teacher_check');
  const statsChanged = JSON.stringify(statsDraft) !== JSON.stringify(initialStats());
  const lookChanged = !!lookDraft && !!look.data && JSON.stringify(lookDraft) !== JSON.stringify(lookDraftOf(look.data));
  const lookValid = !lookDraft || draftValid(lookDraft.d, !!look.data?.appearance.background_image_id);
  const save = useMutation({
    mutationFn: async () => {
      let saved = await marathonsApi.update(marathon.id, value);
      if (passageChanged) saved = await marathonsApi.setPassage(marathon.id, { start_at: startIso, end_at: endIso, auto_deactivate_days: autoDays });
      if (reviewChanged) saved = await marathonsApi.setReviewSettings(marathon.id, { teacher_check: review.on, unlock_after: review.unlock });
      if (statsChanged) {
        saved = await marathonsApi.setStatsSettings(marathon.id, {
          stats_enabled: statsDraft.stats, rating_enabled: statsDraft.stats && statsDraft.rating, rating_limit: statsDraft.limit, rating_show_place: statsDraft.showPlace,
        });
      }
      if (lookChanged && lookDraft) await marathonsApi.setAppearance(marathon.id, { lesson_appearance: lookDraft.lesson, ...lookDraft.d });
      return saved;
    },
    onSuccess: (m) => {
      qc.setQueryData(qk.classroom(m.id), m);
      qc.invalidateQueries({ queryKey: ['classrooms'] });
      qc.invalidateQueries({ queryKey: ['marathon', m.id] });
      toast(t('marathons.saved'), 'success');
      onClose();
    },
  });
  const archive = useMutation({
    mutationFn: () => marathonsApi.archive(marathon.id),
    onSuccess: () => {
      qc.removeQueries({ queryKey: qk.classroom(marathon.id) });
      qc.invalidateQueries({ queryKey: ['classrooms'] });
      toast(t('marathons.deleted'), 'success');
      router.replace(classesUrl({ type: 'marathon' }));
    },
  });
  const errs = fieldErrors(save.error);
  const orderError = startIso && endIso && Date.parse(endIso) <= Date.parse(startIso) ? t('marathons.endBeforeStart') : undefined;
  const autoValid = !auto.on || (Number.isInteger(autoDays) && autoDays! >= 1 && autoDays! <= 3650);
  const endValid = (!end.on || !!endIso) && (!start.on || !!startIso) && !orderError && autoValid;
  const submit = () => marathonReady(value) && endValid && lookValid && !save.isPending && save.mutate();

  return (
    <>
      <Modal open={open && !confirmDelete && !pendingMode && !datesConfirm} onClose={onClose} title={t('marathons.settingsTitle')}
        footer={<>
          <Button variant="secondary" onClick={onClose}>{setup ? t('marathons.setupLater') : t('common.cancel')}</Button>
          <Button onClick={submit} loading={save.isPending} disabled={!marathonReady(value) || !endValid || !lookValid}>{t('common.save')}</Button>
        </>}>
        <Tabs value={section} onChange={setSection} idPrefix="marathon-settings" className="mb-4 border-b border-line-soft"
          items={[...(setup ? [] : [{ value: 'main' as const, label: t('marathons.tabMain') }]), { value: 'passage', label: t('marathons.tabPassage') }, { value: 'extra', label: t('marathons.tabExtra') }, { value: 'appearance', label: t('marathons.tabAppearance') }]} />
        {section === 'main' ? (
          <>
            <MarathonForm value={value} onChange={setValue} errors={errs} onSubmit={submit} coverImageId={marathon.cover_image_id}
              coverImage={<ImageField marathonId={marathon.id} slot="cover" imageId={marathon.cover_image_id} testId="marathon-cover-image" />} />
            <div className="mt-4 flex flex-wrap items-center justify-between gap-3 border-t border-line-soft pt-4">
              <span className="text-sm font-medium text-ink-body">{t('marathons.delete')}</span>
              <Button variant="danger" onClick={() => setConfirmDelete(true)}>{t('common.delete')}</Button>
            </div>
          </>
        ) : section === 'extra' ? (
          <StatsFields value={statsDraft} onChange={setStatsDraft} />
        ) : section === 'appearance' ? (
          !lookDraft ? (look.error ? <ErrorState error={look.error} onRetry={() => look.refetch()} /> : <Loading />) : (
            <div className="flex flex-col" data-testid="marathon-look">
              {lookDraft.lesson && <Notice tone="info" className="mb-2 text-sm">{t('marathons.look.lessonAppearanceOnNotice')}</Notice>}
              <div className="border-b border-line-soft pb-4">
                <Switch checked={lookDraft.lesson} onChange={(v) => setLookDraft({ ...lookDraft, lesson: v })}
                  label={t('marathons.look.lessonAppearance')} hint={t('marathons.look.lessonAppearanceHint')} />
              </div>
              <AppearanceFields value={lookDraft.d} onChange={setLookPart} marathonId={marathon.id}
                backgroundImageId={look.data?.appearance.background_image_id ?? null} idPrefix="marathon" />
            </div>
          )
        ) : (
          <div className="flex flex-col gap-6">
            <ModeField info={marathon.marathon} busy={changeMode.isPending}
              onPick={(choice) => { changeMode.reset(); setPendingMode(choice); }}
              onSaveSchedule={(choice) => changeMode.mutate({ choice })} />
            {changeMode.error && !pendingMode && !datesConfirm && <p role="alert" className="-mt-4 text-sm text-danger-text">{errorMessage(changeMode.error, t)}</p>}
            <ReviewFields value={review} onChange={setReview} progressive={marathon.marathon?.mode === 'progressive'} />
            <DateFields kind="start" value={start} onChange={setStart} timezone={tz} iso={startIso} error={errs.start_at} />
            <DateFields kind="end" value={end} onChange={setEnd} timezone={tz} iso={endIso} error={errs.end_at ?? orderError} />
            <AutoDeactivationFields value={auto} onChange={setAuto} error={errs.auto_deactivate_days ?? (autoValid ? undefined : t('marathons.autoDaysInvalid'))} />
          </div>
        )}
        {save.error && !Object.keys(errs).length && <p role="alert" className="mt-3 text-sm text-danger-text">{errorMessage(save.error, t)}</p>}
      </Modal>
      <ConfirmDialog open={open && !!pendingMode} onClose={() => setPendingMode(null)} onConfirm={() => pendingMode && changeMode.mutate({ choice: pendingMode })}
        loading={changeMode.isPending} title={t('marathons.modeChangeTitle')} text={t('marathons.modeChangeText')} confirmLabel={t('marathons.modeChangeConfirm')}>
        {pendingMode && <p className="mt-3 text-sm text-ink-soft">{t('marathons.modeChangeTo', { mode: t(`marathons.mode.${pendingMode.mode}`) })}</p>}
      </ConfirmDialog>
      <ConfirmDialog open={open && !!datesConfirm} onClose={() => setDatesConfirm(null)} onConfirm={() => datesConfirm && changeMode.mutate({ choice: datesConfirm, resetDates: true })}
        loading={changeMode.isPending} tone="danger" title={t('marathons.datesResetTitle')} text={t('marathons.datesResetText')} confirmLabel={t('marathons.datesResetConfirm')}>
        <Notice tone="info" className="mt-3 text-sm">{t('marathons.datesResetNotice')}</Notice>
      </ConfirmDialog>
      <ConfirmDialog open={open && confirmDelete} onClose={() => setConfirmDelete(false)} onConfirm={() => archive.mutate()} loading={archive.isPending}
        tone="danger" title={t('marathons.deleteConfirmTitle')} text={t('marathons.deleteConfirmText', { name: marathon.name })} confirmLabel={t('common.delete')}>
        {archive.error && <p role="alert" className="mt-3 text-sm text-danger-text">{errorMessage(archive.error, t)}</p>}
      </ConfirmDialog>
    </>
  );
}

const MODES: { mode: MarathonMode; icon: ReactNode }[] = [
  { mode: 'open', icon: <LockOpen size={20} /> },
  { mode: 'interval', icon: <CalendarRange size={20} /> },
  { mode: 'progressive', icon: <Footprints size={20} /> },
  { mode: 'closed', icon: <Lock size={20} /> },
  { mode: 'weekdays', icon: <CalendarDays size={20} /> },
];
const SCHEDULED: MarathonMode[] = ['interval', 'weekdays'];
const HOURS = Array.from({ length: 24 }, (_, h) => `${String(h).padStart(2, '0')}:00`);

interface Schedule { interval_days?: number; weekdays?: number[]; open_time?: string }
interface ModeChoice { mode: MarathonMode; schedule: Schedule }

const scheduleOf = (info: MarathonInfo | null | undefined, mode: MarathonMode): Schedule => {
  if (mode === 'interval') return { interval_days: info?.interval_days ?? 1, open_time: info?.open_time ?? '09:00' };
  if (mode === 'weekdays') return { weekdays: info?.weekdays?.length ? info.weekdays : [], open_time: info?.open_time ?? '09:00' };
  return {};
};
const sameSchedule = (a: Schedule, b: Schedule) => JSON.stringify(a) === JSON.stringify(b);
const scheduleReady = (mode: MarathonMode, sc: Schedule) =>
  mode === 'interval' ? !!sc.interval_days && !!sc.open_time : mode === 'weekdays' ? !!sc.weekdays?.length && !!sc.open_time : true;

/**
 * "Способ прохождения": the five modes of the original with their descriptions. Picking open / progressive /
 * closed asks for confirmation at once (lesson statuses start over). "По интервалам" and "По дням недели" first
 * show their schedule ("Раз в N дней" or weekdays, "в Время" by whole hours in the marathon zone); the student's
 * schedule counts from max(start_at, the day they were added) and lesson 1 opens at that moment (Н1 = A).
 * Changing only the schedule of the current mode keeps the statuses.
 */
function ModeField({ info, busy, onPick, onSaveSchedule }: {
  info: MarathonInfo | null | undefined; busy: boolean; onPick: (c: ModeChoice) => void; onSaveSchedule: (c: ModeChoice) => void;
}) {
  const { t, locale } = useI18n();
  const zone = formatMarathonMoment(new Date().toISOString(), info?.timezone ?? 'Asia/Almaty', locale).zone;
  const current = info?.mode ?? 'open';
  const [draft, setDraft] = useState<MarathonMode>(current);
  const [sc, setSc] = useState<Schedule>(() => scheduleOf(info, current));
  const choose = (m: MarathonMode) => {
    if (SCHEDULED.includes(m)) {
      setDraft(m);
      setSc(scheduleOf(info?.mode === m ? info : null, m));
    } else if (m !== current) {
      setDraft(current);
      onPick({ mode: m, schedule: {} });
    } else setDraft(m);
  };
  const scheduled = SCHEDULED.includes(draft);
  const changed = draft !== current;
  const toggleDay = (d: number) => setSc((v) => {
    const days = v.weekdays ?? [];
    return { ...v, weekdays: days.includes(d) ? days.filter((x) => x !== d) : [...days, d].sort() };
  });
  return (
    <fieldset>
      <legend className="mb-2 text-sm font-medium text-ink-body">{t('marathons.passageMode')}</legend>
      <div role="radiogroup" aria-label={t('marathons.passageMode')} className="flex flex-col gap-2">
        {MODES.map((m) => {
          const on = m.mode === draft;
          return (
            <button key={m.mode} type="button" role="radio" aria-checked={on} onClick={() => choose(m.mode)}
              className={cn('flex items-start gap-3 rounded-lg border px-3 py-2.5 text-left transition-colors focus-visible:shadow-focus focus-visible:outline-none',
                on ? 'border-primary bg-primary-soft' : 'border-line hover:bg-surface-hover')}>
              <span className={cn('mt-0.5 shrink-0', on ? 'text-primary-text' : 'text-ink-soft')} aria-hidden>{m.icon}</span>
              <span className="min-w-0">
                <span className="block text-sm font-medium text-ink-body">{t(`marathons.mode.${m.mode}`)}</span>
                <span className="block text-xs text-ink-soft">{t(`marathons.modeHint.${m.mode}`)}</span>
              </span>
            </button>
          );
        })}
      </div>
      {scheduled && (
        <div className="mt-3 flex flex-col gap-3 rounded-lg border border-line-soft p-3" data-testid="schedule-fields">
          <p className="text-sm font-medium text-ink-body">{t('marathons.lessonOpening')}</p>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            {draft === 'interval' ? (
              <Field label={t('marathons.intervalLabel')}>
                {(p) => (
                  <Select {...p} value={sc.interval_days ?? 1} onChange={(e) => setSc({ ...sc, interval_days: Number(e.target.value) })}>
                    {[1, 2, 3, 4, 5, 6, 7].map((n) => <option key={n} value={n}>{t('marathons.intervalEvery', { n })}</option>)}
                  </Select>
                )}
              </Field>
            ) : (
              <fieldset className="sm:col-span-2">
                <legend className="mb-1.5 text-sm text-ink-body">{t('marathons.weekdaysLabel')}</legend>
                <div className="flex flex-wrap gap-2">
                  {[1, 2, 3, 4, 5, 6, 7].map((d) => {
                    const on = !!sc.weekdays?.includes(d);
                    return (
                      <button key={d} type="button" aria-pressed={on} onClick={() => toggleDay(d)}
                        className={cn('h-9 min-w-[48px] rounded-md border px-2 text-sm font-medium transition-colors focus-visible:shadow-focus focus-visible:outline-none',
                          on ? 'border-primary bg-primary text-white' : 'border-line text-ink-body hover:bg-surface-hover')}>
                        {t(`marathons.weekdayShort.${d}`)}
                      </button>
                    );
                  })}
                </div>
              </fieldset>
            )}
            <Field label={t('marathons.openTimeLabel', { zone })}>
              {(p) => (
                <Select {...p} value={sc.open_time ?? '09:00'} onChange={(e) => setSc({ ...sc, open_time: e.target.value })}>
                  {HOURS.map((h) => <option key={h} value={h}>{h} ({zone})</option>)}
                </Select>
              )}
            </Field>
          </div>
          <Notice tone="info" className="text-sm">{t('marathons.scheduleRule')}</Notice>
          <div className="flex flex-wrap gap-2">
            {changed ? (
              <>
                <Button disabled={!scheduleReady(draft, sc) || busy} onClick={() => onPick({ mode: draft, schedule: sc })}>{t('marathons.modeChangeButton')}</Button>
                <Button variant="secondary" onClick={() => { setDraft(current); setSc(scheduleOf(info, current)); }}>{t('common.cancel')}</Button>
              </>
            ) : (
              <Button disabled={!scheduleReady(draft, sc) || sameSchedule(sc, scheduleOf(info, current)) || busy} loading={busy}
                onClick={() => onSaveSchedule({ mode: draft, schedule: sc })}>{t('marathons.scheduleSave')}</Button>
            )}
          </div>
        </div>
      )}
    </fieldset>
  );
}

type SettingsSection = 'main' | 'passage' | 'extra' | 'appearance';
type LookDraft = { lesson: boolean; d: AppearanceDraft };
const lookDraftOf = (a: MarathonAppearance): LookDraft => ({ lesson: a.lesson_appearance, d: draftOf(a.appearance) });
interface StatsDraft { stats: boolean; rating: boolean; limit: 3 | 5 | 10 | null; showPlace: boolean }

/**
 * «Дополнительно → Статистика учеников»: "Статистика"; "Рейтинг учеников" is locked while the statistics is off
 * (as in the original); with the rating on — "Отображать" (all / first 3 / 5 / 10) and "Место ученика".
 * Dictionary and irregular verbs of the original are outside this stage and are not shown.
 */
function StatsFields({ value, onChange }: { value: StatsDraft; onChange: (v: StatsDraft) => void }) {
  const t = useT();
  return (
    <fieldset className="flex flex-col gap-4" data-testid="stats-fields">
      <legend className="mb-3 text-sm font-medium text-ink-soft">{t('marathons.statsSection')}</legend>
      <Switch checked={value.stats} onChange={(stats) => onChange({ ...value, stats, rating: stats && value.rating })} label={t('marathons.statsSwitch')} hint={t('marathons.statsHint')} />
      <Switch checked={value.stats && value.rating} disabled={!value.stats} onChange={(rating) => onChange({ ...value, rating })}
        label={<span className="inline-flex items-center gap-1.5">{!value.stats && <Lock size={14} aria-hidden />}{t('marathons.ratingSwitch')}</span>}
        hint={value.stats ? t('marathons.studentsRatingHint') : t('marathons.ratingLocked')} />
      {value.stats && value.rating && (
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <Field label={t('marathons.ratingLimitLabel')}>
            {(p) => (
              <Select {...p} value={value.limit ?? 'all'} onChange={(e) => onChange({ ...value, limit: e.target.value === 'all' ? null : (Number(e.target.value) as 3 | 5 | 10) })}>
                <option value="all">{t('marathons.ratingLimit.all')}</option>
                {[3, 5, 10].map((n) => <option key={n} value={n}>{t(`marathons.ratingLimit.${n}`)}</option>)}
              </Select>
            )}
          </Field>
          <Field label={t('marathons.showPlaceLabel')} hint={t('marathons.showPlaceHint')}>
            {(p) => (
              <Select {...p} value={value.showPlace ? 'show' : 'hide'} onChange={(e) => onChange({ ...value, showPlace: e.target.value === 'show' })}>
                <option value="hide">{t('marathons.showPlace.hide')}</option>
                <option value="show">{t('marathons.showPlace.show')}</option>
              </Select>
            )}
          </Field>
        </div>
      )}
    </fieldset>
  );
}

interface ReviewDraft { on: boolean; unlock: MarathonUnlockAfter }

/**
 * "Проверка учителем": the student sends a lesson for review and the teacher completes it. In "По мере прохождения"
 * the next lesson opens after the teacher's review or right after sending ("Открытие уроков"); the field is shown
 * only there, as in the original. Switching does not reset lesson statuses.
 */
function ReviewFields({ value, onChange, progressive }: { value: ReviewDraft; onChange: (v: ReviewDraft) => void; progressive: boolean }) {
  const t = useT();
  return (
    <div className="flex flex-col gap-4" data-testid="review-fields">
      <Switch checked={value.on} onChange={(on) => onChange({ ...value, on })} label={t('marathons.teacherCheck')} hint={t('marathons.teacherCheckHint')} />
      {value.on && progressive && (
        <Field label={t('marathons.unlockAfterLabel')} hint={t('marathons.unlockAfterHint')}>
          {(p) => (
            <Select {...p} value={value.unlock} onChange={(e) => onChange({ ...value, unlock: e.target.value as MarathonUnlockAfter })}>
              <option value="teacher_check">{t('marathons.unlockAfter.teacher_check')}</option>
              <option value="submission">{t('marathons.unlockAfter.submission')}</option>
            </Select>
          )}
        </Field>
      )}
      <p className="text-xs text-ink-soft">{t('marathons.teacherCheckNotice')}</p>
    </div>
  );
}

interface AutoDraft { on: boolean; days: string }

/** "Авто-деактивация": each student loses access N days after their activation date (computed by the server). */
function AutoDeactivationFields({ value, onChange, error }: { value: AutoDraft; onChange: (v: AutoDraft) => void; error?: string }) {
  const t = useT();
  return (
    <div className="flex flex-col gap-4">
      <Switch checked={value.on} onChange={(on) => onChange({ ...value, on })} label={t('marathons.autoDeactivation')} hint={t('marathons.autoDeactivationHint')} />
      {value.on && (
        <>
          <Field label={t('marathons.autoDays')} error={error}>
            {(p) => (
              <Input {...p} type="number" inputMode="numeric" min={1} max={3650} step={1} value={value.days}
                onChange={(e) => onChange({ ...value, days: e.target.value })} className="max-w-[160px]" />
            )}
          </Field>
          <Notice tone="info" className="text-sm">{t('marathons.autoDeactivationRule')}</Notice>
        </>
      )}
    </div>
  );
}

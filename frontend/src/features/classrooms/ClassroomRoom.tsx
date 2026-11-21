'use client';

import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { useHoverExpand } from '@/hooks/useHoverExpand';
import Link from 'next/link';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { useMutation, useQuery, useQueryClient, type UseQueryResult } from '@tanstack/react-query';
import {
  ArrowLeft, BookOpenCheck, Check, ChevronLeft, ChevronRight, Flag, Languages, ListOrdered, Lock, MessageCircle, PanelRight, Pencil, PenTool, Shuffle, Timer, Video, WifiOff,
} from 'lucide-react';
import { AvatarStack, Button, CountBubble, EmptyState, ErrorState, IconButton, Loading, LogoMark, QueryState, Select, Tabs, cn, useToast } from '@/components/ui';
import { attemptsApi, classroomsApi, gradingApi, homeworkApi } from '@/lib/api/endpoints';
import { qk } from '@/lib/queryKeys';
import { useI18n } from '@/lib/i18n';
import { errorMessage } from '@/lib/errors';
import { useClassroomSocket } from '@/lib/realtime/useClassroomSocket';
import type { Classroom, ExerciseView, Me, Progress, RealtimeEvent } from '@/types/api';
import { TestPlayer } from '@/features/exercises/TestPlayer';
import { LessonFinish } from '@/features/grading/LessonFinish';
import { HomeworkPanel } from '@/features/homework/HomeworkPanel';
import { LessonPicker } from '@/features/materials/LessonPicker';
import { ChatPanel } from '@/features/chat/ChatPanel';
import { applyChatEvent } from '@/features/chat/chatCache';
import { CLASSES_ROUTE, marathonUrl } from '@/lib/routes';

type Tab = 'lesson' | 'homework';
const FINISH = 'finish';

export function ClassroomRoom({ id, me }: { id: string; me: Me }) {
  const { t } = useI18n();
  const qc = useQueryClient();
  const router = useRouter();
  const path = usePathname();
  const params = useSearchParams();
  const tab: Tab = params.get('tab') === 'homework' ? 'homework' : 'lesson';
  const sectionParam = params.get('section');
  const [chatOpen, setChatOpen] = useState(false);
  const [sectionsOpen, setSectionsOpen] = useState(true);
  const [pickerOpen, setPickerOpen] = useState(false);
  const [unread, setUnread] = useState(0);

  const classroom = useQuery({ queryKey: qk.classroom(id), queryFn: () => classroomsApi.get(id) });
  const c = classroom.data;
  const role = c?.my_role ?? 'student';
  const teacher = role === 'teacher';
  const student = c?.members.find((m) => m.role === 'student');
  const studentId = teacher ? student?.user_id : me.id;
  const lessonId = c?.current_lesson?.lesson_id;
  // Marathons have their own page; the classroom room is only for teacher-led classes.
  useEffect(() => {
    if (c?.lesson_type === 'marathon') router.replace(marathonUrl(c.id));
  }, [c, router]);

  const setParams = useCallback(
    (patch: Record<string, string | null>) => {
      const p = new URLSearchParams(params.toString());
      for (const [k, v] of Object.entries(patch)) {
        if (v === null) p.delete(k);
        else p.set(k, v);
      }
      router.replace(`${path}?${p.toString()}`, { scroll: false });
    },
    [params, path, router],
  );

  const onEvent = useCallback(
    (ev: RealtimeEvent) => {
      switch (ev.type) {
        case 'chat.message.created':
          applyChatEvent(qc, id, ev);
          if (!chatOpen && (ev.payload as { author: { id: string } }).author.id !== me.id) setUnread((n) => n + 1);
          break;
        case 'chat.message.updated':
        case 'chat.message.deleted':
          applyChatEvent(qc, id, ev);
          break;
        case 'homework.status_changed':
          qc.invalidateQueries({ queryKey: ['homework', id] });
          qc.invalidateQueries({ queryKey: ['lesson-exercises', id] });
          qc.invalidateQueries({ queryKey: ['classrooms'] });
          break;
        case 'lesson.completion_changed':
        case 'section.completion_changed':
          qc.invalidateQueries({ queryKey: ['progress', id] });
          break;
        case 'classroom.lesson_changed':
          qc.invalidateQueries({ queryKey: qk.classroom(id) });
          break;
        case 'attempt.updated':
        case 'materials.changed':
          qc.invalidateQueries({ queryKey: ['lesson-exercises', id] });
          qc.invalidateQueries({ queryKey: ['progress', id] });
          qc.invalidateQueries({ queryKey: ['homework', id] });
          break;
      }
    },
    [qc, id, chatOpen, me.id],
  );
  const socket = useClassroomSocket(c ? id : undefined, onEvent);

  useEffect(() => {
    if (chatOpen) setUnread(0);
  }, [chatOpen]);

  if (classroom.isLoading) return <Loading className="min-h-screen" />;
  if (classroom.error || !c) return <ErrorState error={classroom.error} onRetry={() => classroom.refetch()} className="min-h-screen" />;

  return (
    <div className="flex h-[100dvh] flex-col bg-page">
      <TopBar c={c} me={me} tab={tab} setTab={(v) => setParams({ tab: v === 'lesson' ? null : v })} lessonId={lessonId} studentId={studentId}
        onOtherLesson={() => setPickerOpen(true)} />
      {socket === 'reconnecting' && (
        <div role="status" className="flex items-center justify-center gap-2 bg-warning-soft px-4 py-1.5 text-sm text-warning-text"><WifiOff size={14} />{t('common.reconnecting')}</div>
      )}
      <div className="flex min-h-0 flex-1">
        <Toolbar teacher={teacher} lessonId={lessonId} chatOpen={chatOpen} unread={unread} sectionsOpen={sectionsOpen && !chatOpen}
          onChat={() => setChatOpen((o) => !o)} onSections={() => { setChatOpen(false); setSectionsOpen((o) => !o); }} showSections={tab === 'lesson' && !!lessonId} />

        <main id="main" className="min-w-0 flex-1 overflow-y-auto px-4 py-6 pb-24 scrollbar-thin md:px-8 md:pb-6">
          {!lessonId ? (
            <EmptyState icon={<BookOpenCheck size={26} />} title={teacher ? t('classroom.noLessonTeacher') : t('classroom.noLessonStudent')}
              action={teacher ? <Button onClick={() => setPickerOpen(true)}>{t('classroom.chooseLesson')}</Button> : undefined} className="min-h-[60vh]" />
          ) : tab === 'homework' ? (
            <div className="mx-auto max-w-[820px]">
              <HomeworkPanel classroomId={id} lessonId={lessonId} role={role} studentId={studentId!} studentName={student?.full_name ?? ''} me={me} />
            </div>
          ) : (
            <LessonArea classroomId={id} lessonId={lessonId} role={role} studentId={studentId!} studentName={student?.full_name ?? ''} me={me}
              sectionParam={sectionParam} setSection={(s) => setParams({ section: s })} sectionsOpen={sectionsOpen && !chatOpen} onOtherLesson={() => setPickerOpen(true)} />
          )}
        </main>

        {chatOpen && (
          <div className="fixed inset-0 z-40 md:static md:z-auto md:w-[380px] md:shrink-0 md:border-l md:border-line">
            <ChatPanel classroomId={id} me={me} onClose={() => setChatOpen(false)} />
          </div>
        )}
      </div>
      {teacher && <LessonPicker open={pickerOpen} onClose={() => setPickerOpen(false)} classroomId={id} currentLessonId={lessonId}
        onOpened={() => setParams({ section: null, tab: null })} />}
    </div>
  );
}

function TopBar({ c, me, tab, setTab, lessonId, studentId, onOtherLesson }: {
  c: Classroom; me: Me; tab: Tab; setTab: (t: Tab) => void; lessonId?: string; studentId?: string; onOtherLesson: () => void;
}) {
  const { t } = useI18n();
  const teacher = c.my_role === 'teacher';
  const hw = useQuery({
    queryKey: qk.homework(c.id, lessonId ?? '', teacher ? studentId : undefined),
    queryFn: () => homeworkApi.get(c.id, lessonId!, teacher ? studentId : undefined),
    enabled: !!lessonId && !!studentId,
  });
  return (
    <header className="relative flex h-topbar shrink-0 items-center gap-3 border-b border-line bg-surface px-3 md:px-4">
      <Link href={CLASSES_ROUTE} className="flex items-center gap-2 rounded-md px-1 py-1 text-ink-muted hover:text-ink" aria-label={t('classroom.backToClasses')}>
        <ArrowLeft size={18} />
        <LogoMark size={26} />
      </Link>
      <div className="hidden min-w-0 flex-col leading-tight lg:flex">
        <span className="truncate text-sm font-medium text-ink">{c.name}</span>
        <span className="truncate text-xs text-ink-muted">{c.current_lesson ? `${c.current_lesson.course_title} · ${c.current_lesson.lesson_title}` : ''}</span>
      </div>
      <nav className="absolute left-1/2 top-0 -translate-x-1/2 max-sm:static max-sm:translate-x-0" aria-label={t('classroom.tabLesson')}>
        <Tabs<Tab> size="lg" value={tab} onChange={setTab} items={[
          { value: 'lesson', label: t('classroom.tabLesson') },
          { value: 'homework', label: t('classroom.tabHomework'), badge: hw.data && hw.data.has_exercises ? <HomeworkDot status={hw.data.status} teacher={teacher} /> : undefined },
        ]} />
      </nav>
      <div className="ml-auto flex items-center gap-3">
        <span className="hidden sm:inline-flex" title={t('classroom.participants')}>
          <AvatarStack people={c.members.map((m) => ({ name: m.user_id === me.id ? `${m.full_name} (${t('classroom.you')})` : m.full_name, color: m.avatar_color }))} size={30} />
        </span>
        {teacher && <Button size="sm" variant="soft" icon={<Shuffle size={15} />} onClick={onOtherLesson}>{t('classroom.otherLesson')}</Button>}
      </div>
    </header>
  );
}

/** Small status marker next to the Homework tab (icon/shape + accessible label, not colour only). */
function HomeworkDot({ status, teacher }: { status: 'not_submitted' | 'pending' | 'revision' | 'done'; teacher: boolean }) {
  const { t } = useI18n();
  const label = t(`homework.status.${status}`);
  if (status === 'done') return <Check size={14} className="text-success-text" aria-label={label} />;
  const attention = teacher ? status === 'pending' : status !== 'pending';
  if (!attention) return <span className="h-2 w-2 rounded-full bg-warning" title={label} aria-label={label} role="img" />;
  const tone = status === 'revision' ? 'bg-pink' : status === 'pending' ? 'bg-warning' : 'bg-primary';
  return <span className={cn('inline-flex h-5 min-w-5 items-center justify-center rounded-full px-1 text-[11px] font-bold text-white', tone)} title={label} aria-label={label} role="img">!</span>;
}

function ToolButton({ label, icon, onClick, active, disabled, badge, href, className, expanded }: {
  label: string; icon: ReactNode; onClick?: () => void; active?: boolean; disabled?: boolean; badge?: number; href?: string; className?: string; expanded: boolean;
}) {
  const { t } = useI18n();
  const cls = cn('relative flex h-11 shrink-0 items-center rounded-xl transition-colors max-md:w-11 max-md:justify-center md:w-full md:gap-3 md:px-[11px]', className,
    disabled ? 'cursor-not-allowed text-ink-faint' : active ? 'bg-primary-soft text-primary' : 'text-ink-muted hover:bg-surface-hover hover:text-ink');
  const title = disabled ? `${label} — ${t('common.unavailable')}` : label;
  const inner = (
    <>
      <span className="relative flex w-[22px] shrink-0 justify-center">
        {icon}
        {disabled && <Lock size={10} className="absolute -bottom-1 -right-1.5" aria-hidden />}
        {!!badge && <span className="absolute -right-3 -top-2.5"><CountBubble n={badge} /></span>}
      </span>
      {/* Label is visible only while the desktop rail is expanded; phones keep icon-only buttons. */}
      <span aria-hidden className={cn('hidden min-w-0 flex-1 items-baseline justify-between gap-2 whitespace-nowrap text-left text-base font-medium transition-opacity duration-150 md:flex',
        expanded ? 'opacity-100' : 'pointer-events-none opacity-0')}>
        <span className="truncate">{label}</span>
        {disabled && <span className="text-xs font-normal">{t('common.soon')}</span>}
      </span>
    </>
  );
  if (href && !disabled) return <Link href={href} className={cls} aria-label={title} title={expanded ? undefined : title}>{inner}</Link>;
  return (
    <button type="button" className={cls} onClick={disabled ? undefined : onClick} aria-label={title} title={expanded ? undefined : title}
      aria-disabled={disabled || undefined} aria-pressed={disabled ? undefined : !!active}>
      {inner}
    </button>
  );
}

const TOOLBAR_COLLAPSED = 64;
const TOOLBAR_EXPANDED = 232;

/**
 * Classroom tool rail (separate from the cabinet menu). Desktop: a fixed 64px column is reserved,
 * the rail expands over the lesson on hover/keyboard focus (no layout shift) and collapses 180ms
 * after the pointer leaves. Phones: bottom bar, tools open on tap.
 */
function Toolbar({ teacher, lessonId, chatOpen, unread, sectionsOpen, onChat, onSections, showSections }: {
  teacher: boolean; lessonId?: string; chatOpen: boolean; unread: number; sectionsOpen: boolean; onChat: () => void; onSections: () => void; showSections: boolean;
}) {
  const { t } = useI18n();
  const rail = useRef<HTMLElement>(null);
  const { expanded, panelProps } = useHoverExpand(rail);
  const [animate, setAnimate] = useState(false);
  useEffect(() => {
    const id = requestAnimationFrame(() => setAnimate(true));
    return () => cancelAnimationFrame(id);
  }, []);
  const tb = { expanded };
  return (
    <div className="md:relative md:w-16 md:shrink-0">
      <nav ref={rail} id="classroom-toolbar" aria-label={t('classroom.toolbar')} data-expanded={expanded} {...panelProps}
        style={{ ['--tb-w' as string]: `${expanded ? TOOLBAR_EXPANDED : TOOLBAR_COLLAPSED}px` }}
        className={cn(
          'fixed bottom-0 left-0 z-30 flex w-full justify-around border-t border-line bg-surface p-1.5',
          'md:absolute md:inset-y-0 md:w-[var(--tb-w)] md:flex-col md:justify-start md:gap-1.5 md:overflow-hidden md:border-r md:border-t-0 md:px-2.5 md:py-3',
          expanded && 'md:shadow-pop',
          animate && 'md:transition-[width,box-shadow] md:duration-200 md:ease-out',
        )}>
        <ToolButton {...tb} label={t('classroom.toolChat')} icon={<MessageCircle size={21} />} active={chatOpen} onClick={onChat} badge={unread} />
        {showSections && <ToolButton {...tb} className="max-lg:hidden" label={t('classroom.toolSections')} icon={<PanelRight size={21} />} active={sectionsOpen} onClick={onSections} />}
        {teacher && lessonId && <ToolButton {...tb} label={t('classroom.toolEdit')} icon={<Pencil size={20} />} href={`/materials/lessons/${lessonId}`} />}
        <span className="my-1 hidden h-px w-full shrink-0 bg-line md:block" aria-hidden />
        <ToolButton {...tb} label={t('classroom.toolVideo')} icon={<Video size={21} />} disabled />
        <ToolButton {...tb} label={t('classroom.toolBoard')} icon={<PenTool size={20} />} disabled />
        <ToolButton {...tb} label={t('classroom.toolDictionary')} icon={<Languages size={20} />} disabled />
        <ToolButton {...tb} label={t('classroom.toolTimer')} icon={<Timer size={20} />} disabled />
      </nav>
    </div>
  );
}

function LessonArea({ classroomId, lessonId, role, studentId, studentName, me, sectionParam, setSection, sectionsOpen, onOtherLesson }: {
  classroomId: string; lessonId: string; role: 'teacher' | 'student'; studentId: string; studentName: string; me: Me;
  sectionParam: string | null; setSection: (s: string | null) => void; sectionsOpen: boolean; onOtherLesson: () => void;
}) {
  const { t } = useI18n();
  const teacher = role === 'teacher';
  const sid = teacher ? studentId : undefined;
  const progress = useQuery({ queryKey: qk.progress(classroomId, lessonId, sid), queryFn: () => gradingApi.progress(classroomId, lessonId, sid) });
  const exercises = useQuery({ queryKey: qk.lessonExercises(classroomId, lessonId, 'lesson', sid), queryFn: () => attemptsApi.lessonExercises(classroomId, lessonId, 'lesson', sid) });

  return (
    <QueryState query={progress}>
      {(p) => {
        const sections = p.sections;
        const idx = sectionParam === FINISH ? sections.length : Math.max(0, sections.findIndex((s) => s.id === sectionParam));
        const current = sections[idx];
        const onFinish = idx >= sections.length;
        return (
          <div className="mx-auto flex max-w-[1180px] gap-6">
            <div className="min-w-0 flex-1">
              <div className="mx-auto max-w-[820px]">
                <p className="text-sm text-ink-muted">{p.course_title}</p>
                <h1 className="mb-5 text-2xl font-bold text-ink">{p.lesson_title}</h1>
                {sections.length > 0 && (
                  <label className="mb-5 block lg:hidden">
                    <span className="sr-only">{t('classroom.sections')}</span>
                    <Select value={onFinish ? FINISH : current?.id} onChange={(e) => setSection(e.target.value)}>
                      {sections.map((s, i) => <option key={s.id} value={s.id}>{i + 1}. {s.title}{s.completed ? ' ✓' : ''}</option>)}
                      <option value={FINISH}>{t('classroom.finishLesson')}{p.completion.completed ? ' ✓' : ''}</option>
                    </Select>
                  </label>
                )}
                {onFinish ? (
                  <LessonFinish classroomId={classroomId} progress={p} role={role} me={me} onOtherLesson={onOtherLesson} />
                ) : !current ? (
                  <EmptyState title={t('classroom.noSections')} />
                ) : (
                  <SectionView classroomId={classroomId} progress={p} sectionIndex={idx} teacher={teacher} studentId={studentId} studentName={studentName}
                    exercises={exercises} />
                )}
                <nav className="mt-8 flex items-center justify-between gap-3 [&>button]:min-w-0 max-sm:[&>button]:flex-1 max-sm:[&>button]:px-3" aria-label={t('classroom.sections')}>
                  <Button variant="secondary" icon={<ChevronLeft size={18} />} disabled={idx === 0}
                    onClick={() => setSection(sections[idx - 1]?.id ?? null)}>{t('classroom.prevSection')}</Button>
                  {!onFinish && (
                    <Button icon={idx === sections.length - 1 ? <Flag size={16} /> : undefined}
                      onClick={() => setSection(idx + 1 < sections.length ? sections[idx + 1].id : FINISH)}>
                      {idx === sections.length - 1 || !sections.length ? t('classroom.toFinish') : t('classroom.nextSection')}
                      {idx < sections.length - 1 && <ChevronRight size={18} />}
                    </Button>
                  )}
                </nav>
              </div>
            </div>
            {sectionsOpen && <SectionsPanel classroomId={classroomId} progress={p} idx={idx} teacher={teacher} onSelect={(i) => setSection(i < sections.length ? sections[i].id : FINISH)} />}
          </div>
        );
      }}
    </QueryState>
  );
}

function SectionView({ classroomId, progress, sectionIndex, teacher, studentId, studentName, exercises }: {
  classroomId: string; progress: Progress; sectionIndex: number; teacher: boolean; studentId: string; studentName: string;
  exercises: UseQueryResult<{ items: ExerciseView[] }>;
}) {
  const { t } = useI18n();
  const s = progress.sections[sectionIndex];
  const toggle = useSectionToggle(classroomId);
  return (
    <section aria-labelledby={`sec-${s.id}`}>
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div>
          <p className="text-xs font-medium uppercase tracking-wide text-ink-muted">{t('classroom.sectionCounter', { a: sectionIndex + 1, b: progress.sections.length })}</p>
          <h2 id={`sec-${s.id}`} className="flex items-center gap-2 text-xl font-bold text-ink">
            {s.title}
            {s.completed && <span className="inline-flex h-6 w-6 items-center justify-center rounded-full bg-success text-white" title={t('classroom.sectionDone')} aria-label={t('classroom.sectionDone')}><Check size={14} strokeWidth={3} /></span>}
          </h2>
        </div>
        {teacher && (
          <Button variant={s.completed ? 'secondary' : 'success'} size="sm" icon={<Check size={16} />} loading={toggle.isPending}
            onClick={() => toggle.mutate({ id: s.id, completed: !s.completed })}>
            {s.completed ? t('classroom.unmarkDone') : t('classroom.markDone')}
          </Button>
        )}
      </div>
      <QueryState query={exercises}>
        {(data) => {
          const items = data.items.filter((e) => e.section_id === s.id);
          if (!items.length) return <EmptyState title={t('classroom.sectionEmpty')} className="rounded-2xl border border-dashed border-line-strong bg-surface" />;
          return (
            <div className="flex flex-col gap-4">
              {items.map((v, i) => (
                <TestPlayer key={v.exercise_id} classroomId={classroomId} view={v} role={teacher ? 'teacher' : 'student'} studentId={studentId} studentName={studentName} index={i} />
              ))}
            </div>
          );
        }}
      </QueryState>
    </section>
  );
}

function useSectionToggle(classroomId: string) {
  const qc = useQueryClient();
  const toast = useToast();
  const { t } = useI18n();
  return useMutation({
    mutationFn: ({ id, completed }: { id: string; completed: boolean }) => gradingApi.setSection(classroomId, id, completed),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['progress', classroomId] }),
    onError: (e) => toast(errorMessage(e, t), 'error'),
  });
}

function SectionsPanel({ classroomId, progress, idx, teacher, onSelect }: { classroomId: string; progress: Progress; idx: number; teacher: boolean; onSelect: (i: number) => void }) {
  const { t } = useI18n();
  const toggle = useSectionToggle(classroomId);
  const items = useMemo(() => progress.sections, [progress.sections]);
  return (
    <aside className="sticky top-0 hidden h-fit w-[300px] shrink-0 rounded-2xl border border-line bg-surface p-3 shadow-card lg:block" aria-label={t('classroom.sections')}>
      <h2 className="flex items-center gap-2 px-2 pb-2 pt-1 text-base font-bold text-ink"><ListOrdered size={18} className="text-ink-muted" />{t('classroom.sections')}</h2>
      <ol className="flex flex-col gap-0.5">
        {items.map((s, i) => (
          <li key={s.id} className={cn('group flex items-center gap-1 rounded-lg pr-1', i === idx ? 'bg-primary-soft' : 'hover:bg-surface-hover')}>
            <button type="button" onClick={() => onSelect(i)} aria-current={i === idx ? 'step' : undefined}
              className="flex min-w-0 flex-1 items-center gap-2.5 px-2 py-2 text-left">
              <span className={cn('flex h-6 w-6 shrink-0 items-center justify-center rounded-full text-xs font-bold',
                s.completed ? 'bg-success text-white' : i === idx ? 'bg-primary text-white' : 'bg-surface-hover text-ink-body')}>
                {s.completed ? <Check size={13} strokeWidth={3} aria-label={t('classroom.sectionDone')} /> : i + 1}
              </span>
              <span className={cn('truncate text-base', i === idx ? 'font-medium text-primary-text' : 'text-ink')}>{s.title}</span>
            </button>
            {teacher && (
              <IconButton size="sm" label={s.completed ? t('classroom.unmarkDone') : t('classroom.markDone')} active={s.completed}
                onClick={() => toggle.mutate({ id: s.id, completed: !s.completed })} className={cn(!s.completed && 'opacity-0 focus:opacity-100 group-hover:opacity-100')}>
                <Check size={15} />
              </IconButton>
            )}
          </li>
        ))}
        <li className={cn('mt-1 rounded-lg border-t border-line pt-1', idx === items.length && 'bg-primary-soft')}>
          <button type="button" onClick={() => onSelect(items.length)} aria-current={idx === items.length ? 'step' : undefined}
            className="flex w-full items-center gap-2.5 px-2 py-2 text-left">
            <span className={cn('flex h-6 w-6 items-center justify-center rounded-full', progress.completion.completed ? 'bg-success text-white' : 'bg-surface-hover text-ink-body')}>
              {progress.completion.completed ? <Check size={13} strokeWidth={3} /> : <Flag size={13} />}
            </span>
            <span className={cn('text-base', idx === items.length ? 'font-medium text-primary-text' : 'text-ink')}>{t('classroom.finishLesson')}</span>
          </button>
        </li>
      </ol>
    </aside>
  );
}

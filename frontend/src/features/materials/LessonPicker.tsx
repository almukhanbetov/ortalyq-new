'use client';

import { useEffect, useMemo, useState } from 'react';
import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, Filter, Search, X } from 'lucide-react';
import { Badge, Button, EmptyState, Field, Input, Loading, Modal, QueryState, Select, Tabs, cn, useToast } from '@/components/ui';
import { classroomsApi, materialsApi } from '@/lib/api/endpoints';
import { qk } from '@/lib/queryKeys';
import { useT } from '@/lib/i18n';
import { errorMessage } from '@/lib/errors';
import type { FilterOptions, LessonFilter } from '@/types/api';
import { Cover } from '@/features/classrooms/Cover';

const FIELDS: { key: keyof LessonFilter; opt: keyof FilterOptions }[] = [
  { key: 'language', opt: 'language' }, { key: 'age', opt: 'age' }, { key: 'level', opt: 'level' }, { key: 'lesson_type', opt: 'lesson_type' },
  { key: 'skill', opt: 'skills' }, { key: 'max_duration', opt: 'duration' }, { key: 'grammar', opt: 'grammar' }, { key: 'vocabulary', opt: 'vocabulary' },
  { key: 'function', opt: 'functions' }, { key: 'tag', opt: 'tags' },
];

function useDebounced<T>(value: T, ms = 300) {
  const [v, setV] = useState(value);
  useEffect(() => {
    const id = setTimeout(() => setV(value), ms);
    return () => clearTimeout(id);
  }, [value, ms]);
  return v;
}

export function LessonPicker({ open, onClose, classroomId, currentLessonId, onOpened, onPick, pickedIds }: {
  open: boolean;
  onClose: () => void;
  classroomId: string;
  currentLessonId?: string;
  onOpened: () => void;
  /** Marathon mode: add lessons one by one instead of opening a current lesson; the picker stays open. */
  onPick?: (lessonId: string) => Promise<unknown>;
  pickedIds?: string[];
}) {
  const t = useT();
  const qc = useQueryClient();
  const toast = useToast();
  const [q, setQ] = useState('');
  const [filter, setFilter] = useState<LessonFilter>({});
  const [showFilter, setShowFilter] = useState(false);
  const [courseId, setCourseId] = useState<string | null>(null);
  const [tab, setTab] = useState<'content' | 'description'>('content');
  const dq = useDebounced(q);
  const f = useMemo(() => ({ ...filter, q: dq }), [filter, dq]);
  const activeFilters = Object.values(filter).filter(Boolean).length;

  const options = useQuery({ queryKey: qk.filters, queryFn: materialsApi.filters, enabled: open });
  const courses = useQuery({ queryKey: qk.courses(f), queryFn: () => materialsApi.courses({ ...f, limit: 100 }), enabled: open, placeholderData: keepPreviousData });
  const selected = courseId ?? courses.data?.items[0]?.id ?? null;
  const course = useQuery({ queryKey: qk.course(selected ?? '', f), queryFn: () => materialsApi.course(selected!, f), enabled: open && !!selected, placeholderData: keepPreviousData });

  const openLesson = useMutation({
    mutationFn: async (lessonId: string) => {
      if (onPick) {
        await onPick(lessonId);
        return null;
      }
      return classroomsApi.setLesson(classroomId, lessonId);
    },
    onSuccess: (c) => {
      if (!c) {
        toast(t('marathons.lessonAdded'), 'success');
        return;
      }
      qc.setQueryData(qk.classroom(classroomId), c);
      qc.invalidateQueries({ queryKey: ['classrooms'] });
      onOpened();
      onClose();
    },
    onError: (e) => toast(errorMessage(e, t), 'error'),
  });

  return (
    <Modal open={open} onClose={onClose} title={t('picker.title')} size="xl" bodyClassName="p-0"
      footer={<Button variant="secondary" icon={<ArrowLeft size={16} />} onClick={onClose}>{onPick ? t('marathons.pickerBack') : t('picker.backToClass')}</Button>}>
      <div className="flex min-h-[560px] flex-col md:flex-row">
        <aside className="flex w-full shrink-0 flex-col border-line md:w-[320px] md:border-r">
          <div className="flex gap-2 p-4">
            <div className="relative flex-1">
              <Search size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-ink-faint" aria-hidden />
              <Input value={q} onChange={(e) => setQ(e.target.value)} placeholder={t('common.search')} aria-label={t('picker.searchPlaceholder')} title={t('picker.searchPlaceholder')} className="pl-9" />
            </div>
            <Button variant={showFilter || activeFilters ? 'soft' : 'secondary'} icon={<Filter size={16} />} onClick={() => setShowFilter((s) => !s)} aria-expanded={showFilter}>
              {t('picker.filter')}{activeFilters ? ` · ${activeFilters}` : ''}
            </Button>
          </div>
          {showFilter && (
            <div className="border-y border-line bg-surface-alt p-4">
              <QueryState query={options}>
                {(o) => (
                  <div className="grid grid-cols-2 gap-3">
                    {FIELDS.map(({ key, opt }) => (
                      <Field key={key} label={t(`picker.fields.${key}`)}>
                        {(p) => (
                          <Select {...p} value={filter[key] ?? ''} onChange={(e) => setFilter((x) => ({ ...x, [key]: e.target.value || undefined }))} className="h-9 text-sm">
                            <option value="">{t('picker.any')}</option>
                            {(o[opt] as (string | number)[]).map((v) => (
                              <option key={String(v)} value={String(v)}>{key === 'max_duration' ? t('picker.upTo', { n: v }) : v}</option>
                            ))}
                          </Select>
                        )}
                      </Field>
                    ))}
                  </div>
                )}
              </QueryState>
              {activeFilters > 0 && <Button variant="ghost" size="sm" className="mt-3" icon={<X size={14} />} onClick={() => setFilter({})}>{t('picker.resetFilter')}</Button>}
            </div>
          )}
          <h3 className="px-4 pb-2 pt-3 text-xs font-medium uppercase tracking-wide text-ink-muted">{t('picker.courses')}</h3>
          <div className="max-h-[260px] flex-1 overflow-y-auto px-2 pb-3 scrollbar-thin md:max-h-none">
            <QueryState query={courses}>
              {(data) => data.items.length === 0 ? <EmptyState title={t('picker.noCourses')} /> : (
                <ul className="flex flex-col gap-1">
                  {data.items.map((c) => (
                    <li key={c.id}>
                      <button type="button" onClick={() => { setCourseId(c.id); setTab('content'); }} aria-current={c.id === selected || undefined}
                        className={cn('flex w-full items-center gap-3 rounded-lg p-2 text-left transition-colors', c.id === selected ? 'bg-primary-soft' : 'hover:bg-surface-hover')}>
                        <Cover color={c.cover_color} title={c.title} className="h-11 w-14 shrink-0 rounded-md [&>span]:bottom-1 [&>span]:left-2 [&>span]:text-sm" />
                        <span className="min-w-0">
                          <span className={cn('block truncate text-base font-medium', c.id === selected ? 'text-primary-text' : 'text-ink')}>{c.title}</span>
                          <span className="block text-xs text-ink-muted">{t('picker.lessonsCount', { n: c.lesson_count })}</span>
                        </span>
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </QueryState>
          </div>
        </aside>

        <section className="min-w-0 flex-1 p-4 md:p-6">
          {!selected ? <EmptyState title={t('picker.selectCourse')} /> : course.isLoading ? <Loading /> : course.data && (
            <>
              <Cover color={course.data.cover_color} title={course.data.title} className="mb-4 h-28 rounded-xl" />
              <h3 className="text-xl font-bold text-ink">{course.data.title}</h3>
              <div className="mb-4 border-b border-line">
                <Tabs value={tab} onChange={setTab} items={[{ value: 'content', label: t('picker.tabContent') }, { value: 'description', label: t('picker.tabDescription') }]} />
              </div>
              {tab === 'description' ? (
                <p className="whitespace-pre-line text-base text-ink-body">{course.data.description || t('picker.noDescription')}</p>
              ) : (course.data.lessons ?? []).length === 0 ? (
                <EmptyState title={t('picker.noLessons')} />
              ) : (
                <ol className="flex flex-col gap-2">
                  {course.data.lessons!.map((l, i) => {
                    const current = pickedIds ? pickedIds.includes(l.id) : l.id === currentLessonId;
                    const tags = [l.level, l.language, l.lesson_type, l.duration_min ? t('picker.upTo', { n: l.duration_min }) : '', ...l.skills].filter(Boolean);
                    return (
                      <li key={l.id} className={cn('flex flex-wrap items-center gap-3 rounded-xl border px-4 py-3', current ? 'border-primary bg-primary-soft/50' : 'border-line')}>
                        <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-surface-hover text-sm font-bold text-ink-body">{i + 1}</span>
                        <div className="min-w-0 flex-1">
                          <p className="truncate font-medium text-ink">{l.title}</p>
                          <p className="mt-1 flex flex-wrap gap-1">
                            <span className="text-xs text-ink-muted">{t('picker.sectionsCount', { n: l.section_count })}</span>
                            {tags.slice(0, 5).map((tg) => <span key={tg} className="rounded bg-surface-hover px-1.5 text-xs text-ink-body">{tg}</span>)}
                          </p>
                        </div>
                        {current ? <Badge tone="primary">{onPick ? t('marathons.pickerAdded') : t('picker.current')}</Badge> : (
                          <Button size="sm" onClick={() => openLesson.mutate(l.id)} loading={openLesson.isPending && openLesson.variables === l.id} disabled={openLesson.isPending}>
                            {onPick ? t('marathons.pickerAdd') : t('picker.openLesson')}
                          </Button>
                        )}
                      </li>
                    );
                  })}
                </ol>
              )}
            </>
          )}
        </section>
      </div>
    </Modal>
  );
}

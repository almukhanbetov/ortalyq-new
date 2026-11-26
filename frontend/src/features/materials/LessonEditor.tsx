'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useSearchParams } from 'next/navigation';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowDown, ArrowLeft, ArrowUp, Copy, Eye, EyeOff, ListChecks, MoreHorizontal, Pencil, PencilLine, Plus, Settings2, Trash2 } from 'lucide-react';
import { Badge, Button, ConfirmDialog, EmptyState, Field, IconButton, Input, Menu, QueryState, Tabs, useToast } from '@/components/ui';
import { PageCard } from '@/components/layout/PageCard';
import { exercisesApi, materialsApi } from '@/lib/api/endpoints';
import { qk } from '@/lib/queryKeys';
import { useT } from '@/lib/i18n';
import { marathonFromParam, marathonUrl, materialUrl } from '@/lib/routes';
import { errorMessage, fieldErrors } from '@/lib/errors';
import type { Exercise, Lesson, LessonAttrs, Scope, Section } from '@/types/api';
import { CatalogModal } from '@/features/exercises/CatalogModal';
import { ExerciseEditorModal, isExerciseKind, type EditorTarget } from '@/features/exercises/ExerciseEditorModal';
import { TitleModal } from './TitleModal';

export function LessonEditor({ id }: { id: string }) {
  const t = useT();
  const [tab, setTab] = useState<Scope>('lesson');
  const lesson = useQuery({ queryKey: qk.lesson(id), queryFn: () => materialsApi.lesson(id) });
  const [catalogFor, setCatalogFor] = useState<{ scope: Scope; sectionId: string | null } | null>(null);
  const [editor, setEditor] = useState<EditorTarget | null>(null);
  // Opened from a marathon ("Редактировать" on the "Урок" tab): go back there instead of the course.
  const marathonId = marathonFromParam(useSearchParams().get('marathon'));

  return (
    <QueryState query={lesson}>
      {(l) => (
        <PageCard>
          <Link href={marathonId ? marathonUrl(marathonId, 'lessons') : materialUrl(l.course_id)} className="mb-4 inline-flex items-center gap-1 text-sm text-ink-muted hover:text-ink">
            <ArrowLeft size={16} />{marathonId ? t('marathons.backToMarathon') : t('materials.backToCourse')}
          </Link>
          <LessonHeader lesson={l} />
          <div className="mb-5 border-b border-line">
            <Tabs<Scope> value={tab} onChange={setTab} items={[{ value: 'lesson', label: t('materials.lessonTab') }, { value: 'homework', label: t('materials.homeworkTab') }]} />
          </div>
          {tab === 'lesson' ? (
            <SectionsEditor lesson={l} onAdd={(sectionId) => setCatalogFor({ scope: 'lesson', sectionId })} onEdit={(e) => setEditor({ lessonId: l.id, scope: 'lesson', sectionId: e.section_id, exercise: e })} />
          ) : (
            <HomeworkEditor lessonId={l.id} onAdd={() => setCatalogFor({ scope: 'homework', sectionId: null })} onEdit={(e) => setEditor({ lessonId: l.id, scope: 'homework', sectionId: null, exercise: e })} />
          )}
          <CatalogModal open={!!catalogFor} onClose={() => setCatalogFor(null)}
            onPick={(kind) => {
              if (isExerciseKind(kind) && catalogFor) { setEditor({ lessonId: l.id, scope: catalogFor.scope, sectionId: catalogFor.sectionId, kind }); setCatalogFor(null); }
            }} />
          <ExerciseEditorModal target={editor} onClose={() => setEditor(null)} />
        </PageCard>
      )}
    </QueryState>
  );
}

const ATTR_LISTS = ['skills', 'grammar', 'vocabulary', 'functions', 'tags'] as const;
const ATTR_LABEL: Record<string, string> = {
  language: 'language', age: 'age', level: 'level', lesson_type: 'lesson_type', skills: 'skill', grammar: 'grammar', vocabulary: 'vocabulary', functions: 'function', tags: 'tag',
};

function LessonHeader({ lesson }: { lesson: Lesson }) {
  const t = useT();
  const qc = useQueryClient();
  const toast = useToast();
  const [renaming, setRenaming] = useState(false);
  const [showAttrs, setShowAttrs] = useState(false);
  const [attrs, setAttrs] = useState<Record<string, string>>({});
  const update = useMutation({
    mutationFn: (body: { title?: string; attrs?: LessonAttrs }) => materialsApi.updateLesson(lesson.id, body),
    onSuccess: (l) => { qc.setQueryData(qk.lesson(lesson.id), l); qc.invalidateQueries({ queryKey: ['course'] }); setRenaming(false); toast(t('common.save') + ' ✓', 'success'); },
  });
  const openAttrs = () => {
    setAttrs({
      language: lesson.language, age: lesson.age, level: lesson.level, lesson_type: lesson.lesson_type,
      duration_min: lesson.duration_min ? String(lesson.duration_min) : '',
      ...Object.fromEntries(ATTR_LISTS.map((k) => [k, lesson[k].join(', ')])),
    });
    setShowAttrs((s) => !s);
  };
  const saveAttrs = () => {
    const split = (s: string) => s.split(',').map((x) => x.trim()).filter(Boolean);
    update.mutate({
      attrs: {
        language: attrs.language ?? '', age: attrs.age ?? '', level: attrs.level ?? '', lesson_type: attrs.lesson_type ?? '',
        duration_min: attrs.duration_min ? Number(attrs.duration_min) : null,
        skills: split(attrs.skills ?? ''), grammar: split(attrs.grammar ?? ''), vocabulary: split(attrs.vocabulary ?? ''), functions: split(attrs.functions ?? ''), tags: split(attrs.tags ?? ''),
      },
    });
  };
  const errs = fieldErrors(update.error);
  return (
    <div className="mb-5">
      <div className="flex flex-wrap items-center gap-2">
        <h1 className="text-2xl font-bold text-ink">{lesson.title}</h1>
        <IconButton label={t('common.rename')} onClick={() => setRenaming(true)}><Pencil size={16} /></IconButton>
        <Button variant="ghost" size="sm" icon={<Settings2 size={16} />} onClick={openAttrs} aria-expanded={showAttrs}>{t('materials.lessonSettings')}</Button>
      </div>
      {showAttrs && (
        <div className="mt-4 rounded-xl border border-line bg-surface-alt p-4">
          <p className="mb-3 text-sm text-ink-muted">{t('materials.attrsHint')}</p>
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {(['language', 'age', 'level', 'lesson_type'] as const).map((k) => (
              <Field key={k} label={t(`picker.fields.${ATTR_LABEL[k]}`)}>
                {(p) => <Input {...p} value={attrs[k] ?? ''} maxLength={60} onChange={(e) => setAttrs((a) => ({ ...a, [k]: e.target.value }))} />}
              </Field>
            ))}
            <Field label={t('materials.durationMin')} error={errs['attrs.duration_min']}>
              {(p) => <Input {...p} type="number" min={1} max={600} value={attrs.duration_min ?? ''} onChange={(e) => setAttrs((a) => ({ ...a, duration_min: e.target.value }))} />}
            </Field>
            {ATTR_LISTS.map((k) => (
              <Field key={k} label={`${t(`picker.fields.${ATTR_LABEL[k]}`)} (${t('materials.commaSeparated')})`}>
                {(p) => <Input {...p} value={attrs[k] ?? ''} onChange={(e) => setAttrs((a) => ({ ...a, [k]: e.target.value }))} />}
              </Field>
            ))}
          </div>
          <div className="mt-4 flex justify-end gap-2">
            <Button variant="secondary" onClick={() => setShowAttrs(false)}>{t('common.cancel')}</Button>
            <Button onClick={saveAttrs} loading={update.isPending}>{t('common.save')}</Button>
          </div>
        </div>
      )}
      <TitleModal open={renaming} onClose={() => setRenaming(false)} title={t('common.rename')} label={t('materials.lessonTitle')} initial={lesson.title}
        loading={update.isPending} error={errs.title} onSubmit={(title) => update.mutate({ title })} />
    </div>
  );
}

function useExerciseList(lessonId: string, scope: Scope) {
  return useQuery({ queryKey: qk.editorExercises(lessonId, scope), queryFn: () => exercisesApi.list(lessonId, scope) });
}

function SectionsEditor({ lesson, onAdd, onEdit }: { lesson: Lesson; onAdd: (sectionId: string) => void; onEdit: (e: Exercise) => void }) {
  const t = useT();
  const qc = useQueryClient();
  const exercises = useExerciseList(lesson.id, 'lesson');
  const [creating, setCreating] = useState(false);
  const [renaming, setRenaming] = useState<Section | null>(null);
  const sections = lesson.sections ?? [];
  const refresh = () => qc.invalidateQueries({ queryKey: qk.lesson(lesson.id) });
  const create = useMutation({ mutationFn: (title: string) => materialsApi.createSection(lesson.id, title), onSuccess: () => { refresh(); setCreating(false); } });
  const rename = useMutation({ mutationFn: ({ id, title }: { id: string; title: string }) => materialsApi.updateSection(id, title), onSuccess: () => { refresh(); setRenaming(null); } });
  const reorder = useMutation({ mutationFn: (ids: string[]) => materialsApi.reorderSections(lesson.id, ids), onSettled: refresh });
  const move = (i: number, d: -1 | 1) => { const ids = sections.map((s) => s.id); [ids[i], ids[i + d]] = [ids[i + d], ids[i]]; reorder.mutate(ids); };

  return (
    <div>
      {sections.length === 0 && <EmptyState title={t('materials.noSections')} />}
      <ol className="flex flex-col gap-4">
        {sections.map((s, i) => {
          const items = (exercises.data?.items ?? []).filter((e) => e.section_id === s.id);
          return (
            <li key={s.id} className="rounded-xl border border-line">
              <div className="flex items-center gap-2 border-b border-line bg-surface-alt px-4 py-3">
                <span className="flex h-7 w-7 items-center justify-center rounded-md bg-primary-soft text-sm font-bold text-primary-text">{i + 1}</span>
                <h3 className="flex-1 truncate font-medium text-ink">{s.title}</h3>
                <IconButton size="sm" label={t('common.rename')} onClick={() => setRenaming(s)}><Pencil size={15} /></IconButton>
                <IconButton size="sm" label={t('common.moveUp')} disabled={i === 0 || reorder.isPending} onClick={() => move(i, -1)}><ArrowUp size={15} /></IconButton>
                <IconButton size="sm" label={t('common.moveDown')} disabled={i === sections.length - 1 || reorder.isPending} onClick={() => move(i, 1)}><ArrowDown size={15} /></IconButton>
              </div>
              <div className="p-4">
                <ExerciseRows items={items} lessonId={lesson.id} scope="lesson" sectionId={s.id} onEdit={onEdit} emptyText={t('materials.sectionEmpty')} />
                <Button variant="soft" size="sm" className="mt-3" icon={<Plus size={16} />} onClick={() => onAdd(s.id)}>{t('materials.addExercise')}</Button>
              </div>
            </li>
          );
        })}
      </ol>
      <Button variant="secondary" className="mt-4" icon={<Plus size={18} />} onClick={() => setCreating(true)}>{t('materials.createSection')}</Button>
      <TitleModal open={creating} onClose={() => setCreating(false)} title={t('materials.createSection')} label={t('materials.sectionTitle')}
        loading={create.isPending} error={fieldErrors(create.error).title} onSubmit={(title) => create.mutate(title)} />
      <TitleModal open={!!renaming} onClose={() => setRenaming(null)} title={t('common.rename')} label={t('materials.sectionTitle')} initial={renaming?.title}
        loading={rename.isPending} error={fieldErrors(rename.error).title} onSubmit={(title) => renaming && rename.mutate({ id: renaming.id, title })} />
    </div>
  );
}

function HomeworkEditor({ lessonId, onAdd, onEdit }: { lessonId: string; onAdd: () => void; onEdit: (e: Exercise) => void }) {
  const t = useT();
  const q = useExerciseList(lessonId, 'homework');
  return (
    <QueryState query={q}>
      {(data) => (
        <div>
          <ExerciseRows items={data.items} lessonId={lessonId} scope="homework" sectionId={null} onEdit={onEdit} emptyText={t('materials.homeworkEmpty')} />
          <Button variant="soft" className="mt-4" icon={<Plus size={18} />} onClick={onAdd}>{t('materials.addExercise')}</Button>
        </div>
      )}
    </QueryState>
  );
}

function ExerciseRows({ items, lessonId, scope, sectionId, onEdit, emptyText }: { items: Exercise[]; lessonId: string; scope: Scope; sectionId: string | null; onEdit: (e: Exercise) => void; emptyText: string }) {
  const t = useT();
  const qc = useQueryClient();
  const toast = useToast();
  const [deleting, setDeleting] = useState<Exercise | null>(null);
  const refresh = () => {
    qc.invalidateQueries({ queryKey: ['editor-exercises', lessonId] });
    qc.invalidateQueries({ queryKey: ['lesson-exercises'] });
  };
  const onError = (e: unknown) => toast(errorMessage(e, t), 'error');
  const hide = useMutation({ mutationFn: (e: Exercise) => exercisesApi.setHidden(e.id, !e.hidden), onSuccess: refresh, onError });
  const copy = useMutation({ mutationFn: (e: Exercise) => exercisesApi.copyToHomework(e.id), onSuccess: () => { refresh(); toast(t('editor.copied'), 'success'); }, onError });
  const remove = useMutation({ mutationFn: (e: Exercise) => exercisesApi.remove(e.id), onSuccess: () => { refresh(); setDeleting(null); }, onError });
  const reorder = useMutation({ mutationFn: (ids: string[]) => exercisesApi.reorder(lessonId, scope, sectionId, ids), onSettled: refresh, onError });
  const move = (i: number, d: -1 | 1) => { const ids = items.map((x) => x.id); [ids[i], ids[i + d]] = [ids[i + d], ids[i]]; reorder.mutate(ids); };

  if (!items.length) return <p className="text-sm text-ink-muted">{emptyText}</p>;
  return (
    <>
      <ul className="flex flex-col gap-2">
        {items.map((e, i) => (
          <li key={e.id} className="flex items-center gap-3 rounded-lg border border-line bg-surface px-3 py-2.5">
            <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-primary-soft text-primary-text">{e.kind === 'gaps' ? <PencilLine size={18} /> : <ListChecks size={18} />}</span>
            <button type="button" onClick={() => onEdit(e)} className="min-w-0 flex-1 text-left">
              <p className="truncate font-medium text-ink hover:text-primary-text">{e.version.title}</p>
              <p className="text-xs text-ink-muted">
                {t(`catalog.types.${e.kind}`)} · {e.kind === 'gaps' ? e.version.questions[0]?.prompt : t('editor.questionsCount', { n: e.version.questions.length })} · v{e.version.number}
                {e.copied_from_id ? ` · ${t('materials.copiedFrom')}` : ''}
              </p>
            </button>
            {e.hidden && <Badge icon={<EyeOff size={12} />}>{t('classroom.hiddenBadge')}</Badge>}
            {e.version.settings.scoring_enabled && <Badge tone="primary">{e.version.settings.category_name}</Badge>}
            <Menu label={t('chat.menu')} items={[
              { key: 'edit', label: t('common.edit'), icon: <Pencil size={16} />, onSelect: () => onEdit(e) },
              { key: 'hide', label: e.hidden ? t('editor.show') : t('editor.hide'), icon: e.hidden ? <Eye size={16} /> : <EyeOff size={16} />, onSelect: () => hide.mutate(e) },
              ...(scope === 'lesson' ? [{ key: 'copy', label: t('editor.copyToHomework'), icon: <Copy size={16} />, onSelect: () => copy.mutate(e) }] : []),
              { key: 'up', label: t('common.moveUp'), icon: <ArrowUp size={16} />, disabled: i === 0, onSelect: () => move(i, -1) },
              { key: 'down', label: t('common.moveDown'), icon: <ArrowDown size={16} />, disabled: i === items.length - 1, onSelect: () => move(i, 1) },
              { key: 'del', label: t('common.delete'), icon: <Trash2 size={16} />, tone: 'danger', onSelect: () => setDeleting(e) },
            ]} trigger={(p) => <IconButton {...p} label={t('chat.menu')}><MoreHorizontal size={18} /></IconButton>} />
          </li>
        ))}
      </ul>
      <ConfirmDialog open={!!deleting} onClose={() => setDeleting(null)} onConfirm={() => deleting && remove.mutate(deleting)} loading={remove.isPending}
        tone="danger" title={t('editor.deleteConfirmTitle')} text={t('editor.deleteConfirm')} confirmLabel={t('common.delete')} />
    </>
  );
}

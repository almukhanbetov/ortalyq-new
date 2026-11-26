'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowDown, ArrowLeft, ArrowUp, FileSearch, Pencil, Plus } from 'lucide-react';
import { Badge, Button, EmptyState, ErrorState, IconButton, Loading, buttonClass } from '@/components/ui';
import { PageCard } from '@/components/layout/PageCard';
import { ApiError } from '@/lib/api/client';
import { materialsApi } from '@/lib/api/endpoints';
import { qk } from '@/lib/queryKeys';
import { useT } from '@/lib/i18n';
import { fieldErrors } from '@/lib/errors';
import { lessonEditorUrl, materialsUrl } from '@/lib/routes';
import { Cover } from '@/features/classrooms/Cover';
import { MaterialModal } from './MaterialModal';
import { TitleModal } from './TitleModal';

/** One personal material: its cover, description and lessons. The server answers 404 for anyone but the owner. */
export function CourseDetail({ id }: { id: string }) {
  const t = useT();
  const qc = useQueryClient();
  const [creating, setCreating] = useState(false);
  const [editing, setEditing] = useState(false);
  const q = useQuery({ queryKey: qk.course(id), queryFn: () => materialsApi.course(id) });
  const invalidate = () => { qc.invalidateQueries({ queryKey: ['course', id] }); qc.invalidateQueries({ queryKey: ['courses'] }); };
  const create = useMutation({ mutationFn: (title: string) => materialsApi.createLesson(id, title), onSuccess: () => { invalidate(); setCreating(false); } });
  const reorder = useMutation({ mutationFn: (ids: string[]) => materialsApi.reorderLessons(id, ids), onSettled: invalidate });

  const back = (
    <Link href={materialsUrl()} className="mb-4 inline-flex items-center gap-1 text-sm text-ink-muted hover:text-ink"><ArrowLeft size={16} />{t('materials.backToMaterials')}</Link>
  );

  if (q.isLoading) return <PageCard maxWidth={1024}><Loading /></PageCard>;
  if (q.error instanceof ApiError && (q.error.status === 404 || q.error.status === 403)) {
    return (
      <PageCard maxWidth={1024}>
        {back}
        <EmptyState icon={<FileSearch size={26} />} title={t('materials.notFound')} text={t('materials.notFoundHint')}
          action={<Link href={materialsUrl()} className={buttonClass({ variant: 'secondary' })}>{t('materials.backToMaterials')}</Link>} />
      </PageCard>
    );
  }
  if (q.error || !q.data) return <PageCard maxWidth={1024}>{back}<ErrorState error={q.error} onRetry={() => q.refetch()} /></PageCard>;

  const course = q.data;
  const lessons = course.lessons ?? [];
  const move = (i: number, d: -1 | 1) => { const ids = lessons.map((l) => l.id); [ids[i], ids[i + d]] = [ids[i + d], ids[i]]; reorder.mutate(ids); };

  return (
    <PageCard maxWidth={1024}>
      {back}
      <div className="mb-8 flex flex-col gap-5 sm:flex-row sm:items-start">
        <Cover color={course.cover_color} title={course.title} className="aspect-[16/9] w-full shrink-0 rounded-xl sm:w-60"
          imageId={course.cover_image_id} imageSrc={course.cover_image_id ? materialsApi.coverUrl(course.cover_image_id) : undefined} />
        <div className="min-w-0 flex-1">
          <p className="text-sm font-medium text-ink-faint">{t('materials.personal')}</p>
          <h1 className="mt-1 break-words text-2xl font-bold text-ink">{course.title}</h1>
          {course.description && <p className="mt-2 max-w-2xl whitespace-pre-line text-base text-ink-body">{course.description}</p>}
          <p className="mt-2 text-sm text-ink-muted">
            {t('picker.lessonsCount', { n: course.lesson_count })}
            {course.language && <> · <span data-testid="course-language">{t(`languages.${course.language}`)}</span></>}
            {' · '}{course.sections_enabled ? t('materialForm.sectionsOnShort') : t('materialForm.sectionsOffShort')}
          </p>
          {course.tags.length > 0 && (
            <ul className="mt-2 flex flex-wrap gap-1.5" aria-label={t('materialForm.tags')} data-testid="course-tags">
              {course.tags.map((x) => <li key={x}><Badge tone="primary">{x}</Badge></li>)}
            </ul>
          )}
          <div className="mt-4 flex flex-wrap gap-2">
            <Button variant="secondary" icon={<Pencil size={16} />} onClick={() => setEditing(true)}>{t('materials.editMaterial')}</Button>
            <Button icon={<Plus size={18} />} onClick={() => setCreating(true)}>{t('materials.createLesson')}</Button>
          </div>
        </div>
      </div>

      <h2 className="mb-3 text-md font-bold text-ink">{t('materials.lessons')}</h2>
      {lessons.length === 0 ? <EmptyState title={t('materials.noLessons')} /> : (
        <ol className="flex flex-col gap-2">
          {lessons.map((l, i) => (
            <li key={l.id} className="flex items-center gap-3 rounded-xl border border-line px-4 py-3 hover:bg-surface-alt">
              <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary-soft text-sm font-bold text-primary-text">{i + 1}</span>
              <Link href={lessonEditorUrl(l.id)} className="min-w-0 flex-1">
                <p className="truncate font-medium text-ink">{l.title}</p>
                <p className="text-sm text-ink-muted">{t('picker.sectionsCount', { n: l.section_count })}{l.level ? ` · ${l.level}` : ''}{l.language ? ` · ${l.language}` : ''}</p>
              </Link>
              <IconButton label={t('common.moveUp')} disabled={i === 0 || reorder.isPending} onClick={() => move(i, -1)}><ArrowUp size={16} /></IconButton>
              <IconButton label={t('common.moveDown')} disabled={i === lessons.length - 1 || reorder.isPending} onClick={() => move(i, 1)}><ArrowDown size={16} /></IconButton>
            </li>
          ))}
        </ol>
      )}
      <TitleModal open={creating} onClose={() => setCreating(false)} title={t('materials.createLesson')} label={t('materials.lessonTitle')}
        loading={create.isPending} error={fieldErrors(create.error).title} onSubmit={(title) => create.mutate(title)} />
      <MaterialModal open={editing} onClose={() => setEditing(false)} course={course} onSaved={() => { invalidate(); setEditing(false); }} />
    </PageCard>
  );
}

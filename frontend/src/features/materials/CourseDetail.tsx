'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowDown, ArrowLeft, ArrowUp, Pencil, Plus } from 'lucide-react';
import { Button, EmptyState, IconButton, QueryState } from '@/components/ui';
import { PageCard } from '@/components/layout/PageCard';
import { materialsApi } from '@/lib/api/endpoints';
import { qk } from '@/lib/queryKeys';
import { useT } from '@/lib/i18n';
import { fieldErrors } from '@/lib/errors';
import { TitleModal } from './TitleModal';

export function CourseDetail({ id }: { id: string }) {
  const t = useT();
  const qc = useQueryClient();
  const [creating, setCreating] = useState(false);
  const [editing, setEditing] = useState(false);
  const q = useQuery({ queryKey: qk.course(id), queryFn: () => materialsApi.course(id) });
  const invalidate = () => { qc.invalidateQueries({ queryKey: ['course', id] }); qc.invalidateQueries({ queryKey: ['courses'] }); };
  const create = useMutation({ mutationFn: (title: string) => materialsApi.createLesson(id, title), onSuccess: () => { invalidate(); setCreating(false); } });
  const update = useMutation({ mutationFn: (b: { title: string; description: string }) => materialsApi.updateCourse(id, b), onSuccess: () => { invalidate(); setEditing(false); } });
  const reorder = useMutation({ mutationFn: (ids: string[]) => materialsApi.reorderLessons(id, ids), onSettled: invalidate });

  return (
    <QueryState query={q}>
      {(course) => {
        const lessons = course.lessons ?? [];
        const move = (i: number, d: -1 | 1) => { const ids = lessons.map((l) => l.id); [ids[i], ids[i + d]] = [ids[i + d], ids[i]]; reorder.mutate(ids); };
        return (
          <PageCard>
            <Link href="/materials" className="mb-4 inline-flex items-center gap-1 text-sm text-ink-muted hover:text-ink"><ArrowLeft size={16} />{t('materials.backToMaterials')}</Link>
            <div className="mb-6 flex flex-wrap items-start justify-between gap-3">
              <div className="min-w-0">
                <h1 className="text-2xl font-bold text-ink">{course.title}</h1>
                {course.description && <p className="mt-2 max-w-2xl whitespace-pre-line text-base text-ink-body">{course.description}</p>}
              </div>
              <div className="flex gap-2">
                <Button variant="secondary" icon={<Pencil size={16} />} onClick={() => setEditing(true)}>{t('materials.editCourse')}</Button>
                <Button icon={<Plus size={18} />} onClick={() => setCreating(true)}>{t('materials.createLesson')}</Button>
              </div>
            </div>
            <h2 className="mb-3 text-md font-bold text-ink">{t('materials.lessons')}</h2>
            {lessons.length === 0 ? <EmptyState title={t('materials.noLessons')} /> : (
              <ol className="flex flex-col gap-2">
                {lessons.map((l, i) => (
                  <li key={l.id} className="flex items-center gap-3 rounded-xl border border-line px-4 py-3 hover:bg-surface-alt">
                    <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary-soft text-sm font-bold text-primary-text">{i + 1}</span>
                    <Link href={`/materials/lessons/${l.id}`} className="min-w-0 flex-1">
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
            <TitleModal open={editing} onClose={() => setEditing(false)} title={t('materials.editCourse')} label={t('materials.courseTitle')}
              initial={course.title} withDescription={t('materials.courseDescription')} initialDescription={course.description}
              loading={update.isPending} error={fieldErrors(update.error).title} onSubmit={(title, description) => update.mutate({ title, description })} />
          </PageCard>
        );
      }}
    </QueryState>
  );
}

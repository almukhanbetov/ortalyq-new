'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowDown, ArrowUp, Plus } from 'lucide-react';
import { Button, EmptyState, IconButton, QueryState } from '@/components/ui';
import { PageCard } from '@/components/layout/PageCard';
import { materialsApi } from '@/lib/api/endpoints';
import { qk } from '@/lib/queryKeys';
import { useT } from '@/lib/i18n';
import { fieldErrors } from '@/lib/errors';
import { Cover } from '@/features/classrooms/Cover';
import { TitleModal } from './TitleModal';
import { MaterialsTabs } from './MaterialsTabs';

export function CourseList() {
  const t = useT();
  const qc = useQueryClient();
  const [creating, setCreating] = useState(false);
  const q = useQuery({ queryKey: qk.courses({}), queryFn: () => materialsApi.courses({ limit: 100 }) });
  const create = useMutation({
    mutationFn: ({ title, description }: { title: string; description: string }) => materialsApi.createCourse({ title, description }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['courses'] }); setCreating(false); },
  });
  const reorder = useMutation({
    mutationFn: (ids: string[]) => materialsApi.reorderCourses(ids),
    onSettled: () => qc.invalidateQueries({ queryKey: ['courses'] }),
  });

  return (
    <PageCard title={t('materials.title')} actions={<Button icon={<Plus size={18} />} onClick={() => setCreating(true)}>{t('materials.createCourse')}</Button>}>
      <MaterialsTabs current="personal" />
      <QueryState query={q}>
        {(data) =>
          data.items.length === 0 ? <EmptyState title={t('materials.noCourses')} /> : (
            <ul className="flex flex-col gap-3">
              {data.items.map((c, i) => (
                <li key={c.id} className="flex items-center gap-4 rounded-xl border border-line p-3 hover:bg-surface-alt">
                  <Cover color={c.cover_color} title={c.title} className="h-16 w-24 shrink-0 rounded-lg" />
                  <Link href={`/materials/courses/${c.id}`} className="min-w-0 flex-1">
                    <p className="truncate text-md font-medium text-ink">{c.title}</p>
                    <p className="text-sm text-ink-muted">{t('picker.lessonsCount', { n: c.lesson_count })}</p>
                  </Link>
                  <div className="flex">
                    <IconButton label={t('common.moveUp')} disabled={i === 0 || reorder.isPending}
                      onClick={() => { const ids = data.items.map((x) => x.id); [ids[i - 1], ids[i]] = [ids[i], ids[i - 1]]; reorder.mutate(ids); }}><ArrowUp size={16} /></IconButton>
                    <IconButton label={t('common.moveDown')} disabled={i === data.items.length - 1 || reorder.isPending}
                      onClick={() => { const ids = data.items.map((x) => x.id); [ids[i + 1], ids[i]] = [ids[i], ids[i + 1]]; reorder.mutate(ids); }}><ArrowDown size={16} /></IconButton>
                  </div>
                </li>
              ))}
            </ul>
          )
        }
      </QueryState>
      <TitleModal open={creating} onClose={() => setCreating(false)} title={t('materials.createCourse')} label={t('materials.courseTitle')}
        withDescription={t('materials.courseDescription')} loading={create.isPending} error={fieldErrors(create.error).title}
        onSubmit={(title, description) => create.mutate({ title, description })} />
    </PageCard>
  );
}

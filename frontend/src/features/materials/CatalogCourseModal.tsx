'use client';

import Link from 'next/link';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowRight, Plus } from 'lucide-react';
import { Badge, Button, Modal, QueryState, buttonClass, useToast } from '@/components/ui';
import { libraryApi } from '@/lib/api/endpoints';
import { qk } from '@/lib/queryKeys';
import { useT } from '@/lib/i18n';
import { errorMessage } from '@/lib/errors';
import { materialUrl } from '@/lib/routes';
import { Cover } from '@/features/classrooms/Cover';

/**
 * A catalog course before adding: description, level, the lessons. «Добавить в мои материалы» makes the teacher's own
 * copy (once: the server returns the existing copy on a repeated add); then the button opens it in «Личные материалы».
 */
export function CatalogCourseModal({ courseKey, onClose }: { courseKey: string | null; onClose: () => void }) {
  const t = useT();
  const toast = useToast();
  const qc = useQueryClient();
  const q = useQuery({ queryKey: qk.libraryCourse(courseKey ?? ''), queryFn: () => libraryApi.get(courseKey!), enabled: !!courseKey });
  const add = useMutation({
    mutationFn: () => libraryApi.add(courseKey!),
    onSuccess: (r) => {
      qc.invalidateQueries({ queryKey: ['material-catalog'] });
      qc.invalidateQueries({ queryKey: ['courses'] });
      toast(r.created ? t('materials.catalogAdded') : t('materials.catalogAlready'), 'success');
    },
    onError: (e) => toast(errorMessage(e, t), 'error'),
  });
  const owned = q.data?.owned_course_id ?? null;

  return (
    <Modal open={!!courseKey} onClose={onClose} size="lg" title={q.data?.title ?? t('materials.catalog')}
      footer={q.data && (
        <div className="flex w-full flex-wrap items-center justify-between gap-3">
          <p className="max-w-[420px] text-xs text-ink-muted">{owned ? t('materials.catalogOwnedHint') : t('materials.catalogAddHint')}</p>
          {owned ? (
            <Link href={materialUrl(owned)} className={buttonClass()}>
              {t('materials.openInMine')} <ArrowRight size={18} />
            </Link>
          ) : (
            <Button icon={<Plus size={18} />} loading={add.isPending} onClick={() => add.mutate()}>{t('materials.addToMine')}</Button>
          )}
        </div>
      )}>
      <QueryState query={q}>
        {(c) => (
          <div className="flex flex-col gap-5">
            <div className="flex flex-col gap-4 sm:flex-row">
              <Cover color={c.cover_color} title={c.title} className="aspect-[16/9] w-full shrink-0 rounded-xl sm:w-56" />
              <div className="flex flex-col gap-2">
                <div className="flex flex-wrap gap-2">
                  <Badge tone="info">{t('materials.level', { level: c.level })}</Badge>
                  <Badge>{c.language}</Badge>
                  <Badge>{t('picker.lessonsCount', { n: c.lesson_count })}</Badge>
                </div>
                <p className="whitespace-pre-line text-base text-ink-body">{c.description}</p>
              </div>
            </div>
            <section aria-labelledby="catalog-lessons">
              <h3 id="catalog-lessons" className="mb-2 text-md font-bold text-ink">{t('materials.lessons')}</h3>
              <ol className="flex flex-col gap-2">
                {c.lessons.map((l, i) => (
                  <li key={l.title} className="flex items-start gap-3 rounded-lg border border-line px-3 py-2.5">
                    <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-primary-soft text-sm font-bold text-primary-text">{i + 1}</span>
                    <div className="min-w-0">
                      <p className="font-medium text-ink">{l.title}</p>
                      <p className="text-xs text-ink-muted">
                        {t('materials.catalogLessonMeta', { level: l.level, min: l.duration_min, texts: l.text_blocks, ex: l.exercises, hw: l.homework })}
                      </p>
                      <p className="text-xs text-ink-faint">{l.sections.join(' · ')}</p>
                    </div>
                  </li>
                ))}
              </ol>
            </section>
          </div>
        )}
      </QueryState>
    </Modal>
  );
}

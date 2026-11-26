'use client';

import { AlignLeft, EyeOff } from 'lucide-react';
import { Badge } from '@/components/ui';
import { useT } from '@/lib/i18n';
import type { ExerciseView } from '@/types/api';

/** «Name: line» at the start of a line is a dialogue speaker; the name is shown in bold. */
const SPEAKER = /^([\p{L}][\p{L}' .-]{0,30}):\s/u;

/**
 * «Текст» in an open lesson: a block for reading and speaking practice. No answer field, no attempt and no grade.
 * A block hidden from the student («только для преподавателя») reaches only the teacher: the server leaves it out
 * of the student's list.
 */
export function TextBlock({ view, teacher, index }: { view: ExerciseView; teacher: boolean; index: number }) {
  const t = useT();
  const v = view.version;
  const body = v.questions[0]?.prompt ?? '';
  return (
    <article className="rounded-2xl border border-line bg-surface p-5 shadow-card sm:p-6" aria-labelledby={`ex-${view.exercise_id}`}>
      <header className="mb-3 flex flex-wrap items-start justify-between gap-3">
        <div>
          <p className="inline-flex items-center gap-1.5 text-xs font-medium uppercase tracking-wide text-ink-muted"><AlignLeft size={13} />{t('catalog.types.text')} · {index + 1}</p>
          <h3 id={`ex-${view.exercise_id}`} className="mt-1 text-lg font-bold text-ink">{v.title}</h3>
        </div>
        {teacher && view.hidden && <Badge tone="neutral" icon={<EyeOff size={12} />}>{t('text.teacherOnly')}</Badge>}
      </header>
      <div className="flex flex-col gap-1.5 text-md leading-relaxed text-ink-body">
        {body.split('\n').map((line, i) => {
          if (!line.trim()) return <span key={i} aria-hidden className="h-2" />;
          const m = SPEAKER.exec(line);
          return (
            <p key={i} className="whitespace-pre-wrap break-words">
              {m ? <><strong className="font-semibold text-ink">{m[1]}:</strong> {line.slice(m[0].length)}</> : line}
            </p>
          );
        })}
      </div>
    </article>
  );
}

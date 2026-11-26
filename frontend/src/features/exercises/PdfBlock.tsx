'use client';

import { ExternalLink, EyeOff, FileText } from 'lucide-react';
import { Badge, buttonClass, cn } from '@/components/ui';
import { formatBytes } from '@/lib/format';
import { useT } from '@/lib/i18n';
import type { ExerciseView, LessonFile } from '@/types/api';

/**
 * A PDF shown inside the page by the browser's own viewer, with «Открыть PDF в новой вкладке» for browsers that do
 * not show PDFs in a page (most phones). The address is an API route that checks access on every request, so the
 * same link opened directly by someone else is refused.
 */
export function PdfViewer({ url, title, file, className }: { url: string; title: string; file: LessonFile; className?: string }) {
  const t = useT();
  // The route of a block stays the same when its file is replaced: the file id in the address makes the frame and the
  // browser cache load the new document (the server ignores it and serves the block's current file).
  const src = url.startsWith('blob:') ? url : `${url}?f=${file.id}`;
  return (
    <div className={cn('flex flex-col gap-2', className)} data-testid="pdf-viewer">
      <iframe key={src} src={src} title={t('pdf.frameTitle', { title })} className="h-[70vh] max-h-[760px] min-h-[420px] w-full rounded-xl border border-line bg-surface-alt" />
      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="inline-flex min-w-0 items-center gap-1.5 text-sm text-ink-muted">
          <FileText size={15} className="shrink-0" />
          <span className="truncate">{file.name}</span>
          <span className="shrink-0">· {formatBytes(file.size_bytes)}</span>
        </p>
        <a href={src} target="_blank" rel="noopener noreferrer" className={buttonClass({ variant: 'secondary', size: 'sm' })}>
          <ExternalLink size={16} />{t('pdf.open')}
        </a>
      </div>
      <p className="text-xs text-ink-muted">{t('pdf.fallback')}</p>
    </div>
  );
}

/**
 * «PDF-документ» in an open lesson: a document to read. No answer field, no attempt and no grade. A block hidden from
 * the student reaches only the teacher: the server leaves it (and its file) out for the student.
 */
export function PdfBlock({ view, teacher, index, url }: { view: ExerciseView; teacher: boolean; index: number; url: string }) {
  const t = useT();
  const v = view.version;
  return (
    <article className="rounded-2xl border border-line bg-surface p-5 shadow-card sm:p-6" aria-labelledby={`ex-${view.exercise_id}`} data-testid="pdf-block">
      <header className="mb-3 flex flex-wrap items-start justify-between gap-3">
        <div>
          <p className="inline-flex items-center gap-1.5 text-xs font-medium uppercase tracking-wide text-ink-muted"><FileText size={13} />{t('catalog.types.pdf')} · {index + 1}</p>
          <h3 id={`ex-${view.exercise_id}`} className="mt-1 text-lg font-bold text-ink">{v.title}</h3>
        </div>
        {teacher && view.hidden && <Badge tone="neutral" icon={<EyeOff size={12} />}>{t('text.teacherOnly')}</Badge>}
      </header>
      {v.file ? <PdfViewer url={url} title={v.title} file={v.file} /> : <p className="text-sm text-ink-muted">{t('errors.not_found')}</p>}
    </article>
  );
}

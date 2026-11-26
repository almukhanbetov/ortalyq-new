'use client';

import { useEffect, useRef, useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { CheckCircle2, FilePlus2, FileText, RefreshCw } from 'lucide-react';
import { Button, Spinner, cn } from '@/components/ui';
import { exercisesApi } from '@/lib/api/endpoints';
import { ApiError } from '@/lib/api/client';
import { errorMessage } from '@/lib/errors';
import { formatBytes } from '@/lib/format';
import { useT } from '@/lib/i18n';
import type { Exercise } from '@/types/api';
import type { TestDraft } from './testDraft';
import { PDF_MAX_BYTES, looksLikePdf } from './pdfDraft';
import { PdfViewer } from './PdfBlock';

/**
 * «PDF-документ» editor: choose a PDF from the computer, see its name and the upload state, replace it. The file is
 * uploaded at once (the server checks that it is a real PDF of at most 20 MB) and the block refers to it when saved.
 * The preview shows the saved file through the API, or the newly chosen file from the computer before saving.
 */
export function PdfEditorFields({ lessonId, exercise, draft, setDraft, errors, onBusy }: {
  lessonId: string; exercise?: Exercise; draft: TestDraft; setDraft: (fn: (d: TestDraft) => TestDraft) => void;
  errors: Record<string, string>; onBusy: (busy: boolean) => void;
}) {
  const t = useT();
  const input = useRef<HTMLInputElement>(null);
  const [error, setError] = useState<string | null>(null);
  const [local, setLocal] = useState<{ id: string; url: string } | null>(null); // the chosen file, before saving

  useEffect(() => () => { if (local) URL.revokeObjectURL(local.url); }, [local]);

  const upload = useMutation({
    mutationFn: (f: File) => exercisesApi.uploadFile(lessonId, f),
    onMutate: () => { setError(null); onBusy(true); },
    onSuccess: (file, f) => {
      setDraft((d) => ({ ...d, file, title: d.title.trim() ? d.title : f.name.replace(/\.pdf$/i, '').slice(0, 200) }));
      setLocal({ id: file.id, url: URL.createObjectURL(f) });
    },
    onError: (e, f) => {
      if (e instanceof ApiError && e.code === 'pdf_type') setError(t('pdf.notPdf', { name: f.name }));
      else if (e instanceof ApiError && e.code === 'file_too_large') setError(t('pdf.tooLarge', { name: f.name }));
      else setError(t('pdf.uploadFailed', { name: f.name, reason: errorMessage(e, t) }));
    },
    onSettled: () => onBusy(false),
  });

  const pick = (f: File | undefined) => {
    if (!f) return;
    if (!looksLikePdf(f)) return setError(t('pdf.notPdf', { name: f.name }));
    if (f.size > PDF_MAX_BYTES) return setError(t('pdf.tooLarge', { name: f.name }));
    upload.mutate(f);
  };

  const file = draft.file;
  const saved = !!exercise && !!file && exercise.version.file?.id === file.id;
  const previewUrl = !file ? null : saved ? exercisesApi.fileUrl(exercise!.id) : local?.id === file.id ? local.url : null;
  // «Выберите PDF-файл» of an earlier save is answered by choosing a file.
  const shownError = error ?? (file ? null : errors.file_id) ?? null;

  return (
    <div className="flex flex-col gap-3" data-testid="pdf-editor">
      <p className="text-base font-medium text-ink">{t('pdf.file')}<span className="text-danger-text"> *</span></p>
      <div className={cn('flex flex-wrap items-center justify-between gap-3 rounded-xl border p-4', shownError ? 'border-danger bg-danger-soft/40' : 'border-line bg-surface-alt')}>
        <div className="flex min-w-0 items-center gap-3" aria-live="polite" data-testid="pdf-file-state">
          <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-primary-soft text-primary-text">
            {upload.isPending ? <Spinner /> : <FileText size={20} />}
          </span>
          <div className="min-w-0">
            {upload.isPending ? (
              <p className="truncate text-sm font-medium text-ink">{t('pdf.uploading', { name: upload.variables?.name ?? '' })}</p>
            ) : file ? (
              <>
                <p className="truncate text-sm font-medium text-ink" data-testid="pdf-file-name">{file.name}</p>
                <p className="inline-flex items-center gap-1 text-xs text-success-text">
                  <CheckCircle2 size={13} />{saved ? t('pdf.current') : t('pdf.uploaded')} · {formatBytes(file.size_bytes)}
                </p>
              </>
            ) : (
              <p className="text-sm text-ink-muted">{t('pdf.hint')}</p>
            )}
          </div>
        </div>
        <Button variant={file ? 'secondary' : 'primary'} size="sm" icon={file ? <RefreshCw size={16} /> : <FilePlus2 size={16} />}
          disabled={upload.isPending} onClick={() => input.current?.click()}>
          {file ? t('pdf.replace') : t('pdf.choose')}
        </Button>
        <input ref={input} type="file" accept="application/pdf,.pdf" hidden aria-hidden tabIndex={-1} data-testid="pdf-input"
          onChange={(e) => { pick(e.target.files?.[0]); e.target.value = ''; }} />
      </div>
      {shownError && <p role="alert" className="text-sm text-danger-text">{shownError}</p>}
      {file && !saved && exercise && <p className="text-xs text-ink-muted">{t('pdf.replaceNote')}</p>}
      {file && previewUrl && (
        <div>
          <p className="mb-2 text-sm font-medium text-ink-body">{t('pdf.preview')}</p>
          <PdfViewer url={previewUrl} title={draft.title || file.name} file={file} />
        </div>
      )}
    </div>
  );
}

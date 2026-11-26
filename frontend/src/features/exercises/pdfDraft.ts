// «PDF-документ»: a read-only block of a lesson section that shows a PDF file. Stored like a text block without
// questions; the version keeps the title and the uploaded file. The student only reads it.
import type { TFunction } from '@/lib/i18n';
import { DEFAULT_SETTINGS, type TestDraft } from './testDraft';

/** The same limit as the server (uploads.PDFMaxBytes); the server checks the real content and size again. */
export const PDF_MAX_BYTES = 20 * 1024 * 1024;

export function newPdfDraft(): TestDraft {
  return { title: '', hidden: false, settings: { ...DEFAULT_SETTINGS }, questions: [], file: null };
}

/** Errors keyed like the API ("file_id"). */
export function validatePdfDraft(d: TestDraft, t: TFunction): Record<string, string> {
  const e: Record<string, string> = {};
  if (!d.title.trim()) e.title = t('common.required');
  if (!d.file) e.file_id = t('pdf.required');
  return e;
}

/** A file the browser says is a PDF (by type or by name); the server checks the content. */
export const looksLikePdf = (f: File) => f.type === 'application/pdf' || /\.pdf$/i.test(f.name);


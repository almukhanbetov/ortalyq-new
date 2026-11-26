// «Текст»: a read-only block of a lesson section (a dialogue, useful phrases, a speaking task). Stored like a test
// with one question whose prompt is the text and no options; the student only reads it.
import type { TFunction } from '@/lib/i18n';
import { DEFAULT_SETTINGS, type TestDraft } from './testDraft';

export const TEXT_MAX = 2000;

const key = () => Math.random().toString(36).slice(2, 10);

export function newTextDraft(): TestDraft {
  return { title: '', hidden: false, settings: { ...DEFAULT_SETTINGS }, questions: [{ key: key(), prompt: '', multiple: false, options: [] }] };
}

/** Errors keyed like the API ("questions.0.prompt"). */
export function validateTextDraft(d: TestDraft, t: TFunction): Record<string, string> {
  const e: Record<string, string> = {};
  if (!d.title.trim()) e.title = t('common.required');
  const body = d.questions[0]?.prompt.trim() ?? '';
  if (!body) e['questions.0.prompt'] = t('common.required');
  else if ([...body].length > TEXT_MAX) e['questions.0.prompt'] = t('editor.textTooLong', { n: TEXT_MAX });
  return e;
}

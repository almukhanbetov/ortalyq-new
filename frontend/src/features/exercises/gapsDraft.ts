// «Слова и пропуски», first step: one phrase with one gap («I ___ a student.») and the accepted word («am»).
// Stored like a test with one question (the phrase) and one correct option (the word); the server is authoritative.
import type { TFunction } from '@/lib/i18n';
import { DEFAULT_SETTINGS, type TestDraft } from './testDraft';

/** The gap marker: three or more underscores (same rule as the server). */
export const GAP_MARKER = /_{3,}/g;
export const GAP_TOKEN = '___';
export const GAP_ANSWER_MAX = 100;

export function gapCount(prompt: string) {
  return (prompt.match(GAP_MARKER) ?? []).length;
}

/** The text before and after the first gap, or null when the phrase has no gap. */
export function splitGap(prompt: string): [string, string] | null {
  const m = /_{3,}/.exec(prompt);
  return m ? [prompt.slice(0, m.index), prompt.slice(m.index + m[0].length)] : null;
}

const key = () => Math.random().toString(36).slice(2, 10);

export function newGapsDraft(): TestDraft {
  return {
    title: '', hidden: false, settings: { ...DEFAULT_SETTINGS },
    questions: [{ key: key(), prompt: '', multiple: false, options: [{ key: key(), label: '', correct: true }] }],
  };
}

/** Errors keyed like the API ("questions.0.prompt", "questions.0.options.0.label"). */
export function validateGapsDraft(d: TestDraft, t: TFunction): Record<string, string> {
  const e: Record<string, string> = {};
  if (!d.title.trim()) e.title = t('common.required');
  const q = d.questions[0];
  const prompt = q?.prompt.trim() ?? '';
  const n = gapCount(prompt);
  if (!prompt) e['questions.0.prompt'] = t('common.required');
  else if (n === 0) e['questions.0.prompt'] = t('editor.gapNoMarker');
  else if (n > 1) e['questions.0.prompt'] = t('editor.gapManyMarkers');
  if (!q?.options[0]?.label.trim()) e['questions.0.options.0.label'] = t('common.required');
  if (d.settings.timer_enabled && (!d.settings.timer_seconds || d.settings.timer_seconds < 10)) e['settings.timer_seconds'] = t('common.required');
  return e;
}

// Editor draft model for the "test" exercise and its client-side validation
// (mirrors the server rules; the server remains authoritative).
import type { Exercise, ExerciseSettings, SaveExerciseRequest } from '@/types/api';
import type { TFunction } from '@/lib/i18n';

export interface DraftOption { key: string; label: string; correct: boolean }
export interface DraftQuestion { key: string; prompt: string; multiple: boolean; options: DraftOption[] }
export interface TestDraft { title: string; hidden: boolean; settings: ExerciseSettings; questions: DraftQuestion[] }

const key = () => Math.random().toString(36).slice(2, 10);

export const DEFAULT_SETTINGS: ExerciseSettings = {
  check_mode: 'instant', timer_enabled: false, timer_seconds: null, scoring_enabled: false, points_per_correct: 1, category_id: null,
};

export function emptyQuestion(): DraftQuestion {
  return { key: key(), prompt: '', multiple: false, options: [{ key: key(), label: '', correct: false }, { key: key(), label: '', correct: false }] };
}

export function emptyOption(): DraftOption {
  return { key: key(), label: '', correct: false };
}

export function newDraft(): TestDraft {
  return { title: '', hidden: false, settings: { ...DEFAULT_SETTINGS }, questions: [emptyQuestion()] };
}

export function draftFromExercise(e: Exercise): TestDraft {
  return {
    title: e.version.title,
    hidden: e.hidden,
    settings: { ...e.version.settings },
    questions: e.version.questions.map((q) => ({
      key: key(), prompt: q.prompt, multiple: q.multiple,
      options: q.options.map((o) => ({ key: key(), label: o.label, correct: !!o.is_correct })),
    })),
  };
}

export function toRequest(d: TestDraft): SaveExerciseRequest {
  return {
    title: d.title.trim(),
    hidden: d.hidden,
    settings: { ...d.settings, timer_seconds: d.settings.timer_enabled ? d.settings.timer_seconds : null },
    questions: d.questions.map((q) => ({
      prompt: q.prompt.trim(), multiple: q.multiple,
      options: q.options.map((o) => ({ label: o.label.trim(), is_correct: o.correct })),
    })),
  };
}

/** Returns errors keyed like the API ("questions.0.prompt"). */
export function validateDraft(d: TestDraft, t: TFunction): Record<string, string> {
  const e: Record<string, string> = {};
  if (!d.title.trim()) e.title = t('common.required');
  d.questions.forEach((q, i) => {
    const k = `questions.${i}`;
    if (!q.prompt.trim()) e[`${k}.prompt`] = t('common.required');
    if (q.options.length < 2) e[`${k}.options`] = t('common.required');
    q.options.forEach((o, j) => {
      if (!o.label.trim()) e[`${k}.options.${j}.label`] = t('common.required');
    });
    const correct = q.options.filter((o) => o.correct).length;
    if (correct === 0) e[`${k}.correct`] = t('editor.correctHint');
  });
  if (d.settings.timer_enabled && (!d.settings.timer_seconds || d.settings.timer_seconds < 10)) e['settings.timer_seconds'] = t('common.required');
  return e;
}

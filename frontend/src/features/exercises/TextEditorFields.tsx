'use client';

import { Field, Textarea } from '@/components/ui';
import { useT } from '@/lib/i18n';
import type { TestDraft } from './testDraft';
import { TEXT_MAX } from './textDraft';

/** «Текст» editor: the text of the block. Line breaks are kept; «Name: line» is shown as a dialogue line. */
export function TextEditorFields({ draft, setDraft, errors }: { draft: TestDraft; setDraft: (fn: (d: TestDraft) => TestDraft) => void; errors: Record<string, string> }) {
  const t = useT();
  const body = draft.questions[0]?.prompt ?? '';
  const setBody = (prompt: string) => setDraft((d) => ({ ...d, questions: [{ ...d.questions[0], prompt, multiple: false, options: [] }] }));
  return (
    <Field label={t('editor.textBody')} hint={`${t('editor.textBodyHint')} ${[...body].length} / ${TEXT_MAX}`} error={errors['questions.0.prompt'] || errors.questions} required>
      {(p) => <Textarea {...p} rows={12} value={body} maxLength={TEXT_MAX} placeholder={t('editor.textBodyPlaceholder')} onChange={(e) => setBody(e.target.value)} />}
    </Field>
  );
}

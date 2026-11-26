'use client';

import { useRef } from 'react';
import { Plus } from 'lucide-react';
import { Button, Field, Input, Textarea } from '@/components/ui';
import { useT } from '@/lib/i18n';
import type { TestDraft } from './testDraft';
import { GAP_ANSWER_MAX, GAP_TOKEN, splitGap } from './gapsDraft';

/** «Слова и пропуски» editor: the phrase with one gap, the accepted word and a preview of what the student sees. */
export function GapsEditorFields({ draft, setDraft, errors }: { draft: TestDraft; setDraft: (fn: (d: TestDraft) => TestDraft) => void; errors: Record<string, string> }) {
  const t = useT();
  const area = useRef<HTMLTextAreaElement>(null);
  const q = draft.questions[0];
  const answer = q.options[0]?.label ?? '';
  const setPrompt = (prompt: string) => setDraft((d) => ({ ...d, questions: [{ ...d.questions[0], prompt }] }));
  const setAnswer = (label: string) =>
    setDraft((d) => ({ ...d, questions: [{ ...d.questions[0], multiple: false, options: [{ ...(d.questions[0].options[0] ?? { key: 'a' }), label, correct: true }] }] }));

  // Puts the marker at the caret (or replaces the selection), so the teacher does not have to type underscores.
  const insertGap = () => {
    const el = area.current;
    const start = el?.selectionStart ?? q.prompt.length;
    const end = el?.selectionEnd ?? q.prompt.length;
    const before = q.prompt.slice(0, start);
    const after = q.prompt.slice(end);
    const pad = (s: string, side: 'l' | 'r') => (s && !/\s$|^\s/.test(side === 'l' ? s.slice(-1) : s[0]) ? ' ' : '');
    const next = `${before}${pad(before, 'l')}${GAP_TOKEN}${pad(after, 'r')}${after}`;
    setPrompt(next);
    requestAnimationFrame(() => {
      const pos = next.length - after.length;
      el?.focus();
      el?.setSelectionRange(pos, pos);
    });
  };

  const parts = splitGap(q.prompt);
  return (
    <div className="flex flex-col gap-5">
      <Field label={t('editor.gapPhrase')} hint={t('editor.gapPhraseHint')} error={errors['questions.0.prompt']} required>
        {(p) => (
          <div className="flex flex-col gap-2">
            <Textarea {...p} ref={area} rows={2} value={q.prompt} maxLength={2000} placeholder={t('editor.gapPhrasePlaceholder')} onChange={(e) => setPrompt(e.target.value)} />
            <Button variant="ghost" size="sm" className="self-start text-primary-text" icon={<Plus size={16} />} onClick={insertGap}>{t('editor.gapInsert')}</Button>
          </div>
        )}
      </Field>
      <Field label={t('editor.gapAnswer')} hint={t('editor.gapAnswerHint')} error={errors['questions.0.options.0.label'] || errors['questions.0.options']} required className="max-w-sm">
        {(p) => <Input {...p} value={answer} maxLength={GAP_ANSWER_MAX} placeholder={t('editor.gapAnswerPlaceholder')} autoComplete="off" onChange={(e) => setAnswer(e.target.value)} />}
      </Field>
      {parts && (
        <section aria-label={t('editor.gapPreview')} className="rounded-xl border border-line bg-surface-alt p-4">
          <p className="mb-2 text-xs font-medium uppercase tracking-wide text-ink-muted">{t('editor.gapPreview')}</p>
          <p className="text-md text-ink">
            {parts[0]}
            <span className="mx-1 inline-block min-w-[96px] rounded-md border-2 border-dashed border-line-strong bg-surface px-2 py-0.5 text-center text-ink-faint" aria-hidden>&nbsp;</span>
            {parts[1]}
          </p>
        </section>
      )}
    </div>
  );
}

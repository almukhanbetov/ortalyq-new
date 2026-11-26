'use client';

import { Check, X } from 'lucide-react';
import { Button, cn } from '@/components/ui';
import { useT } from '@/lib/i18n';
import type { AnswerView, Question } from '@/types/api';
import { GAP_ANSWER_MAX, splitGap } from './gapsDraft';

/**
 * One «Слова и пропуски» phrase in the exercise player: the student types the missing word into the gap.
 * The verdict and the accepted word come only from the server (after the instant check or after finishing);
 * the teacher sees the student's word and the key.
 */
export function GapQuestion({ q, index, answer, teacher, finished, instant, interactive, text, busy, onText, onSave }: {
  q: Question;
  index: number;
  answer?: AnswerView;
  teacher: boolean;
  finished: boolean;
  instant: boolean;
  interactive: boolean;
  /** The word being typed (not saved yet); undefined = show the saved one. */
  text?: string;
  busy: boolean;
  onText: (v: string) => void;
  onSave: (text: string, check: boolean) => void;
}) {
  const t = useT();
  const saved = answer?.text ?? '';
  const value = text ?? saved;
  const parts = splitGap(q.prompt) ?? [q.prompt + ' ', ''];
  const editable = interactive && !answer?.checked;
  const revealed = !!answer?.result && (teacher || finished || (instant && answer.checked));
  const result = revealed ? answer!.result : null;
  // The key: from the server once revealed; the teacher also has it in the full version.
  const key = answer?.correct_answers?.length ? answer.correct_answers : teacher ? q.options.filter((o) => o.is_correct !== false).map((o) => o.label) : [];
  const inputId = `gap-${q.id}`;

  const save = (check: boolean) => {
    const v = value.trim();
    if (check ? !v : v === saved) return;
    onSave(v, check);
  };

  const box = cn(
    'mx-1 inline-flex h-10 min-w-[110px] items-center rounded-lg border-2 px-2 align-middle text-base',
    result === 'correct' && 'border-success bg-success-soft text-success-text',
    result === 'wrong' && 'border-danger bg-danger-soft text-danger-text',
    (result === 'unanswered' || !result) && 'border-line-strong bg-surface text-ink',
  );

  return (
    <li>
      <p className="mb-1 text-xs text-ink-muted">{t('test.gapHint')}{!instant && !finished && !teacher ? ` · ${t('test.afterHint')}` : ''}</p>
      <div id={`q-${q.id}`} className="text-md leading-[3rem] text-ink">
        <span className="font-medium">{index + 1}. </span>
        {parts[0]}
        {editable ? (
          <input id={inputId} type="text" value={value} maxLength={GAP_ANSWER_MAX} autoComplete="off" autoCapitalize="off" spellCheck={false}
            aria-label={t('test.gapInput')} size={Math.max(8, value.length + 2)}
            onChange={(e) => onText(e.target.value)}
            // «Check after finishing»: the word is saved when the student leaves the gap. With the instant check the
            // word is sent by «Проверить»/Enter, so a blur never races the check click.
            onBlur={() => { if (!instant) save(false); }}
            onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); save(instant); } }}
            className="mx-1 h-10 min-w-[110px] rounded-lg border-2 border-primary bg-surface px-2 align-middle text-base text-ink focus:shadow-focus focus:outline-none" />
        ) : (
          <span className={box} aria-label={t('test.gapStudentAnswer')}>
            {saved || <span className="text-ink-faint">{t('test.gapNoAnswer')}</span>}
          </span>
        )}
        {parts[1]}
      </div>
      <div className="mt-1 flex flex-wrap items-center gap-3 text-sm">
        {editable && instant && (
          <Button size="sm" variant="soft" disabled={!value.trim() || busy} onClick={() => save(true)}>{t('test.check')}</Button>
        )}
        {editable && !instant && saved && value.trim() === saved && <span className="text-ink-muted">{t('test.gapSaved')}</span>}
        {result === 'correct' && <span className="inline-flex items-center gap-1 font-medium text-success-text"><Check size={16} />{t('test.correct')}</span>}
        {result === 'wrong' && <span className="inline-flex items-center gap-1 font-medium text-danger-text"><X size={16} />{t('test.wrong')}</span>}
        {(result === 'wrong' || result === 'unanswered' || (teacher && !revealed)) && key.length > 0 && (
          <span className="text-success-text">{t('test.correctAnswer')}: <strong>{key.join(', ')}</strong></span>
        )}
      </div>
    </li>
  );
}

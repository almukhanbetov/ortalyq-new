'use client';

import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowDown, ArrowUp, Plus, Trash2, X } from 'lucide-react';
import {
  Button, ChoiceMark, ConfirmDialog, EmptyState, Field, IconButton, Input, Modal, ProgressBar, QueryState, RadioCard, Select, Switch, Tabs, Textarea, cn, useToast,
} from '@/components/ui';
import { exercisesApi } from '@/lib/api/endpoints';
import { qk } from '@/lib/queryKeys';
import { useT } from '@/lib/i18n';
import { errorMessage, fieldErrors } from '@/lib/errors';
import type { Exercise, ExerciseKind, Scope } from '@/types/api';
import { draftFromExercise, emptyOption, emptyQuestion, newDraft, toRequest, validateDraft, type DraftQuestion, type TestDraft } from './testDraft';
import { newGapsDraft, validateGapsDraft } from './gapsDraft';
import { GapsEditorFields } from './GapsEditorFields';

type Tab = 'exercise' | 'settings' | 'stats';

export interface EditorTarget {
  lessonId: string;
  scope: Scope;
  sectionId: string | null;
  exercise?: Exercise; // undefined = create
  /** Kind of a new exercise (from the catalog); an existing one keeps its own kind. */
  kind?: ExerciseKind;
}

const KINDS: ExerciseKind[] = ['test', 'gaps'];
export const isExerciseKind = (k: string): k is ExerciseKind => (KINDS as string[]).includes(k);

export function ExerciseEditorModal({ target, onClose }: { target: EditorTarget | null; onClose: () => void }) {
  const t = useT();
  const toast = useToast();
  const qc = useQueryClient();
  const [tab, setTab] = useState<Tab>('exercise');
  const [draft, setDraft] = useState<TestDraft>(newDraft);
  const [initial, setInitial] = useState('');
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [confirmClose, setConfirmClose] = useState(false);
  const kind: ExerciseKind = (target?.exercise?.kind as ExerciseKind | undefined) ?? target?.kind ?? 'test';
  const gaps = kind === 'gaps';

  useEffect(() => {
    if (!target) return;
    const d = target.exercise ? draftFromExercise(target.exercise) : target.kind === 'gaps' ? newGapsDraft() : newDraft();
    setDraft(d);
    setInitial(JSON.stringify(d));
    setErrors({});
    setTab('exercise');
  }, [target]);

  const dirty = useMemo(() => JSON.stringify(draft) !== initial, [draft, initial]);
  const categories = useQuery({ queryKey: qk.categories, queryFn: exercisesApi.categories, enabled: !!target });

  const save = useMutation({
    mutationFn: async () => {
      const body = toRequest(draft);
      if (target!.exercise) return exercisesApi.update(target!.exercise.id, body);
      return exercisesApi.create(target!.lessonId, { ...body, scope: target!.scope, section_id: target!.sectionId, kind });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['editor-exercises', target!.lessonId] });
      qc.invalidateQueries({ queryKey: ['lesson-exercises'] });
      toast(t('common.save') + ' ✓', 'success');
      onClose();
    },
    onError: (e) => {
      const f = fieldErrors(e);
      setErrors(f);
      if (Object.keys(f).some((k) => k.startsWith('settings'))) setTab('settings');
      else if (Object.keys(f).length) setTab('exercise');
    },
  });

  const submit = () => {
    const errs = gaps ? validateGapsDraft(draft, t) : validateDraft(draft, t);
    setErrors(errs);
    if (Object.keys(errs).length) {
      setTab(Object.keys(errs).some((k) => k.startsWith('settings')) ? 'settings' : 'exercise');
      toast(t('editor.fixErrors'), 'error');
      return;
    }
    save.mutate();
  };

  const requestClose = () => (dirty ? setConfirmClose(true) : onClose());

  const setQ = (i: number, fn: (q: DraftQuestion) => DraftQuestion) =>
    setDraft((d) => ({ ...d, questions: d.questions.map((q, k) => (k === i ? fn(q) : q)) }));
  const moveQ = (i: number, dir: -1 | 1) =>
    setDraft((d) => {
      const qs = [...d.questions];
      const j = i + dir;
      if (j < 0 || j >= qs.length) return d;
      [qs[i], qs[j]] = [qs[j], qs[i]];
      return { ...d, questions: qs };
    });

  const s = draft.settings;
  const setS = (patch: Partial<typeof s>) => setDraft((d) => ({ ...d, settings: { ...d.settings, ...patch } }));

  return (
    <>
      <Modal open={!!target} onClose={requestClose} size="lg" bodyClassName="px-0 py-0"
        title={gaps ? (target?.exercise ? t('editor.titleEditGaps') : t('editor.titleNewGaps')) : target?.exercise ? t('editor.titleEdit') : t('editor.titleNew')}
        footer={
          <div className="flex w-full flex-wrap items-center justify-between gap-3">
            <p className="max-w-[420px] text-xs text-ink-muted">{target?.exercise ? t('editor.versionNote') : ''}</p>
            <div className="flex gap-2">
              <Button variant="secondary" onClick={requestClose}>{t('common.cancel')}</Button>
              <Button onClick={submit} loading={save.isPending}>{t('common.save')}</Button>
            </div>
          </div>
        }>
        <div className="sticky top-0 z-10 border-b border-line bg-surface px-6">
          <Tabs<Tab> idPrefix="ed" value={tab} onChange={setTab}
            items={[
              { value: 'exercise', label: t('editor.tabExercise') },
              { value: 'settings', label: t('editor.tabSettings') },
              ...(target?.exercise ? [{ value: 'stats' as Tab, label: t('editor.tabStats') }] : []),
            ]} />
        </div>

        <div className="px-6 py-5" role="tabpanel" aria-labelledby={`ed-tab-${tab}`}>
          {save.error && !Object.keys(fieldErrors(save.error)).length && (
            <p role="alert" className="mb-4 rounded-md bg-danger-soft px-3 py-2 text-sm text-danger-text">{errorMessage(save.error, t)}</p>
          )}

          {tab === 'exercise' && gaps && (
            <div className="flex flex-col gap-5">
              <Field label={t('editor.title')} error={errors.title} required>
                {(p) => <Input {...p} data-autofocus value={draft.title} maxLength={200} placeholder={t('catalog.types.gaps')} onChange={(e) => setDraft((d) => ({ ...d, title: e.target.value }))} />}
              </Field>
              <GapsEditorFields draft={draft} setDraft={setDraft} errors={errors} />
            </div>
          )}

          {tab === 'exercise' && !gaps && (
            <div className="flex flex-col gap-5">
              <Field label={t('editor.title')} error={errors.title} required>
                {(p) => <Input {...p} data-autofocus value={draft.title} maxLength={200} placeholder={t('editor.titlePlaceholder')} onChange={(e) => setDraft((d) => ({ ...d, title: e.target.value }))} />}
              </Field>

              {draft.questions.map((q, i) => {
                const base = `questions.${i}`;
                const type = q.multiple ? 'checkbox' : 'radio';
                return (
                  <fieldset key={q.key} className="rounded-xl border border-line bg-surface-alt p-4">
                    <div className="mb-3 flex items-center justify-between gap-2">
                      <legend className="text-base font-bold text-ink">{t('editor.question', { n: i + 1 })}</legend>
                      <div className="flex items-center gap-0.5">
                        <IconButton size="sm" label={t('common.moveUp')} disabled={i === 0} onClick={() => moveQ(i, -1)}><ArrowUp size={16} /></IconButton>
                        <IconButton size="sm" label={t('common.moveDown')} disabled={i === draft.questions.length - 1} onClick={() => moveQ(i, 1)}><ArrowDown size={16} /></IconButton>
                        <IconButton size="sm" tone="danger" label={t('editor.removeQuestion')} disabled={draft.questions.length === 1}
                          onClick={() => setDraft((d) => ({ ...d, questions: d.questions.filter((_, k) => k !== i) }))}><Trash2 size={16} /></IconButton>
                      </div>
                    </div>
                    <Field label={<span className="sr-only">{t('editor.questionPlaceholder')}</span>} error={errors[`${base}.prompt`]}>
                      {(p) => <Textarea {...p} rows={2} value={q.prompt} placeholder={t('editor.questionPlaceholder')} maxLength={2000} onChange={(e) => setQ(i, (x) => ({ ...x, prompt: e.target.value }))} />}
                    </Field>

                    <div className="mt-4 flex flex-wrap items-center justify-between gap-2">
                      <p className="text-sm font-medium text-ink-body">{t('editor.answers')}</p>
                      <label className="inline-flex cursor-pointer items-center gap-2 text-sm text-ink-body">
                        <input type="checkbox" className="sr-only" checked={q.multiple}
                          onChange={(e) => setQ(i, (x) => {
                            const multiple = e.target.checked;
                            if (multiple) return { ...x, multiple };
                            let seen = false; // keep only the first correct option for single choice
                            return { ...x, multiple, options: x.options.map((o) => { const c = o.correct && !seen; if (o.correct) seen = true; return { ...o, correct: c }; }) };
                          })} />
                        <ChoiceMark type="checkbox" checked={q.multiple} />
                        {t('editor.multiple')}
                      </label>
                    </div>

                    <ul className="mt-2 flex flex-col gap-2">
                      {q.options.map((o, j) => (
                        <li key={o.key} className="flex items-start gap-2">
                          <button type="button" role={type} aria-checked={o.correct} aria-label={t('editor.markCorrect', { n: j + 1 })} title={t('editor.markCorrect', { n: j + 1 })}
                            onClick={() => setQ(i, (x) => ({ ...x, options: x.options.map((oo, k) => (k === j ? { ...oo, correct: x.multiple ? !oo.correct : true } : x.multiple ? oo : { ...oo, correct: false })) }))}
                            className="mt-2.5 rounded-full">
                            <ChoiceMark type={type} checked={o.correct} tone="success" />
                          </button>
                          <div className="flex-1">
                            <Input value={o.label} maxLength={500} aria-label={t('editor.answerPlaceholder', { n: j + 1 })} placeholder={t('editor.answerPlaceholder', { n: j + 1 })}
                              invalid={!!errors[`${base}.options.${j}.label`]}
                              className={cn(o.correct && 'border-success bg-success-soft/40')}
                              onChange={(e) => setQ(i, (x) => ({ ...x, options: x.options.map((oo, k) => (k === j ? { ...oo, label: e.target.value } : oo)) }))} />
                          </div>
                          {o.correct && <span className="mt-2.5 hidden text-xs font-medium text-success-text sm:inline">✓ {t('test.correct')}</span>}
                          <IconButton className="mt-0.5" tone="danger" label={t('editor.removeAnswer')} disabled={q.options.length <= 2}
                            onClick={() => setQ(i, (x) => ({ ...x, options: x.options.filter((_, k) => k !== j) }))}><X size={16} /></IconButton>
                        </li>
                      ))}
                    </ul>
                    {(errors[`${base}.correct`] || errors[`${base}.options`]) && (
                      <p role="alert" className="mt-2 text-xs text-danger-text">{errors[`${base}.correct`] || errors[`${base}.options`]}</p>
                    )}
                    <Button variant="ghost" size="sm" className="mt-2 text-primary-text" icon={<Plus size={16} />} disabled={q.options.length >= 20}
                      onClick={() => setQ(i, (x) => ({ ...x, options: [...x.options, emptyOption()] }))}>{t('editor.addAnswer')}</Button>
                  </fieldset>
                );
              })}
              <Button variant="soft" icon={<Plus size={18} />} onClick={() => setDraft((d) => ({ ...d, questions: [...d.questions, emptyQuestion()] }))} disabled={draft.questions.length >= 100}>
                {t('editor.addQuestion')}
              </Button>
            </div>
          )}

          {tab === 'settings' && (
            <div className="flex flex-col gap-6">
              <Switch checked={draft.hidden} onChange={(v) => setDraft((d) => ({ ...d, hidden: v }))} label={t('editor.hidden')} hint={t('editor.hiddenHint')} />

              <fieldset>
                <legend className="mb-2 text-base font-medium text-ink">{t('editor.checkMode')}</legend>
                <div className="grid gap-2 sm:grid-cols-2">
                  <RadioCard name="check" value="instant" checked={s.check_mode === 'instant'} onChange={() => setS({ check_mode: 'instant' })} title={t('editor.checkInstant')} hint={t('editor.checkInstantHint')} />
                  <RadioCard name="check" value="after" checked={s.check_mode === 'after'} onChange={() => setS({ check_mode: 'after' })} title={t('editor.checkAfter')} hint={t('editor.checkAfterHint')} />
                </div>
              </fieldset>

              <div className="flex flex-col gap-3">
                <Switch checked={s.timer_enabled} onChange={(v) => setS({ timer_enabled: v, timer_seconds: v ? s.timer_seconds ?? 300 : s.timer_seconds })} label={t('editor.timer')} hint={t('editor.timerHint')} />
                {s.timer_enabled && (
                  <Field label={t('editor.timerDuration')} error={errors['settings.timer_seconds']} className="max-w-[200px]">
                    {(p) => <Input {...p} type="number" min={0.5} max={600} step={0.5} value={s.timer_seconds ? s.timer_seconds / 60 : ''}
                      onChange={(e) => setS({ timer_seconds: e.target.value ? Math.round(Number(e.target.value) * 60) : null })} />}
                  </Field>
                )}
              </div>

              <div className="flex flex-col gap-3 rounded-xl border border-line p-4">
                <Switch checked={s.scoring_enabled} onChange={(v) => setS({ scoring_enabled: v })} label={t('editor.scoring')} />
                {s.scoring_enabled && (
                  <>
                    <fieldset>
                      <legend className="mb-2 text-sm font-medium text-ink-body">{t('editor.scoringMode')}</legend>
                      <div className="flex flex-col gap-2">
                        <RadioCard name="scoring" value="correct" checked onChange={() => {}} title={t('editor.scoringByCorrect')} />
                        {(['scoringScale', 'scoringManual'] as const).map((k) => (
                          <div key={k} aria-disabled className="flex items-center justify-between gap-3 rounded-lg border border-dashed border-line bg-surface-alt p-3 text-ink-muted">
                            <span className="flex items-center gap-3"><ChoiceMark type="radio" checked={false} />{t(`editor.${k}`)}</span>
                            <span className="text-xs">{t('common.unavailable')}</span>
                          </div>
                        ))}
                      </div>
                    </fieldset>
                    <div className="grid gap-4 sm:grid-cols-2">
                      <Field label={t('editor.points')} error={errors['settings.points_per_correct']} hint={t('editor.maxPoints', { n: s.points_per_correct * draft.questions.length })}>
                        {(p) => <Input {...p} type="number" min={1} max={100} value={s.points_per_correct} onChange={(e) => setS({ points_per_correct: Number(e.target.value) || 0 })} />}
                      </Field>
                      <CategoryField value={s.category_id} onChange={(id) => setS({ category_id: id })} categories={categories.data?.items ?? []} error={errors['settings.category_id']} />
                    </div>
                  </>
                )}
              </div>
            </div>
          )}

          {tab === 'stats' && target?.exercise && <StatsPanel exerciseId={target.exercise.id} />}
        </div>
      </Modal>
      <ConfirmDialog open={confirmClose} onClose={() => setConfirmClose(false)} title={t('editor.unsavedConfirm')} confirmLabel={t('common.close')} tone="danger"
        onConfirm={() => { setConfirmClose(false); onClose(); }} />
    </>
  );
}

function CategoryField({ value, onChange, categories, error }: { value: string | null; onChange: (id: string | null) => void; categories: { id: string; name: string }[]; error?: string }) {
  const t = useT();
  const qc = useQueryClient();
  const [adding, setAdding] = useState(false);
  const [name, setName] = useState('');
  const create = useMutation({
    mutationFn: () => exercisesApi.createCategory(name),
    onSuccess: (c) => {
      qc.invalidateQueries({ queryKey: qk.categories });
      onChange(c.id);
      setAdding(false);
      setName('');
    },
  });
  const current = value ?? categories[0]?.id ?? '';
  return (
    <Field label={t('editor.category')} error={error || fieldErrors(create.error).name}>
      {(p) =>
        adding ? (
          <div className="flex gap-2">
            <Input {...p} autoFocus value={name} maxLength={60} placeholder={t('editor.newCategory')} onChange={(e) => setName(e.target.value)} />
            <Button size="md" onClick={() => create.mutate()} loading={create.isPending} disabled={!name.trim()}>{t('common.add')}</Button>
            <IconButton label={t('common.cancel')} onClick={() => setAdding(false)}><X size={16} /></IconButton>
          </div>
        ) : (
          <Select {...p} value={current} onChange={(e) => (e.target.value === '__new' ? setAdding(true) : onChange(e.target.value))}>
            {categories.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
            <option value="__new">+ {t('editor.newCategory')}</option>
          </Select>
        )
      }
    </Field>
  );
}

function StatsPanel({ exerciseId }: { exerciseId: string }) {
  const t = useT();
  const q = useQuery({ queryKey: qk.exerciseStats(exerciseId), queryFn: () => exercisesApi.stats(exerciseId) });
  return (
    <QueryState query={q}>
      {(st) =>
        st.attempts === 0 ? (
          <EmptyState title={t('editor.statsEmpty')} />
        ) : (
          <div className="flex flex-col gap-4">
            <p className="text-sm text-ink-muted">{t('editor.statsHint', { n: st.version_number })}</p>
            <div className="grid grid-cols-2 gap-3">
              <Stat label={t('editor.attempts')} value={st.attempts} />
              <Stat label={t('editor.finished')} value={st.finished} />
            </div>
            <ul className="flex flex-col gap-3">
              {st.questions.map((qq, i) => (
                <li key={qq.question_id} className="rounded-xl border border-line p-4">
                  <p className="mb-2 text-base font-medium text-ink">{i + 1}. {qq.prompt}</p>
                  <ProgressBar value={qq.correct} max={st.attempts} tone="success" label={t('editor.correct')} />
                  <p className="mt-2 flex flex-wrap gap-4 text-sm">
                    <span className="text-success-text">✓ {t('editor.correct')}: {qq.correct}</span>
                    <span className="text-danger-text">✗ {t('editor.wrong')}: {qq.wrong}</span>
                    <span className="text-ink-muted">— {t('editor.unanswered')}: {qq.unanswered}</span>
                  </p>
                </li>
              ))}
            </ul>
          </div>
        )
      }
    </QueryState>
  );
}

function Stat({ label, value }: { label: string; value: number }) {
  return (
    <div className="rounded-xl bg-surface-alt p-4">
      <p className="text-sm text-ink-muted">{label}</p>
      <p className="text-xl font-bold text-ink">{value}</p>
    </div>
  );
}

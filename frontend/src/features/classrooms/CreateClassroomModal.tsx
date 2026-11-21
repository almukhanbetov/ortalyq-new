'use client';

import { useEffect, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useRouter } from 'next/navigation';
import { ArrowLeft, ChevronRight } from 'lucide-react';
import { Button, Field, IconButton, Input, Modal, Select, cn, useToast } from '@/components/ui';
import { classroomsApi, marathonsApi } from '@/lib/api/endpoints';
import { qk } from '@/lib/queryKeys';
import { useT } from '@/lib/i18n';
import { errorMessage, fieldErrors } from '@/lib/errors';
import { marathonSetupUrl } from '@/lib/routes';
import { MarathonForm, emptyMarathon, marathonReady } from '@/features/marathons/MarathonForm';
import { COVER_COLORS as COLORS } from './Cover';

type Step = 'type' | 'individual' | 'marathon';

/** "Create class": choose a type first (individual / group — unavailable / marathon), then fill its form. */
export function CreateClassroomModal({ open, onClose }: { open: boolean; onClose: () => void }) {
  const t = useT();
  const [step, setStep] = useState<Step>('type');
  useEffect(() => {
    if (open) setStep('type');
  }, [open]);
  const back = (
    <IconButton label={t('common.back')} onClick={() => setStep('type')}><ArrowLeft size={20} /></IconButton>
  );

  if (step === 'individual') return <IndividualForm open={open} onClose={onClose} back={back} />;
  if (step === 'marathon') return <MarathonCreate open={open} onClose={onClose} back={back} />;

  const types: { key: Step | 'group'; emoji: string; title: string; hint: string; disabled?: boolean }[] = [
    { key: 'individual', emoji: '🙋‍♂️', title: t('marathons.typeIndividual'), hint: t('marathons.typeIndividualHint') },
    { key: 'group', emoji: '👨‍👩‍👧', title: t('marathons.typeGroup'), hint: t('marathons.typeGroupHint'), disabled: true },
    { key: 'marathon', emoji: '🏃', title: t('marathons.typeMarathon'), hint: t('marathons.typeMarathonHint') },
  ];
  return (
    <Modal open={open} onClose={onClose} title={t('marathons.typeChooserTitle')}>
      <ul className="flex flex-col gap-2 pb-2">
        {types.map((it) => (
          <li key={it.key}>
            <button type="button" disabled={it.disabled} onClick={() => setStep(it.key as Step)}
              className={cn('flex w-full items-center gap-4 rounded-lg border border-line-soft px-4 py-3 text-left transition-colors',
                it.disabled ? 'cursor-not-allowed opacity-60' : 'hover:border-primary hover:bg-primary-soft/40 focus-visible:shadow-focus focus-visible:outline-none')}>
              <span className="text-[32px] leading-none" aria-hidden>{it.emoji}</span>
              <span className="min-w-0 flex-1">
                <span className="block text-md font-medium text-ink">{it.title}</span>
                <span className="block text-sm text-ink-muted">{it.disabled ? `${it.hint} — ${t('common.unavailable')}` : it.hint}</span>
              </span>
              {!it.disabled && <ChevronRight size={18} className="text-ink-faint" aria-hidden />}
            </button>
          </li>
        ))}
      </ul>
    </Modal>
  );
}

function StepTitle({ back, title }: { back: React.ReactNode; title: string }) {
  return <span className="flex items-center gap-2">{back}{title}</span>;
}

function MarathonCreate({ open, onClose, back }: { open: boolean; onClose: () => void; back: React.ReactNode }) {
  const t = useT();
  const qc = useQueryClient();
  const router = useRouter();
  const toast = useToast();
  const [value, setValue] = useState(emptyMarathon);
  const create = useMutation({
    mutationFn: () => marathonsApi.create(value),
    onSuccess: (m) => {
      qc.invalidateQueries({ queryKey: ['classrooms'] });
      toast(t('marathons.created'), 'success');
      onClose();
      router.push(marathonSetupUrl(m.id)); // opens «Настройки марафона» with «Настроить позже»
    },
  });
  const errs = fieldErrors(create.error);
  const submit = () => marathonReady(value) && !create.isPending && create.mutate();
  return (
    <Modal open={open} onClose={onClose} title={<StepTitle back={back} title={t('marathons.title')} />}
      footer={<>
        <Button variant="secondary" onClick={onClose}>{t('common.cancel')}</Button>
        <Button onClick={submit} loading={create.isPending} disabled={!marathonReady(value)}>{t('common.create')}</Button>
      </>}>
      <MarathonForm value={value} onChange={setValue} errors={errs} onSubmit={submit} />
      {create.error && !Object.keys(errs).length && <p role="alert" className="text-sm text-danger-text">{errorMessage(create.error, t)}</p>}
    </Modal>
  );
}

function IndividualForm({ open, onClose, back }: { open: boolean; onClose: () => void; back: React.ReactNode }) {
  const t = useT();
  const qc = useQueryClient();
  const router = useRouter();
  const [name, setName] = useState('');
  const [studentId, setStudentId] = useState('');
  const [color, setColor] = useState(COLORS[0]);
  const students = useQuery({ queryKey: qk.students, queryFn: classroomsApi.myStudents, enabled: open });
  const create = useMutation({
    mutationFn: () => classroomsApi.create({ name, student_id: studentId, lesson_type: 'individual', cover_color: color }),
    onSuccess: (c) => {
      qc.invalidateQueries({ queryKey: ['classrooms'] });
      onClose();
      router.push(`/classes/${c.id}`);
    },
  });
  const errs = fieldErrors(create.error);
  const list = students.data?.items ?? [];

  return (
    <Modal open={open} onClose={onClose} title={<StepTitle back={back} title={t('classes.createTitle')} />}
      footer={<>
        <Button variant="secondary" onClick={onClose}>{t('common.cancel')}</Button>
        <Button onClick={() => create.mutate()} loading={create.isPending} disabled={!name.trim() || !studentId}>{t('common.create')}</Button>
      </>}>
      <form className="flex flex-col gap-4 pb-2" onSubmit={(e) => { e.preventDefault(); create.mutate(); }}>
        <Field label={t('classes.name')} error={errs.name} required>
          {(p) => <Input {...p} data-autofocus value={name} onChange={(e) => setName(e.target.value)} placeholder={t('classes.namePlaceholder')} maxLength={120} />}
        </Field>
        <Field label={t('classes.student')} error={errs.student_id} hint={!students.isLoading && !list.length ? t('classes.noStudents') : undefined} required>
          {(p) => (
            <Select {...p} value={studentId} onChange={(e) => setStudentId(e.target.value)} disabled={!list.length}>
              <option value="">{t('classes.studentPlaceholder')}</option>
              {list.map((s) => <option key={s.id} value={s.id}>{s.full_name} · {s.email}</option>)}
            </Select>
          )}
        </Field>
        <fieldset>
          <legend className="mb-1.5 text-sm font-medium text-ink-body">{t('classes.cover')}</legend>
          <div className="flex gap-2">
            {COLORS.map((c) => (
              <button key={c} type="button" onClick={() => setColor(c)} aria-label={c} aria-pressed={c === color}
                className={cn('h-8 w-8 rounded-full ring-offset-2 transition', c === color && 'ring-2 ring-ink')} style={{ background: c }} />
            ))}
          </div>
        </fieldset>
        {create.error && !Object.keys(errs).length && <p role="alert" className="text-sm text-danger-text">{errorMessage(create.error, t)}</p>}
      </form>
    </Modal>
  );
}

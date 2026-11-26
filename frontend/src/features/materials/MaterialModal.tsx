'use client';

import { useEffect, useState } from 'react';
import { Button, Field, Input, Modal, Textarea, cn } from '@/components/ui';
import { useT } from '@/lib/i18n';
import { errorMessage, fieldErrors } from '@/lib/errors';
import { COVER_COLORS, Cover } from '@/features/classrooms/Cover';

export interface MaterialFields { title: string; description: string; cover_color: string }

/** Create / edit a personal material (a course): title, description and cover colour. */
export function MaterialModal({ open, onClose, heading, initial, onSubmit, loading, error }: {
  open: boolean;
  onClose: () => void;
  heading: string;
  initial?: Partial<MaterialFields>;
  onSubmit: (v: MaterialFields) => void;
  loading?: boolean;
  error?: unknown;
}) {
  const t = useT();
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [color, setColor] = useState(COVER_COLORS[0]);
  useEffect(() => {
    if (!open) return;
    setTitle(initial?.title ?? '');
    setDescription(initial?.description ?? '');
    setColor(initial?.cover_color ?? COVER_COLORS[0]);
  }, [open, initial?.title, initial?.description, initial?.cover_color]);

  const errs = fieldErrors(error);
  const colors = COVER_COLORS.includes(color) ? COVER_COLORS : [color, ...COVER_COLORS];
  const submit = () => { if (title.trim()) onSubmit({ title: title.trim(), description: description.trim(), cover_color: color }); };

  return (
    <Modal open={open} onClose={onClose} title={heading} size="sm"
      footer={<>
        <Button variant="secondary" onClick={onClose}>{t('common.cancel')}</Button>
        <Button onClick={submit} loading={loading} disabled={!title.trim()}>{t('common.save')}</Button>
      </>}>
      <form className="flex flex-col gap-4 pb-2" onSubmit={(e) => { e.preventDefault(); submit(); }}>
        <Cover color={color} title={title || t('materials.materialTitle')} className="h-24 w-full rounded-lg" />
        <Field label={t('materials.materialTitle')} error={errs.title} required>
          {(p) => <Input {...p} data-autofocus value={title} maxLength={200} onChange={(e) => setTitle(e.target.value)} />}
        </Field>
        <Field label={t('materials.materialDescription')} error={errs.description}>
          {(p) => <Textarea {...p} rows={4} value={description} maxLength={5000} onChange={(e) => setDescription(e.target.value)} />}
        </Field>
        <fieldset>
          <legend className="mb-1.5 text-sm font-medium text-ink-body">{t('materials.cover')}</legend>
          <div className="flex flex-wrap gap-2">
            {colors.map((c) => (
              <button key={c} type="button" onClick={() => setColor(c)} aria-label={c} aria-pressed={c === color}
                className={cn('h-8 w-8 rounded-full ring-offset-2 transition', c === color && 'ring-2 ring-ink')} style={{ background: c }} />
            ))}
          </div>
        </fieldset>
        {error != null && !Object.keys(errs).length && <p role="alert" className="text-sm text-danger-text">{errorMessage(error, t)}</p>}
      </form>
    </Modal>
  );
}

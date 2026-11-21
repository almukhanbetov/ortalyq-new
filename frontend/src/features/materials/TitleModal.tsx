'use client';

import { useEffect, useState } from 'react';
import { Button, Field, Input, Modal, Textarea } from '@/components/ui';
import { useT } from '@/lib/i18n';

/** Small create/rename dialog used for courses, lessons and sections. */
export function TitleModal({ open, onClose, title, label, initial = '', withDescription, initialDescription = '', onSubmit, loading, error }: {
  open: boolean;
  onClose: () => void;
  title: string;
  label: string;
  initial?: string;
  withDescription?: string;
  initialDescription?: string;
  onSubmit: (value: string, description: string) => void;
  loading?: boolean;
  error?: string;
}) {
  const t = useT();
  const [value, setValue] = useState(initial);
  const [desc, setDesc] = useState(initialDescription);
  useEffect(() => {
    if (open) {
      setValue(initial);
      setDesc(initialDescription);
    }
  }, [open, initial, initialDescription]);
  return (
    <Modal open={open} onClose={onClose} title={title} size="sm"
      footer={<>
        <Button variant="secondary" onClick={onClose}>{t('common.cancel')}</Button>
        <Button onClick={() => onSubmit(value.trim(), desc)} loading={loading} disabled={!value.trim()}>{t('common.save')}</Button>
      </>}>
      <form className="flex flex-col gap-4 pb-2" onSubmit={(e) => { e.preventDefault(); if (value.trim()) onSubmit(value.trim(), desc); }}>
        <Field label={label} error={error} required>
          {(p) => <Input {...p} data-autofocus value={value} maxLength={200} onChange={(e) => setValue(e.target.value)} />}
        </Field>
        {withDescription && (
          <Field label={withDescription}>
            {(p) => <Textarea {...p} rows={4} value={desc} maxLength={5000} onChange={(e) => setDesc(e.target.value)} />}
          </Field>
        )}
      </form>
    </Modal>
  );
}

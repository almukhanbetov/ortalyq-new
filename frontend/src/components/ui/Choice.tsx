'use client';

import { useId, type ReactNode } from 'react';
import { Check } from 'lucide-react';
import { cn } from './cn';

export function Switch({ checked, onChange, label, hint, disabled }: { checked: boolean; onChange: (v: boolean) => void; label: ReactNode; hint?: ReactNode; disabled?: boolean }) {
  const id = useId();
  return (
    <div className="flex items-start justify-between gap-4">
      <div>
        <label htmlFor={id} className={cn('text-base font-medium', disabled ? 'text-ink-muted' : 'text-ink')}>{label}</label>
        {hint && <p className="mt-0.5 text-sm text-ink-muted">{hint}</p>}
      </div>
      <button
        id={id}
        type="button"
        role="switch"
        aria-checked={checked}
        disabled={disabled}
        onClick={() => onChange(!checked)}
        className={cn('relative mt-0.5 h-6 w-10 shrink-0 rounded-full transition-colors disabled:cursor-not-allowed disabled:opacity-50', checked ? 'bg-primary' : 'bg-line-strong')}
      >
        <span className={cn('absolute top-0.5 h-5 w-5 rounded-full bg-white shadow transition-all', checked ? 'left-[18px]' : 'left-0.5')} />
      </button>
    </div>
  );
}

/** Visual radio/checkbox mark used both in the editor and in the test player. */
export function ChoiceMark({ type, checked, tone = 'primary' }: { type: 'radio' | 'checkbox'; checked: boolean; tone?: 'primary' | 'success' | 'danger' | 'neutral' }) {
  const color = { primary: 'border-primary bg-primary', success: 'border-success bg-success', danger: 'border-danger bg-danger', neutral: 'border-ink-muted bg-ink-muted' }[tone];
  return (
    <span aria-hidden className={cn('flex h-5 w-5 shrink-0 items-center justify-center border-2 transition-colors', type === 'radio' ? 'rounded-full' : 'rounded-[5px]', checked ? color : 'border-line-strong bg-surface')}>
      {checked && (type === 'radio' ? <span className="h-2 w-2 rounded-full bg-white" /> : <Check size={13} strokeWidth={3} className="text-white" />)}
    </span>
  );
}

export function RadioCard({ name, value, checked, onChange, title, hint }: { name: string; value: string; checked: boolean; onChange: (v: string) => void; title: ReactNode; hint?: ReactNode }) {
  return (
    <label className={cn('flex cursor-pointer items-start gap-3 rounded-lg border p-3 transition-colors', checked ? 'border-primary bg-primary-soft' : 'border-line hover:border-line-strong hover:bg-surface-alt')}>
      <input type="radio" className="sr-only" name={name} value={value} checked={checked} onChange={() => onChange(value)} />
      <ChoiceMark type="radio" checked={checked} />
      <span>
        <span className="block text-base font-medium text-ink">{title}</span>
        {hint && <span className="mt-0.5 block text-sm text-ink-muted">{hint}</span>}
      </span>
    </label>
  );
}

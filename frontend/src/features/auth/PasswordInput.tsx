'use client';

import { useState, type InputHTMLAttributes } from 'react';
import { Eye, EyeOff } from 'lucide-react';
import { Input } from '@/components/ui';
import { useT } from '@/lib/i18n';

/** Password input with a show/hide button inside the field. */
export function PasswordInput(props: Omit<InputHTMLAttributes<HTMLInputElement>, 'type'> & { invalid?: boolean }) {
  const t = useT();
  const [shown, setShown] = useState(false);
  return (
    <div className="relative">
      <Input {...props} type={shown ? 'text' : 'password'} className="pr-11" />
      <button type="button" onClick={() => setShown((s) => !s)} aria-pressed={shown}
        aria-label={shown ? t('authLayout.hidePassword') : t('authLayout.showPassword')}
        title={shown ? t('authLayout.hidePassword') : t('authLayout.showPassword')}
        className="absolute inset-y-0 right-0 flex w-10 items-center justify-center rounded-r-md text-ink-muted hover:text-ink focus-visible:shadow-focus focus-visible:outline-none">
        {shown ? <EyeOff size={18} aria-hidden /> : <Eye size={18} aria-hidden />}
      </button>
    </div>
  );
}

'use client';

import { useState, type FormEvent } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { useQueryClient } from '@tanstack/react-query';
import { Button, Field, Input } from '@/components/ui';
import { authApi } from '@/lib/api/endpoints';
import { useI18n } from '@/lib/i18n';
import { errorMessage, fieldErrors } from '@/lib/errors';
import { qk } from '@/lib/queryKeys';
import { CLASSES_ROUTE } from '@/lib/routes';
import { AuthCard, FormAlert } from './AuthCard';
import { PasswordInput } from './PasswordInput';

/** Demo accounts exist only in a local stand: `next dev`, or an image built with NEXT_PUBLIC_APP_ENV=local. */
const IS_LOCAL = process.env.NODE_ENV === 'development' || process.env.NEXT_PUBLIC_APP_ENV === 'local';

export function LoginForm() {
  const { t, locale } = useI18n();
  const router = useRouter();
  const params = useSearchParams();
  const qc = useQueryClient();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [fields, setFields] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    setFields({});
    try {
      let me = await authApi.login(email, password);
      // A language picked on the sign-in screen becomes the saved preference.
      if (me.locale !== locale) me = await authApi.updateMe({ locale }).catch(() => me);
      qc.setQueryData(qk.me, me);
      const next = params.get('next');
      router.replace(next && next.startsWith('/') && !next.startsWith('//') ? next : me.role === 'admin' ? '/admin' : CLASSES_ROUTE);
    } catch (err) {
      setFields(fieldErrors(err));
      setError(errorMessage(err, t));
    } finally {
      setBusy(false);
    }
  };

  return (
    <AuthCard tab="login" title={t('auth.title')} subtitle={t('auth.subtitle')}>
      <form onSubmit={submit} noValidate className="flex flex-col gap-4">
        <Field label={t('auth.email')} error={fields.email}>
          {(p) => <Input {...p} type="email" autoComplete="username" inputMode="email" value={email} onChange={(e) => setEmail(e.target.value)} required />}
        </Field>
        <Field label={t('auth.password')} error={fields.password}>
          {(p) => <PasswordInput {...p} autoComplete="current-password" value={password} onChange={(e) => setPassword(e.target.value)} required />}
        </Field>
        {error && !Object.keys(fields).length && <FormAlert>{error}</FormAlert>}
        <Button type="submit" size="lg" block loading={busy}>{busy ? t('auth.submitting') : t('auth.submit')}</Button>
        {IS_LOCAL && <p className="text-xs text-ink-muted">{t('auth.demoHint')}</p>}
      </form>
    </AuthCard>
  );
}

'use client';

import { useState, type FormEvent } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { useQueryClient } from '@tanstack/react-query';
import { Button, Field, Input, Logo } from '@/components/ui';
import { authApi } from '@/lib/api/endpoints';
import { useI18n } from '@/lib/i18n';
import { errorMessage, fieldErrors } from '@/lib/errors';
import { qk } from '@/lib/queryKeys';
import type { Locale } from '@/types/api';
import { CLASSES_ROUTE } from '@/lib/routes';

export function LoginForm() {
  const { t, locale, setLocale } = useI18n();
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
    <div className="flex min-h-screen flex-col items-center justify-center bg-page px-4 py-10">
      <form onSubmit={submit} noValidate className="w-full max-w-[400px] rounded-2xl bg-surface p-8 shadow-card">
        <Logo className="mb-6" />
        <h1 className="text-xl font-bold text-ink">{t('auth.title')}</h1>
        <p className="mb-6 mt-1 text-sm text-ink-muted">{t('auth.subtitle')}</p>
        <div className="flex flex-col gap-4">
          <Field label={t('auth.email')} error={fields.email}>
            {(p) => <Input {...p} type="email" autoComplete="username" value={email} onChange={(e) => setEmail(e.target.value)} required />}
          </Field>
          <Field label={t('auth.password')} error={fields.password}>
            {(p) => <Input {...p} type="password" autoComplete="current-password" value={password} onChange={(e) => setPassword(e.target.value)} required />}
          </Field>
          {error && !Object.keys(fields).length && <p role="alert" className="rounded-md bg-danger-soft px-3 py-2 text-sm text-danger-text">{error}</p>}
          <Button type="submit" size="lg" block loading={busy}>{busy ? t('auth.submitting') : t('auth.submit')}</Button>
        </div>
        <p className="mt-5 text-xs text-ink-muted">{t('auth.demoHint')}</p>
      </form>
      <div className="mt-6 flex gap-1" role="group" aria-label={t('common.language')}>
        {(['ru', 'kk', 'en'] as Locale[]).map((l) => (
          <button key={l} type="button" onClick={() => setLocale(l)} aria-pressed={l === locale}
            className={`rounded-md px-3 py-1.5 text-sm ${l === locale ? 'bg-surface font-medium text-ink shadow-card' : 'text-ink-muted hover:text-ink'}`}>
            {t(`locales.${l}`)}
          </button>
        ))}
      </div>
    </div>
  );
}

'use client';

import { useEffect, useState, type FormEvent } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Button, buttonClass, Field, Loading } from '@/components/ui';
import { ApiError } from '@/lib/api/client';
import { resetApi } from '@/lib/api/endpoints';
import { errorMessage } from '@/lib/errors';
import { useT } from '@/lib/i18n';
import { qk } from '@/lib/queryKeys';
import { AuthCard, FormAlert } from './AuthCard';
import { useLinkToken } from './emailLink';
import { PasswordInput } from './PasswordInput';

const STORE = 'reset-token';
type LinkProblem = 'invalid' | 'expired' | 'used';

function problemOf(err: unknown): LinkProblem | null {
  if (!(err instanceof ApiError)) return null;
  if (err.code === 'reset_link_expired') return 'expired';
  if (err.code === 'reset_link_used') return 'used';
  if (err.code === 'reset_link_invalid') return 'invalid';
  return null;
}

function Problem({ kind }: { kind: LinkProblem }) {
  const t = useT();
  const title = { invalid: t('reset.linkInvalidTitle'), expired: t('reset.linkExpiredTitle'), used: t('reset.linkUsedTitle') }[kind];
  const text = { invalid: t('reset.linkInvalidText'), expired: t('reset.linkExpiredText'), used: t('reset.linkUsedText') }[kind];
  return (
    <AuthCard title={title} subtitle={text}>
      {kind === 'used' ? (
        <Link href="/login" className={buttonClass({ size: 'lg', block: true })}>{t('signup.login')}</Link>
      ) : (
        <Link href="/forgot-password" className={buttonClass({ size: 'lg', block: true })}>{t('reset.requestNew')}</Link>
      )}
    </AuthCard>
  );
}

/** /reset-password#<token>: the link from the letter, then the new password; afterwards the sign-in screen. */
export function ResetPassword() {
  const t = useT();
  const router = useRouter();
  const qc = useQueryClient();
  const [token, forget] = useLinkToken(STORE);
  const [password, setPassword] = useState('');
  const [repeat, setRepeat] = useState('');
  const [mismatch, setMismatch] = useState(false);

  const link = useQuery({
    queryKey: qk.resetLink(token ?? ''),
    queryFn: () => resetApi.verify(token!),
    enabled: !!token,
    retry: false,
    staleTime: Infinity,
  });
  const complete = useMutation({
    mutationFn: () => resetApi.complete(token!, password),
    onSuccess: () => {
      forget();
      // Every session of the account has ended, this tab's included.
      qc.setQueryData(qk.me, null);
      qc.removeQueries({ queryKey: ['reset-link'] });
      router.replace('/login?reset=done');
    },
  });

  // A different link in the same tab starts clean.
  const resetComplete = complete.reset;
  useEffect(() => { resetComplete(); }, [token, resetComplete]);

  if (token === undefined || link.isLoading) return <Loading label={t('reset.checking')} className="min-h-screen" />;
  if (token === null) return <Problem kind="invalid" />;
  const problem = problemOf(link.error) ?? problemOf(complete.error);
  if (problem) return <Problem kind={problem} />;
  if (link.error || !link.data) {
    return <AuthCard title={t('reset.title')}><FormAlert>{errorMessage(link.error, t)}</FormAlert></AuthCard>;
  }

  const fields = complete.error instanceof ApiError ? complete.error.fields : {};
  const submit = (e: FormEvent) => {
    e.preventDefault();
    if (password !== repeat) { setMismatch(true); return; }
    complete.mutate();
  };

  return (
    <AuthCard title={t('reset.newTitle')} subtitle={t('reset.newSubtitle', { email: link.data.email })}>
      <form onSubmit={submit} noValidate className="flex flex-col gap-4">
        {/* Lets password managers save the new password for the right account. */}
        <input type="email" autoComplete="username" value={link.data.email} readOnly hidden />
        <Field label={t('reset.password')} hint={t('signup.passwordHint')} error={fields.password} required>
          {(p) => <PasswordInput {...p} autoComplete="new-password" value={password} minLength={8} maxLength={72}
            onChange={(e) => { setPassword(e.target.value); setMismatch(false); }} required />}
        </Field>
        <Field label={t('signup.passwordRepeat')} error={mismatch ? t('signup.passwordMismatch') : undefined} required>
          {(p) => <PasswordInput {...p} autoComplete="new-password" value={repeat}
            onChange={(e) => { setRepeat(e.target.value); setMismatch(false); }} required />}
        </Field>
        {complete.error && !Object.keys(fields).length && <FormAlert>{errorMessage(complete.error, t)}</FormAlert>}
        <Button type="submit" size="lg" block loading={complete.isPending}>
          {complete.isPending ? t('reset.saving') : t('reset.save')}
        </Button>
      </form>
    </AuthCard>
  );
}

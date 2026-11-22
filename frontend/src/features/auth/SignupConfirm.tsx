'use client';

import { useEffect, useState, type FormEvent } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Button, buttonClass, Field, Input, Loading } from '@/components/ui';
import { ApiError } from '@/lib/api/client';
import { signupApi } from '@/lib/api/endpoints';
import { errorMessage } from '@/lib/errors';
import { useT } from '@/lib/i18n';
import { qk } from '@/lib/queryKeys';
import { AuthCard, FormAlert } from './AuthCard';
import { useLinkToken } from './emailLink';

const STORE = 'signup-token';
type LinkProblem = 'invalid' | 'expired' | 'used';

function problemOf(err: unknown): LinkProblem | null {
  if (!(err instanceof ApiError)) return null;
  if (err.code === 'signup_link_expired') return 'expired';
  if (err.code === 'signup_link_used') return 'used';
  if (err.code === 'signup_link_invalid') return 'invalid';
  return null;
}

function Problem({ kind }: { kind: LinkProblem }) {
  const t = useT();
  const title = { invalid: t('signup.linkInvalidTitle'), expired: t('signup.linkExpiredTitle'), used: t('signup.linkUsedTitle') }[kind];
  const text = { invalid: t('signup.linkInvalidText'), expired: t('signup.linkExpiredText'), used: t('signup.linkUsedText') }[kind];
  return (
    <AuthCard title={title} subtitle={text}>
      {kind === 'used' ? (
        <Link href="/login" className={buttonClass({ size: 'lg', block: true })}>{t('signup.login')}</Link>
      ) : (
        <Link href="/signup" className={buttonClass({ size: 'lg', block: true })}>{t('signup.requestNew')}</Link>
      )}
    </AuthCard>
  );
}

/** Steps 2–3: the confirmation link is opened, then the password is created and the account signs in. */
export function SignupConfirm() {
  const t = useT();
  const router = useRouter();
  const qc = useQueryClient();
  const [token, forget] = useLinkToken(STORE);
  const [password, setPassword] = useState('');
  const [repeat, setRepeat] = useState('');
  const [mismatch, setMismatch] = useState(false);

  const link = useQuery({
    queryKey: qk.signupLink(token ?? ''),
    queryFn: () => signupApi.verify(token!),
    enabled: !!token,
    retry: false,
    staleTime: Infinity,
  });
  const complete = useMutation({
    mutationFn: () => signupApi.complete(token!, password),
    onSuccess: (me) => {
      forget();
      qc.setQueryData(qk.me, me);
      qc.removeQueries({ queryKey: ['signup-link'] });
      router.replace('/signup/welcome');
    },
  });

  // A different link in the same tab starts clean.
  const resetComplete = complete.reset;
  useEffect(() => { resetComplete(); }, [token, resetComplete]);

  if (token === undefined || link.isLoading) return <Loading label={t('signup.checking')} className="min-h-screen" />;
  if (token === null) return <Problem kind="invalid" />;
  const problem = problemOf(link.error) ?? problemOf(complete.error);
  if (problem) return <Problem kind={problem} />;
  if (link.error || !link.data) {
    return <AuthCard title={t('signup.title')}><FormAlert>{errorMessage(link.error, t)}</FormAlert></AuthCard>;
  }

  const fields = complete.error instanceof ApiError ? complete.error.fields : {};
  const submit = (e: FormEvent) => {
    e.preventDefault();
    if (password !== repeat) { setMismatch(true); return; }
    complete.mutate();
  };

  return (
    <AuthCard title={t('signup.passwordTitle')} subtitle={t('signup.passwordSubtitle', { email: link.data.email })}>
      <form onSubmit={submit} noValidate className="flex flex-col gap-4">
        {/* Lets password managers save the new password for the right account. */}
        <input type="email" autoComplete="username" value={link.data.email} readOnly hidden />
        <Field label={t('signup.password')} hint={t('signup.passwordHint')} error={fields.password} required>
          {(p) => <Input {...p} type="password" autoComplete="new-password" value={password} minLength={8} maxLength={72}
            onChange={(e) => { setPassword(e.target.value); setMismatch(false); }} required />}
        </Field>
        <Field label={t('signup.passwordRepeat')} error={mismatch ? t('signup.passwordMismatch') : undefined} required>
          {(p) => <Input {...p} type="password" autoComplete="new-password" value={repeat}
            onChange={(e) => { setRepeat(e.target.value); setMismatch(false); }} required />}
        </Field>
        {complete.error && !Object.keys(fields).length && <FormAlert>{errorMessage(complete.error, t)}</FormAlert>}
        <Button type="submit" size="lg" block loading={complete.isPending}>
          {complete.isPending ? t('signup.creating') : t('signup.create')}
        </Button>
      </form>
    </AuthCard>
  );
}

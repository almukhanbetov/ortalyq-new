'use client';

import { useEffect, useRef, useState, type FormEvent } from 'react';
import Link from 'next/link';
import { useMutation } from '@tanstack/react-query';
import { Button, Field, Input } from '@/components/ui';
import { ApiError } from '@/lib/api/client';
import { resetApi } from '@/lib/api/endpoints';
import { errorMessage } from '@/lib/errors';
import { useI18n } from '@/lib/i18n';
import { AuthCard, FormAlert } from './AuthCard';
import { CheckMailCard, clock, retryAfter, useCountdown } from './emailLink';

const backToLogin = (label: string) => <Link href="/login" className="font-medium text-primary-text hover:underline">{label}</Link>;

/**
 * /forgot-password: e-mail → "check your mail". The server answers the same for an existing and an unknown address
 * and sends a letter only to an existing account, so the texts here never say whether the account exists.
 */
export function ForgotPasswordForm() {
  const { t, locale } = useI18n();
  const [email, setEmail] = useState('');
  const [sentTo, setSentTo] = useState<string | null>(null);
  const [resent, setResent] = useState(false);
  const [left, startCountdown] = useCountdown();
  const emailRef = useRef<HTMLInputElement>(null);
  const editing = useRef(false);

  const request = useMutation({
    mutationFn: (to: string) => resetApi.request({ email: to, locale }),
    onSuccess: (res, to) => {
      setResent(sentTo !== null);
      setSentTo(to.trim());
      startCountdown(res.resend_after);
    },
    onError: (err) => { const n = retryAfter(err); if (n) startCountdown(n); },
  });
  const err = request.error;
  const wait = retryAfter(err);
  const waiting = wait > 0 && left > 0;
  const fields = err instanceof ApiError && !wait ? err.fields : {};
  const message = waiting ? t('signup.tooMany', { time: clock(left) })
    : err && !wait && !Object.keys(fields).length ? errorMessage(err, t) : null;

  // "Change email" returns to the form with the address selected for editing.
  useEffect(() => {
    if (sentTo !== null || !editing.current) return;
    editing.current = false;
    emailRef.current?.focus();
    emailRef.current?.select();
  }, [sentTo]);

  const changeEmail = () => {
    editing.current = true;
    request.reset();
    setResent(false);
    setSentTo(null);
  };

  if (sentTo) {
    return (
      <CheckMailCard text={t(resent ? 'reset.resentText' : 'reset.sentText', { email: sentTo })} hint={t('reset.sentHint')}
        message={message} left={left} waiting={waiting} pending={request.isPending}
        onResend={() => request.mutate(sentTo)} onChangeEmail={changeEmail} footer={backToLogin(t('reset.backToLogin'))} />
    );
  }

  const submit = (e: FormEvent) => {
    e.preventDefault();
    request.mutate(email);
  };

  return (
    <AuthCard title={t('reset.title')} subtitle={t('reset.subtitle')} footer={backToLogin(t('reset.backToLogin'))}>
      <form onSubmit={submit} noValidate className="flex flex-col gap-4">
        <Field label={t('auth.email')} error={fields.email} required>
          {(p) => <Input {...p} ref={emailRef} type="email" autoComplete="username" inputMode="email" value={email} onChange={(e) => setEmail(e.target.value)} required />}
        </Field>
        {message && <FormAlert>{message}</FormAlert>}
        <Button type="submit" size="lg" block loading={request.isPending} disabled={waiting}>
          {request.isPending ? t('reset.submitting') : t('reset.submit')}
        </Button>
      </form>
    </AuthCard>
  );
}

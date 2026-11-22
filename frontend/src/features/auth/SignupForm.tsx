'use client';

import { Fragment, useCallback, useEffect, useRef, useState, type FormEvent } from 'react';
import Link from 'next/link';
import { useMutation } from '@tanstack/react-query';
import { MailCheck } from 'lucide-react';
import { Button, ChoiceMark, Field, Input } from '@/components/ui';
import { ApiError } from '@/lib/api/client';
import { signupApi } from '@/lib/api/endpoints';
import { errorMessage } from '@/lib/errors';
import { useI18n } from '@/lib/i18n';
import { AuthCard, FormAlert } from './AuthCard';

/** "I accept the {terms} and the {privacy}" with both placeholders rendered as links. */
function Consent() {
  const { t } = useI18n();
  const links: Record<string, { href: string; label: string }> = {
    terms: { href: '/terms', label: t('signup.terms') },
    privacy: { href: '/privacy', label: t('signup.privacy') },
  };
  return (
    <>
      {t('signup.consent').split(/(\{terms\}|\{privacy\})/).map((part, i) => {
        const l = links[part.slice(1, -1)];
        return l ? (
          <Link key={i} href={l.href} target="_blank" className="font-medium text-primary-text hover:underline">{l.label}</Link>
        ) : <Fragment key={i}>{part}</Fragment>;
      })}
    </>
  );
}

/** Seconds left until a deadline set from a server answer; re-renders once a second while it runs. */
function useCountdown() {
  const [until, setUntil] = useState(0);
  const [, tick] = useState(0);
  const left = Math.max(0, Math.ceil((until - Date.now()) / 1000));
  useEffect(() => {
    if (left <= 0) return;
    const id = setTimeout(() => tick((n) => n + 1), ((until - Date.now()) % 1000 || 1000) + 20);
    return () => clearTimeout(id);
  });
  const start = useCallback((sec: number) => setUntil(Date.now() + sec * 1000), []);
  return [left, start] as const;
}

/** 42 → "0:42", 300 → "5:00". */
const clock = (sec: number) => `${Math.floor(sec / 60)}:${String(sec % 60).padStart(2, '0')}`;

function retryAfter(err: unknown): number {
  return err instanceof ApiError && err.status === 429 ? Number(err.fields.retry_after) || 60 : 0;
}

/**
 * Step 1 of the teacher sign-up: e-mail and consent, then "check your mail" with resend and change of address.
 * The server answers the same for a new and an existing address, so these screens never say whether an account
 * exists. Resend limits live on the server: the timer only shows how long it asked to wait.
 */
export function SignupForm() {
  const { t, locale } = useI18n();
  const [email, setEmail] = useState('');
  const [accepted, setAccepted] = useState(false);
  const [sentTo, setSentTo] = useState<string | null>(null);
  const [resent, setResent] = useState(false);
  const [consentError, setConsentError] = useState(false);
  const [left, startCountdown] = useCountdown();
  const emailRef = useRef<HTMLInputElement>(null);
  const editing = useRef(false);

  const request = useMutation({
    mutationFn: (to: string) => signupApi.requestEmail({ email: to, locale, accept_terms: true }),
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

  const submit = (e: FormEvent) => {
    e.preventDefault();
    if (!accepted) { setConsentError(true); return; }
    request.mutate(email);
  };

  const changeEmail = () => {
    editing.current = true;
    request.reset();
    setResent(false);
    setSentTo(null);
  };

  if (sentTo) {
    return (
      <AuthCard title={t('signup.sentTitle')}
        footer={<>{t('signup.haveAccount')} <Link href="/login" className="font-medium text-primary-text hover:underline">{t('signup.login')}</Link></>}>
        <div className="flex flex-col gap-4">
          <span className="flex h-12 w-12 items-center justify-center rounded-full bg-primary-soft text-primary-text"><MailCheck size={24} aria-hidden /></span>
          <p className="text-base text-ink" aria-live="polite">
            {t(resent ? 'signup.resentText' : 'signup.sentText', { email: sentTo })}
          </p>
          <p className="text-sm text-ink-muted">{t('signup.sentHint')}</p>
          {message && <FormAlert>{message}</FormAlert>}
          <div className="flex flex-col gap-2">
            <Button block disabled={left > 0} loading={request.isPending} onClick={() => request.mutate(sentTo)}>
              {left > 0 && !waiting ? t('signup.resendIn', { time: clock(left) }) : t('signup.resend')}
            </Button>
            <Button block variant="secondary" onClick={changeEmail}>{t('signup.changeEmail')}</Button>
          </div>
        </div>
      </AuthCard>
    );
  }

  return (
    <AuthCard tab="signup" title={t('signup.title')} subtitle={t('signup.subtitle')}>
      <form onSubmit={submit} noValidate className="flex flex-col gap-4">
        <Field label={t('signup.email')} error={fields.email} required>
          {(p) => <Input {...p} ref={emailRef} type="email" autoComplete="email" inputMode="email" value={email} onChange={(e) => setEmail(e.target.value)} required />}
        </Field>
        <div>
          <label className="flex cursor-pointer items-start gap-3 text-sm text-ink">
            <input type="checkbox" className="peer sr-only" checked={accepted}
              aria-invalid={consentError || !!fields.accept_terms} aria-describedby={consentError || fields.accept_terms ? 'consent-error' : undefined}
              onChange={(e) => { setAccepted(e.target.checked); setConsentError(false); }} />
            <span className="mt-px rounded-[5px] peer-focus-visible:shadow-focus"><ChoiceMark type="checkbox" checked={accepted} /></span>
            <span><Consent /></span>
          </label>
          {(consentError || fields.accept_terms) && (
            <p id="consent-error" className="mt-1.5 text-sm text-danger-text">{consentError ? t('signup.consentRequired') : fields.accept_terms}</p>
          )}
        </div>
        {message && <FormAlert>{message}</FormAlert>}
        <Button type="submit" size="lg" block loading={request.isPending} disabled={waiting}>
          {request.isPending ? t('signup.submitting') : t('signup.submit')}
        </Button>
      </form>
    </AuthCard>
  );
}

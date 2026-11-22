'use client';

import { Fragment, useEffect, useRef, useState, type FormEvent } from 'react';
import Link from 'next/link';
import { useMutation } from '@tanstack/react-query';
import { Button, ChoiceMark, Field, Input } from '@/components/ui';
import { ApiError } from '@/lib/api/client';
import { signupApi } from '@/lib/api/endpoints';
import { errorMessage } from '@/lib/errors';
import { useI18n } from '@/lib/i18n';
import { AuthCard, FormAlert } from './AuthCard';
import { CheckMailCard, clock, retryAfter, useCountdown } from './emailLink';

/** "I accept the {terms} and the {privacy}" with both placeholders rendered as links. */
export function Consent() {
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
      <CheckMailCard text={t(resent ? 'signup.resentText' : 'signup.sentText', { email: sentTo })} hint={t('signup.sentHint')}
        message={message} left={left} waiting={waiting} pending={request.isPending}
        onResend={() => request.mutate(sentTo)} onChangeEmail={changeEmail}
        footer={<>{t('signup.haveAccount')} <Link href="/login" className="font-medium text-primary-text hover:underline">{t('signup.login')}</Link></>} />
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

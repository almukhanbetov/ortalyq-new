'use client';

import { useEffect, useState, type FormEvent, type ReactNode } from 'react';
import Link from 'next/link';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { CheckCircle2, Mail } from 'lucide-react';
import { Button, buttonClass, ChoiceMark, Field, Input, Loading } from '@/components/ui';
import { ApiError } from '@/lib/api/client';
import { authApi, studentInvitesApi } from '@/lib/api/endpoints';
import { errorMessage, fieldErrors } from '@/lib/errors';
import { useT } from '@/lib/i18n';
import { qk } from '@/lib/queryKeys';
import { CLASSES_ROUTE } from '@/lib/routes';
import { useMe } from '@/hooks/useMe';
import { AuthCard, FormAlert } from './AuthCard';
import { PasswordInput } from './PasswordInput';
import { Consent } from './SignupForm';
import { useLinkToken } from './emailLink';

const STORE = 'student-invite-token';
const SELF = '/student-invite';
type LinkProblem = 'invalid' | 'expired' | 'used';

function problemOf(err: unknown): LinkProblem | null {
  if (!(err instanceof ApiError)) return null;
  if (err.code === 'student_invite_expired') return 'expired';
  if (err.code === 'student_invite_used') return 'used';
  if (err.code === 'student_invite_invalid') return 'invalid';
  return null;
}

function Problem({ kind }: { kind: LinkProblem }) {
  const t = useT();
  return (
    <AuthCard title={t(`studentInvite.${kind}Title`)} subtitle={t(`studentInvite.${kind}Text`)}>
      <Link href="/login" className={buttonClass({ size: 'lg', block: true })}>{t('studentInvite.login')}</Link>
    </AuthCard>
  );
}

/**
 * The invitation link. A new address creates a student account here (the link proves the address); an address
 * with an account signs in first and then accepts, so an existing student keeps one account. The token stays in
 * this tab's storage across the sign-in.
 */
export function StudentInvite() {
  const t = useT();
  const qc = useQueryClient();
  const [token, forget] = useLinkToken(STORE);
  const me = useMe();
  const [accepted, setAccepted] = useState<string | null>(null);

  const link = useQuery({
    queryKey: qk.studentInviteLink(token ?? ''),
    queryFn: () => studentInvitesApi.verify(token!),
    enabled: !!token && !accepted,
    retry: false,
    staleTime: Infinity,
  });
  const done = (teacher: string) => {
    forget();
    setAccepted(teacher);
    qc.invalidateQueries({ queryKey: ['classrooms'] });
  };
  const accept = useMutation({
    mutationFn: () => studentInvitesApi.accept(token!),
    onSuccess: (res) => done(res.teacher_name),
  });
  const signOut = useMutation({
    mutationFn: () => authApi.logout().catch(() => {}),
    onSuccess: () => qc.setQueryData(qk.me, null),
  });
  // Another link opened in the same tab (only the fragment changes) starts clean.
  const resetAccept = accept.reset;
  useEffect(() => { if (token) { setAccepted(null); resetAccept(); } }, [token, resetAccept]);

  if (accepted !== null) {
    return (
      <AuthCard title={t('studentInvite.title')}>
        <div className="flex flex-col gap-4" data-testid="student-invite-accepted">
          <p role="status" className="flex items-start gap-2 rounded-md bg-success-soft px-3 py-2 text-sm text-success-text">
            <CheckCircle2 size={18} className="mt-px shrink-0" aria-hidden />{t('studentInvite.accepted', { teacher: accepted })}
          </p>
          <Link href={CLASSES_ROUTE} className={buttonClass({ size: 'lg', block: true })}>{t('studentInvite.toClasses')}</Link>
        </div>
      </AuthCard>
    );
  }
  if (token === undefined || link.isLoading || me.isLoading) return <Loading label={t('studentInvite.checking')} className="min-h-screen" />;
  if (token === null) return <Problem kind="invalid" />;
  const problem = problemOf(link.error) ?? problemOf(accept.error);
  if (problem) return <Problem kind={problem} />;
  if (link.error || !link.data) {
    return <AuthCard title={t('studentInvite.title')}><FormAlert>{errorMessage(link.error, t)}</FormAlert></AuthCard>;
  }

  const d = link.data;
  const intro = (
    <div className="flex flex-col gap-1.5">
      <p className="text-base text-ink">{t('studentInvite.text', { teacher: d.teacher_name })}</p>
      <p className="flex items-center gap-2 text-sm text-ink-muted"><Mail size={16} aria-hidden />{t('studentInvite.sentTo', { email: d.email })}</p>
    </div>
  );

  if (me.data) {
    const same = me.data.email.toLowerCase() === d.email.toLowerCase();
    const blocked = !same ? t('studentInvite.wrongAccount', { current: me.data.email, email: d.email })
      : me.data.role !== 'student' ? t('studentInvite.notStudent', { email: d.email }) : null;
    return (
      <AuthCard title={t('studentInvite.title')}>
        <div className="flex flex-col gap-4" data-testid="student-invite" data-state={blocked ? 'blocked' : 'accept'}>
          {intro}
          {blocked ? (
            <>
              <FormAlert>{blocked}</FormAlert>
              <Button size="lg" block variant="secondary" loading={signOut.isPending} onClick={() => signOut.mutate()}>{t('studentInvite.logout')}</Button>
            </>
          ) : (
            <>
              {accept.error && <FormAlert>{errorMessage(accept.error, t)}</FormAlert>}
              <Button size="lg" block loading={accept.isPending} onClick={() => accept.mutate()}>{t('studentInvite.accept')}</Button>
            </>
          )}
        </div>
      </AuthCard>
    );
  }

  if (d.has_account) {
    return (
      <AuthCard title={t('studentInvite.title')}>
        <div className="flex flex-col gap-4" data-testid="student-invite" data-state="sign-in">
          {intro}
          <p className="text-sm text-ink-body">{t('studentInvite.signInText', { email: d.email })}</p>
          <Link href={`/login?next=${encodeURIComponent(SELF)}`} className={buttonClass({ size: 'lg', block: true })}>{t('studentInvite.signIn')}</Link>
        </div>
      </AuthCard>
    );
  }
  return <Register token={token} email={d.email} teacher={d.teacher_name} intro={intro} onDone={done} />;
}

function Register({ token, email, teacher, intro, onDone }: {
  token: string; email: string; teacher: string; intro: ReactNode; onDone: (teacher: string) => void;
}) {
  const t = useT();
  const qc = useQueryClient();
  const [name, setName] = useState('');
  const [password, setPassword] = useState('');
  const [repeat, setRepeat] = useState('');
  const [mismatch, setMismatch] = useState(false);
  const [terms, setTerms] = useState(false);
  const [termsError, setTermsError] = useState(false);
  const register = useMutation({
    mutationFn: () => studentInvitesApi.register({ token, full_name: name, password, accept_terms: terms }),
    onSuccess: (me) => {
      qc.setQueryData(qk.me, me);
      onDone(teacher);
    },
  });
  const reset = register.reset;
  useEffect(() => { reset(); }, [token, reset]);
  const fields = fieldErrors(register.error);
  const submit = (e: FormEvent) => {
    e.preventDefault();
    if (password !== repeat) { setMismatch(true); return; }
    if (!terms) { setTermsError(true); return; }
    register.mutate();
  };
  return (
    <AuthCard title={t('studentInvite.title')} subtitle={t('studentInvite.registerSubtitle', { email })}>
      <form onSubmit={submit} noValidate className="flex flex-col gap-4" data-testid="student-invite" data-state="register">
        {intro}
        <input type="email" autoComplete="username" value={email} readOnly hidden />
        <Field label={t('studentInvite.fullName')} error={fields.full_name} required>
          {(p) => <Input {...p} autoComplete="name" value={name} maxLength={120} onChange={(e) => setName(e.target.value)} required />}
        </Field>
        <Field label={t('studentInvite.password')} hint={t('signup.passwordHint')} error={fields.password} required>
          {(p) => <PasswordInput {...p} autoComplete="new-password" value={password} maxLength={72}
            onChange={(e) => { setPassword(e.target.value); setMismatch(false); }} required />}
        </Field>
        <Field label={t('studentInvite.passwordRepeat')} error={mismatch ? t('signup.passwordMismatch') : undefined} required>
          {(p) => <PasswordInput {...p} autoComplete="new-password" value={repeat}
            onChange={(e) => { setRepeat(e.target.value); setMismatch(false); }} required />}
        </Field>
        <div>
          <label className="flex cursor-pointer items-start gap-3 text-sm text-ink">
            <input type="checkbox" className="peer sr-only" checked={terms}
              aria-invalid={termsError || !!fields.accept_terms} aria-describedby={termsError || fields.accept_terms ? 'invite-consent-error' : undefined}
              onChange={(e) => { setTerms(e.target.checked); setTermsError(false); }} />
            <span className="mt-px rounded-[5px] peer-focus-visible:shadow-focus"><ChoiceMark type="checkbox" checked={terms} /></span>
            <span><Consent /></span>
          </label>
          {(termsError || fields.accept_terms) && (
            <p id="invite-consent-error" className="mt-1.5 text-sm text-danger-text">{termsError ? t('signup.consentRequired') : fields.accept_terms}</p>
          )}
        </div>
        {register.error && !Object.keys(fields).length && <FormAlert>{errorMessage(register.error, t)}</FormAlert>}
        <Button type="submit" size="lg" block loading={register.isPending}>
          {register.isPending ? t('studentInvite.registering') : t('studentInvite.register')}
        </Button>
      </form>
    </AuthCard>
  );
}

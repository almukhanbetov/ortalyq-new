'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertCircle, CheckCircle2, Clock, Mail, MailCheck, MailWarning, MoreHorizontal, Plus, RotateCw, Send, X } from 'lucide-react';
import { Avatar, Button, ConfirmDialog, EmptyState, Field, IconButton, Input, Loading, Menu, Notice, Select, cn, useToast } from '@/components/ui';
import { Logo } from '@/components/ui/Logo';
import { marathonsApi } from '@/lib/api/endpoints';
import { ApiError } from '@/lib/api/client';
import { qk } from '@/lib/queryKeys';
import { useI18n } from '@/lib/i18n';
import { errorMessage } from '@/lib/errors';
import { marathonUrl } from '@/lib/routes';
import { useMe } from '@/hooks/useMe';
import type { Classroom, InviteResult, MarathonInvitation } from '@/types/api';
import { formatMarathonMoment } from './marathonTime';

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

/**
 * «Добавить ученика → Вручную»: addresses and «Язык писем»; each address gets an invitation letter with a one-time
 * link. Only students assigned to the teacher can be invited (Н6 default): the server answers per address and
 * never says whether an unknown address has an account.
 */
export function ManualInvite({ marathon, onDone }: { marathon: Classroom; onDone: () => void }) {
  const { t } = useI18n();
  const qc = useQueryClient();
  const [draft, setDraft] = useState('');
  const [emails, setEmails] = useState<string[]>([]);
  const [locale, setLocale] = useState<'ru' | 'kk' | 'en'>('ru');
  const [results, setResults] = useState<InviteResult[] | null>(null);
  const draftOk = EMAIL_RE.test(draft.trim());
  const addDraft = () => {
    const e = draft.trim().toLowerCase();
    if (!EMAIL_RE.test(e)) return;
    setEmails((l) => (l.includes(e) || l.length >= 20 ? l : [...l, e]));
    setDraft('');
  };
  const send = useMutation({
    mutationFn: () => marathonsApi.invite(marathon.id, emails, locale),
    onSuccess: (res) => {
      setResults(res.items);
      qc.invalidateQueries({ queryKey: qk.marathonInvitations(marathon.id) });
    },
  });

  if (results) {
    return (
      <div className="flex flex-col gap-3" data-testid="invite-results">
        <ul className="flex flex-col gap-1">
          {results.map((r) => {
            const good = r.result === 'invited' || r.result === 'renewed';
            return (
              <li key={r.email} data-result={r.result} className="flex flex-wrap items-center gap-2 rounded-lg border border-line-soft px-3 py-2 text-sm">
                {good ? <CheckCircle2 size={16} className="text-success" aria-hidden /> : <AlertCircle size={16} className="text-warning-text" aria-hidden />}
                <span className="min-w-0 flex-1 truncate font-medium text-ink-body">{r.email}</span>
                <span className={good ? 'text-success-text' : 'text-ink-soft'}>{t(`marathons.invite.result.${r.result}`)}</span>
              </li>
            );
          })}
        </ul>
        <div className="flex justify-end gap-2">
          <Button variant="secondary" onClick={() => { setResults(null); setEmails([]); }}>{t('marathons.invite.more')}</Button>
          <Button onClick={onDone}>{t('marathons.invite.done')}</Button>
        </div>
      </div>
    );
  }
  return (
    <div className="flex flex-col gap-4" data-testid="manual-invite">
      <form className="flex items-end gap-2" onSubmit={(e) => { e.preventDefault(); addDraft(); }}>
        <Field label={t('marathons.invite.emailLabel')} className="min-w-0 flex-1"
          error={draft.trim() && !draftOk ? t('marathons.invite.invalidEmail') : undefined}>
          {(p) => <Input {...p} type="email" inputMode="email" autoComplete="off" value={draft} onChange={(e) => setDraft(e.target.value)}
            placeholder={t('marathons.invite.emailPlaceholder')} data-autofocus />}
        </Field>
        <Button type="submit" variant="secondary" icon={<Plus size={16} />} disabled={!draftOk || emails.length >= 20}>{t('marathons.invite.addEmail')}</Button>
      </form>
      {emails.length === 0 ? (
        <div className="flex flex-col items-center rounded-lg border border-line-soft px-4 py-8 text-center">
          <span className="text-[44px] leading-none" aria-hidden>🙋</span>
          <p className="mt-2 text-sm text-ink-soft">{t('marathons.invite.emailsEmpty')}</p>
        </div>
      ) : (
        <ul className="flex flex-wrap gap-2" data-testid="invite-emails">
          {emails.map((e) => (
            <li key={e} className="inline-flex h-8 items-center gap-1 rounded-full bg-surface-muted pl-3 pr-1 text-sm text-ink-body">
              <Mail size={14} aria-hidden />{e}
              <button type="button" onClick={() => setEmails((l) => l.filter((x) => x !== e))} aria-label={t('marathons.invite.removeEmail', { email: e })}
                className="flex h-6 w-6 items-center justify-center rounded-full text-ink-soft hover:bg-surface-hover focus-visible:shadow-focus focus-visible:outline-none">
                <X size={14} aria-hidden />
              </button>
            </li>
          ))}
        </ul>
      )}
      <Field label={t('marathons.invite.localeLabel')}>
        {(p) => (
          <Select {...p} value={locale} onChange={(e) => setLocale(e.target.value as 'ru' | 'kk' | 'en')}>
            <option value="ru">Русский</option>
            <option value="kk">Қазақша</option>
            <option value="en">English</option>
          </Select>
        )}
      </Field>
      <Notice tone="info" className="text-sm">{t('marathons.invite.notice')}</Notice>
      {send.error && <p role="alert" className="text-sm text-danger-text">{errorMessage(send.error, t)}</p>}
      <div className="flex justify-end">
        <Button icon={<Send size={16} />} loading={send.isPending} disabled={!emails.length} onClick={() => send.mutate()}>
          {t('marathons.invite.send', { n: emails.length })}
        </Button>
      </div>
    </div>
  );
}

const STATUS_TONE: Record<MarathonInvitation['status'], string> = {
  pending: 'bg-primary-soft text-primary-text',
  expired: 'bg-warning-soft text-warning-text',
  accepted: 'bg-success-soft text-success-text',
  revoked: 'bg-surface-muted text-ink-soft',
  participant: 'bg-success-soft text-success-text',
};

/** «Приглашения» under the students list: status, delivery of the letter, validity; resend and revoke. */
export function InvitationsList({ marathon }: { marathon: Classroom }) {
  const { t, locale } = useI18n();
  const qc = useQueryClient();
  const toast = useToast();
  const [revoking, setRevoking] = useState<MarathonInvitation | null>(null);
  const key = qk.marathonInvitations(marathon.id);
  const list = useQuery({ queryKey: key, queryFn: () => marathonsApi.invitations(marathon.id) });
  const applied = (res: { items: MarathonInvitation[] }) => qc.setQueryData(key, res);
  const resend = useMutation({
    mutationFn: (i: MarathonInvitation) => marathonsApi.resendInvitation(marathon.id, i.id),
    onSuccess: (res) => { applied(res); toast(t('marathons.invite.resent'), 'success'); },
    onError: (e) => { qc.invalidateQueries({ queryKey: key }); toast(errorMessage(e, t), 'error'); },
  });
  const revoke = useMutation({
    mutationFn: (i: MarathonInvitation) => marathonsApi.revokeInvitation(marathon.id, i.id),
    onSuccess: (res) => { applied(res); setRevoking(null); toast(t('marathons.invite.revoked'), 'success'); },
    onError: (e) => { qc.invalidateQueries({ queryKey: key }); setRevoking(null); toast(errorMessage(e, t), 'error'); },
  });
  const tz = marathon.marathon?.timezone ?? 'Asia/Almaty';
  const items = list.data?.items ?? [];
  if (!items.length) return null;
  return (
    <section className="mt-8" aria-labelledby="invitations-title" data-testid="invitations">
      <h2 id="invitations-title" className="text-lg font-bold text-ink-body">{t('marathons.invite.listTitle')}</h2>
      <ul className="mt-2">
        {items.map((i) => {
          const open = i.status === 'pending' || i.status === 'expired';
          const DeliveryIcon = i.delivery === 'sent' ? MailCheck : i.delivery === 'failed' ? MailWarning : Clock;
          return (
            <li key={i.id} data-invitation={i.id} data-status={i.status} data-delivery={i.delivery}
              className="flex flex-wrap items-center gap-3 border-b border-line-soft py-3 first:border-t">
              <Avatar name={i.student_name} color={i.avatar_color} size={40} />
              <div className="min-w-0 flex-1">
                <p className="truncate text-base font-medium text-ink-body">{i.student_name}</p>
                <p className="truncate text-sm text-ink-soft">{i.email}</p>
                <div className="mt-1 flex flex-wrap items-center gap-1.5">
                  <span className={cn('inline-flex h-5 items-center rounded px-1.5 text-xs font-medium', STATUS_TONE[i.status])}>{t(`marathons.invite.status.${i.status}`)}</span>
                  {open && (
                    <span className={cn('inline-flex h-5 items-center gap-1 rounded px-1.5 text-xs font-medium',
                      i.delivery === 'failed' ? 'bg-danger-soft text-danger-text' : 'bg-surface-muted text-ink-soft')}>
                      <DeliveryIcon size={12} aria-hidden />{t(`marathons.invite.delivery.${i.delivery}`)}
                    </span>
                  )}
                  {i.status === 'pending' && (
                    <span className="text-xs text-ink-soft">{t('marathons.invite.validUntil', formatMarathonMoment(i.expires_at, tz, locale))}</span>
                  )}
                </div>
              </div>
              {open && (
                <Menu label={t('marathons.invite.menu', { email: i.email })} items={[
                  { key: 'resend', label: t('marathons.invite.resend'), icon: <RotateCw size={16} />, disabled: resend.isPending, onSelect: () => resend.mutate(i) },
                  { key: 'revoke', label: t('marathons.invite.revoke'), icon: <X size={16} />, tone: 'danger', onSelect: () => setRevoking(i) },
                ]} trigger={(p) => <IconButton {...p} label={t('marathons.invite.menu', { email: i.email })}><MoreHorizontal size={18} /></IconButton>} />
              )}
            </li>
          );
        })}
      </ul>
      <ConfirmDialog open={!!revoking} onClose={() => setRevoking(null)} onConfirm={() => revoking && revoke.mutate(revoking)} loading={revoke.isPending}
        tone="danger" title={t('marathons.invite.revokeTitle')} text={revoking ? t('marathons.invite.revokeText', { email: revoking.email }) : ''}
        confirmLabel={t('marathons.invite.revoke')} />
    </section>
  );
}

/**
 * The page of an invitation link (/invite/:token). Anyone with the link sees the marathon and a masked address;
 * accepting needs the invited student signed in — the server checks the account, the period and the single use.
 */
export function InvitationPage({ token }: { token: string }) {
  const { t } = useI18n();
  const router = useRouter();
  const me = useMe();
  const info = useQuery({ queryKey: qk.invitation(token), queryFn: () => marathonsApi.invitationInfo(token), retry: false });
  const accept = useMutation({
    mutationFn: () => marathonsApi.acceptInvitation(token),
    onSuccess: (res) => router.replace(marathonUrl(res.marathon_id)),
  });
  const code = accept.error instanceof ApiError ? accept.error.code : '';
  const acceptError = !accept.error ? null
    : code === 'invitation_wrong_account' ? t('marathons.invite.page.wrongAccount', { email: info.data?.email_masked ?? '' })
      : code === 'invitation_expired' ? t('marathons.invite.page.expired')
        : code === 'invitation_revoked' ? t('marathons.invite.page.revoked')
          : code === 'invitation_used' ? t('marathons.invite.page.used')
            : code === 'invitation_unavailable' ? t('marathons.invite.page.unavailable')
              : errorMessage(accept.error, t);
  const next = `/login?next=${encodeURIComponent(`/invite/${token}`)}`;

  let body;
  if (info.isLoading || me.isLoading) body = <Loading />;
  else if (!info.data) body = <EmptyState icon={<AlertCircle size={26} />} title={t('marathons.invite.page.notFound')} />;
  else {
    const d = info.data;
    body = (
      <div className="flex flex-col gap-4" data-testid="invitation-page" data-status={d.status}>
        <span className="text-[56px] leading-none" aria-hidden>🙋</span>
        <h1 className="text-[26px] font-bold leading-8 text-ink-body">{t('marathons.invite.page.title')}</h1>
        <p className="text-base text-ink-body">{t('marathons.invite.page.text', { teacher: d.teacher_name, marathon: d.marathon_name })}</p>
        <p className="flex items-center gap-2 text-sm text-ink-soft"><Mail size={16} aria-hidden />{t('marathons.invite.page.sentTo', { email: d.email_masked })}</p>
        {d.status === 'expired' && <Notice tone="warning" icon={<Clock size={20} />}>{t('marathons.invite.page.expired')}</Notice>}
        {d.status === 'revoked' && <Notice tone="neutral" icon={<X size={20} />}>{t('marathons.invite.page.revoked')}</Notice>}
        {d.status === 'accepted' && <Notice tone="success" icon={<CheckCircle2 size={20} />}>{t('marathons.invite.page.used')}</Notice>}
        {acceptError && <Notice tone="danger" icon={<AlertCircle size={20} />}><span role="alert">{acceptError}</span></Notice>}
        {d.status === 'pending' && (me.data ? (
          <div><Button size="lg" loading={accept.isPending} onClick={() => accept.mutate()}>{t('marathons.invite.page.accept')}</Button></div>
        ) : (
          <div>
            <Link href={next} className="inline-flex h-11 items-center rounded-md bg-primary px-5 text-base font-medium text-white transition-colors hover:bg-primary-hover focus-visible:shadow-focus focus-visible:outline-none">
              {t('marathons.invite.page.signIn')}
            </Link>
          </div>
        ))}
      </div>
    );
  }
  return (
    <main className="flex min-h-screen items-start justify-center bg-surface-alt px-4 py-10 sm:items-center">
      <div className="w-full max-w-[520px] rounded-2xl bg-surface p-6 shadow-card sm:p-8">
        <div className="mb-6"><Logo /></div>
        {body}
      </div>
    </main>
  );
}

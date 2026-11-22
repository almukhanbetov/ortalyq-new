'use client';

import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import { MailCheck } from 'lucide-react';
import { Button } from '@/components/ui';
import { ApiError } from '@/lib/api/client';
import { useT } from '@/lib/i18n';
import { AuthCard, FormAlert } from './AuthCard';

// Shared pieces of the e-mail link flows (teacher sign-up, password reset).

/** Seconds left until a deadline set from a server answer; re-renders once a second while it runs. */
export function useCountdown() {
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
export const clock = (sec: number) => `${Math.floor(sec / 60)}:${String(sec % 60).padStart(2, '0')}`;

export function retryAfter(err: unknown): number {
  return err instanceof ApiError && err.status === 429 ? Number(err.fields.retry_after) || 60 : 0;
}

/**
 * Takes the token from the fragment of the letter link (#…), removes it from the address bar and keeps it for this
 * tab only, so a reload still works but the token does not stay in history.
 */
export function useLinkToken(store: string) {
  const [token, setToken] = useState<string | null | undefined>(undefined);
  const seen = useRef<string | null>(null); // survives the double effect run of React strict mode
  useEffect(() => {
    const read = () => {
      const fromHash = window.location.hash.slice(1);
      // A fragment that is present but malformed is an invalid link, not "use the stored one".
      let tok = fromHash ? (/^[A-Za-z0-9_-]{43}$/.test(fromHash) ? fromHash : '') : seen.current;
      seen.current = tok || null;
      try {
        if (tok) sessionStorage.setItem(store, tok);
        else if (tok === null) tok = sessionStorage.getItem(store);
        else sessionStorage.removeItem(store);
        if (tok && !/^[A-Za-z0-9_-]{43}$/.test(tok)) tok = null;
      } catch { /* storage unavailable: the token from the link still works */ }
      if (window.location.hash) window.history.replaceState(null, '', window.location.pathname);
      setToken(tok || null);
    };
    read();
    // Another link opened in the same tab changes only the fragment: the page is not remounted.
    window.addEventListener('hashchange', read);
    return () => window.removeEventListener('hashchange', read);
  }, [store]);
  const forget = () => { try { sessionStorage.removeItem(store); } catch { /* ignore */ } };
  return [token, forget] as const;
}

/**
 * "Check your mail" with «Отправить ещё раз» (locked while the server-side pause runs, with a timer) and
 * «Изменить email». `waiting` means the server answered 429: the message already shows the timer.
 */
export function CheckMailCard({ text, hint, message, left, waiting, pending, onResend, onChangeEmail, footer }: {
  text: ReactNode; hint: ReactNode; message: ReactNode; left: number; waiting: boolean; pending: boolean;
  onResend: () => void; onChangeEmail: () => void; footer?: ReactNode;
}) {
  const t = useT();
  return (
    <AuthCard title={t('signup.sentTitle')} footer={footer}>
      <div className="flex flex-col gap-4">
        <span className="flex h-12 w-12 items-center justify-center rounded-full bg-primary-soft text-primary-text"><MailCheck size={24} aria-hidden /></span>
        <p className="text-base text-ink" aria-live="polite">{text}</p>
        <p className="text-sm text-ink-muted">{hint}</p>
        {message && <FormAlert>{message}</FormAlert>}
        <div className="flex flex-col gap-2">
          <Button block disabled={left > 0} loading={pending} onClick={onResend}>
            {left > 0 && !waiting ? t('signup.resendIn', { time: clock(left) }) : t('signup.resend')}
          </Button>
          <Button block variant="secondary" onClick={onChangeEmail}>{t('signup.changeEmail')}</Button>
        </div>
      </div>
    </AuthCard>
  );
}

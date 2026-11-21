// Time is stored in UTC by the API and shown in the user's time zone.
import type { Locale } from '@/types/api';

const intlLocale: Record<Locale, string> = { ru: 'ru-RU', kk: 'kk-KZ', en: 'en-GB' };

function safeTz(tz?: string) {
  try {
    if (tz) Intl.DateTimeFormat(undefined, { timeZone: tz });
    return tz;
  } catch {
    return undefined;
  }
}

export function formatTime(iso: string, locale: Locale, tz?: string) {
  return new Intl.DateTimeFormat(intlLocale[locale], { hour: '2-digit', minute: '2-digit', timeZone: safeTz(tz) }).format(new Date(iso));
}

export function formatDateTime(iso: string, locale: Locale, tz?: string) {
  return new Intl.DateTimeFormat(intlLocale[locale], { day: 'numeric', month: 'long', hour: '2-digit', minute: '2-digit', timeZone: safeTz(tz) }).format(new Date(iso));
}

export function formatDay(iso: string, locale: Locale, tz?: string) {
  return new Intl.DateTimeFormat(intlLocale[locale], { day: 'numeric', month: 'long', year: 'numeric', timeZone: safeTz(tz) }).format(new Date(iso));
}

/** Calendar-day key in the user's time zone, used for chat date separators. */
export function dayKey(iso: string, tz?: string) {
  return new Intl.DateTimeFormat('en-CA', { year: 'numeric', month: '2-digit', day: '2-digit', timeZone: safeTz(tz) }).format(new Date(iso));
}

export function formatDuration(totalSeconds: number) {
  const s = Math.max(0, Math.floor(totalSeconds));
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const sec = s % 60;
  const pad = (n: number) => String(n).padStart(2, '0');
  return h > 0 ? `${h}:${pad(m)}:${pad(sec)}` : `${pad(m)}:${pad(sec)}`;
}

export function formatBytes(n: number) {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(0)} KB`;
  return `${(n / 1024 / 1024).toFixed(1)} MB`;
}

export function initials(name: string) {
  return name
    .split(/\s+/)
    .map((p) => p.replace(/[^\p{L}\p{N}]/gu, ''))
    .filter(Boolean)
    .slice(0, 2)
    .map((p) => p[0]?.toUpperCase())
    .join('');
}

export function newClientId() {
  return crypto.randomUUID();
}

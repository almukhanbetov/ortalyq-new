// Marathon dates are shown and edited in the marathon's own time zone (not the browser's),
// so "17 ноября в 17:00 (UTC+5)" means the same moment for the teacher and every student.
import { useEffect, useState } from 'react';
import type { Locale } from '@/types/api';

const INTL_LOCALE: Record<Locale, string> = { ru: 'ru-RU', kk: 'kk-KZ', en: 'en-GB' };

function wallParts(d: Date, timeZone: string) {
  const p = Object.fromEntries(
    new Intl.DateTimeFormat('en-US', { timeZone, year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hourCycle: 'h23' })
      .formatToParts(d).map((x) => [x.type, x.value]),
  );
  return { y: +p.year, m: +p.month, d: +p.day, h: +p.hour % 24, min: +p.minute };
}

/** Offset of the zone from UTC at the given moment, in minutes. */
function offsetMinutes(d: Date, timeZone: string) {
  const p = wallParts(d, timeZone);
  const atMinute = Math.floor(d.getTime() / 60000) * 60000; // wall parts have no seconds
  return Math.round((Date.UTC(p.y, p.m - 1, p.d, p.h, p.min) - atMinute) / 60000);
}

/** "UTC+5", "UTC+5:30", "UTC-3". */
export function zoneLabel(d: Date, timeZone: string) {
  const off = offsetMinutes(d, timeZone);
  const sign = off < 0 ? '-' : '+';
  const abs = Math.abs(off);
  return `UTC${sign}${Math.floor(abs / 60)}${abs % 60 ? `:${String(abs % 60).padStart(2, '0')}` : ''}`;
}

/** Parts for the banner text: date, time and zone in the marathon time zone. */
export function formatMarathonMoment(iso: string, timeZone: string, locale: Locale) {
  const d = new Date(iso);
  const loc = INTL_LOCALE[locale];
  return {
    // "17 ноября 2026" / "17 қараша 2026" / "17 November 2026" (without the "г."/"ж." suffix of the full format).
    date: `${new Intl.DateTimeFormat(loc, { timeZone, day: 'numeric', month: 'long' }).format(d)} ${wallParts(d, timeZone).y}`,
    time: new Intl.DateTimeFormat(loc, { timeZone, hour: '2-digit', minute: '2-digit', hourCycle: 'h23' }).format(d),
    zone: zoneLabel(d, timeZone),
  };
}

const pad = (n: number) => String(n).padStart(2, '0');

/** ISO moment → values for <input type="date"> and the time select, in the marathon zone. */
export function toZonedInputs(iso: string, timeZone: string) {
  const p = wallParts(new Date(iso), timeZone);
  return { date: `${p.y}-${pad(p.m)}-${pad(p.d)}`, time: `${pad(p.h)}:${pad(p.min)}` };
}

/** Wall date + time in the marathon zone → ISO moment (UTC). */
export function fromZonedInputs(date: string, time: string, timeZone: string) {
  const [y, m, d] = date.split('-').map(Number);
  const [h, min] = time.split(':').map(Number);
  const guess = Date.UTC(y, m - 1, d, h, min);
  let at = guess - offsetMinutes(new Date(guess), timeZone) * 60000;
  const second = guess - offsetMinutes(new Date(at), timeZone) * 60000; // daylight-saving edge
  if (second !== at) at = second;
  return new Date(at).toISOString();
}

/** Half-hour steps for the time select, as in the original. */
export const TIME_STEPS = Array.from({ length: 48 }, (_, i) => `${pad(Math.floor(i / 2))}:${i % 2 ? '30' : '00'}`);

/**
 * Calls onReached when the server clock reaches `at`. The skew between the browser and the server
 * (from server_now) is taken into account; the server still decides access on every request.
 */
export function useServerMoment(at: string | null | undefined, serverNow: string | undefined, onReached: () => void) {
  const [skew, setSkew] = useState(0);
  useEffect(() => {
    if (serverNow) setSkew(Date.parse(serverNow) - Date.now());
  }, [serverNow]);
  useEffect(() => {
    if (!at) return;
    const left = Date.parse(at) - (Date.now() + skew);
    if (left <= 0) return;
    // setTimeout overflows above ~24.8 days; re-arm on the next render in that case.
    const id = setTimeout(onReached, Math.min(left + 500, 2 ** 31 - 1));
    return () => clearTimeout(id);
  }, [at, skew, onReached]);
}

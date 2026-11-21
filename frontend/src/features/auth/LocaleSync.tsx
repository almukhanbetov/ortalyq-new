'use client';

import { useEffect } from 'react';
import { useMe } from '@/hooks/useMe';
import { useI18n } from '@/lib/i18n';

/** Applies the signed-in user's saved locale. */
export function LocaleSync() {
  const { data: me } = useMe();
  const { setLocale } = useI18n();
  useEffect(() => {
    if (me?.locale) setLocale(me.locale);
  }, [me?.locale, setLocale]);
  return null;
}

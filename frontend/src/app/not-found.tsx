'use client';

import Link from 'next/link';
import { useT } from '@/lib/i18n';
import { Logo } from '@/components/ui';

export default function NotFound() {
  const t = useT();
  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-4 p-6 text-center">
      <Logo />
      <p className="text-xl font-bold">{t('common.notFound')}</p>
      <Link href="/" className="text-primary-text underline">{t('common.toHome')}</Link>
    </div>
  );
}

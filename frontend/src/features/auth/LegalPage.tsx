'use client';

import Link from 'next/link';
import { useT } from '@/lib/i18n';
import { AuthCard } from './AuthCard';

/** /terms and /privacy: the documents are not published yet, and the page says so instead of inventing a text. */
export function LegalPage({ doc }: { doc: 'terms' | 'privacy' }) {
  const t = useT();
  return (
    <AuthCard title={t(doc === 'terms' ? 'legal.termsTitle' : 'legal.privacyTitle')} subtitle={t('legal.notPublished')}
      footer={<Link href="/signup" className="font-medium text-primary-text hover:underline">{t('legal.back')}</Link>} />
  );
}

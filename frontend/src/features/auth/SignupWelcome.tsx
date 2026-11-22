'use client';

import Link from 'next/link';
import { PartyPopper } from 'lucide-react';
import { buttonClass } from '@/components/ui';
import { useT } from '@/lib/i18n';
import { CLASSES_ROUTE } from '@/lib/routes';
import { AuthCard } from './AuthCard';

/** Step 4: the account exists and is signed in; the next stop is the teacher cabinet. */
export function SignupWelcome() {
  const t = useT();
  return (
    <AuthCard title={t('signup.welcomeTitle')} subtitle={t('signup.welcomeText')}>
      <div className="flex flex-col gap-4">
        <span className="flex h-12 w-12 items-center justify-center rounded-full bg-primary-soft text-primary-text"><PartyPopper size={24} aria-hidden /></span>
        <p className="text-sm text-ink-muted">{t('signup.welcomeLater')}</p>
        <Link href={CLASSES_ROUTE} className={buttonClass({ size: 'lg', block: true })}>{t('signup.toCabinet')}</Link>
      </div>
    </AuthCard>
  );
}

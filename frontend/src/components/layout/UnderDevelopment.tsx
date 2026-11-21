'use client';

import Link from 'next/link';
import { Construction } from 'lucide-react';
import { Badge } from '@/components/ui';
import { useT } from '@/lib/i18n';
import { PageCard } from './PageCard';
import { CLASSES_ROUTE } from '@/lib/routes';

/** Honest placeholder for menu sections that are not built yet. No fake controls. */
export function UnderDevelopment({ section }: { section: string }) {
  const t = useT();
  const name = t(`nav.${section}`);
  return (
    <PageCard title={name}>
      <div className="flex flex-col items-center gap-3 py-16 text-center">
        <span className="flex h-14 w-14 items-center justify-center rounded-full bg-warning-soft text-warning-text"><Construction size={26} /></span>
        <Badge tone="warning">{t('wip.badge')}</Badge>
        <h2 className="text-lg font-bold text-ink">{t('wip.title')}</h2>
        <p className="max-w-md text-base text-ink-muted">{t('wip.text', { name })}</p>
        <Link href={CLASSES_ROUTE} className="mt-2 text-base font-medium text-primary-text hover:underline">{t('wip.back')}</Link>
      </div>
    </PageCard>
  );
}

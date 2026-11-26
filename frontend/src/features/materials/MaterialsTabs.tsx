'use client';

import Link from 'next/link';
import { useT } from '@/lib/i18n';
import { cn } from '@/components/ui';
import { boardsUrl, materialsCatalogUrl, materialsUrl } from '@/lib/routes';

/** «Личные материалы» | «Каталог» | «Интерактивные доски» under the «Материалы» heading. */
export function MaterialsTabs({ current }: { current: 'personal' | 'catalog' | 'boards' }) {
  const t = useT();
  const tabs = [
    { key: 'personal', href: materialsUrl(), label: t('materials.personal') },
    { key: 'catalog', href: materialsCatalogUrl(), label: t('materials.catalog') },
    { key: 'boards', href: boardsUrl(), label: t('boards.tab') },
  ] as const;
  return (
    <nav aria-label={t('materials.sectionsNav')} className="mt-4 flex gap-6 border-b border-line-soft">
      {tabs.map((x) => (
        <Link key={x.key} href={x.href} aria-current={x.key === current ? 'page' : undefined}
          className={cn('-mb-px border-b-2 pb-2 text-md', x.key === current ? 'border-primary font-bold text-ink-body' : 'border-transparent text-ink-muted hover:text-ink-body')}>
          {x.label}
        </Link>
      ))}
    </nav>
  );
}

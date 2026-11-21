'use client';

import { useRouter } from 'next/navigation';
import { useQueryClient } from '@tanstack/react-query';
import { ChevronDown, Languages, LogOut } from 'lucide-react';
import { Avatar, Menu, type MenuItem } from '@/components/ui';
import { authApi } from '@/lib/api/endpoints';
import { useI18n } from '@/lib/i18n';
import type { Locale, Me } from '@/types/api';

export function UserMenu({ me, compact }: { me: Me; compact?: boolean }) {
  const { t, locale, setLocale } = useI18n();
  const qc = useQueryClient();
  const router = useRouter();

  const changeLocale = async (l: Locale) => {
    setLocale(l);
    try {
      const updated = await authApi.updateMe({ locale: l });
      qc.setQueryData(['me'], updated);
    } catch {
      /* local preference still applied */
    }
  };

  const items: MenuItem[] = [
    ...(['ru', 'kk', 'en'] as Locale[]).map((l) => ({
      key: l,
      label: (
        <span className="flex w-full items-center justify-between gap-3">
          {t(`locales.${l}`)} {l === locale && <span className="text-primary-text" aria-label="✓">✓</span>}
        </span>
      ),
      icon: <Languages size={16} />,
      onSelect: () => changeLocale(l),
    })),
    {
      key: 'logout',
      label: t('common.logout'),
      icon: <LogOut size={16} />,
      tone: 'danger' as const,
      onSelect: async () => {
        await authApi.logout().catch(() => {});
        qc.clear();
        router.replace('/login');
      },
    },
  ];

  return (
    <Menu
      label={t('nav.menu')}
      items={items}
      trigger={(p) => (
        <button {...p} type="button" className="flex items-center gap-2 rounded-lg px-1.5 py-1 hover:bg-surface-hover" aria-label={t('nav.menu')}>
          <Avatar name={me.full_name} color={me.avatar_color} size={32} />
          {!compact && (
            <span className="hidden text-left leading-tight sm:block">
              <span className="block max-w-[160px] truncate text-sm font-medium text-ink">{me.full_name}</span>
              <span className="block text-xs text-ink-muted">{t(`roles.${me.role}`)}</span>
            </span>
          )}
          <ChevronDown size={16} className="text-ink-muted" />
        </button>
      )}
    />
  );
}

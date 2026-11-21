'use client';

import Link from 'next/link';
import { Menu, Pin, PinOff } from 'lucide-react';
import { Logo, cn } from '@/components/ui';
import { UserMenu } from '@/features/auth/UserMenu';
import { useT } from '@/lib/i18n';
import type { Me } from '@/types/api';
import { CLASSES_ROUTE } from '@/lib/routes';

export function CabinetHeader({ me, pinned, onTogglePin, mobileOpen, onOpenMobile }: {
  me: Me; pinned: boolean; onTogglePin: () => void; mobileOpen: boolean; onOpenMobile: () => void;
}) {
  const t = useT();
  return (
    <header className="fixed inset-x-0 top-0 z-40 flex h-cabinetbar items-center gap-2 border-b border-line-shell bg-surface pl-2 pr-4 md:pl-6 md:pr-6">
      <button type="button" onClick={onOpenMobile} aria-label={t('nav.open')} aria-expanded={mobileOpen} aria-controls="app-sidebar"
        className="flex h-10 w-10 items-center justify-center rounded-lg text-ink-muted hover:bg-surface-hover hover:text-ink md:hidden">
        <Menu size={22} />
      </button>
      <Link href={me.role === 'admin' ? '/admin' : CLASSES_ROUTE} className="flex items-center rounded-md" aria-label="ORTALYQ-NEW">
        <Logo />
      </Link>
      <span className="ml-2 hidden h-8 w-px bg-line-shell md:block" aria-hidden />
      <button type="button" onClick={onTogglePin} aria-label={pinned ? t('nav.unpin') : t('nav.pin')} title={pinned ? t('nav.unpin') : t('nav.pin')}
        aria-pressed={pinned} aria-controls="app-sidebar"
        className={cn('hidden h-10 w-10 items-center justify-center rounded-lg md:flex',
          pinned ? 'bg-primary-soft text-primary-text hover:bg-[#d6f2ff]' : 'text-ink-faint hover:bg-surface-hover hover:text-ink-body')}>
        {pinned ? <PinOff size={19} /> : <Pin size={19} />}
      </button>
      <div className="ml-auto"><UserMenu me={me} /></div>
    </header>
  );
}

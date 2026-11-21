'use client';

import Link from 'next/link';
import { forwardRef, type HTMLAttributes } from 'react';
import { X } from 'lucide-react';
import { Logo, cn } from '@/components/ui';
import { useT } from '@/lib/i18n';
import type { Role } from '@/types/api';
import { CABINET_DIVIDER_AFTER, CABINET_NAV, isActivePath } from './navigation';

interface Props extends HTMLAttributes<HTMLElement> {
  role: Role;
  path: string;
  expanded: boolean;
  peeking: boolean;
  animate: boolean;
  mobileOpen: boolean;
  onCloseMobile: () => void;
}

/** The only cabinet menu: narrow rail on desktop (expands on hover/keyboard focus), drawer on phones. */
export const CabinetSidebar = forwardRef<HTMLElement, Props>(function CabinetSidebar(
  { role, path, expanded, peeking, animate, mobileOpen, onCloseMobile, ...panelProps },
  ref,
) {
  const t = useT();
  const items = CABINET_NAV.filter((n) => n.roles.includes(role));
  return (
    <aside ref={ref} id="app-sidebar" aria-label={t('nav.main')} data-expanded={expanded}
      {...panelProps}
      className={cn(
        'fixed bottom-0 left-0 z-50 flex flex-col overflow-hidden border-r border-line-shell bg-surface',
        'top-0 w-[272px] transition-transform duration-200 ease-out',
        mobileOpen ? 'translate-x-0 shadow-modal' : '-translate-x-full',
        'md:top-cabinetbar md:z-30 md:w-[var(--sidebar-w)] md:translate-x-0 md:shadow-none',
        peeking && 'md:shadow-pop',
        animate && 'md:transition-[width,box-shadow] md:duration-200 md:ease-out',
      )}>
      <div className="flex h-topbar shrink-0 items-center justify-between border-b border-line pl-4 pr-2 md:hidden">
        <Logo />
        <button type="button" onClick={onCloseMobile} aria-label={t('nav.close')}
          className="flex h-10 w-10 items-center justify-center rounded-lg text-ink-muted hover:bg-surface-hover hover:text-ink"><X size={20} /></button>
      </div>
      <nav className="flex-1 overflow-y-auto overflow-x-hidden px-3 py-2 scrollbar-thin">
        <ul className="flex flex-col">
          {items.map((n) => {
            const active = isActivePath(path, n.href, n.also);
            return (
              <li key={n.href}>
                <Link href={n.href} aria-current={active ? 'page' : undefined}
                  className={cn('group relative flex h-12 items-center gap-[22px] rounded-lg px-[10px] text-lg font-medium transition-colors',
                    active ? 'text-primary' : 'text-ink-faint hover:text-ink-body')}>
                  {active && <span className="absolute -left-3 inset-y-px w-[3px] rounded-r bg-primary" aria-hidden />}
                  <span className={cn('flex w-5 shrink-0 justify-center', active ? 'text-primary' : 'text-ink-faint group-hover:text-ink-body')}>{n.icon}</span>
                  <span className={cn('truncate whitespace-nowrap transition-opacity duration-150', !expanded && 'md:pointer-events-none md:opacity-0')}>{t(`nav.${n.key}`)}</span>
                </Link>
                {CABINET_DIVIDER_AFTER.has(n.key) && <hr className="mx-1 my-2 border-line-shell" />}
              </li>
            );
          })}
        </ul>
      </nav>
    </aside>
  );
});

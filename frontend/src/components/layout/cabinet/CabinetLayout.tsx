'use client';

import { usePathname } from 'next/navigation';
import { useEffect, useRef, useState, type ReactNode } from 'react';
import { cn } from '@/components/ui';
import { useT } from '@/lib/i18n';
import { useSidebar } from '@/hooks/useSidebar';
import type { Me } from '@/types/api';
import { CabinetHeader } from './CabinetHeader';
import { CabinetSidebar } from './CabinetSidebar';
import { SIDEBAR_COLLAPSED, SIDEBAR_EXPANDED } from './navigation';

/**
 * The single shell of the personal cabinet (header + sidebar + content), mounted once in
 * app/(app)/layout.tsx so it persists across page changes. The classroom has its own shell.
 */
export function CabinetLayout({ me, children }: { me: Me; children: ReactNode }) {
  const t = useT();
  const path = usePathname();
  const panel = useRef<HTMLElement>(null);
  const sb = useSidebar(panel);
  const [mobileOpen, setMobileOpen] = useState(false);

  useEffect(() => setMobileOpen(false), [path]);
  useEffect(() => {
    if (!mobileOpen) return;
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && setMobileOpen(false);
    document.addEventListener('keydown', onKey);
    const overflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    return () => {
      document.removeEventListener('keydown', onKey);
      document.body.style.overflow = overflow;
    };
  }, [mobileOpen]);

  // Content offset follows only the pinned mode: a hover/focus peek overlays the page.
  const vars = {
    ['--sidebar-w' as string]: `${sb.expanded ? SIDEBAR_EXPANDED : SIDEBAR_COLLAPSED}px`,
    ['--content-offset' as string]: `${sb.pinned ? SIDEBAR_EXPANDED : SIDEBAR_COLLAPSED}px`,
  };

  return (
    <div className="min-h-screen bg-page" style={vars}>
      <a href="#main" className="sr-only-focusable fixed left-2 top-2 z-50 rounded bg-surface px-3 py-2 text-sm">{t('common.skipToContent')}</a>
      <CabinetHeader me={me} pinned={sb.pinned} onTogglePin={sb.togglePin} mobileOpen={mobileOpen} onOpenMobile={() => setMobileOpen(true)} />
      <div aria-hidden onClick={() => setMobileOpen(false)}
        className={cn('fixed inset-0 z-40 bg-ink/40 transition-opacity duration-200 md:hidden', mobileOpen ? 'opacity-100' : 'pointer-events-none opacity-0')} />
      <CabinetSidebar ref={panel} role={me.role} path={path} expanded={sb.expanded} peeking={sb.peeking} animate={sb.ready}
        mobileOpen={mobileOpen} onCloseMobile={() => setMobileOpen(false)} {...sb.panelProps} />
      <main id="main" className={cn('px-4 pb-4 pt-[calc(64px+16px)] md:pl-[calc(var(--content-offset)+32px)] md:pr-[30px]', sb.ready && 'md:transition-[padding] md:duration-200')}>
        {children}
      </main>
    </div>
  );
}

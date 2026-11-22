'use client';

import type { ReactNode } from 'react';
import Link from 'next/link';
import { BookOpen, Check, Trophy, Users } from 'lucide-react';
import { Logo } from '@/components/ui';
import { cn } from '@/components/ui/cn';
import { useI18n, useT } from '@/lib/i18n';
import type { Locale } from '@/types/api';

/** Language buttons under the sign-in and sign-up cards. */
export function LanguageSwitch() {
  const { t, locale, setLocale } = useI18n();
  return (
    <div className="mt-6 flex justify-center gap-1" role="group" aria-label={t('common.language')}>
      {(['ru', 'kk', 'en'] as Locale[]).map((l) => (
        <button key={l} type="button" onClick={() => setLocale(l)} aria-pressed={l === locale}
          className={`rounded-md px-3 py-1.5 text-sm ${l === locale ? 'bg-surface font-medium text-ink shadow-card' : 'text-ink-muted hover:text-ink'}`}>
          {t(`locales.${l}`)}
        </button>
      ))}
    </div>
  );
}

export type AuthTab = 'login' | 'signup';

/** «Вход / Регистрация»: two links styled as a segmented control, so each form keeps its own address. */
function AuthSwitch({ active }: { active: AuthTab }) {
  const t = useT();
  const items: { tab: AuthTab; href: string; label: string }[] = [
    { tab: 'login', href: '/login', label: t('authLayout.login') },
    { tab: 'signup', href: '/signup', label: t('authLayout.signup') },
  ];
  return (
    <nav aria-label={t('authLayout.switch')} className="mb-6 grid grid-cols-2 rounded-lg bg-surface-hover p-1">
      {items.map((it) => (
        <Link key={it.tab} href={it.href} replace aria-current={it.tab === active ? 'page' : undefined}
          className={cn('flex h-9 items-center justify-center rounded-md text-base font-medium transition-colors focus-visible:shadow-focus focus-visible:outline-none',
            it.tab === active ? 'bg-surface text-ink shadow-card' : 'text-ink-muted hover:text-ink')}>
          {it.label}
        </Link>
      ))}
    </nav>
  );
}

/** Brand panel shown beside the form on wide screens. */
function BrandPanel() {
  const t = useT();
  const points = [
    { icon: BookOpen, text: t('authLayout.point1') },
    { icon: Users, text: t('authLayout.point2') },
    { icon: Trophy, text: t('authLayout.point3') },
  ];
  return (
    <aside className="relative hidden overflow-hidden bg-primary-soft lg:flex lg:w-[44%] lg:flex-col lg:justify-between lg:p-12 xl:p-16">
      <span aria-hidden className="pointer-events-none absolute -right-24 -top-24 h-72 w-72 rounded-full bg-primary/15" />
      <span aria-hidden className="pointer-events-none absolute -bottom-32 -left-16 h-80 w-80 rounded-full bg-primary/10" />
      <span aria-hidden className="pointer-events-none absolute bottom-40 right-16 h-6 w-6 rounded-full bg-warning" />
      <Link href="/" className="relative self-start rounded-md focus-visible:shadow-focus focus-visible:outline-none"><Logo name={t('authLayout.brand')} /></Link>
      <div className="relative max-w-md">
        <p className="text-sm font-medium uppercase tracking-wide text-primary-text">{t('authLayout.domain')}</p>
        <h2 className="mt-3 text-3xl font-bold text-ink">{t('authLayout.headline')}</h2>
        <ul className="mt-8 flex flex-col gap-4">
          {points.map(({ icon: Icon, text }) => (
            <li key={text} className="flex items-center gap-3 text-md text-ink-body">
              <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-surface text-primary-text shadow-card"><Icon size={20} aria-hidden /></span>
              {text}
            </li>
          ))}
        </ul>
      </div>
      <p className="relative flex items-center gap-2 text-sm text-ink-muted"><Check size={16} aria-hidden className="text-success" />{t('authLayout.languages')}</p>
    </aside>
  );
}

/**
 * Layout of the public auth screens (sign-in, sign-up steps, legal pages): a brand panel on wide screens and the
 * form card. `tab` adds the «Вход / Регистрация» switch above the form.
 */
export function AuthCard({ title, subtitle, children, footer, tab }: {
  title: ReactNode; subtitle?: ReactNode; children?: ReactNode; footer?: ReactNode; tab?: AuthTab;
}) {
  const t = useT();
  return (
    <div className="flex min-h-screen bg-page">
      <BrandPanel />
      <main className="flex flex-1 flex-col items-center px-4 py-8 sm:justify-center sm:py-12">
        <div className="w-full max-w-[420px]">
          <Link href="/" className="mb-6 flex justify-center rounded-md focus-visible:shadow-focus focus-visible:outline-none lg:hidden"><Logo name={t('authLayout.brand')} /></Link>
          <div className="rounded-2xl bg-surface p-5 shadow-card sm:p-8">
            {tab && <AuthSwitch active={tab} />}
            <h1 className="text-xl font-bold text-ink">{title}</h1>
            {subtitle && <p className="mt-1 text-sm text-ink-muted">{subtitle}</p>}
            {children && <div className="mt-6">{children}</div>}
            {footer && <div className="mt-5 text-sm text-ink-muted">{footer}</div>}
          </div>
          <LanguageSwitch />
        </div>
      </main>
    </div>
  );
}

export function FormAlert({ children }: { children: ReactNode }) {
  return <p role="alert" className="rounded-md bg-danger-soft px-3 py-2 text-sm text-danger-text">{children}</p>;
}

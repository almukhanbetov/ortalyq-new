'use client';

import { useEffect, useRef, useState, type ReactNode } from 'react';
import { cn } from './cn';

export interface MenuItem {
  key: string;
  label: ReactNode;
  icon?: ReactNode;
  onSelect: () => void;
  tone?: 'default' | 'danger';
  disabled?: boolean;
}

/** Accessible dropdown menu: Enter/Space opens, arrows move, Esc closes. */
export function Menu({ trigger, items, align = 'right', label }: {
  trigger: (props: { onClick: () => void; 'aria-expanded': boolean; 'aria-haspopup': 'menu'; ref: (el: HTMLButtonElement | null) => void }) => ReactNode;
  items: MenuItem[];
  align?: 'left' | 'right';
  label: string;
}) {
  const [open, setOpen] = useState(false);
  const root = useRef<HTMLDivElement>(null);
  const btn = useRef<HTMLButtonElement | null>(null);
  const refs = useRef<(HTMLButtonElement | null)[]>([]);

  useEffect(() => {
    if (!open) return;
    refs.current.find((r) => r && !r.disabled)?.focus();
    const onDoc = (e: MouseEvent) => !root.current?.contains(e.target as Node) && setOpen(false);
    document.addEventListener('mousedown', onDoc);
    return () => document.removeEventListener('mousedown', onDoc);
  }, [open]);

  const onKey = (e: React.KeyboardEvent) => {
    const enabled = refs.current.filter((r): r is HTMLButtonElement => !!r && !r.disabled);
    const i = enabled.indexOf(document.activeElement as HTMLButtonElement);
    if (e.key === 'Escape') {
      setOpen(false);
      btn.current?.focus();
    } else if (e.key === 'ArrowDown') {
      e.preventDefault();
      enabled[(i + 1) % enabled.length]?.focus();
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      enabled[(i - 1 + enabled.length) % enabled.length]?.focus();
    } else if (e.key === 'Tab') setOpen(false);
  };

  return (
    <div ref={root} className="relative inline-flex">
      {trigger({ onClick: () => setOpen((o) => !o), 'aria-expanded': open, 'aria-haspopup': 'menu', ref: (el) => { btn.current = el; } })}
      {open && (
        <div role="menu" aria-label={label} onKeyDown={onKey}
          className={cn('absolute top-full z-40 mt-1 min-w-[190px] rounded-xl border border-line bg-surface p-1.5 shadow-pop animate-pop-in', align === 'right' ? 'right-0' : 'left-0')}>
          {items.map((it, i) => (
            <button key={it.key} ref={(el) => { refs.current[i] = el; }} role="menuitem" type="button" disabled={it.disabled}
              onClick={() => { setOpen(false); it.onSelect(); }}
              className={cn('flex w-full items-center gap-2.5 rounded-md px-3 py-2 text-left text-base transition-colors focus:outline-none disabled:cursor-not-allowed disabled:opacity-40',
                it.tone === 'danger' ? 'text-danger-text hover:bg-danger-soft focus:bg-danger-soft' : 'text-ink hover:bg-surface-hover focus:bg-surface-hover')}>
              {it.icon && <span className="shrink-0 text-ink-muted" aria-hidden>{it.icon}</span>}
              {it.label}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

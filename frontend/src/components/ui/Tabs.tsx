'use client';

import { useRef, type KeyboardEvent, type ReactNode } from 'react';
import { cn } from './cn';

export interface TabItem<T extends string> {
  value: T;
  label: ReactNode;
  badge?: ReactNode;
}

/** Underlined tabs (Edvibe-like). Arrow keys move between tabs. */
export function Tabs<T extends string>({ items, value, onChange, className, size = 'md', idPrefix }: {
  items: TabItem<T>[];
  value: T;
  onChange: (v: T) => void;
  className?: string;
  size?: 'md' | 'lg';
  idPrefix?: string;
}) {
  const refs = useRef<(HTMLButtonElement | null)[]>([]);
  const onKey = (e: KeyboardEvent, i: number) => {
    const d = e.key === 'ArrowRight' ? 1 : e.key === 'ArrowLeft' ? -1 : 0;
    if (!d) return;
    e.preventDefault();
    const next = (i + d + items.length) % items.length;
    refs.current[next]?.focus();
    onChange(items[next].value);
  };
  return (
    <div role="tablist" className={cn('flex items-stretch gap-6', className)}>
      {items.map((it, i) => {
        const active = it.value === value;
        return (
          <button
            key={it.value}
            ref={(el) => {
              refs.current[i] = el;
            }}
            role="tab"
            id={idPrefix ? `${idPrefix}-tab-${it.value}` : undefined}
            aria-selected={active}
            tabIndex={active ? 0 : -1}
            onKeyDown={(e) => onKey(e, i)}
            onClick={() => onChange(it.value)}
            className={cn(
              'relative inline-flex items-center gap-2 whitespace-nowrap border-b-2 font-medium transition-colors',
              size === 'lg' ? 'h-14 text-md' : 'h-11 text-base',
              active ? 'border-primary text-ink' : 'border-transparent text-ink-muted hover:text-ink',
            )}
          >
            {it.label}
            {it.badge}
          </button>
        );
      })}
    </div>
  );
}

/** Pill segmented control. */
export function Segmented<T extends string>({ items, value, onChange }: { items: TabItem<T>[]; value: T; onChange: (v: T) => void }) {
  return (
    <div role="tablist" className="inline-flex rounded-lg bg-surface-hover p-1">
      {items.map((it) => (
        <button key={it.value} role="tab" aria-selected={it.value === value} onClick={() => onChange(it.value)}
          className={cn('h-8 rounded-md px-3 text-sm font-medium transition-colors', it.value === value ? 'bg-surface text-ink shadow-card' : 'text-ink-muted hover:text-ink')}>
          {it.label}
        </button>
      ))}
    </div>
  );
}

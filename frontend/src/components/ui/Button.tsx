'use client';

import { forwardRef, type ButtonHTMLAttributes, type ReactNode } from 'react';
import { cn } from './cn';
import { Spinner } from './Feedback';

type Variant = 'primary' | 'secondary' | 'soft' | 'ghost' | 'danger' | 'success' | 'warning';
type Size = 'sm' | 'md' | 'lg';

const variants: Record<Variant, string> = {
  primary: 'bg-primary text-white hover:bg-primary-hover active:bg-primary-active disabled:bg-line disabled:text-ink-faint',
  secondary: 'bg-surface text-ink border border-line-strong hover:bg-surface-hover active:bg-line disabled:text-ink-faint disabled:bg-surface',
  soft: 'bg-primary-soft text-primary-text hover:bg-[#d6f2ff] active:bg-[#c4ecff] disabled:bg-surface-alt disabled:text-ink-faint',
  ghost: 'bg-transparent text-ink-body hover:bg-surface-hover hover:text-ink active:bg-line disabled:text-ink-faint',
  danger: 'bg-danger text-white hover:bg-[#f03a3c] active:bg-[#d92f31] disabled:bg-line disabled:text-ink-faint',
  success: 'bg-success text-white hover:bg-[#12b475] active:bg-[#0fa068] disabled:bg-line disabled:text-ink-faint',
  warning: 'bg-pink-soft text-pink-text hover:bg-[#ffd9e7] active:bg-[#ffc9dc] disabled:bg-surface-alt disabled:text-ink-faint',
};

const sizes: Record<Size, string> = {
  sm: 'h-8 px-3 text-sm gap-1.5 rounded-md',
  md: 'h-10 px-4 text-base gap-2 rounded-md',
  lg: 'h-12 px-6 text-md gap-2 rounded-lg',
};

const base = 'inline-flex select-none items-center justify-center whitespace-nowrap font-medium transition-colors duration-150 disabled:cursor-not-allowed';

/** Button look for a link (`<Link className={buttonClass()}>`), so no button is nested inside an anchor. */
export function buttonClass({ variant = 'primary', size = 'md', block }: { variant?: Variant; size?: Size; block?: boolean } = {}) {
  return cn(base, variants[variant], sizes[size], block && 'w-full', 'focus-visible:shadow-focus focus-visible:outline-none');
}

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  size?: Size;
  loading?: boolean;
  icon?: ReactNode;
  block?: boolean;
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  { variant = 'primary', size = 'md', loading, icon, block, className, children, disabled, type = 'button', ...rest },
  ref,
) {
  return (
    <button
      ref={ref}
      type={type}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      className={cn(
        base,
        variants[variant],
        sizes[size],
        block && 'w-full',
        className,
      )}
      {...rest}
    >
      {loading ? <Spinner size={16} /> : icon}
      {typeof children === 'string' ? <span className="truncate">{children}</span> : children}
    </button>
  );
});

export interface IconButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  label: string;
  size?: 'sm' | 'md' | 'lg';
  active?: boolean;
  tone?: 'default' | 'danger';
}

export const IconButton = forwardRef<HTMLButtonElement, IconButtonProps>(function IconButton(
  { label, size = 'md', active, tone = 'default', className, children, type = 'button', ...rest },
  ref,
) {
  const dim = size === 'sm' ? 'h-7 w-7' : size === 'lg' ? 'h-11 w-11' : 'h-9 w-9';
  return (
    <button
      ref={ref}
      type={type}
      aria-label={label}
      title={label}
      className={cn(
        'inline-flex shrink-0 items-center justify-center rounded-md transition-colors disabled:cursor-not-allowed disabled:opacity-40',
        dim,
        active ? 'bg-primary-soft text-primary-text' : tone === 'danger' ? 'text-ink-muted hover:bg-danger-soft hover:text-danger-text' : 'text-ink-muted hover:bg-surface-hover hover:text-ink',
        className,
      )}
      {...rest}
    >
      {children}
    </button>
  );
});

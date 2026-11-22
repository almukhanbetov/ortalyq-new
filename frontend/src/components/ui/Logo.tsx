import { useId } from 'react';
import { cn } from './cn';

/** ORTALYQ-NEW mark: a ring (ortalyq = "centre") with an off-centre dot. Own artwork. */
export function LogoMark({ size = 28, className }: { size?: number; className?: string }) {
  // Unique per instance: a shared id breaks the fill when the first logo on the page is hidden (display: none).
  const g = `olq-g-${useId().replace(/:/g, '')}`;
  return (
    <svg width={size} height={size} viewBox="0 0 32 32" className={className} aria-hidden>
      <defs>
        <linearGradient id={g} x1="4" y1="4" x2="28" y2="28" gradientUnits="userSpaceOnUse">
          <stop stopColor="#1AC6FF" />
          <stop offset="1" stopColor="#5B6CFF" />
        </linearGradient>
      </defs>
      <rect width="32" height="32" rx="9" fill={`url(#${g})`} />
      <circle cx="15" cy="16.5" r="7.5" fill="none" stroke="#fff" strokeWidth="3.2" />
      <circle cx="23.5" cy="8.5" r="3" fill="#FFB020" />
    </svg>
  );
}

/** `name` replaces the default wordmark (the public screens show the public brand). */
export function Logo({ compact, name, className }: { compact?: boolean; name?: string; className?: string }) {
  return (
    <span className={cn('inline-flex items-center gap-2', className)}>
      <LogoMark />
      {!compact && (name
        ? <span className="text-md font-bold tracking-tight text-ink">{name}</span>
        : <span className="text-md font-bold tracking-tight text-ink">ORTALYQ<span className="text-primary">-NEW</span></span>)}
    </span>
  );
}

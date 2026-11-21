import { cn } from './cn';
import { initials } from '@/lib/format';

export function Avatar({ name, color, size = 32, className, ring }: { name: string; color: string; size?: number; className?: string; ring?: boolean }) {
  return (
    <span
      title={name}
      aria-label={name}
      role="img"
      className={cn('inline-flex shrink-0 select-none items-center justify-center rounded-full font-medium text-white', ring && 'ring-2 ring-surface', className)}
      style={{ width: size, height: size, background: color, fontSize: Math.max(10, Math.round(size * 0.38)) }}
    >
      {initials(name)}
    </span>
  );
}

export function AvatarStack({ people, size = 28, max = 4 }: { people: { name: string; color: string }[]; size?: number; max?: number }) {
  const shown = people.slice(0, max);
  const rest = people.length - shown.length;
  return (
    <span className="inline-flex items-center">
      {shown.map((p, i) => (
        <Avatar key={i} name={p.name} color={p.color} size={size} ring className={i ? '-ml-2' : ''} />
      ))}
      {rest > 0 && (
        <span className="-ml-2 inline-flex items-center justify-center rounded-full bg-surface-hover text-xs font-medium text-ink-body ring-2 ring-surface" style={{ width: size, height: size }}>
          +{rest}
        </span>
      )}
    </span>
  );
}

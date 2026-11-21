'use client';

import { useState } from 'react';
import { initials } from '@/lib/format';
import { attachmentUrl } from '@/lib/api/client';

/** Cover colours offered when creating a class or a marathon. */
export const COVER_COLORS = ['#1AC6FF', '#FF7A45', '#9254DE', '#16C784', '#F759AB', '#FFB020', '#2F80ED'];

/**
 * Generated class/course cover (no external images).
 * - `banner` (default): gradient with initials in the corner — material lists and pickers;
 * - `tile`: soft tint of the class colour with large centred initials — the classes page.
 * With `imageId` (a marathon or lesson cover picture, stage 10) the picture is shown; if it cannot be loaded
 * (deleted meanwhile, no access) the generated cover is shown instead.
 */
export function Cover({ color, title, className = '', variant = 'banner', imageId }: { color: string; title: string; className?: string; variant?: 'banner' | 'tile'; imageId?: string | null }) {
  const [failed, setFailed] = useState<string | null>(null);
  if (imageId && failed !== imageId) {
    return (
      <div className={`relative overflow-hidden bg-surface-alt ${className}`} aria-hidden data-cover-image={imageId}>
        {/* eslint-disable-next-line @next/next/no-img-element -- private attachment served by the API with the session cookie */}
        <img src={attachmentUrl(imageId, true)} alt="" className="h-full w-full object-cover" onError={() => setFailed(imageId)} />
      </div>
    );
  }
  if (variant === 'tile') {
    return (
      <div className={`relative flex items-center justify-center overflow-hidden ${className}`} style={{ background: `color-mix(in srgb, ${color} 24%, white)` }} aria-hidden>
        <span className="text-[48px] font-bold leading-none" style={{ color: `color-mix(in srgb, ${color} 62%, white)` }}>{initials(title)}</span>
      </div>
    );
  }
  return (
    <div className={`relative overflow-hidden ${className}`} style={{ background: `linear-gradient(135deg, ${color} 0%, ${color}cc 55%, #5B6CFF99 100%)` }} aria-hidden>
      <svg className="absolute inset-0 h-full w-full opacity-25" viewBox="0 0 200 120" preserveAspectRatio="xMidYMid slice">
        <circle cx="170" cy="18" r="34" fill="#fff" />
        <circle cx="22" cy="110" r="44" fill="#fff" fillOpacity=".6" />
        <rect x="110" y="70" width="60" height="60" rx="14" fill="#fff" fillOpacity=".5" transform="rotate(18 140 100)" />
      </svg>
      <span className="absolute bottom-3 left-4 text-2xl font-bold text-white drop-shadow-sm">{initials(title)}</span>
    </div>
  );
}

import type { Grade } from '@/types/api';

/** Own emotion icons for lesson grades (fine / good / amazing). */
export function GradeFace({ grade, size = 56 }: { grade: Grade; size?: number }) {
  const bg = { fine: '#FFD25A', good: '#FFC23D', amazing: '#FFB020' }[grade];
  return (
    <svg width={size} height={size} viewBox="0 0 64 64" aria-hidden>
      <circle cx="32" cy="32" r="30" fill={bg} />
      <circle cx="32" cy="32" r="30" fill="url(#gf-shine)" />
      <defs>
        <radialGradient id="gf-shine" cx="0.35" cy="0.3" r="0.8">
          <stop offset="0" stopColor="#fff" stopOpacity=".45" />
          <stop offset="1" stopColor="#fff" stopOpacity="0" />
        </radialGradient>
      </defs>
      {grade === 'fine' && (
        <>
          <circle cx="23" cy="27" r="3.6" fill="#3B2A10" />
          <circle cx="41" cy="27" r="3.6" fill="#3B2A10" />
          <path d="M23 42c3 2.6 6 3.6 9 3.6s6-1 9-3.6" stroke="#3B2A10" strokeWidth="3.2" fill="none" strokeLinecap="round" />
        </>
      )}
      {grade === 'good' && (
        <>
          <path d="M18 28c2-4 8-4 10 0M36 28c2-4 8-4 10 0" stroke="#3B2A10" strokeWidth="3.2" fill="none" strokeLinecap="round" />
          <path d="M19 37h26c0 8-6 13-13 13s-13-5-13-13z" fill="#3B2A10" />
          <path d="M24 45c2.5 2.5 5 3.4 8 3.4s5.5-.9 8-3.4" fill="#FF6B6B" />
          <circle cx="15" cy="38" r="4" fill="#FF8A65" opacity=".5" />
          <circle cx="49" cy="38" r="4" fill="#FF8A65" opacity=".5" />
        </>
      )}
      {grade === 'amazing' && (
        <>
          <path d="M23 18l2.6 5.4 5.9.8-4.3 4.1 1 5.8L23 31.3l-5.2 2.8 1-5.8-4.3-4.1 5.9-.8z" fill="#FF4D4F" />
          <path d="M41 18l2.6 5.4 5.9.8-4.3 4.1 1 5.8L41 31.3l-5.2 2.8 1-5.8-4.3-4.1 5.9-.8z" fill="#FF4D4F" />
          <path d="M18 38h28c0 9-6.5 14-14 14s-14-5-14-14z" fill="#3B2A10" />
          <path d="M23 46.5c2.7 2.6 5.8 3.6 9 3.6s6.3-1 9-3.6" fill="#FF6B6B" />
        </>
      )}
    </svg>
  );
}

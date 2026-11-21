'use client';

import { ChevronLeft, ChevronRight } from 'lucide-react';
import { useT } from '@/lib/i18n';
import { IconButton } from './Button';

export function Pagination({ total, limit, offset, onChange }: { total: number; limit: number; offset: number; onChange: (offset: number) => void }) {
  const t = useT();
  if (total <= limit) return null;
  return (
    <nav className="mt-6 flex items-center justify-end gap-2 text-sm text-ink-muted" aria-label="pagination">
      <span>{t('pagination.range', { a: offset + 1, b: Math.min(total, offset + limit), n: total })}</span>
      <IconButton label={t('pagination.prev')} disabled={offset === 0} onClick={() => onChange(Math.max(0, offset - limit))}><ChevronLeft size={18} /></IconButton>
      <IconButton label={t('pagination.next')} disabled={offset + limit >= total} onClick={() => onChange(offset + limit)}><ChevronRight size={18} /></IconButton>
    </nav>
  );
}

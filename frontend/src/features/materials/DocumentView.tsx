'use client';

import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';
import { ArrowLeft, FileSearch } from 'lucide-react';
import { EmptyState, ErrorState, Loading, buttonClass } from '@/components/ui';
import { PageCard } from '@/components/layout/PageCard';
import { ApiError } from '@/lib/api/client';
import { documentsApi } from '@/lib/api/endpoints';
import { formatDay } from '@/lib/format';
import { qk } from '@/lib/queryKeys';
import { useI18n } from '@/lib/i18n';
import { materialsUrl } from '@/lib/routes';
import { PdfViewer } from '@/features/exercises/PdfBlock';

/** A PDF document of «Личные материалы». The server answers 404 for anyone but its owner, also for the file itself. */
export function DocumentView({ id }: { id: string }) {
  const { t, locale } = useI18n();
  const q = useQuery({ queryKey: qk.document(id), queryFn: () => documentsApi.get(id) });
  const back = (
    <Link href={materialsUrl()} className="mb-4 inline-flex items-center gap-1 text-sm text-ink-muted hover:text-ink"><ArrowLeft size={16} />{t('materials.backToMaterials')}</Link>
  );
  if (q.isLoading) return <PageCard maxWidth={1024}><Loading /></PageCard>;
  if (q.error instanceof ApiError && (q.error.status === 404 || q.error.status === 403)) {
    return (
      <PageCard maxWidth={1024}>
        {back}
        <EmptyState icon={<FileSearch size={26} />} title={t('materials.documentNotFound')} text={t('materials.documentNotFoundHint')}
          action={<Link href={materialsUrl()} className={buttonClass({ variant: 'secondary' })}>{t('materials.backToMaterials')}</Link>} />
      </PageCard>
    );
  }
  if (q.error || !q.data) return <PageCard maxWidth={1024}>{back}<ErrorState error={q.error} onRetry={() => q.refetch()} /></PageCard>;
  const d = q.data;
  return (
    <PageCard maxWidth={1024}>
      {back}
      <p className="text-sm font-medium text-ink-faint">{t('materials.personal')} · PDF</p>
      <h1 className="mb-4 mt-1 break-words text-2xl font-bold text-ink" data-testid="document-title">{d.title}</h1>
      <p className="-mt-2 mb-4 text-sm text-ink-muted">{t('materials.uploadedAt', { date: formatDay(d.created_at, locale) })}</p>
      <PdfViewer url={documentsApi.fileUrl(d.id)} title={d.title} file={{ id: d.id, name: d.file_name, size_bytes: d.size_bytes }} />
    </PageCard>
  );
}

import { Suspense } from 'react';
import { Loading } from '@/components/ui';
import { CatalogList } from '@/features/materials/CatalogList';

/** /materials/catalog[?q=…&course=…] — «Материалы» → «Каталог». */
export default function MaterialsCatalogPage() {
  return (
    <Suspense fallback={<Loading />}>
      <CatalogList />
    </Suspense>
  );
}

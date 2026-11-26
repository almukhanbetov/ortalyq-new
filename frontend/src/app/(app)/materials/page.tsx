import { Suspense } from 'react';
import { Loading } from '@/components/ui';
import { CourseList } from '@/features/materials/CourseList';

/** /materials[?q=…] — «Материалы» → «Личные материалы». */
export default function MaterialsPage() {
  return (
    <Suspense fallback={<Loading />}>
      <CourseList />
    </Suspense>
  );
}

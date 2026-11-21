'use client';

import { Suspense } from 'react';
import { useMe } from '@/hooks/useMe';
import { Loading } from '@/components/ui';
import { ClassroomList } from '@/features/classrooms/ClassroomList';

/** /cabinet/school/classes[?type=1|2|group][&q=…] — the classes list (same path as the original cabinet). */
export default function ClassesPage() {
  const { data: me } = useMe();
  return me ? (
    <Suspense fallback={<Loading />}>
      <ClassroomList me={me} />
    </Suspense>
  ) : null;
}

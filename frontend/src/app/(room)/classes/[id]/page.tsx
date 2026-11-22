'use client';

import { Suspense } from 'react';
import { useParams } from 'next/navigation';
import { AuthGuard } from '@/features/auth/AuthGuard';
import { ClassroomRoom } from '@/features/classrooms/ClassroomRoom';
import { Loading } from '@/components/ui';
import { TEACHING_ROLES } from '@/lib/roles';

export default function ClassroomPage() {
  const { id } = useParams<{ id: string }>();
  return (
    <AuthGuard roles={[...TEACHING_ROLES, 'student']}>
      {(me) => (
        <Suspense fallback={<Loading className="min-h-screen" />}>
          <ClassroomRoom id={id} me={me} />
        </Suspense>
      )}
    </AuthGuard>
  );
}

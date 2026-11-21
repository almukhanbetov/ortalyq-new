'use client';

import { Suspense } from 'react';
import { useParams } from 'next/navigation';
import { AuthGuard } from '@/features/auth/AuthGuard';
import { ClassroomRoom } from '@/features/classrooms/ClassroomRoom';
import { Loading } from '@/components/ui';

export default function ClassroomPage() {
  const { id } = useParams<{ id: string }>();
  return (
    <AuthGuard roles={['teacher', 'student']}>
      {(me) => (
        <Suspense fallback={<Loading className="min-h-screen" />}>
          <ClassroomRoom id={id} me={me} />
        </Suspense>
      )}
    </AuthGuard>
  );
}

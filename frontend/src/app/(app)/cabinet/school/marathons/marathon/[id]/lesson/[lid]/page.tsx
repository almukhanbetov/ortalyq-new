'use client';

import { Suspense } from 'react';
import { useParams } from 'next/navigation';
import { Loading } from '@/components/ui';
import { MarathonLessonPlayer } from '@/features/marathons/MarathonLessonPlayer';

/** /cabinet/school/marathons/marathon/:id/lesson/:lid — a student takes a marathon lesson; with ?student= the owner reviews that student's answers. */
export default function MarathonLessonPage() {
  const { id, lid } = useParams<{ id: string; lid: string }>();
  return <Suspense fallback={<Loading />}><MarathonLessonPlayer id={id} lessonId={lid} /></Suspense>;
}

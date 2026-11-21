'use client';

import { useParams } from 'next/navigation';
import { MarathonLessonPreview } from '@/features/marathons/MarathonLessonPreview';

/** /cabinet/school/marathons/marathon/:id/preview/:lid — «Режим предпросмотра» of a lesson for its owner. */
export default function MarathonPreviewPage() {
  const { id, lid } = useParams<{ id: string; lid: string }>();
  return <MarathonLessonPreview id={id} lessonId={lid} />;
}

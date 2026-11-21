'use client';

import { useParams } from 'next/navigation';
import { LessonEditor } from '@/features/materials/LessonEditor';

export default function LessonPage() {
  const { id } = useParams<{ id: string }>();
  return <LessonEditor id={id} />;
}

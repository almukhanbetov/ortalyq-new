'use client';

import { useParams } from 'next/navigation';
import { CourseDetail } from '@/features/materials/CourseDetail';

export default function CoursePage() {
  const { id } = useParams<{ id: string }>();
  return <CourseDetail id={id} />;
}

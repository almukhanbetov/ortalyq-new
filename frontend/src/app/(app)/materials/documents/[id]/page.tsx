'use client';

import { useParams } from 'next/navigation';
import { DocumentView } from '@/features/materials/DocumentView';

/** /materials/documents/[id] — a PDF document of «Личные материалы». */
export default function DocumentPage() {
  const { id } = useParams<{ id: string }>();
  return <DocumentView id={id} />;
}

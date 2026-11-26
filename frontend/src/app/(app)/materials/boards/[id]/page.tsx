'use client';

import { useParams } from 'next/navigation';
import { BoardEditor } from '@/features/boards/BoardEditor';

/** /materials/boards/:id — the board editor; the server answers 404 to anyone but the owner. */
export default function BoardPage() {
  const { id } = useParams<{ id: string }>();
  return <BoardEditor id={id} />;
}

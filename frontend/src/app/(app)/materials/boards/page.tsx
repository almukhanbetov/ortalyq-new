import { Suspense } from 'react';
import { Loading } from '@/components/ui';
import { BoardList } from '@/features/boards/BoardList';

/** /materials/boards[?q=…] — «Материалы» → «Интерактивные доски». */
export default function BoardsPage() {
  return (
    <Suspense fallback={<Loading />}>
      <BoardList />
    </Suspense>
  );
}

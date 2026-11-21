'use client';

import { useParams } from 'next/navigation';
import { MyResultsPage } from '@/features/marathons/MarathonStats';

/** /cabinet/school/marathons/marathon/:id/results — «Успехи ученика в марафоне» for the student. */
export default function MarathonResultsPage() {
  const { id } = useParams<{ id: string }>();
  return <MyResultsPage id={id} />;
}

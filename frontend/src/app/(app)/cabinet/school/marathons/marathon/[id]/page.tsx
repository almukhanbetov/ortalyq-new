import { redirect } from 'next/navigation';
import { marathonUrl } from '@/lib/routes';

/** /cabinet/school/marathons/marathon/:id → the "Ученики" tab, as in the original. */
export default async function MarathonIndex({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  redirect(marathonUrl(id));
}

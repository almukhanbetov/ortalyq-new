'use client';

import { Suspense, useEffect } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { MARATHON_TABS, marathonUrl, type MarathonTab } from '@/lib/routes';
import { MarathonPage } from '@/features/marathons/MarathonPage';
import { Loading } from '@/components/ui';

/** /cabinet/school/marathons/marathon/:id/(students|lessons|statistics) */
export default function MarathonTabPage() {
  const { id, tab } = useParams<{ id: string; tab: string }>();
  const router = useRouter();
  const valid = (MARATHON_TABS as readonly string[]).includes(tab);
  useEffect(() => {
    if (!valid) router.replace(marathonUrl(id));
  }, [valid, id, router]);
  // The students tab keeps the open progress window in ?student=, read with useSearchParams.
  return valid ? <Suspense fallback={<Loading />}><MarathonPage id={id} tab={tab as MarathonTab} /></Suspense> : null;
}

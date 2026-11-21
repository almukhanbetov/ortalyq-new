'use client';

import { useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { useMe } from '@/hooks/useMe';
import { Loading } from '@/components/ui';
import { CLASSES_ROUTE } from '@/lib/routes';

export default function Home() {
  const me = useMe();
  const router = useRouter();
  useEffect(() => {
    if (me.data === null) router.replace('/login');
    else if (me.data) router.replace(me.data.role === 'admin' ? '/admin' : CLASSES_ROUTE);
  }, [me.data, router]);
  return <Loading className="min-h-screen" />;
}

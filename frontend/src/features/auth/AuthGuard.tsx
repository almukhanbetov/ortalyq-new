'use client';

import { useEffect, type ReactNode } from 'react';
import { usePathname, useRouter } from 'next/navigation';
import { useMe } from '@/hooks/useMe';
import { ErrorState, Loading, EmptyState } from '@/components/ui';
import { useT } from '@/lib/i18n';
import type { Me, Role } from '@/types/api';
import { ShieldAlert } from 'lucide-react';

/** Client-side guard for UX only; every API call is authorized on the server. */
export function AuthGuard({ roles, children }: { roles?: Role[]; children: (me: Me) => ReactNode }) {
  const t = useT();
  const me = useMe();
  const router = useRouter();
  const path = usePathname();

  useEffect(() => {
    if (me.data === null) router.replace(`/login?next=${encodeURIComponent(path)}`);
  }, [me.data, router, path]);

  if (me.isLoading || me.data === null) return <Loading className="min-h-screen" />;
  if (me.error || !me.data) return <ErrorState error={me.error} onRetry={() => me.refetch()} className="min-h-screen" />;
  if (roles && !roles.includes(me.data.role)) {
    return <EmptyState icon={<ShieldAlert size={26} />} title={t('errors.forbidden')} className="min-h-[60vh]" />;
  }
  return <>{children(me.data)}</>;
}

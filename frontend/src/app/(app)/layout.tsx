'use client';

import { AuthGuard } from '@/features/auth/AuthGuard';
import { CabinetLayout } from '@/components/layout/cabinet/CabinetLayout';

/** Every cabinet page renders inside this one shared shell (header + sidebar). */
export default function CabinetRouteLayout({ children }: { children: React.ReactNode }) {
  return <AuthGuard>{(me) => <CabinetLayout me={me}>{children}</CabinetLayout>}</AuthGuard>;
}

'use client';

import { AuthGuard } from '@/features/auth/AuthGuard';
import { TEACHING_ROLES } from '@/lib/roles';

export default function MaterialsLayout({ children }: { children: React.ReactNode }) {
  return <AuthGuard roles={TEACHING_ROLES}>{() => children}</AuthGuard>;
}

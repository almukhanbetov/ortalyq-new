'use client';

import { AuthGuard } from '@/features/auth/AuthGuard';
import { AdminPage } from '@/features/admin/AdminPage';

export default function Admin() {
  return <AuthGuard roles={['admin']}>{() => <AdminPage />}</AuthGuard>;
}

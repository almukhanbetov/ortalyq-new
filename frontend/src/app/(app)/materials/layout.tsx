'use client';

import { AuthGuard } from '@/features/auth/AuthGuard';

export default function MaterialsLayout({ children }: { children: React.ReactNode }) {
  return <AuthGuard roles={['teacher']}>{() => children}</AuthGuard>;
}

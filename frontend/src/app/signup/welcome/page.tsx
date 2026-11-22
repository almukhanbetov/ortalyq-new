'use client';

import { AuthGuard } from '@/features/auth/AuthGuard';
import { SignupWelcome } from '@/features/auth/SignupWelcome';

/** /signup/welcome — after the account is created (signed in). */
export default function SignupWelcomePage() {
  return <AuthGuard>{() => <SignupWelcome />}</AuthGuard>;
}

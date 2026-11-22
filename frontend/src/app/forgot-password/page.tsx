import type { Metadata } from 'next';
import { ForgotPasswordForm } from '@/features/auth/ForgotPasswordForm';

export const metadata: Metadata = { title: 'Bilimway' };

/** /forgot-password — password reset, step 1 (e-mail). */
export default function ForgotPasswordPage() {
  return <ForgotPasswordForm />;
}

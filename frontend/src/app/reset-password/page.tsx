import type { Metadata } from 'next';
import { ResetPassword } from '@/features/auth/ResetPassword';

export const metadata: Metadata = { title: 'Bilimway' };

/** /reset-password#<token> — the link from the reset letter; then the new password. */
export default function ResetPasswordPage() {
  return <ResetPassword />;
}

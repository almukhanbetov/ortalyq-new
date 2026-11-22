import type { Metadata } from 'next';
import { StudentInvite } from '@/features/auth/StudentInvite';

export const metadata: Metadata = { title: 'Bilimway' };

/** /student-invite#<token> — the link from a private teacher's invitation letter. */
export default function StudentInvitePage() {
  return <StudentInvite />;
}

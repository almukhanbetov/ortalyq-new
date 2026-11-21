'use client';

import { useParams } from 'next/navigation';
import { InvitationPage } from '@/features/marathons/MarathonInvites';

/** /invite/:token — the link from an invitation letter (works before signing in; accepting needs the invited account). */
export default function InvitePage() {
  const { token } = useParams<{ token: string }>();
  return <InvitationPage token={token} />;
}

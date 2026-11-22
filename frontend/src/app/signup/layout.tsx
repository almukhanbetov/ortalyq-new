import type { Metadata } from 'next';

/** Public sign-up screens carry the public brand in the tab title. */
export const metadata: Metadata = { title: 'Bilimway' };

export default function SignupLayout({ children }: { children: React.ReactNode }) {
  return children;
}

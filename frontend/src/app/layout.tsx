import type { Metadata, Viewport } from 'next';
import { Providers } from './providers';
// Self-hosted Ubuntu font (OFL). Each file declares per-subset unicode-ranges
// (latin, cyrillic, cyrillic-ext incl. Kazakh letters); browsers fetch only what is used.
import '@fontsource/ubuntu/400.css';
import '@fontsource/ubuntu/500.css';
import '@fontsource/ubuntu/700.css';
import '@/styles/globals.css';

export const metadata: Metadata = {
  title: 'ORTALYQ-NEW',
  description: 'ORTALYQ-NEW — учебная платформа для онлайн-занятий',
  icons: { icon: '/favicon.svg' },
};

export const viewport: Viewport = { width: 'device-width', initialScale: 1, themeColor: '#ffffff' };

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="ru">
      <body>
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}

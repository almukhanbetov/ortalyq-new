/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'standalone',
  reactStrictMode: true,
  poweredByHeader: false,
  images: { unoptimized: true },
  // Legacy list URL → original-style cabinet route on our own domain. Query (?q=, ?type=) is kept.
  // Only the exact /classes path is redirected; classrooms (/classes/:id) stay where they are.
  async redirects() {
    return [{ source: '/classes', destination: '/cabinet/school/classes', permanent: false }];
  },
};
export default nextConfig;

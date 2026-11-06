import type { NextConfig } from "next";

const backend = new URL(process.env.BACKEND_ORIGIN ?? "http://127.0.0.1:8080");
if (!['http:', 'https:'].includes(backend.protocol) || backend.username || backend.password || backend.pathname !== '/' || backend.search || backend.hash) {
  throw new Error("BACKEND_ORIGIN must be an HTTP(S) origin without credentials or a path");
}

const nextConfig: NextConfig = {
  poweredByHeader: false,
  // Keep the public site and Go API on the same browser origin. Business logic
  // and authorization remain in Go, not in Next.js route handlers.
  async rewrites() {
    return [{ source: "/api/:path*", destination: `${backend.origin}/api/:path*` }];
  },
};

export default nextConfig;

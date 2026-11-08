import type { NextConfig } from "next";

const backend = new URL(process.env.BACKEND_ORIGIN ?? "http://127.0.0.1:8080");
if (!['http:', 'https:'].includes(backend.protocol) || backend.username || backend.password || backend.pathname !== '/' || backend.search || backend.hash) {
  throw new Error("BACKEND_ORIGIN must be an HTTP(S) origin without credentials or a path");
}

const nextConfig: NextConfig = {
  output: "standalone",
  poweredByHeader: false,
  // Allow the 50 MiB archive plus multipart overhead through the API rewrite.
  // Go independently enforces per-file and expanded archive limits.
  experimental: { proxyClientMaxBodySize: "51mb", proxyTimeout: 60000 },
  // Keep the public site and Go API on the same browser origin. Business logic
  // and authorization remain in Go, not in Next.js route handlers.
  async rewrites() {
    return [{ source: "/api/:path*", destination: `${backend.origin}/api/:path*` }];
  },
};

export default nextConfig;

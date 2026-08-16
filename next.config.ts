import type { NextConfig } from "next";

// The Go engine's dashboard/API binds to loopback (default 127.0.0.1:8696).
// Proxying it under /engine/* keeps the browser same-origin and needs no CORS
// or engine changes — see docs/ADR-001-ui-engine-bridge.md.
const engineUrl = process.env.WINFORGE_ENGINE_URL ?? "http://127.0.0.1:8696";

const nextConfig: NextConfig = {
  async rewrites() {
    return [
      {
        source: "/engine/:path*",
        destination: `${engineUrl}/:path*`,
      },
    ];
  },
};

export default nextConfig;

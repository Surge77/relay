import type { NextConfig } from "next";

// The browser talks to the REST control plane through same-origin /api/* so the
// httpOnly refresh cookie works; Next proxies those to cmd/api. Realtime traffic
// goes directly to the gateway over WebSocket (NEXT_PUBLIC_GATEWAY_WS / ?gw=).
const API_ORIGIN = process.env.API_ORIGIN ?? "http://localhost:9000";

const nextConfig: NextConfig = {
  reactStrictMode: true,
  async rewrites() {
    return [{ source: "/api/:path*", destination: `${API_ORIGIN}/:path*` }];
  },
};

export default nextConfig;

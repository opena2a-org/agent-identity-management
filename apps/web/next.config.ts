import type { NextConfig } from 'next';
import { dashboardRedirects } from './lib/redirects';
import { dashboardHeaderRules } from './lib/content-security-policy';

// The API URL the build writes in where the code reads process.env.NEXT_PUBLIC_API_URL.
// A page rendered with NEXT_PUBLIC_API_URL in the server's environment hands the
// browser that value instead (lib/runtime-config.ts).
const apiUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

const nextConfig: NextConfig = {
  reactStrictMode: true,
  output: 'standalone', // Enable standalone output for Docker
  images: {
    // No page uses next/image, so the self-hosted image optimizer at /_next/image
    // only adds attack surface: it decodes attacker-chosen images with sharp/libheif.
    // unoptimized stops the component from requesting the route; the localPatterns
    // entry matches no path, so the route itself rejects every source.
    unoptimized: true,
    localPatterns: [{ pathname: '/__image-optimizer-closed__/**' }],
  },
  typescript: {
    // React 19 + recharts v2 type incompatibilities; tracked for recharts v3 upgrade
    ignoreBuildErrors: true,
  },
  env: {
    NEXT_PUBLIC_API_URL: apiUrl,
    NEXT_PUBLIC_APP_NAME: process.env.NEXT_PUBLIC_APP_NAME || 'Agent Identity Management',
  },
  // Content-Security-Policy on every dashboard page. The policy lives in
  // lib/content-security-policy.ts; each page also carries its connect-src
  // (app/layout.tsx).
  async headers() {
    return dashboardHeaderRules({
      development: process.env.NODE_ENV === 'development',
    });
  },
  // Moved dashboard pages. The table lives in lib/redirects.ts; add rows there, not here.
  async redirects() {
    return dashboardRedirects();
  },
  async rewrites() {
    // AIM_BACKEND_URL is the canonical backend URL for Vercel deployments.
    // Falls back to NEXT_PUBLIC_API_URL for local dev, then localhost.
    const backendUrl = process.env.AIM_BACKEND_URL
      || process.env.NEXT_PUBLIC_API_URL
      || 'http://localhost:8080';

    return {
      // afterFiles: runs after Next.js pages/static files, so the SPA still works
      afterFiles: [
        // Core API routes
        {
          source: '/api/v1/:path*',
          destination: `${backendUrl}/api/v1/:path*`,
        },
        // Legacy /api catch-all (preserves existing behavior)
        {
          source: '/api/:path*',
          destination: `${backendUrl}/api/:path*`,
        },
        // Health endpoints
        {
          source: '/health',
          destination: `${backendUrl}/health`,
        },
        {
          source: '/health/:path*',
          destination: `${backendUrl}/health/:path*`,
        },
        // Well-known discovery endpoints
        {
          source: '/.well-known/agent.json',
          destination: `${backendUrl}/.well-known/agent.json`,
        },
        {
          source: '/.well-known/aip',
          destination: `${backendUrl}/.well-known/aip`,
        },
        // /metrics is not proxied: the backend serves it on its own listener
        // (METRICS_LISTEN_ADDR), never through the dashboard origin.
      ],
    };
  },
};

export default nextConfig;

/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  output: 'standalone', // Enable standalone output for Docker
  typescript: {
    // React 19 + recharts v2 type incompatibilities; tracked for recharts v3 upgrade
    ignoreBuildErrors: true,
  },
  env: {
    NEXT_PUBLIC_API_URL: process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080',
    NEXT_PUBLIC_APP_NAME: process.env.NEXT_PUBLIC_APP_NAME || 'Agent Identity Management',
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
        // Metrics endpoint
        {
          source: '/metrics',
          destination: `${backendUrl}/metrics`,
        },
      ],
    };
  },
};

module.exports = nextConfig;

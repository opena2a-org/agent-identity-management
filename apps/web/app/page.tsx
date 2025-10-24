'use client';

import Link from 'next/link';
import { Shield, Lock, TrendingUp, Users, Activity, CheckCircle, ArrowRight, Github, Building2 } from 'lucide-react';

export default function Home() {
  return (
    <main className="min-h-screen bg-gradient-to-br from-gray-900 via-blue-900 to-purple-900">
      {/* Hero Section */}
      <div className="relative overflow-hidden">
        <div className="absolute inset-0 bg-[url('/grid.svg')] bg-center [mask-image:linear-gradient(180deg,white,rgba(255,255,255,0))]"></div>

        <div className="relative max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 pt-20 pb-32">
          {/* Header */}
          <div className="text-center mb-16">
            <div className="flex justify-center mb-6">
              <div className="p-4 bg-white/10 backdrop-blur-sm rounded-2xl">
                <Shield className="h-16 w-16 text-white" />
              </div>
            </div>

            <h1 className="text-5xl md:text-6xl font-bold text-white mb-6">
              Agent Identity Management
            </h1>

            <p className="text-xl text-blue-200 max-w-3xl mx-auto mb-8">
              Production-grade identity verification and security platform for AI agents and MCP servers.
              Built for scale, security, and compliance.
            </p>

            <div className="flex gap-4 justify-center flex-wrap">
              <Link
                href="/auth/login"
                className="group px-8 py-3.5 bg-blue-600 hover:bg-blue-700 text-white rounded-lg font-semibold
                         transition-all duration-200 transform hover:scale-105 active:scale-95
                         focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 focus:ring-offset-gray-900
                         flex items-center gap-2"
              >
                Sign In
                <ArrowRight className="h-5 w-5 group-hover:translate-x-1 transition-transform" />
              </Link>

              <Link
                href="https://github.com/opena2a/identity"
                target="_blank"
                className="px-8 py-3.5 bg-white/10 hover:bg-white/20 backdrop-blur-sm text-white border border-white/20
                         rounded-lg font-semibold transition-all duration-200
                         focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 focus:ring-offset-gray-900
                         flex items-center gap-2"
              >
                <Github className="h-5 w-5" />
                View on GitHub
              </Link>
            </div>
          </div>

          {/* Feature Cards */}
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6 mb-16">
            {/* Verification Card */}
            <div className="group bg-white/10 backdrop-blur-md rounded-2xl p-8 hover:bg-white/15 transition-all duration-300 border border-white/10">
              <div className="flex items-center justify-between mb-4">
                <div className="p-3 bg-blue-500/20 rounded-xl">
                  <Shield className="h-8 w-8 text-blue-400" />
                </div>
              </div>
              <h3 className="text-xl font-semibold text-white mb-3">Cryptographic Verification</h3>
              <p className="text-blue-200 leading-relaxed">
                Public key-based verification system ensuring authentic identity of AI agents and MCP servers with certificate management.
              </p>
            </div>

            {/* Trust Scoring Card */}
            <div className="group bg-white/10 backdrop-blur-md rounded-2xl p-8 hover:bg-white/15 transition-all duration-300 border border-white/10">
              <div className="flex items-center justify-between mb-4">
                <div className="p-3 bg-purple-500/20 rounded-xl">
                  <TrendingUp className="h-8 w-8 text-purple-400" />
                </div>
              </div>
              <h3 className="text-xl font-semibold text-white mb-3">ML-Powered Trust Scoring</h3>
              <p className="text-blue-200 leading-relaxed">
                8-factor trust algorithm analyzing verification status, security audits, community trust, and more for comprehensive risk assessment.
              </p>
            </div>

            {/* SSO Card */}
            <div className="group bg-white/10 backdrop-blur-md rounded-2xl p-8 hover:bg-white/15 transition-all duration-300 border border-white/10">
              <div className="flex items-center justify-between mb-4">
                <div className="p-3 bg-green-500/20 rounded-xl">
                  <Lock className="h-8 w-8 text-green-400" />
                </div>
              </div>
              <h3 className="text-xl font-semibold text-white mb-3">SSO Integration</h3>
              <p className="text-blue-200 leading-relaxed">
                OAuth2/OIDC integration with Google, Microsoft, and Okta. Zero passwords stored, JWT-based sessions with auto-provisioning.
              </p>
            </div>

            {/* Multi-Tenancy Card */}
            <div className="group bg-white/10 backdrop-blur-md rounded-2xl p-8 hover:bg-white/15 transition-all duration-300 border border-white/10">
              <div className="flex items-center justify-between mb-4">
                <div className="p-3 bg-orange-500/20 rounded-xl">
                  <Building2 className="h-8 w-8 text-orange-400" />
                </div>
              </div>
              <h3 className="text-xl font-semibold text-white mb-3">Multi-Tenant Architecture</h3>
              <p className="text-blue-200 leading-relaxed">
                Organization-level isolation with role-based access control. Perfect for enterprises managing multiple teams and environments.
              </p>
            </div>

            {/* Audit Logging Card */}
            <div className="group bg-white/10 backdrop-blur-md rounded-2xl p-8 hover:bg-white/15 transition-all duration-300 border border-white/10">
              <div className="flex items-center justify-between mb-4">
                <div className="p-3 bg-indigo-500/20 rounded-xl">
                  <Activity className="h-8 w-8 text-indigo-400" />
                </div>
              </div>
              <h3 className="text-xl font-semibold text-white mb-3">Complete Audit Trail</h3>
              <p className="text-blue-200 leading-relaxed">
                TimescaleDB-powered immutable audit logging. Track every agent action with full context and compliance-ready reporting.
              </p>
            </div>

            {/* API Management Card */}
            <div className="group bg-white/10 backdrop-blur-md rounded-2xl p-8 hover:bg-white/15 transition-all duration-300 border border-white/10">
              <div className="flex items-center justify-between mb-4">
                <div className="p-3 bg-cyan-500/20 rounded-xl">
                  <CheckCircle className="h-8 w-8 text-cyan-400" />
                </div>
              </div>
              <h3 className="text-xl font-semibold text-white mb-3">API Key Management</h3>
              <p className="text-blue-200 leading-relaxed">
                SHA-256 hashed API keys with expiration tracking, usage monitoring, and automated rotation for secure programmatic access.
              </p>
            </div>
          </div>

          {/* Stats Section */}
          <div className="grid grid-cols-2 md:grid-cols-4 gap-6 mb-16">
            <div className="text-center">
              <div className="text-4xl font-bold text-white mb-2">100%</div>
              <div className="text-blue-200">Test Coverage</div>
            </div>
            <div className="text-center">
              <div className="text-4xl font-bold text-white mb-2">&lt;100ms</div>
              <div className="text-blue-200">API Response</div>
            </div>
            <div className="text-center">
              <div className="text-4xl font-bold text-white mb-2">99.9%</div>
              <div className="text-blue-200">Uptime SLA</div>
            </div>
            <div className="text-center">
              <div className="text-4xl font-bold text-white mb-2">24/7</div>
              <div className="text-blue-200">Support</div>
            </div>
          </div>

          {/* Technology Stack */}
          <div className="bg-white/10 backdrop-blur-md rounded-2xl p-8 border border-white/10">
            <h2 className="text-2xl font-bold text-white mb-6 text-center">Production-Ready Technology Stack</h2>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-6 text-center">
              <div>
                <div className="text-lg font-semibold text-white mb-1">Backend</div>
                <div className="text-blue-200 text-sm">Go + Fiber v3</div>
              </div>
              <div>
                <div className="text-lg font-semibold text-white mb-1">Database</div>
                <div className="text-blue-200 text-sm">PostgreSQL 16</div>
              </div>
              <div>
                <div className="text-lg font-semibold text-white mb-1">Frontend</div>
                <div className="text-blue-200 text-sm">Next.js 15 + React 19</div>
              </div>
              <div>
                <div className="text-lg font-semibold text-white mb-1">Cache</div>
                <div className="text-blue-200 text-sm">Redis 7</div>
              </div>
            </div>
          </div>

          {/* Footer */}
          <div className="mt-16 text-center">
            <p className="text-blue-300 text-sm mb-2">
              Part of the{' '}
              <a
                href="https://opena2a.org"
                target="_blank"
                className="text-blue-400 hover:text-blue-300 font-medium underline decoration-blue-400/30 hover:decoration-blue-300"
              >
                OpenA2A
              </a>
              {' '}ecosystem
            </p>
            <p className="text-blue-400/60 text-xs">
              Built with Claude Sonnet 4.5 • AGPL-3.0 License
            </p>
          </div>
        </div>
      </div>
    </main>
  );
}

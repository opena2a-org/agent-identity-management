'use client';

import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { useState } from 'react';
import { useRouter } from 'next/navigation';

export default function NewAgentPage() {
  const router = useRouter();
  const [formData, setFormData] = useState({
    name: '',
    display_name: '',
    description: '',
    agent_type: 'ai_agent',
    version: '',
    repository_url: '',
    documentation_url: '',
    public_key: '',
  });

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    // TODO: Call API to create agent
    console.log('Creating agent:', formData);
    router.push('/dashboard/agents');
  };

  return (
    <div className="max-w-3xl mx-auto space-y-6">
      <div>
        <h1 className="text-3xl font-bold text-gray-900">Register New Agent</h1>
        <p className="mt-2 text-gray-600">
          Register an AI agent or MCP server for identity verification
        </p>
      </div>

      <form onSubmit={handleSubmit}>
        <Card>
          <CardHeader>
            <CardTitle>Agent Information</CardTitle>
            <CardDescription>
              Provide details about your agent or MCP server
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            {/* Agent Type */}
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Agent Type *
              </label>
              <div className="grid grid-cols-2 gap-4">
                <button
                  type="button"
                  onClick={() => setFormData({ ...formData, agent_type: 'ai_agent' })}
                  className={`p-4 border-2 rounded-lg transition-colors ${
                    formData.agent_type === 'ai_agent'
                      ? 'border-blue-600 bg-blue-50'
                      : 'border-gray-200 hover:border-gray-300'
                  }`}
                >
                  <div className="text-2xl mb-2">🤖</div>
                  <div className="font-semibold">AI Agent</div>
                  <div className="text-sm text-gray-500">
                    Autonomous AI assistant or chatbot
                  </div>
                </button>
                <button
                  type="button"
                  onClick={() => setFormData({ ...formData, agent_type: 'mcp_server' })}
                  className={`p-4 border-2 rounded-lg transition-colors ${
                    formData.agent_type === 'mcp_server'
                      ? 'border-blue-600 bg-blue-50'
                      : 'border-gray-200 hover:border-gray-300'
                  }`}
                >
                  <div className="text-2xl mb-2">🔧</div>
                  <div className="font-semibold">MCP Server</div>
                  <div className="text-sm text-gray-500">
                    Model Context Protocol server
                  </div>
                </button>
              </div>
            </div>

            {/* Name */}
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Name (Identifier) *
              </label>
              <input
                type="text"
                required
                placeholder="e.g., customer-support-agent"
                className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-600 focus:border-transparent"
                value={formData.name}
                onChange={(e) => setFormData({ ...formData, name: e.target.value })}
              />
              <p className="mt-1 text-sm text-gray-500">
                Lowercase, alphanumeric with hyphens (e.g., my-agent-name)
              </p>
            </div>

            {/* Display Name */}
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Display Name *
              </label>
              <input
                type="text"
                required
                placeholder="e.g., Customer Support Agent"
                className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-600 focus:border-transparent"
                value={formData.display_name}
                onChange={(e) => setFormData({ ...formData, display_name: e.target.value })}
              />
            </div>

            {/* Description */}
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Description *
              </label>
              <textarea
                required
                rows={4}
                placeholder="Describe what your agent does and its capabilities..."
                className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-600 focus:border-transparent"
                value={formData.description}
                onChange={(e) => setFormData({ ...formData, description: e.target.value })}
              />
            </div>

            {/* Version */}
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Version
              </label>
              <input
                type="text"
                placeholder="e.g., 1.0.0"
                className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-600 focus:border-transparent"
                value={formData.version}
                onChange={(e) => setFormData({ ...formData, version: e.target.value })}
              />
            </div>

            {/* Repository URL */}
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Repository URL
              </label>
              <input
                type="url"
                placeholder="https://github.com/org/repo"
                className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-600 focus:border-transparent"
                value={formData.repository_url}
                onChange={(e) => setFormData({ ...formData, repository_url: e.target.value })}
              />
              <p className="mt-1 text-sm text-gray-500">
                Improves trust score if provided
              </p>
            </div>

            {/* Documentation URL */}
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Documentation URL
              </label>
              <input
                type="url"
                placeholder="https://docs.example.com"
                className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-600 focus:border-transparent"
                value={formData.documentation_url}
                onChange={(e) => setFormData({ ...formData, documentation_url: e.target.value })}
              />
            </div>

            {/* Public Key */}
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Public Key (Optional)
              </label>
              <textarea
                rows={6}
                placeholder="-----BEGIN PUBLIC KEY-----&#10;...&#10;-----END PUBLIC KEY-----"
                className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-600 focus:border-transparent font-mono text-sm"
                value={formData.public_key}
                onChange={(e) => setFormData({ ...formData, public_key: e.target.value })}
              />
              <p className="mt-1 text-sm text-gray-500">
                Provide a PEM-encoded public key for cryptographic verification
              </p>
            </div>

            {/* Actions */}
            <div className="flex justify-end gap-4 pt-6 border-t">
              <Button
                type="button"
                variant="outline"
                onClick={() => router.back()}
              >
                Cancel
              </Button>
              <Button type="submit">
                Register Agent
              </Button>
            </div>
          </CardContent>
        </Card>
      </form>
    </div>
  );
}

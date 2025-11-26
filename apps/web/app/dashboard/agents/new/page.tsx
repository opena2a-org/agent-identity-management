'use client';

import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { api } from '@/lib/api';
import { AuthGuard } from '@/components/auth-guard';

interface AgentFormData {
  name: string;
  displayName: string;
  description: string;
  agentType: 'ai_agent' | 'mcp_server';
  version: string;
  repositoryUrl: string;
  documentationUrl: string;
}

export default function NewAgentPage() {
  const router = useRouter();
  const [formData, setFormData] = useState<AgentFormData>({
    name: '',
    displayName: '',
    description: '',
    agentType: 'ai_agent',
    version: '',
    repositoryUrl: '',
    documentationUrl: '',
    // ✅ REMOVED: public_key - AIM generates this automatically
  });

  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsSubmitting(true);
    setError(null);

    try {
      // Call API to create agent
      const response = await api.createAgent(formData);

      // Redirect to success page with the new agent ID
      router.push(`/dashboard/agents/${response.id}/success`);
    } catch (err: any) {
      console.error('Failed to create agent:', err);
      setError(err.message || 'Failed to create agent. Please try again.');
      setIsSubmitting(false);
    }
  };

  return (
    <AuthGuard>
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
                  onClick={() => setFormData({ ...formData, agentType: 'ai_agent' })}
                  className={`p-4 border-2 rounded-lg transition-colors ${
                    formData.agentType === 'ai_agent'
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
                  onClick={() => setFormData({ ...formData, agentType: 'mcp_server' })}
                  className={`p-4 border-2 rounded-lg transition-colors ${
                    formData.agentType === 'mcp_server'
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
                value={formData.displayName}
                onChange={(e) => setFormData({ ...formData, displayName: e.target.value })}
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
                value={formData.repositoryUrl}
                onChange={(e) => setFormData({ ...formData, repositoryUrl: e.target.value })}
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
                value={formData.documentationUrl}
                onChange={(e) => setFormData({ ...formData, documentationUrl: e.target.value })}
              />
            </div>

            {/* Error Message */}
            {error && (
              <div className="p-4 bg-red-50 border border-red-200 rounded-lg">
                <p className="text-sm text-red-800">{error}</p>
              </div>
            )}

            {/* Actions */}
            <div className="flex justify-end gap-4 pt-6 border-t">
              <Button
                type="button"
                variant="outline"
                onClick={() => router.back()}
                disabled={isSubmitting}
              >
                Cancel
              </Button>
              <Button type="submit" disabled={isSubmitting}>
                {isSubmitting ? (
                  <>
                    <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-white mr-2"></div>
                    Registering...
                  </>
                ) : (
                  'Register Agent'
                )}
              </Button>
            </div>
          </CardContent>
        </Card>
      </form>
    </div>
    </AuthGuard>
  );
}

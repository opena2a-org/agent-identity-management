'use client';

import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { api, AgentType } from '@/lib/api';
import { AuthGuard } from '@/components/auth-guard';

interface AgentFormData {
  name: string;
  displayName: string;
  description: string;
  agentType: AgentType;
  version: string;
  repositoryUrl: string;
  documentationUrl: string;
}

// Agent type options organized by category
const AGENT_TYPE_OPTIONS: { value: AgentType; label: string; description: string; category: string }[] = [
  // LLM Providers
  { value: "claude", label: "Claude", description: "Anthropic Claude models", category: "LLM Providers" },
  { value: "gpt", label: "GPT", description: "OpenAI GPT models", category: "LLM Providers" },
  { value: "gemini", label: "Gemini", description: "Google Gemini models", category: "LLM Providers" },
  { value: "llama", label: "Llama", description: "Meta Llama models", category: "LLM Providers" },
  { value: "mistral", label: "Mistral", description: "Mistral AI models", category: "LLM Providers" },
  { value: "cohere", label: "Cohere", description: "Cohere models", category: "LLM Providers" },
  // Frameworks
  { value: "langchain", label: "LangChain", description: "LangChain framework agents", category: "Frameworks" },
  { value: "llamaindex", label: "LlamaIndex", description: "LlamaIndex agents", category: "Frameworks" },
  { value: "langgraph", label: "LangGraph", description: "LangGraph workflow agents", category: "Frameworks" },
  { value: "crewai", label: "CrewAI", description: "CrewAI multi-agent systems", category: "Frameworks" },
  { value: "autogen", label: "AutoGen", description: "Microsoft AutoGen agents", category: "Frameworks" },
  { value: "semantic_kernel", label: "Semantic Kernel", description: "Microsoft Semantic Kernel", category: "Frameworks" },
  { value: "haystack", label: "Haystack", description: "Haystack pipeline agents", category: "Frameworks" },
  // Copilots & Assistants
  { value: "copilot", label: "Copilot", description: "GitHub Copilot, Microsoft Copilot, etc.", category: "Copilots & Assistants" },
  { value: "assistant", label: "Assistant", description: "OpenAI Assistants API, custom assistants", category: "Copilots & Assistants" },
  { value: "chatbot", label: "Chatbot", description: "Conversational chatbots", category: "Copilots & Assistants" },
  // Autonomous Agents
  { value: "autogpt", label: "AutoGPT", description: "AutoGPT autonomous agent", category: "Autonomous Agents" },
  { value: "babyagi", label: "BabyAGI", description: "BabyAGI task-driven agent", category: "Autonomous Agents" },
  // Other
  { value: "custom", label: "Custom", description: "Custom or other agent type", category: "Other" },
];

// Get unique categories in order
const AGENT_TYPE_CATEGORIES = [...new Set(AGENT_TYPE_OPTIONS.map(o => o.category))];

export default function NewAgentPage() {
  const router = useRouter();
  const [formData, setFormData] = useState<AgentFormData>({
    name: '',
    displayName: '',
    description: '',
    agentType: 'claude',
    version: '',
    repositoryUrl: '',
    documentationUrl: '',
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
          Register an AI agent for identity verification and capability management
        </p>
      </div>

      <form onSubmit={handleSubmit}>
        <Card>
          <CardHeader>
            <CardTitle>Agent Information</CardTitle>
            <CardDescription>
              Provide details about your AI agent
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            {/* Agent Type */}
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Agent Type *
              </label>
              <select
                required
                value={formData.agentType}
                onChange={(e) => setFormData({ ...formData, agentType: e.target.value as AgentType })}
                className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-600 focus:border-transparent bg-white"
              >
                {AGENT_TYPE_CATEGORIES.map((category) => (
                  <optgroup key={category} label={category}>
                    {AGENT_TYPE_OPTIONS.filter(opt => opt.category === category).map((option) => (
                      <option key={option.value} value={option.value}>
                        {option.label} - {option.description}
                      </option>
                    ))}
                  </optgroup>
                ))}
              </select>
              <p className="mt-1 text-sm text-gray-500">
                Select the type of AI agent or framework you are registering
              </p>
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

'use client';

import { useState, useEffect } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
  Code,
  Copy,
  CheckCircle,
  Search,
  ChevronRight,
  ChevronDown,
  Play,
  Key,
  LogIn,
  AlertCircle,
  Lock,
  Unlock,
  BookOpen,
  Braces,
  FileJson,
  Zap
} from 'lucide-react';
import { toast } from 'sonner';

interface APIEndpoint {
  method: 'GET' | 'POST' | 'PUT' | 'DELETE';
  path: string;
  description: string;
  summary: string;
  auth: string;
  requestSchema?: {
    type: string;
    properties: Record<string, { type: string; description: string; required?: boolean }>;
  };
  responseSchema?: {
    type: string;
    properties: Record<string, { type: string; description: string }>;
  };
  example: string;
  requiresAuth: boolean;
  tags: string[];
}

interface EndpointCategory {
  category: string;
  description: string;
  icon: string;
  endpoints: APIEndpoint[];
}

export default function DevelopersPage() {
  const [searchTerm, setSearchTerm] = useState('');
  const [selectedCategory, setSelectedCategory] = useState<string>('Authentication & Authorization');
  const [selectedEndpoint, setSelectedEndpoint] = useState<APIEndpoint | null>(null);
  const [copiedCode, setCopiedCode] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState('overview');

  // API Playground state
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [userToken, setUserToken] = useState<string>('');
  const [manualToken, setManualToken] = useState<string>('');
  const [showTokenInput, setShowTokenInput] = useState(false);
  const [requestBody, setRequestBody] = useState<string>('{}');
  const [responseData, setResponseData] = useState<any>(null);
  const [isExecuting, setIsExecuting] = useState(false);
  const [executionError, setExecutionError] = useState<string | null>(null);

  // Check authentication status on mount
  useEffect(() => {
    const token = localStorage.getItem('auth_token');
    if (token) {
      setIsAuthenticated(true);
      setUserToken(token);
    }
  }, []);

  // Auto-select first endpoint when category changes
  useEffect(() => {
    const category = apiEndpoints.find(c => c.category === selectedCategory);
    if (category && category.endpoints.length > 0) {
      setSelectedEndpoint(category.endpoints[0]);
      setResponseData(null);
      setExecutionError(null);
    }
  }, [selectedCategory]);

  const apiEndpoints: EndpointCategory[] = [
    {
      category: 'Authentication & Authorization',
      description: 'OAuth 2.0, JWT tokens, and user authentication',
      icon: '🔐',
      endpoints: [
        {
          method: 'GET',
          path: '/api/v1/oauth/google/login',
          summary: 'Initiate Google OAuth login flow',
          description: 'Redirects user to Google OAuth consent page for authentication',
          auth: 'None (public)',
          responseSchema: {
            type: 'object',
            properties: {
              redirect_url: { type: 'string', description: 'Google OAuth consent page URL' }
            }
          },
          example: `curl -X GET https://api.aim.com/api/v1/oauth/google/login`,
          requiresAuth: false,
          tags: ['oauth', 'authentication']
        },
        {
          method: 'GET',
          path: '/api/v1/auth/me',
          summary: 'Get current authenticated user',
          description: 'Returns the profile of the currently authenticated user including role and organization',
          auth: 'JWT (all roles)',
          responseSchema: {
            type: 'object',
            properties: {
              id: { type: 'string', description: 'User UUID' },
              email: { type: 'string', description: 'User email address' },
              role: { type: 'string', description: 'User role (admin|manager|member|viewer)' },
              organization_id: { type: 'string', description: 'Organization UUID' },
              created_at: { type: 'string', description: 'ISO 8601 timestamp' }
            }
          },
          example: `curl -X GET https://api.aim.com/api/v1/auth/me \\
  -H "Authorization: Bearer YOUR_JWT_TOKEN"`,
          requiresAuth: true,
          tags: ['authentication', 'user']
        },
        {
          method: 'POST',
          path: '/api/v1/auth/logout',
          summary: 'Logout user',
          description: 'Invalidates the current JWT token and logs out the user',
          auth: 'JWT (all roles)',
          example: `curl -X POST https://api.aim.com/api/v1/auth/logout \\
  -H "Authorization: Bearer YOUR_JWT_TOKEN"`,
          requiresAuth: true,
          tags: ['authentication', 'logout']
        }
      ]
    },
    {
      category: 'Agent Lifecycle Management',
      description: 'Create, manage, and control AI agent identities',
      icon: '🤖',
      endpoints: [
        {
          method: 'POST',
          path: '/api/v1/agents',
          summary: 'Register new agent',
          description: 'Creates a new agent with automatically generated Ed25519 cryptographic keypair. Returns API key (shown only once!).',
          auth: 'JWT (member+)',
          requestSchema: {
            type: 'object',
            properties: {
              name: { type: 'string', description: 'Agent display name', required: true },
              type: { type: 'string', description: 'Agent type (ai_agent|mcp_server|autonomous_agent)', required: true },
              description: { type: 'string', description: 'Optional agent description' }
            }
          },
          responseSchema: {
            type: 'object',
            properties: {
              agent: { type: 'object', description: 'Created agent with credentials' },
              api_key: { type: 'string', description: 'API key (shown only once!)' }
            }
          },
          example: `curl -X POST https://api.aim.com/api/v1/agents \\
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{"name":"MyAgent","type":"ai_agent","description":"My AI assistant"}'`,
          requiresAuth: true,
          tags: ['agents', 'create']
        },
        {
          method: 'GET',
          path: '/api/v1/agents',
          summary: 'List all agents',
          description: 'Returns a paginated list of all agents in your organization with their trust scores and verification status',
          auth: 'JWT (member+)',
          responseSchema: {
            type: 'object',
            properties: {
              agents: { type: 'array', description: 'Array of agent objects' },
              total: { type: 'number', description: 'Total agent count' },
              page: { type: 'number', description: 'Current page number' },
              limit: { type: 'number', description: 'Results per page' }
            }
          },
          example: `curl -X GET https://api.aim.com/api/v1/agents \\
  -H "Authorization: Bearer YOUR_JWT_TOKEN"`,
          requiresAuth: true,
          tags: ['agents', 'list']
        },
        {
          method: 'GET',
          path: '/api/v1/agents/:id',
          summary: 'Get agent details',
          description: 'Retrieves detailed information about a specific agent including public key, trust score, and verification history',
          auth: 'JWT (member+)',
          responseSchema: {
            type: 'object',
            properties: {
              id: { type: 'string', description: 'Agent UUID' },
              name: { type: 'string', description: 'Agent name' },
              type: { type: 'string', description: 'Agent type' },
              public_key: { type: 'string', description: 'Ed25519 public key' },
              trust_score: { type: 'number', description: 'Trust score (0-100)' },
              is_verified: { type: 'boolean', description: 'Verification status' },
              created_at: { type: 'string', description: 'ISO 8601 timestamp' }
            }
          },
          example: `curl -X GET https://api.aim.com/api/v1/agents/AGENT_ID \\
  -H "Authorization: Bearer YOUR_JWT_TOKEN"`,
          requiresAuth: true,
          tags: ['agents', 'details']
        },
        {
          method: 'PUT',
          path: '/api/v1/agents/:id',
          summary: 'Update agent metadata',
          description: 'Updates agent name, description, or custom metadata. Cryptographic keys cannot be changed.',
          auth: 'JWT (member+)',
          requestSchema: {
            type: 'object',
            properties: {
              name: { type: 'string', description: 'New agent name' },
              description: { type: 'string', description: 'New description' },
              metadata: { type: 'object', description: 'Custom metadata (JSON)' }
            }
          },
          example: `curl -X PUT https://api.aim.com/api/v1/agents/AGENT_ID \\
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{"name":"Updated Agent Name","description":"New description"}'`,
          requiresAuth: true,
          tags: ['agents', 'update']
        },
        {
          method: 'POST',
          path: '/api/v1/agents/:id/suspend',
          summary: 'Suspend agent',
          description: 'Immediately suspends an agent and revokes all active sessions. Can be reactivated later.',
          auth: 'JWT (manager+)',
          example: `curl -X POST https://api.aim.com/api/v1/agents/AGENT_ID/suspend \\
  -H "Authorization: Bearer YOUR_JWT_TOKEN"`,
          requiresAuth: true,
          tags: ['agents', 'suspend']
        },
        {
          method: 'POST',
          path: '/api/v1/agents/:id/reactivate',
          summary: 'Reactivate suspended agent',
          description: 'Reactivates a previously suspended agent and restores access',
          auth: 'JWT (manager+)',
          example: `curl -X POST https://api.aim.com/api/v1/agents/AGENT_ID/reactivate \\
  -H "Authorization: Bearer YOUR_JWT_TOKEN"`,
          requiresAuth: true,
          tags: ['agents', 'reactivate']
        }
      ]
    },
    {
      category: 'Compliance & Audit',
      description: 'Compliance monitoring, audit trails, and regulatory reporting',
      icon: '📋',
      endpoints: [
        {
          method: 'GET',
          path: '/api/v1/compliance/status',
          summary: 'Get compliance status',
          description: 'Returns overall compliance health including SOC 2, HIPAA, and GDPR status',
          auth: 'JWT (admin)',
          responseSchema: {
            type: 'object',
            properties: {
              overall_status: { type: 'string', description: 'compliant|warning|critical' },
              last_check: { type: 'string', description: 'ISO 8601 timestamp' },
              violations: { type: 'array', description: 'Active compliance violations' }
            }
          },
          example: `curl -X GET https://api.aim.com/api/v1/compliance/status \\
  -H "Authorization: Bearer YOUR_JWT_TOKEN"`,
          requiresAuth: true,
          tags: ['compliance', 'status']
        },
        {
          method: 'GET',
          path: '/api/v1/compliance/metrics',
          summary: 'Get compliance metrics',
          description: 'Detailed compliance metrics across SOC 2, HIPAA, and GDPR frameworks',
          auth: 'JWT (admin)',
          responseSchema: {
            type: 'object',
            properties: {
              soc2: { type: 'object', description: 'SOC 2 compliance metrics' },
              hipaa: { type: 'object', description: 'HIPAA compliance metrics' },
              gdpr: { type: 'object', description: 'GDPR compliance metrics' }
            }
          },
          example: `curl -X GET https://api.aim.com/api/v1/compliance/metrics \\
  -H "Authorization: Bearer YOUR_JWT_TOKEN"`,
          requiresAuth: true,
          tags: ['compliance', 'metrics']
        }
      ]
    },
    {
      category: 'Security & Alerts',
      description: 'Security monitoring, threat detection, and alert management',
      icon: '🛡️',
      endpoints: [
        {
          method: 'GET',
          path: '/api/v1/admin/alerts',
          summary: 'List security alerts',
          description: 'Returns all security alerts with severity levels and acknowledgment status',
          auth: 'JWT (admin)',
          responseSchema: {
            type: 'object',
            properties: {
              alerts: { type: 'array', description: 'Array of security alert objects' },
              total: { type: 'number', description: 'Total alert count' }
            }
          },
          example: `curl -X GET https://api.aim.com/api/v1/admin/alerts \\
  -H "Authorization: Bearer YOUR_JWT_TOKEN"`,
          requiresAuth: true,
          tags: ['security', 'alerts']
        },
        {
          method: 'POST',
          path: '/api/v1/admin/alerts/:id/acknowledge',
          summary: 'Acknowledge alert',
          description: 'Marks a security alert as acknowledged by the current admin user',
          auth: 'JWT (admin)',
          example: `curl -X POST https://api.aim.com/api/v1/admin/alerts/ALERT_ID/acknowledge \\
  -H "Authorization: Bearer YOUR_JWT_TOKEN"`,
          requiresAuth: true,
          tags: ['security', 'alerts']
        }
      ]
    },
    {
      category: 'Analytics & Reporting',
      description: 'Usage statistics, trust trends, and activity monitoring',
      icon: '📊',
      endpoints: [
        {
          method: 'GET',
          path: '/api/v1/analytics/dashboard',
          summary: 'Get dashboard statistics',
          description: 'Returns comprehensive dashboard metrics including agent counts, trust scores, and verification statistics',
          auth: 'JWT (member+)',
          responseSchema: {
            type: 'object',
            properties: {
              total_agents: { type: 'number', description: 'Total agent count' },
              verified_agents: { type: 'number', description: 'Verified agents count' },
              avg_trust_score: { type: 'number', description: 'Average trust score (0-100)' },
              total_verifications: { type: 'number', description: 'Total verification count' },
              total_users: { type: 'number', description: 'Total user count' }
            }
          },
          example: `curl -X GET https://api.aim.com/api/v1/analytics/dashboard \\
  -H "Authorization: Bearer YOUR_JWT_TOKEN"`,
          requiresAuth: true,
          tags: ['analytics', 'dashboard']
        },
        {
          method: 'GET',
          path: '/api/v1/analytics/usage',
          summary: 'Get usage statistics',
          description: 'API usage metrics including request counts, active agents, and bandwidth consumption',
          auth: 'JWT (manager+)',
          responseSchema: {
            type: 'object',
            properties: {
              api_calls: { type: 'number', description: 'Total API calls (last 30 days)' },
              active_agents: { type: 'number', description: 'Currently active agents' },
              bandwidth: { type: 'number', description: 'Bandwidth used (bytes)' }
            }
          },
          example: `curl -X GET https://api.aim.com/api/v1/analytics/usage \\
  -H "Authorization: Bearer YOUR_JWT_TOKEN"`,
          requiresAuth: true,
          tags: ['analytics', 'usage']
        }
      ]
    },
    {
      category: 'SDK & Integration',
      description: 'Client SDKs, webhooks, and third-party integrations',
      icon: '🔌',
      endpoints: [
        {
          method: 'GET',
          path: '/api/v1/sdk/python',
          summary: 'Download Python SDK',
          description: 'Returns download URL for the latest Python SDK package',
          auth: 'None (public)',
          responseSchema: {
            type: 'object',
            properties: {
              download_url: { type: 'string', description: 'Python SDK download URL' },
              version: { type: 'string', description: 'SDK version (semver)' },
              docs_url: { type: 'string', description: 'Documentation URL' }
            }
          },
          example: `curl -X GET https://api.aim.com/api/v1/sdk/python`,
          requiresAuth: false,
          tags: ['sdk', 'python']
        },
        {
          method: 'POST',
          path: '/api/v1/webhooks',
          summary: 'Register webhook',
          description: 'Creates a new webhook subscription for event notifications (agent.created, agent.suspended, etc.)',
          auth: 'JWT (manager+)',
          requestSchema: {
            type: 'object',
            properties: {
              url: { type: 'string', description: 'Webhook endpoint URL', required: true },
              events: { type: 'array', description: 'Event types to subscribe to', required: true },
              secret: { type: 'string', description: 'Webhook signing secret (optional)' }
            }
          },
          example: `curl -X POST https://api.aim.com/api/v1/webhooks \\
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{"url":"https://example.com/webhook","events":["agent.created","agent.suspended"]}'`,
          requiresAuth: true,
          tags: ['webhooks', 'integration']
        }
      ]
    }
  ];

  const filteredEndpoints = apiEndpoints
    .map(category => ({
      ...category,
      endpoints: category.endpoints.filter(endpoint => {
        const matchesSearch = searchTerm === '' ||
          endpoint.path.toLowerCase().includes(searchTerm.toLowerCase()) ||
          endpoint.description.toLowerCase().includes(searchTerm.toLowerCase()) ||
          endpoint.summary.toLowerCase().includes(searchTerm.toLowerCase());
        return matchesSearch;
      })
    }))
    .filter(category => category.endpoints.length > 0);

  const copyToClipboard = (text: string, label: string) => {
    navigator.clipboard.writeText(text);
    setCopiedCode(label);
    toast.success('Copied to clipboard!');
    setTimeout(() => setCopiedCode(null), 2000);
  };

  const getMethodColor = (method: string) => {
    switch (method) {
      case 'GET': return 'bg-blue-500 hover:bg-blue-600';
      case 'POST': return 'bg-green-500 hover:bg-green-600';
      case 'PUT': return 'bg-orange-500 hover:bg-orange-600';
      case 'DELETE': return 'bg-red-500 hover:bg-red-600';
      default: return 'bg-gray-500 hover:bg-gray-600';
    }
  };

  const executeAPIRequest = async (endpoint: APIEndpoint) => {
    setIsExecuting(true);
    setExecutionError(null);
    setResponseData(null);

    try {
      const token = isAuthenticated ? userToken : manualToken;

      if (endpoint.requiresAuth && !token) {
        setExecutionError('Authentication required. Please login or provide a JWT token.');
        setIsExecuting(false);
        return;
      }

      const headers: HeadersInit = {
        'Content-Type': 'application/json',
      };

      if (token) {
        headers['Authorization'] = `Bearer ${token}`;
      }

      const fetchOptions: RequestInit = {
        method: endpoint.method,
        headers,
      };

      if ((endpoint.method === 'POST' || endpoint.method === 'PUT') && requestBody) {
        try {
          JSON.parse(requestBody);
          fetchOptions.body = requestBody;
        } catch (err) {
          setExecutionError('Invalid JSON in request body');
          setIsExecuting(false);
          return;
        }
      }

      const response = await fetch(`http://localhost:8080${endpoint.path}`, fetchOptions);

      const contentType = response.headers.get('content-type');
      let data;

      if (contentType && contentType.includes('application/json')) {
        data = await response.json();
      } else {
        data = { message: await response.text() };
      }

      if (!response.ok) {
        setExecutionError(`HTTP ${response.status}: ${data.error || data.message || 'Request failed'}`);
      } else {
        setResponseData(data);
        toast.success('Request executed successfully!');
      }
    } catch (error) {
      setExecutionError(error instanceof Error ? error.message : 'Network error');
    } finally {
      setIsExecuting(false);
    }
  };

  return (
    <div className="flex h-screen bg-gray-50">
      {/* Left Sidebar - Swagger style navigation */}
      <div className="w-80 bg-white border-r border-gray-200 overflow-y-auto">
        <div className="p-6 border-b border-gray-200">
          <div className="flex items-center justify-between mb-4">
            <div>
              <h1 className="text-2xl font-bold text-gray-900">AIM API v1.0</h1>
              <p className="text-sm text-gray-500 mt-1">OpenAPI 3.0 Specification</p>
            </div>
            {isAuthenticated ? (
              <Badge variant="default" className="flex items-center gap-1.5 bg-green-500 hover:bg-green-600">
                <Unlock className="h-3 w-3" />
                Auth
              </Badge>
            ) : (
              <Button
                variant="outline"
                size="sm"
                onClick={() => window.location.href = '/auth/login'}
              >
                <LogIn className="mr-1.5 h-3.5 w-3.5" />
                Login
              </Button>
            )}
          </div>

          {/* Search */}
          <div className="relative">
            <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-gray-400" />
            <Input
              placeholder="Search endpoints..."
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              className="pl-10 bg-gray-50 border-gray-200"
            />
          </div>
        </div>

        {/* Endpoint Categories */}
        <div className="p-4 space-y-2">
          {filteredEndpoints.map((category) => (
            <div key={category.category}>
              <button
                onClick={() => setSelectedCategory(category.category)}
                className={`w-full text-left px-3 py-2 rounded-lg transition-colors ${
                  selectedCategory === category.category
                    ? 'bg-blue-50 text-blue-700 font-medium'
                    : 'text-gray-700 hover:bg-gray-50'
                }`}
              >
                <div className="flex items-center gap-2">
                  <span className="text-lg">{category.icon}</span>
                  <span className="text-sm font-medium">{category.category}</span>
                  <Badge variant="outline" className="ml-auto text-xs">
                    {category.endpoints.length}
                  </Badge>
                </div>
              </button>

              {/* Endpoints in this category */}
              {selectedCategory === category.category && (
                <div className="ml-4 mt-2 space-y-1">
                  {category.endpoints.map((endpoint) => (
                    <button
                      key={endpoint.path}
                      onClick={() => {
                        setSelectedEndpoint(endpoint);
                        setResponseData(null);
                        setExecutionError(null);
                        setActiveTab('overview');
                      }}
                      className={`w-full text-left px-3 py-2 rounded-md transition-colors group ${
                        selectedEndpoint?.path === endpoint.path
                          ? 'bg-blue-100 border border-blue-300'
                          : 'hover:bg-gray-100'
                      }`}
                    >
                      <div className="flex items-start gap-2">
                        <Badge
                          className={`${getMethodColor(endpoint.method)} text-white text-xs px-1.5 py-0 mt-0.5`}
                        >
                          {endpoint.method}
                        </Badge>
                        <div className="flex-1 min-w-0">
                          <p className="text-xs font-mono text-gray-700 truncate">
                            {endpoint.path}
                          </p>
                          <p className="text-xs text-gray-500 mt-0.5">
                            {endpoint.summary}
                          </p>
                        </div>
                        {endpoint.requiresAuth && (
                          <Lock className="h-3 w-3 text-amber-500 flex-shrink-0 mt-1" />
                        )}
                      </div>
                    </button>
                  ))}
                </div>
              )}
            </div>
          ))}
        </div>
      </div>

      {/* Main Content Area - Swagger style details */}
      <div className="flex-1 overflow-y-auto">
        {selectedEndpoint ? (
          <div className="max-w-5xl mx-auto p-8">
            {/* Endpoint Header */}
            <div className="mb-6">
              <div className="flex items-center gap-3 mb-3">
                <Badge className={`${getMethodColor(selectedEndpoint.method)} text-white px-3 py-1`}>
                  {selectedEndpoint.method}
                </Badge>
                <code className="text-lg font-mono font-semibold text-gray-900">
                  {selectedEndpoint.path}
                </code>
                {selectedEndpoint.requiresAuth && (
                  <div className="flex items-center gap-1.5 text-amber-600 text-sm">
                    <Lock className="h-4 w-4" />
                    <span className="font-medium">{selectedEndpoint.auth}</span>
                  </div>
                )}
              </div>
              <h2 className="text-2xl font-bold text-gray-900 mb-2">
                {selectedEndpoint.summary}
              </h2>
              <p className="text-gray-600 leading-relaxed">
                {selectedEndpoint.description}
              </p>
              <div className="flex gap-2 mt-3">
                {selectedEndpoint.tags.map(tag => (
                  <Badge key={tag} variant="outline" className="text-xs">
                    {tag}
                  </Badge>
                ))}
              </div>
            </div>

            {/* Tabbed Interface */}
            <Tabs value={activeTab} onValueChange={setActiveTab} className="space-y-4">
              <TabsList className="bg-white border border-gray-200">
                <TabsTrigger value="overview" className="data-[state=active]:bg-blue-50 data-[state=active]:text-blue-700">
                  <BookOpen className="h-4 w-4 mr-2" />
                  Overview
                </TabsTrigger>
                {selectedEndpoint.requestSchema && (
                  <TabsTrigger value="request" className="data-[state=active]:bg-blue-50 data-[state=active]:text-blue-700">
                    <Braces className="h-4 w-4 mr-2" />
                    Request
                  </TabsTrigger>
                )}
                {selectedEndpoint.responseSchema && (
                  <TabsTrigger value="response" className="data-[state=active]:bg-blue-50 data-[state=active]:text-blue-700">
                    <FileJson className="h-4 w-4 mr-2" />
                    Response
                  </TabsTrigger>
                )}
                <TabsTrigger value="try-it" className="data-[state=active]:bg-blue-50 data-[state=active]:text-blue-700">
                  <Zap className="h-4 w-4 mr-2" />
                  Try it out
                </TabsTrigger>
              </TabsList>

              {/* Overview Tab */}
              <TabsContent value="overview" className="space-y-4">
                <Card>
                  <CardHeader>
                    <CardTitle className="text-lg">cURL Example</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <div className="relative">
                      <pre className="bg-gray-900 text-gray-100 p-4 rounded-lg overflow-x-auto text-sm font-mono">
                        <code>{selectedEndpoint.example}</code>
                      </pre>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => copyToClipboard(selectedEndpoint.example, `example-${selectedEndpoint.path}`)}
                        className="absolute top-2 right-2 bg-gray-800 hover:bg-gray-700"
                      >
                        {copiedCode === `example-${selectedEndpoint.path}` ? (
                          <CheckCircle className="h-4 w-4 text-green-400" />
                        ) : (
                          <Copy className="h-4 w-4 text-gray-300" />
                        )}
                      </Button>
                    </div>
                  </CardContent>
                </Card>
              </TabsContent>

              {/* Request Schema Tab */}
              {selectedEndpoint.requestSchema && (
                <TabsContent value="request" className="space-y-4">
                  <Card>
                    <CardHeader>
                      <CardTitle className="text-lg">Request Body Schema</CardTitle>
                      <CardDescription>
                        Content-Type: application/json
                      </CardDescription>
                    </CardHeader>
                    <CardContent>
                      <div className="bg-gray-50 rounded-lg p-4 border border-gray-200">
                        <div className="space-y-3">
                          {Object.entries(selectedEndpoint.requestSchema.properties).map(([key, prop]) => (
                            <div key={key} className="flex items-start gap-3 pb-3 border-b border-gray-200 last:border-0">
                              <div className="flex-1">
                                <div className="flex items-center gap-2">
                                  <code className="text-sm font-semibold text-blue-600">{key}</code>
                                  <Badge variant="outline" className="text-xs">{prop.type}</Badge>
                                  {prop.required && (
                                    <Badge variant="destructive" className="text-xs">required</Badge>
                                  )}
                                </div>
                                <p className="text-sm text-gray-600 mt-1">{prop.description}</p>
                              </div>
                            </div>
                          ))}
                        </div>
                      </div>
                    </CardContent>
                  </Card>
                </TabsContent>
              )}

              {/* Response Schema Tab */}
              {selectedEndpoint.responseSchema && (
                <TabsContent value="response" className="space-y-4">
                  <Card>
                    <CardHeader>
                      <CardTitle className="text-lg">Response Schema (200 OK)</CardTitle>
                      <CardDescription>
                        Content-Type: application/json
                      </CardDescription>
                    </CardHeader>
                    <CardContent>
                      <div className="bg-gray-50 rounded-lg p-4 border border-gray-200">
                        <div className="space-y-3">
                          {Object.entries(selectedEndpoint.responseSchema.properties).map(([key, prop]) => (
                            <div key={key} className="flex items-start gap-3 pb-3 border-b border-gray-200 last:border-0">
                              <div className="flex-1">
                                <div className="flex items-center gap-2">
                                  <code className="text-sm font-semibold text-green-600">{key}</code>
                                  <Badge variant="outline" className="text-xs">{prop.type}</Badge>
                                </div>
                                <p className="text-sm text-gray-600 mt-1">{prop.description}</p>
                              </div>
                            </div>
                          ))}
                        </div>
                      </div>
                    </CardContent>
                  </Card>
                </TabsContent>
              )}

              {/* Try it out Tab */}
              <TabsContent value="try-it" className="space-y-4">
                <Card className="border-2 border-blue-200 bg-blue-50">
                  <CardHeader>
                    <div className="flex items-center justify-between">
                      <CardTitle className="text-lg flex items-center gap-2">
                        <Play className="h-5 w-5 text-blue-600" />
                        Interactive API Console
                      </CardTitle>
                      {!isAuthenticated && selectedEndpoint.requiresAuth && (
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => setShowTokenInput(!showTokenInput)}
                        >
                          <Key className="mr-2 h-3.5 w-3.5" />
                          {showTokenInput ? 'Hide Token' : 'Add Token'}
                        </Button>
                      )}
                    </div>
                  </CardHeader>
                  <CardContent className="space-y-4">
                    {/* Token Input for Unauthenticated Users */}
                    {!isAuthenticated && selectedEndpoint.requiresAuth && showTokenInput && (
                      <div className="space-y-2">
                        <label className="text-sm font-semibold text-gray-700">Authorization Token:</label>
                        <Input
                          placeholder="Paste your JWT token here..."
                          value={manualToken}
                          onChange={(e) => setManualToken(e.target.value)}
                          type="password"
                          className="bg-white"
                        />
                      </div>
                    )}

                    {/* Request Body Editor (for POST/PUT) */}
                    {(selectedEndpoint.method === 'POST' || selectedEndpoint.method === 'PUT') && (
                      <div className="space-y-2">
                        <label className="text-sm font-semibold text-gray-700">Request Body:</label>
                        <Textarea
                          placeholder='{"key": "value"}'
                          value={requestBody}
                          onChange={(e) => setRequestBody(e.target.value)}
                          className="font-mono text-sm bg-white min-h-[150px]"
                        />
                      </div>
                    )}

                    {/* Execute Button */}
                    <Button
                      onClick={() => executeAPIRequest(selectedEndpoint)}
                      disabled={isExecuting}
                      className="w-full bg-blue-600 hover:bg-blue-700"
                      size="lg"
                    >
                      {isExecuting ? (
                        <>Executing...</>
                      ) : (
                        <>
                          <Play className="mr-2 h-4 w-4" />
                          Execute Request
                        </>
                      )}
                    </Button>

                    {/* Response Display */}
                    {(responseData || executionError) && (
                      <div className="space-y-2 mt-4">
                        <div className="flex items-center justify-between">
                          <label className="text-sm font-semibold text-gray-700">Response:</label>
                          {responseData && (
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => copyToClipboard(
                                JSON.stringify(responseData, null, 2),
                                `response-${selectedEndpoint.path}`
                              )}
                            >
                              {copiedCode === `response-${selectedEndpoint.path}` ? (
                                <CheckCircle className="h-4 w-4 text-green-500" />
                              ) : (
                                <Copy className="h-4 w-4" />
                              )}
                            </Button>
                          )}
                        </div>

                        {executionError ? (
                          <div className="bg-red-50 border-2 border-red-200 rounded-lg p-4">
                            <div className="flex items-start gap-3">
                              <AlertCircle className="h-5 w-5 text-red-600 flex-shrink-0 mt-0.5" />
                              <div>
                                <p className="text-sm font-semibold text-red-900">Error</p>
                                <p className="text-sm text-red-700 mt-1">{executionError}</p>
                              </div>
                            </div>
                          </div>
                        ) : (
                          <pre className="bg-green-50 border-2 border-green-200 rounded-lg p-4 overflow-x-auto text-sm font-mono">
                            <code className="text-green-900">
                              {JSON.stringify(responseData, null, 2)}
                            </code>
                          </pre>
                        )}
                      </div>
                    )}
                  </CardContent>
                </Card>
              </TabsContent>
            </Tabs>
          </div>
        ) : (
          <div className="flex items-center justify-center h-full">
            <div className="text-center text-gray-500">
              <Code className="h-16 w-16 mx-auto mb-4 text-gray-300" />
              <p className="text-lg font-medium">Select an endpoint to view details</p>
              <p className="text-sm mt-2">Choose from the sidebar to explore our API</p>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

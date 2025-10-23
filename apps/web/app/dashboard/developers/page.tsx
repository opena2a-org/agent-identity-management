'use client';

import { useState, useEffect } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import {
  Code,
  Copy,
  CheckCircle,
  Search,
  Filter,
  ChevronDown,
  ChevronUp,
  Play,
  Key,
  LogIn,
  AlertCircle,
  Lock,
  Unlock
} from 'lucide-react';
import { toast } from 'sonner';

interface APIEndpoint {
  method: 'GET' | 'POST' | 'PUT' | 'DELETE';
  path: string;
  description: string;
  auth: string;
  request?: Record<string, string>;
  response?: Record<string, string>;
  example: string;
  requiresAuth: boolean;
}

interface EndpointCategory {
  category: string;
  description: string;
  endpoints: APIEndpoint[];
}

export default function DevelopersPage() {
  const [searchTerm, setSearchTerm] = useState('');
  const [selectedCategory, setSelectedCategory] = useState<string | null>(null);
  const [expandedEndpoints, setExpandedEndpoints] = useState<Set<string>>(new Set());
  const [copiedCode, setCopiedCode] = useState<string | null>(null);

  // API Playground state
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [userToken, setUserToken] = useState<string>('');
  const [manualToken, setManualToken] = useState<string>('');
  const [showTokenInput, setShowTokenInput] = useState(false);
  const [playgroundEndpoint, setPlaygroundEndpoint] = useState<string | null>(null);
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

  const apiEndpoints: EndpointCategory[] = [
    {
      category: 'Authentication & Authorization',
      description: 'OAuth 2.0, JWT tokens, and user authentication',
      endpoints: [
        {
          method: 'GET',
          path: '/api/v1/oauth/google/login',
          description: 'Initiate Google OAuth login flow',
          auth: 'None (public)',
          response: {
            redirect_url: 'string - Google OAuth consent page URL'
          },
          example: `curl -X GET https://api.aim.com/api/v1/oauth/google/login`,
          requiresAuth: false
        },
        {
          method: 'GET',
          path: '/api/v1/auth/me',
          description: 'Get current authenticated user profile',
          auth: 'JWT (all roles)',
          response: {
            id: 'string - User ID',
            email: 'string - User email',
            role: 'string - User role (admin|manager|member|viewer)',
            organization_id: 'string - Organization ID'
          },
          example: `curl -X GET https://api.aim.com/api/v1/auth/me \\
  -H "Authorization: Bearer YOUR_JWT_TOKEN"`,
          requiresAuth: true
        },
        {
          method: 'POST',
          path: '/api/v1/auth/logout',
          description: 'Logout and invalidate JWT token',
          auth: 'JWT (all roles)',
          example: `curl -X POST https://api.aim.com/api/v1/auth/logout \\
  -H "Authorization: Bearer YOUR_JWT_TOKEN"`,
          requiresAuth: true
        }
      ]
    },
    {
      category: 'Agent Lifecycle Management',
      description: 'Create, manage, and control AI agent identities',
      endpoints: [
        {
          method: 'POST',
          path: '/api/v1/agents',
          description: 'Register a new agent. Automatically generates Ed25519 keypair.',
          auth: 'JWT (member+)',
          request: {
            name: 'string (required) - Agent display name',
            type: 'string (required) - Agent type (ai_agent|mcp_server|autonomous_agent)',
            description: 'string (optional) - Agent description'
          },
          response: {
            agent: 'object - Created agent with credentials',
            api_key: 'string - API key (shown only once!)'
          },
          example: `curl -X POST https://api.aim.com/api/v1/agents \\
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{"name":"MyAgent","type":"ai_agent"}'`,
          requiresAuth: true
        },
        {
          method: 'GET',
          path: '/api/v1/agents',
          description: 'List all agents in your organization',
          auth: 'JWT (member+)',
          response: {
            agents: 'array - List of agent objects',
            total: 'number - Total count'
          },
          example: `curl -X GET https://api.aim.com/api/v1/agents \\
  -H "Authorization: Bearer YOUR_JWT_TOKEN"`,
          requiresAuth: true
        },
        {
          method: 'GET',
          path: '/api/v1/agents/:id',
          description: 'Get detailed information about a specific agent',
          auth: 'JWT (member+)',
          response: {
            id: 'string - Agent ID',
            name: 'string - Agent name',
            public_key: 'string - Ed25519 public key',
            trust_score: 'number - Trust score (0-100)',
            status: 'string - Agent status'
          },
          example: `curl -X GET https://api.aim.com/api/v1/agents/AGENT_ID \\
  -H "Authorization: Bearer YOUR_JWT_TOKEN"`,
          requiresAuth: true
        },
        {
          method: 'PUT',
          path: '/api/v1/agents/:id',
          description: 'Update agent metadata (name, description, metadata)',
          auth: 'JWT (member+)',
          request: {
            name: 'string (optional) - New agent name',
            description: 'string (optional) - New description',
            metadata: 'object (optional) - Custom metadata'
          },
          example: `curl -X PUT https://api.aim.com/api/v1/agents/AGENT_ID \\
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{"name":"Updated Agent Name"}'`,
          requiresAuth: true
        },
        {
          method: 'POST',
          path: '/api/v1/agents/:id/suspend',
          description: 'Suspend an agent (revokes access immediately)',
          auth: 'JWT (manager+)',
          example: `curl -X POST https://api.aim.com/api/v1/agents/AGENT_ID/suspend \\
  -H "Authorization: Bearer YOUR_JWT_TOKEN"`,
          requiresAuth: true
        },
        {
          method: 'POST',
          path: '/api/v1/agents/:id/reactivate',
          description: 'Reactivate a suspended agent',
          auth: 'JWT (manager+)',
          example: `curl -X POST https://api.aim.com/api/v1/agents/AGENT_ID/reactivate \\
  -H "Authorization: Bearer YOUR_JWT_TOKEN"`,
          requiresAuth: true
        }
      ]
    },
    {
      category: 'Compliance & Audit',
      description: 'Compliance monitoring, audit trails, and regulatory reporting',
      endpoints: [
        {
          method: 'GET',
          path: '/api/v1/compliance/status',
          description: 'Get overall compliance status and health',
          auth: 'JWT (admin)',
          response: {
            overall_status: 'string - compliant|warning|critical',
            last_check: 'string - ISO 8601 timestamp',
            violations: 'array - Active violations'
          },
          example: `curl -X GET https://api.aim.com/api/v1/compliance/status \\
  -H "Authorization: Bearer YOUR_JWT_TOKEN"`,
          requiresAuth: true
        },
        {
          method: 'GET',
          path: '/api/v1/compliance/metrics',
          description: 'Get compliance metrics (SOC 2, HIPAA, GDPR)',
          auth: 'JWT (admin)',
          response: {
            soc2: 'object - SOC 2 compliance metrics',
            hipaa: 'object - HIPAA compliance metrics',
            gdpr: 'object - GDPR compliance metrics'
          },
          example: `curl -X GET https://api.aim.com/api/v1/compliance/metrics \\
  -H "Authorization: Bearer YOUR_JWT_TOKEN"`,
          requiresAuth: true
        },
        {
          method: 'GET',
          path: '/api/v1/compliance/access-review',
          description: 'Get access review report (user permissions audit)',
          auth: 'JWT (admin)',
          response: {
            users: 'array - Users with access details',
            last_review: 'string - Last review date'
          },
          example: `curl -X GET https://api.aim.com/api/v1/compliance/access-review \\
  -H "Authorization: Bearer YOUR_JWT_TOKEN"`,
          requiresAuth: true
        },
        {
          method: 'POST',
          path: '/api/v1/compliance/check',
          description: 'Run manual compliance check across all systems',
          auth: 'JWT (admin)',
          response: {
            check_id: 'string - Compliance check ID',
            status: 'string - running|completed',
            findings: 'array - Compliance findings'
          },
          example: `curl -X POST https://api.aim.com/api/v1/compliance/check \\
  -H "Authorization: Bearer YOUR_JWT_TOKEN"`,
          requiresAuth: true
        },
        {
          method: 'GET',
          path: '/api/v1/compliance/export',
          description: 'Export compliance report (PDF/JSON)',
          auth: 'JWT (admin)',
          response: {
            report_url: 'string - Download URL',
            format: 'string - pdf|json',
            expires_at: 'string - URL expiration time'
          },
          example: `curl -X GET https://api.aim.com/api/v1/compliance/export?format=pdf \\
  -H "Authorization: Bearer YOUR_JWT_TOKEN"`,
          requiresAuth: true
        }
      ]
    },
    {
      category: 'Security & Alerts',
      description: 'Security monitoring, threat detection, and alert management',
      endpoints: [
        {
          method: 'GET',
          path: '/api/v1/admin/alerts',
          description: 'List all security alerts',
          auth: 'JWT (admin)',
          response: {
            alerts: 'array - Security alerts',
            total: 'number - Total alert count'
          },
          example: `curl -X GET https://api.aim.com/api/v1/admin/alerts \\
  -H "Authorization: Bearer YOUR_JWT_TOKEN"`,
          requiresAuth: true
        },
        {
          method: 'POST',
          path: '/api/v1/admin/alerts/:id/acknowledge',
          description: 'Acknowledge a security alert',
          auth: 'JWT (admin)',
          example: `curl -X POST https://api.aim.com/api/v1/admin/alerts/ALERT_ID/acknowledge \\
  -H "Authorization: Bearer YOUR_JWT_TOKEN"`,
          requiresAuth: true
        },
        {
          method: 'POST',
          path: '/api/v1/admin/alerts/:id/resolve',
          description: 'Mark security alert as resolved',
          auth: 'JWT (admin)',
          example: `curl -X POST https://api.aim.com/api/v1/admin/alerts/ALERT_ID/resolve \\
  -H "Authorization: Bearer YOUR_JWT_TOKEN"`,
          requiresAuth: true
        },
        {
          method: 'GET',
          path: '/api/v1/admin/security-policies',
          description: 'List all security policies',
          auth: 'JWT (admin)',
          response: {
            policies: 'array - Security policies',
            total: 'number - Total policy count'
          },
          example: `curl -X GET https://api.aim.com/api/v1/admin/security-policies \\
  -H "Authorization: Bearer YOUR_JWT_TOKEN"`,
          requiresAuth: true
        }
      ]
    },
    {
      category: 'Analytics & Reporting',
      description: 'Usage statistics, trust trends, and activity monitoring',
      endpoints: [
        {
          method: 'GET',
          path: '/api/v1/analytics/dashboard',
          description: 'Get dashboard statistics (agents, verifications, trust score)',
          auth: 'JWT (member+)',
          response: {
            total_agents: 'number - Total agent count',
            verified_agents: 'number - Verified agents',
            avg_trust_score: 'number - Average trust score',
            total_verifications: 'number - Verification count'
          },
          example: `curl -X GET https://api.aim.com/api/v1/analytics/dashboard \\
  -H "Authorization: Bearer YOUR_JWT_TOKEN"`,
          requiresAuth: true
        },
        {
          method: 'GET',
          path: '/api/v1/analytics/usage',
          description: 'Get usage statistics (API calls, active agents)',
          auth: 'JWT (manager+)',
          response: {
            api_calls: 'number - Total API calls',
            active_agents: 'number - Active agents',
            bandwidth: 'number - Bandwidth used (bytes)'
          },
          example: `curl -X GET https://api.aim.com/api/v1/analytics/usage \\
  -H "Authorization: Bearer YOUR_JWT_TOKEN"`,
          requiresAuth: true
        },
        {
          method: 'GET',
          path: '/api/v1/analytics/trends',
          description: 'Get trust score trends over time',
          auth: 'JWT (member+)',
          response: {
            trends: 'array - Time-series trust score data',
            period: 'string - Time period (7d|30d|90d)'
          },
          example: `curl -X GET https://api.aim.com/api/v1/analytics/trends?period=30d \\
  -H "Authorization: Bearer YOUR_JWT_TOKEN"`,
          requiresAuth: true
        },
        {
          method: 'GET',
          path: '/api/v1/analytics/verification-activity',
          description: 'Get verification activity history',
          auth: 'JWT (member+)',
          response: {
            verifications: 'array - Verification events',
            total: 'number - Total verification count'
          },
          example: `curl -X GET https://api.aim.com/api/v1/analytics/verification-activity \\
  -H "Authorization: Bearer YOUR_JWT_TOKEN"`,
          requiresAuth: true
        }
      ]
    },
    {
      category: 'SDK & Integration',
      description: 'Client SDKs, webhooks, and third-party integrations',
      endpoints: [
        {
          method: 'GET',
          path: '/api/v1/sdk/python',
          description: 'Download Python SDK package',
          auth: 'None (public)',
          response: {
            download_url: 'string - Python SDK download URL',
            version: 'string - SDK version',
            docs_url: 'string - Documentation URL'
          },
          example: `curl -X GET https://api.aim.com/api/v1/sdk/python`,
          requiresAuth: false
        },
        {
          method: 'GET',
          path: '/api/v1/sdk/typescript',
          description: 'Download TypeScript SDK package',
          auth: 'None (public)',
          response: {
            download_url: 'string - TypeScript SDK download URL',
            version: 'string - SDK version',
            docs_url: 'string - Documentation URL'
          },
          example: `curl -X GET https://api.aim.com/api/v1/sdk/typescript`,
          requiresAuth: false
        },
        {
          method: 'POST',
          path: '/api/v1/webhooks',
          description: 'Register a webhook for event notifications',
          auth: 'JWT (manager+)',
          request: {
            url: 'string (required) - Webhook URL',
            events: 'array (required) - Event types to subscribe to',
            secret: 'string (optional) - Webhook signing secret'
          },
          example: `curl -X POST https://api.aim.com/api/v1/webhooks \\
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{"url":"https://example.com/webhook","events":["agent.created"]}'`,
          requiresAuth: true
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
          endpoint.description.toLowerCase().includes(searchTerm.toLowerCase());
        const matchesCategory = selectedCategory === null || category.category === selectedCategory;
        return matchesSearch && matchesCategory;
      })
    }))
    .filter(category => category.endpoints.length > 0);

  const toggleEndpoint = (path: string) => {
    const newExpanded = new Set(expandedEndpoints);
    if (newExpanded.has(path)) {
      newExpanded.delete(path);
      // Close playground when collapsing endpoint
      if (playgroundEndpoint === path) {
        setPlaygroundEndpoint(null);
        setResponseData(null);
        setExecutionError(null);
      }
    } else {
      newExpanded.add(path);
    }
    setExpandedEndpoints(newExpanded);
  };

  const copyToClipboard = (text: string, label: string) => {
    navigator.clipboard.writeText(text);
    setCopiedCode(label);
    toast.success('Copied to clipboard!');
    setTimeout(() => setCopiedCode(null), 2000);
  };

  const getMethodColor = (method: string) => {
    switch (method) {
      case 'GET': return 'bg-blue-500';
      case 'POST': return 'bg-green-500';
      case 'PUT': return 'bg-yellow-500';
      case 'DELETE': return 'bg-red-500';
      default: return 'bg-gray-500';
    }
  };

  const executeAPIRequest = async (endpoint: APIEndpoint) => {
    setIsExecuting(true);
    setExecutionError(null);
    setResponseData(null);

    try {
      // Determine which token to use
      const token = isAuthenticated ? userToken : manualToken;

      if (endpoint.requiresAuth && !token) {
        setExecutionError('Authentication required. Please login or provide a JWT token.');
        setIsExecuting(false);
        return;
      }

      // Build request
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

      // Add body for POST/PUT requests
      if ((endpoint.method === 'POST' || endpoint.method === 'PUT') && requestBody) {
        try {
          JSON.parse(requestBody); // Validate JSON
          fetchOptions.body = requestBody;
        } catch (err) {
          setExecutionError('Invalid JSON in request body');
          setIsExecuting(false);
          return;
        }
      }

      // Execute request
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
    <div className="p-8 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">API Documentation</h1>
          <p className="text-muted-foreground mt-1">
            Complete reference for AIM REST API with interactive playground
          </p>
        </div>

        {/* Authentication Status */}
        <div className="flex items-center gap-3">
          {isAuthenticated ? (
            <Badge variant="default" className="flex items-center gap-2">
              <Unlock className="h-3 w-3" />
              Authenticated
            </Badge>
          ) : (
            <Button
              variant="outline"
              size="sm"
              onClick={() => window.location.href = '/auth/login'}
            >
              <LogIn className="mr-2 h-4 w-4" />
              Login to Test APIs
            </Button>
          )}
        </div>
      </div>

      {/* Quick Start Guide */}
      <Card className="border-blue-200 bg-blue-50">
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Code className="h-5 w-5 text-blue-600" />
            Quick Start
          </CardTitle>
          <CardDescription className="text-blue-900">
            Get started with AIM API in 3 simple steps
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="grid md:grid-cols-3 gap-4">
            <div className="flex items-start gap-3">
              <div className="flex-shrink-0 w-8 h-8 rounded-full bg-blue-600 text-white flex items-center justify-center font-bold">
                1
              </div>
              <div>
                <h3 className="font-semibold text-blue-900">Authenticate</h3>
                <p className="text-sm text-blue-700 mt-1">
                  Login with Google OAuth or use email/password to get JWT token
                </p>
              </div>
            </div>
            <div className="flex items-start gap-3">
              <div className="flex-shrink-0 w-8 h-8 rounded-full bg-blue-600 text-white flex items-center justify-center font-bold">
                2
              </div>
              <div>
                <h3 className="font-semibold text-blue-900">Make Requests</h3>
                <p className="text-sm text-blue-700 mt-1">
                  Include JWT token in Authorization header for all API calls
                </p>
              </div>
            </div>
            <div className="flex items-start gap-3">
              <div className="flex-shrink-0 w-8 h-8 rounded-full bg-blue-600 text-white flex items-center justify-center font-bold">
                3
              </div>
              <div>
                <h3 className="font-semibold text-blue-900">Test Live</h3>
                <p className="text-sm text-blue-700 mt-1">
                  Use the interactive playground below to test endpoints directly
                </p>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Search and Filter */}
      <div className="flex gap-4">
        <div className="flex-1 relative">
          <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder="Search endpoints..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="pl-10"
          />
        </div>
        <div className="flex gap-2">
          <Button
            variant={selectedCategory === null ? 'default' : 'outline'}
            size="sm"
            onClick={() => setSelectedCategory(null)}
          >
            <Filter className="mr-2 h-4 w-4" />
            All
          </Button>
          {apiEndpoints.map(category => (
            <Button
              key={category.category}
              variant={selectedCategory === category.category ? 'default' : 'outline'}
              size="sm"
              onClick={() => setSelectedCategory(
                selectedCategory === category.category ? null : category.category
              )}
            >
              {category.category.split(' ')[0]}
            </Button>
          ))}
        </div>
      </div>

      {/* API Endpoints */}
      <div className="space-y-6">
        {filteredEndpoints.map((category) => (
          <div key={category.category} className="space-y-3">
            <div>
              <h2 className="text-xl font-semibold">{category.category}</h2>
              <p className="text-sm text-muted-foreground">{category.description}</p>
            </div>

            {category.endpoints.map((endpoint) => {
              const isExpanded = expandedEndpoints.has(endpoint.path);
              const isPlaygroundActive = playgroundEndpoint === endpoint.path;

              return (
                <Card key={endpoint.path} className="hover:shadow-md transition-shadow">
                  <CardHeader
                    className="cursor-pointer"
                    onClick={() => toggleEndpoint(endpoint.path)}
                  >
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-3 flex-1">
                        <Badge className={`${getMethodColor(endpoint.method)} text-white`}>
                          {endpoint.method}
                        </Badge>
                        <code className="text-sm font-mono bg-muted px-2 py-1 rounded">
                          {endpoint.path}
                        </code>
                        {endpoint.requiresAuth && (
                          <Lock className="h-4 w-4 text-yellow-600" />
                        )}
                      </div>
                      <div className="flex items-center gap-2">
                        {isExpanded ? (
                          <ChevronUp className="h-5 w-5 text-muted-foreground" />
                        ) : (
                          <ChevronDown className="h-5 w-5 text-muted-foreground" />
                        )}
                      </div>
                    </div>
                    <CardDescription className="mt-2">
                      {endpoint.description}
                    </CardDescription>
                    <div className="flex items-center gap-2 mt-1">
                      <Badge variant="outline" className="text-xs">
                        {endpoint.auth}
                      </Badge>
                    </div>
                  </CardHeader>

                  {isExpanded && (
                    <CardContent className="space-y-4 border-t pt-4">
                      {/* Request/Response Documentation */}
                      {endpoint.request && (
                        <div>
                          <h4 className="font-semibold mb-2">Request Body:</h4>
                          <div className="bg-muted p-3 rounded-md space-y-1">
                            {Object.entries(endpoint.request).map(([key, value]) => (
                              <div key={key} className="text-sm font-mono">
                                <span className="text-blue-600">• {key}</span>: {value}
                              </div>
                            ))}
                          </div>
                        </div>
                      )}

                      {endpoint.response && (
                        <div>
                          <h4 className="font-semibold mb-2">Response:</h4>
                          <div className="bg-muted p-3 rounded-md space-y-1">
                            {Object.entries(endpoint.response).map(([key, value]) => (
                              <div key={key} className="text-sm font-mono">
                                <span className="text-green-600">• {key}</span>: {value}
                              </div>
                            ))}
                          </div>
                        </div>
                      )}

                      {/* Example curl command */}
                      <div>
                        <div className="flex items-center justify-between mb-2">
                          <h4 className="font-semibold">Example Request:</h4>
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={(e) => {
                              e.stopPropagation();
                              copyToClipboard(endpoint.example, endpoint.path);
                            }}
                          >
                            {copiedCode === endpoint.path ? (
                              <CheckCircle className="h-4 w-4 text-green-500" />
                            ) : (
                              <Copy className="h-4 w-4" />
                            )}
                          </Button>
                        </div>
                        <pre className="bg-gray-900 text-gray-100 p-4 rounded-md overflow-x-auto text-sm">
                          <code>{endpoint.example}</code>
                        </pre>
                      </div>

                      {/* Interactive API Playground */}
                      <div className="border-t pt-4 space-y-3">
                        <div className="flex items-center justify-between">
                          <h4 className="font-semibold flex items-center gap-2">
                            <Play className="h-4 w-4 text-blue-600" />
                            Interactive Playground
                          </h4>
                          {!isAuthenticated && endpoint.requiresAuth && (
                            <Button
                              variant="outline"
                              size="sm"
                              onClick={() => setShowTokenInput(!showTokenInput)}
                            >
                              <Key className="mr-2 h-3 w-3" />
                              {showTokenInput ? 'Hide Token' : 'Add Token'}
                            </Button>
                          )}
                        </div>

                        {/* Token Input for Unauthenticated Users */}
                        {!isAuthenticated && endpoint.requiresAuth && showTokenInput && (
                          <div className="space-y-2">
                            <label className="text-sm font-medium">JWT Token:</label>
                            <Input
                              placeholder="Paste your JWT token here..."
                              value={manualToken}
                              onChange={(e) => setManualToken(e.target.value)}
                              type="password"
                            />
                          </div>
                        )}

                        {/* Request Body Editor (for POST/PUT) */}
                        {(endpoint.method === 'POST' || endpoint.method === 'PUT') && (
                          <div className="space-y-2">
                            <label className="text-sm font-medium">Request Body (JSON):</label>
                            <Textarea
                              placeholder='{"key": "value"}'
                              value={requestBody}
                              onChange={(e) => setRequestBody(e.target.value)}
                              className="font-mono text-sm"
                              rows={6}
                            />
                          </div>
                        )}

                        {/* Execute Button */}
                        <Button
                          onClick={() => {
                            setPlaygroundEndpoint(endpoint.path);
                            executeAPIRequest(endpoint);
                          }}
                          disabled={isExecuting}
                          className="w-full"
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
                        {(responseData || executionError) && isPlaygroundActive && (
                          <div className="space-y-2">
                            <div className="flex items-center justify-between">
                              <label className="text-sm font-medium">Response:</label>
                              {responseData && (
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  onClick={() => copyToClipboard(
                                    JSON.stringify(responseData, null, 2),
                                    `response-${endpoint.path}`
                                  )}
                                >
                                  {copiedCode === `response-${endpoint.path}` ? (
                                    <CheckCircle className="h-4 w-4 text-green-500" />
                                  ) : (
                                    <Copy className="h-4 w-4" />
                                  )}
                                </Button>
                              )}
                            </div>

                            {executionError ? (
                              <div className="bg-red-50 border border-red-200 rounded-md p-4">
                                <div className="flex items-start gap-2">
                                  <AlertCircle className="h-5 w-5 text-red-600 flex-shrink-0 mt-0.5" />
                                  <div>
                                    <p className="text-sm font-semibold text-red-900">Error</p>
                                    <p className="text-sm text-red-700 mt-1">{executionError}</p>
                                  </div>
                                </div>
                              </div>
                            ) : (
                              <pre className="bg-green-50 border border-green-200 rounded-md p-4 overflow-x-auto text-sm">
                                <code className="text-green-900">
                                  {JSON.stringify(responseData, null, 2)}
                                </code>
                              </pre>
                            )}
                          </div>
                        )}
                      </div>
                    </CardContent>
                  )}
                </Card>
              );
            })}
          </div>
        ))}
      </div>
    </div>
  );
}

'use client';

import { useState } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Code,
  Book,
  Terminal,
  Copy,
  Check,
  ChevronDown,
  ChevronRight,
  Search,
  Filter,
  Download,
  ExternalLink
} from 'lucide-react';

// API endpoint categories with complete documentation
const apiEndpoints = [
  {
    category: 'Authentication & Authorization',
    description: 'User authentication, session management, and access control',
    endpoints: [
      {
        method: 'POST',
        path: '/api/v1/auth/login/local',
        description: 'Email/password authentication. Returns JWT token and user information.',
        auth: 'None',
        request: {
          email: 'string (required) - User email address',
          password: 'string (required) - User password'
        },
        response: {
          token: 'string - JWT access token',
          user: 'object - User information (id, email, role, organization)'
        },
        example: `curl -X POST https://api.aim.com/api/v1/auth/login/local \\
  -H "Content-Type: application/json" \\
  -d '{"email":"user@example.com","password":"secret"}'`
      },
      {
        method: 'POST',
        path: '/api/v1/auth/refresh',
        description: 'Refresh access token using refresh token. Implements token rotation for security.',
        auth: 'Refresh Token',
        request: {
          refresh_token: 'string (required) - Valid refresh token'
        },
        response: {
          token: 'string - New JWT access token',
          refresh_token: 'string - New refresh token (rotation)'
        }
      },
      {
        method: 'GET',
        path: '/api/v1/auth/me',
        description: 'Get currently authenticated user information.',
        auth: 'JWT',
        response: {
          id: 'string - User UUID',
          email: 'string - User email',
          role: 'string - User role (admin|manager|member|viewer)',
          organization_id: 'string - Organization UUID'
        }
      }
    ]
  },
  {
    category: 'Agent Lifecycle Management',
    description: 'Create, manage, and control AI agent identities',
    endpoints: [
      {
        method: 'GET',
        path: '/api/v1/agents',
        description: 'List all agents in organization. Supports filtering and pagination.',
        auth: 'JWT',
        query: {
          status: 'string (optional) - Filter by status (verified|pending|suspended)',
          page: 'number (optional) - Page number (default: 1)',
          limit: 'number (optional) - Results per page (default: 20, max: 100)'
        },
        response: {
          agents: 'array - List of agent objects',
          total: 'number - Total count',
          page: 'number - Current page',
          pages: 'number - Total pages'
        }
      },
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
  -d '{"name":"MyAgent","type":"ai_agent"}'`
      },
      {
        method: 'POST',
        path: '/api/v1/agents/:id/suspend',
        description: 'Temporarily suspend an agent. All API keys become invalid immediately.',
        auth: 'JWT (manager+)',
        response: {
          success: 'boolean',
          message: 'string'
        }
      },
      {
        method: 'POST',
        path: '/api/v1/agents/:id/reactivate',
        description: 'Reactivate a suspended agent. Previous API keys remain invalid.',
        auth: 'JWT (manager+)',
        response: {
          success: 'boolean',
          message: 'string'
        }
      },
      {
        method: 'POST',
        path: '/api/v1/agents/:id/rotate-credentials',
        description: 'Generate new API key and revoke old one. Use for key rotation or security incidents.',
        auth: 'JWT (member+)',
        response: {
          api_key: 'string - New API key (shown only once!)',
          message: 'string'
        }
      },
      {
        method: 'POST',
        path: '/api/v1/agents/:id/verify',
        description: 'Manually verify agent identity (admin action).',
        auth: 'JWT (manager+)',
        response: {
          success: 'boolean',
          agent: 'object - Updated agent'
        }
      }
    ]
  },
  {
    category: 'Compliance & Audit',
    description: 'Compliance monitoring, audit logging, and access reviews',
    endpoints: [
      {
        method: 'GET',
        path: '/api/v1/compliance/status',
        description: 'Get overall compliance status and score.',
        auth: 'JWT (admin)',
        response: {
          score: 'number - Compliance score (0-100)',
          status: 'string - compliant|non_compliant|needs_review',
          last_check: 'string - ISO 8601 timestamp'
        }
      },
      {
        method: 'GET',
        path: '/api/v1/compliance/metrics',
        description: 'Get detailed compliance metrics.',
        auth: 'JWT (admin)',
        response: {
          verification_rate: 'number - Percentage of verified agents',
          audit_coverage: 'number - Percentage of actions audited',
          policy_compliance: 'number - Policy adherence rate'
        }
      },
      {
        method: 'GET',
        path: '/api/v1/compliance/access-review',
        description: 'List all users with access for quarterly access review.',
        auth: 'JWT (admin)',
        response: {
          users: 'array - Users with access details',
          last_review: 'string - Last review date',
          next_review: 'string - Next scheduled review'
        }
      },
      {
        method: 'POST',
        path: '/api/v1/compliance/check',
        description: 'Run compliance check and generate report.',
        auth: 'JWT (admin)',
        response: {
          report_id: 'string - Report UUID',
          status: 'string - Check status',
          findings: 'array - Compliance findings'
        }
      },
      {
        method: 'GET',
        path: '/api/v1/compliance/export',
        description: 'Export compliance report as PDF or CSV.',
        auth: 'JWT (admin)',
        query: {
          format: 'string (required) - Export format (pdf|csv)',
          from_date: 'string (optional) - Start date (ISO 8601)',
          to_date: 'string (optional) - End date (ISO 8601)'
        },
        response: 'Binary file download'
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
        description: 'List security alerts with filtering options.',
        auth: 'JWT (admin|manager)',
        query: {
          severity: 'string (optional) - Filter by severity (low|medium|high|critical)',
          status: 'string (optional) - Filter by status (new|acknowledged|resolved)',
          limit: 'number (optional) - Results per page (default: 20)'
        },
        response: {
          alerts: 'array - Security alerts',
          unacknowledged_count: 'number - Unread alert count'
        }
      },
      {
        method: 'POST',
        path: '/api/v1/admin/alerts/:id/acknowledge',
        description: 'Mark alert as acknowledged by current user.',
        auth: 'JWT (admin)',
        response: {
          success: 'boolean',
          alert: 'object - Updated alert'
        }
      },
      {
        method: 'POST',
        path: '/api/v1/admin/alerts/:id/resolve',
        description: 'Mark alert as resolved with optional resolution notes.',
        auth: 'JWT (admin)',
        request: {
          notes: 'string (optional) - Resolution notes'
        },
        response: {
          success: 'boolean',
          alert: 'object - Updated alert'
        }
      },
      {
        method: 'GET',
        path: '/api/v1/admin/security-policies',
        description: 'List all security policies.',
        auth: 'JWT (admin)',
        response: {
          policies: 'array - Security policy objects'
        }
      }
    ]
  },
  {
    category: 'Analytics & Reporting',
    description: 'Usage analytics, verification activity, and trust score trends',
    endpoints: [
      {
        method: 'GET',
        path: '/api/v1/analytics/dashboard',
        description: 'Get high-level dashboard statistics.',
        auth: 'JWT',
        response: {
          total_agents: 'number - Total agent count',
          verified_agents: 'number - Verified agent count',
          total_verifications: 'number - Verification event count',
          success_rate: 'number - Verification success rate (0-100)'
        }
      },
      {
        method: 'GET',
        path: '/api/v1/analytics/verification-activity',
        description: 'Get verification events over time for charting.',
        auth: 'JWT',
        query: {
          days: 'number (optional) - Number of days to query (default: 30)',
          interval: 'string (optional) - Time interval (hour|day|week)'
        },
        response: {
          data: 'array - Time-series data points',
          total_events: 'number - Total event count'
        }
      },
      {
        method: 'GET',
        path: '/api/v1/analytics/trends',
        description: 'Get trust score trends over time.',
        auth: 'JWT',
        query: {
          agent_id: 'string (optional) - Filter by agent UUID',
          days: 'number (optional) - Number of days (default: 30)'
        },
        response: {
          trends: 'array - Trust score trend data'
        }
      },
      {
        method: 'GET',
        path: '/api/v1/analytics/agents/activity',
        description: 'Get agent activity logs and statistics.',
        auth: 'JWT',
        query: {
          agent_id: 'string (optional) - Filter by agent UUID',
          action_type: 'string (optional) - Filter by action type',
          limit: 'number (optional) - Results per page'
        },
        response: {
          activities: 'array - Activity log entries',
          total: 'number - Total count'
        }
      }
    ]
  },
  {
    category: 'SDK & Integration',
    description: 'SDK download, token management, and programmatic access',
    endpoints: [
      {
        method: 'GET',
        path: '/api/v1/sdk/download',
        description: 'Download Python SDK with embedded credentials for quick start.',
        auth: 'JWT',
        response: 'Binary file download (.zip)'
      },
      {
        method: 'GET',
        path: '/api/v1/users/me/sdk-tokens',
        description: 'List all SDK tokens for current user.',
        auth: 'JWT',
        response: {
          tokens: 'array - SDK token objects',
          active_count: 'number - Active token count'
        }
      },
      {
        method: 'POST',
        path: '/api/v1/users/me/sdk-tokens/:id/revoke',
        description: 'Revoke specific SDK token immediately.',
        auth: 'JWT',
        response: {
          success: 'boolean',
          message: 'string'
        }
      }
    ]
  }
];

export default function DevelopersPage() {
  const [searchTerm, setSearchTerm] = useState('');
  const [selectedCategory, setSelectedCategory] = useState<string | null>(null);
  const [expandedEndpoints, setExpandedEndpoints] = useState<Set<string>>(new Set());
  const [copiedCode, setCopiedCode] = useState<string | null>(null);

  const toggleEndpoint = (path: string) => {
    const newExpanded = new Set(expandedEndpoints);
    if (newExpanded.has(path)) {
      newExpanded.delete(path);
    } else {
      newExpanded.add(path);
    }
    setExpandedEndpoints(newExpanded);
  };

  const copyToClipboard = (text: string, id: string) => {
    navigator.clipboard.writeText(text);
    setCopiedCode(id);
    setTimeout(() => setCopiedCode(null), 2000);
  };

  const getMethodColor = (method: string) => {
    switch (method) {
      case 'GET': return 'bg-purple-100 dark:bg-purple-900/30 text-purple-700 dark:text-purple-300';
      case 'POST': return 'bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300';
      case 'PUT': return 'bg-orange-100 dark:bg-orange-900/30 text-orange-700 dark:text-orange-300';
      case 'DELETE': return 'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300';
      default: return 'bg-gray-100 dark:bg-gray-900/30 text-gray-700 dark:text-gray-300';
    }
  };

  const filteredEndpoints = apiEndpoints.filter(category => {
    if (selectedCategory && category.category !== selectedCategory) return false;
    if (!searchTerm) return true;

    const searchLower = searchTerm.toLowerCase();
    return (
      category.category.toLowerCase().includes(searchLower) ||
      category.description.toLowerCase().includes(searchLower) ||
      category.endpoints.some(endpoint =>
        endpoint.path.toLowerCase().includes(searchLower) ||
        endpoint.description.toLowerCase().includes(searchLower)
      )
    );
  });

  return (
    <div className="p-8 space-y-6 max-w-7xl">
      {/* Header */}
      <div>
        <h1 className="text-3xl font-bold tracking-tight">API Documentation</h1>
        <p className="text-muted-foreground mt-1">
          Complete reference for AIM REST API endpoints
        </p>
      </div>

      {/* Quick Start Card */}
      <Card className="border-blue-200 dark:border-blue-800 bg-blue-50 dark:bg-blue-900/10">
        <CardHeader>
          <div className="flex items-center gap-2">
            <Terminal className="h-5 w-5 text-blue-600 dark:text-blue-400" />
            <CardTitle className="text-blue-900 dark:text-blue-100">Quick Start</CardTitle>
          </div>
          <CardDescription className="text-blue-700 dark:text-blue-300">
            Get started with the AIM API in under 5 minutes
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div className="flex flex-col gap-2">
              <div className="flex items-center gap-2">
                <div className="w-6 h-6 rounded-full bg-blue-600 text-white flex items-center justify-center text-sm font-bold">1</div>
                <span className="font-semibold text-blue-900 dark:text-blue-100">Get API Key</span>
              </div>
              <p className="text-sm text-blue-700 dark:text-blue-300 pl-8">
                Register an agent or download SDK to get your API key
              </p>
            </div>
            <div className="flex flex-col gap-2">
              <div className="flex items-center gap-2">
                <div className="w-6 h-6 rounded-full bg-blue-600 text-white flex items-center justify-center text-sm font-bold">2</div>
                <span className="font-semibold text-blue-900 dark:text-blue-100">Make Request</span>
              </div>
              <p className="text-sm text-blue-700 dark:text-blue-300 pl-8">
                Use Bearer token authentication in Authorization header
              </p>
            </div>
            <div className="flex flex-col gap-2">
              <div className="flex items-center gap-2">
                <div className="w-6 h-6 rounded-full bg-blue-600 text-white flex items-center justify-center text-sm font-bold">3</div>
                <span className="font-semibold text-blue-900 dark:text-blue-100">Handle Response</span>
              </div>
              <p className="text-sm text-blue-700 dark:text-blue-300 pl-8">
                All responses are JSON with consistent error structure
              </p>
            </div>
          </div>
          <div className="flex gap-2">
            <Button variant="default" size="sm" className="bg-blue-600 hover:bg-blue-700">
              <Download className="h-4 w-4 mr-2" />
              Download SDK
            </Button>
            <Button variant="outline" size="sm">
              <Book className="h-4 w-4 mr-2" />
              View Examples
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* Search and Filter */}
      <div className="flex flex-col sm:flex-row gap-4">
        <div className="flex-1 relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" />
          <input
            type="text"
            placeholder="Search endpoints..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="w-full pl-10 pr-4 py-2 border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
        </div>
        <div className="flex gap-2 flex-wrap">
          <Button
            variant={selectedCategory === null ? 'default' : 'outline'}
            size="sm"
            onClick={() => setSelectedCategory(null)}
          >
            All
          </Button>
          {apiEndpoints.map(category => (
            <Button
              key={category.category}
              variant={selectedCategory === category.category ? 'default' : 'outline'}
              size="sm"
              onClick={() => setSelectedCategory(category.category)}
            >
              {category.category}
            </Button>
          ))}
        </div>
      </div>

      {/* API Endpoints */}
      {filteredEndpoints.map((category) => (
        <div key={category.category} className="space-y-4">
          <div>
            <h2 className="text-2xl font-bold text-gray-900 dark:text-gray-100">{category.category}</h2>
            <p className="text-gray-600 dark:text-gray-400 text-sm mt-1">{category.description}</p>
          </div>

          <div className="space-y-3">
            {category.endpoints.map((endpoint, idx) => {
              const endpointId = `${category.category}-${idx}`;
              const isExpanded = expandedEndpoints.has(endpointId);

              return (
                <Card key={endpointId} className="overflow-hidden">
                  <button
                    onClick={() => toggleEndpoint(endpointId)}
                    className="w-full text-left p-4 hover:bg-gray-50 dark:hover:bg-gray-800/50 transition-colors"
                  >
                    <div className="flex items-center gap-3">
                      <span className={`px-2.5 py-1 rounded text-xs font-mono font-bold ${getMethodColor(endpoint.method)}`}>
                        {endpoint.method}
                      </span>
                      <code className="text-sm font-mono text-gray-900 dark:text-gray-100 flex-1">
                        {endpoint.path}
                      </code>
                      <Badge variant="outline" className="text-xs">
                        {endpoint.auth}
                      </Badge>
                      {isExpanded ? (
                        <ChevronDown className="h-4 w-4 text-gray-400" />
                      ) : (
                        <ChevronRight className="h-4 w-4 text-gray-400" />
                      )}
                    </div>
                    <p className="text-sm text-gray-600 dark:text-gray-400 mt-2 ml-20">
                      {endpoint.description}
                    </p>
                  </button>

                  {isExpanded && (
                    <div className="border-t border-gray-200 dark:border-gray-700 p-4 space-y-4 bg-gray-50 dark:bg-gray-900">
                      {/* Request Parameters */}
                      {endpoint.request && (
                        <div>
                          <h4 className="text-sm font-semibold text-gray-900 dark:text-gray-100 mb-2">Request Body</h4>
                          <div className="bg-white dark:bg-gray-800 rounded-lg p-3 space-y-1">
                            {Object.entries(endpoint.request).map(([key, value]) => (
                              <div key={key} className="flex gap-2 text-sm">
                                <code className="text-blue-600 dark:text-blue-400 font-mono">{key}:</code>
                                <span className="text-gray-600 dark:text-gray-400">{value}</span>
                              </div>
                            ))}
                          </div>
                        </div>
                      )}

                      {/* Query Parameters */}
                      {endpoint.query && (
                        <div>
                          <h4 className="text-sm font-semibold text-gray-900 dark:text-gray-100 mb-2">Query Parameters</h4>
                          <div className="bg-white dark:bg-gray-800 rounded-lg p-3 space-y-1">
                            {Object.entries(endpoint.query).map(([key, value]) => (
                              <div key={key} className="flex gap-2 text-sm">
                                <code className="text-purple-600 dark:text-purple-400 font-mono">{key}:</code>
                                <span className="text-gray-600 dark:text-gray-400">{value}</span>
                              </div>
                            ))}
                          </div>
                        </div>
                      )}

                      {/* Response */}
                      {endpoint.response && (
                        <div>
                          <h4 className="text-sm font-semibold text-gray-900 dark:text-gray-100 mb-2">Response</h4>
                          <div className="bg-white dark:bg-gray-800 rounded-lg p-3 space-y-1">
                            {typeof endpoint.response === 'string' ? (
                              <span className="text-sm text-gray-600 dark:text-gray-400">{endpoint.response}</span>
                            ) : (
                              Object.entries(endpoint.response).map(([key, value]) => (
                                <div key={key} className="flex gap-2 text-sm">
                                  <code className="text-green-600 dark:text-green-400 font-mono">{key}:</code>
                                  <span className="text-gray-600 dark:text-gray-400">{value}</span>
                                </div>
                              ))
                            )}
                          </div>
                        </div>
                      )}

                      {/* Example curl */}
                      {endpoint.example && (
                        <div>
                          <div className="flex items-center justify-between mb-2">
                            <h4 className="text-sm font-semibold text-gray-900 dark:text-gray-100">Example Request</h4>
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => copyToClipboard(endpoint.example!, `example-${endpointId}`)}
                              className="h-8"
                            >
                              {copiedCode === `example-${endpointId}` ? (
                                <Check className="h-4 w-4 text-green-600" />
                              ) : (
                                <Copy className="h-4 w-4" />
                              )}
                            </Button>
                          </div>
                          <pre className="bg-gray-900 dark:bg-black text-gray-100 rounded-lg p-3 overflow-x-auto text-xs font-mono">
                            {endpoint.example}
                          </pre>
                        </div>
                      )}
                    </div>
                  )}
                </Card>
              );
            })}
          </div>
        </div>
      ))}

      {/* No Results */}
      {filteredEndpoints.length === 0 && (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12">
            <Search className="h-12 w-12 text-gray-400 mb-4" />
            <h3 className="text-lg font-semibold mb-2">No endpoints found</h3>
            <p className="text-gray-600 dark:text-gray-400 text-center max-w-md">
              Try adjusting your search or filter criteria
            </p>
          </CardContent>
        </Card>
      )}

      {/* Footer Resources */}
      <Card>
        <CardHeader>
          <CardTitle>Additional Resources</CardTitle>
          <CardDescription>Learn more about integrating with AIM</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <a
              href="/docs/quickstart"
              className="flex items-center gap-3 p-4 rounded-lg border border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors"
            >
              <Terminal className="h-5 w-5 text-blue-600" />
              <div className="flex-1">
                <div className="font-semibold text-sm">Quick Start Guide</div>
                <div className="text-xs text-gray-600 dark:text-gray-400">Get started in 5 minutes</div>
              </div>
              <ExternalLink className="h-4 w-4 text-gray-400" />
            </a>
            <a
              href="/docs/sdk"
              className="flex items-center gap-3 p-4 rounded-lg border border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors"
            >
              <Code className="h-5 w-5 text-purple-600" />
              <div className="flex-1">
                <div className="font-semibold text-sm">SDK Documentation</div>
                <div className="text-xs text-gray-600 dark:text-gray-400">Python, Node.js, Go SDKs</div>
              </div>
              <ExternalLink className="h-4 w-4 text-gray-400" />
            </a>
            <a
              href="/docs/examples"
              className="flex items-center gap-3 p-4 rounded-lg border border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors"
            >
              <Book className="h-5 w-5 text-green-600" />
              <div className="flex-1">
                <div className="font-semibold text-sm">Code Examples</div>
                <div className="text-xs text-gray-600 dark:text-gray-400">Real-world use cases</div>
              </div>
              <ExternalLink className="h-4 w-4 text-gray-400" />
            </a>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

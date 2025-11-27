'use client';

import { useState, useEffect } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Shield, Plus, Edit, Trash2, AlertCircle } from 'lucide-react';

interface SecurityPolicy {
  id: string;
  name: string;
  description: string;
  policyType: string;
  enabled: boolean;
  severity: 'low' | 'medium' | 'high' | 'critical';
  conditions: any;
  actions: any;
  createdAt: string;
  updatedAt: string;
}

export default function SecurityPoliciesPage() {
  const [policies, setPolicies] = useState<SecurityPolicy[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetchPolicies();
  }, []);

  const fetchPolicies = async () => {
    try {
      const token = localStorage.getItem('auth_token');
      if (!token) {
        setError('Not authenticated');
        setLoading(false);
        return;
      }

      const response = await fetch('http://localhost:8080/api/v1/admin/security-policies', {
        headers: {
          'Authorization': `Bearer ${token}`,
        },
      });

      if (!response.ok) {
        throw new Error(`Failed to fetch policies: ${response.statusText}`);
      }

      const data = await response.json();
      // API returns array directly, not wrapped in { policies: [...] }
      setPolicies(Array.isArray(data) ? data : (data.policies || []));
      setError(null);
    } catch (err) {
      console.error('Error fetching security policies:', err);
      setError(err instanceof Error ? err.message : 'Failed to load policies');
    } finally {
      setLoading(false);
    }
  };

  const getSeverityVariant = (severity: string) => {
    switch (severity) {
      case 'critical':
        return 'destructive';
      case 'high':
        return 'destructive';
      case 'medium':
        return 'default';
      case 'low':
        return 'secondary';
      default:
        return 'secondary';
    }
  };

  if (loading) {
    return (
      <div className="p-8">
        <div className="flex items-center justify-center h-64">
          <div className="text-muted-foreground">Loading security policies...</div>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="p-8">
        <Card className="border-red-200 bg-red-50">
          <CardHeader>
            <div className="flex items-center gap-2">
              <AlertCircle className="h-5 w-5 text-red-500" />
              <CardTitle className="text-red-700">Error Loading Policies</CardTitle>
            </div>
            <CardDescription className="text-red-600">{error}</CardDescription>
          </CardHeader>
        </Card>
      </div>
    );
  }

  return (
    <div className="p-8 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Security Policies</h1>
          <p className="text-muted-foreground mt-1">
            Manage security policies for agent behavior and access control
          </p>
        </div>
        <Button>
          <Plus className="mr-2 h-4 w-4" />
          Create Policy
        </Button>
      </div>

      {/* Stats Overview */}
      <div className="grid gap-4 md:grid-cols-4">
        <Card>
          <CardHeader className="pb-3">
            <CardDescription>Total Policies</CardDescription>
            <CardTitle className="text-2xl">{policies.length}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-3">
            <CardDescription>Enabled</CardDescription>
            <CardTitle className="text-2xl text-green-600">
              {policies.filter(p => p.enabled).length}
            </CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-3">
            <CardDescription>Disabled</CardDescription>
            <CardTitle className="text-2xl text-gray-500">
              {policies.filter(p => !p.enabled).length}
            </CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-3">
            <CardDescription>Critical Severity</CardDescription>
            <CardTitle className="text-2xl text-red-600">
              {policies.filter(p => p.severity === 'critical').length}
            </CardTitle>
          </CardHeader>
        </Card>
      </div>

      {/* Policies List */}
      {policies.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12">
            <Shield className="h-12 w-12 text-muted-foreground mb-4" />
            <h3 className="text-lg font-semibold mb-2">No Security Policies</h3>
            <p className="text-muted-foreground text-center max-w-md mb-4">
              Get started by creating your first security policy to control agent behavior and access.
            </p>
            <Button>
              <Plus className="mr-2 h-4 w-4" />
              Create Your First Policy
            </Button>
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-4">
          {policies.map((policy) => (
            <Card key={policy.id} className="hover:shadow-md transition-shadow">
              <CardHeader>
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3 flex-1">
                    <Shield className="h-5 w-5 text-blue-500 flex-shrink-0" />
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2 mb-1">
                        <CardTitle className="text-lg">{policy.name}</CardTitle>
                        <Badge variant={policy.enabled ? 'default' : 'secondary'}>
                          {policy.enabled ? 'Enabled' : 'Disabled'}
                        </Badge>
                        <Badge variant={getSeverityVariant(policy.severity)}>
                          {policy.severity.toUpperCase()}
                        </Badge>
                      </div>
                      <CardDescription className="line-clamp-2">
                        {policy.description}
                      </CardDescription>
                      <div className="flex items-center gap-4 mt-2 text-xs text-muted-foreground">
                        <span>Type: {policy.policyType}</span>
                        <span>•</span>
                        <span>Created: {new Date(policy.createdAt).toLocaleDateString()}</span>
                      </div>
                    </div>
                  </div>
                  <div className="flex items-center gap-2 flex-shrink-0">
                    <Button variant="ghost" size="sm">
                      <Edit className="h-4 w-4" />
                    </Button>
                    <Button variant="ghost" size="sm">
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                </div>
              </CardHeader>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}

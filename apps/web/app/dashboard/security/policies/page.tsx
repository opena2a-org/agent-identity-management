'use client';

import { useState, useEffect } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import { Shield, Plus, Edit, Trash2, AlertCircle, ShieldAlert, Eye, Lock, Loader2, Info } from 'lucide-react';
import { api } from '@/lib/api';

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

interface EnforcementSettings {
  enforcementMode: 'strict' | 'monitoring';
  description: string;
  explanation: string;
  impact: string;
}

export default function SecurityPoliciesPage() {
  const [policies, setPolicies] = useState<SecurityPolicy[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Enforcement mode state
  const [enforcementSettings, setEnforcementSettings] = useState<EnforcementSettings | null>(null);
  const [enforcementLoading, setEnforcementLoading] = useState(true);
  const [enforcementUpdating, setEnforcementUpdating] = useState(false);
  const [enforcementError, setEnforcementError] = useState<string | null>(null);

  useEffect(() => {
    fetchPolicies();
    fetchEnforcementSettings();
  }, []);

  const fetchEnforcementSettings = async () => {
    try {
      const settings = await api.getEnforcementSettings();
      setEnforcementSettings(settings);
      setEnforcementError(null);
    } catch (err) {
      console.error('Error fetching enforcement settings:', err);
      setEnforcementError('Failed to load enforcement settings');
    } finally {
      setEnforcementLoading(false);
    }
  };

  const handleEnforcementModeChange = async (checked: boolean) => {
    const newMode = checked ? 'strict' : 'monitoring';
    setEnforcementUpdating(true);
    try {
      const settings = await api.updateEnforcementSettings(newMode);
      setEnforcementSettings(settings);
      setEnforcementError(null);
    } catch (err) {
      console.error('Error updating enforcement mode:', err);
      setEnforcementError('Failed to update enforcement mode');
    } finally {
      setEnforcementUpdating(false);
    }
  };

  const fetchPolicies = async () => {
    try {
      const token = localStorage.getItem('auth_token');
      if (!token) {
        setError('Not authenticated');
        setLoading(false);
        return;
      }

      // Dynamic API URL based on environment
      const apiUrl = process.env.NEXT_PUBLIC_API_URL ||
        (typeof window !== 'undefined' && (window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1')
          ? `${window.location.protocol}//${window.location.hostname}:8080`
          : `${window.location.protocol}//${window.location.host}`);
      const response = await fetch(`${apiUrl}/api/v1/admin/security-policies`, {
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

      {/* Enforcement Mode Section */}
      <Card className="border-2 border-primary/20 bg-gradient-to-r from-primary/5 to-transparent">
        <CardHeader>
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              {enforcementSettings?.enforcementMode === 'strict' ? (
                <div className="p-2 bg-red-100 dark:bg-red-900/30 rounded-lg">
                  <Lock className="h-6 w-6 text-red-600 dark:text-red-400" />
                </div>
              ) : (
                <div className="p-2 bg-blue-100 dark:bg-blue-900/30 rounded-lg">
                  <Eye className="h-6 w-6 text-blue-600 dark:text-blue-400" />
                </div>
              )}
              <div>
                <CardTitle className="text-xl flex items-center gap-2">
                  Enforcement Mode
                  {enforcementSettings && (
                    <Badge variant={enforcementSettings.enforcementMode === 'strict' ? 'destructive' : 'secondary'}>
                      {enforcementSettings.enforcementMode === 'strict' ? 'Strict' : 'Monitoring'}
                    </Badge>
                  )}
                </CardTitle>
                <CardDescription>
                  Control how the SDK handles verification failures
                </CardDescription>
              </div>
            </div>
            {enforcementLoading ? (
              <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
            ) : enforcementUpdating ? (
              <div className="flex items-center gap-2">
                <Loader2 className="h-4 w-4 animate-spin" />
                <span className="text-sm text-muted-foreground">Updating...</span>
              </div>
            ) : (
              <div className="flex items-center gap-3">
                <span className={`text-sm font-medium ${enforcementSettings?.enforcementMode === 'strict' ? 'text-red-600' : 'text-muted-foreground'}`}>
                  {enforcementSettings?.enforcementMode === 'strict' ? 'Strict' : 'Monitoring'}
                </span>
                <Switch
                  checked={enforcementSettings?.enforcementMode === 'strict'}
                  onCheckedChange={handleEnforcementModeChange}
                  disabled={enforcementUpdating}
                />
              </div>
            )}
          </div>
        </CardHeader>
        <CardContent>
          {enforcementError ? (
            <div className="flex items-center gap-2 text-red-600">
              <AlertCircle className="h-4 w-4" />
              <span className="text-sm">{enforcementError}</span>
            </div>
          ) : enforcementSettings ? (
            <div className="space-y-4">
              {/* Mode explanation */}
              <div className="grid md:grid-cols-2 gap-4">
                {/* Monitoring Mode */}
                <div className={`p-4 rounded-lg border ${enforcementSettings.enforcementMode === 'monitoring' ? 'border-blue-200 bg-blue-50 dark:border-blue-800 dark:bg-blue-900/20' : 'border-gray-200 bg-gray-50 dark:border-gray-700 dark:bg-gray-800/50'}`}>
                  <div className="flex items-center gap-2 mb-2">
                    <Eye className={`h-5 w-5 ${enforcementSettings.enforcementMode === 'monitoring' ? 'text-blue-600' : 'text-gray-400'}`} />
                    <h4 className={`font-semibold ${enforcementSettings.enforcementMode === 'monitoring' ? 'text-blue-700 dark:text-blue-300' : 'text-gray-600 dark:text-gray-400'}`}>
                      Monitoring Mode
                      {enforcementSettings.enforcementMode === 'monitoring' && (
                        <Badge variant="outline" className="ml-2 text-xs">Active</Badge>
                      )}
                    </h4>
                  </div>
                  <p className="text-sm text-muted-foreground mb-2">
                    When verification fails, actions are <strong>logged but allowed</strong> to proceed.
                  </p>
                  <p className="text-sm text-muted-foreground mb-2">
                    This covers verifications AIM could not complete. An action AIM
                    explicitly <strong>denies</strong> is still blocked, in either mode.
                  </p>
                  <div className="text-xs text-muted-foreground bg-background/50 p-2 rounded">
                    <Info className="h-3 w-3 inline mr-1" />
                    Use during initial deployment to understand agent behavior before enforcing strict policies.
                  </div>
                </div>

                {/* Strict Mode */}
                <div className={`p-4 rounded-lg border ${enforcementSettings.enforcementMode === 'strict' ? 'border-red-200 bg-red-50 dark:border-red-800 dark:bg-red-900/20' : 'border-gray-200 bg-gray-50 dark:border-gray-700 dark:bg-gray-800/50'}`}>
                  <div className="flex items-center gap-2 mb-2">
                    <Lock className={`h-5 w-5 ${enforcementSettings.enforcementMode === 'strict' ? 'text-red-600' : 'text-gray-400'}`} />
                    <h4 className={`font-semibold ${enforcementSettings.enforcementMode === 'strict' ? 'text-red-700 dark:text-red-300' : 'text-gray-600 dark:text-gray-400'}`}>
                      Strict Mode
                      {enforcementSettings.enforcementMode === 'strict' && (
                        <Badge variant="destructive" className="ml-2 text-xs">Active</Badge>
                      )}
                    </h4>
                  </div>
                  <p className="text-sm text-muted-foreground mb-2">
                    When verification fails, actions are <strong>blocked immediately</strong>.
                  </p>
                  <div className="text-xs text-muted-foreground bg-background/50 p-2 rounded">
                    <ShieldAlert className="h-3 w-3 inline mr-1" />
                    Use in production for maximum security. Unauthorized actions will not execute.
                  </div>
                </div>
              </div>

              {/* Current impact */}
              <div className="p-3 bg-muted/50 rounded-lg text-sm">
                <strong>Current Impact:</strong> {enforcementSettings.impact}
              </div>
            </div>
          ) : null}
        </CardContent>
      </Card>

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

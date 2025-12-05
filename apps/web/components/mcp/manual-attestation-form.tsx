'use client';

import { useState } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import { Checkbox } from '@/components/ui/checkbox';
import { Label } from '@/components/ui/label';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import {
  CheckCircle,
  XCircle,
  Loader2,
  Wifi,
  Heart,
  Shield,
  FileText,
  AlertTriangle,
  RefreshCw,
  Send,
} from 'lucide-react';
import { api } from '@/lib/api';

interface ManualAttestationFormProps {
  mcpServerId: string;
  mcpServerName: string;
  mcpServerUrl: string;
  capabilities: string[];
  onAttestationSubmitted?: () => void;
  userRole?: 'admin' | 'manager' | 'member' | 'viewer';
}

interface ConnectionTestResult {
  success: boolean;
  latencyMs: number;
  error?: string;
}

interface HealthCheckResult {
  success: boolean;
  details?: string;
  error?: string;
}

export function ManualAttestationForm({
  mcpServerId,
  mcpServerName,
  mcpServerUrl,
  capabilities,
  onAttestationSubmitted,
  userRole = 'viewer',
}: ManualAttestationFormProps) {
  const [connectionTesting, setConnectionTesting] = useState(false);
  const [connectionResult, setConnectionResult] = useState<ConnectionTestResult | null>(null);
  const [healthChecking, setHealthChecking] = useState(false);
  const [healthResult, setHealthResult] = useState<HealthCheckResult | null>(null);
  const [confirmedCapabilities, setConfirmedCapabilities] = useState<string[]>([]);
  const [notes, setNotes] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [submitSuccess, setSubmitSuccess] = useState(false);

  const canSubmit = userRole === 'admin' || userRole === 'manager';

  // Test connection to MCP server
  const testConnection = async () => {
    setConnectionTesting(true);
    setConnectionResult(null);
    try {
      const startTime = Date.now();
      // Call API to test connection
      const result = await api.testMCPConnection(mcpServerId);
      const latency = Date.now() - startTime;

      setConnectionResult({
        success: result.success ?? true,
        latencyMs: result.latencyMs ?? latency,
        error: result.error,
      });
    } catch (err: any) {
      setConnectionResult({
        success: false,
        latencyMs: 0,
        error: err.message || 'Connection failed',
      });
    } finally {
      setConnectionTesting(false);
    }
  };

  // Run health check
  const runHealthCheck = async () => {
    setHealthChecking(true);
    setHealthResult(null);
    try {
      // Call API to run health check
      const result = await api.healthCheckMCP(mcpServerId);
      setHealthResult({
        success: result.healthy ?? result.success ?? true,
        details: result.details,
        error: result.error,
      });
    } catch (err: any) {
      setHealthResult({
        success: false,
        error: err.message || 'Health check failed',
      });
    } finally {
      setHealthChecking(false);
    }
  };

  // Handle capability checkbox toggle
  const toggleCapability = (capability: string) => {
    setConfirmedCapabilities(prev =>
      prev.includes(capability)
        ? prev.filter(c => c !== capability)
        : [...prev, capability]
    );
  };

  // Submit manual attestation
  const submitAttestation = async () => {
    if (!canSubmit) return;

    setSubmitting(true);
    setSubmitError(null);
    setSubmitSuccess(false);

    try {
      await api.manualAttestMCP(mcpServerId, {
        connectionTested: connectionResult?.success ?? false,
        connectionLatencyMs: connectionResult?.latencyMs ?? 0,
        healthCheckPassed: healthResult?.success ?? false,
        confirmedCapabilities,
        notes,
      });

      setSubmitSuccess(true);
      setNotes('');
      setConfirmedCapabilities([]);
      setConnectionResult(null);
      setHealthResult(null);

      if (onAttestationSubmitted) {
        onAttestationSubmitted();
      }
    } catch (err: any) {
      setSubmitError(err.message || 'Failed to submit attestation');
    } finally {
      setSubmitting(false);
    }
  };

  const allCapabilitiesConfirmed = confirmedCapabilities.length === capabilities.length;

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-2">
          <Shield className="h-5 w-5 text-blue-600" />
          <CardTitle>Manual Attestation</CardTitle>
        </div>
        <CardDescription>
          Manually verify this MCP server by testing its connection and confirming its capabilities.
          {!canSubmit && (
            <span className="block mt-1 text-orange-600">
              Only admins and managers can submit attestations.
            </span>
          )}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        {/* Server Info */}
        <div className="p-4 rounded-lg bg-gray-50 dark:bg-gray-800">
          <h4 className="text-sm font-medium mb-2">MCP Server</h4>
          <p className="text-sm font-semibold">{mcpServerName}</p>
          <p className="text-xs text-muted-foreground mt-1">{mcpServerUrl}</p>
        </div>

        {/* Connection Test */}
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Wifi className="h-4 w-4" />
              <Label className="font-medium">Connection Test</Label>
            </div>
            <Button
              variant="outline"
              size="sm"
              onClick={testConnection}
              disabled={connectionTesting}
            >
              {connectionTesting ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  Testing...
                </>
              ) : (
                <>
                  <RefreshCw className="mr-2 h-4 w-4" />
                  Test Connection
                </>
              )}
            </Button>
          </div>
          {connectionResult && (
            <div className={`p-3 rounded-lg ${
              connectionResult.success
                ? 'bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800'
                : 'bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800'
            }`}>
              <div className="flex items-center gap-2">
                {connectionResult.success ? (
                  <CheckCircle className="h-4 w-4 text-green-600" />
                ) : (
                  <XCircle className="h-4 w-4 text-red-600" />
                )}
                <span className={`text-sm font-medium ${
                  connectionResult.success ? 'text-green-700 dark:text-green-300' : 'text-red-700 dark:text-red-300'
                }`}>
                  {connectionResult.success
                    ? `Connected successfully (${connectionResult.latencyMs}ms)`
                    : connectionResult.error || 'Connection failed'
                  }
                </span>
              </div>
            </div>
          )}
        </div>

        {/* Health Check */}
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Heart className="h-4 w-4" />
              <Label className="font-medium">Health Check</Label>
            </div>
            <Button
              variant="outline"
              size="sm"
              onClick={runHealthCheck}
              disabled={healthChecking}
            >
              {healthChecking ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  Checking...
                </>
              ) : (
                <>
                  <RefreshCw className="mr-2 h-4 w-4" />
                  Run Health Check
                </>
              )}
            </Button>
          </div>
          {healthResult && (
            <div className={`p-3 rounded-lg ${
              healthResult.success
                ? 'bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800'
                : 'bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800'
            }`}>
              <div className="flex items-center gap-2">
                {healthResult.success ? (
                  <CheckCircle className="h-4 w-4 text-green-600" />
                ) : (
                  <XCircle className="h-4 w-4 text-red-600" />
                )}
                <span className={`text-sm font-medium ${
                  healthResult.success ? 'text-green-700 dark:text-green-300' : 'text-red-700 dark:text-red-300'
                }`}>
                  {healthResult.success
                    ? 'Health check passed'
                    : healthResult.error || 'Health check failed'
                  }
                </span>
              </div>
              {healthResult.details && (
                <p className="text-xs text-muted-foreground mt-2">{healthResult.details}</p>
              )}
            </div>
          )}
        </div>

        {/* Capability Confirmation */}
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <FileText className="h-4 w-4" />
              <Label className="font-medium">Confirm Capabilities</Label>
            </div>
            {capabilities.length > 0 && (
              <Badge variant={allCapabilitiesConfirmed ? 'default' : 'secondary'}>
                {confirmedCapabilities.length} / {capabilities.length}
              </Badge>
            )}
          </div>
          {capabilities.length > 0 ? (
            <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
              {capabilities.map((capability) => (
                <div
                  key={capability}
                  className="flex items-center space-x-2 p-2 rounded-lg border hover:bg-gray-50 dark:hover:bg-gray-800"
                >
                  <Checkbox
                    id={`cap-${capability}`}
                    checked={confirmedCapabilities.includes(capability)}
                    onCheckedChange={() => toggleCapability(capability)}
                    disabled={!canSubmit}
                  />
                  <Label
                    htmlFor={`cap-${capability}`}
                    className="text-sm cursor-pointer flex-1"
                  >
                    {capability}
                  </Label>
                </div>
              ))}
            </div>
          ) : (
            <p className="text-sm text-muted-foreground">No capabilities defined for this MCP server.</p>
          )}
        </div>

        {/* Notes */}
        <div className="space-y-2">
          <Label htmlFor="notes" className="font-medium">Notes (optional)</Label>
          <Textarea
            id="notes"
            placeholder="Add any additional notes or observations about this MCP server..."
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
            disabled={!canSubmit}
            rows={3}
          />
        </div>

        {/* Error/Success Messages */}
        {submitError && (
          <Alert variant="destructive">
            <AlertTriangle className="h-4 w-4" />
            <AlertDescription>{submitError}</AlertDescription>
          </Alert>
        )}

        {submitSuccess && (
          <Alert className="bg-green-50 dark:bg-green-900/20 border-green-200 dark:border-green-800">
            <CheckCircle className="h-4 w-4 text-green-600" />
            <AlertDescription className="text-green-700 dark:text-green-300">
              Attestation submitted successfully! The MCP server's consensus progress has been updated.
            </AlertDescription>
          </Alert>
        )}

        {/* Submit Button */}
        <Button
          className="w-full"
          onClick={submitAttestation}
          disabled={!canSubmit || submitting}
        >
          {submitting ? (
            <>
              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              Submitting Attestation...
            </>
          ) : (
            <>
              <Send className="mr-2 h-4 w-4" />
              Submit Manual Attestation
            </>
          )}
        </Button>

        {!canSubmit && (
          <p className="text-xs text-center text-muted-foreground">
            You need admin or manager permissions to submit attestations.
          </p>
        )}
      </CardContent>
    </Card>
  );
}

'use client';

import { useState, useEffect } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Progress } from '@/components/ui/progress';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import {
  Users,
  Shield,
  CheckCircle,
  AlertTriangle,
  Clock,
  Target,
  UserCheck,
  Award,
  TrendingUp,
  Info,
} from 'lucide-react';
import { api } from '@/lib/api';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';

interface ConsensusProgressProps {
  mcpServerId: string;
  currentAgents?: number;
  requiredAgents?: number;  // Default: 3
  currentOwners?: number;
  requiredOwners?: number;  // Default: 2
  confidenceScore?: number;
  requiredScore?: number;   // Default: 60
  status?: string;
  isVerified?: boolean;
}

interface AttestationStats {
  totalAttestations: number;
  validAttestations: number;
  uniqueAgents: number;
  uniqueOwners: number;
  averageConfidence: number;
  lastAttestedAt?: string;
  attestingAgents: Array<{
    agentId: string;
    agentName: string;
    attestedAt: string;
    healthCheckPassed: boolean;
  }>;
}

// Default thresholds for multi-agent consensus
const DEFAULT_REQUIRED_AGENTS = 3;
const DEFAULT_REQUIRED_OWNERS = 2;
const DEFAULT_REQUIRED_SCORE = 60;

export function ConsensusProgress({
  mcpServerId,
  currentAgents = 0,
  requiredAgents = DEFAULT_REQUIRED_AGENTS,
  currentOwners = 0,
  requiredOwners = DEFAULT_REQUIRED_OWNERS,
  confidenceScore = 0,
  requiredScore = DEFAULT_REQUIRED_SCORE,
  status = 'pending',
  isVerified = false,
}: ConsensusProgressProps) {
  const [stats, setStats] = useState<AttestationStats | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchStats = async () => {
      setLoading(true);
      try {
        // Get attestations for this MCP server
        const attestations = await api.getMCPServerAttestations(mcpServerId);

        // Calculate stats from attestations
        const validAttestations = attestations.filter((a: any) => a.isValid);
        const uniqueAgentIds = new Set(validAttestations.map((a: any) => a.agentId).filter(Boolean));
        const uniqueOwnerIds = new Set(validAttestations.map((a: any) => a.agentOwnerId).filter(Boolean));

        // Calculate average confidence from health checks
        const healthPassedCount = validAttestations.filter((a: any) => a.healthCheckPassed).length;
        const avgConfidence = validAttestations.length > 0
          ? (healthPassedCount / validAttestations.length) * 100
          : 0;

        const latestAttestation = validAttestations.length > 0
          ? validAttestations.reduce((latest: any, current: any) =>
              new Date(current.verifiedAt) > new Date(latest.verifiedAt) ? current : latest
            )
          : null;

        setStats({
          totalAttestations: attestations.length,
          validAttestations: validAttestations.length,
          uniqueAgents: uniqueAgentIds.size,
          uniqueOwners: uniqueOwnerIds.size,
          averageConfidence: avgConfidence,
          lastAttestedAt: latestAttestation?.verifiedAt,
          attestingAgents: validAttestations.slice(0, 5).map((a: any) => ({
            agentId: a.agentId,
            agentName: a.agentName,
            attestedAt: a.verifiedAt,
            healthCheckPassed: a.healthCheckPassed,
          })),
        });
      } catch (err) {
        console.error('Failed to fetch attestation stats:', err);
        // Use prop values as fallback
        setStats({
          totalAttestations: currentAgents,
          validAttestations: currentAgents,
          uniqueAgents: currentAgents,
          uniqueOwners: currentOwners,
          averageConfidence: confidenceScore,
          lastAttestedAt: undefined,
          attestingAgents: [],
        });
      } finally {
        setLoading(false);
      }
    };

    fetchStats();
  }, [mcpServerId, currentAgents, currentOwners, confidenceScore]);

  // Use fetched stats or fall back to props
  const agentCount = stats?.uniqueAgents ?? currentAgents;
  const ownerCount = stats?.uniqueOwners ?? currentOwners;
  const avgConfidence = stats?.averageConfidence ?? confidenceScore;

  // Calculate progress percentages
  const agentProgress = Math.min(100, (agentCount / requiredAgents) * 100);
  const ownerProgress = Math.min(100, (ownerCount / requiredOwners) * 100);
  const confidenceProgress = Math.min(100, (avgConfidence / requiredScore) * 100);

  // Check if each threshold is met
  const agentsMet = agentCount >= requiredAgents;
  const ownersMet = ownerCount >= requiredOwners;
  const confidenceMet = avgConfidence >= requiredScore;
  const allMet = agentsMet && ownersMet && confidenceMet;

  // Calculate what's still needed
  const agentsNeeded = Math.max(0, requiredAgents - agentCount);
  const ownersNeeded = Math.max(0, requiredOwners - ownerCount);
  const confidenceNeeded = Math.max(0, requiredScore - avgConfidence);

  // Estimate time to verification based on attestation velocity
  const estimatedDays = agentsNeeded > 0 ? Math.ceil(agentsNeeded * 2) : 0; // Rough estimate

  const getStatusBadge = () => {
    if (isVerified || status === 'verified') {
      return <Badge className="bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400">Verified</Badge>;
    }
    if (status === 'suspended') {
      return <Badge className="bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400">Suspended</Badge>;
    }
    if (allMet) {
      return <Badge className="bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400">Ready for Verification</Badge>;
    }
    return <Badge className="bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400">Pending Consensus</Badge>;
  };

  const getProgressColor = (met: boolean, progress: number) => {
    if (met) return 'bg-green-600';
    if (progress >= 66) return 'bg-yellow-600';
    return 'bg-orange-600';
  };

  if (loading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Target className="h-5 w-5" />
            Consensus Progress
          </CardTitle>
          <CardDescription>Loading attestation data...</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <Skeleton className="h-20 w-full" />
          <Skeleton className="h-16 w-full" />
          <Skeleton className="h-16 w-full" />
          <Skeleton className="h-16 w-full" />
        </CardContent>
      </Card>
    );
  }

  return (
    <TooltipProvider>
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Target className="h-5 w-5 text-blue-600" />
              <CardTitle>Multi-Agent Consensus</CardTitle>
            </div>
            {getStatusBadge()}
          </div>
          <CardDescription>
            Progress toward automated verification through agent attestations
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          {/* Overall Progress Summary */}
          {allMet ? (
            <div className="p-4 rounded-lg bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800">
              <div className="flex items-center gap-3">
                <CheckCircle className="h-6 w-6 text-green-600" />
                <div>
                  <p className="font-semibold text-green-800 dark:text-green-300">
                    Consensus Threshold Met
                  </p>
                  <p className="text-sm text-green-600 dark:text-green-400">
                    All requirements satisfied. MCP server is ready for verification.
                  </p>
                </div>
              </div>
            </div>
          ) : (
            <div className="p-4 rounded-lg bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800">
              <div className="flex items-center gap-3">
                <Clock className="h-6 w-6 text-yellow-600" />
                <div>
                  <p className="font-semibold text-yellow-800 dark:text-yellow-300">
                    Awaiting Additional Attestations
                  </p>
                  <p className="text-sm text-yellow-600 dark:text-yellow-400">
                    {agentsNeeded > 0 && `${agentsNeeded} more agent${agentsNeeded > 1 ? 's' : ''} `}
                    {agentsNeeded > 0 && ownersNeeded > 0 && 'and '}
                    {ownersNeeded > 0 && `${ownersNeeded} more owner${ownersNeeded > 1 ? 's' : ''} `}
                    {(agentsNeeded > 0 || ownersNeeded > 0) && 'needed.'}
                    {confidenceNeeded > 0 && ` ${confidenceNeeded.toFixed(0)}% more confidence required.`}
                    {estimatedDays > 0 && ` Estimated: ${estimatedDays} day${estimatedDays > 1 ? 's' : ''}.`}
                  </p>
                </div>
              </div>
            </div>
          )}

          {/* Factor 1: Unique Agents */}
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Users className="h-4 w-4 text-blue-600" />
                <span className="font-medium">Unique Agents</span>
                <Tooltip>
                  <TooltipTrigger>
                    <Info className="h-3.5 w-3.5 text-muted-foreground" />
                  </TooltipTrigger>
                  <TooltipContent>
                    <p>Number of different agents that have attested this MCP server</p>
                  </TooltipContent>
                </Tooltip>
              </div>
              <div className="flex items-center gap-2">
                <span className={`font-bold ${agentsMet ? 'text-green-600' : 'text-orange-600'}`}>
                  {agentCount} / {requiredAgents}
                </span>
                {agentsMet && <CheckCircle className="h-4 w-4 text-green-600" />}
              </div>
            </div>
            <div className="relative">
              <Progress value={agentProgress} className="h-2" />
              <div
                className={`absolute top-0 left-0 h-full rounded-full ${getProgressColor(agentsMet, agentProgress)}`}
                style={{ width: `${agentProgress}%` }}
              />
            </div>
          </div>

          {/* Factor 2: Unique Owners */}
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <UserCheck className="h-4 w-4 text-purple-600" />
                <span className="font-medium">Unique Owners</span>
                <Tooltip>
                  <TooltipTrigger>
                    <Info className="h-3.5 w-3.5 text-muted-foreground" />
                  </TooltipTrigger>
                  <TooltipContent>
                    <p>Number of different users whose agents have attested</p>
                  </TooltipContent>
                </Tooltip>
              </div>
              <div className="flex items-center gap-2">
                <span className={`font-bold ${ownersMet ? 'text-green-600' : 'text-orange-600'}`}>
                  {ownerCount} / {requiredOwners}
                </span>
                {ownersMet && <CheckCircle className="h-4 w-4 text-green-600" />}
              </div>
            </div>
            <div className="relative">
              <Progress value={ownerProgress} className="h-2" />
              <div
                className={`absolute top-0 left-0 h-full rounded-full ${getProgressColor(ownersMet, ownerProgress)}`}
                style={{ width: `${ownerProgress}%` }}
              />
            </div>
          </div>

          {/* Factor 3: Confidence Score */}
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Award className="h-4 w-4 text-yellow-600" />
                <span className="font-medium">Confidence Score</span>
                <Tooltip>
                  <TooltipTrigger>
                    <Info className="h-3.5 w-3.5 text-muted-foreground" />
                  </TooltipTrigger>
                  <TooltipContent>
                    <p>Average confidence from health checks and connection tests</p>
                  </TooltipContent>
                </Tooltip>
              </div>
              <div className="flex items-center gap-2">
                <span className={`font-bold ${confidenceMet ? 'text-green-600' : 'text-orange-600'}`}>
                  {Math.round(avgConfidence)}% / {requiredScore}%
                </span>
                {confidenceMet && <CheckCircle className="h-4 w-4 text-green-600" />}
              </div>
            </div>
            <div className="relative">
              <Progress value={confidenceProgress} className="h-2" />
              <div
                className={`absolute top-0 left-0 h-full rounded-full ${getProgressColor(confidenceMet, confidenceProgress)}`}
                style={{ width: `${confidenceProgress}%` }}
              />
            </div>
          </div>

          {/* Attesting Agents List */}
          {stats?.attestingAgents && stats.attestingAgents.length > 0 && (
            <div className="pt-4 border-t">
              <h4 className="text-sm font-medium mb-3 flex items-center gap-2">
                <Shield className="h-4 w-4" />
                Recent Attestations
              </h4>
              <div className="space-y-2">
                {stats.attestingAgents.map((agent, index) => (
                  <div
                    key={`${agent.agentId}-${index}`}
                    className="flex items-center justify-between p-2 rounded-lg bg-gray-50 dark:bg-gray-800"
                  >
                    <div className="flex items-center gap-2">
                      <div className={`w-2 h-2 rounded-full ${agent.healthCheckPassed ? 'bg-green-500' : 'bg-yellow-500'}`} />
                      <span className="text-sm font-medium">{agent.agentName || 'Unknown Agent'}</span>
                    </div>
                    <span className="text-xs text-muted-foreground">
                      {new Date(agent.attestedAt).toLocaleDateString()}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Verification Requirements Checklist */}
          <div className="pt-4 border-t">
            <h4 className="text-sm font-medium mb-3 flex items-center gap-2">
              <TrendingUp className="h-4 w-4" />
              Verification Requirements
            </h4>
            <div className="space-y-2">
              <div className={`flex items-center gap-2 text-sm ${agentsMet ? 'text-green-600' : 'text-muted-foreground'}`}>
                {agentsMet ? <CheckCircle className="h-4 w-4" /> : <AlertTriangle className="h-4 w-4" />}
                At least {requiredAgents} unique agents must attest
              </div>
              <div className={`flex items-center gap-2 text-sm ${ownersMet ? 'text-green-600' : 'text-muted-foreground'}`}>
                {ownersMet ? <CheckCircle className="h-4 w-4" /> : <AlertTriangle className="h-4 w-4" />}
                At least {requiredOwners} unique owners (independent verification)
              </div>
              <div className={`flex items-center gap-2 text-sm ${confidenceMet ? 'text-green-600' : 'text-muted-foreground'}`}>
                {confidenceMet ? <CheckCircle className="h-4 w-4" /> : <AlertTriangle className="h-4 w-4" />}
                Minimum {requiredScore}% confidence score
              </div>
            </div>
          </div>
        </CardContent>
      </Card>
    </TooltipProvider>
  );
}

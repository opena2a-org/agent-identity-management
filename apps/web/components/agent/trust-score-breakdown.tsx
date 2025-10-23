'use client';

import { useState, useEffect } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Progress } from '@/components/ui/progress';
import { Skeleton } from '@/components/ui/skeleton';
import {
  Shield,
  Activity,
  CheckCircle,
  AlertTriangle,
  FileCheck,
  Clock,
  TrendingUp,
  ThumbsUp,
  Info
} from 'lucide-react';
import { api } from '@/lib/api';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';

interface TrustScoreBreakdownProps {
  agentId: string;
  userRole?: "admin" | "manager" | "member" | "viewer";
}

interface TrustScoreBreakdown {
  agentId: string;
  agentName: string;
  overall: number;
  factors: {
    verificationStatus: number;
    uptime: number;
    successRate: number;
    securityAlerts: number;
    compliance: number;
    age: number;
    driftDetection: number;
    userFeedback: number;
  };
  weights: {
    verificationStatus: number;
    uptime: number;
    successRate: number;
    securityAlerts: number;
    compliance: number;
    age: number;
    driftDetection: number;
    userFeedback: number;
  };
  contributions: {
    verificationStatus: number;
    uptime: number;
    successRate: number;
    securityAlerts: number;
    compliance: number;
    age: number;
    driftDetection: number;
    userFeedback: number;
  };
  confidence: number;
  calculatedAt: string;
}

// Factor metadata: icons, labels, and descriptions
const factorMetadata = {
  verificationStatus: {
    icon: Shield,
    label: 'Verification Status',
    description: 'Ed25519 signature verification for all actions',
    color: 'text-blue-600',
    bgColor: 'bg-blue-500/10',
  },
  uptime: {
    icon: Activity,
    label: 'Uptime & Availability',
    description: 'Health check responsiveness over time',
    color: 'text-green-600',
    bgColor: 'bg-green-500/10',
  },
  successRate: {
    icon: CheckCircle,
    label: 'Action Success Rate',
    description: 'Percentage of actions that complete successfully',
    color: 'text-emerald-600',
    bgColor: 'bg-emerald-500/10',
  },
  securityAlerts: {
    icon: AlertTriangle,
    label: 'Security Alerts',
    description: 'Active security alerts by severity (critical, high, medium, low)',
    color: 'text-orange-600',
    bgColor: 'bg-orange-500/10',
  },
  compliance: {
    icon: FileCheck,
    label: 'Compliance Score',
    description: 'SOC 2, HIPAA, GDPR adherence',
    color: 'text-purple-600',
    bgColor: 'bg-purple-500/10',
  },
  age: {
    icon: Clock,
    label: 'Age & History',
    description: 'How long agent has been operating successfully (<7d, 7-30d, 30-90d, 90d+)',
    color: 'text-cyan-600',
    bgColor: 'bg-cyan-500/10',
  },
  driftDetection: {
    icon: TrendingUp,
    label: 'Drift Detection',
    description: 'Behavioral pattern changes and anomaly detection',
    color: 'text-indigo-600',
    bgColor: 'bg-indigo-500/10',
  },
  userFeedback: {
    icon: ThumbsUp,
    label: 'User Feedback',
    description: 'Explicit user ratings and feedback',
    color: 'text-pink-600',
    bgColor: 'bg-pink-500/10',
  },
};

export function TrustScoreBreakdown({ agentId, userRole = "viewer" }: TrustScoreBreakdownProps) {
  const [breakdown, setBreakdown] = useState<TrustScoreBreakdown | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchBreakdown = async () => {
      setLoading(true);
      setError(null);
      try {
        const data = await api.getTrustScoreBreakdown(agentId);
        setBreakdown(data);
      } catch (err: any) {
        console.error('Failed to fetch trust score breakdown:', err);
        setError(err.message || 'Failed to load trust score breakdown');
      } finally {
        setLoading(false);
      }
    };

    fetchBreakdown();
  }, [agentId]);

  const getScoreColor = (score: number): string => {
    if (score >= 0.95) return 'text-green-600';
    if (score >= 0.75) return 'text-yellow-600';
    return 'text-red-600';
  };

  const getProgressColor = (score: number): string => {
    if (score >= 0.95) return 'bg-green-600';
    if (score >= 0.75) return 'bg-yellow-600';
    return 'bg-red-600';
  };

  if (loading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Trust Score Breakdown</CardTitle>
          <CardDescription>Loading trust score analysis...</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {[...Array(8)].map((_, i) => (
            <div key={i} className="space-y-2">
              <Skeleton className="h-4 w-48" />
              <Skeleton className="h-3 w-full" />
            </div>
          ))}
        </CardContent>
      </Card>
    );
  }

  if (error || !breakdown) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Trust Score Breakdown</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="text-center py-8 text-muted-foreground">
            <AlertTriangle className="h-12 w-12 mx-auto mb-3 text-yellow-600" />
            <p>{error || 'No trust score data available'}</p>
          </div>
        </CardContent>
      </Card>
    );
  }

  return (
    <TooltipProvider>
      <div className="space-y-4">
        {/* Overall Score Card */}
        <Card>
          <CardHeader>
            <CardTitle>Overall Trust Score</CardTitle>
            <CardDescription>
              Weighted average of 8 behavioral and security factors
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="flex items-center justify-between mb-4">
              <div>
                <div className={`text-4xl font-bold ${getScoreColor(breakdown.overall)}`}>
                  {(breakdown.overall * 100).toFixed(1)}%
                </div>
                <p className="text-sm text-muted-foreground mt-1">
                  Confidence: {(breakdown.confidence * 100).toFixed(1)}%
                </p>
              </div>
              <div className="text-right text-sm text-muted-foreground">
                <p>Last calculated:</p>
                <p>{new Date(breakdown.calculatedAt).toLocaleString()}</p>
              </div>
            </div>
            <Progress
              value={breakdown.overall * 100}
              className="h-3"
            />
          </CardContent>
        </Card>

        {/* Individual Factors */}
        <Card>
          <CardHeader>
            <CardTitle>Factor Breakdown</CardTitle>
            <CardDescription>
              Individual components contributing to the overall trust score
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            {Object.entries(breakdown.factors).map(([key, value]) => {
              const metadata = factorMetadata[key as keyof typeof factorMetadata];
              const Icon = metadata.icon;
              const weight = breakdown.weights[key as keyof typeof breakdown.weights];
              const contribution = breakdown.contributions[key as keyof typeof breakdown.contributions];

              return (
                <div key={key} className="space-y-2">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <div className={`p-2 rounded-lg ${metadata.bgColor}`}>
                        <Icon className={`h-4 w-4 ${metadata.color}`} />
                      </div>
                      <div>
                        <div className="flex items-center gap-2">
                          <span className="font-medium">{metadata.label}</span>
                          <Tooltip>
                            <TooltipTrigger>
                              <Info className="h-3 w-3 text-muted-foreground" />
                            </TooltipTrigger>
                            <TooltipContent>
                              <p className="max-w-xs">{metadata.description}</p>
                            </TooltipContent>
                          </Tooltip>
                        </div>
                        <p className="text-xs text-muted-foreground">
                          Weight: {(weight * 100).toFixed(0)}% • Contribution: {(contribution * 100).toFixed(1)}%
                        </p>
                      </div>
                    </div>
                    <div className={`text-lg font-semibold ${getScoreColor(value)}`}>
                      {(value * 100).toFixed(1)}%
                    </div>
                  </div>
                  <div className="relative">
                    <Progress
                      value={value * 100}
                      className="h-2"
                    />
                  </div>
                </div>
              );
            })}
          </CardContent>
        </Card>
      </div>
    </TooltipProvider>
  );
}

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
  Box,
  Info,
  History
} from 'lucide-react';
import { api } from '@/lib/api';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip as RechartsTooltip, ResponsiveContainer, Legend } from 'recharts';

interface TrustScoreBreakdownProps {
  agentId: string;
  userRole?: "admin" | "manager" | "member" | "viewer";
  onTrustScoreUpdate?: (trustScore: number) => void;
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
    executionIsolation: number;
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
    executionIsolation: number;
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
    executionIsolation: number;
  };
  confidence: number;
  calculatedAt: string;
}

interface TrustScoreHistoryEntry {
  timestamp: string;
  trustScore: number;
  reason: string;
  changedBy: string;
}

interface TrustScoreHistory {
  agentId: string;
  history: TrustScoreHistoryEntry[];
}

// Factor metadata: icons, labels, and descriptions
const factorMetadata = {
  verificationStatus: {
    icon: Shield,
    label: 'Verification status',
    description: 'Ed25519 signature verification for all actions',
    color: 'text-brand-text',
    bgColor: 'bg-brand-soft',
  },
  uptime: {
    icon: Activity,
    label: 'Uptime & availability',
    description: 'Health check responsiveness over time',
    color: 'text-success-text',
    bgColor: 'bg-success-fill',
  },
  successRate: {
    icon: CheckCircle,
    label: 'Action success rate',
    description: 'Percentage of actions that complete successfully',
    color: 'text-success-text',
    bgColor: 'bg-success-fill',
  },
  securityAlerts: {
    icon: AlertTriangle,
    label: 'Security alerts',
    description: 'Active security alerts by severity (critical, high, medium, low)',
    color: 'text-warning-text',
    bgColor: 'bg-warning-fill',
  },
  compliance: {
    icon: FileCheck,
    label: 'Compliance score',
    description: 'SOC 2, HIPAA, GDPR adherence',
    color: 'text-brand-text',
    bgColor: 'bg-brand-soft',
  },
  age: {
    icon: Clock,
    label: 'Age & history',
    description: 'How long agent has been operating successfully (<7d, 7-30d, 30-90d, 90d+)',
    color: 'text-ink-secondary',
    bgColor: 'bg-glass-inset-gray',
  },
  driftDetection: {
    icon: TrendingUp,
    label: 'Drift detection',
    description: 'Behavioral pattern changes and anomaly detection',
    color: 'text-brand-text',
    bgColor: 'bg-brand-soft',
  },
  userFeedback: {
    icon: ThumbsUp,
    label: 'User feedback',
    description: 'Explicit user ratings and feedback',
    color: 'text-ink-secondary',
    bgColor: 'bg-glass-inset-gray',
  },
  executionIsolation: {
    icon: Box,
    label: 'Execution isolation',
    description: 'Self-reported runtime isolation posture (sandbox, network, filesystem, process). Defaults to a low baseline until the agent reports it.',
    color: 'text-brand-text',
    bgColor: 'bg-brand-soft',
  },
};

// Fallback rendering metadata for any factor key the backend emits that is not
// yet in factorMetadata. Prevents a newly added trust factor from crashing the
// entire breakdown tab (regression guard: the 9th factor "executionIsolation"
// shipped server-side before this map was updated).
const fallbackFactorMetadata = {
  icon: Shield,
  label: 'Trust factor',
  description: 'Contributing trust factor',
  color: 'text-ink-secondary',
  bgColor: 'bg-glass-inset-gray',
} as const;

export function TrustScoreBreakdown({ agentId, userRole = "viewer", onTrustScoreUpdate }: TrustScoreBreakdownProps) {
  const [breakdown, setBreakdown] = useState<TrustScoreBreakdown | null>(null);
  const [history, setHistory] = useState<TrustScoreHistory | null>(null);
  const [loading, setLoading] = useState(true);
  const [historyLoading, setHistoryLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [historyError, setHistoryError] = useState<string | null>(null);

  useEffect(() => {
    const fetchBreakdown = async () => {
      setLoading(true);
      setError(null);
      try {
        const data = await api.getTrustScoreBreakdown(agentId);
        setBreakdown(data);
        // Notify parent of the fresh trust score for consistency
        if (onTrustScoreUpdate && data?.overall !== undefined) {
          onTrustScoreUpdate(data.overall);
        }
      } catch (err: any) {
        console.error('Failed to fetch trust score breakdown:', err);
        setError(err.message || 'Failed to load trust score breakdown');
      } finally {
        setLoading(false);
      }
    };

    const fetchHistory = async () => {
      setHistoryLoading(true);
      setHistoryError(null);
      try {
        const data = await api.getAgentTrustScoreHistory(agentId);
        setHistory(data);
      } catch (err: any) {
        console.error('Failed to fetch trust score history:', err);
        setHistoryError(err.message || 'Failed to load trust score history');
      } finally {
        setHistoryLoading(false);
      }
    };

    fetchBreakdown();
    fetchHistory();
  }, [agentId]);

  const getScoreColor = (score: number): string => {
    if (score >= 0.95) return 'text-success-text';
    if (score >= 0.75) return 'text-warning-text';
    return 'text-danger-text';
  };

  const getProgressColor = (score: number): string => {
    if (score >= 0.95) return 'bg-success';
    if (score >= 0.75) return 'bg-warning';
    return 'bg-danger';
  };

  if (loading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Trust score breakdown</CardTitle>
          <CardDescription>Loading trust score analysis...</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {[...Array(9)].map((_, i) => (
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
          <CardTitle>Trust score breakdown</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="text-center py-8 text-ink-secondary">
            <AlertTriangle className="h-12 w-12 mx-auto mb-3 text-warning-text" />
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
            <CardTitle>Overall trust score</CardTitle>
            <CardDescription>
              Weighted average of 9 behavioral and security factors
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="flex items-center justify-between mb-4">
              <div>
                <div className={`text-4xl font-bold ${getScoreColor(breakdown.overall)}`}>
                  {(breakdown.overall * 100).toFixed(1)}%
                </div>
                <p className="text-sm text-ink-secondary mt-1">
                  Confidence: {(breakdown.confidence * 100).toFixed(1)}%
                </p>
              </div>
              <div className="text-right text-sm text-ink-secondary">
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
            <CardTitle>Factor breakdown</CardTitle>
            <CardDescription>
              Individual components contributing to the overall trust score
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            {Object.entries(breakdown.factors).map(([key, value]) => {
              const metadata = factorMetadata[key as keyof typeof factorMetadata] ?? fallbackFactorMetadata;
              const Icon = metadata.icon;
              const weight = breakdown.weights[key as keyof typeof breakdown.weights];
              const contribution = breakdown.contributions[key as keyof typeof breakdown.contributions];

              return (
                <div key={key} className="group p-4 rounded-panel border border-divider hover:border-brand transition-all">
                  <div className="flex items-start justify-between mb-3">
                    <div className="flex items-start gap-3 flex-1">
                      <div className={`p-2.5 rounded-inset-sm ${metadata.bgColor} transition-transform group-hover:scale-110`}>
                        <Icon className={`h-5 w-5 ${metadata.color}`} />
                      </div>
                      <div className="flex-1">
                        <div className="flex items-center gap-2 mb-1">
                          <span className="font-semibold text-base text-ink">{metadata.label}</span>
                          <Tooltip>
                            <TooltipTrigger>
                              <Info className="h-3.5 w-3.5 text-ink-tertiary hover:text-brand-text transition-colors" />
                            </TooltipTrigger>
                            <TooltipContent side="top" className="max-w-xs">
                              <p>{metadata.description}</p>
                            </TooltipContent>
                          </Tooltip>
                        </div>

                        {/* Visual weight and contribution indicators */}
                        <div className="flex items-center gap-4 mt-2">
                          <div className="flex items-center gap-1.5">
                            <div className="text-xs font-medium text-ink-tertiary">Weight</div>
                            <div className="px-2 py-0.5 rounded-md bg-glass-inset-gray text-xs font-semibold text-ink-body">
                              {(weight * 100).toFixed(0)}%
                            </div>
                          </div>
                          <div className="flex items-center gap-1.5">
                            <div className="text-xs font-medium text-ink-tertiary">Impact</div>
                            <div className="px-2 py-0.5 rounded-md bg-brand-soft text-xs font-semibold text-brand-text">
                              +{(contribution * 100).toFixed(1)}%
                            </div>
                          </div>
                        </div>
                      </div>
                    </div>

                    {/* Score badge */}
                    <div className="flex flex-col items-end ml-4">
                      <div className={`text-2xl font-bold ${getScoreColor(value)}`}>
                        {(value * 100).toFixed(1)}%
                      </div>
                      <div className="text-xs text-ink-tertiary mt-0.5">
                        score
                      </div>
                    </div>
                  </div>

                  {/* Progress bar with gradient */}
                  <div className="relative">
                    <div className="h-2.5 w-full bg-track rounded-full overflow-hidden">
                      <div
                        className={`h-full rounded-full transition-all ${
                          value >= 0.95 ? 'bg-success' :
                          value >= 0.75 ? 'bg-warning' :
                          'bg-danger'
                        }`}
                        style={{ width: `${value * 100}%` }}
                      />
                    </div>
                  </div>
                </div>
              );
            })}
          </CardContent>
        </Card>

        {/* Trust Score History */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <History className="h-5 w-5" />
              Trust score history
            </CardTitle>
            <CardDescription>
              Historical changes in trust score over time
            </CardDescription>
          </CardHeader>
          <CardContent>
            {historyLoading ? (
              <div className="space-y-4">
                <Skeleton className="h-64 w-full" />
              </div>
            ) : historyError || !history || history.history.length === 0 ? (
              <div className="text-center py-12 text-ink-secondary">
                <History className="h-12 w-12 mx-auto mb-3 opacity-50" />
                <p>{historyError || 'No historical data available yet'}</p>
                <p className="text-xs mt-2">Trust score changes will appear here over time</p>
              </div>
            ) : (
              <div className="space-y-4">
                {/* Line Chart */}
                <div className="h-64">
                  <ResponsiveContainer width="100%" height="100%">
                    <LineChart
                      data={history.history.map(entry => ({
                        timestamp: new Date(entry.timestamp).toLocaleDateString(),
                        score: (entry.trustScore * 100).toFixed(1),
                        fullTimestamp: new Date(entry.timestamp).toLocaleString(),
                        reason: entry.reason,
                        changedBy: entry.changedBy,
                      }))}
                      margin={{ top: 5, right: 30, left: 20, bottom: 5 }}
                    >
                      <CartesianGrid strokeDasharray="3 3" stroke="var(--divider)" />
                      <XAxis
                        dataKey="timestamp"
                        stroke="var(--stroke)"
                        tick={{ fill: 'var(--text-tertiary)', fontSize: 11 }}
                      />
                      <YAxis
                        domain={[0, 100]}
                        stroke="var(--stroke)"
                        tick={{ fill: 'var(--text-tertiary)', fontSize: 11 }}
                        label={{ value: 'Trust score (%)', angle: -90, position: 'insideLeft', fill: 'var(--text-tertiary)', fontSize: 11 }}
                      />
                      <RechartsTooltip
                        content={({ active, payload }: { active?: boolean; payload?: any[] }) => {
                          if (active && payload && payload.length) {
                            const data = payload[0].payload;
                            return (
                              <div className="glass p-3 text-ink">
                                <p className="font-semibold">{data.fullTimestamp}</p>
                                <p className="text-sm mt-1">
                                  Score: <span className="font-semibold">{data.score}%</span>
                                </p>
                                <p className="text-xs text-ink-secondary mt-1">
                                  Reason: {data.reason}
                                </p>
                                <p className="text-xs text-ink-secondary">
                                  By: {data.changedBy}
                                </p>
                              </div>
                            );
                          }
                          return null;
                        }}
                      />
                      <Legend />
                      <Line
                        type="monotone"
                        dataKey="score"
                        stroke="var(--brand)"
                        strokeWidth={2}
                        dot={{ r: 4 }}
                        activeDot={{ r: 6 }}
                        name="Trust score (%)"
                      />
                    </LineChart>
                  </ResponsiveContainer>
                </div>

                {/* History Table */}
                <div className="border border-divider rounded-panel overflow-hidden">
                  <div className="max-h-96 overflow-y-auto">
                    <table className="w-full">
                      <thead className="bg-glass-inset-gray sticky top-0">
                        <tr>
                          <th className="px-4 py-3 text-left text-xs font-medium text-ink-tertiary uppercase tracking-wider">
                            Date & time
                          </th>
                          <th className="px-4 py-3 text-left text-xs font-medium text-ink-tertiary uppercase tracking-wider">
                            Trust score
                          </th>
                          <th className="px-4 py-3 text-left text-xs font-medium text-ink-tertiary uppercase tracking-wider">
                            Reason
                          </th>
                          <th className="px-4 py-3 text-left text-xs font-medium text-ink-tertiary uppercase tracking-wider">
                            Changed by
                          </th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-divider">
                        {history.history.map((entry, index) => (
                          <tr key={index} className="hover:bg-glass-inset-gray">
                            <td className="px-4 py-3 whitespace-nowrap text-sm text-ink">
                              {new Date(entry.timestamp).toLocaleString()}
                            </td>
                            <td className="px-4 py-3 whitespace-nowrap">
                              <span className={`text-sm font-semibold ${getScoreColor(entry.trustScore)}`}>
                                {(entry.trustScore * 100).toFixed(1)}%
                              </span>
                            </td>
                            <td className="px-4 py-3 text-sm text-ink-secondary">
                              {entry.reason}
                            </td>
                            <td className="px-4 py-3 whitespace-nowrap text-sm text-ink-secondary">
                              {entry.changedBy}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </TooltipProvider>
  );
}

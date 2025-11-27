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
  Info,
  History
} from 'lucide-react';
import { api } from '@/lib/api';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip as RechartsTooltip, ResponsiveContainer, Legend } from 'recharts';

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
          <CardContent className="space-y-4">
            {Object.entries(breakdown.factors).map(([key, value]) => {
              const metadata = factorMetadata[key as keyof typeof factorMetadata];
              const Icon = metadata.icon;
              const weight = breakdown.weights[key as keyof typeof breakdown.weights];
              const contribution = breakdown.contributions[key as keyof typeof breakdown.contributions];

              return (
                <div key={key} className="group p-4 rounded-lg border border-gray-200 dark:border-gray-700 hover:border-blue-500 dark:hover:border-blue-500 transition-all">
                  <div className="flex items-start justify-between mb-3">
                    <div className="flex items-start gap-3 flex-1">
                      <div className={`p-2.5 rounded-lg ${metadata.bgColor} transition-transform group-hover:scale-110`}>
                        <Icon className={`h-5 w-5 ${metadata.color}`} />
                      </div>
                      <div className="flex-1">
                        <div className="flex items-center gap-2 mb-1">
                          <span className="font-semibold text-base">{metadata.label}</span>
                          <Tooltip>
                            <TooltipTrigger>
                              <Info className="h-3.5 w-3.5 text-muted-foreground hover:text-blue-600 transition-colors" />
                            </TooltipTrigger>
                            <TooltipContent side="top" className="max-w-xs">
                              <p>{metadata.description}</p>
                            </TooltipContent>
                          </Tooltip>
                        </div>

                        {/* Visual weight and contribution indicators */}
                        <div className="flex items-center gap-4 mt-2">
                          <div className="flex items-center gap-1.5">
                            <div className="text-xs font-medium text-gray-500 dark:text-gray-400">Weight</div>
                            <div className="px-2 py-0.5 rounded-md bg-gray-100 dark:bg-gray-800 text-xs font-semibold text-gray-700 dark:text-gray-300">
                              {(weight * 100).toFixed(0)}%
                            </div>
                          </div>
                          <div className="flex items-center gap-1.5">
                            <div className="text-xs font-medium text-gray-500 dark:text-gray-400">Impact</div>
                            <div className="px-2 py-0.5 rounded-md bg-blue-50 dark:bg-blue-950 text-xs font-semibold text-blue-700 dark:text-blue-300">
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
                      <div className="text-xs text-muted-foreground mt-0.5">
                        score
                      </div>
                    </div>
                  </div>

                  {/* Progress bar with gradient */}
                  <div className="relative">
                    <div className="h-2.5 w-full bg-gray-100 dark:bg-gray-800 rounded-full overflow-hidden">
                      <div
                        className={`h-full rounded-full transition-all ${
                          value >= 0.95 ? 'bg-gradient-to-r from-green-500 to-green-600' :
                          value >= 0.75 ? 'bg-gradient-to-r from-yellow-500 to-yellow-600' :
                          'bg-gradient-to-r from-red-500 to-red-600'
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
              Trust Score History
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
              <div className="text-center py-12 text-muted-foreground">
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
                      <CartesianGrid strokeDasharray="3 3" className="stroke-gray-200 dark:stroke-gray-700" />
                      <XAxis
                        dataKey="timestamp"
                        className="text-xs text-muted-foreground"
                      />
                      <YAxis
                        domain={[0, 100]}
                        className="text-xs text-muted-foreground"
                        label={{ value: 'Trust Score (%)', angle: -90, position: 'insideLeft' }}
                      />
                      <RechartsTooltip
                        content={({ active, payload }) => {
                          if (active && payload && payload.length) {
                            const data = payload[0].payload;
                            return (
                              <div className="bg-white dark:bg-gray-800 p-3 border border-gray-200 dark:border-gray-700 rounded-lg shadow-lg">
                                <p className="font-semibold">{data.fullTimestamp}</p>
                                <p className="text-sm mt-1">
                                  Score: <span className="font-semibold">{data.score}%</span>
                                </p>
                                <p className="text-xs text-muted-foreground mt-1">
                                  Reason: {data.reason}
                                </p>
                                <p className="text-xs text-muted-foreground">
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
                        stroke="#3b82f6"
                        strokeWidth={2}
                        dot={{ r: 4 }}
                        activeDot={{ r: 6 }}
                        name="Trust Score (%)"
                      />
                    </LineChart>
                  </ResponsiveContainer>
                </div>

                {/* History Table */}
                <div className="border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden">
                  <div className="max-h-96 overflow-y-auto">
                    <table className="w-full">
                      <thead className="bg-gray-50 dark:bg-gray-800 sticky top-0">
                        <tr>
                          <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                            Date & Time
                          </th>
                          <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                            Trust Score
                          </th>
                          <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                            Reason
                          </th>
                          <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                            Changed By
                          </th>
                        </tr>
                      </thead>
                      <tbody className="bg-white dark:bg-gray-900 divide-y divide-gray-200 dark:divide-gray-700">
                        {history.history.map((entry, index) => (
                          <tr key={index} className="hover:bg-gray-50 dark:hover:bg-gray-800">
                            <td className="px-4 py-3 whitespace-nowrap text-sm text-gray-900 dark:text-gray-100">
                              {new Date(entry.timestamp).toLocaleString()}
                            </td>
                            <td className="px-4 py-3 whitespace-nowrap">
                              <span className={`text-sm font-semibold ${getScoreColor(entry.trustScore)}`}>
                                {(entry.trustScore * 100).toFixed(1)}%
                              </span>
                            </td>
                            <td className="px-4 py-3 text-sm text-gray-500 dark:text-gray-400">
                              {entry.reason}
                            </td>
                            <td className="px-4 py-3 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
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

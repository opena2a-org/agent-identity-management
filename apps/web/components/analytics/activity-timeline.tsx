'use client';

import { useState, useEffect } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { Badge } from '@/components/ui/badge';
import {
  Activity,
  CheckCircle,
  XCircle,
  Clock,
  AlertCircle,
  RefreshCw,
  TrendingUp,
  ExternalLink
} from 'lucide-react';
import { api } from '@/lib/api';
import { formatDistanceToNow } from 'date-fns';
import Link from 'next/link';

interface ActivityTimelineProps {
  defaultLimit?: number;
}

interface AgentActivity {
  id: string;
  agentId: string;
  agentName: string;
  action: string;
  status: "success" | "failure" | "pending";
  timestamp: string;
  details?: string;
}

interface ActivityData {
  activities: AgentActivity[];
  summary: {
    totalActivities: number;
    successCount: number;
    failureCount: number;
    successRate: number;
  };
}

export function ActivityTimeline({ defaultLimit = 50 }: ActivityTimelineProps) {
  const [data, setData] = useState<ActivityData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [limit, setLimit] = useState<number>(defaultLimit);
  const [refreshing, setRefreshing] = useState(false);
  const [refreshTrigger, setRefreshTrigger] = useState(0);

  useEffect(() => {
    const fetchActivity = async () => {
      setLoading(true);
      setError(null);
      try {
        const activityData = await api.getAgentActivity(limit);
        setData(activityData);
      } catch (err: any) {
        console.error('Failed to fetch agent activity:', err);
        setError(err.message || 'Failed to load agent activity');
      } finally {
        setLoading(false);
        setRefreshing(false);
      }
    };

    fetchActivity();
  }, [limit, refreshTrigger]);

  const handleRefresh = () => {
    setRefreshing(true);
    setRefreshTrigger(prev => prev + 1);
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'success':
        return <CheckCircle className="h-5 w-5 text-success-text" />;
      case 'failure':
        return <XCircle className="h-5 w-5 text-danger-text" />;
      case 'pending':
        return <Clock className="h-5 w-5 text-warning-text" />;
      default:
        return <Activity className="h-5 w-5 text-ink-secondary" />;
    }
  };

  const getStatusBadge = (status: string) => {
    const variants = {
      success: 'bg-success-fill text-success-text border-success-border',
      failure: 'bg-danger-fill text-danger-text border-danger-border',
      pending: 'bg-warning-fill text-warning-text border-warning-border',
    };

    return (
      <Badge
        variant="outline"
        className={variants[status as keyof typeof variants] || 'bg-glass-inset-gray text-ink-body border-stroke'}
      >
        {status}
      </Badge>
    );
  };

  if (loading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Agent activity timeline</CardTitle>
          <CardDescription>Loading activity data...</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {[...Array(5)].map((_, i) => (
            <div key={i} className="flex gap-4">
              <Skeleton className="h-12 w-12 rounded-full" />
              <div className="flex-1 space-y-2">
                <Skeleton className="h-4 w-3/4" />
                <Skeleton className="h-3 w-1/2" />
              </div>
            </div>
          ))}
        </CardContent>
      </Card>
    );
  }

  if (error || !data) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Agent activity timeline</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="text-center py-8 text-ink-secondary">
            <AlertCircle className="h-12 w-12 mx-auto mb-3 text-warning-text" />
            <p>{error || 'No activity data available'}</p>
            <Button onClick={handleRefresh} className="mt-4" variant="outline">
              <RefreshCw className="h-4 w-4 mr-2" />
              Try again
            </Button>
          </div>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h2 className="text-2xl font-semibold tracking-[-0.02em] text-ink">
          Agent activity timeline
        </h2>
        <Button
          variant="outline"
          size="sm"
          onClick={handleRefresh}
          disabled={refreshing}
        >
          {refreshing ? (
            <>
              <RefreshCw className="h-4 w-4 mr-2 animate-spin" />
              Refreshing...
            </>
          ) : (
            <>
              <RefreshCw className="h-4 w-4 mr-2" />
              Refresh
            </>
          )}
        </Button>
      </div>

      {/* Summary Stats Grid */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-6">
        {/* Total Activities */}
        <div className="glass p-6">
          <div className="flex items-center">
            <div className="flex-shrink-0">
              <Activity className="h-6 w-6 text-ink-tertiary" />
            </div>
            <div className="ml-5 w-0 flex-1">
              <dl>
                <dt className="text-sm font-medium text-ink-secondary truncate">
                  Total activities
                </dt>
                <dd className="flex items-baseline">
                  <div className="text-2xl font-semibold text-ink">
                    {data.summary.totalActivities}
                  </div>
                </dd>
              </dl>
            </div>
          </div>
        </div>

        {/* Successful */}
        <div className="glass p-6">
          <div className="flex items-center">
            <div className="flex-shrink-0">
              <CheckCircle className="h-6 w-6 text-success-text" />
            </div>
            <div className="ml-5 w-0 flex-1">
              <dl>
                <dt className="text-sm font-medium text-ink-secondary truncate">
                  Successful
                </dt>
                <dd className="flex items-baseline">
                  <div className="text-2xl font-semibold text-success-text">
                    {data.summary.successCount}
                  </div>
                </dd>
              </dl>
            </div>
          </div>
        </div>

        {/* Failed */}
        <div className="glass p-6">
          <div className="flex items-center">
            <div className="flex-shrink-0">
              <XCircle className="h-6 w-6 text-danger-text" />
            </div>
            <div className="ml-5 w-0 flex-1">
              <dl>
                <dt className="text-sm font-medium text-ink-secondary truncate">
                  Failed
                </dt>
                <dd className="flex items-baseline">
                  <div className="text-2xl font-semibold text-danger-text">
                    {data.summary.failureCount}
                  </div>
                </dd>
              </dl>
            </div>
          </div>
        </div>

        {/* Success Rate */}
        <div className="glass p-6">
          <div className="flex items-center">
            <div className="flex-shrink-0">
              <TrendingUp className="h-6 w-6 text-ink-tertiary" />
            </div>
            <div className="ml-5 w-0 flex-1">
              <dl>
                <dt className="text-sm font-medium text-ink-secondary truncate">
                  Success rate
                </dt>
                <dd className="flex items-baseline">
                  <div className="text-2xl font-semibold text-ink">
                    {Math.round(data.summary.successRate)}%
                  </div>
                </dd>
              </dl>
            </div>
          </div>
        </div>
      </div>

      {/* Timeline */}
      <div className="glass p-6">
        <h3 className="text-lg font-medium text-ink mb-4">
          Recent activity
        </h3>
        {data.activities.length === 0 ? (
          <div className="text-center py-8 text-ink-secondary">
            <Activity className="h-12 w-12 mx-auto mb-3 text-ink-tertiary" />
            <p>No activities recorded yet</p>
          </div>
        ) : (
          <div className="space-y-3">
            {data.activities.map((activity, index) => (
              <Link
                key={`${activity.agentId}-${activity.timestamp}-${index}`}
                href={`/dashboard/agents/${activity.agentId}`}
                className="flex gap-4 p-4 rounded-inset border border-divider bg-glass-inset-gray hover:bg-brand-soft hover:border-brand-text/30 transition-colors cursor-pointer group"
              >
                {/* Status Icon */}
                <div className="flex-shrink-0 mt-1">
                  {getStatusIcon(activity.status)}
                </div>

                {/* Content */}
                <div className="flex-1 min-w-0">
                  <div className="flex items-start justify-between gap-4 mb-1">
                    <div className="flex items-center gap-2 flex-wrap">
                      <span className="font-semibold text-sm text-ink group-hover:text-brand-text transition-colors">
                        {activity.agentName}
                      </span>
                      <span className="text-sm text-ink-body">
                        {activity.action}
                      </span>
                      {getStatusBadge(activity.status)}
                    </div>
                    <div className="flex items-center gap-2">
                      <span className="text-xs text-ink-secondary whitespace-nowrap">
                        {formatDistanceToNow(new Date(activity.timestamp), {
                          addSuffix: true,
                        })}
                      </span>
                      <ExternalLink className="h-3 w-3 text-ink-tertiary opacity-0 group-hover:opacity-100 transition-opacity" />
                    </div>
                  </div>

                  {activity.details && (
                    <p className="text-xs text-ink-body mt-1">
                      {activity.details}
                    </p>
                  )}

                  <div className="text-xs text-ink-secondary mt-1 font-mono">
                    Agent ID: {activity.agentId}
                  </div>
                </div>
              </Link>
            ))}
          </div>
        )}

        {/* Load More */}
        {data.activities.length > 0 && data.summary.totalActivities > limit && (
          <div className="text-center mt-4">
            <Button
              variant="outline"
              onClick={() => setLimit(limit + 50)}
            >
              Load more activities
            </Button>
          </div>
        )}
      </div>
    </div>
  );
}

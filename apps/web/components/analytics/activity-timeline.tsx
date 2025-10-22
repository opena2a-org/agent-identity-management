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
  RefreshCw
} from 'lucide-react';
import { api } from '@/lib/api';
import { formatDistanceToNow } from 'date-fns';

interface ActivityTimelineProps {
  defaultLimit?: number;
}

interface AgentActivity {
  id: string;
  agent_id: string;
  agent_name: string;
  action: string;
  status: "success" | "failure" | "pending";
  timestamp: string;
  details?: string;
}

interface ActivityData {
  activities: AgentActivity[];
  summary: {
    total_activities: number;
    success_count: number;
    failure_count: number;
    success_rate: number;
  };
}

export function ActivityTimeline({ defaultLimit = 50 }: ActivityTimelineProps) {
  const [data, setData] = useState<ActivityData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [limit, setLimit] = useState<number>(defaultLimit);
  const [refreshing, setRefreshing] = useState(false);

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

  useEffect(() => {
    fetchActivity();
  }, [limit]);

  const handleRefresh = () => {
    setRefreshing(true);
    fetchActivity();
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'success':
        return <CheckCircle className="h-5 w-5 text-green-600" />;
      case 'failure':
        return <XCircle className="h-5 w-5 text-red-600" />;
      case 'pending':
        return <Clock className="h-5 w-5 text-yellow-600" />;
      default:
        return <Activity className="h-5 w-5 text-gray-600" />;
    }
  };

  const getStatusBadge = (status: string) => {
    const variants = {
      success: 'bg-green-100 text-green-800 border-green-200',
      failure: 'bg-red-100 text-red-800 border-red-200',
      pending: 'bg-yellow-100 text-yellow-800 border-yellow-200',
    };

    return (
      <Badge
        variant="outline"
        className={variants[status as keyof typeof variants] || 'bg-gray-100 text-gray-800'}
      >
        {status}
      </Badge>
    );
  };

  if (loading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Agent Activity Timeline</CardTitle>
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
          <CardTitle>Agent Activity Timeline</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="text-center py-8 text-muted-foreground">
            <AlertCircle className="h-12 w-12 mx-auto mb-3 text-yellow-600" />
            <p>{error || 'No activity data available'}</p>
            <Button onClick={handleRefresh} className="mt-4" variant="outline">
              <RefreshCw className="h-4 w-4 mr-2" />
              Try Again
            </Button>
          </div>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div>
            <CardTitle className="flex items-center gap-2">
              <Activity className="h-5 w-5" />
              Agent Activity Timeline
            </CardTitle>
            <CardDescription>
              Recent agent actions and their outcomes
            </CardDescription>
          </div>
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
      </CardHeader>
      <CardContent className="space-y-6">
        {/* Summary Stats */}
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
          <Card className="bg-gray-50">
            <CardContent className="pt-6">
              <div className="text-sm text-muted-foreground mb-1">Total Activities</div>
              <div className="text-2xl font-bold">
                {data.summary.total_activities}
              </div>
            </CardContent>
          </Card>

          <Card className="bg-green-50 border-green-200">
            <CardContent className="pt-6">
              <div className="text-sm text-muted-foreground mb-1">Successful</div>
              <div className="text-2xl font-bold text-green-600">
                {data.summary.success_count}
              </div>
            </CardContent>
          </Card>

          <Card className="bg-red-50 border-red-200">
            <CardContent className="pt-6">
              <div className="text-sm text-muted-foreground mb-1">Failed</div>
              <div className="text-2xl font-bold text-red-600">
                {data.summary.failure_count}
              </div>
            </CardContent>
          </Card>

          <Card className="bg-blue-50 border-blue-200">
            <CardContent className="pt-6">
              <div className="text-sm text-muted-foreground mb-1">Success Rate</div>
              <div className="text-2xl font-bold text-blue-600">
                {(data.summary.success_rate * 100).toFixed(1)}%
              </div>
            </CardContent>
          </Card>
        </div>

        {/* Timeline */}
        <div className="space-y-1">
          <h4 className="text-sm font-semibold mb-4">Recent Activity</h4>
          {data.activities.length === 0 ? (
            <div className="text-center py-8 text-muted-foreground">
              <Activity className="h-12 w-12 mx-auto mb-3 text-gray-400" />
              <p>No activities recorded yet</p>
            </div>
          ) : (
            <div className="space-y-1">
              {data.activities.map((activity, index) => (
                <div
                  key={activity.id}
                  className="flex gap-4 p-4 rounded-lg border bg-white hover:bg-gray-50 transition-colors"
                >
                  {/* Status Icon */}
                  <div className="flex-shrink-0 mt-1">
                    {getStatusIcon(activity.status)}
                  </div>

                  {/* Content */}
                  <div className="flex-1 min-w-0">
                    <div className="flex items-start justify-between gap-4 mb-1">
                      <div className="flex items-center gap-2 flex-wrap">
                        <span className="font-semibold text-sm">
                          {activity.agent_name}
                        </span>
                        <span className="text-sm text-muted-foreground">
                          {activity.action}
                        </span>
                        {getStatusBadge(activity.status)}
                      </div>
                      <span className="text-xs text-muted-foreground whitespace-nowrap">
                        {formatDistanceToNow(new Date(activity.timestamp), {
                          addSuffix: true,
                        })}
                      </span>
                    </div>

                    {activity.details && (
                      <p className="text-xs text-muted-foreground mt-1">
                        {activity.details}
                      </p>
                    )}

                    <div className="text-xs text-muted-foreground mt-1 font-mono">
                      Agent ID: {activity.agent_id.substring(0, 8)}...
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Load More */}
        {data.activities.length > 0 && data.summary.total_activities > limit && (
          <div className="text-center">
            <Button
              variant="outline"
              onClick={() => setLimit(limit + 50)}
            >
              Load More Activities
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

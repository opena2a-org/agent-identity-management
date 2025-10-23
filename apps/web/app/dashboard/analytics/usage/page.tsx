'use client';

import { useState, useEffect } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { Button } from '@/components/ui/button';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Activity,
  Users,
  CheckCircle,
  XCircle,
  Calendar,
  BarChart3,
  TrendingUp
} from 'lucide-react';
import { api } from '@/lib/api';
import { AuthGuard } from '@/components/auth-guard';

interface UsageData {
  period: string;
  api_calls: {
    total: number;
    by_endpoint: Array<{
      endpoint: string;
      count: number;
      percentage: number;
    }>;
    by_day: Array<{
      date: string;
      count: number;
    }>;
  };
  active_users: {
    total: number;
    by_role: Array<{
      role: string;
      count: number;
    }>;
  };
  agent_metrics: {
    total_requests: number;
    successful_requests: number;
    failed_requests: number;
    success_rate: number;
  };
}

export default function UsageStatisticsPage() {
  const [data, setData] = useState<UsageData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [days, setDays] = useState<number>(30);

  useEffect(() => {
    const fetchUsage = async () => {
      setLoading(true);
      setError(null);
      try {
        const usageData = await api.getUsageStatistics(days);
        setData(usageData);
      } catch (err: any) {
        console.error('Failed to fetch usage statistics:', err);
        setError(err.message || 'Failed to load usage statistics');
      } finally {
        setLoading(false);
      }
    };

    fetchUsage();
  }, [days]);

  const formatDate = (dateString: string) => {
    const date = new Date(dateString);
    return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
  };

  if (loading) {
    return (
      <AuthGuard>
        <div className="space-y-6">
          <div className="flex items-center justify-between">
            <div className="space-y-2">
              <Skeleton className="h-9 w-64" />
              <Skeleton className="h-4 w-96" />
            </div>
            <Skeleton className="h-10 w-40" />
          </div>

          <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4">
            {[...Array(4)].map((_, i) => (
              <div
                key={i}
                className="bg-white dark:bg-gray-800 p-6 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm"
              >
                <div className="flex items-center">
                  <div className="flex-shrink-0">
                    <Skeleton className="h-6 w-6 rounded" />
                  </div>
                  <div className="ml-5 flex-1 space-y-2">
                    <Skeleton className="h-4 w-24" />
                    <Skeleton className="h-8 w-16" />
                  </div>
                </div>
              </div>
            ))}
          </div>

          <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700">
            <div className="p-6">
              <Skeleton className="h-96 w-full" />
            </div>
          </div>
        </div>
      </AuthGuard>
    );
  }

  if (error || !data) {
    return (
      <AuthGuard>
        <div className="space-y-6">
          <div className="text-center py-16">
            <Activity className="h-16 w-16 mx-auto mb-4 text-gray-400" />
            <h2 className="text-2xl font-bold mb-2 text-gray-900 dark:text-white">Unable to Load Statistics</h2>
            <p className="text-sm text-gray-500 dark:text-gray-400">{error || 'No usage data available'}</p>
          </div>
        </div>
      </AuthGuard>
    );
  }

  return (
    <AuthGuard>
      <div className="space-y-6">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold text-gray-900 dark:text-white">
              Usage Statistics
            </h1>
            <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
              API usage and user activity metrics for {data.period}
            </p>
          </div>
          <Select value={days.toString()} onValueChange={(v) => setDays(Number(v))}>
            <SelectTrigger className="w-40">
              <Calendar className="h-4 w-4 mr-2" />
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="7">Last 7 days</SelectItem>
              <SelectItem value="14">Last 14 days</SelectItem>
              <SelectItem value="30">Last 30 days</SelectItem>
              <SelectItem value="60">Last 60 days</SelectItem>
              <SelectItem value="90">Last 90 days</SelectItem>
            </SelectContent>
          </Select>
        </div>

        {/* Summary Cards */}
        <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4">
          <div className="bg-white dark:bg-gray-800 p-6 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
            <div className="flex items-center">
              <div className="flex-shrink-0">
                <Activity className="h-6 w-6 text-gray-400" />
              </div>
              <div className="ml-5 w-0 flex-1">
                <dl>
                  <dt className="text-sm font-medium text-gray-500 dark:text-gray-400 truncate">
                    Total API Calls
                  </dt>
                  <dd className="flex items-baseline">
                    <div className="text-2xl font-semibold text-gray-900 dark:text-gray-100">
                      {data?.api_calls?.total?.toLocaleString() || '0'}
                    </div>
                  </dd>
                </dl>
              </div>
            </div>
          </div>

          <div className="bg-white dark:bg-gray-800 p-6 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
            <div className="flex items-center">
              <div className="flex-shrink-0">
                <Users className="h-6 w-6 text-gray-400" />
              </div>
              <div className="ml-5 w-0 flex-1">
                <dl>
                  <dt className="text-sm font-medium text-gray-500 dark:text-gray-400 truncate">
                    Active Users
                  </dt>
                  <dd className="flex items-baseline">
                    <div className="text-2xl font-semibold text-gray-900 dark:text-gray-100">
                      {data?.active_users?.total || 0}
                    </div>
                  </dd>
                </dl>
              </div>
            </div>
          </div>

          <div className="bg-white dark:bg-gray-800 p-6 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
            <div className="flex items-center">
              <div className="flex-shrink-0">
                <CheckCircle className="h-6 w-6 text-gray-400" />
              </div>
              <div className="ml-5 w-0 flex-1">
                <dl>
                  <dt className="text-sm font-medium text-gray-500 dark:text-gray-400 truncate">
                    Success Rate
                  </dt>
                  <dd className="flex items-baseline">
                    <div className="text-2xl font-semibold text-green-600 dark:text-green-400">
                      {((data?.agent_metrics?.success_rate || 0) * 100).toFixed(1)}%
                    </div>
                  </dd>
                </dl>
              </div>
            </div>
          </div>

          <div className="bg-white dark:bg-gray-800 p-6 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
            <div className="flex items-center">
              <div className="flex-shrink-0">
                <XCircle className="h-6 w-6 text-gray-400" />
              </div>
              <div className="ml-5 w-0 flex-1">
                <dl>
                  <dt className="text-sm font-medium text-gray-500 dark:text-gray-400 truncate">
                    Failed Requests
                  </dt>
                  <dd className="flex items-baseline">
                    <div className="text-2xl font-semibold text-red-600 dark:text-red-400">
                      {data?.agent_metrics?.failed_requests?.toLocaleString() || '0'}
                    </div>
                  </dd>
                </dl>
              </div>
            </div>
          </div>
        </div>

        {/* API Calls Over Time */}
        <Card>
          <CardHeader>
            <CardTitle className="text-lg font-medium text-gray-900 dark:text-gray-100">API Calls Over Time</CardTitle>
            <CardDescription className="text-sm text-gray-500 dark:text-gray-400">Daily API request volume</CardDescription>
          </CardHeader>
          <CardContent>
            {data?.api_calls?.by_day && data.api_calls.by_day.length > 0 ? (
              <div className="relative h-64 border border-gray-200 dark:border-gray-700 rounded-lg p-4 bg-gradient-to-b from-white to-gray-50 dark:from-gray-900 dark:to-gray-800">
                {/* Y-axis labels */}
                <div className="absolute left-2 top-4 bottom-4 flex flex-col justify-between text-xs text-gray-500 dark:text-gray-400">
                  {[...Array(5)].map((_, i) => {
                    const max = Math.max(...data.api_calls.by_day.map(d => d.count));
                    const value = Math.ceil(max * (4 - i) / 4);
                    return <span key={i}>{value}</span>;
                  })}
                </div>

                {/* Chart area */}
                <div className="ml-12 mr-4 h-full flex items-end gap-1">
                  {data.api_calls.by_day.map((day, index) => {
                    const maxCount = Math.max(...data.api_calls.by_day.map(d => d.count));
                    const height = (day.count / maxCount) * 100;

                    return (
                      <div key={index} className="flex-1 flex flex-col items-center gap-1 group">
                        <div className="relative w-full">
                          <div
                            className="w-full bg-blue-500 dark:bg-blue-600 rounded-t transition-all hover:bg-blue-600 dark:hover:bg-blue-500"
                            style={{ height: `${height * 2}px` }}
                            title={`${formatDate(day.date)}: ${day.count} calls`}
                          />
                          <div className="absolute -top-6 left-1/2 -translate-x-1/2 text-xs font-semibold opacity-0 group-hover:opacity-100 transition-opacity bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 px-1 rounded shadow border border-gray-200 dark:border-gray-700">
                            {day.count}
                          </div>
                        </div>
                        {index % Math.ceil(data.api_calls.by_day.length / 7) === 0 && (
                          <span className="text-xs text-gray-500 dark:text-gray-400 rotate-45 origin-top-left mt-2">
                            {formatDate(day.date)}
                          </span>
                        )}
                      </div>
                    );
                  })}
                </div>
              </div>
            ) : (
              <div className="flex items-center justify-center h-64 text-center">
                <div>
                  <Activity className="h-12 w-12 mx-auto mb-3 text-gray-400" />
                  <p className="text-sm text-gray-500 dark:text-gray-400">No API call data available for the selected period</p>
                </div>
              </div>
            )}
          </CardContent>
        </Card>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          {/* Top API Endpoints */}
          <Card>
            <CardHeader>
              <CardTitle className="text-lg font-medium text-gray-900 dark:text-gray-100">Top API Endpoints</CardTitle>
              <CardDescription className="text-sm text-gray-500 dark:text-gray-400">Most frequently accessed endpoints</CardDescription>
            </CardHeader>
            <CardContent>
              {data?.api_calls?.by_endpoint && data.api_calls.by_endpoint.length > 0 ? (
                <div className="space-y-4">
                  {data.api_calls.by_endpoint.slice(0, 10).map((endpoint, index) => (
                    <div key={index} className="space-y-2">
                      <div className="flex items-center justify-between text-sm">
                        <span className="font-mono text-xs truncate flex-1">
                          {endpoint.endpoint}
                        </span>
                        <span className="font-semibold ml-2">
                          {endpoint.count.toLocaleString()}
                        </span>
                      </div>
                      <div className="flex items-center gap-2">
                        <div className="flex-1 h-2 bg-gray-200 rounded-full overflow-hidden">
                          <div
                            className="h-full bg-blue-500 rounded-full"
                            style={{ width: `${endpoint.percentage}%` }}
                          />
                        </div>
                        <span className="text-xs text-muted-foreground w-12 text-right">
                          {endpoint.percentage.toFixed(1)}%
                        </span>
                      </div>
                    </div>
                  ))}
                </div>
              ) : (
                <div className="flex items-center justify-center py-8 text-center">
                  <div>
                    <Activity className="h-12 w-12 mx-auto mb-3 text-gray-400" />
                    <p className="text-sm text-gray-500 dark:text-gray-400">No endpoint data available</p>
                  </div>
                </div>
              )}
            </CardContent>
          </Card>

          {/* Active Users by Role */}
          <Card>
            <CardHeader>
              <CardTitle className="text-lg font-medium text-gray-900 dark:text-gray-100">Active Users by Role</CardTitle>
              <CardDescription className="text-sm text-gray-500 dark:text-gray-400">User distribution across roles</CardDescription>
            </CardHeader>
            <CardContent>
              {data?.active_users?.by_role && data.active_users.by_role.length > 0 ? (
                <div className="space-y-4">
                  {data.active_users.by_role.map((role, index) => {
                    const percentage = (role.count / data.active_users.total) * 100;
                    const colors = ['bg-blue-500', 'bg-purple-500', 'bg-green-500', 'bg-orange-500'];

                    return (
                      <div key={index} className="space-y-2">
                        <div className="flex items-center justify-between text-sm">
                          <span className="font-medium capitalize">{role.role}</span>
                          <span className="font-semibold">{role.count} users</span>
                        </div>
                        <div className="flex items-center gap-2">
                          <div className="flex-1 h-2 bg-gray-200 rounded-full overflow-hidden">
                            <div
                              className={`h-full ${colors[index % colors.length]} rounded-full`}
                              style={{ width: `${percentage}%` }}
                            />
                          </div>
                          <span className="text-xs text-muted-foreground w-12 text-right">
                            {percentage.toFixed(1)}%
                          </span>
                        </div>
                      </div>
                    );
                  })}
                </div>
              ) : (
                <div className="flex items-center justify-center py-8 text-center">
                  <div>
                    <Users className="h-12 w-12 mx-auto mb-3 text-gray-400" />
                    <p className="text-sm text-gray-500 dark:text-gray-400">No user role data available</p>
                  </div>
                </div>
              )}
            </CardContent>
          </Card>
        </div>

        {/* Agent Metrics */}
        <Card>
          <CardHeader>
            <CardTitle className="text-lg font-medium text-gray-900 dark:text-gray-100">Agent Request Metrics</CardTitle>
            <CardDescription className="text-sm text-gray-500 dark:text-gray-400">Agent-initiated API requests breakdown</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
              <div className="flex flex-col items-center p-6 bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg">
                <Activity className="h-6 w-6 text-gray-600 dark:text-gray-400 mb-3" />
                <div className="text-2xl font-semibold text-gray-900 dark:text-gray-100">
                  {data?.agent_metrics?.total_requests?.toLocaleString() || '0'}
                </div>
                <div className="text-sm text-gray-500 dark:text-gray-400">Total Requests</div>
              </div>

              <div className="flex flex-col items-center p-6 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg">
                <CheckCircle className="h-6 w-6 text-green-600 dark:text-green-400 mb-3" />
                <div className="text-2xl font-semibold text-green-600 dark:text-green-400">
                  {data?.agent_metrics?.successful_requests?.toLocaleString() || '0'}
                </div>
                <div className="text-sm text-gray-500 dark:text-gray-400">Successful</div>
              </div>

              <div className="flex flex-col items-center p-6 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg">
                <XCircle className="h-6 w-6 text-red-600 dark:text-red-400 mb-3" />
                <div className="text-2xl font-semibold text-red-600 dark:text-red-400">
                  {data?.agent_metrics?.failed_requests?.toLocaleString() || '0'}
                </div>
                <div className="text-sm text-gray-500 dark:text-gray-400">Failed</div>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </AuthGuard>
  );
}

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
        <div className="p-8 space-y-6">
          <div>
            <h1 className="text-3xl font-bold">Usage Statistics</h1>
            <p className="text-muted-foreground">Loading usage data...</p>
          </div>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
            {[...Array(3)].map((_, i) => (
              <Skeleton key={i} className="h-32" />
            ))}
          </div>
          <Skeleton className="h-96" />
        </div>
      </AuthGuard>
    );
  }

  if (error || !data) {
    return (
      <AuthGuard>
        <div className="p-8">
          <div className="text-center py-16">
            <Activity className="h-16 w-16 mx-auto mb-4 text-muted-foreground" />
            <h2 className="text-2xl font-semibold mb-2">Unable to Load Statistics</h2>
            <p className="text-muted-foreground">{error || 'No usage data available'}</p>
          </div>
        </div>
      </AuthGuard>
    );
  }

  return (
    <AuthGuard>
      <div className="p-8 space-y-6">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold flex items-center gap-2">
              <BarChart3 className="h-8 w-8" />
              Usage Statistics
            </h1>
            <p className="text-muted-foreground mt-1">
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
        <div className="grid grid-cols-1 md:grid-cols-4 gap-6">
          <Card className="bg-gradient-to-br from-blue-50 to-blue-100 border-blue-200">
            <CardContent className="pt-6">
              <div className="flex items-center gap-3">
                <Activity className="h-10 w-10 text-blue-600" />
                <div>
                  <div className="text-sm text-muted-foreground">Total API Calls</div>
                  <div className="text-3xl font-bold text-blue-600">
                    {data.api_calls.total.toLocaleString()}
                  </div>
                </div>
              </div>
            </CardContent>
          </Card>

          <Card className="bg-gradient-to-br from-purple-50 to-purple-100 border-purple-200">
            <CardContent className="pt-6">
              <div className="flex items-center gap-3">
                <Users className="h-10 w-10 text-purple-600" />
                <div>
                  <div className="text-sm text-muted-foreground">Active Users</div>
                  <div className="text-3xl font-bold text-purple-600">
                    {data.active_users.total}
                  </div>
                </div>
              </div>
            </CardContent>
          </Card>

          <Card className="bg-gradient-to-br from-green-50 to-green-100 border-green-200">
            <CardContent className="pt-6">
              <div className="flex items-center gap-3">
                <CheckCircle className="h-10 w-10 text-green-600" />
                <div>
                  <div className="text-sm text-muted-foreground">Success Rate</div>
                  <div className="text-3xl font-bold text-green-600">
                    {(data.agent_metrics.success_rate * 100).toFixed(1)}%
                  </div>
                </div>
              </div>
            </CardContent>
          </Card>

          <Card className="bg-gradient-to-br from-orange-50 to-orange-100 border-orange-200">
            <CardContent className="pt-6">
              <div className="flex items-center gap-3">
                <XCircle className="h-10 w-10 text-orange-600" />
                <div>
                  <div className="text-sm text-muted-foreground">Failed Requests</div>
                  <div className="text-3xl font-bold text-orange-600">
                    {data.agent_metrics.failed_requests.toLocaleString()}
                  </div>
                </div>
              </div>
            </CardContent>
          </Card>
        </div>

        {/* API Calls Over Time */}
        <Card>
          <CardHeader>
            <CardTitle>API Calls Over Time</CardTitle>
            <CardDescription>Daily API request volume</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="relative h-64 border rounded-lg p-4 bg-gradient-to-b from-white to-gray-50">
              {/* Y-axis labels */}
              <div className="absolute left-2 top-4 bottom-4 flex flex-col justify-between text-xs text-muted-foreground">
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
                          className="w-full bg-blue-500 rounded-t transition-all hover:bg-blue-600"
                          style={{ height: `${height * 2}px` }}
                          title={`${formatDate(day.date)}: ${day.count} calls`}
                        />
                        <div className="absolute -top-6 left-1/2 -translate-x-1/2 text-xs font-semibold opacity-0 group-hover:opacity-100 transition-opacity bg-white px-1 rounded shadow">
                          {day.count}
                        </div>
                      </div>
                      {index % Math.ceil(data.api_calls.by_day.length / 7) === 0 && (
                        <span className="text-xs text-muted-foreground rotate-45 origin-top-left mt-2">
                          {formatDate(day.date)}
                        </span>
                      )}
                    </div>
                  );
                })}
              </div>
            </div>
          </CardContent>
        </Card>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          {/* Top API Endpoints */}
          <Card>
            <CardHeader>
              <CardTitle>Top API Endpoints</CardTitle>
              <CardDescription>Most frequently accessed endpoints</CardDescription>
            </CardHeader>
            <CardContent>
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
            </CardContent>
          </Card>

          {/* Active Users by Role */}
          <Card>
            <CardHeader>
              <CardTitle>Active Users by Role</CardTitle>
              <CardDescription>User distribution across roles</CardDescription>
            </CardHeader>
            <CardContent>
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
            </CardContent>
          </Card>
        </div>

        {/* Agent Metrics */}
        <Card>
          <CardHeader>
            <CardTitle>Agent Request Metrics</CardTitle>
            <CardDescription>Agent-initiated API requests breakdown</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
              <div className="flex flex-col items-center p-6 bg-gray-50 border rounded-lg">
                <Activity className="h-12 w-12 text-gray-600 mb-3" />
                <div className="text-3xl font-bold text-gray-600">
                  {data.agent_metrics.total_requests.toLocaleString()}
                </div>
                <div className="text-sm text-muted-foreground">Total Requests</div>
              </div>

              <div className="flex flex-col items-center p-6 bg-green-50 border border-green-200 rounded-lg">
                <CheckCircle className="h-12 w-12 text-green-600 mb-3" />
                <div className="text-3xl font-bold text-green-600">
                  {data.agent_metrics.successful_requests.toLocaleString()}
                </div>
                <div className="text-sm text-muted-foreground">Successful</div>
              </div>

              <div className="flex flex-col items-center p-6 bg-red-50 border border-red-200 rounded-lg">
                <XCircle className="h-12 w-12 text-red-600 mb-3" />
                <div className="text-3xl font-bold text-red-600">
                  {data.agent_metrics.failed_requests.toLocaleString()}
                </div>
                <div className="text-sm text-muted-foreground">Failed</div>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </AuthGuard>
  );
}

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
  apiCalls: number;
  activeAgents: number;
  totalAgents: number;
  dataVolume: number;
  uptime: number;
  generatedAt: string;
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
                      {data?.apiCalls?.toLocaleString() || '0'}
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
                    Active Agents
                  </dt>
                  <dd className="flex items-baseline">
                    <div className="text-2xl font-semibold text-gray-900 dark:text-gray-100">
                      {data?.activeAgents || 0}
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
                    Total Agents
                  </dt>
                  <dd className="flex items-baseline">
                    <div className="text-2xl font-semibold text-green-600 dark:text-green-400">
                      {data?.totalAgents || 0}
                    </div>
                  </dd>
                </dl>
              </div>
            </div>
          </div>

          <div className="bg-white dark:bg-gray-800 p-6 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
            <div className="flex items-center">
              <div className="flex-shrink-0">
                <TrendingUp className="h-6 w-6 text-gray-400" />
              </div>
              <div className="ml-5 w-0 flex-1">
                <dl>
                  <dt className="text-sm font-medium text-gray-500 dark:text-gray-400 truncate">
                    System Uptime
                  </dt>
                  <dd className="flex items-baseline">
                    <div className="text-2xl font-semibold text-blue-600 dark:text-blue-400">
                      {(data?.uptime || 0).toFixed(1)}%
                    </div>
                  </dd>
                </dl>
              </div>
            </div>
          </div>
        </div>

        {/* System Metrics */}
        <Card>
          <CardHeader>
            <CardTitle className="text-lg font-medium text-gray-900 dark:text-gray-100">System Metrics</CardTitle>
            <CardDescription className="text-sm text-gray-500 dark:text-gray-400">Current system usage and performance indicators</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              <div className="flex flex-col items-center p-6 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg">
                <BarChart3 className="h-8 w-8 text-blue-600 dark:text-blue-400 mb-3" />
                <div className="text-3xl font-semibold text-blue-600 dark:text-blue-400">
                  {(data?.dataVolume || 0).toFixed(2)} MB
                </div>
                <div className="text-sm text-gray-500 dark:text-gray-400 mt-1">Data Volume</div>
                <p className="text-xs text-gray-400 dark:text-gray-500 mt-2 text-center">
                  Total data processed across all API calls
                </p>
              </div>

              <div className="flex flex-col items-center p-6 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg">
                <TrendingUp className="h-8 w-8 text-green-600 dark:text-green-400 mb-3" />
                <div className="text-3xl font-semibold text-green-600 dark:text-green-400">
                  {(data?.uptime || 0).toFixed(2)}%
                </div>
                <div className="text-sm text-gray-500 dark:text-gray-400 mt-1">System Uptime</div>
                <p className="text-xs text-gray-400 dark:text-gray-500 mt-2 text-center">
                  Service availability over the selected period
                </p>
              </div>
            </div>

            <div className="mt-6 p-4 bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg">
              <div className="flex items-center justify-between text-sm">
                <span className="text-gray-500 dark:text-gray-400">Last Updated</span>
                <span className="font-medium text-gray-900 dark:text-gray-100">
                  {data?.generatedAt ? new Date(data.generatedAt).toLocaleString() : 'N/A'}
                </span>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </AuthGuard>
  );
}

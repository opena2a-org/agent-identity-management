'use client';

import { useState, useEffect } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { TrendingUp, TrendingDown, Minus, Calendar, Activity } from 'lucide-react';
import { api } from '@/lib/api';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

interface TrustTrendsProps {
  defaultDays?: number;
}

interface TrustTrend {
  date: string;
  avgScore: number;
  agentCount: number;
  scoresByRange: {
    excellent: number;
    good: number;
    fair: number;
    poor: number;
  };
}

interface TrustTrendsData {
  period: string;
  trends: TrustTrend[];
  summary: {
    overallAvg: number;
    trendDirection: "up" | "down" | "stable";
    changePercentage: number;
  };
}

export function TrustTrends({ defaultDays = 30 }: TrustTrendsProps) {
  const [data, setData] = useState<TrustTrendsData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [days, setDays] = useState<number>(defaultDays);

  useEffect(() => {
    const fetchTrends = async () => {
      setLoading(true);
      setError(null);
      try {
        const trendsData = await api.getTrustScoreTrends(days);
        setData(trendsData);
      } catch (err: any) {
        console.error('Failed to fetch trust trends:', err);
        setError(err.message || 'Failed to load trust score trends');
      } finally {
        setLoading(false);
      }
    };

    fetchTrends();
  }, [days]);

  const getTrendIcon = () => {
    if (!data) return null;

    switch (data.summary.trendDirection) {
      case 'up':
        return <TrendingUp className="h-5 w-5 text-green-600" />;
      case 'down':
        return <TrendingDown className="h-5 w-5 text-red-600" />;
      default:
        return <Minus className="h-5 w-5 text-yellow-600" />;
    }
  };

  const getTrendColor = () => {
    if (!data) return 'text-gray-600';

    switch (data.summary.trendDirection) {
      case 'up':
        return 'text-green-600';
      case 'down':
        return 'text-red-600';
      default:
        return 'text-yellow-600';
    }
  };

  const formatDate = (dateString: string) => {
    const date = new Date(dateString);
    return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
  };

  if (loading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Trust Score Trends</CardTitle>
          <CardDescription>Loading trend data...</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <Skeleton className="h-48 w-full" />
          <div className="grid grid-cols-4 gap-4">
            {[...Array(4)].map((_, i) => (
              <Skeleton key={i} className="h-20 w-full" />
            ))}
          </div>
        </CardContent>
      </Card>
    );
  }

  if (error || !data) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Trust Score Trends</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="text-center py-8 text-muted-foreground">
            <p>{error || 'No trend data available'}</p>
          </div>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h2 className="text-2xl font-semibold text-gray-900 dark:text-gray-100">
          Trust Score Trends
        </h2>
        <Select value={days.toString()} onValueChange={(v) => setDays(Number(v))}>
          <SelectTrigger className="w-32">
            <Calendar className="h-4 w-4 mr-2" />
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="7">7 days</SelectItem>
            <SelectItem value="14">14 days</SelectItem>
            <SelectItem value="30">30 days</SelectItem>
            <SelectItem value="60">60 days</SelectItem>
            <SelectItem value="90">90 days</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {/* Summary Stats Grid */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-6">
        {/* Overall Average */}
        <div className="bg-white dark:bg-gray-800 p-6 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
          <div className="flex items-center">
            <div className="flex-shrink-0">
              <TrendingUp className="h-6 w-6 text-gray-400" />
            </div>
            <div className="ml-5 w-0 flex-1">
              <dl>
                <dt className="text-sm font-medium text-gray-500 dark:text-gray-400 truncate">
                  Overall Average
                </dt>
                <dd className="flex items-baseline">
                  <div className="text-2xl font-semibold text-gray-900 dark:text-gray-100">
                    {(data.summary.overallAvg * 100).toFixed(1)}%
                  </div>
                </dd>
              </dl>
            </div>
          </div>
        </div>

        {/* Trend Direction */}
        <div className="bg-white dark:bg-gray-800 p-6 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
          <div className="flex items-center">
            <div className="flex-shrink-0">
              {getTrendIcon()}
            </div>
            <div className="ml-5 w-0 flex-1">
              <dl>
                <dt className="text-sm font-medium text-gray-500 dark:text-gray-400 truncate">
                  Trend
                </dt>
                <dd className="flex items-baseline">
                  <div className={`text-2xl font-semibold ${getTrendColor()}`}>
                    {data.summary.changePercentage > 0 ? '+' : ''}
                    {data.summary.changePercentage.toFixed(1)}%
                  </div>
                </dd>
              </dl>
            </div>
          </div>
        </div>

        {/* Period */}
        <div className="bg-white dark:bg-gray-800 p-6 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
          <div className="flex items-center">
            <div className="flex-shrink-0">
              <Calendar className="h-6 w-6 text-gray-400" />
            </div>
            <div className="ml-5 w-0 flex-1">
              <dl>
                <dt className="text-sm font-medium text-gray-500 dark:text-gray-400 truncate">
                  Period
                </dt>
                <dd className="flex items-baseline">
                  <div className="text-2xl font-semibold text-gray-900 dark:text-gray-100">
                    {days} days
                  </div>
                </dd>
              </dl>
            </div>
          </div>
        </div>

        {/* Data Points */}
        <div className="bg-white dark:bg-gray-800 p-6 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
          <div className="flex items-center">
            <div className="flex-shrink-0">
              <Activity className="h-6 w-6 text-gray-400" />
            </div>
            <div className="ml-5 w-0 flex-1">
              <dl>
                <dt className="text-sm font-medium text-gray-500 dark:text-gray-400 truncate">
                  Data Points
                </dt>
                <dd className="flex items-baseline">
                  <div className="text-2xl font-semibold text-gray-900 dark:text-gray-100">
                    {data.trends.length}
                  </div>
                </dd>
              </dl>
            </div>
          </div>
        </div>
      </div>

      {/* Score Distribution */}
      {data.trends.length > 0 && (
        <div className="bg-white dark:bg-gray-800 p-6 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
          <h3 className="text-lg font-medium text-gray-900 dark:text-gray-100 mb-4">
            Latest Score Distribution
          </h3>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            {/* Excellent */}
            <div className="text-center">
              <div className="text-3xl font-bold text-green-600">
                {data.trends[data.trends.length - 1].scoresByRange.excellent}
              </div>
              <div className="text-sm text-gray-500 dark:text-gray-400 mt-1">
                Excellent
              </div>
              <div className="text-xs text-gray-400 dark:text-gray-500">
                90-100%
              </div>
            </div>

            {/* Good */}
            <div className="text-center">
              <div className="text-3xl font-bold text-blue-600">
                {data.trends[data.trends.length - 1].scoresByRange.good}
              </div>
              <div className="text-sm text-gray-500 dark:text-gray-400 mt-1">
                Good
              </div>
              <div className="text-xs text-gray-400 dark:text-gray-500">
                75-89%
              </div>
            </div>

            {/* Fair */}
            <div className="text-center">
              <div className="text-3xl font-bold text-yellow-600">
                {data.trends[data.trends.length - 1].scoresByRange.fair}
              </div>
              <div className="text-sm text-gray-500 dark:text-gray-400 mt-1">
                Fair
              </div>
              <div className="text-xs text-gray-400 dark:text-gray-500">
                50-74%
              </div>
            </div>

            {/* Poor */}
            <div className="text-center">
              <div className="text-3xl font-bold text-red-600">
                {data.trends[data.trends.length - 1].scoresByRange.poor}
              </div>
              <div className="text-sm text-gray-500 dark:text-gray-400 mt-1">
                Poor
              </div>
              <div className="text-xs text-gray-400 dark:text-gray-500">
                0-49%
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

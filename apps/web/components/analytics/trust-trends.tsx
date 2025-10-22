'use client';

import { useState, useEffect } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { TrendingUp, TrendingDown, Minus, Calendar } from 'lucide-react';
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
  avg_score: number;  // ✅ FIXED: Backend returns avg_score, not avg_trust_score
  agent_count: number;
  scores_by_range: {
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
    overall_avg: number;
    trend_direction: "up" | "down" | "stable";
    change_percentage: number;
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

    switch (data.summary.trend_direction) {
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

    switch (data.summary.trend_direction) {
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
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div>
            <CardTitle className="flex items-center gap-2">
              Trust Score Trends
              {getTrendIcon()}
            </CardTitle>
            <CardDescription>
              Average trust scores over the selected period
            </CardDescription>
          </div>
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
      </CardHeader>
      <CardContent className="space-y-6">
        {/* Summary Cards */}
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
          <Card className="bg-blue-50 border-blue-200">
            <CardContent className="pt-6">
              <div className="text-sm text-muted-foreground mb-1">Overall Average</div>
              <div className="text-3xl font-bold text-blue-600">
                {(data.summary.overall_avg * 100).toFixed(1)}%
              </div>
            </CardContent>
          </Card>

          <Card className={`border-2 ${
            data.summary.trend_direction === 'up' ? 'bg-green-50 border-green-200' :
            data.summary.trend_direction === 'down' ? 'bg-red-50 border-red-200' :
            'bg-yellow-50 border-yellow-200'
          }`}>
            <CardContent className="pt-6">
              <div className="text-sm text-muted-foreground mb-1">Trend</div>
              <div className={`text-3xl font-bold ${getTrendColor()}`}>
                {data.summary.change_percentage > 0 ? '+' : ''}
                {data.summary.change_percentage.toFixed(1)}%
              </div>
            </CardContent>
          </Card>

          <Card className="bg-purple-50 border-purple-200">
            <CardContent className="pt-6">
              <div className="text-sm text-muted-foreground mb-1">Period</div>
              <div className="text-2xl font-bold text-purple-600">
                {days} days
              </div>
            </CardContent>
          </Card>

          <Card className="bg-gray-50 border-gray-200">
            <CardContent className="pt-6">
              <div className="text-sm text-muted-foreground mb-1">Data Points</div>
              <div className="text-2xl font-bold text-gray-600">
                {data.trends.length}
              </div>
            </CardContent>
          </Card>
        </div>

        {/* Simple Line Chart Visualization */}
        <div className="space-y-2">
          <h4 className="text-sm font-semibold">Trust Score Progression</h4>
          <div className="relative h-64 border rounded-lg p-4 bg-gradient-to-b from-white to-gray-50">
            {/* Y-axis labels */}
            <div className="absolute left-2 top-4 bottom-4 flex flex-col justify-between text-xs text-muted-foreground">
              <span>100%</span>
              <span>75%</span>
              <span>50%</span>
              <span>25%</span>
              <span>0%</span>
            </div>

            {/* Chart area */}
            <div className="ml-12 mr-4 h-full flex items-end gap-1">
              {data.trends.map((trend, index) => {
                const height = (trend.avg_score * 100);
                const color = height >= 90 ? 'bg-green-500' :
                             height >= 75 ? 'bg-blue-500' :
                             height >= 50 ? 'bg-yellow-500' :
                             'bg-red-500';

                return (
                  <div key={index} className="flex-1 flex flex-col items-center gap-1 group">
                    <div className="relative w-full">
                      <div
                        className={`w-full ${color} rounded-t transition-all hover:opacity-80`}
                        style={{ height: `${height * 2}px` }}
                        title={`${formatDate(trend.date)}: ${(trend.avg_score * 100).toFixed(1)}%`}
                      />
                      <div className="absolute -top-6 left-1/2 -translate-x-1/2 text-xs font-semibold opacity-0 group-hover:opacity-100 transition-opacity bg-white px-1 rounded shadow">
                        {(trend.avg_score * 100).toFixed(0)}%
                      </div>
                    </div>
                    {index % Math.ceil(data.trends.length / 7) === 0 && (
                      <span className="text-xs text-muted-foreground rotate-45 origin-top-left mt-2">
                        {formatDate(trend.date)}
                      </span>
                    )}
                  </div>
                );
              })}
            </div>
          </div>
        </div>

        {/* Score Distribution */}
        {data.trends.length > 0 && (
          <div className="space-y-2">
            <h4 className="text-sm font-semibold">Latest Score Distribution</h4>
            <div className="grid grid-cols-4 gap-3">
              <div className="flex flex-col items-center p-3 bg-green-50 border border-green-200 rounded">
                <div className="text-2xl font-bold text-green-600">
                  {data.trends[data.trends.length - 1].scores_by_range.excellent}
                </div>
                <div className="text-xs text-muted-foreground text-center">
                  Excellent<br />(90-100%)
                </div>
              </div>
              <div className="flex flex-col items-center p-3 bg-blue-50 border border-blue-200 rounded">
                <div className="text-2xl font-bold text-blue-600">
                  {data.trends[data.trends.length - 1].scores_by_range.good}
                </div>
                <div className="text-xs text-muted-foreground text-center">
                  Good<br />(75-89%)
                </div>
              </div>
              <div className="flex flex-col items-center p-3 bg-yellow-50 border border-yellow-200 rounded">
                <div className="text-2xl font-bold text-yellow-600">
                  {data.trends[data.trends.length - 1].scores_by_range.fair}
                </div>
                <div className="text-xs text-muted-foreground text-center">
                  Fair<br />(50-74%)
                </div>
              </div>
              <div className="flex flex-col items-center p-3 bg-red-50 border border-red-200 rounded">
                <div className="text-2xl font-bold text-red-600">
                  {data.trends[data.trends.length - 1].scores_by_range.poor}
                </div>
                <div className="text-xs text-muted-foreground text-center">
                  Poor<br />(0-49%)
                </div>
              </div>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

"use client";

import { useState, useEffect } from "react";
import {
  Activity,
  CheckCircle,
  XCircle,
  Clock,
  AlertTriangle,
} from "lucide-react";
import { api } from "@/lib/api";
import { formatTime, formatDateTime } from "@/lib/date-utils";
import { Skeleton } from "@/components/ui/skeleton";

interface VerificationEvent {
  id: string;
  agentId: string;
  agentName: string;
  protocol: string;
  verificationType: string;
  status: string;
  confidence: number;
  trustScore: number;
  durationMs: number;
  initiatorType: string;
  initiatorName?: string | null;
  initiatorIp?: string | null;
  startedAt: string;
  completedAt: string | null;
  createdAt: string;
}

interface VerificationStatistics {
  totalVerifications: number;
  successCount: number;
  failedCount: number;
  pendingCount: number;
  timeoutCount: number;
  successRate: number;
  avgDurationMs: number;
  avgConfidence: number;
  avgTrustScore: number;
  verificationsPerMinute: number;
  uniqueAgentsVerified: number;
  protocolDistribution: { [key: string]: number };
  typeDistribution: { [key: string]: number };
  initiatorDistribution: { [key: string]: number };
}

type EventTimeRange = "15min" | "1h" | "24h" | "7d";

export default function MonitoringPage() {
  const [recentEvents, setRecentEvents] = useState<VerificationEvent[]>([]);
  const [statistics, setStatistics] = useState<VerificationStatistics | null>(
    null
  );
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [timeRange, setTimeRange] = useState<"24h" | "7d" | "30d">("24h");
  const [eventTimeRange, setEventTimeRange] = useState<EventTimeRange>("15min");
  const [isPaused, setIsPaused] = useState(false);

  // Map time ranges to minutes
  const getMinutesFromTimeRange = (range: EventTimeRange): number => {
    switch (range) {
      case "15min":
        return 15;
      case "1h":
        return 60;
      case "24h":
        return 1440;
      case "7d":
        return 10080;
      default:
        return 15;
    }
  };

  // Get refresh interval based on time range
  const getRefreshInterval = (range: EventTimeRange): number => {
    switch (range) {
      case "15min":
        return 2000; // 2 seconds
      case "1h":
        return 30000; // 30 seconds
      case "24h":
        return 120000; // 2 minutes
      case "7d":
        return 300000; // 5 minutes
      default:
        return 2000;
    }
  };

  // Fetch recent events (real-time feed)
  const fetchRecentEvents = async () => {
    if (isPaused) return;

    try {
      const minutes = getMinutesFromTimeRange(eventTimeRange);
      const response = await api.getRecentVerificationEvents(minutes);
      setRecentEvents(response.events || []);
    } catch (err: any) {
      console.error("Failed to fetch recent events:", err);
    }
  };

  // Fetch statistics
  const fetchStatistics = async () => {
    try {
      const response = await api.getVerificationStatistics(timeRange);
      setStatistics(response);
      setError(null);
    } catch (err: any) {
      console.error("Failed to fetch statistics:", err);
      setError(err.message || "Failed to load statistics");
    }
  };

  // Initial load
  useEffect(() => {
    const loadData = async () => {
      setLoading(true);
      await Promise.all([fetchRecentEvents(), fetchStatistics()]);
      setLoading(false);
    };
    loadData();
  }, [timeRange, eventTimeRange]);

  // Dynamic polling based on time range selection
  useEffect(() => {
    if (isPaused) return;

    const refreshInterval = getRefreshInterval(eventTimeRange);
    const interval = setInterval(fetchRecentEvents, refreshInterval);
    return () => clearInterval(interval);
  }, [eventTimeRange, isPaused]);

  // Refresh statistics every 30 seconds
  useEffect(() => {
    const interval = setInterval(fetchStatistics, 30000);
    return () => clearInterval(interval);
  }, [timeRange]);

  const getStatusColor = (status: string) => {
    switch (status) {
      case "success":
        return "text-green-600 bg-green-50";
      case "failed":
        return "text-red-600 bg-red-50";
      case "pending":
        return "text-yellow-600 bg-yellow-50";
      case "timeout":
        return "text-orange-600 bg-orange-50";
      default:
        return "text-gray-600 bg-gray-50";
    }
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case "success":
        return <CheckCircle className="h-4 w-4" />;
      case "failed":
        return <XCircle className="h-4 w-4" />;
      case "pending":
        return <Clock className="h-4 w-4" />;
      case "timeout":
        return <AlertTriangle className="h-4 w-4" />;
      default:
        return <Activity className="h-4 w-4" />;
    }
  };

  if (loading) {
    return (
      <div className="space-y-6">
        {/* Header Skeleton */}
        <div className="flex items-center justify-between">
          <div className="space-y-2">
            <Skeleton className="h-9 w-64" />
            <Skeleton className="h-4 w-96" />
          </div>
          <div className="flex items-center gap-2">
            <Skeleton className="h-10 w-32" />
            <Skeleton className="h-10 w-40" />
          </div>
        </div>

        {/* Stats Cards Skeleton */}
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
          {[...Array(8)].map((_, i) => (
            <div
              key={i}
              className="bg-white dark:bg-gray-800 p-6 rounded-lg border border-gray-200 dark:border-gray-700"
            >
              <div className="space-y-2">
                <Skeleton className="h-4 w-32" />
                <Skeleton className="h-8 w-20" />
                <Skeleton className="h-3 w-24" />
              </div>
            </div>
          ))}
        </div>

        {/* Events Table Skeleton */}
        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700">
          <div className="p-6 border-b border-gray-200 dark:border-gray-700">
            <div className="flex items-center justify-between">
              <Skeleton className="h-6 w-48" />
              <div className="flex items-center gap-2">
                <Skeleton className="h-10 w-32" />
                <Skeleton className="h-10 w-24" />
              </div>
            </div>
          </div>
          <div className="p-6">
            <div className="space-y-3">
              {[...Array(5)].map((_, i) => (
                <Skeleton key={i} className="h-20 w-full" />
              ))}
            </div>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">
            Verification Monitoring
          </h1>
          <p className="text-gray-600 mt-1">
            Real-time cryptographic verification analytics
          </p>
        </div>

        {/* Time Range Selector */}
        <div className="flex items-center gap-2">
          <span className="text-sm text-gray-600">Time Range:</span>
          <select
            value={timeRange}
            onChange={(e) =>
              setTimeRange(e.target.value as "24h" | "7d" | "30d")
            }
            className="px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent"
          >
            <option value="24h">Last 24 Hours</option>
            <option value="7d">Last 7 Days</option>
            <option value="30d">Last 30 Days</option>
          </select>
        </div>
      </div>

      {error && (
        <div className="bg-red-50 border border-red-200 rounded-lg p-4">
          <p className="text-red-600">{error}</p>
        </div>
      )}

      {/* Statistics Cards */}
      {statistics && (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
          {/* Total Verifications */}
          <div className="bg-white rounded-lg border border-gray-200 p-6">
            <div className="flex items-center justify-between mb-2">
              <h3 className="text-sm font-medium text-gray-600">
                Total Verifications
              </h3>
              <Activity className="h-5 w-5 text-blue-600" />
            </div>
            <p className="text-3xl font-bold text-gray-900">
              {statistics?.totalVerifications?.toLocaleString() || "0"}
            </p>
            <p className="text-sm text-gray-500 mt-1">
              {statistics?.verificationsPerMinute?.toFixed(2) || "0"} per minute
            </p>
          </div>

          {/* Success Rate */}
          <div className="bg-white rounded-lg border border-gray-200 p-6">
            <div className="flex items-center justify-between mb-2">
              <h3 className="text-sm font-medium text-gray-600">
                Success Rate
              </h3>
              <CheckCircle className="h-5 w-5 text-green-600" />
            </div>
            <p className="text-3xl font-bold text-gray-900">
              {statistics?.successRate?.toFixed(1) || "0"}%
            </p>
            <p className="text-sm text-gray-500 mt-1">
              {statistics?.successCount || 0} /{" "}
              {statistics?.totalVerifications || 0} successful
            </p>
          </div>

          {/* Average Latency */}
          <div className="bg-white rounded-lg border border-gray-200 p-6">
            <div className="flex items-center justify-between mb-2">
              <h3 className="text-sm font-medium text-gray-600">Avg Latency</h3>
              <Clock className="h-5 w-5 text-purple-600" />
            </div>
            <p className="text-3xl font-bold text-gray-900">
              {statistics?.avgDurationMs
                ? Math.round(statistics.avgDurationMs)
                : "0"}
              ms
            </p>
            <p className="text-sm text-gray-500 mt-1">Average duration</p>
          </div>

          {/* Unique Agents */}
          <div className="bg-white rounded-lg border border-gray-200 p-6">
            <div className="flex items-center justify-between mb-2">
              <h3 className="text-sm font-medium text-gray-600">
                Active Agents
              </h3>
              <Activity className="h-5 w-5 text-indigo-600" />
            </div>
            <p className="text-3xl font-bold text-gray-900">
              {statistics?.uniqueAgentsVerified || "0"}
            </p>
            <p className="text-sm text-gray-500 mt-1">Verified agents</p>
          </div>
        </div>
      )}

      {/* Distribution Charts Row */}
      {statistics && (
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          {/* Protocol Distribution */}
          <div className="bg-white rounded-lg border border-gray-200 p-6">
            <h3 className="text-lg font-semibold text-gray-900 mb-4">
              Protocol Distribution
            </h3>
            <div className="space-y-3">
              {Object.entries(statistics.protocolDistribution).map(
                ([protocol, count]) => {
                  const percentage =
                    (count / statistics.totalVerifications) * 100;
                  return (
                    <div key={protocol}>
                      <div className="flex items-center justify-between mb-1">
                        <span className="text-sm font-medium text-gray-700">
                          {protocol}
                        </span>
                        <span className="text-sm text-gray-600">
                          {count} ({percentage.toFixed(1)}%)
                        </span>
                      </div>
                      <div className="w-full bg-gray-200 rounded-full h-2">
                        <div
                          className="bg-blue-600 h-2 rounded-full transition-all duration-300"
                          style={{ width: `${percentage}%` }}
                        ></div>
                      </div>
                    </div>
                  );
                }
              )}
            </div>
          </div>

          {/* Type Distribution */}
          <div className="bg-white rounded-lg border border-gray-200 p-6">
            <h3 className="text-lg font-semibold text-gray-900 mb-4">
              Verification Type
            </h3>
            <div className="space-y-3">
              {Object.entries(statistics.typeDistribution).map(
                ([type, count]) => {
                  const percentage =
                    (count / statistics.totalVerifications) * 100;
                  return (
                    <div key={type}>
                      <div className="flex items-center justify-between mb-1">
                        <span className="text-sm font-medium text-gray-700 capitalize">
                          {type}
                        </span>
                        <span className="text-sm text-gray-600">
                          {count} ({percentage.toFixed(1)}%)
                        </span>
                      </div>
                      <div className="w-full bg-gray-200 rounded-full h-2">
                        <div
                          className="bg-green-600 h-2 rounded-full transition-all duration-300"
                          style={{ width: `${percentage}%` }}
                        ></div>
                      </div>
                    </div>
                  );
                }
              )}
            </div>
          </div>

          {/* Status Breakdown */}
          <div className="bg-white rounded-lg border border-gray-200 p-6">
            <h3 className="text-lg font-semibold text-gray-900 mb-4">
              Status Breakdown
            </h3>
            <div className="space-y-3">
              <div>
                <div className="flex items-center justify-between mb-1">
                  <span className="text-sm font-medium text-green-700">
                    Success
                  </span>
                  <span className="text-sm text-gray-600">
                    {statistics.successCount}
                  </span>
                </div>
                <div className="w-full bg-gray-200 rounded-full h-2">
                  <div
                    className="bg-green-600 h-2 rounded-full"
                    style={{
                      width: `${(statistics.successCount / statistics.totalVerifications) * 100}%`,
                    }}
                  ></div>
                </div>
              </div>
              <div>
                <div className="flex items-center justify-between mb-1">
                  <span className="text-sm font-medium text-red-700">
                    Failed
                  </span>
                  <span className="text-sm text-gray-600">
                    {statistics.failedCount}
                  </span>
                </div>
                <div className="w-full bg-gray-200 rounded-full h-2">
                  <div
                    className="bg-red-600 h-2 rounded-full"
                    style={{
                      width: `${(statistics.failedCount / statistics.totalVerifications) * 100}%`,
                    }}
                  ></div>
                </div>
              </div>
              {statistics.timeoutCount > 0 && (
                <div>
                  <div className="flex items-center justify-between mb-1">
                    <span className="text-sm font-medium text-orange-700">
                      Timeout
                    </span>
                    <span className="text-sm text-gray-600">
                      {statistics.timeoutCount}
                    </span>
                  </div>
                  <div className="w-full bg-gray-200 rounded-full h-2">
                    <div
                      className="bg-orange-600 h-2 rounded-full"
                      style={{
                        width: `${(statistics.timeoutCount / statistics.totalVerifications) * 100}%`,
                      }}
                    ></div>
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Recent Events Feed */}
      <div className="bg-white rounded-lg border border-gray-200">
        <div className="px-6 py-4 border-b border-gray-200">
          <div className="flex items-center justify-between">
            <h2 className="text-xl font-semibold text-gray-900">
              Recent Events
            </h2>

            <div className="flex items-center gap-4">
              {/* Time Range Filter */}
              <div className="flex items-center gap-2">
                <span className="text-sm text-gray-600">Show:</span>
                <div className="flex items-center gap-1 bg-gray-100 rounded-lg p-1">
                  {(["15min", "1h", "24h", "7d"] as EventTimeRange[]).map(
                    (range) => (
                      <button
                        key={range}
                        onClick={() => {
                          setEventTimeRange(range);
                          setIsPaused(false);
                        }}
                        className={`px-3 py-1 text-xs font-medium rounded-md transition-colors ${
                          eventTimeRange === range
                            ? "bg-white text-blue-600 shadow-sm"
                            : "text-gray-600 hover:text-gray-900"
                        }`}
                      >
                        {range === "15min"
                          ? "Live"
                          : range === "1h"
                            ? "1H"
                            : range === "24h"
                              ? "24H"
                              : "7D"}
                      </button>
                    )
                  )}
                </div>
              </div>

              {/* Pause/Resume Button */}
              <button
                onClick={() => setIsPaused(!isPaused)}
                className="text-sm text-gray-600 hover:text-gray-900 flex items-center gap-1"
              >
                {isPaused ? (
                  <>
                    <Activity className="h-4 w-4" />
                    <span>Resume</span>
                  </>
                ) : (
                  <>
                    <div className="h-2 w-2 bg-green-500 rounded-full animate-pulse"></div>
                    <span>Pause</span>
                  </>
                )}
              </button>

              {/* Status Indicator */}
              <div className="text-xs text-gray-500">
                {isPaused ? (
                  <span className="flex items-center gap-1">
                    <span className="h-2 w-2 bg-gray-400 rounded-full"></span>
                    Paused
                  </span>
                ) : (
                  <span>
                    {eventTimeRange === "15min" && "Updates every 2s"}
                    {eventTimeRange === "1h" && "Updates every 30s"}
                    {eventTimeRange === "24h" && "Updates every 2min"}
                    {eventTimeRange === "7d" && "Updates every 5min"}
                  </span>
                )}
              </div>
            </div>
          </div>
        </div>

        <div className="divide-y divide-gray-200">
          {recentEvents.length === 0 ? (
            <div className="px-6 py-12 text-center">
              <Activity className="h-12 w-12 text-gray-400 mx-auto mb-4" />
              <p className="text-gray-600">
                No verification events in the selected time range
              </p>
              <p className="text-sm text-gray-500 mt-1">
                {eventTimeRange === "15min" &&
                  "No events in the last 15 minutes"}
                {eventTimeRange === "1h" && "No events in the last hour"}
                {eventTimeRange === "24h" && "No events in the last 24 hours"}
                {eventTimeRange === "7d" && "No events in the last 7 days"}
              </p>
            </div>
          ) : (
            recentEvents.slice(0, 10).map((event) => (
              <div
                key={event.id}
                className="px-6 py-4 hover:bg-gray-50 transition-colors"
              >
                <div className="flex items-start justify-between">
                  <div className="flex-1">
                    <div className="flex items-center gap-3 mb-2">
                      <span
                        className={`inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs font-medium ${getStatusColor(event.status)}`}
                      >
                        {getStatusIcon(event.status)}
                        {event.status}
                      </span>
                      <span className="text-sm font-medium text-gray-900">
                        {event.agentName}
                      </span>
                      <span className="text-xs text-gray-500">•</span>
                      <span className="text-xs text-gray-600">
                        {event.protocol}
                      </span>
                      <span className="text-xs text-gray-500">•</span>
                      <span className="text-xs text-gray-600 capitalize">
                        {event.verificationType}
                      </span>
                    </div>

                    <div className="flex items-center gap-4 text-sm text-gray-600">
                      <span>Duration: {event.durationMs}ms</span>
                      <span>
                        Confidence: {(event.confidence * 100).toFixed(1)}%
                      </span>
                      <span>Trust: {event.trustScore.toFixed(1)}</span>
                      <span className="capitalize">
                        Initiator: {event.initiatorName || event.initiatorType}
                      </span>
                    </div>
                  </div>

                  <div className="text-right text-sm text-gray-500">
                    {formatTime(event.createdAt)}
                  </div>
                </div>
              </div>
            ))
          )}
        </div>
      </div>
    </div>
  );
}

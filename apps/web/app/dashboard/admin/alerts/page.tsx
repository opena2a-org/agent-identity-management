"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  AlertTriangle,
  Info,
  ShieldAlert,
  CheckCircle2,
  Clock,
  Key,
  TrendingDown,
  GitBranch,
  Check,
} from "lucide-react";
import { api } from "@/lib/api";
import { formatDateTime } from "@/lib/date-utils";
import { Skeleton } from "@/components/ui/skeleton";
import { AuthGuard } from "@/components/auth-guard";
import { eventEmitter, Events } from "@/lib/events";

interface Alert {
  id: string;
  alert_type: string;
  severity: "low" | "medium" | "high" | "critical" | "info" | "warning"; // ✅ All severity levels
  title: string;
  description: string;
  resource_type: string;
  resource_id: string;
  is_acknowledged: boolean;
  acknowledged_by?: string;
  acknowledged_at?: string;
  created_at: string;
}

const severityConfig = {
  low: {
    color: "bg-gray-100 text-gray-800 border-gray-200",
    icon: Info,
  },
  medium: {
    color: "bg-blue-100 text-blue-800 border-blue-200",
    icon: Info,
  },
  high: {
    color: "bg-yellow-100 text-yellow-800 border-yellow-200",
    icon: AlertTriangle,
  },
  critical: {
    color: "bg-red-100 text-red-800 border-red-200",
    icon: ShieldAlert,
  },
  // Legacy aliases
  info: {
    color: "bg-blue-100 text-blue-800 border-blue-200",
    icon: Info,
  },
  warning: {
    color: "bg-yellow-100 text-yellow-800 border-yellow-200",
    icon: AlertTriangle,
  },
};

const alertTypeIcons: Record<string, any> = {
  certificate_expiring: Clock,
  api_key_expiring: Key,
  trust_score_low: TrendingDown,
  agent_offline: AlertTriangle,
  security_breach: ShieldAlert,
  unusual_activity: Info,
  configuration_drift: GitBranch,
};

export default function AlertsPage() {
  const router = useRouter();
  const [authChecked, setAuthChecked] = useState(false);
  const [role, setRole] = useState<"admin" | "manager" | "member" | "viewer">(
    "viewer"
  );

  // Admin-only guard per request
  useEffect(() => {
    try {
      const token = (require("@/lib/api") as any).api.getToken?.();
      if (!token) {
        router.replace("/auth/login");
        return;
      }
      const payload = JSON.parse(atob(token.split(".")[1]));
      const userRole = (payload.role as any) || "viewer";
      setRole(userRole);
      if (userRole !== "admin") {
        router.replace("/dashboard");
        return;
      }
    } catch {
      router.replace("/auth/login");
      return;
    } finally {
      setAuthChecked(true);
    }
  }, [router]);
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [total, setTotal] = useState<number>(0);
  const [allCount, setAllCount] = useState<number>(0);
  const [acknowledgedCount, setAcknowledgedCount] = useState<number>(0);
  const [unacknowledgedCount, setUnacknowledgedCount] = useState<number>(0);
  const [loading, setLoading] = useState(true);
  const [severityFilter, setSeverityFilter] = useState<string>("all");
  const [statusFilter, setStatusFilter] = useState<string>("unacknowledged");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [searchField, setSearchField] = useState<string>("title");
  const [searchQuery, setSearchQuery] = useState<string>("");

  useEffect(() => {
    // Reset to page 1 when filters change
    setPage(1);
  }, [severityFilter, statusFilter]);

  useEffect(() => {
    fetchAlerts();
  }, [page, pageSize, severityFilter, statusFilter]);

  const fetchAlerts = async () => {
    setLoading(true);
    try {
      const offset = (page - 1) * pageSize;
      const data = await api.getAlerts(pageSize, offset);
      setAlerts(data.alerts);
      setTotal(data.total);
      setAllCount(data.all_count || 0);
      setAcknowledgedCount(data.acknowledged_count || 0);
      setUnacknowledgedCount(data.unacknowledged_count || 0);
    } catch (error) {
      console.error("Failed to fetch alerts:", error);
    } finally {
      setLoading(false);
    }
  };

  const acknowledgeAlert = async (alertId: string) => {
    try {
      await api.acknowledgeAlert(alertId);
      // Update local state
      setAlerts(
        alerts.map((a) =>
          a.id === alertId
            ? {
                ...a,
                is_acknowledged: true,
                acknowledged_at: new Date().toISOString(),
              }
            : a
        )
      );
      // Emit event for real-time sidebar update
      eventEmitter.emit(Events.ALERT_ACKNOWLEDGED);
    } catch (error) {
      console.error("Failed to acknowledge alert:", error);
      window.alert("Failed to acknowledge alert");
    }
  };

  const resolveAlert = async (alertId: string) => {
    try {
      const resolution_notes = prompt("Enter resolution notes (required):");
      if (!resolution_notes || resolution_notes.trim() === "") {
        return; // User cancelled or entered empty notes
      }

      await api.resolveAlert(alertId, resolution_notes.trim());

      // Remove resolved alert from local state
      setAlerts(alerts.filter((a) => a.id !== alertId));

      // Emit event for real-time sidebar update
      eventEmitter.emit(Events.ALERT_RESOLVED);

      window.alert("Alert resolved successfully");
    } catch (error) {
      console.error("Failed to resolve alert:", error);
      window.alert("Failed to resolve alert");
    }
  };

  const acknowledgeAll = async () => {
    try {
      const unacknowledged = filteredAlerts.filter((a) => !a.is_acknowledged);
      await Promise.all(unacknowledged.map((a) => api.acknowledgeAlert(a.id)));
      setAlerts(
        alerts.map((a) =>
          !a.is_acknowledged
            ? {
                ...a,
                is_acknowledged: true,
                acknowledged_at: new Date().toISOString(),
              }
            : a
        )
      );
      // Emit event for real-time sidebar update
      eventEmitter.emit(Events.ALERT_ACKNOWLEDGED);
    } catch (error) {
      console.error("Failed to acknowledge all alerts:", error);
      window.alert("Failed to acknowledge all alerts");
    }
  };

  const filteredAlerts = alerts.filter((alert) => {
    const matchesSeverity =
      severityFilter === "all" || alert.severity === severityFilter;
    const matchesStatus =
      statusFilter === "all" ||
      (statusFilter === "acknowledged" && alert.is_acknowledged) ||
      (statusFilter === "unacknowledged" && !alert.is_acknowledged);
    let matchesSearch = true;
    if (searchQuery.trim() !== "") {
      const searchFields: { [key: string]: string } = {
        title: alert.title,
        description: alert.description,
        resource_id: alert.resource_id,
        alert_type: alert.alert_type,
      };
      const value = (searchFields[searchField] ?? "").toLowerCase();
      matchesSearch = value.includes(searchQuery.toLowerCase());
    }
    return matchesSeverity && matchesStatus && matchesSearch;
  });

  // Get the total count based on current status filter
  const getTotalCountForFilter = () => {
    if (statusFilter === "acknowledged") {
      return acknowledgedCount;
    } else if (statusFilter === "unacknowledged") {
      return unacknowledgedCount;
    }
    return allCount;
  };

  // Calculate total pages based on filtered count
  const totalFilteredCount = getTotalCountForFilter();
  const totalPages = Math.max(1, Math.ceil(totalFilteredCount / pageSize));

  useEffect(() => {
    // If current page is beyond total pages, go to last page
    if (page > totalPages && totalPages > 0) {
      setPage(totalPages);
    }
  }, [page, totalPages]);

  // Stats based on API counts and current filter
  const stats = {
    total: allCount,
    acknowledged: acknowledgedCount,
    unacknowledged: unacknowledgedCount,
    // Severity counts are still calculated from loaded alerts (client-side)
    // since API doesn't provide severity breakdown
    critical: alerts.filter(
      (a) => a.severity === "critical" && !a.is_acknowledged
    ).length,
    high: alerts.filter((a) => a.severity === "high" && !a.is_acknowledged)
      .length,
    medium: alerts.filter(
      (a) =>
        (a.severity === "medium" || a.severity === "warning") &&
        !a.is_acknowledged
    ).length,
    low: alerts.filter(
      (a) =>
        (a.severity === "low" || a.severity === "info") && !a.is_acknowledged
    ).length,
  };

  if (loading) {
    return (
      <div className="space-y-6">
        <div className="space-y-2">
          <Skeleton className="h-9 w-32" />
          <Skeleton className="h-4 w-96" />
        </div>
        <div className="grid gap-4 md:grid-cols-4">
          {[...Array(4)].map((_, i) => (
            <Card key={i}>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <Skeleton className="h-4 w-24" />
                <Skeleton className="h-4 w-4" />
              </CardHeader>
              <CardContent>
                <Skeleton className="h-8 w-16 mb-2" />
                <Skeleton className="h-3 w-32" />
              </CardContent>
            </Card>
          ))}
        </div>
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <Skeleton className="h-6 w-32" />
              <div className="flex items-center gap-2">
                <Skeleton className="h-10 w-32" />
                <Skeleton className="h-10 w-32" />
              </div>
            </div>
          </CardHeader>
          <CardContent className="space-y-4">
            {[...Array(5)].map((_, i) => (
              <div
                key={i}
                className="flex items-start gap-4 p-4 border border-gray-200 rounded-lg"
              >
                <Skeleton className="h-10 w-10 rounded-full" />
                <div className="flex-1 space-y-2">
                  <div className="flex items-center gap-2">
                    <Skeleton className="h-5 w-24 rounded-full" />
                    <Skeleton className="h-4 w-48" />
                  </div>
                  <Skeleton className="h-4 w-full" />
                  <Skeleton className="h-3 w-32" />
                </div>
                <Skeleton className="h-9 w-28" />
              </div>
            ))}
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <AuthGuard>
      <div className="space-y-6">
        <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">Security Alerts</h1>
          <p className="text-muted-foreground mt-1">
            Proactive monitoring and notifications
          </p>
        </div>
        {stats.unacknowledged > 0 && (
          <Button onClick={acknowledgeAll}>
            <CheckCircle2 className="mr-2 h-4 w-4" />
            Acknowledge All ({stats.unacknowledged})
          </Button>
        )}
      </div>

      {/* Stats */}
      <div className="grid gap-4 md:grid-cols-5">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Total Alerts</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{stats.total}</div>
          </CardContent>
        </Card>
        <Card className="border-red-200 bg-red-50">
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-red-800">
              Critical
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-red-900">
              {stats.critical}
            </div>
          </CardContent>
        </Card>
        <Card className="border-orange-200 bg-orange-50">
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-orange-800">
              High
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-orange-900">
              {stats.high}
            </div>
          </CardContent>
        </Card>
        <Card className="border-yellow-200 bg-yellow-50">
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-yellow-800">
              Medium
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-yellow-900">
              {stats.medium}
            </div>
          </CardContent>
        </Card>
        <Card className="border-gray-200 bg-gray-50">
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-gray-800">
              Low
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-gray-900">{stats.low}</div>
          </CardContent>
        </Card>
      </div>

      {/* Filters & Pagination */}
      <Card>
        <CardHeader>
          <CardTitle>Filter Alerts</CardTitle>
        </CardHeader>
        <CardContent className="flex gap-4 items-center">
          <Select value={statusFilter} onValueChange={setStatusFilter}>
            <SelectTrigger className="w-[240px]">
              <SelectValue placeholder="Filter by status" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">
                <div className="flex items-center justify-between w-full">
                  <span>All Alerts</span>
                  <span className="ml-2 px-2 py-0.5 rounded-full bg-red-500 text-white text-xs font-semibold">
                    {allCount}
                  </span>
                </div>
              </SelectItem>
              <SelectItem value="unacknowledged">
                <div className="flex items-center justify-between w-full">
                  <span>Unacknowledged</span>
                  <span className="ml-2 px-2 py-0.5 rounded-full bg-red-500 text-white text-xs font-semibold">
                    {unacknowledgedCount}
                  </span>
                </div>
              </SelectItem>
              <SelectItem value="acknowledged">
                <div className="flex items-center justify-between w-full">
                  <span>Acknowledged</span>
                  <span className="ml-2 px-2 py-0.5 rounded-full bg-red-500 text-white text-xs font-semibold">
                    {acknowledgedCount}
                  </span>
                </div>
              </SelectItem>
            </SelectContent>
          </Select>
          <Select value={severityFilter} onValueChange={setSeverityFilter}>
            <SelectTrigger className="w-[200px]">
              <SelectValue placeholder="Filter by severity" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All Severities</SelectItem>
              <SelectItem value="critical">Critical</SelectItem>
              <SelectItem value="high">High</SelectItem>
              <SelectItem value="medium">Medium</SelectItem>
              <SelectItem value="low">Low</SelectItem>
              <SelectItem value="info">Info</SelectItem>
              <SelectItem value="warning">Warning</SelectItem>
            </SelectContent>
          </Select>
         

          <div className="flex gap-2 items-center">
            <Select value={searchField} onValueChange={setSearchField}>
              <SelectTrigger className="w-[140px]">
                <SelectValue placeholder="Search by" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="title">Title</SelectItem>
                <SelectItem value="description">Description</SelectItem>
                <SelectItem value="resource_id">Resource ID</SelectItem>
                <SelectItem value="alert_type">Alert Type</SelectItem>
              </SelectContent>
            </Select>
            <input
              type="text"
              className="border rounded px-2 py-1 w-[220px] focus:outline-none focus:ring-2 focus:ring-blue-400 transition-all"
              placeholder={`🔍 Search ${searchField.charAt(0).toUpperCase() + searchField.slice(1)}`}
              value={searchQuery}
              onChange={e => setSearchQuery(e.target.value)}
              autoComplete="off"
            />
            {searchQuery && (
              <Button variant="ghost" size="sm" onClick={() => setSearchQuery("")}>Clear</Button>
            )}
          </div>

          <div className="flex gap-2 items-center">
            <span>Rows per page:</span>
            <Select value={String(pageSize)} onValueChange={v => setPageSize(Number(v))}>
              <SelectTrigger className="w-[80px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="10">10</SelectItem>
                <SelectItem value="50">50</SelectItem>
                <SelectItem value="100">100</SelectItem>
              </SelectContent>
            </Select>
          </div>

         
        </CardContent>
      </Card>

      {/* Pagination Controls */}
      <div className="flex gap-2 items-center mt-4">
        <Button
          disabled={page === 1}
          onClick={() => setPage(page - 1)}
          variant="outline"
        >
          Previous
        </Button>
        <span>
          Page {page} of {totalPages}
        </span>
        <Button
          disabled={page >= totalPages}
          onClick={() => setPage(page + 1)}
          variant="outline"
        >
          Next
        </Button>
      </div>

      {/* Alerts List */}
      <Card>
        <CardHeader>
         
          <CardDescription>
            Security and operational notifications requiring attention
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="space-y-3">
            {filteredAlerts.map((alert) => {
              const config = severityConfig[alert.severity];
              const Icon = config.icon;
              const TypeIcon =
                alertTypeIcons[alert.alert_type] || AlertTriangle;

              return (
                <div
                  key={alert.id}
                  className={`p-4 border-2 rounded-lg ${
                    alert.is_acknowledged
                      ? "opacity-60 bg-muted/30"
                      : config.color
                  }`}
                >
                  <div className="flex items-start justify-between">
                    <div className="flex items-start gap-3 flex-1">
                      <Icon className="h-5 w-5 mt-0.5" />

                      <div className="flex-1 space-y-2">
                        <div className="flex items-start justify-between">
                          <div>
                            <div className="flex items-center gap-2">
                              <h3 className="font-semibold">{alert.title}</h3>
                              <Badge variant="outline" className="text-xs">
                                <TypeIcon className="h-3 w-3 mr-1" />
                                {alert.alert_type.replace(/_/g, " ")}
                              </Badge>
                            </div>
                            <p className="text-sm mt-1">{alert.description}</p>
                          </div>
                        </div>

                        <div className="flex items-center gap-4 text-xs">
                          <span>
                            {alert.resource_type}:{" "}
                            {alert.resource_id.substring(0, 8)}...
                          </span>
                          <span>•</span>
                          <span>{formatDateTime(alert.created_at)}</span>
                        </div>

                        {alert.is_acknowledged && (
                          <div className="flex items-center gap-2 text-xs text-muted-foreground">
                            <CheckCircle2 className="h-3 w-3" />
                            <span>
                              Acknowledged{" "}
                              {alert.acknowledged_at &&
                                formatDateTime(alert.acknowledged_at)}
                            </span>
                          </div>
                        )}
                      </div>

                      <div className="flex gap-2">
                        {!alert.is_acknowledged && (
                          <Button
                            size="sm"
                            variant="outline"
                            onClick={() => acknowledgeAlert(alert.id)}
                          >
                            <CheckCircle2 className="h-4 w-4 mr-2" />
                            Acknowledge
                          </Button>
                        )}
                        {alert.is_acknowledged && (
                          <Button
                            size="sm"
                            variant="outline"
                            onClick={() => resolveAlert(alert.id)}
                            className="border-green-500 text-green-600 hover:bg-green-50"
                          >
                            <CheckCircle2 className="h-4 w-4 mr-2" />
                            Resolve
                          </Button>
                        )}
                      </div>
                    </div>
                  </div>
                </div>
              );
            })}

            {filteredAlerts.length === 0 && (
              <div className="text-center py-12 text-muted-foreground">
                <CheckCircle2 className="h-12 w-12 mx-auto mb-4 text-green-600" />
                <p className="text-lg font-medium">No alerts to display</p>
                <p className="text-sm">
                  {statusFilter === "unacknowledged"
                    ? "All alerts have been acknowledged"
                    : "No alerts match your filter criteria"}
                </p>
              </div>
            )}
          </div>
        </CardContent>
      </Card>
    </div>
    </AuthGuard>
  );
}

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
  const [loading, setLoading] = useState(true);
  const [severityFilter, setSeverityFilter] = useState<string>("all");
  const [statusFilter, setStatusFilter] = useState<string>("unacknowledged");

  useEffect(() => {
    fetchAlerts();
  }, []);

  const fetchAlerts = async () => {
    try {
      const data = await api.getAlerts(100, 0);
      setAlerts(data);
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
    } catch (error) {
      console.error("Failed to acknowledge alert:", error);
      window.alert("Failed to acknowledge alert");
    }
  };

  const approveDrift = async (alertId: string, driftedServers: string[]) => {
    try {
      await api.approveDrift(alertId, driftedServers);
      // Update local state - mark as acknowledged
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
      window.alert(
        "Configuration drift approved successfully. Agent registration has been updated."
      );
    } catch (error) {
      console.error("Failed to approve drift:", error);
      window.alert("Failed to approve drift");
    }
  };

  // Extract drifted MCP servers from alert description
  const extractDriftedServers = (description: string): string[] => {
    const servers: string[] = [];
    const lines = description.split("\n");
    let inMCPSection = false;

    for (const line of lines) {
      if (line.includes("Unauthorized MCP Server Communication:")) {
        inMCPSection = true;
        continue;
      }
      if (
        line.includes("Undeclared Capability Usage:") ||
        line.includes("Registered Configuration:")
      ) {
        inMCPSection = false;
        continue;
      }
      if (
        inMCPSection &&
        line.includes("`") &&
        line.includes("not registered")
      ) {
        // Extract server name between backticks
        const match = line.match(/`([^`]+)`/);
        if (match) {
          servers.push(match[1]);
        }
      }
    }
    return servers;
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

    return matchesSeverity && matchesStatus;
  });

  const stats = {
    total: alerts.length,
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
    unacknowledged: alerts.filter((a) => !a.is_acknowledged).length,
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

      {/* Filters */}
      <Card>
        <CardHeader>
          <CardTitle>Filter Alerts</CardTitle>
        </CardHeader>
        <CardContent className="flex gap-4">
          <Select value={statusFilter} onValueChange={setStatusFilter}>
            <SelectTrigger className="w-[200px]">
              <SelectValue placeholder="Filter by status" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All Alerts</SelectItem>
              <SelectItem value="unacknowledged">Unacknowledged</SelectItem>
              <SelectItem value="acknowledged">Acknowledged</SelectItem>
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
            </SelectContent>
          </Select>

          {(severityFilter !== "all" || statusFilter !== "unacknowledged") && (
            <Button
              variant="ghost"
              onClick={() => {
                setSeverityFilter("all");
                setStatusFilter("unacknowledged");
              }}
            >
              Clear filters
            </Button>
          )}
        </CardContent>
      </Card>

      {/* Alerts List */}
      <Card>
        <CardHeader>
          <CardTitle>Active Alerts ({filteredAlerts.length})</CardTitle>
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

                      {!alert.is_acknowledged && (
                        <div className="flex gap-2">
                          {alert.alert_type === "configuration_drift" && (
                            <Button
                              size="sm"
                              variant="default"
                              onClick={() => {
                                const driftedServers = extractDriftedServers(
                                  alert.description
                                );
                                if (driftedServers.length > 0) {
                                  if (
                                    confirm(
                                      `Approve drift and add these MCP servers to agent registration:\n\n${driftedServers.join("\n")}\n\nThis will update the agent's configuration.`
                                    )
                                  ) {
                                    approveDrift(alert.id, driftedServers);
                                  }
                                } else {
                                  window.alert(
                                    "Could not extract drifted servers from alert"
                                  );
                                }
                              }}
                            >
                              <Check className="h-4 w-4 mr-2" />
                              Approve Drift
                            </Button>
                          )}
                          <Button
                            size="sm"
                            variant="outline"
                            onClick={() => acknowledgeAlert(alert.id)}
                          >
                            <CheckCircle2 className="h-4 w-4 mr-2" />
                            Acknowledge
                          </Button>
                        </div>
                      )}
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
  );
}

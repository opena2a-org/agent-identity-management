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
import Link from "next/link";
import {
  Users,
  ShieldAlert,
  FileText,
  TrendingUp,
  AlertTriangle,
  CheckCircle2,
} from "lucide-react";
import { Skeleton } from "@/components/ui/skeleton";
import { api } from "@/lib/api";
import { formatDateTime } from "@/lib/date-utils";

interface AdminStats {
  totalUsers: number;
  totalOrganizations: number;
  pendingAgents: number;
  unacknowledgedAlerts: number;
  totalAuditLogs: number;
  recentActions24h: number;
}

export default function AdminDashboard() {
  const router = useRouter();
  const [stats, setStats] = useState<AdminStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [authChecked, setAuthChecked] = useState(false);
  const [role, setRole] = useState<"admin" | "manager" | "member" | "viewer">(
    "viewer"
  );
  const [recentLogs, setRecentLogs] = useState<any[]>([]);
  const [recentLoading, setRecentLoading] = useState(true);

  // Admin-only guard
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

  useEffect(() => {
    if (!authChecked || role !== "admin") return;
    fetchStats();
    fetchRecent();
  }, [authChecked, role]);

  const fetchStats = async () => {
    try {
      // In real implementation, fetch from API
      // For now, mock data
      setStats({
        totalUsers: 24,
        totalOrganizations: 5,
        pendingAgents: 3,
        unacknowledgedAlerts: 7,
        totalAuditLogs: 1247,
        recentActions24h: 156,
      });
    } catch (error) {
      console.error("Failed to fetch admin stats:", error);
    } finally {
      setLoading(false);
    }
  };

  const fetchRecent = async () => {
    try {
      setRecentLoading(true);
      const logs = await api.getAuditLogs(10, 0);
      setRecentLogs(logs || []);
    } catch (e) {
      console.error("Failed to fetch recent audit logs", e);
      setRecentLogs([]);
    } finally {
      setRecentLoading(false);
    }
  };

  if (!authChecked || role !== "admin") {
    return null;
  }

  if (loading) {
    return (
      <div className="space-y-8">
        <div className="space-y-2">
          <Skeleton className="h-9 w-64" />
          <Skeleton className="h-4 w-96" />
        </div>
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {[...Array(6)].map((_, i) => (
            <Card key={i}>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <Skeleton className="h-4 w-32" />
                <Skeleton className="h-4 w-4" />
              </CardHeader>
              <CardContent>
                <Skeleton className="h-8 w-16 mb-2" />
                <Skeleton className="h-3 w-40" />
              </CardContent>
            </Card>
          ))}
        </div>
        <div className="grid gap-6 md:grid-cols-2">
          {[...Array(2)].map((_, i) => (
            <Card key={i}>
              <CardHeader>
                <Skeleton className="h-6 w-48 mb-2" />
                <Skeleton className="h-4 w-64" />
              </CardHeader>
              <CardContent className="space-y-3">
                {[...Array(4)].map((_, j) => (
                  <Skeleton key={j} className="h-16 w-full" />
                ))}
              </CardContent>
            </Card>
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-3xl font-bold">Admin Dashboard</h1>
        <p className="text-muted-foreground mt-1">
          Platform overview and management
        </p>
      </div>

      {/* Stats Grid */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Users</CardTitle>
            <Users className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{stats?.totalUsers}</div>
            <p className="text-xs text-muted-foreground">
              Across {stats?.totalOrganizations} organizations
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">
              Pending Agents
            </CardTitle>
            <ShieldAlert className="h-4 w-4 text-yellow-600" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{stats?.pendingAgents}</div>
            <p className="text-xs text-muted-foreground">
              Awaiting verification
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">
              Unacknowledged Alerts
            </CardTitle>
            <AlertTriangle className="h-4 w-4 text-red-600" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {stats?.unacknowledgedAlerts}
            </div>
            <p className="text-xs text-muted-foreground">Require attention</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">
              Total Audit Logs
            </CardTitle>
            <FileText className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {stats?.totalAuditLogs.toLocaleString()}
            </div>
            <p className="text-xs text-muted-foreground">All-time records</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">
              Recent Activity
            </CardTitle>
            <TrendingUp className="h-4 w-4 text-green-600" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{stats?.recentActions24h}</div>
            <p className="text-xs text-muted-foreground">
              Actions in last 24 hours
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">System Status</CardTitle>
            <CheckCircle2 className="h-4 w-4 text-green-600" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">Healthy</div>
            <p className="text-xs text-muted-foreground">
              All services operational
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Quick Actions */}
      <Card>
        <CardHeader>
          <CardTitle>Quick Actions</CardTitle>
          <CardDescription>Common administrative tasks</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
          <Link href="/dashboard/admin/users">
            <Button className="w-full" variant="outline">
              <Users className="mr-2 h-4 w-4" />
              Manage Users
            </Button>
          </Link>

          <Link href="/dashboard/admin/alerts">
            <Button className="w-full" variant="outline">
              <AlertTriangle className="mr-2 h-4 w-4" />
              Review Alerts
            </Button>
          </Link>

          <Link href="/dashboard/admin/audit-logs">
            <Button className="w-full" variant="outline">
              <FileText className="mr-2 h-4 w-4" />
              View Audit Logs
            </Button>
          </Link>

          <Link href="/dashboard/compliance">
            <Button className="w-full" variant="outline">
              <ShieldAlert className="mr-2 h-4 w-4" />
              Generate Report
            </Button>
          </Link>
        </CardContent>
      </Card>

      {/* Recent Activity Feed */}
      <Card>
        <CardHeader>
          <CardTitle>Recent Activity</CardTitle>
          <CardDescription>Latest platform actions</CardDescription>
        </CardHeader>
        <CardContent>
          {recentLoading ? (
            <div className="space-y-3">
              {[...Array(5)].map((_, i) => (
                <Skeleton key={i} className="h-6 w-full" />
              ))}
            </div>
          ) : recentLogs.length === 0 ? (
            <div className="text-sm text-muted-foreground">
              No recent activity
            </div>
          ) : (
            <div className="space-y-3">
              {recentLogs.map((log: any) => {
                const action = (log.action || "").toLowerCase();
                const color = action.includes("delete")
                  ? "bg-red-600"
                  : action.includes("update")
                    ? "bg-blue-600"
                    : action.includes("verify") || action.includes("create")
                      ? "bg-green-600"
                      : action.includes("alert")
                        ? "bg-yellow-600"
                        : "bg-gray-500";
                const when = log.timestamp
                  ? formatDateTime(log.timestamp)
                  : formatDateTime(log.created_at || new Date().toISOString());
                const who = log.user_email || log.user || "system";
                const resource = log.resource_type
                  ? `${log.resource_type}${log.resource_id ? ` ${String(log.resource_id).toString().substring(0, 8)}…` : ""}`
                  : "";
                const message =
                  log.message || log.details || `${who} ${action} ${resource}`;
                return (
                  <div
                    key={log.id || `${log.timestamp}-${Math.random()}`}
                    className="flex items-center gap-4 text-sm"
                  >
                    <div className={`h-2 w-2 rounded-full ${color}`} />
                    <span className="text-muted-foreground">{when}</span>
                    <span className="truncate">{message}</span>
                  </div>
                );
              })}
            </div>
          )}
          <div className="mt-4">
            <Link href="/dashboard/admin/audit-logs">
              <Button variant="ghost" className="w-full">
                View All Activity
              </Button>
            </Link>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

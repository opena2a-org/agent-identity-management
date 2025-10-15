"use client";

import { useEffect, useState } from "react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import {
  Search,
  Shield,
  Users,
  Mail,
  Check,
  X,
  Clock,
  Ban,
  UserX,
  Settings,
  Key,
  Bot,
  Server,
  ArrowRight,
} from "lucide-react";
import { api } from "@/lib/api";
import { Switch } from "@/components/ui/switch";
import { Label } from "@/components/ui/label";
import Link from "next/link";
import { formatDate } from "@/lib/date-utils";
import { Skeleton } from "@/components/ui/skeleton";
import { TableSkeleton } from "@/components/ui/content-loaders";

interface User {
  id: string;
  email: string;
  name: string;
  display_name?: string;
  role: "admin" | "manager" | "member" | "viewer" | "pending";
  status:
    | "pending"
    | "active"
    | "suspended"
    | "deactivated"
    | "pending_approval";
  organization_id?: string;
  organization_name?: string;
  provider: string;
  approved_by?: string;
  approved_at?: string;
  created_at: string;
  last_login_at?: string;
  requested_at?: string;
  picture_url?: string;
  is_registration_request?: boolean;
}

const roleColors = {
  admin: "bg-purple-100 text-purple-800",
  manager: "bg-blue-100 text-blue-800",
  member: "bg-green-100 text-green-800",
  viewer: "bg-gray-100 text-gray-800",
  pending: "bg-yellow-100 text-yellow-800",
};

const statusColors = {
  pending: "bg-yellow-100 text-yellow-800",
  active: "bg-green-100 text-green-800",
  suspended: "bg-orange-100 text-orange-800",
  deactivated: "bg-red-100 text-red-800",
  pending_approval: "bg-orange-100 text-orange-800",
};

const statusIcons = {
  pending: Clock,
  active: Check,
  suspended: Ban,
  deactivated: UserX,
  pending_approval: Clock,
};

const roleIcons = {
  admin: Shield,
  manager: Users,
  member: Users,
  viewer: Users,
  pending: Clock,
};

export default function UsersPage() {
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");
  const [filterOrg, setFilterOrg] = useState<string>("all");
  const [filterStatus, setFilterStatus] = useState<string>("all");
  const [autoApproveSSO, setAutoApproveSSO] = useState(true);
  const [savingSettings, setSavingSettings] = useState(false);
  const [apiKeysCount, setApiKeysCount] = useState(0);

  useEffect(() => {
    fetchUsers();
    fetchSettings();
    fetchAPIKeysCount();
  }, []);

  const fetchUsers = async () => {
    try {
      const data = await api.getUsers();
      setUsers(data);
    } catch (error) {
      console.error("Failed to fetch users:", error);
    } finally {
      setLoading(false);
    }
  };

  const fetchSettings = async () => {
    try {
      const settings = await api.getOrganizationSettings();
      setAutoApproveSSO(settings.auto_approve_sso);
    } catch (error) {
      console.error("Failed to fetch settings:", error);
    }
  };

  const fetchAPIKeysCount = async () => {
    try {
      const { api_keys } = await api.listAPIKeys();
      setApiKeysCount(api_keys?.length || 0);
    } catch (error) {
      console.error("Failed to fetch API keys count:", error);
    }
  };

  const toggleAutoApprove = async (enabled: boolean) => {
    setSavingSettings(true);
    try {
      await api.updateOrganizationSettings(enabled);
      setAutoApproveSSO(enabled);
    } catch (error) {
      console.error("Failed to update settings:", error);
      alert("Failed to update settings");
    } finally {
      setSavingSettings(false);
    }
  };

  const updateUserRole = async (userId: string, newRole: string) => {
    try {
      await api.updateUserRole(userId, newRole as User["role"]);
      // Update local state
      setUsers(
        users?.map((u) =>
          u.id === userId ? { ...u, role: newRole as User["role"] } : u
        ) || []
      );
    } catch (error) {
      console.error("Failed to update user role:", error);
      alert("Failed to update user role");
    }
  };

  const approveUser = async (user: User) => {
    try {
      if (user.is_registration_request) {
        await api.approveRegistrationRequest(user.id);
        alert("Registration request approved successfully");
      } else {
        await api.approveUser(user.id);
        alert("User approved successfully");
      }
      fetchUsers(); // Refresh the list
    } catch (error) {
      console.error("Failed to approve user:", error);
      alert("Failed to approve user");
    }
  };

  const rejectUser = async (user: User) => {
    const message = user.is_registration_request
      ? "Are you sure you want to reject this registration request?"
      : "Are you sure you want to reject this user? This will delete their account.";

    if (!confirm(message)) {
      return;
    }

    try {
      if (user.is_registration_request) {
        await api.rejectRegistrationRequest(user.id);
        alert("Registration request rejected successfully");
      } else {
        await api.rejectUser(user.id);
        alert("User rejected successfully");
      }
      fetchUsers(); // Refresh the list
    } catch (error) {
      console.error("Failed to reject user:", error);
      alert("Failed to reject user");
    }
  };

  const filteredUsers =
    users?.filter((user) => {
      const matchesSearch =
        user.email.toLowerCase().includes(searchQuery.toLowerCase()) ||
        user.name?.toLowerCase().includes(searchQuery.toLowerCase()) ||
        user.display_name?.toLowerCase().includes(searchQuery.toLowerCase());

      const matchesOrg =
        filterOrg === "all" || user.organization_id === filterOrg;
      const matchesStatus =
        filterStatus === "all" || user.status === filterStatus;

      return matchesSearch && matchesOrg && matchesStatus;
    }) || [];

  const organizations = Array.from(
    new Set(
      users
        ?.filter((u) => u.organization_id)
        .map((u) => ({
          id: u.organization_id!,
          name: u.organization_name || "Unknown",
        })) || []
    )
  );

  const pendingCount =
    users?.filter(
      (u) =>
        u.status === "pending" ||
        u.status === "pending_approval" ||
        u.is_registration_request
    ).length || 0;
  const activeCount = users?.filter((u) => u.status === "active").length || 0;

  if (loading) {
    return (
      <div className="space-y-6">
        <div className="space-y-2">
          <Skeleton className="h-9 w-32" />
          <Skeleton className="h-4 w-96" />
        </div>
        <div className="grid gap-4 md:grid-cols-3">
          {[...Array(3)].map((_, i) => (
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
            <Skeleton className="h-6 w-48" />
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex items-center gap-4">
              <Skeleton className="h-10 flex-1" />
              <Skeleton className="h-10 w-32" />
            </div>
            <TableSkeleton rows={6} columns={5} />
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">User Management</h1>
        <p className="text-muted-foreground mt-1">
          Manage <strong>human users</strong> who access the AIM dashboard
        </p>
        <div className="flex items-center gap-2 mt-3 text-sm text-muted-foreground">
          <span>Also manage programmatic identities:</span>
          <Link
            href="/dashboard/agents"
            className="inline-flex items-center gap-1 text-blue-600 hover:text-blue-700 hover:underline"
          >
            <Bot className="h-4 w-4" />
            AI Agents
            <ArrowRight className="h-3 w-3" />
          </Link>
          <span className="text-muted-foreground">•</span>
          <Link
            href="/dashboard/mcp"
            className="inline-flex items-center gap-1 text-blue-600 hover:text-blue-700 hover:underline"
          >
            <Server className="h-4 w-4" />
            MCP Servers
            <ArrowRight className="h-3 w-3" />
          </Link>
        </div>
      </div>

      {/* Stats */}
      <div className="grid gap-4 md:grid-cols-5">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Total Users</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{users?.length || 0}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">
              Pending Approval
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-yellow-600">
              {pendingCount}
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Active Users</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-green-600">
              {activeCount}
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Organizations</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {organizations?.length || 0}
            </div>
          </CardContent>
        </Card>
        <Card className="bg-gradient-to-br from-blue-50 to-indigo-50 dark:from-blue-950/20 dark:to-indigo-950/20 border-blue-200 dark:border-blue-800">
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium flex items-center gap-2">
              <Key className="h-4 w-4 text-blue-600" />
              API Keys Issued
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-blue-600">
              {apiKeysCount}
            </div>
            <Link
              href="/dashboard/api-keys"
              className="text-xs text-blue-600 hover:text-blue-700 hover:underline mt-1 inline-block"
            >
              Manage API Keys →
            </Link>
          </CardContent>
        </Card>
      </div>

      {/* Organization Settings */}
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <Settings className="h-5 w-5 text-muted-foreground" />
            <CardTitle>User Approval Settings</CardTitle>
          </div>
          <CardDescription>
            Configure how new SSO users are handled when they sign in
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center justify-between">
            <div className="space-y-0.5">
              <Label htmlFor="auto-approve" className="text-base">
                Auto-Approve SSO Users
              </Label>
              <p className="text-sm text-muted-foreground">
                When enabled, new SSO users are automatically granted access.
                When disabled, admins must manually approve new users.
              </p>
            </div>
            <Switch
              id="auto-approve"
              checked={autoApproveSSO}
              onCheckedChange={toggleAutoApprove}
              disabled={savingSettings}
            />
          </div>
          <div className="text-sm text-muted-foreground bg-muted p-3 rounded-md">
            <strong>Note:</strong> The first user in any organization is always
            automatically approved as admin, regardless of this setting.
          </div>
        </CardContent>
      </Card>

      {/* Filters */}
      <Card>
        <CardHeader>
          <CardTitle>Search and Filter</CardTitle>
        </CardHeader>
        <CardContent className="flex gap-4">
          <div className="flex-1 relative">
            <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-muted-foreground" />
            <Input
              placeholder="Search by email or name..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="pl-10"
            />
          </div>
          <Select value={filterStatus} onValueChange={setFilterStatus}>
            <SelectTrigger className="w-[180px]">
              <SelectValue placeholder="Filter by status" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All Statuses</SelectItem>
              <SelectItem value="pending">Pending</SelectItem>
              <SelectItem value="pending_approval">Pending Approval</SelectItem>
              <SelectItem value="active">Active</SelectItem>
              <SelectItem value="suspended">Suspended</SelectItem>
              <SelectItem value="deactivated">Deactivated</SelectItem>
            </SelectContent>
          </Select>
          <Select value={filterOrg} onValueChange={setFilterOrg}>
            <SelectTrigger className="w-[200px]">
              <SelectValue placeholder="Filter by organization" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All Organizations</SelectItem>
              {organizations.map((org) => (
                <SelectItem key={org.id} value={org.id}>
                  {org.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </CardContent>
      </Card>

      {/* Users Table */}
      <Card>
        <CardHeader>
          <CardTitle>Users ({filteredUsers.length})</CardTitle>
          <CardDescription>
            Click on a role to change user permissions. Approve or reject
            pending users.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            {filteredUsers.map((user) => {
              const RoleIcon = roleIcons[user.role];
              const StatusIcon = statusIcons[user.status];
              const isPending =
                user.status === "pending" ||
                user.status === "pending_approval" ||
                user.is_registration_request;

              return (
                <div
                  key={user.id}
                  className="flex items-center justify-between p-4 border rounded-lg hover:bg-accent/50 transition-colors"
                >
                  <div className="flex items-center gap-4 flex-1">
                    <div className="h-10 w-10 rounded-full bg-gradient-to-br from-blue-500 to-purple-600 flex items-center justify-center text-white font-semibold">
                      {(user.email || "U")[0].toUpperCase()}
                    </div>

                    <div className="flex-1">
                      <div className="flex items-center gap-2">
                        <p className="font-medium">
                          {user.name || user.display_name || user.email}
                        </p>
                        <Badge variant="outline" className="text-xs">
                          {user.provider}
                        </Badge>
                        <Badge
                          className={`text-xs ${statusColors[user.status]}`}
                        >
                          <StatusIcon className="h-3 w-3 mr-1" />
                          {user.status.charAt(0).toUpperCase() +
                            user.status.slice(1)}
                        </Badge>
                      </div>
                      <div className="flex items-center gap-2 text-sm text-muted-foreground">
                        <Mail className="h-3 w-3" />
                        {user.email}
                      </div>
                      {user.organization_name && (
                        <p className="text-xs text-muted-foreground">
                          {user.organization_name}
                        </p>
                      )}
                    </div>
                  </div>

                  <div className="flex items-center gap-4">
                    <div className="text-right">
                      <p className="text-xs text-muted-foreground">Joined</p>
                      <p className="text-sm">{formatDate(user.created_at)}</p>
                    </div>

                    {isPending && (
                      <div className="flex flex-col gap-2">
                        <div className="flex gap-2">
                          <Button
                            size="sm"
                            variant="default"
                            onClick={() => approveUser(user)}
                            className="bg-green-600 hover:bg-green-700"
                          >
                            <Check className="h-4 w-4 mr-1" />
                            Approve
                          </Button>
                          <Button
                            size="sm"
                            variant="destructive"
                            onClick={() => rejectUser(user)}
                          >
                            <X className="h-4 w-4 mr-1" />
                            Reject
                          </Button>
                        </div>
                        <p className="text-xs text-muted-foreground">
                          Role can be updated after approval
                        </p>
                      </div>
                    )}

                    {!isPending && (
                      <Select
                        value={user.role}
                        onValueChange={(role) => updateUserRole(user.id, role)}
                      >
                        <SelectTrigger className="w-[140px]">
                          <SelectValue>
                            <div className="flex items-center gap-2">
                              <RoleIcon className="h-4 w-4" />
                              <span className="capitalize">{user.role}</span>
                            </div>
                          </SelectValue>
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="admin">
                            <div className="flex items-center gap-2">
                              <Shield className="h-4 w-4" />
                              Admin
                            </div>
                          </SelectItem>
                          <SelectItem value="manager">
                            <div className="flex items-center gap-2">
                              <Users className="h-4 w-4" />
                              Manager
                            </div>
                          </SelectItem>
                          <SelectItem value="member">
                            <div className="flex items-center gap-2">
                              <Users className="h-4 w-4" />
                              Member
                            </div>
                          </SelectItem>
                          <SelectItem value="viewer">
                            <div className="flex items-center gap-2">
                              <Users className="h-4 w-4" />
                              Viewer
                            </div>
                          </SelectItem>
                        </SelectContent>
                      </Select>
                    )}
                  </div>
                </div>
              );
            })}

            {filteredUsers.length === 0 && (
              <div className="text-center py-12 text-muted-foreground">
                No users found matching your search criteria
              </div>
            )}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

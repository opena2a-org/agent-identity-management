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
  Key,
  Bot,
  Server,
  ArrowRight,
} from "lucide-react";
import { api } from "@/lib/api";
import Link from "next/link";
import { formatDate } from "@/lib/date-utils";
import { Skeleton } from "@/components/ui/skeleton";
import { TableSkeleton } from "@/components/ui/content-loaders";
import { toast } from "sonner";
import { AuthGuard } from "@/components/auth-guard";

interface User {
  id: string;
  email: string;
  name: string;
  role: "admin" | "manager" | "member" | "viewer" | "pending";
  status:
    | "pending"
    | "active"
    | "suspended"
    | "deactivated"
    | "pending_approval";
  organizationId?: string;
  organizationName?: string;
  provider: string;
  approvedBy?: string;
  approvedAt?: string;
  createdAt: string;
  lastLoginAt?: string;
  requestedAt?: string;
  pictureUrl?: string;
  isRegistrationRequest?: boolean;
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
  const router = useRouter();
  const [authChecked, setAuthChecked] = useState(false);
  const [role, setRole] = useState<"admin" | "manager" | "member" | "viewer">(
    "viewer"
  );
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");
  const [filterOrg, setFilterOrg] = useState<string>("all");
  const [filterStatus, setFilterStatus] = useState<string>("all");
  const [apiKeysCount, setApiKeysCount] = useState(0);
  const [page, setPage] = useState(1);
  const pageSize = 10;

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
    fetchUsers();
    fetchAPIKeysCount();
  }, [authChecked, role]);

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

  const fetchAPIKeysCount = async () => {
    try {
      const { apiKeys } = await api.listAPIKeys();
      setApiKeysCount(apiKeys?.length || 0);
    } catch (error) {
      console.error("Failed to fetch API keys count:", error);
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
      if (user.isRegistrationRequest) {
        await api.approveRegistrationRequest(user.id);
        toast.success("Registration request approved successfully");
      } else {
        await api.approveUser(user.id);
        toast.success("User approved successfully");
      }
      await fetchUsers(); // Refresh the list
    } catch (error: any) {
      console.error("Failed to approve user:", error);
      const errorMessage = error?.message || "Failed to approve user";
      toast.error(errorMessage);
    }
  };

  const rejectUser = async (user: User) => {
    const message = user.isRegistrationRequest
      ? "Are you sure you want to reject this registration request?"
      : "Are you sure you want to reject this user? This will delete their account.";

    if (!confirm(message)) {
      return;
    }

    try {
      if (user.isRegistrationRequest) {
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
        user.name?.toLowerCase().includes(searchQuery.toLowerCase());

      const matchesOrg =
        filterOrg === "all" || user.organizationId === filterOrg;
      const matchesStatus =
        filterStatus === "all" || user.status === filterStatus;

      return matchesSearch && matchesOrg && matchesStatus;
    }) || [];

  const totalPages = Math.max(1, Math.ceil(filteredUsers.length / pageSize));
  const pageData = filteredUsers.slice((page - 1) * pageSize, page * pageSize);

  const organizations = Array.from(
    new Set(
      users
        ?.filter((u) => u.organizationId)
        .map((u) => ({
          id: u.organizationId!,
          name: u.organizationName || "Unknown",
        })) || []
    )
  );

  const pendingCount =
    users?.filter(
      (u) =>
        u.status === "pending" ||
        u.status === "pending_approval" ||
        u.isRegistrationRequest
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
    <AuthGuard>
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
            Table supports search, role/status filters and pagination.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
              <thead className="bg-gray-50 dark:bg-gray-800">
                <tr>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                    Name
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                    Email
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                    Role
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                    Status
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                    Last Login
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                    Actions
                  </th>
                </tr>
              </thead>
              <tbody className="bg-white dark:bg-gray-900 divide-y divide-gray-200 dark:divide-gray-700">
                {pageData.map((user) => {
                  const RoleIcon = roleIcons[user.role];
                  const StatusIcon = statusIcons[user.status];
                  const isPending =
                    user.status === "pending" ||
                    user.status === "pending_approval" ||
                    user.isRegistrationRequest;
                  return (
                    <tr
                      key={user.id}
                      className="hover:bg-gray-50 dark:hover:bg-gray-800"
                    >
                      <td className="px-4 py-3 whitespace-nowrap">
                        <div className="flex items-center gap-3">
                          <div className="h-8 w-8 rounded-full bg-gradient-to-br from-blue-500 to-purple-600 flex items-center justify-center text-white text-xs font-semibold">
                            {(user.email || "U")[0].toUpperCase()}
                          </div>
                          <div>
                            <div className="text-sm font-medium text-gray-900 dark:text-gray-100">
                              {user.name || user.email}
                            </div>
                            <div className="text-xs text-gray-500 dark:text-gray-400">
                              Joined {formatDate(user.createdAt)}
                            </div>
                          </div>
                        </div>
                      </td>
                      <td className="px-4 py-3 text-sm text-gray-900 dark:text-gray-100">
                        {user.email}
                      </td>
                      <td className="px-4 py-3 whitespace-nowrap">
                        {isPending ? (
                          <span
                            className={`text-xs px-2 py-0.5 rounded ${roleColors[user.role]}`}
                          >
                            {user.role}
                          </span>
                        ) : (
                          <Select
                            value={user.role}
                            onValueChange={(role) =>
                              updateUserRole(user.id, role)
                            }
                          >
                            <SelectTrigger className="w-[140px]">
                              <SelectValue>
                                <div className="flex items-center gap-2">
                                  <RoleIcon className="h-4 w-4" />
                                  <span className="capitalize">
                                    {user.role}
                                  </span>
                                </div>
                              </SelectValue>
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem value="admin">
                                <div className="flex items-center gap-2">
                                  <Shield className="h-4 w-4" /> Admin
                                </div>
                              </SelectItem>
                              <SelectItem value="manager">
                                <div className="flex items-center gap-2">
                                  <Users className="h-4 w-4" /> Manager
                                </div>
                              </SelectItem>
                              <SelectItem value="member">
                                <div className="flex items-center gap-2">
                                  <Users className="h-4 w-4" /> Member
                                </div>
                              </SelectItem>
                              <SelectItem value="viewer">
                                <div className="flex items-center gap-2">
                                  <Users className="h-4 w-4" /> Viewer
                                </div>
                              </SelectItem>
                            </SelectContent>
                          </Select>
                        )}
                      </td>
                      <td className="px-4 py-3 whitespace-nowrap">
                        <span
                          className={`text-xs px-2 py-0.5 rounded inline-flex items-center gap-1 ${statusColors[user.status]}`}
                        >
                          <StatusIcon className="h-3 w-3" />
                          {user.status.replace("_", " ")}
                        </span>
                      </td>
                      <td className="px-4 py-3 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                        {user.lastLoginAt
                          ? new Date(user.lastLoginAt).toLocaleString()
                          : "—"}
                      </td>
                      <td className="px-4 py-3 whitespace-nowrap">
                        {isPending ? (
                          <div className="flex items-center gap-2">
                            <Button
                              size="sm"
                              variant="default"
                              onClick={() => approveUser(user)}
                              className="bg-green-600 hover:bg-green-700"
                            >
                              <Check className="h-4 w-4 mr-1" /> Approve
                            </Button>
                            <Button
                              size="sm"
                              variant="destructive"
                              onClick={() => rejectUser(user)}
                            >
                              <X className="h-4 w-4 mr-1" /> Reject
                            </Button>
                          </div>
                        ) : user.status === "deactivated" ? (
                          <Button
                            size="sm"
                            variant="outline"
                            onClick={async () => {
                              if (!confirm(`Activate user ${user.email}?`))
                                return;
                              try {
                                await api.activateUser(user.id);
                                setUsers((prev) =>
                                  prev.map((u) =>
                                    u.id === user.id
                                      ? { ...u, status: "active" }
                                      : u
                                  )
                                );
                                toast.success("User Activated", {
                                  description: `${user.email} has been activated successfully.`,
                                });
                              } catch (e: any) {
                                toast.error("Activation Failed", {
                                  description:
                                    e?.message || "Failed to activate user",
                                });
                              }
                            }}
                            className="border-green-300 text-green-700 hover:bg-green-50 dark:hover:bg-green-900/20"
                          >
                            <Check className="h-4 w-4 mr-1" />
                            Activate
                          </Button>
                        ) : (
                          <Button
                            size="sm"
                            variant="outline"
                            onClick={async () => {
                              if (!confirm(`Deactivate user ${user.email}?`))
                                return;
                              try {
                                await api.deactivateUser(user.id);
                                setUsers((prev) =>
                                  prev.map((u) =>
                                    u.id === user.id
                                      ? { ...u, status: "deactivated" }
                                      : u
                                  )
                                );
                                toast.success("User Deactivated", {
                                  description: `${user.email} has been deactivated successfully.`,
                                });
                              } catch (e: any) {
                                toast.error("Deactivation Failed", {
                                  description:
                                    e?.message || "Failed to deactivate user",
                                });
                              }
                            }}
                            className="border-red-300 text-red-700 hover:bg-red-50 dark:hover:bg-red-900/20"
                          >
                            <UserX className="h-4 w-4 mr-1" />
                            Deactivate
                          </Button>
                        )}
                      </td>
                    </tr>
                  );
                })}
                {pageData.length === 0 && (
                  <tr>
                    <td
                      colSpan={6}
                      className="px-4 py-8 text-center text-sm text-gray-500 dark:text-gray-400"
                    >
                      No users found
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>

          {/* Pagination */}
          <div className="flex items-center justify-between mt-4">
            <div className="text-sm text-muted-foreground">
              Showing {(page - 1) * pageSize + Math.min(1, pageData.length)}-
              {Math.min(page * pageSize, filteredUsers.length)} of{" "}
              {filteredUsers.length}
            </div>
            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                disabled={page === 1}
                onClick={() => setPage((p) => Math.max(1, p - 1))}
              >
                Prev
              </Button>
              <div className="text-sm">
                Page {page} of {totalPages}
              </div>
              <Button
                variant="outline"
                size="sm"
                disabled={page === totalPages}
                onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
              >
                Next
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
    </AuthGuard>
  );
}

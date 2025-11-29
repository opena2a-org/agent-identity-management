"use client";

import { useCallback, useEffect, useState } from "react";
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
import { Skeleton } from "@/components/ui/skeleton";
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Textarea } from "@/components/ui/textarea";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { eventEmitter, Events } from "@/lib/events";
import { formatDateTime } from "@/lib/date-utils";
import { AuthGuard } from "@/components/auth-guard";
import {
  ShieldCheck,
  ShieldOff,
  RefreshCw,
  ChevronLeft,
  ChevronRight,
} from "lucide-react";

interface PendingVerification {
  id: string;
  agent_id: string;
  agent_name: string;
  action_type: string;
  resource: string;
  context?: Record<string, any>;
  risk_level: string;
  trust_score: number;
  status: string;
  requested_at: string;
  expires_at: string;
}

type DialogMode = "approve" | "deny" | null;

const riskStyles: Record<
  string,
  { badge: string; label: string; description: string }
> = {
  low: {
    badge: "bg-emerald-100 text-emerald-800 border border-emerald-200",
    label: "Low",
    description: "Read-only / informational",
  },
  medium: {
    badge: "bg-blue-100 text-blue-800 border border-blue-200",
    label: "Medium",
    description: "Modifies limited data",
  },
  high: {
    badge: "bg-amber-100 text-amber-900 border border-amber-200",
    label: "High",
    description: "Privileged access",
  },
  critical: {
    badge: "bg-red-100 text-red-900 border border-red-200",
    label: "Critical",
    description: "Potentially destructive",
  },
};

const statusStyles: Record<string, string> = {
  pending: "bg-yellow-100 text-yellow-900 border border-yellow-200",
  approved: "bg-emerald-100 text-emerald-800 border border-emerald-200",
  denied: "bg-red-100 text-red-900 border border-red-200",
};

const normalizeRisk = (risk?: string) =>
  (risk || "medium").toLowerCase() as keyof typeof riskStyles;

export default function PendingVerificationsPage() {
  const router = useRouter();
  const [authChecked, setAuthChecked] = useState(false);
  const [role, setRole] = useState<
    "admin" | "manager" | "member" | "viewer"
  >("viewer");
  const [loading, setLoading] = useState(true);
  const [verifications, setVerifications] = useState<PendingVerification[]>([]);
  const [refreshing, setRefreshing] = useState(false);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [dialogMode, setDialogMode] = useState<DialogMode>(null);
  const [selected, setSelected] = useState<PendingVerification | null>(null);
  const [reason, setReason] = useState("");
  const [reasonError, setReasonError] = useState("");
  const [actionLoading, setActionLoading] = useState(false);
  const [statusFilter, setStatusFilter] = useState<"all" | "pending" | "approved" | "denied">("pending");
  const [riskFilter, setRiskFilter] = useState<"all" | "low" | "medium" | "high" | "critical">("all");
  const [searchField, setSearchField] = useState<"all" | "agent" | "action" | "resource">("all");
  const [searchInput, setSearchInput] = useState("");
  const [searchQuery, setSearchQuery] = useState("");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [total, setTotal] = useState(0);
  const [totalPages, setTotalPages] = useState(1);
  const [statusCounts, setStatusCounts] = useState({
    pending: 0,
    approved: 0,
    denied: 0,
  });
  const [expandedContextId, setExpandedContextId] = useState<string | null>(null);

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
    const handle = setTimeout(() => {
      setSearchQuery(searchInput.trim());
    }, 350);
    return () => clearTimeout(handle);
  }, [searchInput]);

  useEffect(() => {
    setPage(1);
  }, [statusFilter, riskFilter, searchField, searchQuery]);

  useEffect(() => {
    if (page > totalPages) {
      setPage(Math.max(1, totalPages));
    }
  }, [page, totalPages]);

  const fetchPending = useCallback(
    async (options?: { skipLoading?: boolean }) => {
      if (!authChecked || role !== "admin") return;
      if (options?.skipLoading) {
        setRefreshing(true);
      } else {
        setLoading(true);
      }
      try {
        const payload = await api.getPendingVerifications({
          page,
          pageSize,
          status: statusFilter,
          risk: riskFilter,
          search: searchQuery || undefined,
          searchField,
        });

        setVerifications(
          (payload?.verifications || []).map((item) => ({
            ...item,
            risk_level: (item?.risk_level || "medium").toLowerCase(),
            status: (item?.status || "pending").toLowerCase(),
          }))
        );
        setStatusCounts({
          pending: payload?.status_counts?.pending ?? 0,
          approved: payload?.status_counts?.approved ?? 0,
          denied: payload?.status_counts?.denied ?? 0,
        });
        const responseTotal = payload?.pagination?.total ?? 0;
        const responseTotalPages =
          payload?.pagination?.total_pages ??
          Math.max(1, Math.ceil(responseTotal / pageSize));
        setTotal(responseTotal);
        setTotalPages(Math.max(1, responseTotalPages));
      } catch (error) {
        console.error("Failed to fetch pending verifications:", error);
        toast.error("Unable to load pending approvals", {
          description:
            error instanceof Error
              ? error.message
              : "Please try again in a moment.",
        });
      } finally {
        if (options?.skipLoading) {
          setRefreshing(false);
        } else {
          setLoading(false);
        }
      }
    },
    [
      authChecked,
      role,
      page,
      pageSize,
      statusFilter,
      riskFilter,
      searchQuery,
      searchField,
    ]
  );

  useEffect(() => {
    if (!authChecked || role !== "admin") return;
    fetchPending();
    const interval = setInterval(() => fetchPending({ skipLoading: true }), 30000);
    return () => clearInterval(interval);
  }, [authChecked, role, fetchPending]);

  const openDialog = (verification: PendingVerification, mode: DialogMode) => {
    setSelected(verification);
    setDialogMode(mode);
    setReason("");
    setDialogOpen(true);
  };

  const closeDialog = () => {
    if (actionLoading) return;
    setDialogOpen(false);
    setSelected(null);
    setDialogMode(null);
    setReason("");
    setReasonError("");
  };

  const handleAction = async () => {
    if (!selected || !dialogMode) return;
    if (!reason.trim()) {
      setReasonError(
        dialogMode === "approve"
          ? "Approval reason is required."
          : "Denial reason is required."
      );
      return;
    }
  setReasonError("");
  setActionLoading(true);
    try {
      if (dialogMode === "approve") {
        await api.approvePendingVerification(selected.id, reason.trim() || undefined);
        eventEmitter.emit(Events.VERIFICATION_APPROVED);
        toast.success("Action approved", {
          description: `${selected.agent_name || "Agent"} can continue "${
            selected.action_type
          }".`,
        });
      } else {
        await api.denyPendingVerification(selected.id, reason.trim());
        eventEmitter.emit(Events.VERIFICATION_DENIED);
        toast.success("Action denied", {
          description: `${selected.agent_name || "Agent"} request blocked.`,
        });
      }
      await fetchPending({ skipLoading: true });
    } catch (error) {
      console.error("Failed to update verification:", error);
      toast.error("Unable to update request", {
        description:
          error instanceof Error
            ? error.message
            : "Please try again or refresh the page.",
      });
    } finally {
    setActionLoading(false);
      closeDialog();
    }
  };

  const handleRefreshClick = () => {
    fetchPending({ skipLoading: true });
  };

  const showingStart = total === 0 ? 0 : (page - 1) * pageSize + 1;
  const showingEnd = total === 0 ? 0 : Math.min(total, page * pageSize);

  if (!authChecked || role !== "admin") {
    return null;
  }

  if (loading) {
    return (
      <div className="space-y-8">
        <div className="space-y-2">
          <Skeleton className="h-9 w-56" />
          <Skeleton className="h-4 w-80" />
        </div>
        <div className="grid gap-4 md:grid-cols-4">
          {[...Array(4)].map((_, i) => (
            <Card key={i}>
              <CardHeader className="pb-2">
                <Skeleton className="h-4 w-32" />
              </CardHeader>
              <CardContent>
                <Skeleton className="h-8 w-16" />
              </CardContent>
            </Card>
          ))}
        </div>
        <Card>
          <CardHeader>
            <Skeleton className="h-6 w-48" />
          </CardHeader>
          <CardContent className="space-y-3">
            {[...Array(3)].map((_, i) => (
              <div
                key={i}
                className="border rounded-lg p-4 space-y-3"
              >
                <Skeleton className="h-4 w-40" />
                <Skeleton className="h-3 w-64" />
                <Skeleton className="h-3 w-32" />
                <div className="flex gap-3">
                  <Skeleton className="h-9 w-24" />
                  <Skeleton className="h-9 w-24" />
                </div>
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
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h1 className="text-3xl font-bold">Pending Agent Approvals</h1>
            <p className="text-muted-foreground mt-1">
              Review actions that require human approval before execution.
            </p>
          </div>
          <Button
            variant="outline"
            onClick={handleRefreshClick}
            disabled={refreshing}
            className="flex items-center gap-2"
          >
            <RefreshCw
              className={`h-4 w-4 ${refreshing ? "animate-spin" : ""}`}
            />
            Refresh
          </Button>
        </div>

        <div className="grid gap-4 md:grid-cols-4">
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium">
                Total Requests
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{total}</div>
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
                {statusCounts.pending}
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium">
                Approved (24h)
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold text-green-600">
                {statusCounts.approved}
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium">
                Denied (24h)
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold text-red-600">
                {statusCounts.denied}
              </div>
            </CardContent>
          </Card>
        </div>

        <div className="space-y-4 rounded-lg border bg-muted/30 p-4">
          <div className="flex flex-col gap-3 sm:flex-row">
            <div className="sm:w-56">
              <p className="text-xs uppercase text-muted-foreground mb-1">
                Search Field
              </p>
              <Select
                value={searchField}
                onValueChange={(value) =>
                  setSearchField(value as "all" | "agent" | "action" | "resource")
                }
              >
                <SelectTrigger>
                  <SelectValue placeholder="All fields" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All fields</SelectItem>
                  <SelectItem value="agent">Agent name</SelectItem>
                  <SelectItem value="action">Action type</SelectItem>
                  <SelectItem value="resource">Resource</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="flex-1">
              <p className="text-xs uppercase text-muted-foreground mb-1">
                Search Query
              </p>
              <Input
                placeholder="Search approvals..."
                value={searchInput}
                onChange={(e) => setSearchInput(e.target.value)}
              />
            </div>
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            <div>
              <p className="text-xs uppercase text-muted-foreground mb-1">
                Status Filter
              </p>
              <Select
                value={statusFilter}
                onValueChange={(value) =>
                  setStatusFilter(
                    value as "all" | "pending" | "approved" | "denied"
                  )
                }
              >
                <SelectTrigger>
                  <SelectValue placeholder="Status" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All statuses</SelectItem>
                  <SelectItem value="pending">Pending</SelectItem>
                  <SelectItem value="approved">Approved</SelectItem>
                  <SelectItem value="denied">Denied</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div>
              <p className="text-xs uppercase text-muted-foreground mb-1">
                Risk Filter
              </p>
              <Select
                value={riskFilter}
                onValueChange={(value) =>
                  setRiskFilter(
                    value as "all" | "low" | "medium" | "high" | "critical"
                  )
                }
              >
                <SelectTrigger>
                  <SelectValue placeholder="Risk level" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All</SelectItem>
                  <SelectItem value="low">Low</SelectItem>
                  <SelectItem value="medium">Medium</SelectItem>
                  <SelectItem value="high">High</SelectItem>
                  <SelectItem value="critical">Critical</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
        </div>

        <Card>
          <CardHeader>
            <CardTitle>Requests Awaiting Approval</CardTitle>
            <CardDescription>
              Approve or deny actions that agents cannot perform automatically.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            {verifications.length === 0 ? (
              <div className="text-center py-12 text-muted-foreground">
                <ShieldCheck className="h-12 w-12 mx-auto mb-4 text-green-600" />
                <p className="text-lg font-semibold">No pending approvals</p>
                <p className="text-sm">
                  Actions requiring human approval will appear here in real time.
                </p>
              </div>
            ) : (
              verifications.map((verification) => {
                const riskKey = normalizeRisk(verification.risk_level);
                const riskMeta = riskStyles[riskKey];
                const statusClass =
                  statusStyles[verification.status] ||
                  "bg-muted text-muted-foreground border";
                const isExpanded = expandedContextId === verification.id;
                return (
                  <div
                    key={verification.id}
                    className="border rounded-lg p-4 space-y-4"
                  >
                    <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
                      <div className="space-y-1">
                        <div className="flex flex-wrap items-center gap-2">
                          <h3 className="text-lg font-semibold">
                            {verification.action_type}
                          </h3>
                          <Badge className={riskMeta.badge}>
                            {riskMeta.label} Risk
                          </Badge>
                          <Badge className={statusClass}>
                            {verification.status}
                          </Badge>
                        </div>
                        <p className="text-sm text-muted-foreground">
                          Requested by {verification.agent_name || "Unknown agent"} •{" "}
                          {formatDateTime(verification.requested_at)}
                        </p>
                        <p className="text-sm text-muted-foreground">
                          Resource: {verification.resource || "n/a"}
                        </p>
                      </div>
                      <div className="text-right">
                        <p className="text-sm text-muted-foreground">
                          Trust score (adjusted)
                        </p>
                        <p className="text-xl font-semibold">
                          {(verification.trust_score * 100).toFixed(1)}%
                        </p>
                      </div>
                    </div>

                    {verification.context && (
                      <div className="space-y-4 rounded-xl border bg-card/60 p-4 text-sm">
                        <div className="flex flex-col gap-1">
                          <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                            Context
                          </p>
                          <p className="text-base font-semibold text-foreground">
                            Runtime metadata captured for this request
                          </p>
                        </div>
                        {(() => {
                          const metadata = (verification.context ||
                            {}) as Record<string, any>;
                          const nestedContext =
                            metadata.context &&
                            typeof metadata.context === "object"
                              ? (metadata.context as Record<string, any>)
                              : null;
                          const baseTrustScore =
                            typeof metadata.trustScore === "number"
                              ? metadata.trustScore
                              : typeof verification.trust_score === "number"
                              ? verification.trust_score
                              : null;

                          const formatValue = (value: any) => {
                            if (value === null || value === undefined) {
                              return "—";
                            }
                            if (typeof value === "boolean") {
                              return value ? "true" : "false";
                            }
                            if (typeof value === "number") {
                              return Number.isFinite(value)
                                ? value.toString()
                                : "—";
                            }
                            if (typeof value === "object") {
                              return JSON.stringify(value);
                            }
                            return String(value);
                          };

                          const primaryCards = [
                            {
                              label: "Action Type",
                              value:
                                metadata.action_type ||
                                verification.action_type ||
                                "—",
                            },
                            {
                              label: "Resource",
                              value:
                                metadata.resource ||
                                verification.resource ||
                                "n/a",
                            },
                            {
                              label: "Auto Approved",
                              value:
                                typeof metadata.auto_approved === "boolean"
                                  ? metadata.auto_approved
                                    ? "true"
                                    : "false"
                                  : "false",
                            },
                            {
                              label: "Trust Score",
                              value: baseTrustScore,
                            },
                            {
                              label: "Verification ID",
                              value:
                                metadata.verification_id || verification.id,
                            },
                          ].filter(
                            (card) =>
                              card.value !== undefined &&
                              card.value !== null &&
                              card.value !== ""
                          );

                          const nestedEntries = nestedContext
                            ? Object.entries(nestedContext)
                            : [];

                          return (
                            <>
                              {primaryCards.length > 0 && (
                                <div className="grid gap-3 md:grid-cols-2">
                                  {primaryCards.map((card) => (
                                    <div
                                      key={card.label}
                                      className="rounded-lg bg-muted/50 p-3"
                                    >
                                      <p className="text-xs uppercase text-muted-foreground">
                                        {card.label}
                                      </p>
                                      <p className="text-sm font-semibold break-words">
                                        {card.label === "Trust Score"
                                          ? typeof card.value === "number"
                                            ? `${(card.value * 100).toFixed(
                                                1
                                              )}%`
                                            : "—"
                                          : formatValue(card.value)}
                                      </p>
                                    </div>
                                  ))}
                                </div>
                              )}
                              {nestedEntries.length > 0 && (
                                <div className="space-y-2">
                                  <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                                   
                                    <Button
                                      variant="outline"
                                      size="sm"
                                      className="h-8 px-3 text-xs uppercase"
                                      onClick={() =>
                                        setExpandedContextId((prev) =>
                                          prev === verification.id
                                            ? null
                                            : verification.id
                                        )
                                      }
                                    >
                                      {isExpanded ? "Hide details" : "Show details"}
                                    </Button>
                                  </div>
                                  {isExpanded && (
                                    <div className="grid gap-3 md:grid-cols-2">
                                      {nestedEntries.map(([key, value]) => (
                                        <div
                                          key={key}
                                          className="rounded-lg border border-dashed border-muted-foreground/40 p-3"
                                        >
                                          <p className="text-xs uppercase text-muted-foreground">
                                            {key.replace(/_/g, " ")}
                                          </p>
                                          <p className="text-sm font-medium break-words">
                                            {formatValue(value)}
                                          </p>
                                        </div>
                                      ))}
                                    </div>
                                  )}
                                </div>
                              )}
                            </>
                          );
                        })()}
                      </div>
                    )}

                    <div className="flex flex-col gap-2 sm:flex-row sm:justify-end">
                      <Button
                        variant="outline"
                        className="sm:w-auto"
                        onClick={() => openDialog(verification, "deny")}
                      >
                        <ShieldOff className="h-4 w-4 mr-2" />
                        Deny
                      </Button>
                      <Button
                        className="sm:w-auto"
                        onClick={() => openDialog(verification, "approve")}
                      >
                        <ShieldCheck className="h-4 w-4 mr-2" />
                        Approve
                      </Button>
                    </div>
                  </div>
                );
              })
            )}
            {verifications.length > 0 && (
              <div className="space-y-3 border-t border-border/60 pt-4">
                <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                  <div className="flex items-center gap-2 text-sm text-muted-foreground">
                    <span>Rows per page</span>
                    <Select
                      value={String(pageSize)}
                      onValueChange={(value) => {
                        setPageSize(Number(value));
                        setPage(1);
                      }}
                    >
                      <SelectTrigger className="w-24">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {[5, 10, 20, 50].map((size) => (
                          <SelectItem key={size} value={String(size)}>
                            {size}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="text-sm text-muted-foreground">
                    Showing {showingStart}-{showingEnd} of {total}
                  </div>
                </div>
                <div className="flex items-center justify-end gap-2">
                  <Button
                    variant="outline"
                    className="flex items-center gap-1"
                    onClick={() => setPage((prev) => Math.max(1, prev - 1))}
                    disabled={page <= 1}
                  >
                    <ChevronLeft className="h-4 w-4" />
                    Previous
                  </Button>
                  <Button
                    variant="outline"
                    className="flex items-center gap-1"
                    onClick={() => setPage((prev) => prev + 1)}
                    disabled={page >= totalPages}
                  >
                    Next
                    <ChevronRight className="h-4 w-4" />
                  </Button>
                </div>
              </div>
            )}
          </CardContent>
        </Card>

        <AlertDialog open={dialogOpen} onOpenChange={(open) => {
          if (!open) {
            closeDialog();
          } else {
            setDialogOpen(true);
          }
        }}>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>
                {dialogMode === "approve"
                  ? "Approve Agent Action"
                  : "Deny Agent Action"}
              </AlertDialogTitle>
              <AlertDialogDescription>
                {dialogMode === "approve"
                  ? "Approving will immediately allow the agent to continue this action."
                  : "Denying will permanently block this action and notify the agent."}
              </AlertDialogDescription>
            </AlertDialogHeader>
            <div className="space-y-3">
              <div className="text-sm">
                <p className="font-medium">{selected?.action_type}</p>
                <p className="text-muted-foreground">
                  Agent: {selected?.agent_name || "Unknown"} • Resource:{" "}
                  {selected?.resource || "n/a"}
                </p>
              </div>
              <div>
                <p className="text-sm font-medium mb-2 flex items-center gap-1">
                  {dialogMode === "approve"
                    ? "Approval reason"
                    : "Denial reason"}
                  <span className="text-red-500">*</span>
                </p>
                <div className="space-y-1">
                  <Textarea
                    placeholder={
                      dialogMode === "approve"
                        ? "Provide a reason (required, shared in audit logs)"
                        : "Provide a reason (required, shared with the requesting agent)"
                    }
                    value={reason}
                    onChange={(e) => {
                      setReason(e.target.value);
                      if (reasonError) setReasonError("");
                    }}
                    disabled={actionLoading}
                    className={reasonError ? "border-red-500 focus-visible:ring-red-500" : ""}
                  />
                  {reasonError && (
                    <p className="text-xs text-red-600">{reasonError}</p>
                  )}
                </div>
              </div>
            </div>
            <AlertDialogFooter>
              <AlertDialogCancel disabled={actionLoading} onClick={closeDialog}>
                Cancel
              </AlertDialogCancel>
              <Button
                type="button"
                disabled={actionLoading}
                className={
                  dialogMode === "deny"
                    ? "bg-red-600 hover:bg-red-700 focus:ring-red-600"
                    : "bg-green-600 hover:bg-green-700 focus:ring-green-600"
                }
                onClick={handleAction}
              >
                {actionLoading ? "Processing..." : dialogMode === "approve" ? "Approve" : "Deny"}
              </Button>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </div>
    </AuthGuard>
  );
}


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
  Check,
  X,
  Clock,
  Shield,
  AlertCircle,
  CheckCircle2,
  XCircle,
} from "lucide-react";
import { api } from "@/lib/api";
import { formatDate } from "@/lib/date-utils";
import { Skeleton } from "@/components/ui/skeleton";
import { AuthGuard } from "@/components/auth-guard";
import { eventEmitter, Events } from "@/lib/events";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";

interface CapabilityRequest {
  id: string;
  agent_id: string;
  agent_name: string;
  agent_display_name: string;
  capability_type: string;
  reason: string;
  status: "pending" | "approved" | "rejected";
  requested_by: string;
  requested_by_email: string;
  reviewed_by?: string;
  reviewed_by_email?: string;
  requested_at: string;
  reviewed_at?: string;
}

const statusColors = {
  pending: "bg-yellow-100 text-yellow-800 border-yellow-200",
  approved: "bg-green-100 text-green-800 border-green-200",
  rejected: "bg-red-100 text-red-800 border-red-200",
};

const statusIcons = {
  pending: Clock,
  approved: CheckCircle2,
  rejected: XCircle,
};

export default function CapabilityRequestsPage() {
  const router = useRouter();
  const [authChecked, setAuthChecked] = useState(false);
  const [role, setRole] = useState<"admin" | "manager" | "member" | "viewer">(
    "viewer"
  );

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
  const [requests, setRequests] = useState<CapabilityRequest[]>([]);
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");
  const [filterStatus, setFilterStatus] = useState<string>("all");
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [pendingAction, setPendingAction] = useState<{
    type: "approve" | "reject";
    request: CapabilityRequest | null;
  }>({ type: "approve", request: null });
  const [actionLoading, setActionLoading] = useState(false);
  const [feedback, setFeedback] = useState<
    { type: "success" | "error"; message: string } | null
  >(null);

  useEffect(() => {
    fetchRequests();
  }, []);

  const fetchRequests = async () => {
    try {
      const data = await api.getCapabilityRequests();
      setRequests(data || []);
    } catch (error) {
      console.error("Failed to fetch capability requests:", error);
      setRequests([]);
    } finally {
      setLoading(false);
    }
  };

  const handleActionClick = (
    request: CapabilityRequest,
    type: "approve" | "reject"
  ) => {
    setPendingAction({ type, request });
    setConfirmOpen(true);
  };

  const resetModalState = () => {
    setConfirmOpen(false);
    setActionLoading(false);
    setPendingAction({ type: "approve", request: null });
  };

  const handleConfirmAction = async () => {
    if (!pendingAction.request) {
      return;
    }

    setActionLoading(true);
    setFeedback(null);

    try {
      if (pendingAction.type === "approve") {
        await api.approveCapabilityRequest(pendingAction.request.id);
        eventEmitter.emit(Events.CAPABILITY_REQUEST_APPROVED);
        setFeedback({
          type: "success",
          message: `Capability request for ${pendingAction.request.agent_display_name} approved.`,
        });
      } else {
        await api.rejectCapabilityRequest(pendingAction.request.id);
        eventEmitter.emit(Events.CAPABILITY_REQUEST_REJECTED);
        setFeedback({
          type: "success",
          message: `Capability request for ${pendingAction.request.agent_display_name} rejected.`,
        });
      }

      await fetchRequests();
      resetModalState();
    } catch (error) {
      console.error("Failed to process request:", error);
      setFeedback({
        type: "error",
        message: `Failed to ${
          pendingAction.type === "approve" ? "approve" : "reject"
        } capability request: ${(error as Error).message}`,
      });
      setActionLoading(false);
    }
  };

  const filteredRequests = requests.filter((request) => {
    const matchesSearch =
      request.agent_name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      request.agent_display_name
        .toLowerCase()
        .includes(searchQuery.toLowerCase()) ||
      request.capability_type
        .toLowerCase()
        .includes(searchQuery.toLowerCase()) ||
      request.requested_by_email
        .toLowerCase()
        .includes(searchQuery.toLowerCase());

    const matchesStatus =
      filterStatus === "all" || request.status === filterStatus;

    return matchesSearch && matchesStatus;
  });

  const pendingCount = requests.filter((r) => r.status === "pending").length;
  const approvedCount = requests.filter((r) => r.status === "approved").length;
  const rejectedCount = requests.filter((r) => r.status === "rejected").length;

  if (loading) {
    return (
      <div className="space-y-6">
        <div className="space-y-2">
          <Skeleton className="h-9 w-56" />
          <Skeleton className="h-4 w-96" />
        </div>
        <div className="grid gap-4 md:grid-cols-3">
          {[...Array(3)].map((_, i) => (
            <Card key={i}>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <Skeleton className="h-4 w-32" />
                <Skeleton className="h-4 w-4" />
              </CardHeader>
              <CardContent>
                <Skeleton className="h-8 w-16 mb-2" />
                <Skeleton className="h-3 w-24" />
              </CardContent>
            </Card>
          ))}
        </div>
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <Skeleton className="h-6 w-48" />
              <div className="flex items-center gap-2">
                <Skeleton className="h-10 flex-1 w-64" />
                <Skeleton className="h-10 w-32" />
              </div>
            </div>
          </CardHeader>
          <CardContent className="space-y-4">
            {[...Array(4)].map((_, i) => (
              <div
                key={i}
                className="flex items-start justify-between p-4 border border-gray-200 rounded-lg"
              >
                <div className="flex-1 space-y-3">
                  <div className="flex items-center gap-2">
                    <Skeleton className="h-5 w-24 rounded-full" />
                    <Skeleton className="h-5 w-40" />
                  </div>
                  <Skeleton className="h-4 w-full" />
                  <div className="flex items-center gap-4">
                    <Skeleton className="h-3 w-32" />
                    <Skeleton className="h-3 w-32" />
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <Skeleton className="h-9 w-20" />
                  <Skeleton className="h-9 w-20" />
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
        <div>
          <h1 className="text-3xl font-bold">Capability Requests</h1>
          <p className="text-muted-foreground mt-1">
            Review and approve agent capability expansion requests
          </p>
          <div className="mt-3 p-3 bg-blue-50 dark:bg-blue-950/20 border border-blue-200 dark:border-blue-800 rounded-md">
            <p className="text-sm text-blue-900 dark:text-blue-200">
              <strong>Auto-Grant Architecture:</strong> Initial capabilities are
              automatically granted during agent registration. This page handles
              requests for <strong>additional capabilities</strong> after
              registration.
            </p>
          </div>
        </div>

        {feedback && (
          <Alert
            variant={feedback.type === "error" ? "destructive" : "default"}
            className="flex items-start gap-2"
          >
            <AlertTitle>
              {feedback.type === "error" ? "Action Failed" : "Action Complete"}
            </AlertTitle>
            <AlertDescription>{feedback.message}</AlertDescription>
          </Alert>
        )}

        {/* Stats */}
        <div className="grid gap-4 md:grid-cols-4">
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium">
                Total Requests
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{requests.length}</div>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium">
                Pending Review
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
              <CardTitle className="text-sm font-medium">Approved</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold text-green-600">
                {approvedCount}
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium">Rejected</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold text-red-600">
                {rejectedCount}
              </div>
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
                placeholder="Search by agent name or capability..."
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
                <SelectItem value="approved">Approved</SelectItem>
                <SelectItem value="rejected">Rejected</SelectItem>
              </SelectContent>
            </Select>
          </CardContent>
        </Card>

        {/* Requests List */}
        <Card>
          <CardHeader>
            <CardTitle>Capability Requests ({filteredRequests.length})</CardTitle>
            <CardDescription>
              Review agent capability expansion requests and approve or reject
              them
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              {filteredRequests.map((request) => {
                const StatusIcon = statusIcons[request.status];
                const isPending = request.status === "pending";

                return (
                  <div
                    key={request.id}
                    className="flex items-center justify-between p-4 border rounded-lg hover:bg-accent/50 transition-colors"
                  >
                    <div className="flex items-center gap-4 flex-1">
                      <div className="h-10 w-10 rounded-full bg-gradient-to-br from-purple-500 to-pink-600 flex items-center justify-center text-white">
                        <Shield className="h-5 w-5" />
                      </div>

                      <div className="flex-1">
                        <div className="flex items-center gap-2">
                          <p className="font-medium">
                            {request.agent_display_name}
                          </p>
                          <Badge variant="outline" className="text-xs">
                            {request.agent_name}
                          </Badge>
                          <Badge
                            className={`text-xs ${statusColors[request.status]}`}
                          >
                            <StatusIcon className="h-3 w-3 mr-1" />
                            {request.status.charAt(0).toUpperCase() +
                              request.status.slice(1)}
                          </Badge>
                        </div>
                        <div className="flex items-center gap-2 mt-1">
                          <span className="text-sm font-mono bg-muted px-2 py-0.5 rounded">
                            {request.capability_type}
                          </span>
                        </div>
                        <p className="text-sm text-muted-foreground mt-1">
                          <strong>Reason:</strong> {request.reason}
                        </p>
                        <p className="text-xs text-muted-foreground mt-1">
                          Requested by {request.requested_by_email} •{" "}
                          {formatDate(request.requested_at)}
                        </p>
                      </div>
                    </div>

                    <div className="flex items-center gap-4">
                      {request.reviewed_at && !isPending && (
                        <div className="text-right text-xs text-muted-foreground">
                          <p>Reviewed by</p>
                          <p className="font-medium">
                            {request.reviewed_by_email}
                          </p>
                          <p>{formatDate(request.reviewed_at)}</p>
                        </div>
                      )}

                      {isPending && (
                        <div className="flex gap-2">
                          <Button
                            size="sm"
                            variant="default"
                            onClick={() => handleActionClick(request, "approve")}
                            className="bg-green-600 hover:bg-green-700"
                          >
                            <Check className="h-4 w-4 mr-1" />
                            Approve
                          </Button>
                          <Button
                            size="sm"
                            variant="destructive"
                            onClick={() => handleActionClick(request, "reject")}
                          >
                            <X className="h-4 w-4 mr-1" />
                            Reject
                          </Button>
                        </div>
                      )}
                    </div>
                  </div>
                );
              })}

              {filteredRequests.length === 0 && (
                <div className="text-center py-12 text-muted-foreground">
                  <AlertCircle className="h-12 w-12 mx-auto mb-4 opacity-50" />
                  <p className="text-lg font-medium">
                    No capability requests found
                  </p>
                  <p className="text-sm mt-1">
                    {searchQuery || filterStatus !== "all"
                      ? "Try adjusting your search or filter criteria"
                      : "Capability requests will appear here when agents request additional permissions"}
                  </p>
                </div>
              )}
            </div>
          </CardContent>
        </Card>
        <AlertDialog
          open={confirmOpen}
          onOpenChange={(open) => {
            if (!open && !actionLoading) {
              resetModalState();
            }
            setConfirmOpen(open);
          }}
        >
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>
                {pendingAction.type === "approve"
                  ? "Approve Capability Request"
                  : "Reject Capability Request"}
              </AlertDialogTitle>
              <AlertDialogDescription>
                {pendingAction.request ? (
                  <>
                    You are about to{" "}
                    <strong>
                      {pendingAction.type === "approve" ? "approve" : "reject"}
                    </strong>{" "}
                    the capability request by{" "}
                    <strong>{pendingAction.request.agent_display_name}</strong> for the
                    capability (<strong>{pendingAction.request.capability_type}</strong>)
                    . This action cannot be undone.
                  </>
                ) : (
                  "Select a request to continue."
                )}
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel disabled={actionLoading}>
                Cancel
              </AlertDialogCancel>
              <AlertDialogAction
                onClick={handleConfirmAction}
                disabled={actionLoading || !pendingAction.request}
                className={
                  pendingAction.type === "reject"
                    ? "bg-red-600 hover:bg-red-700 focus:ring-red-600"
                    : "bg-green-600 hover:bg-green-700 focus:ring-green-600"
                }
              >
                {actionLoading
                  ? "Processing..."
                  : pendingAction.type === "approve"
                  ? "Confirm"
                  : "Decline"}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </div>
    </AuthGuard>
  );
}

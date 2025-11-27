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
import { Switch } from "@/components/ui/switch";
import {
  Shield,
  AlertTriangle,
  Check,
  X,
  Lock,
  Eye,
  AlertOctagon,
  Info,
} from "lucide-react";
import { api } from "@/lib/api";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { AuthGuard } from "@/components/auth-guard";

interface SecurityPolicy {
  id: string;
  organizationId: string;
  name: string;
  description: string;
  policyType: string;
  enforcementAction: "alert_only" | "block_and_alert" | "allow";
  severityThreshold: string;
  rules: Record<string, any>;
  appliesTo: string;
  isEnabled: boolean;
  priority: number;
  createdBy: string;
  createdAt: string;
  updatedAt: string;
}

const enforcementColors = {
  alert_only: "bg-yellow-100 text-yellow-800 border-yellow-300",
  block_and_alert: "bg-red-100 text-red-800 border-red-300",
  allow: "bg-green-100 text-green-800 border-green-300",
};

const enforcementIcons = {
  alert_only: Eye,
  block_and_alert: Lock,
  allow: Check,
};

const enforcementLabels = {
  alert_only: "Alert Only",
  block_and_alert: "Block & Alert",
  allow: "Allow",
};

const policyTypeLabels: Record<string, string> = {
  capability_violation: "Capability Violation",
  trust_score_low: "Low Trust Score",
  data_exfiltration: "Data Exfiltration",
  unusual_activity: "Unusual Activity",
  unauthorized_access: "Unauthorized Access",
  config_drift: "Configuration Drift",
  auth_failure: "Authentication Failure",
};

export default function SecurityPoliciesPage() {
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
  const [policies, setPolicies] = useState<SecurityPolicy[]>([]);
  const [loading, setLoading] = useState(true);
  const [showBlockingWarning, setShowBlockingWarning] = useState(false);
  const [pendingPolicyToggle, setPendingPolicyToggle] = useState<{
    policyId: string;
    policyName: string;
    currentAction: string;
  } | null>(null);
  const [pendingEnforcementChange, setPendingEnforcementChange] = useState<{
    policyId: string;
    policyName: string;
    newAction: "alert_only" | "block_and_alert" | "allow";
  } | null>(null);

  useEffect(() => {
    fetchPolicies();
  }, []);

  const fetchPolicies = async () => {
    try {
      const data = await api.getSecurityPolicies();
      // Sort by priority (highest first)
      const sorted = data.sort(
        (a: SecurityPolicy, b: SecurityPolicy) => b.priority - a.priority
      );
      setPolicies(sorted);
    } catch (error) {
      console.error("Failed to fetch security policies:", error);
    } finally {
      setLoading(false);
    }
  };

  const togglePolicy = async (policyId: string, currentlyEnabled: boolean) => {
    try {
      await api.toggleSecurityPolicy(policyId, !currentlyEnabled);
      setPolicies(
        policies.map((p) =>
          p.id === policyId ? { ...p, isEnabled: !currentlyEnabled } : p
        )
      );
    } catch (error) {
      console.error("Failed to toggle policy:", error);
      alert("Failed to toggle policy. Please try again.");
    }
  };

  const handlePolicyToggle = (policy: SecurityPolicy, newEnabled: boolean) => {
    // If enabling and enforcement is blocking mode, show warning
    if (newEnabled && policy.enforcementAction === "block_and_alert") {
      setPendingPolicyToggle({
        policyId: policy.id,
        policyName: policy.name,
        currentAction: policy.enforcementAction,
      });
      setShowBlockingWarning(true);
    } else {
      // Safe to toggle directly
      togglePolicy(policy.id, policy.isEnabled);
    }
  };

  const confirmBlockingMode = async () => {
    if (pendingPolicyToggle) {
      await togglePolicy(pendingPolicyToggle.policyId, false); // Toggle from disabled to enabled
      setShowBlockingWarning(false);
      setPendingPolicyToggle(null);
    }
  };

  const cancelBlockingMode = () => {
    setShowBlockingWarning(false);
    setPendingPolicyToggle(null);
    setPendingEnforcementChange(null);
  };

  const changeEnforcementAction = async (
    policyId: string,
    newAction: "alert_only" | "block_and_alert" | "allow"
  ) => {
    try {
      const policy = policies.find((p) => p.id === policyId);
      if (!policy) return;

      // Update the policy with new enforcement action
      await api.updateSecurityPolicy(policyId, {
        name: policy.name,
        description: policy.description,
        policyType: policy.policyType,
        enforcementAction: newAction,
        severityThreshold: policy.severityThreshold,
        rules: policy.rules,
        appliesTo: policy.appliesTo,
        isEnabled: policy.isEnabled,
        priority: policy.priority,
      });

      // Update local state
      setPolicies(
        policies.map((p) =>
          p.id === policyId ? { ...p, enforcementAction: newAction } : p
        )
      );
    } catch (error) {
      console.error("Failed to update enforcement action:", error);
      alert("Failed to update enforcement action. Please try again.");
    }
  };

  const handleEnforcementChange = (
    policy: SecurityPolicy,
    newAction: "alert_only" | "block_and_alert" | "allow"
  ) => {
    // If changing to blocking mode, show warning
    if (newAction === "block_and_alert") {
      setPendingEnforcementChange({
        policyId: policy.id,
        policyName: policy.name,
        newAction,
      });
      setShowBlockingWarning(true);
    } else {
      // Safe to change directly
      changeEnforcementAction(policy.id, newAction);
    }
  };

  const confirmEnforcementChange = async () => {
    if (pendingEnforcementChange) {
      await changeEnforcementAction(
        pendingEnforcementChange.policyId,
        pendingEnforcementChange.newAction
      );
      setShowBlockingWarning(false);
      setPendingEnforcementChange(null);
    } else if (pendingPolicyToggle) {
      await togglePolicy(pendingPolicyToggle.policyId, false);
      setShowBlockingWarning(false);
      setPendingPolicyToggle(null);
    }
  };

  const enabledCount = policies.filter((p) => p.isEnabled).length;
  const blockingCount = policies.filter(
    (p) => p.isEnabled && p.enforcementAction === "block_and_alert"
  ).length;
  const monitoringCount = policies.filter(
    (p) => p.isEnabled && p.enforcementAction === "alert_only"
  ).length;

  if (loading) {
    return (
      <div className="space-y-6">
        <div className="space-y-2">
          <Skeleton className="h-9 w-48" />
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
                <Skeleton className="h-3 w-40" />
              </CardContent>
            </Card>
          ))}
        </div>
        <div className="grid gap-6 md:grid-cols-2">
          {[...Array(4)].map((_, i) => (
            <Card key={i}>
              <CardHeader>
                <div className="flex items-start justify-between">
                  <div className="space-y-2 flex-1">
                    <Skeleton className="h-5 w-48" />
                    <Skeleton className="h-4 w-full" />
                  </div>
                  <Skeleton className="h-6 w-16" />
                </div>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="flex items-center justify-between">
                  <Skeleton className="h-4 w-32" />
                  <Skeleton className="h-6 w-24 rounded-full" />
                </div>
                <div className="flex items-center justify-between">
                  <Skeleton className="h-4 w-24" />
                  <Skeleton className="h-6 w-20 rounded-full" />
                </div>
                <Skeleton className="h-10 w-full" />
              </CardContent>
            </Card>
          ))}
        </div>
      </div>
    );
  }

  return (
    <AuthGuard>
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold">Security Policies</h1>
          <p className="text-muted-foreground mt-1">
            Configure security enforcement for capability violations. Actively enforced policies block unauthorized agent actions in real-time.
          </p>
        </div>

      {/* Stats */}
      <div className="grid gap-4 md:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">
              Total Policies
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{policies.length}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Enabled</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-green-600">
              {enabledCount}
            </div>
          </CardContent>
        </Card>
        <Card className="bg-yellow-50 dark:bg-yellow-950/20 border-yellow-200 dark:border-yellow-800">
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium flex items-center gap-2">
              <Eye className="h-4 w-4 text-yellow-600" />
              Monitoring
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-yellow-600">
              {monitoringCount}
            </div>
            <p className="text-xs text-muted-foreground mt-1">
              Alert only mode
            </p>
          </CardContent>
        </Card>
        <Card className="bg-red-50 dark:bg-red-950/20 border-red-200 dark:border-red-800">
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium flex items-center gap-2">
              <Lock className="h-4 w-4 text-red-600" />
              Blocking
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-red-600">
              {blockingCount}
            </div>
            <p className="text-xs text-muted-foreground mt-1">
              Block & alert mode
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Info Banner */}
      <Card className="bg-blue-50 dark:bg-blue-950/20 border-blue-200 dark:border-blue-800">
        <CardContent className="pt-6">
          <div className="flex gap-3">
            <Info className="h-5 w-5 text-blue-600 flex-shrink-0 mt-0.5" />
            <div className="space-y-2 text-sm">
              <p className="font-medium text-blue-900 dark:text-blue-100">
                Security Policy Enforcement
              </p>
              <p className="text-blue-800 dark:text-blue-200">
                Security policies protect your agents from various threats including capability violations,
                trust score degradation, data exfiltration attempts, and unusual activity patterns.
              </p>
              <ul className="space-y-1 text-blue-800 dark:text-blue-200 mt-2">
                <li className="flex items-center gap-2">
                  <Eye className="h-4 w-4" />
                  <strong>Alert Only:</strong> Log violations but allow actions (monitoring mode)
                </li>
                <li className="flex items-center gap-2">
                  <Lock className="h-4 w-4" />
                  <strong>Block & Alert:</strong> Prevent violations and create alerts (enforcement mode - recommended)
                </li>
              </ul>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Policies List */}
      <Card>
        <CardHeader>
          <CardTitle>Security Policies ({policies.length})</CardTitle>
          <CardDescription>
            Policies are evaluated by priority (highest first). Toggle policies
            on/off or adjust enforcement actions.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            {policies.length === 0 && (
              <div className="text-center py-12 text-muted-foreground">
                <Shield className="h-12 w-12 mx-auto mb-4 opacity-50" />
                <p className="text-lg font-medium">
                  No security policies configured
                </p>
                <p className="text-sm mt-1">
                  Default policies are created automatically for new
                  organizations.
                </p>
              </div>
            )}

            {policies.map((policy) => {
              const EnforcementIcon =
                enforcementIcons[policy.enforcementAction] || Shield;
              const isBlocking =
                policy.enforcementAction === "block_and_alert";

              return (
                <div
                  key={policy.id}
                  className={`p-6 border rounded-lg transition-all ${
                    policy.isEnabled
                      ? "bg-white dark:bg-gray-800 border-gray-300 dark:border-gray-600"
                      : "bg-gray-50 dark:bg-gray-900 border-gray-200 dark:border-gray-700 opacity-60"
                  }`}
                >
                  <div className="flex items-start justify-between">
                    <div className="flex-1 space-y-3">
                      <div className="flex items-center gap-3">
                        <div
                          className={`p-2 rounded-lg ${
                            policy.isEnabled
                              ? "bg-green-100 dark:bg-green-900/30"
                              : "bg-gray-100 dark:bg-gray-800"
                          }`}
                        >
                          <Shield
                            className={`h-5 w-5 ${
                              policy.isEnabled
                                ? "text-green-600 dark:text-green-400"
                                : "text-gray-400"
                            }`}
                          />
                        </div>
                        <div className="flex-1">
                          <div className="flex items-center gap-2 flex-wrap">
                            <h3 className="font-semibold text-lg">
                              {policy.name}
                            </h3>
                            <Badge
                              className={`${enforcementColors[policy.enforcementAction] || "bg-gray-100 text-gray-800 border-gray-300"} border`}
                            >
                              <EnforcementIcon className="h-3 w-3 mr-1" />
                              {enforcementLabels[policy.enforcementAction] ||
                                policy.enforcementAction}
                            </Badge>
                            <Badge variant="outline" className="text-xs">
                              Priority: {policy.priority}
                            </Badge>
                            <Badge variant="outline" className="text-xs">
                              {policyTypeLabels[policy.policyType] ||
                                policy.policyType}
                            </Badge>
                          </div>
                          <p className="text-sm text-muted-foreground mt-1">
                            {policy.description}
                          </p>
                        </div>
                      </div>

                      <div className="flex flex-wrap gap-4 text-xs text-muted-foreground pl-14">
                        <div>
                          <span className="font-medium">Applies to:</span>{" "}
                          <span className="capitalize">
                            {policy.appliesTo.replace("_", " ")}
                          </span>
                        </div>
                        <div>
                          <span className="font-medium">
                            Severity threshold:
                          </span>{" "}
                          <span className="capitalize">
                            {policy.severityThreshold}
                          </span>
                        </div>
                        {policy.rules &&
                          Object.keys(policy.rules).length > 0 && (
                            <div>
                              <span className="font-medium">Rules:</span>{" "}
                              <span>
                                {Object.keys(policy.rules).length} configured
                              </span>
                            </div>
                          )}
                      </div>

                      {isBlocking && policy.isEnabled && (
                        <div className="flex items-center gap-2 pl-14 p-3 bg-red-50 dark:bg-red-950/20 rounded-md border border-red-200 dark:border-red-800">
                          <AlertOctagon className="h-4 w-4 text-red-600" />
                          <p className="text-xs text-red-800 dark:text-red-200 font-medium">
                            BLOCKING MODE ACTIVE: This policy will prevent agent
                            actions in real-time
                          </p>
                        </div>
                      )}
                    </div>

                    <div className="flex flex-col gap-4 ml-6">
                      {/* Enforcement Action Selector */}
                      <div className="min-w-[180px]">
                        <p className="text-xs text-muted-foreground mb-2">
                          Enforcement Mode
                        </p>
                        <Select
                          value={policy.enforcementAction}
                          onValueChange={(
                            value: "alert_only" | "block_and_alert" | "allow"
                          ) => handleEnforcementChange(policy, value)}
                        >
                          <SelectTrigger className="w-full">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="alert_only">
                              <div className="flex items-center gap-2">
                                <Eye className="h-4 w-4 text-yellow-600" />
                                <span>Alert Only</span>
                              </div>
                            </SelectItem>
                            <SelectItem value="block_and_alert">
                              <div className="flex items-center gap-2">
                                <Lock className="h-4 w-4 text-red-600" />
                                <span>Block & Alert</span>
                              </div>
                            </SelectItem>
                            <SelectItem value="allow">
                              <div className="flex items-center gap-2">
                                <Check className="h-4 w-4 text-green-600" />
                                <span>Allow</span>
                              </div>
                            </SelectItem>
                          </SelectContent>
                        </Select>
                      </div>

                      {/* Enable/Disable Toggle */}
                      <div className="flex items-center gap-4">
                        <div className="text-right mr-2">
                          <p className="text-xs text-muted-foreground">
                            Status
                          </p>
                          <p
                            className={`text-sm font-medium ${
                              policy.isEnabled
                                ? "text-green-600"
                                : "text-gray-500"
                            }`}
                          >
                            {policy.isEnabled ? "Enabled" : "Disabled"}
                          </p>
                        </div>
                        <Switch
                          checked={policy.isEnabled}
                          onCheckedChange={(checked) =>
                            handlePolicyToggle(policy, checked)
                          }
                          className="data-[state=checked]:bg-green-600"
                        />
                      </div>
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        </CardContent>
      </Card>

      {/* Blocking Mode Warning Dialog */}
      <Dialog open={showBlockingWarning} onOpenChange={setShowBlockingWarning}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2 text-red-600">
              <AlertTriangle className="h-5 w-5" />
              Enable Blocking Mode?
            </DialogTitle>
            <div className="text-sm text-muted-foreground space-y-3 pt-4">
              <div className="font-medium text-foreground">
                You are about to{" "}
                {pendingEnforcementChange ? "switch" : "enable"}{" "}
                <strong>
                  "
                  {pendingEnforcementChange?.policyName ||
                    pendingPolicyToggle?.policyName}
                  "
                </strong>{" "}
                {pendingEnforcementChange ? "to" : "in"} blocking mode.
              </div>
              <div className="bg-red-50 dark:bg-red-950/20 border border-red-200 dark:border-red-800 p-4 rounded-md space-y-2">
                 <div className="text-sm font-semibold text-red-800 dark:text-red-200">
                  ⚠️ Warning: Production Impact
                </div>
                <ul className="text-sm text-red-700 dark:text-red-300 space-y-1 list-disc list-inside">
                  <li>
                    This policy will{" "}
                    <strong>block agent actions in real-time</strong>
                  </li>
                  <li>
                    Blocked actions will return <strong>403 Forbidden</strong>{" "}
                    responses
                  </li>
                  <li>
                    This may <strong>disrupt production workflows</strong>
                  </li>
                  <li>Consider testing in "Alert Only" mode first</li>
                </ul>
              </div>
              <div className="text-sm">
                Are you sure you want to{" "}
                {pendingEnforcementChange ? "switch to" : "enable"} blocking
                mode for this policy?
              </div>
            </div>
          </DialogHeader>
          <DialogFooter className="gap-2">
            <Button variant="outline" onClick={cancelBlockingMode}>
              <X className="h-4 w-4 mr-2" />
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={confirmEnforcementChange}
              className="bg-red-600 hover:bg-red-700"
            >
              <AlertOctagon className="h-4 w-4 mr-2" />
              {pendingEnforcementChange
                ? "Switch to Blocking Mode"
                : "Enable Blocking Mode"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      </div>
    </AuthGuard>
  );
}

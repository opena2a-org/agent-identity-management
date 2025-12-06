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
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
  Shield,
  AlertTriangle,
  Check,
  X,
  Lock,
  Eye,
  AlertOctagon,
  Info,
  Plus,
  Server,
  Bot,
  Ban,
  CheckCircle,
  FileWarning,
  Edit,
  Trash2,
  Loader2,
  Filter,
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

// Unified policy interface for both agent and MCP policies
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

// Agent policy types
const AGENT_POLICY_TYPES = [
  "capability_violation",
  "trust_score_low",
  "data_exfiltration",
  "unusual_activity",
  "unauthorized_access",
  "config_drift",
  "auth_failure",
];

// MCP policy types
const MCP_POLICY_TYPES = [
  "mcp_allowlist",
  "mcp_blocklist",
  "mcp_capabilities",
  "mcp_unverified",
];

const MCP_POLICY_TYPE_CONFIGS = [
  {
    type: "mcp_allowlist",
    label: "MCP Allowlist",
    icon: CheckCircle,
    color: "text-green-600",
    bgColor: "bg-green-100 dark:bg-green-900/30",
    description: "Only allow listed MCP servers",
  },
  {
    type: "mcp_blocklist",
    label: "MCP Blocklist",
    icon: Ban,
    color: "text-red-600",
    bgColor: "bg-red-100 dark:bg-red-900/30",
    description: "Block specific MCP servers",
  },
  {
    type: "mcp_capabilities",
    label: "MCP Capabilities",
    icon: Shield,
    color: "text-blue-600",
    bgColor: "bg-blue-100 dark:bg-blue-900/30",
    description: "Require or forbid specific capabilities",
  },
  {
    type: "mcp_unverified",
    label: "Unverified MCPs",
    icon: FileWarning,
    color: "text-yellow-600",
    bgColor: "bg-yellow-100 dark:bg-yellow-900/30",
    description: "Handle unverified MCP servers",
  },
];

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

const agentPolicyTypeLabels: Record<string, string> = {
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
  const [role, setRole] = useState<"admin" | "manager" | "member" | "viewer">("viewer");
  const [policies, setPolicies] = useState<SecurityPolicy[]>([]);
  const [loading, setLoading] = useState(true);

  // Main filter state
  const [mainFilter, setMainFilter] = useState<"all" | "agent" | "mcp">("all");
  const [mcpSubFilter, setMcpSubFilter] = useState<string>("all");

  // Dialog states
  const [showCreateDialog, setShowCreateDialog] = useState(false);
  const [showEditDialog, setShowEditDialog] = useState(false);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
  const [showBlockingWarning, setShowBlockingWarning] = useState(false);
  const [editingPolicy, setEditingPolicy] = useState<SecurityPolicy | null>(null);
  const [savingPolicy, setSavingPolicy] = useState(false);

  // Pending actions for blocking mode warning
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

  // Global enforcement mode state
  const [globalEnforcementMode, setGlobalEnforcementMode] = useState<"strict" | "monitoring">("monitoring");
  const [enforcementLoading, setEnforcementLoading] = useState(true);
  const [enforcementUpdating, setEnforcementUpdating] = useState(false);
  const [showGlobalEnforcementWarning, setShowGlobalEnforcementWarning] = useState(false);

  // MCP Form state
  const [formName, setFormName] = useState("");
  const [formDescription, setFormDescription] = useState("");
  const [formPolicyType, setFormPolicyType] = useState("mcp_allowlist");
  const [formEnforcement, setFormEnforcement] = useState<"alert_only" | "block_and_alert" | "allow">("alert_only");
  const [formPriority, setFormPriority] = useState(50);

  // MCP Allowlist rules
  const [allowedDomains, setAllowedDomains] = useState("");
  const [allowedNames, setAllowedNames] = useState("");
  const [allowedCapabilities, setAllowedCapabilities] = useState("");
  const [requireVerified, setRequireVerified] = useState(false);
  const [minTrustScore, setMinTrustScore] = useState(0);
  const [minConfidenceScore, setMinConfidenceScore] = useState(0);
  const [minAttestations, setMinAttestations] = useState(0);

  // MCP Blocklist rules
  const [blockedDomains, setBlockedDomains] = useState("");
  const [blockedNames, setBlockedNames] = useState("");
  const [blockedUrls, setBlockedUrls] = useState("");
  const [blockedCapabilities, setBlockedCapabilities] = useState("");

  // MCP Capabilities rules
  const [requiredCapabilities, setRequiredCapabilities] = useState("");
  const [forbiddenCapabilities, setForbiddenCapabilities] = useState("");
  const [requireAllCapabilities, setRequireAllCapabilities] = useState(false);
  const [maxCapabilityCount, setMaxCapabilityCount] = useState(0);

  // MCP Unverified rules
  const [unverifiedAction, setUnverifiedAction] = useState("alert_only");
  const [gracePeriodDays, setGracePeriodDays] = useState(7);
  const [requireManualApproval, setRequireManualApproval] = useState(false);
  const [autoQuarantineAfterDays, setAutoQuarantineAfterDays] = useState(30);
  const [alertOnFirstUse, setAlertOnFirstUse] = useState(true);
  const [notifyOwners, setNotifyOwners] = useState(true);

  // Admin-only guard
  useEffect(() => {
    try {
      const token = api.getToken?.();
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
    fetchPolicies();
    fetchEnforcementSettings();
  }, []);

  const fetchPolicies = async () => {
    try {
      const data = await api.getSecurityPolicies();
      // Sort by priority (highest first)
      const sorted = data.sort((a: SecurityPolicy, b: SecurityPolicy) => b.priority - a.priority);
      setPolicies(sorted);
    } catch (error) {
      console.error("Failed to fetch security policies:", error);
    } finally {
      setLoading(false);
    }
  };

  const fetchEnforcementSettings = async () => {
    try {
      setEnforcementLoading(true);
      const settings = await api.getEnforcementSettings();
      setGlobalEnforcementMode(settings.enforcementMode);
    } catch (error) {
      console.error("Failed to fetch enforcement settings:", error);
    } finally {
      setEnforcementLoading(false);
    }
  };

  const handleGlobalEnforcementToggle = () => {
    if (globalEnforcementMode === "monitoring") {
      // Switching to strict mode - show warning
      setShowGlobalEnforcementWarning(true);
    } else {
      // Switching to monitoring mode - no warning needed
      updateGlobalEnforcement("monitoring");
    }
  };

  const updateGlobalEnforcement = async (mode: "strict" | "monitoring") => {
    try {
      setEnforcementUpdating(true);
      await api.updateEnforcementSettings(mode);
      setGlobalEnforcementMode(mode);
      setShowGlobalEnforcementWarning(false);
    } catch (error) {
      console.error("Failed to update enforcement settings:", error);
    } finally {
      setEnforcementUpdating(false);
    }
  };

  // Helper functions
  const isAgentPolicy = (policy: SecurityPolicy) => AGENT_POLICY_TYPES.includes(policy.policyType);
  const isMCPPolicy = (policy: SecurityPolicy) => MCP_POLICY_TYPES.includes(policy.policyType);

  const getFilteredPolicies = () => {
    let filtered = policies;

    if (mainFilter === "agent") {
      filtered = policies.filter(isAgentPolicy);
    } else if (mainFilter === "mcp") {
      filtered = policies.filter(isMCPPolicy);
      if (mcpSubFilter !== "all") {
        filtered = filtered.filter(p => p.policyType === mcpSubFilter);
      }
    }

    return filtered;
  };

  const getMCPPolicyTypeConfig = (policyType: string) => {
    return MCP_POLICY_TYPE_CONFIGS.find(t => t.type === policyType) || MCP_POLICY_TYPE_CONFIGS[0];
  };

  // Stats
  const agentPolicies = policies.filter(isAgentPolicy);
  const mcpPolicies = policies.filter(isMCPPolicy);
  const enabledCount = policies.filter(p => p.isEnabled).length;
  const blockingCount = policies.filter(p => p.isEnabled && p.enforcementAction === "block_and_alert").length;
  const monitoringCount = policies.filter(p => p.isEnabled && p.enforcementAction === "alert_only").length;

  // Policy toggle with blocking warning
  const togglePolicy = async (policyId: string, currentlyEnabled: boolean) => {
    try {
      await api.toggleSecurityPolicy(policyId, !currentlyEnabled);
      setPolicies(policies.map(p =>
        p.id === policyId ? { ...p, isEnabled: !currentlyEnabled } : p
      ));
    } catch (error) {
      console.error("Failed to toggle policy:", error);
      alert("Failed to toggle policy. Please try again.");
    }
  };

  const handlePolicyToggle = (policy: SecurityPolicy, newEnabled: boolean) => {
    if (newEnabled && policy.enforcementAction === "block_and_alert") {
      setPendingPolicyToggle({
        policyId: policy.id,
        policyName: policy.name,
        currentAction: policy.enforcementAction,
      });
      setShowBlockingWarning(true);
    } else {
      togglePolicy(policy.id, policy.isEnabled);
    }
  };

  const changeEnforcementAction = async (
    policyId: string,
    newAction: "alert_only" | "block_and_alert" | "allow"
  ) => {
    try {
      const policy = policies.find(p => p.id === policyId);
      if (!policy) return;

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

      setPolicies(policies.map(p =>
        p.id === policyId ? { ...p, enforcementAction: newAction } : p
      ));
    } catch (error) {
      console.error("Failed to update enforcement action:", error);
      alert("Failed to update enforcement action. Please try again.");
    }
  };

  const handleEnforcementChange = (
    policy: SecurityPolicy,
    newAction: "alert_only" | "block_and_alert" | "allow"
  ) => {
    if (newAction === "block_and_alert") {
      setPendingEnforcementChange({
        policyId: policy.id,
        policyName: policy.name,
        newAction,
      });
      setShowBlockingWarning(true);
    } else {
      changeEnforcementAction(policy.id, newAction);
    }
  };

  const confirmBlockingMode = async () => {
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

  const cancelBlockingMode = () => {
    setShowBlockingWarning(false);
    setPendingPolicyToggle(null);
    setPendingEnforcementChange(null);
  };

  // MCP Policy CRUD
  const resetForm = () => {
    setFormName("");
    setFormDescription("");
    setFormPolicyType("mcp_allowlist");
    setFormEnforcement("alert_only");
    setFormPriority(50);
    // Allowlist
    setAllowedDomains("");
    setAllowedNames("");
    setAllowedCapabilities("");
    setRequireVerified(false);
    setMinTrustScore(0);
    setMinConfidenceScore(0);
    setMinAttestations(0);
    // Blocklist
    setBlockedDomains("");
    setBlockedNames("");
    setBlockedUrls("");
    setBlockedCapabilities("");
    // Capabilities
    setRequiredCapabilities("");
    setForbiddenCapabilities("");
    setRequireAllCapabilities(false);
    setMaxCapabilityCount(0);
    // Unverified
    setUnverifiedAction("alert_only");
    setGracePeriodDays(7);
    setRequireManualApproval(false);
    setAutoQuarantineAfterDays(30);
    setAlertOnFirstUse(true);
    setNotifyOwners(true);
  };

  const loadPolicyIntoForm = (policy: SecurityPolicy) => {
    setFormName(policy.name);
    setFormDescription(policy.description);
    setFormPolicyType(policy.policyType);
    setFormEnforcement(policy.enforcementAction);
    setFormPriority(policy.priority);

    const rules = policy.rules || {};

    if (policy.policyType === "mcp_allowlist") {
      setAllowedDomains((rules.allowedDomains || []).join(", "));
      setAllowedNames((rules.allowedNames || []).join(", "));
      setAllowedCapabilities((rules.allowedCapabilities || []).join(", "));
      setRequireVerified(rules.requireVerified || false);
      setMinTrustScore(rules.minTrustScore || 0);
      setMinConfidenceScore(rules.minConfidenceScore || 0);
      setMinAttestations(rules.minAttestations || 0);
    } else if (policy.policyType === "mcp_blocklist") {
      setBlockedDomains((rules.blockedDomains || []).join(", "));
      setBlockedNames((rules.blockedNames || []).join(", "));
      setBlockedUrls((rules.blockedUrls || []).join(", "));
      setBlockedCapabilities((rules.blockedCapabilities || []).join(", "));
    } else if (policy.policyType === "mcp_capabilities") {
      setRequiredCapabilities((rules.requiredCapabilities || []).join(", "));
      setForbiddenCapabilities((rules.forbiddenCapabilities || []).join(", "));
      setRequireAllCapabilities(rules.requireAllCapabilities || false);
      setMaxCapabilityCount(rules.maxCapabilityCount || 0);
    } else if (policy.policyType === "mcp_unverified") {
      setUnverifiedAction(rules.action || "alert_only");
      setGracePeriodDays(rules.gracePeriodDays || 7);
      setRequireManualApproval(rules.requireManualApproval || false);
      setAutoQuarantineAfterDays(rules.autoQuarantineAfterDays || 30);
      setAlertOnFirstUse(rules.alertOnFirstUse !== false);
      setNotifyOwners(rules.notifyOwners !== false);
    }
  };

  const buildRulesFromForm = () => {
    if (formPolicyType === "mcp_allowlist") {
      return {
        allowedDomains: allowedDomains.split(",").map(s => s.trim()).filter(Boolean),
        allowedNames: allowedNames.split(",").map(s => s.trim()).filter(Boolean),
        allowedCapabilities: allowedCapabilities.split(",").map(s => s.trim()).filter(Boolean),
        requireVerified,
        minTrustScore,
        minConfidenceScore,
        minAttestations,
      };
    } else if (formPolicyType === "mcp_blocklist") {
      return {
        blockedDomains: blockedDomains.split(",").map(s => s.trim()).filter(Boolean),
        blockedNames: blockedNames.split(",").map(s => s.trim()).filter(Boolean),
        blockedUrls: blockedUrls.split(",").map(s => s.trim()).filter(Boolean),
        blockedCapabilities: blockedCapabilities.split(",").map(s => s.trim()).filter(Boolean),
      };
    } else if (formPolicyType === "mcp_capabilities") {
      return {
        requiredCapabilities: requiredCapabilities.split(",").map(s => s.trim()).filter(Boolean),
        forbiddenCapabilities: forbiddenCapabilities.split(",").map(s => s.trim()).filter(Boolean),
        requireAllCapabilities,
        maxCapabilityCount,
      };
    } else if (formPolicyType === "mcp_unverified") {
      return {
        action: unverifiedAction,
        gracePeriodDays,
        requireManualApproval,
        autoQuarantineAfterDays,
        alertOnFirstUse,
        notifyOwners,
      };
    }
    return {};
  };

  const handleCreatePolicy = async () => {
    setSavingPolicy(true);
    try {
      const rules = buildRulesFromForm();
      await api.createSecurityPolicy({
        name: formName,
        description: formDescription,
        policyType: formPolicyType,
        enforcementAction: formEnforcement,
        severityThreshold: "medium",
        rules,
        appliesTo: "all",
        isEnabled: true,
        priority: formPriority,
      });
      setShowCreateDialog(false);
      resetForm();
      fetchPolicies();
    } catch (error: any) {
      console.error("Failed to create policy:", error);
      alert(error.message || "Failed to create policy");
    } finally {
      setSavingPolicy(false);
    }
  };

  const handleUpdatePolicy = async () => {
    if (!editingPolicy) return;
    setSavingPolicy(true);
    try {
      const rules = buildRulesFromForm();
      await api.updateSecurityPolicy(editingPolicy.id, {
        name: formName,
        description: formDescription,
        policyType: formPolicyType,
        enforcementAction: formEnforcement,
        severityThreshold: "medium",
        rules,
        appliesTo: "all",
        isEnabled: editingPolicy.isEnabled,
        priority: formPriority,
      });
      setShowEditDialog(false);
      setEditingPolicy(null);
      resetForm();
      fetchPolicies();
    } catch (error: any) {
      console.error("Failed to update policy:", error);
      alert(error.message || "Failed to update policy");
    } finally {
      setSavingPolicy(false);
    }
  };

  const handleDeletePolicy = async () => {
    if (!editingPolicy) return;
    try {
      await api.deleteSecurityPolicy(editingPolicy.id);
      setShowDeleteConfirm(false);
      setEditingPolicy(null);
      fetchPolicies();
    } catch (error: any) {
      console.error("Failed to delete policy:", error);
      alert(error.message || "Failed to delete policy");
    }
  };

  // Render MCP rules form
  const renderMCPRulesForm = () => {
    if (formPolicyType === "mcp_allowlist") {
      return (
        <div className="space-y-4">
          <div>
            <Label>Allowed Domains (comma-separated)</Label>
            <Input
              value={allowedDomains}
              onChange={(e) => setAllowedDomains(e.target.value)}
              placeholder="*.company.com, github.com, anthropic.com"
            />
            <p className="text-xs text-muted-foreground mt-1">
              Use wildcards like *.company.com for subdomains
            </p>
          </div>
          <div>
            <Label>Allowed Names (comma-separated)</Label>
            <Input
              value={allowedNames}
              onChange={(e) => setAllowedNames(e.target.value)}
              placeholder="filesystem-mcp, github-mcp, slack-mcp"
            />
          </div>
          <div>
            <Label>Allowed Capabilities (comma-separated)</Label>
            <Input
              value={allowedCapabilities}
              onChange={(e) => setAllowedCapabilities(e.target.value)}
              placeholder="tools, resources, prompts"
            />
          </div>
          <div className="flex items-center space-x-2">
            <Switch checked={requireVerified} onCheckedChange={setRequireVerified} />
            <Label>Require Verified Status</Label>
          </div>
          <div className="grid grid-cols-3 gap-4">
            <div>
              <Label>Min Trust Score (%)</Label>
              <Input
                type="number"
                min={0}
                max={100}
                value={minTrustScore}
                onChange={(e) => setMinTrustScore(Number(e.target.value))}
              />
            </div>
            <div>
              <Label>Min Confidence Score (%)</Label>
              <Input
                type="number"
                min={0}
                max={100}
                value={minConfidenceScore}
                onChange={(e) => setMinConfidenceScore(Number(e.target.value))}
              />
            </div>
            <div>
              <Label>Min Attestations</Label>
              <Input
                type="number"
                min={0}
                value={minAttestations}
                onChange={(e) => setMinAttestations(Number(e.target.value))}
              />
            </div>
          </div>
        </div>
      );
    }

    if (formPolicyType === "mcp_blocklist") {
      return (
        <div className="space-y-4">
          <div>
            <Label>Blocked Domains (comma-separated)</Label>
            <Input
              value={blockedDomains}
              onChange={(e) => setBlockedDomains(e.target.value)}
              placeholder="malicious.com, *.sketchy-domain.net"
            />
          </div>
          <div>
            <Label>Blocked Names (comma-separated)</Label>
            <Input
              value={blockedNames}
              onChange={(e) => setBlockedNames(e.target.value)}
              placeholder="compromised-mcp, deprecated-server"
            />
          </div>
          <div>
            <Label>Blocked URLs (comma-separated)</Label>
            <Textarea
              value={blockedUrls}
              onChange={(e) => setBlockedUrls(e.target.value)}
              placeholder="https://malicious.com/mcp, http://insecure-server.com/api"
              rows={2}
            />
          </div>
          <div>
            <Label>Blocked Capabilities (comma-separated)</Label>
            <Input
              value={blockedCapabilities}
              onChange={(e) => setBlockedCapabilities(e.target.value)}
              placeholder="dangerous-tool, unrestricted-access"
            />
          </div>
        </div>
      );
    }

    if (formPolicyType === "mcp_capabilities") {
      return (
        <div className="space-y-4">
          <div>
            <Label>Required Capabilities (comma-separated)</Label>
            <Input
              value={requiredCapabilities}
              onChange={(e) => setRequiredCapabilities(e.target.value)}
              placeholder="tools, resources"
            />
          </div>
          <div>
            <Label>Forbidden Capabilities (comma-separated)</Label>
            <Input
              value={forbiddenCapabilities}
              onChange={(e) => setForbiddenCapabilities(e.target.value)}
              placeholder="admin, system-access, file-write"
            />
          </div>
          <div className="flex items-center space-x-2">
            <Switch checked={requireAllCapabilities} onCheckedChange={setRequireAllCapabilities} />
            <Label>Require ALL capabilities (AND logic)</Label>
          </div>
          <div>
            <Label>Max Capability Count (0 = no limit)</Label>
            <Input
              type="number"
              min={0}
              value={maxCapabilityCount}
              onChange={(e) => setMaxCapabilityCount(Number(e.target.value))}
            />
          </div>
        </div>
      );
    }

    if (formPolicyType === "mcp_unverified") {
      return (
        <div className="space-y-4">
          <div>
            <Label>Action for Unverified MCPs</Label>
            <Select value={unverifiedAction} onValueChange={setUnverifiedAction}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="alert_only">Alert Only</SelectItem>
                <SelectItem value="block_and_alert">Block & Alert</SelectItem>
                <SelectItem value="quarantine">Quarantine</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div>
            <Label>Grace Period (days)</Label>
            <Input
              type="number"
              min={0}
              value={gracePeriodDays}
              onChange={(e) => setGracePeriodDays(Number(e.target.value))}
            />
            <p className="text-xs text-muted-foreground mt-1">
              Newly registered MCPs get this many days before enforcement
            </p>
          </div>
          <div className="flex items-center space-x-2">
            <Switch checked={requireManualApproval} onCheckedChange={setRequireManualApproval} />
            <Label>Require Manual Approval</Label>
          </div>
          <div>
            <Label>Auto-Quarantine After (days, 0 = never)</Label>
            <Input
              type="number"
              min={0}
              value={autoQuarantineAfterDays}
              onChange={(e) => setAutoQuarantineAfterDays(Number(e.target.value))}
            />
          </div>
          <div className="flex items-center space-x-2">
            <Switch checked={alertOnFirstUse} onCheckedChange={setAlertOnFirstUse} />
            <Label>Alert on First Use</Label>
          </div>
          <div className="flex items-center space-x-2">
            <Switch checked={notifyOwners} onCheckedChange={setNotifyOwners} />
            <Label>Notify MCP Owners</Label>
          </div>
        </div>
      );
    }

    return null;
  };

  // Render Agent Policy Card
  const renderAgentPolicyCard = (policy: SecurityPolicy) => {
    const EnforcementIcon = enforcementIcons[policy.enforcementAction] || Shield;
    const isBlocking = policy.enforcementAction === "block_and_alert";

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
                    ? "bg-blue-100 dark:bg-blue-900/30"
                    : "bg-gray-100 dark:bg-gray-800"
                }`}
              >
                <Bot
                  className={`h-5 w-5 ${
                    policy.isEnabled
                      ? "text-blue-600 dark:text-blue-400"
                      : "text-gray-400"
                  }`}
                />
              </div>
              <div className="flex-1">
                <div className="flex items-center gap-2 flex-wrap">
                  <h3 className="font-semibold text-lg">{policy.name}</h3>
                  <Badge
                    className={`${enforcementColors[policy.enforcementAction] || "bg-gray-100 text-gray-800 border-gray-300"} border`}
                  >
                    <EnforcementIcon className="h-3 w-3 mr-1" />
                    {enforcementLabels[policy.enforcementAction] || policy.enforcementAction}
                  </Badge>
                  <Badge variant="outline" className="text-xs">
                    Priority: {policy.priority}
                  </Badge>
                  <Badge variant="outline" className="text-xs bg-blue-50">
                    {agentPolicyTypeLabels[policy.policyType] || policy.policyType}
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
                <span className="capitalize">{policy.appliesTo.replace("_", " ")}</span>
              </div>
              <div>
                <span className="font-medium">Severity threshold:</span>{" "}
                <span className="capitalize">{policy.severityThreshold}</span>
              </div>
              {policy.rules && Object.keys(policy.rules).length > 0 && (
                <div>
                  <span className="font-medium">Rules:</span>{" "}
                  <span>{Object.keys(policy.rules).length} configured</span>
                </div>
              )}
            </div>

            {isBlocking && policy.isEnabled && (
              <div className="flex items-center gap-2 pl-14 p-3 bg-red-50 dark:bg-red-950/20 rounded-md border border-red-200 dark:border-red-800">
                <AlertOctagon className="h-4 w-4 text-red-600" />
                <p className="text-xs text-red-800 dark:text-red-200 font-medium">
                  BLOCKING MODE ACTIVE: This policy will prevent agent actions in real-time
                </p>
              </div>
            )}
          </div>

          <div className="flex flex-col gap-4 ml-6">
            {/* Enforcement Action Selector */}
            <div className="min-w-[180px]">
              <p className="text-xs text-muted-foreground mb-2">Enforcement Mode</p>
              <Select
                value={policy.enforcementAction}
                onValueChange={(value: "alert_only" | "block_and_alert" | "allow") =>
                  handleEnforcementChange(policy, value)
                }
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
                <p className="text-xs text-muted-foreground">Status</p>
                <p
                  className={`text-sm font-medium ${
                    policy.isEnabled ? "text-green-600" : "text-gray-500"
                  }`}
                >
                  {policy.isEnabled ? "Enabled" : "Disabled"}
                </p>
              </div>
              <Switch
                checked={policy.isEnabled}
                onCheckedChange={(checked) => handlePolicyToggle(policy, checked)}
                className="data-[state=checked]:bg-green-600"
              />
            </div>
          </div>
        </div>
      </div>
    );
  };

  // Render MCP Policy Card - consistent with Agent Policy Card design
  const renderMCPPolicyCard = (policy: SecurityPolicy) => {
    const typeConfig = getMCPPolicyTypeConfig(policy.policyType);
    const TypeIcon = typeConfig.icon;
    const EnforcementIcon = enforcementIcons[policy.enforcementAction] || Shield;
    const isBlocking = policy.enforcementAction === "block_and_alert";

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
              <div className={`p-2 rounded-lg ${typeConfig.bgColor}`}>
                <TypeIcon className={`h-5 w-5 ${typeConfig.color}`} />
              </div>
              <div className="flex-1">
                <div className="flex items-center gap-2 flex-wrap">
                  <h3 className="font-semibold text-lg">{policy.name}</h3>
                  <Badge
                    className={`${enforcementColors[policy.enforcementAction] || "bg-gray-100 text-gray-800 border-gray-300"} border`}
                  >
                    <EnforcementIcon className="h-3 w-3 mr-1" />
                    {enforcementLabels[policy.enforcementAction] || policy.enforcementAction}
                  </Badge>
                  <Badge variant="outline" className="text-xs">
                    Priority: {policy.priority}
                  </Badge>
                  <Badge variant="outline" className="text-xs bg-purple-50">
                    {typeConfig.label}
                  </Badge>
                </div>
                <p className="text-sm text-muted-foreground mt-1">
                  {policy.description}
                </p>
              </div>
            </div>

            <div className="flex flex-wrap gap-4 text-xs text-muted-foreground pl-14">
              {policy.policyType === "mcp_allowlist" && (
                <>
                  {policy.rules?.allowedDomains?.length > 0 && (
                    <div>
                      <span className="font-medium">Domains:</span>{" "}
                      <span>{policy.rules.allowedDomains.slice(0, 3).join(", ")}{policy.rules.allowedDomains.length > 3 ? "..." : ""}</span>
                    </div>
                  )}
                  {policy.rules?.requireVerified && (
                    <div className="text-green-600 font-medium">Requires verification</div>
                  )}
                </>
              )}
              {policy.policyType === "mcp_blocklist" && policy.rules?.blockedDomains?.length > 0 && (
                <div>
                  <span className="font-medium">Blocked:</span>{" "}
                  <span>{policy.rules.blockedDomains.slice(0, 3).join(", ")}{policy.rules.blockedDomains.length > 3 ? "..." : ""}</span>
                </div>
              )}
              {policy.policyType === "mcp_capabilities" && (
                <>
                  {policy.rules?.requiredCapabilities?.length > 0 && (
                    <div>
                      <span className="font-medium">Required:</span>{" "}
                      <span>{policy.rules.requiredCapabilities.join(", ")}</span>
                    </div>
                  )}
                  {policy.rules?.forbiddenCapabilities?.length > 0 && (
                    <div className="text-red-600">
                      <span className="font-medium">Forbidden:</span>{" "}
                      <span>{policy.rules.forbiddenCapabilities.join(", ")}</span>
                    </div>
                  )}
                </>
              )}
              {policy.policyType === "mcp_unverified" && (
                <div>
                  <span className="font-medium">Action:</span>{" "}
                  <span>{policy.rules?.action || "alert_only"}, Grace: {policy.rules?.gracePeriodDays || 7} days</span>
                </div>
              )}
            </div>

            {isBlocking && policy.isEnabled && (
              <div className="flex items-center gap-2 pl-14 p-3 bg-red-50 dark:bg-red-950/20 rounded-md border border-red-200 dark:border-red-800">
                <AlertOctagon className="h-4 w-4 text-red-600" />
                <p className="text-xs text-red-800 dark:text-red-200 font-medium">
                  BLOCKING MODE ACTIVE: This policy will block MCP server connections in real-time
                </p>
              </div>
            )}
          </div>

          <div className="flex flex-col gap-4 ml-6">
            {/* Enforcement Action Selector */}
            <div className="min-w-[180px]">
              <p className="text-xs text-muted-foreground mb-2">Enforcement Mode</p>
              <Select
                value={policy.enforcementAction}
                onValueChange={(value: "alert_only" | "block_and_alert" | "allow") =>
                  handleEnforcementChange(policy, value)
                }
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
                <p className="text-xs text-muted-foreground">Status</p>
                <p
                  className={`text-sm font-medium ${
                    policy.isEnabled ? "text-green-600" : "text-gray-500"
                  }`}
                >
                  {policy.isEnabled ? "Enabled" : "Disabled"}
                </p>
              </div>
              <Switch
                checked={policy.isEnabled}
                onCheckedChange={(checked) => handlePolicyToggle(policy, checked)}
                className="data-[state=checked]:bg-green-600"
              />
            </div>
          </div>
        </div>
      </div>
    );
  };

  if (loading) {
    return (
      <div className="space-y-6">
        <div className="space-y-2">
          <Skeleton className="h-9 w-48" />
          <Skeleton className="h-4 w-96" />
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
        <div className="grid gap-4 md:grid-cols-2">
          {[...Array(4)].map((_, i) => (
            <Skeleton key={i} className="h-40 w-full" />
          ))}
        </div>
      </div>
    );
  }

  return (
    <AuthGuard>
      <div className="space-y-6">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold flex items-center gap-3">
              <Shield className="h-8 w-8 text-blue-600" />
              Security Policies
            </h1>
            <p className="text-muted-foreground mt-1">
              Manage security enforcement for agents and MCP servers
            </p>
          </div>
          {mainFilter === "mcp" && (
            <Button onClick={() => { resetForm(); setShowCreateDialog(true); }}>
              <Plus className="h-4 w-4 mr-2" />
              Create MCP Policy
            </Button>
          )}
        </div>

        {/* Stats */}
        <div className="grid gap-4 md:grid-cols-5">
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium">Total Policies</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{policies.length}</div>
              <p className="text-xs text-muted-foreground">
                {agentPolicies.length} agent, {mcpPolicies.length} MCP
              </p>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium">Enabled</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold text-green-600">{enabledCount}</div>
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
              <div className="text-2xl font-bold text-yellow-600">{monitoringCount}</div>
              <p className="text-xs text-muted-foreground">Alert only mode</p>
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
              <div className="text-2xl font-bold text-red-600">{blockingCount}</div>
              <p className="text-xs text-muted-foreground">Block & alert mode</p>
            </CardContent>
          </Card>
          <Card className="bg-blue-50 dark:bg-blue-950/20 border-blue-200 dark:border-blue-800">
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium flex items-center gap-2">
                <Bot className="h-4 w-4 text-blue-600" />
                Agent Policies
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold text-blue-600">{agentPolicies.length}</div>
            </CardContent>
          </Card>
        </div>

        {/* Global Enforcement Mode Toggle */}
        <Card className={`border-2 ${globalEnforcementMode === 'strict' ? 'border-red-300 bg-gradient-to-r from-red-50 to-orange-50 dark:from-red-950/20 dark:to-orange-950/20' : 'border-blue-300 bg-gradient-to-r from-blue-50 to-cyan-50 dark:from-blue-950/20 dark:to-cyan-950/20'}`}>
          <CardContent className="pt-6">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-4">
                {globalEnforcementMode === 'strict' ? (
                  <div className="p-3 bg-red-100 dark:bg-red-900/30 rounded-xl">
                    <Lock className="h-6 w-6 text-red-600" />
                  </div>
                ) : (
                  <div className="p-3 bg-blue-100 dark:bg-blue-900/30 rounded-xl">
                    <Eye className="h-6 w-6 text-blue-600" />
                  </div>
                )}
                <div>
                  <div className="flex items-center gap-2">
                    <h3 className="text-lg font-semibold">Global Enforcement Mode</h3>
                    <Badge variant={globalEnforcementMode === 'strict' ? 'destructive' : 'secondary'}>
                      {globalEnforcementMode === 'strict' ? 'STRICT' : 'MONITORING'}
                    </Badge>
                  </div>
                  <p className="text-sm text-muted-foreground mt-1">
                    {globalEnforcementMode === 'strict'
                      ? 'SDK will block actions that fail verification. Unauthorized actions are prevented in real-time.'
                      : 'SDK allows all actions but logs violations. Use this to test policies before enforcing.'}
                  </p>
                </div>
              </div>
              <div className="flex items-center gap-3">
                {enforcementLoading ? (
                  <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
                ) : enforcementUpdating ? (
                  <div className="flex items-center gap-2">
                    <Loader2 className="h-4 w-4 animate-spin" />
                    <span className="text-sm text-muted-foreground">Updating...</span>
                  </div>
                ) : (
                  <>
                    <span className={`text-sm font-medium ${globalEnforcementMode === 'strict' ? 'text-red-600' : 'text-muted-foreground'}`}>
                      {globalEnforcementMode === 'strict' ? 'Strict' : 'Monitoring'}
                    </span>
                    <Switch
                      checked={globalEnforcementMode === 'strict'}
                      onCheckedChange={handleGlobalEnforcementToggle}
                      disabled={enforcementUpdating}
                    />
                  </>
                )}
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Info Banner */}
        <Card className="bg-gradient-to-r from-blue-50 to-purple-50 dark:from-blue-950/20 dark:to-purple-950/20 border-blue-200 dark:border-blue-800">
          <CardContent className="pt-6">
            <div className="flex gap-3">
              <Info className="h-5 w-5 text-blue-600 flex-shrink-0 mt-0.5" />
              <div className="space-y-2 text-sm">
                <p className="font-medium text-blue-900 dark:text-blue-100">
                  Unified Security Policy Management
                </p>
                <p className="text-blue-800 dark:text-blue-200">
                  Manage both agent and MCP server security policies in one place. Agent policies protect against capability violations,
                  trust score issues, and unusual activity. MCP policies control server allowlists, blocklists, and verification requirements.
                </p>
                <ul className="space-y-1 text-blue-800 dark:text-blue-200 mt-2">
                  <li className="flex items-center gap-2">
                    <Eye className="h-4 w-4" />
                    <strong>Alert Only:</strong> Log violations but allow actions (monitoring mode)
                  </li>
                  <li className="flex items-center gap-2">
                    <Lock className="h-4 w-4" />
                    <strong>Block & Alert:</strong> Prevent violations and create alerts (enforcement mode)
                  </li>
                </ul>
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Main Tabs */}
        <Tabs value={mainFilter} onValueChange={(v) => setMainFilter(v as any)} className="space-y-4">
          <TabsList className="grid w-full grid-cols-3 max-w-md">
            <TabsTrigger value="all" className="flex items-center gap-2">
              <Filter className="h-4 w-4" />
              All ({policies.length})
            </TabsTrigger>
            <TabsTrigger value="agent" className="flex items-center gap-2">
              <Bot className="h-4 w-4" />
              Agent ({agentPolicies.length})
            </TabsTrigger>
            <TabsTrigger value="mcp" className="flex items-center gap-2">
              <Server className="h-4 w-4" />
              MCP ({mcpPolicies.length})
            </TabsTrigger>
          </TabsList>

          {/* All Policies Tab */}
          <TabsContent value="all" className="space-y-6">
            {/* Agent Policies Section */}
            {agentPolicies.length > 0 && (
              <div className="space-y-4">
                <h2 className="text-xl font-semibold flex items-center gap-2">
                  <Bot className="h-5 w-5 text-blue-600" />
                  Agent Policies
                </h2>
                <div className="space-y-4">
                  {agentPolicies.map(renderAgentPolicyCard)}
                </div>
              </div>
            )}

            {/* MCP Policies Section */}
            {mcpPolicies.length > 0 && (
              <div className="space-y-4">
                <div className="flex items-center justify-between">
                  <h2 className="text-xl font-semibold flex items-center gap-2">
                    <Server className="h-5 w-5 text-purple-600" />
                    MCP Policies
                  </h2>
                  <Button onClick={() => { resetForm(); setShowCreateDialog(true); }} size="sm">
                    <Plus className="h-4 w-4 mr-2" />
                    Create MCP Policy
                  </Button>
                </div>
                <div className="space-y-4">
                  {mcpPolicies.map(renderMCPPolicyCard)}
                </div>
              </div>
            )}

            {policies.length === 0 && (
              <div className="text-center py-12 text-muted-foreground">
                <Shield className="h-12 w-12 mx-auto mb-4 opacity-50" />
                <p className="text-lg font-medium">No security policies configured</p>
                <p className="text-sm mt-1">Default policies are created automatically for new organizations.</p>
              </div>
            )}
          </TabsContent>

          {/* Agent Policies Tab */}
          <TabsContent value="agent" className="space-y-4">
            <Card>
              <CardHeader>
                <CardTitle>Agent Security Policies ({agentPolicies.length})</CardTitle>
                <CardDescription>
                  Policies that protect against capability violations, trust score issues, and unusual agent activity.
                  Toggle policies on/off or adjust enforcement actions.
                </CardDescription>
              </CardHeader>
              <CardContent>
                <div className="space-y-4">
                  {agentPolicies.length === 0 && (
                    <div className="text-center py-12 text-muted-foreground">
                      <Bot className="h-12 w-12 mx-auto mb-4 opacity-50" />
                      <p className="text-lg font-medium">No agent policies configured</p>
                      <p className="text-sm mt-1">Default policies are created automatically for new organizations.</p>
                    </div>
                  )}
                  {agentPolicies.map(renderAgentPolicyCard)}
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          {/* MCP Policies Tab */}
          <TabsContent value="mcp" className="space-y-4">
            <Card>
              <CardHeader>
                <CardTitle>MCP Server Policies ({mcpPolicies.length})</CardTitle>
                <CardDescription>
                  Policies that control MCP server allowlists, blocklists, and verification requirements.
                  Toggle policies on/off or adjust enforcement actions.
                </CardDescription>
              </CardHeader>
              <CardContent>
                {/* MCP Sub-tabs */}
                <Tabs value={mcpSubFilter} onValueChange={setMcpSubFilter} className="space-y-4">
                  <TabsList>
                    <TabsTrigger value="all">All MCP</TabsTrigger>
                    <TabsTrigger value="mcp_allowlist">Allowlists</TabsTrigger>
                    <TabsTrigger value="mcp_blocklist">Blocklists</TabsTrigger>
                    <TabsTrigger value="mcp_capabilities">Capabilities</TabsTrigger>
                    <TabsTrigger value="mcp_unverified">Unverified</TabsTrigger>
                  </TabsList>

                  {["all", "mcp_allowlist", "mcp_blocklist", "mcp_capabilities", "mcp_unverified"].map((tab) => (
                    <TabsContent key={tab} value={tab}>
                      <div className="space-y-4">
                        {mcpPolicies
                          .filter((p) => tab === "all" || p.policyType === tab)
                          .map(renderMCPPolicyCard)}

                        {mcpPolicies.filter((p) => tab === "all" || p.policyType === tab).length === 0 && (
                          <div className="text-center py-12">
                            <Server className="h-12 w-12 mx-auto text-muted-foreground mb-4" />
                            <p className="text-lg font-medium text-muted-foreground">No MCP policies found</p>
                            <p className="text-sm text-muted-foreground mt-1">
                              Create a new MCP policy to get started
                            </p>
                            <Button onClick={() => { resetForm(); setShowCreateDialog(true); }} className="mt-4">
                              <Plus className="h-4 w-4 mr-2" />
                              Create MCP Policy
                            </Button>
                          </div>
                        )}
                      </div>
                    </TabsContent>
                  ))}
                </Tabs>
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>

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
                  You are about to {pendingEnforcementChange ? "switch" : "enable"}{" "}
                  <strong>
                    "{pendingEnforcementChange?.policyName || pendingPolicyToggle?.policyName}"
                  </strong>{" "}
                  {pendingEnforcementChange ? "to" : "in"} blocking mode.
                </div>
                <div className="bg-red-50 dark:bg-red-950/20 border border-red-200 dark:border-red-800 p-4 rounded-md space-y-2">
                  <div className="text-sm font-semibold text-red-800 dark:text-red-200">
                    Warning: Production Impact
                  </div>
                  <ul className="text-sm text-red-700 dark:text-red-300 space-y-1 list-disc list-inside">
                    <li>This policy will <strong>block actions in real-time</strong></li>
                    <li>Blocked actions will return <strong>403 Forbidden</strong> responses</li>
                    <li>This may <strong>disrupt production workflows</strong></li>
                    <li>Consider testing in "Alert Only" mode first</li>
                  </ul>
                </div>
                <div className="text-sm">
                  Are you sure you want to {pendingEnforcementChange ? "switch to" : "enable"} blocking mode for this policy?
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
                onClick={confirmBlockingMode}
                className="bg-red-600 hover:bg-red-700"
              >
                <AlertOctagon className="h-4 w-4 mr-2" />
                {pendingEnforcementChange ? "Switch to Blocking Mode" : "Enable Blocking Mode"}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        {/* Global Enforcement Mode Warning Dialog */}
        <Dialog open={showGlobalEnforcementWarning} onOpenChange={setShowGlobalEnforcementWarning}>
          <DialogContent className="sm:max-w-md">
            <DialogHeader>
              <DialogTitle className="flex items-center gap-2 text-red-600">
                <AlertTriangle className="h-5 w-5" />
                Enable Global Strict Mode?
              </DialogTitle>
              <div className="text-sm text-muted-foreground space-y-3 pt-4">
                <div className="font-medium text-foreground">
                  You are about to enable <strong>Global Strict Enforcement Mode</strong> for all SDK operations.
                </div>
                <div className="bg-red-50 dark:bg-red-950/20 border border-red-200 dark:border-red-800 p-4 rounded-md space-y-2">
                  <div className="text-sm font-semibold text-red-800 dark:text-red-200">
                    Warning: Production Impact
                  </div>
                  <ul className="text-sm text-red-700 dark:text-red-300 space-y-1 list-disc list-inside">
                    <li>ALL agents using the SDK will have enforcement enabled</li>
                    <li>Actions that fail verification will be blocked in real-time</li>
                    <li>Agents without proper capabilities will be denied</li>
                    <li>This affects all environments using this AIM instance</li>
                  </ul>
                </div>
                <div className="text-sm">
                  <strong>Recommended:</strong> Test with individual policy blocking first before enabling global strict mode.
                </div>
              </div>
            </DialogHeader>
            <DialogFooter className="gap-2">
              <Button variant="outline" onClick={() => setShowGlobalEnforcementWarning(false)}>
                <X className="h-4 w-4 mr-2" />
                Cancel
              </Button>
              <Button
                variant="destructive"
                onClick={() => updateGlobalEnforcement("strict")}
                disabled={enforcementUpdating}
                className="bg-red-600 hover:bg-red-700"
              >
                {enforcementUpdating ? (
                  <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                ) : (
                  <Lock className="h-4 w-4 mr-2" />
                )}
                Enable Strict Mode
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        {/* Create MCP Policy Dialog */}
        <Dialog open={showCreateDialog} onOpenChange={setShowCreateDialog}>
          <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
            <DialogHeader>
              <DialogTitle>Create MCP Policy</DialogTitle>
              <DialogDescription>
                Define rules for MCP server access and compliance
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-6 py-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <Label>Policy Name</Label>
                  <Input
                    value={formName}
                    onChange={(e) => setFormName(e.target.value)}
                    placeholder="Production MCP Allowlist"
                  />
                </div>
                <div>
                  <Label>Priority</Label>
                  <Input
                    type="number"
                    min={1}
                    max={100}
                    value={formPriority}
                    onChange={(e) => setFormPriority(Number(e.target.value))}
                  />
                </div>
              </div>
              <div>
                <Label>Description</Label>
                <Textarea
                  value={formDescription}
                  onChange={(e) => setFormDescription(e.target.value)}
                  placeholder="Describe what this policy does..."
                  rows={2}
                />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <Label>Policy Type</Label>
                  <Select value={formPolicyType} onValueChange={setFormPolicyType}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {MCP_POLICY_TYPE_CONFIGS.map((type) => (
                        <SelectItem key={type.type} value={type.type}>
                          <div className="flex items-center gap-2">
                            <type.icon className={`h-4 w-4 ${type.color}`} />
                            {type.label}
                          </div>
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div>
                  <Label>Enforcement Action</Label>
                  <Select value={formEnforcement} onValueChange={(v: any) => setFormEnforcement(v)}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="alert_only">
                        <div className="flex items-center gap-2">
                          <Eye className="h-4 w-4 text-yellow-600" />
                          Alert Only
                        </div>
                      </SelectItem>
                      <SelectItem value="block_and_alert">
                        <div className="flex items-center gap-2">
                          <Lock className="h-4 w-4 text-red-600" />
                          Block & Alert
                        </div>
                      </SelectItem>
                      <SelectItem value="allow">
                        <div className="flex items-center gap-2">
                          <Check className="h-4 w-4 text-green-600" />
                          Allow
                        </div>
                      </SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </div>

              <div className="border-t pt-4">
                <h4 className="font-medium mb-4">Policy Rules</h4>
                {renderMCPRulesForm()}
              </div>
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => setShowCreateDialog(false)}>
                Cancel
              </Button>
              <Button onClick={handleCreatePolicy} disabled={savingPolicy || !formName}>
                {savingPolicy && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
                Create Policy
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        {/* Edit MCP Policy Dialog */}
        <Dialog open={showEditDialog} onOpenChange={setShowEditDialog}>
          <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
            <DialogHeader>
              <DialogTitle>Edit MCP Policy</DialogTitle>
              <DialogDescription>
                Update the policy rules and settings
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-6 py-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <Label>Policy Name</Label>
                  <Input
                    value={formName}
                    onChange={(e) => setFormName(e.target.value)}
                    placeholder="Production MCP Allowlist"
                  />
                </div>
                <div>
                  <Label>Priority</Label>
                  <Input
                    type="number"
                    min={1}
                    max={100}
                    value={formPriority}
                    onChange={(e) => setFormPriority(Number(e.target.value))}
                  />
                </div>
              </div>
              <div>
                <Label>Description</Label>
                <Textarea
                  value={formDescription}
                  onChange={(e) => setFormDescription(e.target.value)}
                  placeholder="Describe what this policy does..."
                  rows={2}
                />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <Label>Policy Type</Label>
                  <Select value={formPolicyType} onValueChange={setFormPolicyType}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {MCP_POLICY_TYPE_CONFIGS.map((type) => (
                        <SelectItem key={type.type} value={type.type}>
                          <div className="flex items-center gap-2">
                            <type.icon className={`h-4 w-4 ${type.color}`} />
                            {type.label}
                          </div>
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div>
                  <Label>Enforcement Action</Label>
                  <Select value={formEnforcement} onValueChange={(v: any) => setFormEnforcement(v)}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="alert_only">
                        <div className="flex items-center gap-2">
                          <Eye className="h-4 w-4 text-yellow-600" />
                          Alert Only
                        </div>
                      </SelectItem>
                      <SelectItem value="block_and_alert">
                        <div className="flex items-center gap-2">
                          <Lock className="h-4 w-4 text-red-600" />
                          Block & Alert
                        </div>
                      </SelectItem>
                      <SelectItem value="allow">
                        <div className="flex items-center gap-2">
                          <Check className="h-4 w-4 text-green-600" />
                          Allow
                        </div>
                      </SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </div>

              <div className="border-t pt-4">
                <h4 className="font-medium mb-4">Policy Rules</h4>
                {renderMCPRulesForm()}
              </div>
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => setShowEditDialog(false)}>
                Cancel
              </Button>
              <Button onClick={handleUpdatePolicy} disabled={savingPolicy || !formName}>
                {savingPolicy && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
                Update Policy
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        {/* Delete Confirmation Dialog */}
        <Dialog open={showDeleteConfirm} onOpenChange={setShowDeleteConfirm}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle className="flex items-center gap-2 text-red-600">
                <AlertTriangle className="h-5 w-5" />
                Delete Policy
              </DialogTitle>
              <DialogDescription>
                Are you sure you want to delete "{editingPolicy?.name}"? This action cannot be undone.
              </DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button variant="outline" onClick={() => setShowDeleteConfirm(false)}>
                Cancel
              </Button>
              <Button variant="destructive" onClick={handleDeletePolicy}>
                <Trash2 className="h-4 w-4 mr-2" />
                Delete Policy
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>
    </AuthGuard>
  );
}

"use client";

import { useState, useEffect } from "react";
import {
  X,
  AlertTriangle,
  ExternalLink,
  Shield,
  Activity,
  FileText,
  User,
  KeyRound,
  Loader2,
  Ban,
  Globe,
  Server,
  Target,
  Zap,
  CheckCircle2,
  Clock,
  Eye,
  Lock,
  RefreshCw,
  Settings,
  FileWarning,
  UserX,
  Database,
} from "lucide-react";
import Link from "next/link";
import { formatDateTime } from "@/lib/date-utils";
import { api, Agent } from "@/lib/api";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Button } from "@/components/ui/button";

interface SecurityThreat {
  id: string;
  targetId: string;
  targetName?: string;
  threatType: string;
  severity: "low" | "medium" | "high" | "critical";
  description: string;
  isBlocked: boolean;
  createdAt: string;
  source?: string;
  metadata?: Record<string, any>;
}

interface ThreatDetailModalProps {
  isOpen: boolean;
  onClose: () => void;
  threat: SecurityThreat | null;
}

interface RecommendationItem {
  priority: "critical" | "high" | "medium" | "low";
  action: string;
  description: string;
  link?: string;
  icon: React.ElementType;
}

// Intelligent recommendations based on threat type
const THREAT_RECOMMENDATIONS: Record<string, RecommendationItem[]> = {
  malicious_agent: [
    {
      priority: "critical",
      action: "Revoke SDK token immediately",
      description: "Prevent further unauthorized access by revoking the token used to create this agent",
      link: "/dashboard/sdk-tokens",
      icon: Ban,
    },
    {
      priority: "high",
      action: "Block agent & investigate",
      description: "Suspend the agent and review all recent activity for signs of compromise",
      link: "/dashboard/agents/{id}",
      icon: UserX,
    },
    {
      priority: "medium",
      action: "Review violation history",
      description: "Check the compliance logs for patterns of malicious behavior",
      link: "/dashboard/admin/compliance",
      icon: FileWarning,
    },
  ],
  unauthorized_access: [
    {
      priority: "high",
      action: "Review agent capabilities",
      description: "Compare the attempted action against the agent's registered capabilities",
      link: "/dashboard/agents/{id}",
      icon: Shield,
    },
    {
      priority: "medium",
      action: "Approve or deny capability",
      description: "If legitimate, grant the capability through the requests dashboard",
      link: "/dashboard/admin/capability-requests",
      icon: CheckCircle2,
    },
    {
      priority: "low",
      action: "Monitor for patterns",
      description: "Watch for recurring unauthorized access attempts from this agent",
      link: "/dashboard/security",
      icon: Eye,
    },
  ],
  configuration_drift: [
    {
      priority: "high",
      action: "Compare configurations",
      description: "Review registered vs detected MCP servers and capabilities",
      link: "/dashboard/agents/{id}",
      icon: RefreshCw,
    },
    {
      priority: "medium",
      action: "Update registration",
      description: "If drift is legitimate, update the agent's registered configuration",
      link: "/dashboard/agents/{id}",
      icon: Settings,
    },
    {
      priority: "low",
      action: "Review drift policy",
      description: "Adjust drift detection sensitivity in security policies",
      link: "/dashboard/security/policies",
      icon: FileText,
    },
  ],
  suspicious_activity: [
    {
      priority: "high",
      action: "Review recent activity",
      description: "Examine the agent's recent actions for anomalous patterns",
      link: "/dashboard/agents/{id}",
      icon: Activity,
    },
    {
      priority: "medium",
      action: "Check trust score trend",
      description: "Review how the agent's trust score has changed over time",
      link: "/dashboard/agents/{id}",
      icon: Activity,
    },
    {
      priority: "low",
      action: "Adjust monitoring threshold",
      description: "Fine-tune what qualifies as suspicious activity",
      link: "/dashboard/security/policies",
      icon: Settings,
    },
  ],
  credential_leak: [
    {
      priority: "critical",
      action: "Rotate all credentials",
      description: "Immediately rotate API keys and tokens associated with this agent",
      link: "/dashboard/sdk-tokens",
      icon: RefreshCw,
    },
    {
      priority: "high",
      action: "Revoke compromised tokens",
      description: "Revoke any tokens that may have been exposed",
      link: "/dashboard/sdk-tokens",
      icon: Ban,
    },
    {
      priority: "medium",
      action: "Audit access logs",
      description: "Review who accessed what with the compromised credentials",
      link: "/dashboard/admin/compliance",
      icon: Database,
    },
  ],
  certificate_expiry: [
    {
      priority: "high",
      action: "Renew certificate",
      description: "Generate and deploy a new certificate before expiration",
      link: "/dashboard/agents/{id}",
      icon: RefreshCw,
    },
    {
      priority: "medium",
      action: "Update agent config",
      description: "Ensure the agent is configured to use the new certificate",
      link: "/dashboard/agents/{id}",
      icon: Settings,
    },
    {
      priority: "low",
      action: "Set up auto-renewal",
      description: "Configure automatic certificate renewal to prevent future issues",
      link: "/dashboard/settings",
      icon: Clock,
    },
  ],
};

// Default recommendations for unknown threat types
const DEFAULT_RECOMMENDATIONS: RecommendationItem[] = [
  {
    priority: "high",
    action: "Investigate the threat",
    description: "Review agent activity logs and audit trail for suspicious patterns",
    link: "/dashboard/admin/compliance",
    icon: Eye,
  },
  {
    priority: "medium",
    action: "Verify agent configuration",
    description: "Ensure agent capabilities match its registered scope",
    link: "/dashboard/agents/{id}",
    icon: Shield,
  },
  {
    priority: "low",
    action: "Monitor trust score",
    description: "Watch for trust score changes that may indicate ongoing issues",
    link: "/dashboard/security",
    icon: Activity,
  },
];

export default function ThreatDetailModal({
  isOpen,
  onClose,
  threat,
}: ThreatDetailModalProps) {
  const [agent, setAgent] = useState<Agent | null>(null);
  const [loadingAgent, setLoadingAgent] = useState(false);
  const [revokingToken, setRevokingToken] = useState(false);
  const [showRevokeConfirm, setShowRevokeConfirm] = useState(false);
  const [revokeSuccess, setRevokeSuccess] = useState(false);

  useEffect(() => {
    if (!isOpen || !threat?.targetId) {
      setAgent(null);
      setRevokeSuccess(false);
      return;
    }

    const fetchAgent = async () => {
      setLoadingAgent(true);
      try {
        const agentData = await api.getAgent(threat.targetId);
        setAgent(agentData);
      } catch (error) {
        console.error("Failed to fetch agent details:", error);
        setAgent(null);
      } finally {
        setLoadingAgent(false);
      }
    };

    fetchAgent();
  }, [isOpen, threat?.targetId]);

  const handleRevokeSDKToken = async () => {
    if (!agent?.createdBySdkTokenId) return;

    setRevokingToken(true);
    try {
      await api.revokeSDKToken(agent.createdBySdkTokenId, `Security threat: ${threat?.threatType}`);
      setRevokeSuccess(true);
      setShowRevokeConfirm(false);
    } catch (error: any) {
      alert(error?.message || "Failed to revoke SDK token");
    } finally {
      setRevokingToken(false);
    }
  };

  if (!isOpen || !threat) return null;

  // Helper functions
  const getSeverityColor = (severity: string) => {
    switch (severity) {
      case "critical":
        return "bg-danger-fill text-danger-text";
      case "high":
        return "bg-warning-fill text-warning-text";
      case "medium":
        return "bg-warning-fill text-warning-text";
      case "low":
        return "bg-brand-soft text-brand-text";
      default:
        return "bg-glass-inset-gray text-ink-body";
    }
  };

  const getPriorityColor = (priority: string) => {
    switch (priority) {
      case "critical":
        return "bg-danger-fill border-danger-border";
      case "high":
        return "bg-warning-fill border-warning-border";
      case "medium":
        return "bg-warning-fill border-warning-border";
      case "low":
        return "bg-brand-soft border-stroke";
      default:
        return "bg-glass-inset-gray border-glass-inset-border";
    }
  };

  const getPriorityIconColor = (priority: string) => {
    switch (priority) {
      case "critical":
        return "text-danger-text";
      case "high":
        return "text-warning-text";
      case "medium":
        return "text-warning-text";
      case "low":
        return "text-brand-text";
      default:
        return "text-ink-body";
    }
  };

  const getStatusColor = (isBlocked: boolean) => {
    return isBlocked
      ? "bg-success-fill text-success-text"
      : "bg-danger-fill text-danger-text";
  };

  // Check if source is an IP address or UUID
  const isIPAddress = (str: string) => {
    const ipPattern = /^(\d{1,3}\.){3}\d{1,3}$/;
    return ipPattern.test(str);
  };

  const isUUID = (str: string) => {
    const uuidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
    return uuidPattern.test(str);
  };

  // Get recommendations for this threat type
  const getRecommendations = () => {
    const threatType = threat.threatType.toLowerCase().replace(/\s+/g, "_");
    return THREAT_RECOMMENDATIONS[threatType] || DEFAULT_RECOMMENDATIONS;
  };

  // Replace {id} placeholder in links
  const resolveLink = (link: string) => {
    return link.replace("{id}", threat.targetId);
  };

  const handleOverlayClick = (e: React.MouseEvent<HTMLDivElement>) => {
    if (e.target === e.currentTarget) {
      onClose();
    }
  };

  const recommendations = getRecommendations();

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-[rgba(29,29,31,0.45)] backdrop-blur-sm"
      onClick={handleOverlayClick}
    >
      <div className="glass-chrome max-w-2xl w-full mx-4 max-h-[90vh] overflow-y-auto">
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b border-divider">
          <div className="flex items-center gap-3">
            <div className={`p-2 rounded-inset-sm ${getSeverityColor(threat.severity)}`}>
              <AlertTriangle className="h-5 w-5" />
            </div>
            <div>
              <h2 className="text-xl font-semibold text-ink">
                Threat details
              </h2>
              <p className="text-sm text-ink-secondary">
                {threat.threatType.replace(/_/g, " ")}
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="text-ink-tertiary hover:text-ink transition-colors"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Body with Tabs */}
        <div className="p-6">
          {/* Quick Actions Bar */}
          <div className="flex flex-col gap-3 p-4 bg-brand-soft rounded-inset border border-stroke mb-6">
            <div className="flex items-center gap-2">
              <Zap className="h-5 w-5 text-brand-text" />
              <span className="text-sm font-medium text-ink">
                Quick actions
              </span>
            </div>
            <div className="flex items-center gap-2 flex-wrap">
              <Link
                href={`/dashboard/agents?search=${threat.targetId}`}
                className="inline-flex items-center gap-1 rounded-pill border border-stroke bg-glass px-3 py-1.5 text-xs font-medium text-brand-text transition-colors hover:bg-brand-soft"
              >
                <User className="h-3 w-3" />
                View agent
                <ExternalLink className="h-3 w-3" />
              </Link>
              <Link
                href={`/dashboard/agents/${threat.targetId}`}
                className="inline-flex items-center gap-1 rounded-pill border border-stroke bg-glass px-3 py-1.5 text-xs font-medium text-brand-text transition-colors hover:bg-brand-soft"
              >
                <Activity className="h-3 w-3" />
                Agent details
                <ExternalLink className="h-3 w-3" />
              </Link>
              <Link
                href="/dashboard/admin/compliance"
                className="inline-flex items-center gap-1 rounded-pill border border-stroke bg-glass px-3 py-1.5 text-xs font-medium text-brand-text transition-colors hover:bg-brand-soft"
              >
                <FileText className="h-3 w-3" />
                Audit log
                <ExternalLink className="h-3 w-3" />
              </Link>

              {loadingAgent ? (
                <span className="inline-flex items-center gap-1 px-3 py-1.5 text-xs text-ink-secondary">
                  <Loader2 className="h-3 w-3 animate-spin" />
                  Loading...
                </span>
              ) : agent?.createdBySdkTokenId ? (
                <>
                  <Link
                    href={`/dashboard/sdk-tokens?highlight=${agent.createdBySdkTokenId}`}
                    className="inline-flex items-center gap-1 rounded-pill border border-stroke bg-glass px-3 py-1.5 text-xs font-medium text-brand-text transition-colors hover:bg-brand-soft"
                  >
                    <KeyRound className="h-3 w-3" />
                    SDK token
                    <ExternalLink className="h-3 w-3" />
                  </Link>
                  {!revokeSuccess ? (
                    <button
                      onClick={() => setShowRevokeConfirm(true)}
                      className="inline-flex items-center gap-1 rounded-pill border border-danger-border bg-glass px-3 py-1.5 text-xs font-medium text-danger-text transition-colors hover:bg-danger-fill"
                    >
                      <Ban className="h-3 w-3" />
                      Revoke token
                    </button>
                  ) : (
                    <span className="inline-flex items-center gap-1 rounded-pill border border-success-border bg-success-fill px-3 py-1.5 text-xs font-medium text-success-text">
                      <Shield className="h-3 w-3" />
                      Revoked
                    </span>
                  )}
                </>
              ) : null}
            </div>
          </div>

          {/* SDK Token Revoke Confirmation */}
          {showRevokeConfirm && (
            <div className="glass-alert p-4 mb-6">
              <div className="flex items-start gap-3">
                <Ban className="h-5 w-5 text-danger-text flex-shrink-0 mt-0.5" />
                <div className="flex-1">
                  <h4 className="text-sm font-medium text-danger-text mb-1">
                    Confirm SDK token revocation
                  </h4>
                  <p className="text-xs text-ink-body mb-3">
                    This will immediately revoke the SDK token. Any applications using this token will no longer authenticate.
                    {agent?.createdByName && (
                      <span className="block mt-1 font-medium">
                        Token owner: {agent.createdByName} {agent.createdByEmail && `(${agent.createdByEmail})`}
                      </span>
                    )}
                  </p>
                  <div className="flex gap-2">
                    <button
                      onClick={handleRevokeSDKToken}
                      disabled={revokingToken}
                      className="inline-flex items-center gap-1 rounded-pill bg-danger px-3 py-1.5 text-xs font-medium text-danger-foreground transition-colors hover:brightness-95 disabled:opacity-50"
                    >
                      {revokingToken ? (
                        <>
                          <Loader2 className="h-3 w-3 animate-spin" />
                          Revoking...
                        </>
                      ) : (
                        <>
                          <Ban className="h-3 w-3" />
                          Revoke token
                        </>
                      )}
                    </button>
                    <button
                      onClick={() => setShowRevokeConfirm(false)}
                      className="rounded-pill border border-stroke bg-glass px-3 py-1.5 text-xs font-medium text-ink-body transition-colors hover:bg-glass-inset-gray"
                    >
                      Cancel
                    </button>
                  </div>
                </div>
              </div>
            </div>
          )}

          {/* Tabs */}
          <Tabs defaultValue="overview" className="w-full">
            <TabsList className="grid w-full grid-cols-3 mb-6">
              <TabsTrigger value="overview" className="flex items-center gap-1.5">
                <Eye className="h-4 w-4" />
                Overview
              </TabsTrigger>
              <TabsTrigger value="source" className="flex items-center gap-1.5">
                <Globe className="h-4 w-4" />
                Source & target
              </TabsTrigger>
              <TabsTrigger value="recommendations" className="flex items-center gap-1.5">
                <Zap className="h-4 w-4" />
                Actions
              </TabsTrigger>
            </TabsList>

            {/* Overview Tab */}
            <TabsContent value="overview" className="space-y-4">
              {/* Threat ID & Severity */}
              <div className="grid grid-cols-2 gap-4">
                <div className="glass-inset p-3">
                  <label className="text-xs font-medium text-ink-tertiary uppercase tracking-wide">
                    Threat ID
                  </label>
                  <p className="mt-1 text-sm text-ink font-mono break-all">
                    {threat.id}
                  </p>
                </div>
                <div className="glass-inset p-3">
                  <label className="text-xs font-medium text-ink-tertiary uppercase tracking-wide">
                    Severity
                  </label>
                  <p className="mt-1">
                    <span className={`inline-flex px-2.5 py-1 text-xs font-semibold rounded-pill uppercase ${getSeverityColor(threat.severity)}`}>
                      {threat.severity}
                    </span>
                  </p>
                </div>
              </div>

              {/* Threat Type */}
              <div className="glass-inset p-3">
                <label className="text-xs font-medium text-ink-tertiary uppercase tracking-wide">
                  Threat type
                </label>
                <p className="mt-1 text-sm text-ink font-semibold capitalize">
                  {threat.threatType.replace(/_/g, " ")}
                </p>
              </div>

              {/* Description */}
              <div className="glass-inset p-3">
                <label className="text-xs font-medium text-ink-tertiary uppercase tracking-wide">
                  Description
                </label>
                <p className="mt-1 text-sm text-ink leading-relaxed">
                  {threat.description}
                </p>
              </div>

              {/* Status & Detection Time */}
              <div className="grid grid-cols-2 gap-4">
                <div className="glass-inset p-3">
                  <label className="text-xs font-medium text-ink-tertiary uppercase tracking-wide">
                    Status
                  </label>
                  <p className="mt-1">
                    <span className={`inline-flex px-2.5 py-1 text-xs font-semibold rounded-pill uppercase ${getStatusColor(threat.isBlocked)}`}>
                      {threat.isBlocked ? "Blocked" : "Active"}
                    </span>
                  </p>
                </div>
                <div className="glass-inset p-3">
                  <label className="text-xs font-medium text-ink-tertiary uppercase tracking-wide">
                    Detected at
                  </label>
                  <p className="mt-1 text-sm text-ink">
                    {formatDateTime(threat.createdAt)}
                  </p>
                </div>
              </div>
            </TabsContent>

            {/* Source & Target Tab */}
            <TabsContent value="source" className="space-y-4">
              {/* Source Information */}
              <div className="p-4 bg-glass-inset-gray rounded-inset border border-stroke">
                <div className="flex items-center gap-2 mb-3">
                  <Globe className="h-5 w-5 text-ink-secondary" />
                  <span className="font-medium text-ink">Source information</span>
                </div>

                {threat.source ? (
                  <div className="space-y-3">
                    {isIPAddress(threat.source) ? (
                      <div className="flex items-center gap-3">
                        <span className="px-2 py-0.5 bg-brand-soft text-brand-text rounded-inset-sm text-xs font-medium">
                          IP address
                        </span>
                        <span className="font-mono text-sm text-ink">
                          {threat.source}
                        </span>
                      </div>
                    ) : isUUID(threat.source) ? (
                      <>
                        <div className="flex items-center gap-3">
                          <span className="px-2 py-0.5 bg-brand-soft text-brand-text rounded-inset-sm text-xs font-medium">
                            Agent ID
                          </span>
                          <span className="font-mono text-sm text-ink">
                            {threat.source}
                          </span>
                        </div>
                        <p className="text-xs text-ink-secondary flex items-center gap-1">
                          <Lock className="h-3 w-3" />
                          Source IP not captured for this event
                        </p>
                      </>
                    ) : (
                      <div className="flex items-center gap-3">
                        <span className="px-2 py-0.5 bg-glass-inset-gray text-ink-body rounded-inset-sm text-xs font-medium">
                          Source
                        </span>
                        <span className="font-mono text-sm text-ink">
                          {threat.source}
                        </span>
                      </div>
                    )}
                  </div>
                ) : (
                  <p className="text-sm text-ink-secondary">
                    Source information not available
                  </p>
                )}
              </div>

              {/* Target Information */}
              <div className="p-4 bg-glass-inset-gray rounded-inset border border-stroke">
                <div className="flex items-center gap-2 mb-3">
                  <Target className="h-5 w-5 text-ink-secondary" />
                  <span className="font-medium text-ink">Affected target</span>
                </div>

                <div className="space-y-3">
                  {/* Agent Name - prioritize fetched agent data over threat.targetName which may be truncated ID */}
                  <div className="flex items-center gap-3">
                    <span className="px-2 py-0.5 bg-brand-soft text-brand-text rounded-inset-sm text-xs font-medium">
                      Agent
                    </span>
                    <span className="font-medium text-ink">
                      {agent?.name || agent?.displayName || (
                        // If threat.targetName looks like UUID/truncated ID, show loading; otherwise show it
                        threat.targetName && !threat.targetName.match(/^[0-9a-f-]+\.{0,3}$/i)
                          ? threat.targetName
                          : "Loading..."
                      )}
                    </span>
                  </div>

                  {/* Agent ID - show full UUID for reference */}
                  <div className="flex items-center gap-3">
                    <span className="px-2 py-0.5 bg-glass-inset-gray text-ink-secondary rounded-inset-sm text-xs font-medium">
                      ID
                    </span>
                    <span className="font-mono text-xs text-ink-secondary">
                      {threat.targetId}
                    </span>
                  </div>

                  {agent && (
                    <>
                      {agent.trustScore !== undefined && (
                        <div className="flex items-center gap-3">
                          <span className="px-2 py-0.5 bg-warning-fill text-warning-text rounded-inset-sm text-xs font-medium">
                            Trust score
                          </span>
                          <span className={`font-semibold ${
                            agent.trustScore >= 0.7 ? "text-success-text" :
                            agent.trustScore >= 0.4 ? "text-warning-text" :
                            "text-danger-text"
                          }`}>
                            {Math.round((agent.trustScore <= 1 ? agent.trustScore * 100 : agent.trustScore))}%
                          </span>
                        </div>
                      )}
                      {agent.status && (
                        <div className="flex items-center gap-3">
                          <span className="px-2 py-0.5 bg-glass-inset-gray text-ink-secondary rounded-inset-sm text-xs font-medium">
                            Status
                          </span>
                          <span className={`font-medium capitalize ${
                            agent.status === "verified" ? "text-success-text" :
                            agent.status === "suspended" ? "text-danger-text" :
                            "text-ink-secondary"
                          }`}>
                            {agent.status}
                          </span>
                        </div>
                      )}
                    </>
                  )}
                </div>
              </div>

              {/* Additional Metadata */}
              {threat.metadata && Object.keys(threat.metadata).length > 0 && (
                <div className="p-4 bg-glass-inset-gray rounded-inset border border-stroke">
                  <div className="flex items-center gap-2 mb-3">
                    <Server className="h-5 w-5 text-ink-secondary" />
                    <span className="font-medium text-ink">Additional details</span>
                  </div>
                  <pre className="text-xs text-ink-body font-mono overflow-x-auto bg-glass p-3 rounded-inset-sm border border-stroke">
                    {JSON.stringify(threat.metadata, null, 2)}
                  </pre>
                </div>
              )}
            </TabsContent>

            {/* Recommendations Tab */}
            <TabsContent value="recommendations" className="space-y-3">
              <div className="flex items-center gap-2 mb-4">
                <AlertTriangle className="h-5 w-5 text-warning" />
                <span className="text-sm font-medium text-ink">
                  Recommended actions for <span className="capitalize">{threat.threatType.replace(/_/g, " ")}</span>
                </span>
              </div>

              {recommendations.map((rec, index) => {
                const IconComponent = rec.icon;
                return (
                  <div
                    key={index}
                    className={`p-4 rounded-inset border ${getPriorityColor(rec.priority)}`}
                  >
                    <div className="flex items-start gap-3">
                      <div className={`p-1.5 rounded-inset-sm ${getPriorityColor(rec.priority)}`}>
                        <IconComponent className={`h-4 w-4 ${getPriorityIconColor(rec.priority)}`} />
                      </div>
                      <div className="flex-1">
                        <div className="flex items-center gap-2 mb-1">
                          <span className={`px-1.5 py-0.5 text-xs font-semibold uppercase rounded-inset-sm ${
                            rec.priority === "critical" ? "bg-danger-fill text-danger-text" :
                            rec.priority === "high" ? "bg-warning-fill text-warning-text" :
                            rec.priority === "medium" ? "bg-warning-fill text-warning-text" :
                            "bg-brand-soft text-brand-text"
                          }`}>
                            {rec.priority}
                          </span>
                          <span className="font-medium text-ink">
                            {rec.action}
                          </span>
                        </div>
                        <p className="text-sm text-ink-secondary mb-2">
                          {rec.description}
                        </p>
                        {rec.link && (
                          <Link
                            href={resolveLink(rec.link)}
                            className="inline-flex items-center gap-1 text-xs font-medium text-brand-text hover:underline"
                          >
                            Take action
                            <ExternalLink className="h-3 w-3" />
                          </Link>
                        )}
                      </div>
                    </div>
                  </div>
                );
              })}

              {/* Additional SDK Token Warning */}
              {agent?.createdBySdkTokenId && !revokeSuccess && (
                <div className="glass-alert p-4">
                  <div className="flex items-start gap-3">
                    <KeyRound className="h-5 w-5 text-danger-text flex-shrink-0" />
                    <div>
                      <p className="text-sm font-medium text-danger-text">
                        SDK token available for revocation
                      </p>
                      <p className="text-xs text-ink-body mt-1">
                        If this agent is compromised, you can immediately revoke its SDK token using the quick actions above.
                      </p>
                    </div>
                  </div>
                </div>
              )}
            </TabsContent>
          </Tabs>
        </div>

        {/* Footer */}
        <div className="flex justify-end p-6 border-t border-divider">
          <Button onClick={onClose} className="px-6">
            Close
          </Button>
        </div>
      </div>
    </div>
  );
}

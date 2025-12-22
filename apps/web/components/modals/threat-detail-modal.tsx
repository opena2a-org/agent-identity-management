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
      action: "Revoke SDK Token Immediately",
      description: "Prevent further unauthorized access by revoking the token used to create this agent",
      link: "/dashboard/sdk-tokens",
      icon: Ban,
    },
    {
      priority: "high",
      action: "Block Agent & Investigate",
      description: "Suspend the agent and review all recent activity for signs of compromise",
      link: "/dashboard/agents/{id}",
      icon: UserX,
    },
    {
      priority: "medium",
      action: "Review Violation History",
      description: "Check the compliance logs for patterns of malicious behavior",
      link: "/dashboard/admin/compliance",
      icon: FileWarning,
    },
  ],
  unauthorized_access: [
    {
      priority: "high",
      action: "Review Agent Capabilities",
      description: "Compare the attempted action against the agent's registered capabilities",
      link: "/dashboard/agents/{id}",
      icon: Shield,
    },
    {
      priority: "medium",
      action: "Approve or Deny Capability",
      description: "If legitimate, grant the capability through the requests dashboard",
      link: "/dashboard/admin/capability-requests",
      icon: CheckCircle2,
    },
    {
      priority: "low",
      action: "Monitor for Patterns",
      description: "Watch for recurring unauthorized access attempts from this agent",
      link: "/dashboard/security",
      icon: Eye,
    },
  ],
  configuration_drift: [
    {
      priority: "high",
      action: "Compare Configurations",
      description: "Review registered vs detected MCP servers and capabilities",
      link: "/dashboard/agents/{id}",
      icon: RefreshCw,
    },
    {
      priority: "medium",
      action: "Update Registration",
      description: "If drift is legitimate, update the agent's registered configuration",
      link: "/dashboard/agents/{id}",
      icon: Settings,
    },
    {
      priority: "low",
      action: "Review Drift Policy",
      description: "Adjust drift detection sensitivity in security policies",
      link: "/dashboard/security/policies",
      icon: FileText,
    },
  ],
  suspicious_activity: [
    {
      priority: "high",
      action: "Review Recent Activity",
      description: "Examine the agent's recent actions for anomalous patterns",
      link: "/dashboard/agents/{id}",
      icon: Activity,
    },
    {
      priority: "medium",
      action: "Check Trust Score Trend",
      description: "Review how the agent's trust score has changed over time",
      link: "/dashboard/agents/{id}",
      icon: Activity,
    },
    {
      priority: "low",
      action: "Adjust Monitoring Threshold",
      description: "Fine-tune what qualifies as suspicious activity",
      link: "/dashboard/security/policies",
      icon: Settings,
    },
  ],
  credential_leak: [
    {
      priority: "critical",
      action: "Rotate All Credentials",
      description: "Immediately rotate API keys and tokens associated with this agent",
      link: "/dashboard/sdk-tokens",
      icon: RefreshCw,
    },
    {
      priority: "high",
      action: "Revoke Compromised Tokens",
      description: "Revoke any tokens that may have been exposed",
      link: "/dashboard/sdk-tokens",
      icon: Ban,
    },
    {
      priority: "medium",
      action: "Audit Access Logs",
      description: "Review who accessed what with the compromised credentials",
      link: "/dashboard/admin/compliance",
      icon: Database,
    },
  ],
  certificate_expiry: [
    {
      priority: "high",
      action: "Renew Certificate",
      description: "Generate and deploy a new certificate before expiration",
      link: "/dashboard/agents/{id}",
      icon: RefreshCw,
    },
    {
      priority: "medium",
      action: "Update Agent Config",
      description: "Ensure the agent is configured to use the new certificate",
      link: "/dashboard/agents/{id}",
      icon: Settings,
    },
    {
      priority: "low",
      action: "Set Up Auto-Renewal",
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
    action: "Investigate the Threat",
    description: "Review agent activity logs and audit trail for suspicious patterns",
    link: "/dashboard/admin/compliance",
    icon: Eye,
  },
  {
    priority: "medium",
    action: "Verify Agent Configuration",
    description: "Ensure agent capabilities match its registered scope",
    link: "/dashboard/agents/{id}",
    icon: Shield,
  },
  {
    priority: "low",
    action: "Monitor Trust Score",
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
        return "bg-red-100 text-red-800 dark:bg-red-900/20 dark:text-red-400";
      case "high":
        return "bg-orange-100 text-orange-800 dark:bg-orange-900/20 dark:text-orange-400";
      case "medium":
        return "bg-yellow-100 text-yellow-800 dark:bg-yellow-900/20 dark:text-yellow-400";
      case "low":
        return "bg-blue-100 text-blue-800 dark:bg-blue-900/20 dark:text-blue-400";
      default:
        return "bg-gray-100 text-gray-800 dark:bg-gray-900/20 dark:text-gray-400";
    }
  };

  const getPriorityColor = (priority: string) => {
    switch (priority) {
      case "critical":
        return "bg-red-50 border-red-200 dark:bg-red-900/10 dark:border-red-800";
      case "high":
        return "bg-orange-50 border-orange-200 dark:bg-orange-900/10 dark:border-orange-800";
      case "medium":
        return "bg-yellow-50 border-yellow-200 dark:bg-yellow-900/10 dark:border-yellow-800";
      case "low":
        return "bg-blue-50 border-blue-200 dark:bg-blue-900/10 dark:border-blue-800";
      default:
        return "bg-gray-50 border-gray-200 dark:bg-gray-900/10 dark:border-gray-700";
    }
  };

  const getPriorityIconColor = (priority: string) => {
    switch (priority) {
      case "critical":
        return "text-red-600 dark:text-red-400";
      case "high":
        return "text-orange-600 dark:text-orange-400";
      case "medium":
        return "text-yellow-600 dark:text-yellow-400";
      case "low":
        return "text-blue-600 dark:text-blue-400";
      default:
        return "text-gray-600 dark:text-gray-400";
    }
  };

  const getStatusColor = (isBlocked: boolean) => {
    return isBlocked
      ? "bg-green-100 text-green-800 dark:bg-green-900/20 dark:text-green-400"
      : "bg-red-100 text-red-800 dark:bg-red-900/20 dark:text-red-400";
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
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
      onClick={handleOverlayClick}
    >
      <div className="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-2xl w-full mx-4 max-h-[90vh] overflow-y-auto">
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b border-gray-200 dark:border-gray-700">
          <div className="flex items-center gap-3">
            <div className={`p-2 rounded-lg ${getSeverityColor(threat.severity)}`}>
              <AlertTriangle className="h-5 w-5" />
            </div>
            <div>
              <h2 className="text-xl font-semibold text-gray-900 dark:text-white">
                Threat Details
              </h2>
              <p className="text-sm text-gray-500 dark:text-gray-400">
                {threat.threatType.replace(/_/g, " ")}
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Body with Tabs */}
        <div className="p-6">
          {/* Quick Actions Bar */}
          <div className="flex flex-col gap-3 p-4 bg-blue-50 dark:bg-blue-900/10 rounded-lg border border-blue-200 dark:border-blue-800 mb-6">
            <div className="flex items-center gap-2">
              <Zap className="h-5 w-5 text-blue-600 dark:text-blue-400" />
              <span className="text-sm font-medium text-blue-900 dark:text-blue-100">
                Quick Actions
              </span>
            </div>
            <div className="flex items-center gap-2 flex-wrap">
              <Link
                href={`/dashboard/agents?search=${threat.targetId}`}
                className="inline-flex items-center gap-1 px-3 py-1.5 text-xs font-medium text-blue-700 dark:text-blue-300 bg-white dark:bg-gray-800 border border-blue-300 dark:border-blue-700 rounded-lg hover:bg-blue-50 dark:hover:bg-blue-900/20 transition-colors"
              >
                <User className="h-3 w-3" />
                View Agent
                <ExternalLink className="h-3 w-3" />
              </Link>
              <Link
                href={`/dashboard/agents/${threat.targetId}`}
                className="inline-flex items-center gap-1 px-3 py-1.5 text-xs font-medium text-blue-700 dark:text-blue-300 bg-white dark:bg-gray-800 border border-blue-300 dark:border-blue-700 rounded-lg hover:bg-blue-50 dark:hover:bg-blue-900/20 transition-colors"
              >
                <Activity className="h-3 w-3" />
                Agent Details
                <ExternalLink className="h-3 w-3" />
              </Link>
              <Link
                href="/dashboard/admin/compliance"
                className="inline-flex items-center gap-1 px-3 py-1.5 text-xs font-medium text-blue-700 dark:text-blue-300 bg-white dark:bg-gray-800 border border-blue-300 dark:border-blue-700 rounded-lg hover:bg-blue-50 dark:hover:bg-blue-900/20 transition-colors"
              >
                <FileText className="h-3 w-3" />
                Audit Log
                <ExternalLink className="h-3 w-3" />
              </Link>

              {loadingAgent ? (
                <span className="inline-flex items-center gap-1 px-3 py-1.5 text-xs text-gray-500">
                  <Loader2 className="h-3 w-3 animate-spin" />
                  Loading...
                </span>
              ) : agent?.createdBySdkTokenId ? (
                <>
                  <Link
                    href={`/dashboard/sdk-tokens?highlight=${agent.createdBySdkTokenId}`}
                    className="inline-flex items-center gap-1 px-3 py-1.5 text-xs font-medium text-purple-700 dark:text-purple-300 bg-white dark:bg-gray-800 border border-purple-300 dark:border-purple-700 rounded-lg hover:bg-purple-50 dark:hover:bg-purple-900/20 transition-colors"
                  >
                    <KeyRound className="h-3 w-3" />
                    SDK Token
                    <ExternalLink className="h-3 w-3" />
                  </Link>
                  {!revokeSuccess ? (
                    <button
                      onClick={() => setShowRevokeConfirm(true)}
                      className="inline-flex items-center gap-1 px-3 py-1.5 text-xs font-medium text-red-700 dark:text-red-300 bg-white dark:bg-gray-800 border border-red-300 dark:border-red-700 rounded-lg hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors"
                    >
                      <Ban className="h-3 w-3" />
                      Revoke Token
                    </button>
                  ) : (
                    <span className="inline-flex items-center gap-1 px-3 py-1.5 text-xs font-medium text-green-700 dark:text-green-300 bg-green-100 dark:bg-green-900/20 border border-green-300 dark:border-green-700 rounded-lg">
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
            <div className="p-4 bg-red-50 dark:bg-red-900/10 rounded-lg border border-red-200 dark:border-red-800 mb-6">
              <div className="flex items-start gap-3">
                <Ban className="h-5 w-5 text-red-600 dark:text-red-400 flex-shrink-0 mt-0.5" />
                <div className="flex-1">
                  <h4 className="text-sm font-medium text-red-900 dark:text-red-100 mb-1">
                    Confirm SDK Token Revocation
                  </h4>
                  <p className="text-xs text-red-800 dark:text-red-200 mb-3">
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
                      className="inline-flex items-center gap-1 px-3 py-1.5 text-xs font-medium text-white bg-red-600 rounded-lg hover:bg-red-700 disabled:opacity-50 transition-colors"
                    >
                      {revokingToken ? (
                        <>
                          <Loader2 className="h-3 w-3 animate-spin" />
                          Revoking...
                        </>
                      ) : (
                        <>
                          <Ban className="h-3 w-3" />
                          Revoke Token
                        </>
                      )}
                    </button>
                    <button
                      onClick={() => setShowRevokeConfirm(false)}
                      className="px-3 py-1.5 text-xs font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
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
                Source & Target
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
                <div className="p-3 bg-gray-50 dark:bg-gray-900/50 rounded-lg">
                  <label className="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide">
                    Threat ID
                  </label>
                  <p className="mt-1 text-sm text-gray-900 dark:text-white font-mono break-all">
                    {threat.id}
                  </p>
                </div>
                <div className="p-3 bg-gray-50 dark:bg-gray-900/50 rounded-lg">
                  <label className="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide">
                    Severity
                  </label>
                  <p className="mt-1">
                    <span className={`inline-flex px-2.5 py-1 text-xs font-semibold rounded-full uppercase ${getSeverityColor(threat.severity)}`}>
                      {threat.severity}
                    </span>
                  </p>
                </div>
              </div>

              {/* Threat Type */}
              <div className="p-3 bg-gray-50 dark:bg-gray-900/50 rounded-lg">
                <label className="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide">
                  Threat Type
                </label>
                <p className="mt-1 text-sm text-gray-900 dark:text-white font-semibold capitalize">
                  {threat.threatType.replace(/_/g, " ")}
                </p>
              </div>

              {/* Description */}
              <div className="p-3 bg-gray-50 dark:bg-gray-900/50 rounded-lg">
                <label className="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide">
                  Description
                </label>
                <p className="mt-1 text-sm text-gray-900 dark:text-white leading-relaxed">
                  {threat.description}
                </p>
              </div>

              {/* Status & Detection Time */}
              <div className="grid grid-cols-2 gap-4">
                <div className="p-3 bg-gray-50 dark:bg-gray-900/50 rounded-lg">
                  <label className="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide">
                    Status
                  </label>
                  <p className="mt-1">
                    <span className={`inline-flex px-2.5 py-1 text-xs font-semibold rounded-full uppercase ${getStatusColor(threat.isBlocked)}`}>
                      {threat.isBlocked ? "Blocked" : "Active"}
                    </span>
                  </p>
                </div>
                <div className="p-3 bg-gray-50 dark:bg-gray-900/50 rounded-lg">
                  <label className="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide">
                    Detected At
                  </label>
                  <p className="mt-1 text-sm text-gray-900 dark:text-white">
                    {formatDateTime(threat.createdAt)}
                  </p>
                </div>
              </div>
            </TabsContent>

            {/* Source & Target Tab */}
            <TabsContent value="source" className="space-y-4">
              {/* Source Information */}
              <div className="p-4 bg-slate-50 dark:bg-slate-900/50 rounded-lg border border-slate-200 dark:border-slate-700">
                <div className="flex items-center gap-2 mb-3">
                  <Globe className="h-5 w-5 text-slate-600 dark:text-slate-400" />
                  <span className="font-medium text-gray-900 dark:text-white">Source Information</span>
                </div>

                {threat.source ? (
                  <div className="space-y-3">
                    {isIPAddress(threat.source) ? (
                      <div className="flex items-center gap-3">
                        <span className="px-2 py-0.5 bg-purple-100 dark:bg-purple-900/30 text-purple-700 dark:text-purple-400 rounded text-xs font-medium">
                          IP Address
                        </span>
                        <span className="font-mono text-sm text-gray-900 dark:text-white">
                          {threat.source}
                        </span>
                      </div>
                    ) : isUUID(threat.source) ? (
                      <>
                        <div className="flex items-center gap-3">
                          <span className="px-2 py-0.5 bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-400 rounded text-xs font-medium">
                            Agent ID
                          </span>
                          <span className="font-mono text-sm text-gray-900 dark:text-white">
                            {threat.source}
                          </span>
                        </div>
                        <p className="text-xs text-gray-500 dark:text-gray-400 flex items-center gap-1">
                          <Lock className="h-3 w-3" />
                          Source IP not captured for this event
                        </p>
                      </>
                    ) : (
                      <div className="flex items-center gap-3">
                        <span className="px-2 py-0.5 bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 rounded text-xs font-medium">
                          Source
                        </span>
                        <span className="font-mono text-sm text-gray-900 dark:text-white">
                          {threat.source}
                        </span>
                      </div>
                    )}
                  </div>
                ) : (
                  <p className="text-sm text-gray-500 dark:text-gray-400">
                    Source information not available
                  </p>
                )}
              </div>

              {/* Target Information */}
              <div className="p-4 bg-slate-50 dark:bg-slate-900/50 rounded-lg border border-slate-200 dark:border-slate-700">
                <div className="flex items-center gap-2 mb-3">
                  <Target className="h-5 w-5 text-slate-600 dark:text-slate-400" />
                  <span className="font-medium text-gray-900 dark:text-white">Affected Target</span>
                </div>

                <div className="space-y-3">
                  {/* Agent Name - prioritize fetched agent data over threat.targetName which may be truncated ID */}
                  <div className="flex items-center gap-3">
                    <span className="px-2 py-0.5 bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-400 rounded text-xs font-medium">
                      Agent
                    </span>
                    <span className="font-medium text-gray-900 dark:text-white">
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
                    <span className="px-2 py-0.5 bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400 rounded text-xs font-medium">
                      ID
                    </span>
                    <span className="font-mono text-xs text-gray-600 dark:text-gray-400">
                      {threat.targetId}
                    </span>
                  </div>

                  {agent && (
                    <>
                      {agent.trustScore !== undefined && (
                        <div className="flex items-center gap-3">
                          <span className="px-2 py-0.5 bg-amber-100 dark:bg-amber-900/30 text-amber-700 dark:text-amber-400 rounded text-xs font-medium">
                            Trust Score
                          </span>
                          <span className={`font-semibold ${
                            agent.trustScore >= 0.7 ? "text-green-600 dark:text-green-400" :
                            agent.trustScore >= 0.4 ? "text-yellow-600 dark:text-yellow-400" :
                            "text-red-600 dark:text-red-400"
                          }`}>
                            {Math.round((agent.trustScore <= 1 ? agent.trustScore * 100 : agent.trustScore))}%
                          </span>
                        </div>
                      )}
                      {agent.status && (
                        <div className="flex items-center gap-3">
                          <span className="px-2 py-0.5 bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400 rounded text-xs font-medium">
                            Status
                          </span>
                          <span className={`font-medium capitalize ${
                            agent.status === "verified" ? "text-green-600 dark:text-green-400" :
                            agent.status === "suspended" ? "text-red-600 dark:text-red-400" :
                            "text-gray-600 dark:text-gray-400"
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
                <div className="p-4 bg-slate-50 dark:bg-slate-900/50 rounded-lg border border-slate-200 dark:border-slate-700">
                  <div className="flex items-center gap-2 mb-3">
                    <Server className="h-5 w-5 text-slate-600 dark:text-slate-400" />
                    <span className="font-medium text-gray-900 dark:text-white">Additional Details</span>
                  </div>
                  <pre className="text-xs text-gray-700 dark:text-gray-300 font-mono overflow-x-auto bg-white dark:bg-gray-800 p-3 rounded border border-gray-200 dark:border-gray-700">
                    {JSON.stringify(threat.metadata, null, 2)}
                  </pre>
                </div>
              )}
            </TabsContent>

            {/* Recommendations Tab */}
            <TabsContent value="recommendations" className="space-y-3">
              <div className="flex items-center gap-2 mb-4">
                <AlertTriangle className="h-5 w-5 text-amber-500" />
                <span className="text-sm font-medium text-gray-900 dark:text-white">
                  Recommended Actions for <span className="capitalize">{threat.threatType.replace(/_/g, " ")}</span>
                </span>
              </div>

              {recommendations.map((rec, index) => {
                const IconComponent = rec.icon;
                return (
                  <div
                    key={index}
                    className={`p-4 rounded-lg border ${getPriorityColor(rec.priority)}`}
                  >
                    <div className="flex items-start gap-3">
                      <div className={`p-1.5 rounded ${getPriorityColor(rec.priority)}`}>
                        <IconComponent className={`h-4 w-4 ${getPriorityIconColor(rec.priority)}`} />
                      </div>
                      <div className="flex-1">
                        <div className="flex items-center gap-2 mb-1">
                          <span className={`px-1.5 py-0.5 text-xs font-semibold uppercase rounded ${
                            rec.priority === "critical" ? "bg-red-200 text-red-800 dark:bg-red-800 dark:text-red-200" :
                            rec.priority === "high" ? "bg-orange-200 text-orange-800 dark:bg-orange-800 dark:text-orange-200" :
                            rec.priority === "medium" ? "bg-yellow-200 text-yellow-800 dark:bg-yellow-800 dark:text-yellow-200" :
                            "bg-blue-200 text-blue-800 dark:bg-blue-800 dark:text-blue-200"
                          }`}>
                            {rec.priority}
                          </span>
                          <span className="font-medium text-gray-900 dark:text-white">
                            {rec.action}
                          </span>
                        </div>
                        <p className="text-sm text-gray-600 dark:text-gray-400 mb-2">
                          {rec.description}
                        </p>
                        {rec.link && (
                          <Link
                            href={resolveLink(rec.link)}
                            className="inline-flex items-center gap-1 text-xs font-medium text-blue-600 dark:text-blue-400 hover:underline"
                          >
                            Take Action
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
                <div className="p-4 bg-red-50 dark:bg-red-900/10 rounded-lg border border-red-200 dark:border-red-800">
                  <div className="flex items-start gap-3">
                    <KeyRound className="h-5 w-5 text-red-600 dark:text-red-400 flex-shrink-0" />
                    <div>
                      <p className="text-sm font-medium text-red-900 dark:text-red-100">
                        SDK Token Available for Revocation
                      </p>
                      <p className="text-xs text-red-800 dark:text-red-200 mt-1">
                        If this agent is compromised, you can immediately revoke its SDK token using the Quick Actions above.
                      </p>
                    </div>
                  </div>
                </div>
              )}
            </TabsContent>
          </Tabs>
        </div>

        {/* Footer */}
        <div className="flex justify-end p-6 border-t border-gray-200 dark:border-gray-700">
          <button
            onClick={onClose}
            className="px-6 py-2 text-sm font-medium text-white bg-blue-600 rounded-lg hover:bg-blue-700 transition-colors"
          >
            Close
          </button>
        </div>
      </div>
    </div>
  );
}

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
} from "lucide-react";
import Link from "next/link";
import { formatDateTime } from "@/lib/date-utils";
import { api, Agent } from "@/lib/api";

interface SecurityThreat {
  id: string;
  targetId: string;
  targetName?: string;
  threatType: string;
  severity: "low" | "medium" | "high" | "critical";
  description: string;
  isBlocked: boolean;
  createdAt: string;
  source?: string; // IP address or agent ID
  metadata?: Record<string, any>;
}

interface ThreatDetailModalProps {
  isOpen: boolean;
  onClose: () => void;
  threat: SecurityThreat | null;
}

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

  // Fetch agent details to get SDK token ID
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

  const getStatusColor = (isBlocked: boolean) => {
    return isBlocked
      ? "bg-green-100 text-green-800 dark:bg-green-900/20 dark:text-green-400"
      : "bg-red-100 text-red-800 dark:bg-red-900/20 dark:text-red-400";
  };

  // Handle click on overlay (outside modal) - threat detail modal is read-only, so close immediately
  const handleOverlayClick = (e: React.MouseEvent<HTMLDivElement>) => {
    if (e.target === e.currentTarget) {
      onClose();
    }
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
      onClick={handleOverlayClick}
    >
      <div className="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-2xl w-full mx-4 max-h-[90vh] overflow-y-auto">
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b border-gray-200 dark:border-gray-700">
          <div className="flex items-center gap-3">
            <div
              className={`p-2 rounded-lg ${getSeverityColor(threat.severity)}`}
            >
              <AlertTriangle className="h-5 w-5" />
            </div>
            <h2 className="text-xl font-semibold text-gray-900 dark:text-white">
              Threat Details
            </h2>
          </div>
          <button
            onClick={onClose}
            className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Body */}
        <div className="p-6 space-y-6">
          {/* Quick Actions Bar */}
          <div className="flex flex-col gap-3 p-4 bg-blue-50 dark:bg-blue-900/10 rounded-lg border border-blue-200 dark:border-blue-800">
            <div className="flex items-center gap-2">
              <Shield className="h-5 w-5 text-blue-600 dark:text-blue-400" />
              <span className="text-sm font-medium text-blue-900 dark:text-blue-100">
                Quick Actions:
              </span>
            </div>
            <div className="flex items-center gap-2 flex-wrap">
              <Link
                href={`/dashboard/agents?search=${threat.targetId}`}
                className="inline-flex items-center gap-1 px-3 py-1 text-xs font-medium text-blue-700 dark:text-blue-300 bg-white dark:bg-gray-800 border border-blue-300 dark:border-blue-700 rounded-lg hover:bg-blue-50 dark:hover:bg-blue-900/20 transition-colors"
              >
                <User className="h-3 w-3" />
                View Agent
                <ExternalLink className="h-3 w-3" />
              </Link>
              <Link
                href={`/dashboard/agents/${threat.targetId}`}
                className="inline-flex items-center gap-1 px-3 py-1 text-xs font-medium text-blue-700 dark:text-blue-300 bg-white dark:bg-gray-800 border border-blue-300 dark:border-blue-700 rounded-lg hover:bg-blue-50 dark:hover:bg-blue-900/20 transition-colors"
              >
                <Activity className="h-3 w-3" />
                View Details
                <ExternalLink className="h-3 w-3" />
              </Link>
              <Link
                href={`/dashboard/admin/compliance`}
                className="inline-flex items-center gap-1 px-3 py-1 text-xs font-medium text-blue-700 dark:text-blue-300 bg-white dark:bg-gray-800 border border-blue-300 dark:border-blue-700 rounded-lg hover:bg-blue-50 dark:hover:bg-blue-900/20 transition-colors"
              >
                <FileText className="h-3 w-3" />
                View Audit Log
                <ExternalLink className="h-3 w-3" />
              </Link>

              {/* SDK Token Quick Actions */}
              {loadingAgent ? (
                <span className="inline-flex items-center gap-1 px-3 py-1 text-xs text-gray-500">
                  <Loader2 className="h-3 w-3 animate-spin" />
                  Loading...
                </span>
              ) : agent?.createdBySdkTokenId ? (
                <>
                  <Link
                    href={`/dashboard/sdk-tokens?highlight=${agent.createdBySdkTokenId}`}
                    className="inline-flex items-center gap-1 px-3 py-1 text-xs font-medium text-purple-700 dark:text-purple-300 bg-white dark:bg-gray-800 border border-purple-300 dark:border-purple-700 rounded-lg hover:bg-purple-50 dark:hover:bg-purple-900/20 transition-colors"
                  >
                    <KeyRound className="h-3 w-3" />
                    View SDK Token
                    <ExternalLink className="h-3 w-3" />
                  </Link>
                  {!revokeSuccess ? (
                    <button
                      onClick={() => setShowRevokeConfirm(true)}
                      className="inline-flex items-center gap-1 px-3 py-1 text-xs font-medium text-red-700 dark:text-red-300 bg-white dark:bg-gray-800 border border-red-300 dark:border-red-700 rounded-lg hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors"
                    >
                      <Ban className="h-3 w-3" />
                      Revoke SDK Token
                    </button>
                  ) : (
                    <span className="inline-flex items-center gap-1 px-3 py-1 text-xs font-medium text-green-700 dark:text-green-300 bg-green-100 dark:bg-green-900/20 border border-green-300 dark:border-green-700 rounded-lg">
                      <Shield className="h-3 w-3" />
                      Token Revoked
                    </span>
                  )}
                </>
              ) : null}
            </div>
          </div>

          {/* SDK Token Revoke Confirmation */}
          {showRevokeConfirm && (
            <div className="p-4 bg-red-50 dark:bg-red-900/10 rounded-lg border border-red-200 dark:border-red-800">
              <div className="flex items-start gap-3">
                <Ban className="h-5 w-5 text-red-600 dark:text-red-400 flex-shrink-0 mt-0.5" />
                <div className="flex-1">
                  <h4 className="text-sm font-medium text-red-900 dark:text-red-100 mb-1">
                    Confirm SDK Token Revocation
                  </h4>
                  <p className="text-xs text-red-800 dark:text-red-200 mb-3">
                    This will immediately revoke the SDK token used to create this agent.
                    Any applications using this token will no longer be able to authenticate.
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
                          Yes, Revoke Token
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

          {/* Basic Info */}
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="text-sm font-medium text-gray-500 dark:text-gray-400">
                Threat ID
              </label>
              <p className="mt-1 text-sm text-gray-900 dark:text-white font-mono">
                {threat.id}
              </p>
            </div>
            <div>
              <label className="text-sm font-medium text-gray-500 dark:text-gray-400">
                Severity
              </label>
              <p className="mt-1">
                <span
                  className={`inline-flex px-2 py-1 text-xs font-semibold rounded-full uppercase ${getSeverityColor(threat.severity)}`}
                >
                  {threat.severity}
                </span>
              </p>
            </div>
          </div>

          {/* Threat Type */}
          <div>
            <label className="text-sm font-medium text-gray-500 dark:text-gray-400">
              Threat Type
            </label>
            <p className="mt-1 text-sm text-gray-900 dark:text-white font-semibold">
              {threat.threatType}
            </p>
          </div>

          {/* Description */}
          <div>
            <label className="text-sm font-medium text-gray-500 dark:text-gray-400">
              Description
            </label>
            <p className="mt-1 text-sm text-gray-900 dark:text-white leading-relaxed">
              {threat.description}
            </p>
          </div>

          {/* Target Info */}
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="text-sm font-medium text-gray-500 dark:text-gray-400">
                Affected Target
              </label>
              <p className="mt-1 text-sm text-gray-900 dark:text-white font-medium">
                {threat.targetName || threat.targetId}
              </p>
              <p className="mt-0.5 text-xs text-gray-500 dark:text-gray-400 font-mono">
                ID: {threat.targetId}
              </p>
            </div>
            <div>
              <label className="text-sm font-medium text-gray-500 dark:text-gray-400">
                Source
              </label>
              {threat.source ? (
                // Check if source is a UUID pattern (agent ID) or an IP address
                /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(threat.source) ? (
                  <div className="mt-1">
                    <span className="text-xs px-2 py-0.5 bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-400 rounded mr-2">
                      Agent ID
                    </span>
                    <span className="text-sm text-gray-900 dark:text-white font-mono">
                      {threat.source.substring(0, 8)}...
                    </span>
                    <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
                      Source IP not captured - this may be from an older event or internal request
                    </p>
                  </div>
                ) : (
                  <p className="mt-1 text-sm text-gray-900 dark:text-white font-mono">
                    {threat.source}
                  </p>
                )
              ) : (
                <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
                  Not available
                </p>
              )}
            </div>
          </div>

          {/* Status and Detection Time */}
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="text-sm font-medium text-gray-500 dark:text-gray-400">
                Status
              </label>
              <p className="mt-1">
                <span
                  className={`inline-flex px-2 py-1 text-xs font-semibold rounded-full uppercase ${getStatusColor(threat.isBlocked)}`}
                >
                  {threat.isBlocked ? "Blocked" : "Active"}
                </span>
              </p>
            </div>
            <div>
              <label className="text-sm font-medium text-gray-500 dark:text-gray-400">
                Detected At
              </label>
              <p className="mt-1 text-sm text-gray-900 dark:text-white">
                {formatDateTime(threat.createdAt)}
              </p>
            </div>
          </div>

          {/* Additional Metadata */}
          {threat.metadata && Object.keys(threat.metadata).length > 0 && (
            <div>
              <label className="text-sm font-medium text-gray-500 dark:text-gray-400 mb-2 block">
                Additional Details
              </label>
              <div className="bg-gray-50 dark:bg-gray-900 rounded-lg p-4 border border-gray-200 dark:border-gray-700">
                <pre className="text-xs text-gray-700 dark:text-gray-300 font-mono overflow-x-auto">
                  {JSON.stringify(threat.metadata, null, 2)}
                </pre>
              </div>
            </div>
          )}

          {/* Recommendation Banner */}
          <div className="p-4 bg-amber-50 dark:bg-amber-900/10 rounded-lg border border-amber-200 dark:border-amber-800">
            <div className="flex items-start gap-3">
              <AlertTriangle className="h-5 w-5 text-amber-600 dark:text-amber-400 flex-shrink-0 mt-0.5" />
              <div className="flex-1">
                <h4 className="text-sm font-medium text-amber-900 dark:text-amber-100 mb-1">
                  Recommended Actions
                </h4>
                <ul className="text-xs text-amber-800 dark:text-amber-200 space-y-1 list-disc list-inside">
                  <li>Review agent activity logs for suspicious patterns</li>
                  <li>Verify agent capabilities match registered scope</li>
                  <li>Check if trust score has decreased recently</li>
                  {!threat.isBlocked && (
                    <li className="font-semibold">
                      Consider blocking this agent if threat persists
                    </li>
                  )}
                  {agent?.createdBySdkTokenId && !revokeSuccess && (
                    <li className="font-semibold text-red-700 dark:text-red-300">
                      If compromised, revoke the SDK token used to create this agent
                    </li>
                  )}
                </ul>
              </div>
            </div>
          </div>
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

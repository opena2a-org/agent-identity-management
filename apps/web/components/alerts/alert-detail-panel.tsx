"use client";

import { useEffect, useState, useCallback } from "react";
import { X, ExternalLink, Copy, Check, Shield, Clock, User, Globe, FileText, AlertTriangle, KeyRound, Ban, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { api, Agent } from "@/lib/api";
import { formatDateTime } from "@/lib/date-utils";
import { toast } from "sonner";
import Link from "next/link";

interface Alert {
  id: string;
  alertType: string;
  severity: "low" | "medium" | "high" | "critical" | "info" | "warning";
  title: string;
  description: string;
  resourceType: string;
  resourceId: string;
  auditId?: string;
  agentName?: string;
  metadata?: Record<string, any>;
  isAcknowledged: boolean;
  acknowledgedBy?: string;
  acknowledgedAt?: string;
  createdAt: string;
}

interface AuditLog {
  id: string;
  organizationId: string;
  userId: string;
  action: string;
  resourceType: string;
  resourceId: string;
  ipAddress: string;
  userAgent: string;
  metadata: Record<string, any>;
  timestamp: string;
}

interface AlertDetailPanelProps {
  alert: Alert | null;
  isOpen: boolean;
  onClose: () => void;
  onAcknowledge: (alertId: string, alertTitle: string) => void;
  onResolve: (alertId: string) => void;
}

const severityColors: Record<string, string> = {
  low: "border-glass-inset-border bg-glass-inset-gray text-ink-body",
  info: "border-transparent bg-brand-soft text-brand-text",
  medium: "border-warning-border bg-warning-fill text-warning-text",
  warning: "border-warning-border bg-warning-fill text-warning-text",
  high: "border-danger-border bg-danger-fill text-danger-text",
  critical: "border-danger-strong bg-danger-fill text-danger-text font-bold",
};

interface VerificationEvent {
  id: string;
  status: string;
  result?: string;
  verificationType: string;
  action?: string;
  resource?: string;
  trustScore: number;
  errorReason?: string;
  durationMs: number;
  initiatorIp?: string;
  createdAt: string;
  allowed?: boolean;
  reason?: string;
  // SDK Execution Status fields
  executed?: boolean;
  strictMode?: boolean;
  executedAt?: string;
  executionError?: string;
}

export function AlertDetailPanel({
  alert,
  isOpen,
  onClose,
  onAcknowledge,
  onResolve,
}: AlertDetailPanelProps) {
  const [auditLog, setAuditLog] = useState<AuditLog | null>(null);
  const [verificationHistory, setVerificationHistory] = useState<VerificationEvent[]>([]);
  const [loadingData, setLoadingData] = useState(false);
  const [copiedField, setCopiedField] = useState<string | null>(null);
  const [showFullMetadata, setShowFullMetadata] = useState(false);
  const [currentTrustScore, setCurrentTrustScore] = useState<number | null>(null);
  // SDK Token revocation state
  const [agentDetails, setAgentDetails] = useState<Agent | null>(null);
  const [revokingToken, setRevokingToken] = useState(false);
  const [showRevokeConfirm, setShowRevokeConfirm] = useState(false);
  const [revokeSuccess, setRevokeSuccess] = useState(false);

  // Fetch audit log, verification history, and current trust score when alert changes
  useEffect(() => {
    if (!alert) {
      setAuditLog(null);
      setVerificationHistory([]);
      setCurrentTrustScore(null);
      setAgentDetails(null);
      setRevokeSuccess(false);
      return;
    }

    setLoadingData(true);

    const fetchData = async () => {
      // Try to fetch the linked audit log (non-fatal if not found)
      if (alert.auditId) {
        try {
          const log = await api.getAuditLogById(alert.auditId);
          setAuditLog(log || null);
        } catch (e) {
          // Audit log may not exist - this is non-fatal
          console.log("Could not fetch audit log:", e);
          setAuditLog(null);
        }
      } else {
        setAuditLog(null);
      }

      // Try to fetch verification history for the agent (non-fatal if not found)
      try {
        const history = await api.getAgentVerificationHistory(alert.resourceId, 5);
        setVerificationHistory(history || []);
      } catch (e) {
        console.log("Could not fetch verification history:", e);
        setVerificationHistory([]);
      }

      // Fetch current trust score from agent trust breakdown
      try {
        const trustBreakdown = await api.getTrustScoreBreakdown(alert.resourceId);
        if (trustBreakdown?.overall !== undefined) {
          setCurrentTrustScore(trustBreakdown.overall);
        }
      } catch (e) {
        // Non-fatal - current trust score just won't be shown
        console.log("Could not fetch current trust score:", e);
      }

      // Fetch agent details to get SDK token info for revocation
      try {
        const agent = await api.getAgent(alert.resourceId);
        setAgentDetails(agent);
      } catch (e) {
        console.log("Could not fetch agent details:", e);
        setAgentDetails(null);
      }

      setLoadingData(false);
    };

    fetchData().catch((e) => {
      console.error("Error in fetchData:", e);
      setLoadingData(false);
    });
  }, [alert?.auditId, alert?.resourceId]);

  // Handle SDK token revocation
  const handleRevokeSDKToken = async () => {
    if (!agentDetails?.createdBySdkTokenId) return;

    setRevokingToken(true);
    try {
      await api.revokeSDKToken(agentDetails.createdBySdkTokenId, `Security alert: ${alert?.alertType}`);
      setRevokeSuccess(true);
      setShowRevokeConfirm(false);
      toast.success("SDK token revoked successfully");
    } catch (error: any) {
      toast.error(error?.message || "Failed to revoke SDK token");
    } finally {
      setRevokingToken(false);
    }
  };

  // Handle keyboard navigation
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (!isOpen) return;

      if (e.key === "Escape") {
        onClose();
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [isOpen, onClose]);

  const copyToClipboard = useCallback((text: string, field: string) => {
    navigator.clipboard.writeText(text).then(() => {
      setCopiedField(field);
      toast.success("Copied to clipboard");
      setTimeout(() => setCopiedField(null), 2000);
    });
  }, []);

  if (!isOpen || !alert) return null;

  const metadata = alert.metadata || {};
  const metadataEntries = Object.entries(metadata);

  return (
    <>
      {/* Backdrop */}
      <div
        className="fixed inset-0 z-40 bg-[rgba(29,29,31,0.45)] backdrop-blur-sm transition-opacity"
        onClick={onClose}
      />

      {/* Panel */}
      <div className="fixed right-0 top-0 h-full w-full max-w-xl bg-glass-chrome border-l border-glass-chrome-border shadow-chrome backdrop-blur-chrome z-50 overflow-y-auto animate-in slide-in-from-right duration-200">
        {/* Header */}
        <div className="sticky top-0 bg-glass-chrome backdrop-blur-chrome border-b border-divider px-6 py-4 flex items-start justify-between">
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2 mb-1">
              <Badge
                variant="outline"
                className={severityColors[alert.severity] || severityColors.medium}
              >
                {alert.severity.toUpperCase()}
              </Badge>
              <Badge variant="outline" className="text-xs">
                {alert.alertType.replace(/_/g, " ")}
              </Badge>
            </div>
            <h2 className="text-lg font-semibold text-ink truncate">{alert.title}</h2>
          </div>
          <Button variant="ghost" size="icon" onClick={onClose} className="ml-2">
            <X className="h-5 w-5" />
          </Button>
        </div>

        {/* Content */}
        <div className="p-6 space-y-6">
          {/* Agent Info */}
          <section className="space-y-3">
            <h3 className="text-sm font-semibold text-ink-tertiary uppercase tracking-wide flex items-center gap-2">
              <User className="h-4 w-4" />
              Agent information
            </h3>
            <div className="bg-glass-inset-gray rounded-inset p-4 space-y-3">
              <div className="flex items-center justify-between">
                <span className="text-sm text-ink-secondary">Agent name</span>
                <span className="font-medium text-ink">{alert.agentName || "Unknown"}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-sm text-ink-secondary">Agent ID</span>
                <div className="flex items-center gap-2">
                  <code className="text-xs bg-glass-inset-gray text-ink px-2 py-1 rounded font-mono">
                    {alert.resourceId}
                  </code>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-6 w-6"
                    onClick={() => copyToClipboard(alert.resourceId, "agentId")}
                  >
                    {copiedField === "agentId" ? (
                      <Check className="h-3 w-3 text-success-text" />
                    ) : (
                      <Copy className="h-3 w-3" />
                    )}
                  </Button>
                </div>
              </div>
              {/* Show current trust score (freshly fetched) */}
              {currentTrustScore !== null && (
                <div className="flex items-center justify-between">
                  <span className="text-sm text-ink-secondary">Current trust score</span>
                  <span className="font-medium text-ink">
                    {(currentTrustScore * 100).toFixed(1)}%
                  </span>
                </div>
              )}
              {/* Show historical trust score at alert time if different from current */}
              {metadata.trustScore !== undefined && currentTrustScore !== null &&
               Math.abs((metadata.trustScore * 100) - (currentTrustScore * 100)) > 1 && (
                <div className="flex items-center justify-between">
                  <span className="text-sm text-ink-secondary">Trust score at alert time</span>
                  <span className="font-medium text-ink-tertiary">
                    {(metadata.trustScore * 100).toFixed(0)}%
                  </span>
                </div>
              )}
              {/* Fallback: show historical score if current couldn't be fetched */}
              {metadata.trustScore !== undefined && currentTrustScore === null && (
                <div className="flex items-center justify-between">
                  <span className="text-sm text-ink-secondary">Trust score at alert time</span>
                  <span className="font-medium text-ink">
                    {(metadata.trustScore * 100).toFixed(0)}%
                  </span>
                </div>
              )}

              {/* Creator Info */}
              {agentDetails?.createdByName && (
                <div className="flex items-center justify-between">
                  <span className="text-sm text-ink-secondary">Created by</span>
                  <span className="font-medium text-ink">
                    {agentDetails.createdByName}
                    {agentDetails.createdByEmail && (
                      <span className="text-ink-tertiary text-xs ml-1">({agentDetails.createdByEmail})</span>
                    )}
                  </span>
                </div>
              )}

              {/* SDK Token Info */}
              {agentDetails?.createdBySdkTokenId && (
                <div className="flex items-center justify-between">
                  <span className="text-sm text-ink-secondary">SDK token</span>
                  <div className="flex items-center gap-2">
                    <Link
                      href={`/dashboard/sdk-tokens?highlight=${agentDetails.createdBySdkTokenId}`}
                      className="text-xs text-brand-text hover:underline flex items-center gap-1"
                    >
                      <KeyRound className="h-3 w-3" />
                      View token
                    </Link>
                    {!revokeSuccess ? (
                      <Button
                        variant="ghost"
                        size="sm"
                        className="h-6 text-xs text-danger-text hover:bg-danger-fill px-2"
                        onClick={() => setShowRevokeConfirm(true)}
                      >
                        <Ban className="h-3 w-3 mr-1" />
                        Revoke
                      </Button>
                    ) : (
                      <Badge variant="outline" className="text-xs text-success-text border-success-border">
                        <Check className="h-3 w-3 mr-1" />
                        Revoked
                      </Badge>
                    )}
                  </div>
                </div>
              )}
            </div>
          </section>

          {/* SDK Token Revoke Confirmation */}
          {showRevokeConfirm && agentDetails?.createdBySdkTokenId && (
            <section className="space-y-3">
              <div className="glass-alert p-4">
                <div className="flex items-start gap-3">
                  <Ban className="h-5 w-5 text-danger-text flex-shrink-0 mt-0.5" />
                  <div className="flex-1">
                    <h4 className="text-sm font-medium text-danger-text mb-1">
                      Confirm SDK token revocation
                    </h4>
                    <p className="text-xs text-danger-text mb-3">
                      This will immediately revoke the SDK token used to create this agent.
                      Any applications using this token will no longer be able to authenticate.
                    </p>
                    <div className="flex gap-2">
                      <Button
                        size="sm"
                        variant="destructive"
                        onClick={handleRevokeSDKToken}
                        disabled={revokingToken}
                        className="text-xs"
                      >
                        {revokingToken ? (
                          <>
                            <Loader2 className="h-3 w-3 mr-1 animate-spin" />
                            Revoking...
                          </>
                        ) : (
                          <>
                            <Ban className="h-3 w-3 mr-1" />
                            Yes, revoke token
                          </>
                        )}
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => setShowRevokeConfirm(false)}
                        className="text-xs"
                      >
                        Cancel
                      </Button>
                    </div>
                  </div>
                </div>
              </div>
            </section>
          )}

          {/* Alert Details */}
          <section className="space-y-3">
            <h3 className="text-sm font-semibold text-ink-tertiary uppercase tracking-wide flex items-center gap-2">
              <AlertTriangle className="h-4 w-4" />
              Alert details
            </h3>
            <div className="bg-glass-inset-gray rounded-inset p-4 space-y-3">
              <div>
                <span className="text-sm text-ink-secondary">Description</span>
                <p className="mt-1 text-sm text-ink-body">{alert.description}</p>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-sm text-ink-secondary">Created</span>
                <span className="text-sm text-ink">{formatDateTime(alert.createdAt)}</span>
              </div>
              {alert.isAcknowledged && alert.acknowledgedAt && (
                <div className="flex items-center justify-between">
                  <span className="text-sm text-ink-secondary">Acknowledged</span>
                  <span className="text-sm text-success-text">
                    {formatDateTime(alert.acknowledgedAt)}
                  </span>
                </div>
              )}
              {metadata.policyName && (
                <div className="flex items-center justify-between">
                  <span className="text-sm text-ink-secondary">Policy</span>
                  <Badge variant="outline">{metadata.policyName}</Badge>
                </div>
              )}
              {metadata.enforcement && (
                <div className="flex items-center justify-between">
                  <span className="text-sm text-ink-secondary">Enforcement</span>
                  <Badge
                    variant={metadata.isBlocked ? "destructive" : "secondary"}
                  >
                    {metadata.enforcement}
                  </Badge>
                </div>
              )}
            </div>
          </section>

          {/* Verification History */}
          <section className="space-y-3">
            <h3 className="text-sm font-semibold text-ink-tertiary uppercase tracking-wide flex items-center gap-2">
              <Clock className="h-4 w-4" />
              Verification history
            </h3>

            {loadingData ? (
              <div className="bg-glass-inset-gray rounded-inset p-4 space-y-3">
                <Skeleton className="h-4 w-full" />
                <Skeleton className="h-4 w-3/4" />
                <Skeleton className="h-4 w-1/2" />
              </div>
            ) : verificationHistory.length > 0 ? (
              <div className="bg-glass-inset-gray rounded-inset p-4 space-y-3">
                <div className="flex items-center justify-between mb-2">
                  <span className="text-sm text-ink-secondary">Recent verifications</span>
                  <Badge variant="outline" className="text-xs">
                    {verificationHistory.length} events
                  </Badge>
                </div>
                <div className="space-y-3">
                  {verificationHistory.map((event) => {
                    const isDenied = event.allowed === false || event.result === "denied" || event.status === "failed";
                    const isSuccess = event.allowed === true || event.result === "verified" || event.status === "success";

                    return (
                      <div
                        key={event.id}
                        className={`p-3 rounded-inset border ${
                          isDenied
                            ? "bg-danger-fill border-danger-border"
                            : isSuccess
                            ? "bg-success-fill border-success-border"
                            : "bg-glass-inset-gray border-stroke"
                        }`}
                      >
                        <div className="flex items-center justify-between mb-1">
                          <div className="flex items-center gap-2">
                            <Badge
                              variant={isDenied ? "destructive" : isSuccess ? "default" : "secondary"}
                              className="text-xs"
                            >
                              {isDenied ? "DENIED" : isSuccess ? "ALLOWED" : event.status.toUpperCase()}
                            </Badge>
                            <span className="text-xs font-medium text-ink-secondary">
                              {event.verificationType}
                            </span>
                          </div>
                          <span className="text-xs text-ink-tertiary">
                            {formatDateTime(event.createdAt)}
                          </span>
                        </div>

                        {(event.action || event.resource) && (
                          <div className="mt-2 space-y-1">
                            {event.action && (
                              <p className="text-sm text-ink-body">
                                <span className="text-ink-tertiary">Capability:</span>{" "}
                                <code className="text-xs bg-glass text-ink px-1.5 py-0.5 rounded border border-stroke">
                                  {event.action}
                                </code>
                              </p>
                            )}
                            {event.resource && (
                              <p className="text-sm text-ink-body">
                                <span className="text-ink-tertiary">Resource:</span>{" "}
                                <code className="text-xs bg-glass text-ink px-1.5 py-0.5 rounded border border-stroke">
                                  {event.resource}
                                </code>
                              </p>
                            )}
                          </div>
                        )}

                        {isDenied && event.reason && (
                          <p className="text-sm text-danger-text mt-2">
                            <span className="font-medium">Denied:</span> {event.reason}
                          </p>
                        )}

                        {/*
                          Execution status: what the AGENT SAID happened after verification.

                          These fields (executed, strictMode, executedAt, executionError)
                          have exactly one writer in the backend — the agent's own SDK, via
                          POST /sdk-api/verifications/:id/execution-status. AIM holds no
                          independent record of whether the action ran, so this block must
                          be attributed to its source and must never assert that AIM
                          enforced anything.

                          It previously rendered "Action BLOCKED (strict mode enforced)",
                          which read as a claim about our control while being derived
                          entirely from the self-report of the party being audited. strictMode
                          is now server-derived from the organization's enforcement mode, so
                          the mode shown here is ours; `executed` is still the agent's word.
                          Labelling these columns by provenance belongs to the
                          execution-outcome follow-up.
                        */}
                        {event.executed !== undefined && (
                          <div className={`mt-2 p-2 rounded-inset-sm text-xs ${
                            event.executed
                              ? isDenied
                                ? "bg-warning-fill border border-warning-border text-warning-text"
                                : "bg-success-fill border border-success-border text-success-text"
                              : "bg-glass-inset-gray border border-stroke text-ink-body"
                          }`}>
                            <span className="font-medium">
                              {event.executed ? (
                                isDenied ? (
                                  "Agent reported: action executed despite denial"
                                ) : (
                                  "Agent reported: action executed"
                                )
                              ) : (
                                "Agent reported: action not executed"
                              )}
                            </span>
                            <p className="mt-1 text-ink-secondary">
                              Self-reported by the agent. AIM does not independently observe
                              execution, so this is not a record of enforcement.
                              {event.strictMode !== undefined && (
                                <> Organization enforcement mode at report time:{" "}
                                  {event.strictMode ? "strict" : "monitoring"}.</>
                              )}
                            </p>
                            {event.executionError && (
                              <p className="mt-1 text-danger-text">Reported error: {event.executionError}</p>
                            )}
                          </div>
                        )}

                        <div className="flex items-center gap-4 mt-2 text-xs text-ink-tertiary">
                          <span>Trust: {(event.trustScore * 100).toFixed(0)}%</span>
                          {event.durationMs > 0 && <span>{event.durationMs}ms</span>}
                          {event.initiatorIp && <span>IP: {event.initiatorIp}</span>}
                        </div>
                      </div>
                    );
                  })}
                </div>
              </div>
            ) : (
              <div className="bg-glass-inset-gray rounded-inset p-4">
                <p className="text-sm text-ink-secondary">
                  No verification events recorded for this agent.
                </p>
              </div>
            )}
          </section>

          {/* Alert Metadata - Only show if there's context beyond what's already displayed */}
          {(() => {
            const contextEntries = metadataEntries.filter(
              ([key]) => !["trustScore", "policyName", "enforcement", "isBlocked", "policyType"].includes(key)
            );
            if (contextEntries.length === 0) return null;
            return (
              <section className="space-y-3">
                <h3 className="text-sm font-semibold text-ink-tertiary uppercase tracking-wide flex items-center gap-2">
                  <Shield className="h-4 w-4" />
                  Context
                </h3>
                <div className="bg-glass-inset-gray rounded-inset p-4 space-y-2">
                  {contextEntries.map(([key, value]) => (
                    <div key={key} className="flex items-center justify-between">
                      <span className="text-sm text-ink-secondary capitalize">
                        {key.replace(/([A-Z])/g, " $1").trim()}
                      </span>
                      <span className="text-sm font-medium text-ink">
                        {typeof value === "object" ? JSON.stringify(value) : String(value)}
                      </span>
                    </div>
                  ))}
                </div>
              </section>
            );
          })()}
        </div>

        {/* Footer Actions */}
        <div className="sticky bottom-0 bg-glass-chrome backdrop-blur-chrome border-t border-divider px-6 py-4 flex gap-3">
          {!alert.isAcknowledged ? (
            <Button
              className="flex-1"
              onClick={() => onAcknowledge(alert.id, alert.title)}
            >
              Acknowledge
            </Button>
          ) : (
            <Button
              variant="outline"
              className="flex-1 border-success-border text-success-text hover:bg-success-fill"
              onClick={() => onResolve(alert.id)}
            >
              Resolve
            </Button>
          )}
          <Link href={`/dashboard/agents/${alert.resourceId}`} className="flex-1">
            <Button variant="outline" className="w-full">
              <ExternalLink className="h-4 w-4 mr-2" />
              View agent
            </Button>
          </Link>
        </div>
      </div>
    </>
  );
}

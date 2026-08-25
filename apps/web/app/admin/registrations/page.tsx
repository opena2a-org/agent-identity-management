"use client";

import { useEffect, useState } from "react";
import { RegistrationRequestCard } from "@/components/admin/registration-request-card";
import { api } from "@/lib/api";
import { UserPlus, RefreshCw, AlertCircle } from "lucide-react";

interface RegistrationRequest {
  id: string;
  email: string;
  firstName: string;
  lastName: string;
  oauthProvider?: "google" | "microsoft" | "okta" | "local";
  oauthUserId?: string;
  status: "pending" | "approved" | "rejected";
  requestedAt: string;
  reviewedAt?: string;
  reviewedBy?: string;
  rejectionReason?: string;
  profilePictureUrl?: string;
  oauthEmailVerified: boolean;
  metadata?: {
    signupProfile?: {
      role?: string;
      primaryUseCase?: string;
      referralSource?: string;
    };
  };
}

export default function RegistrationsPage() {
  const [requests, setRequests] = useState<RegistrationRequest[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [total, setTotal] = useState(0);
  const [filter, setFilter] = useState<
    "all" | "pending" | "approved" | "rejected"
  >("pending");

  const fetchRequests = async () => {
    setLoading(true);
    setError(null);

    try {
      const response = await api.listPendingRegistrations(100, 0);
      setRequests(response.requests || []);
      setTotal(response.total || 0);
    } catch (err: any) {
      setError(err.message || "Failed to load registration requests");
      setRequests([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchRequests();
  }, []);

  const filteredRequests =
    filter === "all"
      ? requests
      : (requests || []).filter((req) => req.status === filter);

  const pendingCount = (requests || []).filter(
    (req) => req.status === "pending"
  ).length;
  const approvedCount = (requests || []).filter(
    (req) => req.status === "approved"
  ).length;
  const rejectedCount = (requests || []).filter(
    (req) => req.status === "rejected"
  ).length;

  const tabs = (["pending", "approved", "rejected", "all"] as const).map((status) => ({
    status,
    label: status === "all" ? "All" : status.charAt(0).toUpperCase() + status.slice(1),
    count: status === "pending" ? pendingCount : status === "approved" ? approvedCount : status === "rejected" ? rejectedCount : total,
  }));

  return (
    <main>
      <div className="mx-auto max-w-5xl">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-3">
            <span className="inline-flex h-11 w-11 items-center justify-center rounded-inset-sm bg-brand-soft text-brand-text">
              <UserPlus className="h-5 w-5" aria-hidden="true" />
            </span>
            <div>
              <h1 className="text-headline">Registration requests</h1>
              <p className="text-sm text-ink-secondary">People waiting for an administrator to let them in.</p>
            </div>
          </div>
          <button
            type="button"
            onClick={fetchRequests}
            disabled={loading}
            className="inline-flex h-9 items-center gap-2 rounded-pill border border-stroke bg-glass px-4 text-xs font-bold text-ink backdrop-blur-card hover:bg-glass-inset disabled:opacity-60"
          >
            <RefreshCw className={`h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} aria-hidden="true" />
            Refresh
          </button>
        </div>

        <div className="mt-6 grid grid-cols-2 gap-3 md:grid-cols-4">
          {[
            { label: "Total", value: total, tone: "text-ink" },
            { label: "Pending review", value: pendingCount, tone: pendingCount > 0 ? "text-warning-text" : "text-ink" },
            { label: "Approved", value: approvedCount, tone: "text-success-text" },
            { label: "Rejected", value: rejectedCount, tone: rejectedCount > 0 ? "text-danger-text" : "text-ink" },
          ].map((t) => (
            <div key={t.label} className="glass p-4">
              <p className="text-xs font-semibold text-ink-secondary">{t.label}</p>
              <p className={`mt-1.5 text-[28px] font-bold leading-none tracking-[-0.03em] ${t.tone}`}>{t.value}</p>
            </div>
          ))}
        </div>

        <div className="glass-segment mt-5" role="tablist" aria-label="Filter requests">
          {tabs.map((t) => (
            <button
              key={t.status}
              type="button"
              role="tab"
              aria-selected={filter === t.status}
              data-active={filter === t.status}
              onClick={() => setFilter(t.status)}
              className="glass-segment-item"
            >
              {t.label} <span className="opacity-70">({t.count})</span>
            </button>
          ))}
        </div>

        {error && (
          <div className="mt-5 flex items-center gap-3 rounded-card border border-danger-border bg-danger-fill p-4" role="alert">
            <AlertCircle className="h-4 w-4 flex-shrink-0 text-danger-text" aria-hidden="true" />
            <div>
              <p className="text-sm font-bold text-danger-text">Requests could not be loaded</p>
              <p className="text-xs text-ink-body">{error}</p>
            </div>
          </div>
        )}

        {loading && (
          <div className="flex items-center justify-center gap-3 py-12 text-sm text-ink-secondary" aria-busy="true">
            <span className="h-5 w-5 animate-spin rounded-full border-2 border-track border-t-brand" aria-hidden="true" />
            Loading registration requests
          </div>
        )}

        {!loading && !error && (
          <div className="mt-5">
            {filteredRequests.length === 0 ? (
              <div className="glass p-10 text-center">
                <UserPlus className="mx-auto mb-3 h-8 w-8 text-ink-tertiary" aria-hidden="true" />
                <h3 className="text-[15px] font-bold text-ink">No {filter !== "all" ? filter : ""} registration requests</h3>
                <p className="mt-1 text-xs text-ink-secondary">
                  {filter === "pending" ? "Nobody is waiting for approval right now." : `There are no requests with status ${filter}.`}
                </p>
              </div>
            ) : (
              <div className="space-y-3">
                {filteredRequests.map((request) => (
                  <RegistrationRequestCard key={request.id} request={request} onApproved={fetchRequests} onRejected={fetchRequests} />
                ))}
              </div>
            )}
          </div>
        )}
      </div>
    </main>
  );
}

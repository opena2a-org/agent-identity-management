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
  oauthProvider: "google" | "microsoft" | "okta";
  oauthUserId: string;
  status: "pending" | "approved" | "rejected";
  requestedAt: string;
  reviewedAt?: string;
  reviewedBy?: string;
  rejectionReason?: string;
  profilePictureUrl?: string;
  oauthEmailVerified: boolean;
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

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Header */}
      <div className="bg-white border-b border-gray-200">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="w-12 h-12 bg-gradient-to-br from-blue-600 to-purple-600 rounded-lg flex items-center justify-center">
                <UserPlus className="w-6 h-6 text-white" />
              </div>
              <div>
                <h1 className="text-2xl font-bold text-gray-900">
                  Registration Requests
                </h1>
                <p className="text-sm text-gray-600">
                  Review and manage user registration requests
                </p>
              </div>
            </div>

            <button
              onClick={fetchRequests}
              disabled={loading}
              className="flex items-center gap-2 bg-blue-600 hover:bg-blue-700 text-white font-medium py-2 px-4 rounded-lg transition-colors disabled:opacity-50"
            >
              <RefreshCw
                className={`w-4 h-4 ${loading ? "animate-spin" : ""}`}
              />
              <span>Refresh</span>
            </button>
          </div>
        </div>
      </div>

      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        {/* Stats Cards */}
        <div className="grid grid-cols-1 md:grid-cols-4 gap-6 mb-8">
          <div className="bg-white border border-gray-200 rounded-lg p-6">
            <div className="text-sm text-gray-600 mb-1">Total Requests</div>
            <div className="text-3xl font-bold text-gray-900">{total}</div>
          </div>

          <div className="bg-white border border-amber-200 rounded-lg p-6">
            <div className="text-sm text-amber-700 mb-1">Pending Review</div>
            <div className="text-3xl font-bold text-amber-600">
              {pendingCount}
            </div>
          </div>

          <div className="bg-white border border-green-200 rounded-lg p-6">
            <div className="text-sm text-green-700 mb-1">Approved</div>
            <div className="text-3xl font-bold text-green-600">
              {approvedCount}
            </div>
          </div>

          <div className="bg-white border border-red-200 rounded-lg p-6">
            <div className="text-sm text-red-700 mb-1">Rejected</div>
            <div className="text-3xl font-bold text-red-600">
              {rejectedCount}
            </div>
          </div>
        </div>

        {/* Filter Tabs */}
        <div className="bg-white border border-gray-200 rounded-lg p-1 mb-6 inline-flex">
          {(["all", "pending", "approved", "rejected"] as const).map(
            (status) => (
              <button
                key={status}
                onClick={() => setFilter(status)}
                className={`px-4 py-2 rounded-md font-medium text-sm transition-colors ${
                  filter === status
                    ? "bg-blue-600 text-white"
                    : "text-gray-600 hover:text-gray-900 hover:bg-gray-50"
                }`}
              >
                {status.charAt(0).toUpperCase() + status.slice(1)}
                {status !== "all" && (
                  <span className="ml-2 text-xs opacity-75">
                    (
                    {status === "pending"
                      ? pendingCount
                      : status === "approved"
                        ? approvedCount
                        : rejectedCount}
                    )
                  </span>
                )}
              </button>
            )
          )}
        </div>

        {/* Error State */}
        {error && (
          <div className="bg-red-50 border border-red-200 rounded-lg p-4 mb-6 flex items-center gap-3">
            <AlertCircle className="w-5 h-5 text-red-600 flex-shrink-0" />
            <div>
              <p className="text-sm font-medium text-red-900">
                Error loading requests
              </p>
              <p className="text-sm text-red-700">{error}</p>
            </div>
          </div>
        )}

        {/* Loading State */}
        {loading && (
          <div className="flex items-center justify-center py-12">
            <div className="flex items-center gap-3 text-gray-600">
              <div className="w-6 h-6 border-4 border-blue-600 border-t-transparent rounded-full animate-spin" />
              <span>Loading registration requests...</span>
            </div>
          </div>
        )}

        {/* Requests List */}
        {!loading && !error && (
          <>
            {filteredRequests.length === 0 ? (
              <div className="bg-white border border-gray-200 rounded-lg p-12 text-center">
                <UserPlus className="w-12 h-12 text-gray-400 mx-auto mb-4" />
                <h3 className="text-lg font-medium text-gray-900 mb-2">
                  No {filter !== "all" && filter} registration requests
                </h3>
                <p className="text-gray-600">
                  {filter === "pending"
                    ? "There are no pending registration requests at the moment."
                    : `No registration requests with status: ${filter}`}
                </p>
              </div>
            ) : (
              <div className="space-y-4">
                {filteredRequests.map((request) => (
                  <RegistrationRequestCard
                    key={request.id}
                    request={request}
                    onApproved={fetchRequests}
                    onRejected={fetchRequests}
                  />
                ))}
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}

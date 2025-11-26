"use client";

import {
  X,
  AlertTriangle,
  ExternalLink,
  Shield,
  Activity,
  FileText,
  User,
} from "lucide-react";
import Link from "next/link";
import { formatDateTime } from "@/lib/date-utils";

interface SecurityThreat {
  id: string;
  targetId: string;
  targetName?: string;
  threatType: string;
  severity: "low" | "medium" | "high" | "critical";
  description: string;
  isBlocked: boolean;
  createdAt: string;
  sourceIp?: string;
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
          <div className="flex items-center gap-2 p-4 bg-blue-50 dark:bg-blue-900/10 rounded-lg border border-blue-200 dark:border-blue-800">
            <Shield className="h-5 w-5 text-blue-600 dark:text-blue-400" />
            <span className="text-sm font-medium text-blue-900 dark:text-blue-100">
              Quick Actions:
            </span>
            <div className="flex items-center gap-2 ml-auto">
              <Link
                href={`/dashboard/agents?search=${threat.targetId}`}
                className="inline-flex items-center gap-1 px-3 py-1 text-xs font-medium text-blue-700 dark:text-blue-300 bg-white dark:bg-gray-800 border border-blue-300 dark:border-blue-700 rounded-lg hover:bg-blue-50 dark:hover:bg-blue-900/20 transition-colors"
              >
                <User className="h-3 w-3" />
                View Agent
                <ExternalLink className="h-3 w-3" />
              </Link>
              <Link
                href={`/dashboard/monitoring?agent=${threat.targetId}`}
                className="inline-flex items-center gap-1 px-3 py-1 text-xs font-medium text-blue-700 dark:text-blue-300 bg-white dark:bg-gray-800 border border-blue-300 dark:border-blue-700 rounded-lg hover:bg-blue-50 dark:hover:bg-blue-900/20 transition-colors"
              >
                <Activity className="h-3 w-3" />
                View Activity
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
            </div>
          </div>

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
                Source IP
              </label>
              <p className="mt-1 text-sm text-gray-900 dark:text-white font-mono">
                {threat.sourceIp || "N/A"}
              </p>
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

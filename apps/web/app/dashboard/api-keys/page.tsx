"use client";

import { useState, useEffect } from "react";
import {
  Key,
  Clock,
  Copy,
  Check,
  Trash2,
  Plus,
  Loader2,
  AlertCircle,
  Search,
  Filter,
  Ban,
} from "lucide-react";
import { api, APIKey, Agent } from "@/lib/api";
import { CreateAPIKeyModal } from "@/components/modals/create-api-key-modal";
import { ConfirmDialog } from "@/components/modals/confirm-dialog";
import { getAgentPermissions, UserRole } from "@/lib/permissions";

interface APIKeyWithAgent extends APIKey {
  agent_name?: string;
}

function StatCard({ stat }: { stat: any }) {
  return (
    <div className="bg-white dark:bg-gray-800 p-6 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
      <div className="flex items-center">
        <div className="flex-shrink-0">
          <stat.icon className="h-6 w-6 text-gray-400" />
        </div>
        <div className="ml-5 w-0 flex-1">
          <dl>
            <dt className="text-sm font-medium text-gray-500 dark:text-gray-400 truncate">
              {stat.name}
            </dt>
            <dd className="flex items-baseline">
              <div className="text-2xl font-semibold text-gray-900 dark:text-gray-100">
                {stat.value}
              </div>
              {stat.change && (
                <div
                  className={`ml-2 flex items-baseline text-sm font-semibold ${
                    stat.changeType === "positive"
                      ? "text-green-600"
                      : "text-red-600"
                  }`}
                >
                  {stat.change}
                </div>
              )}
            </dd>
          </dl>
        </div>
      </div>
    </div>
  );
}

function APIKeysPageSkeleton() {
  return (
    <div className="flex items-center justify-center min-h-[400px]">
      <div className="flex flex-col items-center gap-4">
        <Loader2 className="h-12 w-12 text-blue-500 animate-spin" />
        <p className="text-sm text-gray-500 dark:text-gray-400">
          Loading API keys...
        </p>
      </div>
    </div>
  );
}

export default function APIKeysPage() {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [apiKeys, setApiKeys] = useState<APIKeyWithAgent[]>([]);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [searchTerm, setSearchTerm] = useState("");
  const [statusFilter, setStatusFilter] = useState<string>("all");
  const [copiedId, setCopiedId] = useState<string | null>(null);
  const [userRole, setUserRole] = useState<UserRole>("viewer");

  // Modal states
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [showDisableConfirm, setShowDisableConfirm] = useState(false);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
  const [selectedKey, setSelectedKey] = useState<APIKeyWithAgent | null>(null);

  // Extract user role from JWT token
  useEffect(() => {
    const token = api.getToken();
    if (token) {
      try {
        const payload = JSON.parse(atob(token.split(".")[1]));
        setUserRole((payload.role as UserRole) || "viewer");
      } catch (e) {
        console.error("Failed to decode JWT token:", e);
        setUserRole("viewer");
      }
    }
  }, []);

  // Get role-based permissions
  const permissions = getAgentPermissions(userRole);

  useEffect(() => {
    fetchData();
  }, []);

  const fetchData = async () => {
    try {
      setLoading(true);
      setError(null);

      const [keysData, agentsData] = await Promise.all([
        api.listAPIKeys(),
        api.listAgents(),
      ]);

      // Map agent names to keys
      const keys = keysData?.api_keys || [];
      const agents = agentsData?.agents || [];

      const keysWithAgents = keys.map((key) => ({
        ...key,
        // Use backend-provided agent_name if available, otherwise look up from agents list
        agent_name:
          key.agent_name || agents.find((a) => a.id === key.agent_id)?.name,
      }));

      setApiKeys(keysWithAgents);
      setAgents(agents);
    } catch (err) {
      console.error("Failed to fetch data:", err);
      setError(
        err instanceof Error ? err.message : "An unknown error occurred"
      );
    } finally {
      setLoading(false);
    }
  };

  // Calculate stats
  const stats = {
    total: apiKeys.length,
    active: apiKeys.filter(
      (k) =>
        k.is_active && (!k.expires_at || new Date(k.expires_at) > new Date())
    ).length,
    disabled: apiKeys.filter(
      (k) =>
        !k.is_active && (!k.expires_at || new Date(k.expires_at) > new Date())
    ).length,
    expired: apiKeys.filter(
      (k) => k.expires_at && new Date(k.expires_at) < new Date()
    ).length,
    neverUsed: apiKeys.filter((k) => !k.last_used_at).length,
  };

  const statCards = [
    {
      name: "Total Keys",
      value: stats.total.toLocaleString(),
      icon: Key,
    },
    {
      name: "Active Keys",
      value: stats.active.toLocaleString(),
      change: "+2",
      changeType: "positive",
      icon: Check,
    },
    {
      name: "Expired",
      value: stats.expired.toLocaleString(),
      icon: Clock,
    },
    {
      name: "Never Used",
      value: stats.neverUsed.toLocaleString(),
      icon: AlertCircle,
    },
  ];

  // Filter keys
  const filteredKeys = apiKeys.filter((key) => {
    const matchesSearch =
      key.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
      key.prefix.toLowerCase().includes(searchTerm.toLowerCase()) ||
      key.agent_name?.toLowerCase().includes(searchTerm.toLowerCase());

    let matchesStatus: boolean = true;
    if (statusFilter === "active") {
      matchesStatus =
        key.is_active &&
        (!key.expires_at || new Date(key.expires_at) > new Date());
    } else if (statusFilter === "disabled") {
      matchesStatus =
        !key.is_active &&
        (!key.expires_at || new Date(key.expires_at) > new Date());
    } else if (statusFilter === "expired") {
      matchesStatus = key.expires_at
        ? new Date(key.expires_at) < new Date()
        : false;
    } else if (statusFilter === "never-used") {
      matchesStatus = !key.last_used_at;
    }

    return matchesSearch && matchesStatus;
  });

  const formatDate = (dateString?: string) => {
    if (!dateString) return "Never";
    const date = new Date(dateString);
    return date.toLocaleDateString("en-US", {
      month: "short",
      day: "numeric",
      year: "numeric",
    });
  };

  const copyToClipboard = async (text: string, id: string) => {
    await navigator.clipboard.writeText(text);
    setCopiedId(id);
    setTimeout(() => setCopiedId(null), 2000);
  };

  const handleDisableKey = (key: APIKeyWithAgent) => {
    setSelectedKey(key);
    setShowDisableConfirm(true);
  };

  const confirmDisable = async () => {
    if (!selectedKey) return;

    try {
      await api.disableAPIKey(selectedKey.id);
      // Update the key's is_active status in the local state
      setApiKeys(
        apiKeys.map((k) =>
          k.id === selectedKey.id ? { ...k, is_active: false } : k
        )
      );
    } catch (err) {
      console.error("Failed to disable API key:", err);
      setError(
        err instanceof Error ? err.message : "Failed to disable API key"
      );
    } finally {
      setShowDisableConfirm(false);
      setSelectedKey(null);
    }
  };

  const handleDeleteKey = (key: APIKeyWithAgent) => {
    setSelectedKey(key);
    setShowDeleteConfirm(true);
  };

  const confirmDelete = async () => {
    if (!selectedKey) return;

    try {
      await api.deleteAPIKey(selectedKey.id);
      // Remove the key from the local state
      setApiKeys(apiKeys.filter((k) => k.id !== selectedKey.id));
    } catch (err) {
      console.error("Failed to delete API key:", err);
      alert(
        `Failed to delete API key: ${err instanceof Error ? err.message : "Unknown error"}`
      );
    } finally {
      setShowDeleteConfirm(false);
      setSelectedKey(null);
    }
  };

  const handleKeyCreated = () => {
    fetchData();
    setShowCreateModal(false);
  };

  const isExpired = (expiresAt?: string) => {
    if (!expiresAt) return false;
    return new Date(expiresAt) < new Date();
  };

  if (loading) {
    return <APIKeysPageSkeleton />;
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">
            API Keys
          </h1>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            Manage API keys for agent authentication and authorization.
          </p>
          {error && (
            <div className="mt-2 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg">
              <p className="text-sm text-red-800 dark:text-red-300">
                ⚠️ API connection failed: {error}
              </p>
            </div>
          )}
        </div>
        {permissions.canCreateAPIKey && (
          <button
            onClick={() => setShowCreateModal(true)}
            className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
          >
            <Plus className="h-4 w-4" />
            Create API Key
          </button>
        )}
      </div>

      {/* Stats */}
      <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4">
        {statCards.map((stat) => (
          <StatCard key={stat.name} stat={stat} />
        ))}
      </div>

      {/* Filters */}
      <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm p-4">
        <div className="flex flex-col sm:flex-row gap-4">
          <div className="flex-1 relative">
            <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-5 w-5 text-gray-400" />
            <input
              type="text"
              placeholder="Search by name, prefix, or agent..."
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              className="w-full pl-10 pr-4 py-2 bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100"
            />
          </div>
          <div className="relative">
            <Filter className="absolute left-3 top-1/2 transform -translate-y-1/2 h-5 w-5 text-gray-400" />
            <select
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value)}
              className="pl-10 pr-8 py-2 bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100"
            >
              <option value="all">All Status</option>
              <option value="active">Active</option>
              <option value="disabled">Disabled</option>
              <option value="expired">Expired</option>
              <option value="never-used">Never Used</option>
            </select>
          </div>
        </div>
      </div>

      {/* API Keys Table */}
      <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
            <thead className="bg-gray-50 dark:bg-gray-800">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Name
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Key Prefix
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Agent
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Last Used
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Expires
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Status
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody className="bg-white dark:bg-gray-900 divide-y divide-gray-200 dark:divide-gray-700">
              {filteredKeys.map((key) => (
                <tr
                  key={key.id}
                  className="hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors"
                >
                  <td className="px-6 py-4 whitespace-nowrap">
                    <div className="text-sm font-medium text-gray-900 dark:text-gray-100">
                      {key.name}
                    </div>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <div className="flex items-center gap-2">
                      <code className="text-sm text-gray-600 dark:text-gray-400 font-mono">
                        {key.prefix}
                      </code>
                      <button
                        onClick={() => copyToClipboard(key.prefix, key.id)}
                        className="p-1 text-gray-400 hover:text-blue-600 dark:hover:text-blue-400 transition-colors"
                        title="Copy prefix"
                      >
                        {copiedId === key.id ? (
                          <Check className="h-4 w-4 text-green-600" />
                        ) : (
                          <Copy className="h-4 w-4" />
                        )}
                      </button>
                    </div>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <div className="text-sm text-gray-900 dark:text-gray-100">
                      {key.agent_name || "Unknown"}
                    </div>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <div className="text-sm text-gray-500 dark:text-gray-400">
                      {formatDate(key.last_used_at)}
                    </div>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <div
                      className={`text-sm ${isExpired(key.expires_at) ? "text-red-600 dark:text-red-400" : "text-gray-500 dark:text-gray-400"}`}
                    >
                      {formatDate(key.expires_at)}
                    </div>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <span
                      className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                        !key.is_active
                          ? "bg-gray-100 dark:bg-gray-900/30 text-gray-800 dark:text-gray-300"
                          : isExpired(key.expires_at)
                            ? "bg-red-100 dark:bg-red-900/30 text-red-800 dark:text-red-300"
                            : "bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-300"
                      }`}
                    >
                      {!key.is_active
                        ? "Disabled"
                        : isExpired(key.expires_at)
                          ? "Expired"
                          : "Active"}
                    </span>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <div className="flex items-center gap-2">
                      {key.is_active && !isExpired(key.expires_at) ? (
                        <button
                          onClick={() => handleDisableKey(key)}
                          className="p-1 text-gray-400 hover:text-orange-600 dark:hover:text-orange-400 transition-colors"
                          title="Disable key"
                        >
                          <Ban className="h-4 w-4" />
                        </button>
                      ) : !key.is_active && permissions.canDeleteAPIKey ? (
                        <button
                          onClick={() => handleDeleteKey(key)}
                          className="p-1 text-gray-400 hover:text-red-600 dark:hover:text-red-400 transition-colors"
                          title="Delete key permanently"
                        >
                          <Trash2 className="h-4 w-4" />
                        </button>
                      ) : null}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {filteredKeys.length === 0 && (
          <div className="text-center py-12">
            <Key className="mx-auto h-12 w-12 text-gray-400" />
            <h3 className="mt-2 text-sm font-medium text-gray-900 dark:text-gray-100">
              No API keys found
            </h3>
            <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {searchTerm || statusFilter !== "all"
                ? "Try adjusting your search or filters."
                : "Get started by creating your first API key."}
            </p>
          </div>
        )}
      </div>

      {/* Modals */}
      <CreateAPIKeyModal
        isOpen={showCreateModal}
        onClose={() => setShowCreateModal(false)}
        onSuccess={handleKeyCreated}
        agents={agents}
      />

      <ConfirmDialog
        isOpen={showDisableConfirm}
        title="Disable API Key"
        message={`Are you sure you want to disable "${selectedKey?.name}"? The key will be marked as inactive and cannot be used for authentication. You can delete it permanently later.`}
        confirmText="Disable"
        cancelText="Cancel"
        variant="warning"
        onConfirm={confirmDisable}
        onCancel={() => {
          setShowDisableConfirm(false);
          setSelectedKey(null);
        }}
      />

      <ConfirmDialog
        isOpen={showDeleteConfirm}
        title="Delete API Key"
        message={`Are you sure you want to permanently delete "${selectedKey?.name}"? This action cannot be undone.`}
        confirmText="Delete"
        cancelText="Cancel"
        variant="danger"
        onConfirm={confirmDelete}
        onCancel={() => {
          setShowDeleteConfirm(false);
          setSelectedKey(null);
        }}
      />
    </div>
  );
}

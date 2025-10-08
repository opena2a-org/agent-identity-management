'use client';

import { useState, useEffect } from 'react';
import { X, Shield, Calendar, CheckCircle, Clock, Edit, Trash2, Key } from 'lucide-react';
import { Agent, Tag, AgentCapability, api } from '@/lib/api';
import { TagSelector } from '../ui/tag-selector';

interface AgentDetailModalProps {
  isOpen: boolean;
  onClose: () => void;
  agent: Agent | null;
  onEdit?: (agent: Agent) => void;
  onDelete?: (agent: Agent) => void;
}

export function AgentDetailModal({
  isOpen,
  onClose,
  agent,
  onEdit,
  onDelete
}: AgentDetailModalProps) {
  const [agentTags, setAgentTags] = useState<Tag[]>([]);
  const [availableTags, setAvailableTags] = useState<Tag[]>([]);
  const [suggestedTags, setSuggestedTags] = useState<Tag[]>([]);
  const [loadingTags, setLoadingTags] = useState(false);
  const [capabilities, setCapabilities] = useState<AgentCapability[]>([]);
  const [loadingCapabilities, setLoadingCapabilities] = useState(false);

  useEffect(() => {
    if (isOpen && agent) {
      loadTags();
      loadCapabilities();
    }
  }, [isOpen, agent]);

  const loadTags = async () => {
    if (!agent) return;
    setLoadingTags(true);
    try {
      const [currentTags, allTags, suggestions] = await Promise.all([
        api.getAgentTags(agent.id),
        api.listTags(),
        api.suggestTagsForAgent(agent.id),
      ]);
      setAgentTags(currentTags || []);
      setAvailableTags(allTags || []);
      setSuggestedTags(suggestions || []);
    } catch (error) {
      console.error('Failed to load tags:', error);
    } finally {
      setLoadingTags(false);
    }
  };

  const loadCapabilities = async () => {
    if (!agent) return;
    setLoadingCapabilities(true);
    try {
      const caps = await api.getAgentCapabilities(agent.id, true);
      setCapabilities(caps || []);
    } catch (error) {
      console.error('Failed to load capabilities:', error);
    } finally {
      setLoadingCapabilities(false);
    }
  };

  const handleTagsChange = async (newTags: Tag[]) => {
    if (!agent) return;

    const addedTags = newTags.filter(t => !agentTags.some(at => at.id === t.id));
    const removedTags = agentTags.filter(t => !newTags.some(nt => nt.id === t.id));

    try {
      // Add new tags
      if (addedTags.length > 0) {
        await api.addTagsToAgent(agent.id, addedTags.map(t => t.id));
      }

      // Remove tags
      for (const tag of removedTags) {
        await api.removeTagFromAgent(agent.id, tag.id);
      }

      setAgentTags(newTags);
    } catch (error) {
      console.error('Failed to update tags:', error);
    }
  };

  if (!isOpen || !agent) return null;

  const formatDate = (dateString: string) => {
    const date = new Date(dateString);
    return date.toLocaleDateString('en-US', {
      month: 'long',
      day: 'numeric',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit'
    });
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'verified':
        return 'bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-300';
      case 'pending':
        return 'bg-yellow-100 dark:bg-yellow-900/30 text-yellow-800 dark:text-yellow-300';
      case 'suspended':
      case 'revoked':
        return 'bg-red-100 dark:bg-red-900/30 text-red-800 dark:text-red-300';
      default:
        return 'bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-300';
    }
  };

  const getTrustScoreColor = (score: number) => {
    if (score >= 80) return 'text-green-600 dark:text-green-400';
    if (score >= 60) return 'text-yellow-600 dark:text-yellow-400';
    return 'text-red-600 dark:text-red-400';
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50">
      <div className="bg-white dark:bg-gray-900 rounded-lg shadow-xl max-w-3xl w-full max-h-[90vh] overflow-y-auto">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-200 dark:border-gray-700">
          <div className="flex items-center gap-3">
            <div className="w-12 h-12 bg-gradient-to-br from-blue-500 to-blue-600 rounded-lg flex items-center justify-center">
              <Shield className="h-6 w-6 text-white" />
            </div>
            <div>
              <h2 className="text-xl font-semibold text-gray-900 dark:text-white">
                {agent.display_name}
              </h2>
              <p className="text-sm text-gray-500 dark:text-gray-400">{agent.name}</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Body */}
        <div className="p-6 space-y-6">
          {/* Status and Trust Score */}
          <div className="flex items-center gap-4">
            <div>
              <span className="text-sm text-gray-500 dark:text-gray-400 block mb-1">Status</span>
              <span className={`inline-flex items-center px-3 py-1 rounded-full text-sm font-medium capitalize ${getStatusColor(agent.status)}`}>
                {agent.status}
              </span>
            </div>
            <div>
              <span className="text-sm text-gray-500 dark:text-gray-400 block mb-1">Trust Score</span>
              <span className={`text-2xl font-bold ${getTrustScoreColor(agent.trust_score)}`}>
                {agent.trust_score}%
              </span>
            </div>
            <div>
              <span className="text-sm text-gray-500 dark:text-gray-400 block mb-1">Type</span>
              <span className={`inline-flex items-center px-3 py-1 rounded-full text-sm font-medium ${
                agent.agent_type === 'ai_agent'
                  ? 'bg-blue-100 dark:bg-blue-900/30 text-blue-800 dark:text-blue-300'
                  : 'bg-purple-100 dark:bg-purple-900/30 text-purple-800 dark:text-purple-300'
              }`}>
                {agent.agent_type === 'ai_agent' ? 'AI Agent' : 'MCP Server'}
              </span>
            </div>
          </div>

          {/* Description */}
          {agent.description && (
            <div>
              <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Description</h3>
              <p className="text-sm text-gray-600 dark:text-gray-400">{agent.description}</p>
            </div>
          )}

          {/* Tags */}
          <div>
            <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">Tags</h3>
            {loadingTags ? (
              <div className="text-sm text-gray-500 dark:text-gray-400">Loading tags...</div>
            ) : (
              <TagSelector
                selectedTags={agentTags}
                availableTags={availableTags}
                suggestedTags={suggestedTags}
                maxTags={3}
                onTagsChange={handleTagsChange}
              />
            )}
          </div>

          {/* Capabilities */}
          <div>
            <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3 flex items-center gap-2">
              <Key className="h-4 w-4" />
              Capabilities
            </h3>
            {loadingCapabilities ? (
              <div className="text-sm text-gray-500 dark:text-gray-400">Loading capabilities...</div>
            ) : capabilities && capabilities.length > 0 ? (
              <div className="grid grid-cols-2 gap-2">
                {capabilities.map((capability) => (
                  <div
                    key={capability.id}
                    className="flex items-center gap-2 px-3 py-2 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-md"
                  >
                    <CheckCircle className="h-4 w-4 text-blue-600 dark:text-blue-400 flex-shrink-0" />
                    <div className="flex-1 min-w-0">
                      <p className="text-sm font-medium text-blue-900 dark:text-blue-100 truncate">
                        {capability.capabilityType}
                      </p>
                      {capability.capabilityScope && Object.keys(capability.capabilityScope).length > 0 && (
                        <p className="text-xs text-blue-600 dark:text-blue-400 truncate">
                          {Object.entries(capability.capabilityScope)
                            .map(([key, value]) => `${key}: ${value}`)
                            .join(', ')}
                        </p>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <div className="text-sm text-gray-500 dark:text-gray-400 italic">
                No capabilities registered
              </div>
            )}
          </div>

          {/* Details Grid */}
          <div className="grid grid-cols-2 gap-6">
            <div>
              <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Version</h3>
              <p className="text-sm text-gray-900 dark:text-gray-100 font-mono">{agent.version}</p>
            </div>

            <div>
              <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Organization ID</h3>
              <p className="text-sm text-gray-900 dark:text-gray-100 font-mono">{agent.organization_id}</p>
            </div>

            <div>
              <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2 flex items-center gap-2">
                <Calendar className="h-4 w-4" />
                Created
              </h3>
              <p className="text-sm text-gray-900 dark:text-gray-100">{formatDate(agent.created_at)}</p>
            </div>

            <div>
              <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2 flex items-center gap-2">
                <Clock className="h-4 w-4" />
                Last Updated
              </h3>
              <p className="text-sm text-gray-900 dark:text-gray-100">{formatDate(agent.updated_at)}</p>
            </div>
          </div>

          {/* Audit History */}
          <div>
            <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">Recent Activity</h3>
            <div className="space-y-2">
              <div className="flex items-center gap-3 p-3 bg-gray-50 dark:bg-gray-800 rounded-lg">
                <CheckCircle className="h-4 w-4 text-green-600 dark:text-green-400" />
                <div className="flex-1">
                  <p className="text-sm text-gray-900 dark:text-gray-100">Agent registered</p>
                  <p className="text-xs text-gray-500 dark:text-gray-400">{formatDate(agent.created_at)}</p>
                </div>
              </div>
              <div className="flex items-center gap-3 p-3 bg-gray-50 dark:bg-gray-800 rounded-lg">
                <CheckCircle className="h-4 w-4 text-blue-600 dark:text-blue-400" />
                <div className="flex-1">
                  <p className="text-sm text-gray-900 dark:text-gray-100">Agent updated</p>
                  <p className="text-xs text-gray-500 dark:text-gray-400">{formatDate(agent.updated_at)}</p>
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* Footer */}
        <div className="flex items-center justify-end gap-3 px-6 py-4 border-t border-gray-200 dark:border-gray-700">
          {onDelete && (
            <button
              onClick={() => onDelete(agent)}
              className="px-4 py-2 text-sm font-medium text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg transition-colors flex items-center gap-2"
            >
              <Trash2 className="h-4 w-4" />
              Delete
            </button>
          )}
          {onEdit && (
            <button
              onClick={() => onEdit(agent)}
              className="px-4 py-2 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-lg transition-colors flex items-center gap-2"
            >
              <Edit className="h-4 w-4" />
              Edit Agent
            </button>
          )}
        </div>
      </div>
    </div>
  );
}

'use client';

import { useState, useEffect } from 'react';
import { Card } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
  Copy, Check, Key, Plus, Trash2, Ban, Loader2,
  Calendar, Clock, AlertTriangle, Shield
} from 'lucide-react';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { api } from '@/lib/api';
import { formatDistanceToNow, differenceInDays, format } from 'date-fns';

interface APIKey {
  id: string;
  name: string;
  prefix: string;
  agentId: string;
  isActive: boolean;
  expiresAt?: string;
  lastUsedAt?: string;
  createdAt: string;
}

interface APIKeysTabProps {
  agentId: string;
}

export function APIKeysTab({ agentId }: APIKeysTabProps) {
  const [apiKeys, setApiKeys] = useState<APIKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const [showCreateDialog, setShowCreateDialog] = useState(false);
  const [showNewKeyDialog, setShowNewKeyDialog] = useState(false);
  const [showRevokeDialog, setShowRevokeDialog] = useState(false);
  const [selectedKey, setSelectedKey] = useState<APIKey | null>(null);
  const [newKeyName, setNewKeyName] = useState('');
  const [newKeyValue, setNewKeyValue] = useState('');
  const [copied, setCopied] = useState(false);

  const fetchAPIKeys = async () => {
    setLoading(true);
    try {
      const response = await api.getAPIKeys(agentId);
      setApiKeys(response.apiKeys || []);
    } catch (error) {
      console.error('Failed to fetch API keys:', error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchAPIKeys();
  }, [agentId]);

  const handleCreateKey = async () => {
    if (!newKeyName.trim()) return;

    setCreating(true);
    try {
      const response = await api.createAPIKey(agentId, newKeyName.trim());
      setNewKeyValue(response.apiKey);
      setShowCreateDialog(false);
      setShowNewKeyDialog(true);
      setNewKeyName('');
      await fetchAPIKeys();
    } catch (error: any) {
      alert(error?.message || 'Failed to create API key');
    } finally {
      setCreating(false);
    }
  };

  const handleRevokeKey = async () => {
    if (!selectedKey) return;

    try {
      await api.revokeAPIKey(selectedKey.id);
      await fetchAPIKeys();
      setShowRevokeDialog(false);
      setSelectedKey(null);
    } catch (error: any) {
      alert(error?.message || 'Failed to revoke API key');
    }
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const getExpirationStatus = (expiresAt?: string) => {
    if (!expiresAt) return { status: 'never', label: 'Never expires', color: 'text-muted-foreground' };

    const expDate = new Date(expiresAt);
    const now = new Date();
    const daysUntil = differenceInDays(expDate, now);

    if (daysUntil < 0) {
      return { status: 'expired', label: 'Expired', color: 'text-red-600' };
    } else if (daysUntil <= 7) {
      return { status: 'critical', label: `Expires in ${daysUntil} days`, color: 'text-red-600' };
    } else if (daysUntil <= 30) {
      return { status: 'warning', label: `Expires in ${daysUntil} days`, color: 'text-amber-600' };
    } else {
      return { status: 'ok', label: `Expires ${format(expDate, 'MMM d, yyyy')}`, color: 'text-muted-foreground' };
    }
  };

  if (loading) {
    return <div className="text-center py-8">Loading API keys...</div>;
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <Card className="p-6">
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-2">
            <Key className="h-5 w-5" />
            <h3 className="text-lg font-semibold">API Keys</h3>
            <span className="px-2 py-0.5 text-xs bg-muted rounded-full">
              {apiKeys.filter(k => k.isActive).length} active
            </span>
          </div>
          <Button onClick={() => setShowCreateDialog(true)} size="sm">
            <Plus className="h-4 w-4 mr-1" />
            Generate New Key
          </Button>
        </div>

        <p className="text-sm text-muted-foreground">
          API keys are used to authenticate your agent when making API calls.
          Each key can be independently revoked without affecting other keys.
        </p>
      </Card>

      {/* API Keys List */}
      {apiKeys.length === 0 ? (
        <Card className="p-8 text-center">
          <Key className="h-12 w-12 mx-auto text-muted-foreground mb-4" />
          <h4 className="font-medium mb-2">No API Keys</h4>
          <p className="text-sm text-muted-foreground mb-4">
            This agent doesn't have any API keys yet. Generate one to start making authenticated API calls.
          </p>
          <Button onClick={() => setShowCreateDialog(true)}>
            <Plus className="h-4 w-4 mr-2" />
            Generate Your First API Key
          </Button>
        </Card>
      ) : (
        <div className="space-y-3">
          {apiKeys.map((key) => {
            const expStatus = getExpirationStatus(key.expiresAt);
            const isExpired = expStatus.status === 'expired';

            return (
              <Card
                key={key.id}
                className={`p-4 ${!key.isActive || isExpired ? 'opacity-60' : ''}`}
              >
                <div className="flex items-start justify-between">
                  <div className="space-y-2">
                    <div className="flex items-center gap-2">
                      <span className="font-medium">{key.name}</span>
                      {!key.isActive && (
                        <span className="px-2 py-0.5 text-xs bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300 rounded-full">
                          Revoked
                        </span>
                      )}
                      {key.isActive && isExpired && (
                        <span className="px-2 py-0.5 text-xs bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300 rounded-full">
                          Expired
                        </span>
                      )}
                      {key.isActive && !isExpired && (
                        <span className="px-2 py-0.5 text-xs bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300 rounded-full">
                          Active
                        </span>
                      )}
                    </div>

                    <div className="flex items-center gap-4 text-xs text-muted-foreground">
                      <code className="px-2 py-1 bg-muted rounded font-mono">
                        {key.prefix}...
                      </code>

                      <span className="flex items-center gap-1">
                        <Clock className="h-3 w-3" />
                        Created {formatDistanceToNow(new Date(key.createdAt), { addSuffix: true })}
                      </span>

                      {key.lastUsedAt && (
                        <span className="flex items-center gap-1">
                          <Shield className="h-3 w-3" />
                          Last used {formatDistanceToNow(new Date(key.lastUsedAt), { addSuffix: true })}
                        </span>
                      )}
                    </div>

                    <div className={`text-xs flex items-center gap-1 ${expStatus.color}`}>
                      <Calendar className="h-3 w-3" />
                      {expStatus.label}
                    </div>
                  </div>

                  <div className="flex items-center gap-2">
                    {key.isActive && !isExpired && (
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        onClick={() => {
                          setSelectedKey(key);
                          setShowRevokeDialog(true);
                        }}
                        className="text-red-600 hover:text-red-700 hover:bg-red-50"
                      >
                        <Ban className="h-4 w-4 mr-1" />
                        Revoke
                      </Button>
                    )}
                  </div>
                </div>
              </Card>
            );
          })}
        </div>
      )}

      {/* Security Tips */}
      <Card className="p-4 bg-blue-50 dark:bg-blue-950 border-blue-200 dark:border-blue-800">
        <div className="text-sm text-blue-800 dark:text-blue-200">
          <p className="font-medium mb-2">Security Best Practices</p>
          <ul className="list-disc list-inside space-y-1 text-xs text-blue-700 dark:text-blue-300">
            <li>Rotate API keys every 90 days for enhanced security</li>
            <li>Use descriptive names to track key usage (e.g., "production-server-1")</li>
            <li>Revoke unused or compromised keys immediately</li>
            <li>Never commit API keys to version control</li>
          </ul>
        </div>
      </Card>

      {/* Create Key Dialog */}
      <Dialog open={showCreateDialog} onOpenChange={setShowCreateDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Generate New API Key</DialogTitle>
            <DialogDescription>
              Create a new API key for this agent. Give it a descriptive name to help identify its purpose.
            </DialogDescription>
          </DialogHeader>
          <div className="py-4">
            <label className="text-sm font-medium mb-2 block">Key Name</label>
            <Input
              placeholder="e.g., production-server, dev-testing"
              value={newKeyName}
              onChange={(e) => setNewKeyName(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleCreateKey()}
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowCreateDialog(false)}>
              Cancel
            </Button>
            <Button onClick={handleCreateKey} disabled={creating || !newKeyName.trim()}>
              {creating ? (
                <>
                  <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                  Generating...
                </>
              ) : (
                'Generate Key'
              )}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* New Key Display Dialog */}
      <AlertDialog open={showNewKeyDialog} onOpenChange={setShowNewKeyDialog}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle className="flex items-center gap-2">
              <Check className="h-5 w-5 text-green-600" />
              API Key Generated
            </AlertDialogTitle>
            <AlertDialogDescription>
              <span className="text-amber-600 font-medium">
                Copy this key now - you won't be able to see it again!
              </span>
            </AlertDialogDescription>
          </AlertDialogHeader>
          <div className="my-4">
            <code className="block p-3 bg-muted rounded-md text-xs font-mono break-all">
              {newKeyValue}
            </code>
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => copyToClipboard(newKeyValue)}
              className="mt-2 w-full"
            >
              {copied ? (
                <>
                  <Check className="h-4 w-4 mr-2" />
                  Copied!
                </>
              ) : (
                <>
                  <Copy className="h-4 w-4 mr-2" />
                  Copy API Key
                </>
              )}
            </Button>
          </div>
          <AlertDialogFooter>
            <AlertDialogAction onClick={() => {
              setShowNewKeyDialog(false);
              setNewKeyValue('');
            }}>
              Done
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Revoke Confirmation Dialog */}
      <AlertDialog open={showRevokeDialog} onOpenChange={setShowRevokeDialog}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle className="flex items-center gap-2 text-red-600">
              <AlertTriangle className="h-5 w-5" />
              Revoke API Key
            </AlertDialogTitle>
            <AlertDialogDescription>
              Are you sure you want to revoke the key "<strong>{selectedKey?.name}</strong>"?
              This action cannot be undone. Any applications using this key will lose access immediately.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleRevokeKey}
              className="bg-red-600 hover:bg-red-700"
            >
              Revoke Key
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

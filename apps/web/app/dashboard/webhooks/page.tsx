'use client';

import { useState, useEffect } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import {
  Webhook,
  Plus,
  MoreVertical,
  Edit,
  Trash2,
  Power,
  PowerOff,
  Ban,
  TestTube,
  Eye,
  CheckCircle,
  XCircle,
  AlertCircle,
  Clock,
  ExternalLink,
} from 'lucide-react';
import { api } from '@/lib/api';
import { AuthGuard } from '@/components/auth-guard';
import { WebhookCreateModal } from '@/components/webhook/webhook-create-modal';
import { WebhookDetailModal } from '@/components/webhook/webhook-detail-modal';
import { useToast } from '@/hooks/use-toast';
import { formatDistanceToNow } from 'date-fns';

interface WebhookItem {
  id: string;
  organizationId: string;
  name: string;
  url: string;
  events: string[];
  isActive: boolean;
  secret: string;
  createdAt: string;
  lastTriggeredAt?: string;
  successCount: number;
  failureCount: number;
}

export default function WebhooksPage() {
  const [webhooks, setWebhooks] = useState<WebhookItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [showDetailModal, setShowDetailModal] = useState(false);
  const [selectedWebhook, setSelectedWebhook] = useState<WebhookItem | null>(null);
  const [deleteWebhookId, setDeleteWebhookId] = useState<string | null>(null);
  const [testingWebhookId, setTestingWebhookId] = useState<string | null>(null);
  const [togglingWebhookId, setTogglingWebhookId] = useState<string | null>(null);
  const { toast } = useToast();

  const fetchWebhooks = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.listWebhooks();
      setWebhooks(data);
    } catch (err: any) {
      console.error('Failed to fetch webhooks:', err);
      setError(err.message || 'Failed to load webhooks');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchWebhooks();
  }, []);

  const handleCreateSuccess = () => {
    setShowCreateModal(false);
    fetchWebhooks();
    toast({
      title: 'Webhook created',
      description: 'Your webhook has been created successfully.',
    });
  };

  const handleDeleteWebhook = async () => {
    if (!deleteWebhookId) return;

    try {
      await api.deleteWebhook(deleteWebhookId);
      toast({
        title: 'Webhook deleted',
        description: 'The webhook has been deleted successfully.',
      });
      fetchWebhooks();
    } catch (err: any) {
      toast({
        title: 'Error',
        description: err.message || 'Failed to delete webhook',
        variant: 'destructive',
      });
    } finally {
      setDeleteWebhookId(null);
    }
  };

  const handleTestWebhook = async (id: string) => {
    setTestingWebhookId(id);
    try {
      const result = await api.testWebhook(id);
      if (result.success) {
        toast({
          title: 'Test successful',
          description: `Webhook responded with status ${result.responseCode}`,
        });
      } else {
        toast({
          title: 'Test failed',
          description: result.message || 'Webhook test failed',
          variant: 'destructive',
        });
      }
    } catch (err: any) {
      toast({
        title: 'Test error',
        description: err.message || 'Failed to test webhook',
        variant: 'destructive',
      });
    } finally {
      setTestingWebhookId(null);
    }
  };

  const handleToggleWebhook = async (webhook: WebhookItem) => {
    setTogglingWebhookId(webhook.id);
    try {
      await api.updateWebhook(webhook.id, {
        name: webhook.name,
        url: webhook.url,
        events: webhook.events,
        isActive: !webhook.isActive,
      });
      toast({
        title: webhook.isActive ? 'Webhook disabled' : 'Webhook enabled',
        description: `The webhook has been ${webhook.isActive ? 'disabled' : 'enabled'} successfully.`,
      });
      fetchWebhooks();
    } catch (err: any) {
      toast({
        title: 'Error',
        description: err.message || 'Failed to toggle webhook',
        variant: 'destructive',
      });
    } finally {
      setTogglingWebhookId(null);
    }
  };

  const handleViewDetails = async (webhook: WebhookItem) => {
    setSelectedWebhook(webhook);
    setShowDetailModal(true);
  };

  const getSuccessRate = (webhook: WebhookItem) => {
    const total = webhook.successCount + webhook.failureCount;
    if (total === 0) return 0;
    return (webhook.successCount / total) * 100;
  };

  if (loading) {
    return (
      <AuthGuard>
        <div className="space-y-6">
          <div className="flex items-center justify-between">
            <div className="space-y-2">
              <Skeleton className="h-9 w-64" />
              <Skeleton className="h-4 w-96" />
            </div>
            <Skeleton className="h-10 w-40" />
          </div>

          <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4">
            {[...Array(4)].map((_, i) => (
              <div
                key={i}
                className="glass p-6"
              >
                <div className="flex items-center">
                  <div className="flex-shrink-0">
                    <Skeleton className="h-6 w-6 rounded" />
                  </div>
                  <div className="ml-5 flex-1 space-y-2">
                    <Skeleton className="h-4 w-24" />
                    <Skeleton className="h-8 w-16" />
                  </div>
                </div>
              </div>
            ))}
          </div>

          <div className="glass">
            <div className="p-6">
              <div className="space-y-3">
                {[...Array(5)].map((_, i) => (
                  <Skeleton key={i} className="h-20 w-full" />
                ))}
              </div>
            </div>
          </div>
        </div>
      </AuthGuard>
    );
  }

  if (error) {
    return (
      <AuthGuard>
        <div className="space-y-6">
          <div className="text-center py-16">
            <AlertCircle className="h-16 w-16 mx-auto mb-4 text-ink-tertiary" />
            <h2 className="text-2xl font-semibold mb-2 text-ink">Unable to load webhooks</h2>
            <p className="text-ink-secondary">{error}</p>
            <Button onClick={fetchWebhooks} className="mt-4">
              Try again
            </Button>
          </div>
        </div>
      </AuthGuard>
    );
  }

  return (
    <AuthGuard>
      <div className="space-y-6">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold text-ink">
              Webhooks
            </h1>
            <p className="mt-1 text-sm text-ink-secondary">
              Manage webhook endpoints and monitor delivery status
            </p>
          </div>
          <button
            onClick={() => setShowCreateModal(true)}
            className="flex items-center gap-2 px-4 py-2 rounded-pill bg-brand text-white shadow-accent hover:bg-brand-hover transition-colors"
          >
            <Plus className="h-4 w-4" />
            Create webhook
          </button>
        </div>

        {/* Summary Cards */}
        <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4">
          <div className="glass p-6">
            <div className="flex items-center">
              <div className="flex-shrink-0">
                <Webhook className="h-6 w-6 text-ink-tertiary" />
              </div>
              <div className="ml-5 w-0 flex-1">
                <dl>
                  <dt className="text-sm font-medium text-ink-secondary truncate">
                    Total webhooks
                  </dt>
                  <dd className="flex items-baseline">
                    <div className="text-2xl font-semibold text-ink">
                      {webhooks.length}
                    </div>
                  </dd>
                </dl>
              </div>
            </div>
          </div>

          <div className="glass p-6">
            <div className="flex items-center">
              <div className="flex-shrink-0">
                <Power className="h-6 w-6 text-ink-tertiary" />
              </div>
              <div className="ml-5 w-0 flex-1">
                <dl>
                  <dt className="text-sm font-medium text-ink-secondary truncate">
                    Active
                  </dt>
                  <dd className="flex items-baseline">
                    <div className="text-2xl font-semibold text-success-text">
                      {webhooks.filter((w) => w.isActive).length}
                    </div>
                  </dd>
                </dl>
              </div>
            </div>
          </div>

          <div className="glass p-6">
            <div className="flex items-center">
              <div className="flex-shrink-0">
                <CheckCircle className="h-6 w-6 text-ink-tertiary" />
              </div>
              <div className="ml-5 w-0 flex-1">
                <dl>
                  <dt className="text-sm font-medium text-ink-secondary truncate">
                    Total successes
                  </dt>
                  <dd className="flex items-baseline">
                    <div className="text-2xl font-semibold text-success-text">
                      {webhooks.reduce((sum, w) => sum + w.successCount, 0).toLocaleString()}
                    </div>
                  </dd>
                </dl>
              </div>
            </div>
          </div>

          <div className="glass p-6">
            <div className="flex items-center">
              <div className="flex-shrink-0">
                <XCircle className="h-6 w-6 text-ink-tertiary" />
              </div>
              <div className="ml-5 w-0 flex-1">
                <dl>
                  <dt className="text-sm font-medium text-ink-secondary truncate">
                    Total failures
                  </dt>
                  <dd className="flex items-baseline">
                    <div className="text-2xl font-semibold text-danger-text">
                      {webhooks.reduce((sum, w) => sum + w.failureCount, 0).toLocaleString()}
                    </div>
                  </dd>
                </dl>
              </div>
            </div>
          </div>
        </div>

        {/* Webhooks Table */}
        <Card>
          <CardHeader>
            <CardTitle>Webhook endpoints</CardTitle>
            <CardDescription>
              Configure and monitor webhook endpoints for real-time event notifications
            </CardDescription>
          </CardHeader>
          <CardContent>
            {webhooks.length === 0 ? (
              <div className="text-center py-12">
                <Webhook className="h-16 w-16 mx-auto mb-4 text-muted-foreground" />
                <h3 className="text-lg font-semibold mb-2">No webhooks configured</h3>
                <p className="text-muted-foreground mb-4">
                  Create your first webhook to receive real-time event notifications
                </p>
                <Button onClick={() => setShowCreateModal(true)}>
                  <Plus className="h-4 w-4 mr-2" />
                  Create webhook
                </Button>
              </div>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Name</TableHead>
                    <TableHead>URL</TableHead>
                    <TableHead>Events</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Success rate</TableHead>
                    <TableHead>Last triggered</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {webhooks.map((webhook) => {
                    const successRate = getSuccessRate(webhook);
                    return (
                      <TableRow key={webhook.id}>
                        <TableCell className="font-medium">{webhook.name}</TableCell>
                        <TableCell>
                          <div className="flex items-center gap-2">
                            <code className="text-xs bg-muted px-2 py-1 rounded">
                              {webhook.url.length > 40
                                ? webhook.url.substring(0, 40) + '...'
                                : webhook.url}
                            </code>
                            <a
                              href={webhook.url}
                              target="_blank"
                              rel="noopener noreferrer"
                              className="text-brand-text hover:underline"
                            >
                              <ExternalLink className="h-3 w-3" />
                            </a>
                          </div>
                        </TableCell>
                        <TableCell>
                          <div className="flex flex-wrap gap-1">
                            {webhook.events.slice(0, 2).map((event) => (
                              <Badge key={event} variant="outline" className="text-xs">
                                {event}
                              </Badge>
                            ))}
                            {webhook.events.length > 2 && (
                              <Badge variant="outline" className="text-xs">
                                +{webhook.events.length - 2}
                              </Badge>
                            )}
                          </div>
                        </TableCell>
                        <TableCell>
                          {webhook.isActive ? (
                            <Badge className="bg-success-fill text-success-text border-success-border">
                              <Power className="h-3 w-3 mr-1" />
                              Active
                            </Badge>
                          ) : (
                            <Badge className="bg-glass-inset-gray text-ink-secondary border-divider">
                              <PowerOff className="h-3 w-3 mr-1" />
                              Inactive
                            </Badge>
                          )}
                        </TableCell>
                        <TableCell>
                          <div className="flex items-center gap-2">
                            <div className="flex-1 h-2 bg-track rounded-full overflow-hidden w-20">
                              <div
                                className={`h-full rounded-full ${
                                  successRate >= 90
                                    ? 'bg-success'
                                    : successRate >= 75
                                      ? 'bg-brand'
                                      : successRate >= 50
                                        ? 'bg-warning'
                                        : 'bg-danger'
                                }`}
                                style={{ width: `${successRate}%` }}
                              />
                            </div>
                            <span className="text-xs text-muted-foreground">
                              {successRate.toFixed(0)}%
                            </span>
                          </div>
                        </TableCell>
                        <TableCell>
                          {webhook.lastTriggeredAt ? (
                            <div className="flex items-center gap-1 text-sm text-muted-foreground">
                              <Clock className="h-3 w-3" />
                              {formatDistanceToNow(new Date(webhook.lastTriggeredAt), {
                                addSuffix: true,
                              })}
                            </div>
                          ) : (
                            <span className="text-sm text-muted-foreground">Never</span>
                          )}
                        </TableCell>
                        <TableCell className="text-right">
                          <div className="flex items-center justify-end gap-2">
                            <button
                              onClick={() => handleViewDetails(webhook)}
                              className="p-1 text-ink-tertiary hover:text-brand-text transition-colors"
                              title="View details"
                            >
                              <Eye className="h-4 w-4" />
                            </button>
                            <button
                              onClick={() => handleTestWebhook(webhook.id)}
                              disabled={testingWebhookId === webhook.id}
                              className="p-1 text-ink-tertiary hover:text-success-text transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                              title="Test webhook"
                            >
                              <TestTube className="h-4 w-4" />
                            </button>
                            {webhook.isActive ? (
                              <button
                                onClick={() => handleToggleWebhook(webhook)}
                                disabled={togglingWebhookId === webhook.id}
                                className="p-1 text-ink-tertiary hover:text-warning-text transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                                title="Disable webhook"
                              >
                                <Ban className="h-4 w-4" />
                              </button>
                            ) : (
                              <button
                                onClick={() => setDeleteWebhookId(webhook.id)}
                                className="p-1 text-ink-tertiary hover:text-danger-text transition-colors"
                                title="Delete webhook permanently"
                              >
                                <Trash2 className="h-4 w-4" />
                              </button>
                            )}
                          </div>
                        </TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Create Modal */}
      {showCreateModal && (
        <WebhookCreateModal
          isOpen={showCreateModal}
          onClose={() => setShowCreateModal(false)}
          onSuccess={handleCreateSuccess}
        />
      )}

      {/* Detail Modal */}
      {showDetailModal && selectedWebhook && (
        <WebhookDetailModal
          isOpen={showDetailModal}
          webhookId={selectedWebhook.id}
          onClose={() => {
            setShowDetailModal(false);
            setSelectedWebhook(null);
          }}
          onRefresh={fetchWebhooks}
        />
      )}

      {/* Delete Confirmation Dialog */}
      <AlertDialog open={!!deleteWebhookId} onOpenChange={() => setDeleteWebhookId(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete webhook</AlertDialogTitle>
            <AlertDialogDescription>
              Are you sure you want to delete this webhook? This action cannot be undone, and the
              webhook will stop receiving events immediately.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleDeleteWebhook}
              className="bg-danger text-white hover:opacity-90"
            >
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </AuthGuard>
  );
}

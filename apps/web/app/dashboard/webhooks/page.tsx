'use client';

import { useState, useEffect } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
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
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
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
  organization_id: string;
  name: string;
  url: string;
  events: string[];
  is_active: boolean;
  secret: string;
  created_at: string;
  last_triggered_at?: string;
  success_count: number;
  failure_count: number;
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
          description: `Webhook responded with status ${result.response_code}`,
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
        is_active: !webhook.is_active,
      });
      toast({
        title: webhook.is_active ? 'Webhook disabled' : 'Webhook enabled',
        description: `The webhook has been ${webhook.is_active ? 'disabled' : 'enabled'} successfully.`,
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
    const total = webhook.success_count + webhook.failure_count;
    if (total === 0) return 0;
    return (webhook.success_count / total) * 100;
  };

  if (loading) {
    return (
      <AuthGuard>
        <div className="p-8 space-y-6">
          <div>
            <h1 className="text-3xl font-bold">Webhooks</h1>
            <p className="text-muted-foreground">Loading webhooks...</p>
          </div>
          <div className="space-y-4">
            {[...Array(5)].map((_, i) => (
              <Skeleton key={i} className="h-16" />
            ))}
          </div>
        </div>
      </AuthGuard>
    );
  }

  if (error) {
    return (
      <AuthGuard>
        <div className="p-8">
          <div className="text-center py-16">
            <AlertCircle className="h-16 w-16 mx-auto mb-4 text-muted-foreground" />
            <h2 className="text-2xl font-semibold mb-2">Unable to Load Webhooks</h2>
            <p className="text-muted-foreground">{error}</p>
            <Button onClick={fetchWebhooks} className="mt-4">
              Try Again
            </Button>
          </div>
        </div>
      </AuthGuard>
    );
  }

  return (
    <AuthGuard>
      <div className="p-8 space-y-6">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold flex items-center gap-2">
              <Webhook className="h-8 w-8" />
              Webhooks
            </h1>
            <p className="text-muted-foreground mt-1">
              Manage webhook endpoints and monitor delivery status
            </p>
          </div>
          <Button onClick={() => setShowCreateModal(true)}>
            <Plus className="h-4 w-4 mr-2" />
            Create Webhook
          </Button>
        </div>

        {/* Summary Cards */}
        <div className="grid grid-cols-1 md:grid-cols-4 gap-6">
          <Card>
            <CardContent className="pt-6">
              <div className="flex items-center gap-3">
                <Webhook className="h-10 w-10 text-blue-600" />
                <div>
                  <div className="text-sm text-muted-foreground">Total Webhooks</div>
                  <div className="text-3xl font-bold">{webhooks.length}</div>
                </div>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardContent className="pt-6">
              <div className="flex items-center gap-3">
                <Power className="h-10 w-10 text-green-600" />
                <div>
                  <div className="text-sm text-muted-foreground">Active</div>
                  <div className="text-3xl font-bold text-green-600">
                    {webhooks.filter((w) => w.is_active).length}
                  </div>
                </div>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardContent className="pt-6">
              <div className="flex items-center gap-3">
                <CheckCircle className="h-10 w-10 text-green-600" />
                <div>
                  <div className="text-sm text-muted-foreground">Total Successes</div>
                  <div className="text-3xl font-bold text-green-600">
                    {webhooks.reduce((sum, w) => sum + w.success_count, 0).toLocaleString()}
                  </div>
                </div>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardContent className="pt-6">
              <div className="flex items-center gap-3">
                <XCircle className="h-10 w-10 text-red-600" />
                <div>
                  <div className="text-sm text-muted-foreground">Total Failures</div>
                  <div className="text-3xl font-bold text-red-600">
                    {webhooks.reduce((sum, w) => sum + w.failure_count, 0).toLocaleString()}
                  </div>
                </div>
              </div>
            </CardContent>
          </Card>
        </div>

        {/* Webhooks Table */}
        <Card>
          <CardHeader>
            <CardTitle>Webhook Endpoints</CardTitle>
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
                  Create Webhook
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
                    <TableHead>Success Rate</TableHead>
                    <TableHead>Last Triggered</TableHead>
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
                              className="text-blue-600 hover:text-blue-800"
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
                          {webhook.is_active ? (
                            <Badge className="bg-green-100 text-green-800 border-green-200">
                              <Power className="h-3 w-3 mr-1" />
                              Active
                            </Badge>
                          ) : (
                            <Badge className="bg-gray-100 text-gray-800 border-gray-200">
                              <PowerOff className="h-3 w-3 mr-1" />
                              Inactive
                            </Badge>
                          )}
                        </TableCell>
                        <TableCell>
                          <div className="flex items-center gap-2">
                            <div className="flex-1 h-2 bg-gray-200 rounded-full overflow-hidden w-20">
                              <div
                                className={`h-full rounded-full ${
                                  successRate >= 90
                                    ? 'bg-green-500'
                                    : successRate >= 75
                                      ? 'bg-blue-500'
                                      : successRate >= 50
                                        ? 'bg-yellow-500'
                                        : 'bg-red-500'
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
                          {webhook.last_triggered_at ? (
                            <div className="flex items-center gap-1 text-sm text-muted-foreground">
                              <Clock className="h-3 w-3" />
                              {formatDistanceToNow(new Date(webhook.last_triggered_at), {
                                addSuffix: true,
                              })}
                            </div>
                          ) : (
                            <span className="text-sm text-muted-foreground">Never</span>
                          )}
                        </TableCell>
                        <TableCell className="text-right">
                          <DropdownMenu>
                            <DropdownMenuTrigger asChild>
                              <Button variant="ghost" size="sm">
                                <MoreVertical className="h-4 w-4" />
                              </Button>
                            </DropdownMenuTrigger>
                            <DropdownMenuContent align="end">
                              <DropdownMenuItem onClick={() => handleViewDetails(webhook)}>
                                <Eye className="h-4 w-4 mr-2" />
                                View Details
                              </DropdownMenuItem>
                              <DropdownMenuItem
                                onClick={() => handleTestWebhook(webhook.id)}
                                disabled={testingWebhookId === webhook.id}
                              >
                                <TestTube className="h-4 w-4 mr-2" />
                                {testingWebhookId === webhook.id ? 'Testing...' : 'Test Webhook'}
                              </DropdownMenuItem>
                              <DropdownMenuItem
                                onClick={() => handleToggleWebhook(webhook)}
                                disabled={togglingWebhookId === webhook.id}
                              >
                                {webhook.is_active ? (
                                  <>
                                    <PowerOff className="h-4 w-4 mr-2" />
                                    Disable
                                  </>
                                ) : (
                                  <>
                                    <Power className="h-4 w-4 mr-2" />
                                    Enable
                                  </>
                                )}
                              </DropdownMenuItem>
                              <DropdownMenuSeparator />
                              <DropdownMenuItem
                                onClick={() => setDeleteWebhookId(webhook.id)}
                                className="text-red-600"
                              >
                                <Trash2 className="h-4 w-4 mr-2" />
                                Delete
                              </DropdownMenuItem>
                            </DropdownMenuContent>
                          </DropdownMenu>
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
            <AlertDialogTitle>Delete Webhook</AlertDialogTitle>
            <AlertDialogDescription>
              Are you sure you want to delete this webhook? This action cannot be undone, and the
              webhook will stop receiving events immediately.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleDeleteWebhook}
              className="bg-red-600 hover:bg-red-700"
            >
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </AuthGuard>
  );
}

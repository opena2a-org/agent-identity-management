'use client';

import { useState, useEffect } from 'react';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { Skeleton } from '@/components/ui/skeleton';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
  CheckCircle,
  XCircle,
  Clock,
  ExternalLink,
  Copy,
  Eye,
  EyeOff,
  RefreshCw,
  AlertCircle,
} from 'lucide-react';
import { api } from '@/lib/api';
import { formatDistanceToNow } from 'date-fns';
import { useToast } from '@/hooks/use-toast';

interface WebhookDetailModalProps {
  isOpen: boolean;
  webhookId: string;
  onClose: () => void;
  onRefresh: () => void;
}

interface WebhookDetail {
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
  deliveries: Array<{
    id: string;
    event: string;
    status: 'success' | 'failure';
    responseCode: number;
    timestamp: string;
    errorMessage?: string;
  }>;
}

export function WebhookDetailModal({
  isOpen,
  webhookId,
  onClose,
  onRefresh,
}: WebhookDetailModalProps) {
  const [webhook, setWebhook] = useState<WebhookDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showSecret, setShowSecret] = useState(false);
  const { toast } = useToast();

  const fetchWebhookDetails = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.getWebhook(webhookId);
      setWebhook(data as WebhookDetail);
    } catch (err: any) {
      console.error('Failed to fetch webhook details:', err);
      setError(err.message || 'Failed to load webhook details');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (isOpen && webhookId) {
      fetchWebhookDetails();
    }
  }, [isOpen, webhookId]);

  const handleCopySecret = () => {
    if (webhook) {
      navigator.clipboard.writeText(webhook.secret);
      toast({
        title: 'Secret copied',
        description: 'Webhook secret has been copied to clipboard',
      });
    }
  };

  const handleCopyUrl = () => {
    if (webhook) {
      navigator.clipboard.writeText(webhook.url);
      toast({
        title: 'URL copied',
        description: 'Webhook URL has been copied to clipboard',
      });
    }
  };

  const getSuccessRate = () => {
    if (!webhook) return 0;
    const total = webhook.successCount + webhook.failureCount;
    if (total === 0) return 0;
    return (webhook.successCount / total) * 100;
  };

  if (loading) {
    return (
      <Dialog open={isOpen} onOpenChange={onClose}>
        <DialogContent className="max-w-4xl max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>Webhook Details</DialogTitle>
            <DialogDescription>Loading webhook information...</DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <Skeleton className="h-32" />
            <Skeleton className="h-64" />
          </div>
        </DialogContent>
      </Dialog>
    );
  }

  if (error || !webhook) {
    return (
      <Dialog open={isOpen} onOpenChange={onClose}>
        <DialogContent className="max-w-4xl">
          <DialogHeader>
            <DialogTitle>Webhook Details</DialogTitle>
          </DialogHeader>
          <div className="text-center py-12">
            <AlertCircle className="h-16 w-16 mx-auto mb-4 text-muted-foreground" />
            <p className="text-muted-foreground">{error || 'Failed to load webhook details'}</p>
            <Button onClick={fetchWebhookDetails} className="mt-4">
              <RefreshCw className="h-4 w-4 mr-2" />
              Try Again
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    );
  }

  return (
    <Dialog open={isOpen} onOpenChange={onClose}>
      <DialogContent className="max-w-4xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{webhook.name}</DialogTitle>
          <DialogDescription>
            Webhook configuration and delivery history
          </DialogDescription>
        </DialogHeader>

        <Tabs defaultValue="overview" className="py-4">
          <TabsList className="grid w-full grid-cols-3">
            <TabsTrigger value="overview">Overview</TabsTrigger>
            <TabsTrigger value="events">Events</TabsTrigger>
            <TabsTrigger value="deliveries">
              Delivery History ({webhook.deliveries?.length || 0})
            </TabsTrigger>
          </TabsList>

          {/* Overview Tab */}
          <TabsContent value="overview" className="space-y-4">
            {/* Status Card */}
            <Card>
              <CardHeader>
                <CardTitle className="text-lg">Status & Performance</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
                  <div className="space-y-1">
                    <div className="text-sm text-muted-foreground">Status</div>
                    {webhook.isActive ? (
                      <Badge className="bg-green-100 text-green-800 border-green-200">
                        Active
                      </Badge>
                    ) : (
                      <Badge className="bg-gray-100 text-gray-800 border-gray-200">
                        Inactive
                      </Badge>
                    )}
                  </div>

                  <div className="space-y-1">
                    <div className="text-sm text-muted-foreground">Success Rate</div>
                    <div className="text-2xl font-bold">{getSuccessRate().toFixed(1)}%</div>
                  </div>

                  <div className="space-y-1">
                    <div className="text-sm text-muted-foreground">Successful Deliveries</div>
                    <div className="text-2xl font-bold text-green-600">
                      {webhook.successCount}
                    </div>
                  </div>

                  <div className="space-y-1">
                    <div className="text-sm text-muted-foreground">Failed Deliveries</div>
                    <div className="text-2xl font-bold text-red-600">
                      {webhook.failureCount}
                    </div>
                  </div>
                </div>
              </CardContent>
            </Card>

            {/* Configuration Card */}
            <Card>
              <CardHeader>
                <CardTitle className="text-lg">Configuration</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-2">
                  <Label>Webhook URL</Label>
                  <div className="flex items-center gap-2">
                    <code className="flex-1 bg-muted px-3 py-2 rounded text-sm">
                      {webhook.url}
                    </code>
                    <Button variant="outline" size="sm" onClick={handleCopyUrl}>
                      <Copy className="h-4 w-4" />
                    </Button>
                    <a
                      href={webhook.url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="p-2 hover:bg-gray-100 rounded"
                    >
                      <ExternalLink className="h-4 w-4" />
                    </a>
                  </div>
                </div>

                <div className="space-y-2">
                  <Label>Webhook Secret</Label>
                  <div className="flex items-center gap-2">
                    <code className="flex-1 bg-muted px-3 py-2 rounded text-sm font-mono">
                      {showSecret ? webhook.secret : '•'.repeat(40)}
                    </code>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => setShowSecret(!showSecret)}
                    >
                      {showSecret ? (
                        <EyeOff className="h-4 w-4" />
                      ) : (
                        <Eye className="h-4 w-4" />
                      )}
                    </Button>
                    <Button variant="outline" size="sm" onClick={handleCopySecret}>
                      <Copy className="h-4 w-4" />
                    </Button>
                  </div>
                  <p className="text-xs text-muted-foreground">
                    Use this secret to verify webhook signatures in your endpoint
                  </p>
                </div>

                <div className="space-y-2">
                  <Label>Created</Label>
                  <p className="text-sm text-muted-foreground">
                    {new Date(webhook.createdAt).toLocaleString()} (
                    {formatDistanceToNow(new Date(webhook.createdAt), { addSuffix: true })})
                  </p>
                </div>

                {webhook.lastTriggeredAt && (
                  <div className="space-y-2">
                    <Label>Last Triggered</Label>
                    <p className="text-sm text-muted-foreground">
                      {new Date(webhook.lastTriggeredAt).toLocaleString()} (
                      {formatDistanceToNow(new Date(webhook.lastTriggeredAt), {
                        addSuffix: true,
                      })}
                      )
                    </p>
                  </div>
                )}
              </CardContent>
            </Card>
          </TabsContent>

          {/* Events Tab */}
          <TabsContent value="events">
            <Card>
              <CardHeader>
                <CardTitle className="text-lg">Subscribed Events</CardTitle>
                <CardDescription>
                  Events that will trigger this webhook ({webhook.events.length} events)
                </CardDescription>
              </CardHeader>
              <CardContent>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                  {webhook.events.map((event) => (
                    <div
                      key={event}
                      className="flex items-center gap-2 p-3 bg-blue-50 border border-blue-200 rounded-lg"
                    >
                      <CheckCircle className="h-4 w-4 text-blue-600 flex-shrink-0" />
                      <code className="text-sm font-mono">{event}</code>
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          {/* Deliveries Tab */}
          <TabsContent value="deliveries">
            <Card>
              <CardHeader>
                <div className="flex items-center justify-between">
                  <div>
                    <CardTitle className="text-lg">Delivery History</CardTitle>
                    <CardDescription>
                      Recent webhook delivery attempts and their outcomes
                    </CardDescription>
                  </div>
                  <Button variant="outline" size="sm" onClick={fetchWebhookDetails}>
                    <RefreshCw className="h-4 w-4 mr-2" />
                    Refresh
                  </Button>
                </div>
              </CardHeader>
              <CardContent>
                {(webhook.deliveries?.length || 0) === 0 ? (
                  <div className="text-center py-12">
                    <Clock className="h-16 w-16 mx-auto mb-4 text-muted-foreground" />
                    <p className="text-muted-foreground">No deliveries yet</p>
                    <p className="text-sm text-muted-foreground mt-1">
                      Deliveries will appear here once events are triggered
                    </p>
                  </div>
                ) : (
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Event</TableHead>
                        <TableHead>Status</TableHead>
                        <TableHead>Response Code</TableHead>
                        <TableHead>Timestamp</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {webhook.deliveries?.map((delivery) => (
                        <TableRow key={delivery.id}>
                          <TableCell>
                            <code className="text-xs bg-muted px-2 py-1 rounded">
                              {delivery.event}
                            </code>
                          </TableCell>
                          <TableCell>
                            {delivery.status === 'success' ? (
                              <Badge className="bg-green-100 text-green-800 border-green-200">
                                <CheckCircle className="h-3 w-3 mr-1" />
                                Success
                              </Badge>
                            ) : (
                              <Badge className="bg-red-100 text-red-800 border-red-200">
                                <XCircle className="h-3 w-3 mr-1" />
                                Failed
                              </Badge>
                            )}
                          </TableCell>
                          <TableCell>
                            <span
                              className={`font-mono text-sm ${
                                delivery.responseCode >= 200 && delivery.responseCode < 300
                                  ? 'text-green-600'
                                  : 'text-red-600'
                              }`}
                            >
                              {delivery.responseCode}
                            </span>
                          </TableCell>
                          <TableCell className="text-sm text-muted-foreground">
                            {formatDistanceToNow(new Date(delivery.timestamp), {
                              addSuffix: true,
                            })}
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                )}
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>
      </DialogContent>
    </Dialog>
  );
}

function Label({ children }: { children: React.ReactNode }) {
  return <div className="text-sm font-semibold text-gray-700 mb-1">{children}</div>;
}

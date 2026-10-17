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
import { Skeleton } from '@/components/ui/skeleton';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
  CheckCircle,
  ExternalLink,
  Copy,
  RefreshCw,
  AlertCircle,
  Loader2,
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

// The webhook as GET /api/v1/webhooks/:id answers it. The answer never carries
// the signing secret.
interface WebhookDetail {
  id: string;
  organizationId: string;
  name: string;
  url: string;
  events: string[];
  isActive: boolean;
  createdAt: string;
  lastTriggered?: string;
  successCount: number;
  failureCount: number;
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
  const [confirmRotate, setConfirmRotate] = useState(false);
  const [rotating, setRotating] = useState(false);
  const [rotateError, setRotateError] = useState<string | null>(null);
  // The secret the rotation answered with. It is shown once, here, and the
  // server does not return it again.
  const [rotatedSecret, setRotatedSecret] = useState<string | null>(null);
  const { toast } = useToast();

  const fetchWebhookDetails = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.getWebhook(webhookId);
      setWebhook(data);
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

  const handleRotateSecret = async () => {
    setRotating(true);
    setRotateError(null);
    try {
      // The list is not reloaded here: the page replaces itself, and this
      // modal with it, while it loads, and the new secret would be lost.
      const rotation = await api.rotateWebhookSecret(webhookId);
      setRotatedSecret(rotation.secret);
      setConfirmRotate(false);
    } catch (err: any) {
      console.error('Failed to rotate webhook secret:', err);
      setRotateError(err.message || 'Failed to rotate the webhook secret');
    } finally {
      setRotating(false);
    }
  };

  const handleCopySecret = async () => {
    if (!rotatedSecret) return;
    try {
      await navigator.clipboard.writeText(rotatedSecret);
      toast({
        title: 'Secret copied',
        description: 'Webhook secret has been copied to clipboard',
      });
    } catch (err) {
      console.error('Failed to copy webhook secret:', err);
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
      {/* minmax(0,1fr): the column stays the dialog's width instead of growing to its widest descendant. */}
      <DialogContent className="max-w-4xl max-h-[90vh] overflow-y-auto grid-cols-[minmax(0,1fr)]">
        <DialogHeader>
          <DialogTitle>{webhook.name}</DialogTitle>
          <DialogDescription>
            Webhook configuration and subscribed events
          </DialogDescription>
        </DialogHeader>

        <Tabs defaultValue="overview" className="py-4">
          <TabsList className="grid w-full grid-cols-2">
            <TabsTrigger value="overview">Overview</TabsTrigger>
            <TabsTrigger value="events">Events</TabsTrigger>
          </TabsList>

          {/* Overview Tab */}
          <TabsContent value="overview" className="space-y-4">
            {/* Status Card */}
            <Card>
              <CardHeader>
                <CardTitle className="text-xl">Status & Performance</CardTitle>
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
                    <div className="text-2xl font-bold">{Math.round(getSuccessRate())}%</div>
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
                <CardTitle className="text-xl">Configuration</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-2">
                  <Label>Webhook URL</Label>
                  <div className="flex items-center gap-2">
                    <code className="flex-1 min-w-0 break-all bg-muted px-3 py-2 rounded text-sm">
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
                  {rotatedSecret !== null ? (
                    <>
                      <div className="flex items-center gap-2">
                        <code
                          data-testid="webhook-rotated-secret"
                          className="flex-1 break-all bg-muted px-3 py-2 rounded text-sm font-mono"
                        >
                          {rotatedSecret}
                        </code>
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={handleCopySecret}
                          aria-label="Copy secret"
                        >
                          <Copy className="h-4 w-4" />
                        </Button>
                      </div>
                      <p className="text-xs text-muted-foreground">
                        This secret will not be shown again. Deliveries queued from now on are
                        signed with it.
                      </p>
                    </>
                  ) : confirmRotate ? (
                    <div className="space-y-2">
                      <p className="text-sm text-muted-foreground">
                        Rotating replaces the secret at once. Deliveries queued after it are signed
                        with the new secret, so your endpoint must switch to it to keep verifying
                        them.
                      </p>
                      <div className="flex items-center gap-2">
                        <Button size="sm" onClick={handleRotateSecret} disabled={rotating}>
                          {rotating && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
                          Confirm rotation
                        </Button>
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => setConfirmRotate(false)}
                          disabled={rotating}
                        >
                          Cancel
                        </Button>
                      </div>
                    </div>
                  ) : (
                    <div className="flex items-center justify-between gap-2">
                      <p className="text-xs text-muted-foreground">
                        The secret is shown only when the webhook is created or its secret is
                        rotated.
                      </p>
                      <Button variant="outline" size="sm" onClick={() => setConfirmRotate(true)}>
                        <RefreshCw className="h-4 w-4 mr-2" />
                        Rotate secret
                      </Button>
                    </div>
                  )}
                  {rotateError && <p className="text-sm text-red-600">{rotateError}</p>}
                </div>

                <div className="space-y-2">
                  <Label>Created</Label>
                  <p className="text-sm text-muted-foreground">
                    {new Date(webhook.createdAt).toLocaleString()} (
                    {formatDistanceToNow(new Date(webhook.createdAt), { addSuffix: true })})
                  </p>
                </div>

                {webhook.lastTriggered && (
                  <div className="space-y-2">
                    <Label>Last Triggered</Label>
                    <p className="text-sm text-muted-foreground">
                      {new Date(webhook.lastTriggered).toLocaleString()} (
                      {formatDistanceToNow(new Date(webhook.lastTriggered), {
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
                <CardTitle className="text-xl">Subscribed Events</CardTitle>
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
        </Tabs>
      </DialogContent>
    </Dialog>
  );
}

function Label({ children }: { children: React.ReactNode }) {
  return <div className="text-sm font-semibold text-gray-700 mb-1">{children}</div>;
}

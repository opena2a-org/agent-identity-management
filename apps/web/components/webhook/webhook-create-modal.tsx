'use client';

import { useState } from 'react';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { Checkbox } from '@/components/ui/checkbox';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Loader2, AlertCircle } from 'lucide-react';
import { api } from '@/lib/api';

interface WebhookCreateModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

const AVAILABLE_EVENTS = [
  { id: 'agent.created', label: 'Agent Created', description: 'Triggered when a new agent is registered' },
  { id: 'agent.updated', label: 'Agent Updated', description: 'Triggered when an agent is modified' },
  { id: 'agent.deleted', label: 'Agent Deleted', description: 'Triggered when an agent is deleted' },
  { id: 'agent.verified', label: 'Agent Verified', description: 'Triggered when an agent is verified' },
  { id: 'agent.suspended', label: 'Agent Suspended', description: 'Triggered when an agent is suspended' },
  { id: 'agent.reactivated', label: 'Agent Reactivated', description: 'Triggered when an agent is reactivated' },
  { id: 'trust_score.changed', label: 'Trust Score Changed', description: 'Triggered when trust score changes significantly' },
  { id: 'trust_score.critical', label: 'Trust Score Critical', description: 'Triggered when trust score drops below threshold' },
  { id: 'alert.created', label: 'Alert Created', description: 'Triggered when a new security alert is generated' },
  { id: 'alert.acknowledged', label: 'Alert Acknowledged', description: 'Triggered when an alert is acknowledged' },
  { id: 'alert.resolved', label: 'Alert Resolved', description: 'Triggered when an alert is resolved' },
  { id: 'api_key.created', label: 'API Key Created', description: 'Triggered when a new API key is generated' },
  { id: 'api_key.revoked', label: 'API Key Revoked', description: 'Triggered when an API key is revoked' },
  { id: 'api_key.expired', label: 'API Key Expired', description: 'Triggered when an API key expires' },
  { id: 'verification.failed', label: 'Verification Failed', description: 'Triggered when agent verification fails' },
  { id: 'compliance.violation', label: 'Compliance Violation', description: 'Triggered when a compliance rule is violated' },
];

export function WebhookCreateModal({ isOpen, onClose, onSuccess }: WebhookCreateModalProps) {
  const [name, setName] = useState('');
  const [url, setUrl] = useState('');
  const [secret, setSecret] = useState('');
  const [selectedEvents, setSelectedEvents] = useState<string[]>([]);
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleEventToggle = (eventId: string) => {
    setSelectedEvents((prev) =>
      prev.includes(eventId) ? prev.filter((e) => e !== eventId) : [...prev, eventId]
    );
  };

  const handleSelectAll = () => {
    if (selectedEvents.length === AVAILABLE_EVENTS.length) {
      setSelectedEvents([]);
    } else {
      setSelectedEvents(AVAILABLE_EVENTS.map((e) => e.id));
    }
  };

  const handleCreate = async () => {
    setError(null);

    // Validation
    if (!name.trim()) {
      setError('Webhook name is required');
      return;
    }
    if (!url.trim()) {
      setError('Webhook URL is required');
      return;
    }
    if (!url.startsWith('http://') && !url.startsWith('https://')) {
      setError('Webhook URL must start with http:// or https://');
      return;
    }
    if (selectedEvents.length === 0) {
      setError('Please select at least one event');
      return;
    }

    setCreating(true);

    try {
      await api.createWebhook({
        name: name.trim(),
        url: url.trim(),
        events: selectedEvents,
        secret: secret.trim() || undefined,
      });
      onSuccess();
      handleClose();
    } catch (err: any) {
      console.error('Failed to create webhook:', err);
      setError(err.message || 'Failed to create webhook');
    } finally {
      setCreating(false);
    }
  };

  const handleClose = () => {
    setName('');
    setUrl('');
    setSecret('');
    setSelectedEvents([]);
    setError(null);
    onClose();
  };

  return (
    <Dialog open={isOpen} onOpenChange={handleClose}>
      <DialogContent className="max-w-3xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Create Webhook</DialogTitle>
          <DialogDescription>
            Configure a webhook endpoint to receive real-time event notifications
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-6 py-4">
          {/* Error Alert */}
          {error && (
            <div className="flex items-center gap-2 p-3 bg-red-50 border border-red-200 rounded-lg text-red-800">
              <AlertCircle className="h-5 w-5 flex-shrink-0" />
              <p className="text-sm">{error}</p>
            </div>
          )}

          {/* Basic Information */}
          <div className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="name">Webhook Name *</Label>
              <Input
                id="name"
                placeholder="Production API Webhook"
                value={name}
                onChange={(e) => setName(e.target.value)}
                disabled={creating}
              />
              <p className="text-xs text-muted-foreground">
                A friendly name to identify this webhook
              </p>
            </div>

            <div className="space-y-2">
              <Label htmlFor="url">Webhook URL *</Label>
              <Input
                id="url"
                type="url"
                placeholder="https://api.example.com/webhooks/aim"
                value={url}
                onChange={(e) => setUrl(e.target.value)}
                disabled={creating}
              />
              <p className="text-xs text-muted-foreground">
                The HTTPS endpoint where events will be sent
              </p>
            </div>

            <div className="space-y-2">
              <Label htmlFor="secret">Webhook Secret (Optional)</Label>
              <Input
                id="secret"
                type="password"
                placeholder="Enter a secret for HMAC signature verification"
                value={secret}
                onChange={(e) => setSecret(e.target.value)}
                disabled={creating}
              />
              <p className="text-xs text-muted-foreground">
                Used to generate HMAC signatures for request verification. If not provided, one
                will be generated automatically.
              </p>
            </div>
          </div>

          {/* Event Selection */}
          <Card>
            <CardHeader>
              <div className="flex items-center justify-between">
                <div>
                  <CardTitle className="text-lg">Event Selection *</CardTitle>
                  <CardDescription>
                    Choose which events should trigger this webhook
                  </CardDescription>
                </div>
                <Button variant="outline" size="sm" onClick={handleSelectAll} disabled={creating}>
                  {selectedEvents.length === AVAILABLE_EVENTS.length ? 'Deselect All' : 'Select All'}
                </Button>
              </div>
            </CardHeader>
            <CardContent>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                {AVAILABLE_EVENTS.map((event) => (
                  <div
                    key={event.id}
                    className={`flex items-start gap-3 p-3 rounded-lg border transition-colors ${
                      selectedEvents.includes(event.id)
                        ? 'bg-blue-50 border-blue-200'
                        : 'bg-white border-gray-200 hover:bg-gray-50'
                    }`}
                  >
                    <Checkbox
                      id={event.id}
                      checked={selectedEvents.includes(event.id)}
                      onCheckedChange={() => handleEventToggle(event.id)}
                      disabled={creating}
                      className="mt-1"
                    />
                    <div className="flex-1 min-w-0">
                      <Label
                        htmlFor={event.id}
                        className="text-sm font-medium cursor-pointer"
                      >
                        {event.label}
                      </Label>
                      <p className="text-xs text-muted-foreground mt-0.5">
                        {event.description}
                      </p>
                    </div>
                  </div>
                ))}
              </div>

              {selectedEvents.length > 0 && (
                <div className="mt-4 p-3 bg-blue-50 border border-blue-200 rounded-lg">
                  <p className="text-sm text-blue-800">
                    <strong>{selectedEvents.length}</strong> event
                    {selectedEvents.length === 1 ? '' : 's'} selected
                  </p>
                </div>
              )}
            </CardContent>
          </Card>

          {/* Payload Example */}
          <Card>
            <CardHeader>
              <CardTitle className="text-lg">Webhook Payload Example</CardTitle>
              <CardDescription>
                Your endpoint will receive POST requests with this structure
              </CardDescription>
            </CardHeader>
            <CardContent>
              <pre className="bg-gray-900 text-gray-100 p-4 rounded-lg text-xs overflow-x-auto">
                {`{
  "event": "agent.created",
  "timestamp": "2025-01-22T15:30:00Z",
  "webhook_id": "webhook-uuid",
  "data": {
    "id": "agent-uuid",
    "name": "Example Agent",
    "type": "ai_agent",
    "organization_id": "org-uuid",
    "trust_score": 85.5,
    "status": "pending"
  }
}`}
              </pre>
              <p className="text-xs text-muted-foreground mt-2">
                All requests include an <code className="bg-muted px-1 rounded">X-Webhook-Signature</code>{' '}
                header for verification
              </p>
            </CardContent>
          </Card>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={handleClose} disabled={creating}>
            Cancel
          </Button>
          <Button onClick={handleCreate} disabled={creating}>
            {creating ? (
              <>
                <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                Creating...
              </>
            ) : (
              'Create Webhook'
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

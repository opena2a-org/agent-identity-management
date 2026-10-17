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
import { Checkbox } from '@/components/ui/checkbox';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Loader2, AlertCircle, Copy, Check } from 'lucide-react';
import { api } from '@/lib/api';

interface WebhookCreateModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

// The events the server sends, with when it sends each, as docs/API.md
// § Webhook Events lists them. A webhook subscribed only to a name outside
// this list receives nothing.
export const AVAILABLE_EVENTS = [
  { id: 'agent.created', label: 'Agent Created', description: 'An agent is registered.' },
  {
    id: 'agent.verified',
    label: 'Agent Verified',
    description: 'An agent is verified through POST /api/v1/agents/:id/verify, or at registration.',
  },
  {
    id: 'agent.updated',
    label: 'Agent Updated',
    description: 'An agent is updated through PUT /api/v1/agents/:id.',
  },
  {
    id: 'agent.suspended',
    label: 'Agent Suspended',
    description:
      'An agent is suspended through POST /api/v1/agents/:id/suspend, or by the trust score policy (see trust_score.low).',
  },
  {
    id: 'agent.deleted',
    label: 'Agent Deleted',
    description: 'An agent is deleted through DELETE /api/v1/agents/:id.',
  },
  {
    id: 'trust_score.changed',
    label: 'Trust Score Changed',
    description:
      "An administrator sets an agent's trust score through PUT /api/v1/agents/:id/trust-score, or a capability violation lowers it, and the stored score differs from the one before. Other trust score changes, including POST /api/v1/agents/:id/trust-score/recalculate, send no event.",
  },
  {
    id: 'trust_score.low',
    label: 'Trust Score Low',
    description:
      "After one of the changes trust_score.changed names, the trust score policy finds that a verified or pending agent's score fell and ends below 70%. severity is critical below 50%, where the policy also suspends the agent, and warning otherwise.",
  },
  {
    id: 'alert.created',
    label: 'Alert Created',
    description:
      'An alert is raised for a capability check denied because the agent has no granted capabilities. Other alerts send no event.',
  },
  {
    id: 'alert.acknowledged',
    label: 'Alert Acknowledged',
    description:
      'An alert is acknowledged through POST /api/v1/admin/alerts/:id/acknowledge. Acknowledging every alert at once, and resolving an alert, send no event.',
  },
  {
    id: 'api_key.created',
    label: 'API Key Created',
    description: 'An API key is created, including the key created with an agent.',
  },
  {
    id: 'api_key.revoked',
    label: 'API Key Revoked',
    description: 'An active API key is disabled through PATCH /api/v1/api-keys/:id/disable.',
  },
];

// The fewest characters the server takes in a chosen secret.
const MIN_SECRET_LENGTH = 32;

export function WebhookCreateModal({ isOpen, onClose, onSuccess }: WebhookCreateModalProps) {
  const [name, setName] = useState('');
  const [url, setUrl] = useState('');
  const [secret, setSecret] = useState('');
  const [selectedEvents, setSelectedEvents] = useState<string[]>([]);
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // The secret the create answered with. It is shown once, here, and the
  // server does not return it again.
  const [createdSecret, setCreatedSecret] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

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
    const chosenSecret = secret.trim();
    if (chosenSecret && Array.from(chosenSecret).length < MIN_SECRET_LENGTH) {
      setError(`Webhook secret must be at least ${MIN_SECRET_LENGTH} characters`);
      return;
    }

    setCreating(true);

    try {
      const created = await api.createWebhook({
        name: name.trim(),
        url: url.trim(),
        events: selectedEvents,
        secret: chosenSecret || undefined,
      });
      setCreatedSecret(created.secret);
    } catch (err: any) {
      console.error('Failed to create webhook:', err);
      setError(err.message || 'Failed to create webhook');
    } finally {
      setCreating(false);
    }
  };

  const reset = () => {
    setName('');
    setUrl('');
    setSecret('');
    setSelectedEvents([]);
    setError(null);
    setCreatedSecret(null);
    setCopied(false);
  };

  // Closing after a create tells the page it succeeded, so it lists the new
  // webhook; closing before one is a cancel.
  const handleClose = () => {
    const created = createdSecret !== null;
    reset();
    if (created) {
      onSuccess();
    } else {
      onClose();
    }
  };

  const handleCopySecret = async () => {
    if (!createdSecret) return;
    try {
      await navigator.clipboard.writeText(createdSecret);
      setCopied(true);
    } catch (err) {
      console.error('Failed to copy webhook secret:', err);
    }
  };

  if (createdSecret !== null) {
    return (
      <Dialog open={isOpen} onOpenChange={handleClose}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>Webhook created</DialogTitle>
            <DialogDescription>
              Deliveries to this webhook are signed with the secret below.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-2 py-4">
            <Label htmlFor="created-secret">Webhook secret</Label>
            <div className="flex items-center gap-2">
              <code
                id="created-secret"
                data-testid="webhook-created-secret"
                className="flex-1 break-all bg-muted px-3 py-2 rounded text-sm font-mono"
              >
                {createdSecret}
              </code>
              <Button variant="outline" size="sm" onClick={handleCopySecret} aria-label="Copy secret">
                {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                <span className="ml-2">{copied ? 'Copied' : 'Copy'}</span>
              </Button>
            </div>
            <p className="text-sm text-muted-foreground">
              This secret will not be shown again. Store it with your receiver now; if it is lost,
              rotate the secret from the webhook&apos;s details.
            </p>
          </div>

          <DialogFooter>
            <Button onClick={handleClose}>Done</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    );
  }

  return (
    <Dialog open={isOpen} onOpenChange={handleClose}>
      {/* minmax(0,1fr): the column stays the dialog's width instead of growing to the payload example's longest line. */}
      <DialogContent className="max-w-3xl max-h-[90vh] overflow-y-auto grid-cols-[minmax(0,1fr)]">
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
                Used to generate HMAC signatures for request verification, {MIN_SECRET_LENGTH} to 255
                characters. If not provided, one will be generated automatically. Either way it is
                shown once, after the webhook is created.
              </p>
            </div>
          </div>

          {/* Event Selection */}
          <Card>
            <CardHeader>
              <div className="flex items-center justify-between">
                <div>
                  <CardTitle className="text-xl">Event Selection *</CardTitle>
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
              <CardTitle className="text-xl">Webhook Payload Example</CardTitle>
              <CardDescription>
                Your endpoint will receive POST requests with this structure
              </CardDescription>
            </CardHeader>
            <CardContent>
              <pre className="bg-gray-900 text-gray-100 p-4 rounded-lg text-xs overflow-x-auto">
                {`{
  "id": "0c6f3c1e-8f0e-4a63-9a43-2f1d8b7e5a10",
  "event": "agent.verified",
  "timestamp": "2026-10-11T00:00:00Z",
  "organizationId": "7d9f2b4a-1c3e-4f5a-8b6c-9d0e1f2a3b4c",
  "data": {
    "agentId": "550e8400-e29b-41d4-a716-446655440000",
    "name": "billing-agent",
    "displayName": "Billing Agent",
    "agentType": "ai_agent",
    "status": "verified",
    "trustScore": 0.755
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

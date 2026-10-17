'use client';

import { useCallback, useEffect, useState } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
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
import { AlertCircle, BellRing, Edit, Plus, Power, PowerOff, Send, Trash2 } from 'lucide-react';
import { api, type AlertChannel, type AlertChannelKind, type AlertChannelTestResult } from '@/lib/api';
import { AuthGuard } from '@/components/auth-guard';
import { AlertChannelModal, ALERT_CHANNEL_KINDS } from '@/components/alert-channel/alert-channel-modal';
import { useToast } from '@/hooks/use-toast';
import { formatDistanceToNow } from 'date-fns';

const KIND_LABELS = Object.fromEntries(ALERT_CHANNEL_KINDS.map((k) => [k.value, k.label])) as Record<
  AlertChannelKind,
  string
>;

// One column below lg. From lg the list is one grid of these columns, and the
// header and each channel are subgrids of it, so every row shares the column
// widths: the state and actions columns take their widest content, and
// "Inactive" never runs into the last delivery.
const LIST_COLUMNS =
  'lg:grid-cols-[minmax(0,1.3fr)_minmax(0,1.5fr)_minmax(0,1fr)_max-content_minmax(0,1.2fr)_max-content]';
const LIST_ROW = 'lg:col-span-6 lg:grid-cols-subgrid lg:gap-x-4';

type TestOutcome = { result: AlertChannelTestResult } | { error: string };

function lastDeliveryText(channel: AlertChannel): string {
  if (!channel.lastDeliveryAt) {
    return 'never';
  }
  const status = channel.lastDeliveryStatus === 'failed' ? 'Failed' : 'Delivered';
  const code = channel.lastDeliveryCode != null ? ` (${channel.lastDeliveryCode})` : '';
  return `${status}${code}, ${formatDistanceToNow(new Date(channel.lastDeliveryAt), { addSuffix: true })}`;
}

// What a test answered, in words. A failure names the class the server
// returned, never the destination, and what to do next.
function testOutcomeText(channel: AlertChannel, outcome: TestOutcome): { ok: boolean; text: string } {
  if ('error' in outcome) {
    return { ok: false, text: `Test not sent: ${outcome.error}` };
  }
  const { delivered, statusCode, error } = outcome.result;
  if (delivered) {
    return {
      ok: true,
      text: statusCode != null ? `Delivered (${statusCode})` : 'Delivered to every recipient',
    };
  }
  const failure = error || (statusCode != null ? `status ${statusCode}` : 'no answer');
  const next =
    channel.kind === 'email'
      ? "check the recipients and this server's email service, then test again"
      : 'check the URL or key, then test again';
  return { ok: false, text: `Not delivered (${failure}): ${next}.` };
}

function Destination({ channel }: { channel: AlertChannel }) {
  if (channel.kind === 'email') {
    return (
      <ul className="space-y-0.5">
        {(channel.config.recipients ?? []).map((recipient) => (
          <li key={recipient} className="text-sm text-ink break-words">
            {recipient}
          </li>
        ))}
      </ul>
    );
  }
  const value = channel.kind === 'pagerduty' ? channel.config.routingKey : channel.config.webhookUrl;
  return <code className="rounded bg-muted px-2 py-0.5 text-xs break-all">{value ?? ''}</code>;
}

function CellLabel({ children }: { children: string }) {
  return <span className="block text-xs font-medium text-ink-tertiary lg:sr-only">{children}</span>;
}

export default function AlertChannelsPage() {
  const [channels, setChannels] = useState<AlertChannel[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  // Open when not undefined: null adds a channel, a channel edits it.
  const [modalChannel, setModalChannel] = useState<AlertChannel | null | undefined>(undefined);
  const [deleteTarget, setDeleteTarget] = useState<AlertChannel | null>(null);
  const [pagerDutyTestTarget, setPagerDutyTestTarget] = useState<AlertChannel | null>(null);
  const [testingId, setTestingId] = useState<string | null>(null);
  const [testOutcomes, setTestOutcomes] = useState<Record<string, TestOutcome>>({});
  const { toast } = useToast();

  const fetchChannels = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.listAlertChannels();
      setChannels(data.alertChannels);
    } catch (err: any) {
      setError(err?.message || 'The alert channels were not loaded.');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchChannels();
  }, [fetchChannels]);

  const handleSaved = (saved: AlertChannel) => {
    const added = modalChannel === null;
    setModalChannel(undefined);
    toast({
      title: added ? 'Channel added' : 'Channel updated',
      description: `Alerts at or above ${saved.minSeverity} go to ${saved.name}.`,
    });
    fetchChannels();
  };

  const runTest = async (channel: AlertChannel) => {
    setTestingId(channel.id);
    try {
      const result = await api.testAlertChannel(channel.id);
      setTestOutcomes((prev) => ({ ...prev, [channel.id]: { result } }));
    } catch (err: any) {
      setTestOutcomes((prev) => ({
        ...prev,
        [channel.id]: { error: err?.message || 'the server did not answer' },
      }));
    } finally {
      setTestingId(null);
    }
    fetchChannels();
  };

  // A PagerDuty test opens an alert on the service, so it is asked first.
  const handleTest = (channel: AlertChannel) => {
    if (channel.kind === 'pagerduty') {
      setPagerDutyTestTarget(channel);
      return;
    }
    runTest(channel);
  };

  const handleDelete = async () => {
    const channel = deleteTarget;
    if (!channel) return;
    setDeleteTarget(null);
    try {
      await api.deleteAlertChannel(channel.id);
      setTestOutcomes((prev) => {
        const next = { ...prev };
        delete next[channel.id];
        return next;
      });
      toast({ title: 'Channel deleted', description: `Alerts no longer go to ${channel.name}.` });
      fetchChannels();
    } catch (err: any) {
      toast({
        title: 'Channel not deleted',
        description: err?.message || 'The server did not delete the channel.',
        variant: 'destructive',
      });
    }
  };

  if (loading && channels.length === 0) {
    return (
      <AuthGuard>
        <div className="space-y-6">
          <div className="space-y-2">
            <Skeleton className="h-8 w-48" />
            <Skeleton className="h-4 w-full max-w-md" />
          </div>
          <div className="glass p-6 space-y-3">
            {[...Array(3)].map((_, i) => (
              <Skeleton key={i} className="h-16 w-full" />
            ))}
          </div>
        </div>
      </AuthGuard>
    );
  }

  if (error) {
    return (
      <AuthGuard>
        <div className="py-16 text-center">
          <AlertCircle className="mx-auto mb-4 h-16 w-16 text-ink-tertiary" />
          <h2 className="mb-2 text-2xl font-semibold text-ink">Unable to load alert channels</h2>
          <p className="text-ink-secondary break-words">{error}</p>
          <Button onClick={fetchChannels} className="mt-4">
            Try again
          </Button>
        </div>
      </AuthGuard>
    );
  }

  return (
    <AuthGuard>
      <div className="space-y-6">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div className="min-w-0">
            <h1 className="text-2xl font-bold text-ink">Alert channels</h1>
            <p className="mt-1 text-sm text-ink-secondary">
              Send the organization&apos;s alerts to Slack, PagerDuty, Microsoft Teams or email, each at or
              above a severity floor.
            </p>
          </div>
          {channels.length > 0 && (
            <button
              onClick={() => setModalChannel(null)}
              className="flex flex-shrink-0 items-center justify-center gap-2 self-start rounded-pill bg-brand px-4 py-2 text-white shadow-glow transition-colors hover:bg-brand-hover sm:self-auto"
            >
              <Plus className="h-4 w-4" />
              Add channel
            </button>
          )}
        </div>

        <Card>
          <CardHeader>
            <CardTitle>Channels</CardTitle>
            <CardDescription>
              Each alert the server stores is sent to every active channel whose severity floor admits it.
            </CardDescription>
          </CardHeader>
          <CardContent>
            {channels.length === 0 ? (
              <div data-testid="alert-channels-empty" className="py-12 text-center">
                <BellRing className="mx-auto mb-4 h-16 w-16 text-muted-foreground" />
                <h3 className="mb-2 text-xl font-semibold">No alert channels</h3>
                <p className="mx-auto mb-4 max-w-xl text-muted-foreground">
                  An alert channel sends each alert at or above its severity floor to Slack, PagerDuty, a
                  Microsoft Teams workflow or a list of email addresses.
                </p>
                <Button onClick={() => setModalChannel(null)}>
                  <Plus className="mr-2 h-4 w-4" />
                  Add channel
                </Button>
              </div>
            ) : (
              <div className={`lg:grid lg:gap-x-4 ${LIST_COLUMNS}`}>
                <div
                  aria-hidden="true"
                  className={`hidden border-b border-divider pb-2 text-xs font-medium text-ink-secondary lg:grid ${LIST_ROW}`}
                >
                  <span>Channel</span>
                  <span>Destination</span>
                  <span>Severity floor</span>
                  <span>State</span>
                  <span>Last delivery</span>
                  <span className="text-right">Actions</span>
                </div>
                <ul className={`divide-y divide-divider lg:grid ${LIST_ROW}`}>
                  {channels.map((channel) => {
                    const outcome = testOutcomes[channel.id];
                    const shown = outcome ? testOutcomeText(channel, outcome) : null;
                    const testing = testingId === channel.id;
                    return (
                      <li
                        key={channel.id}
                        data-alert-channel-row
                        className={`grid gap-3 py-4 lg:items-center ${LIST_ROW}`}
                      >
                        <div className="min-w-0">
                          <p className="font-medium text-ink break-words">{channel.name}</p>
                          <p className="text-xs text-ink-secondary">{KIND_LABELS[channel.kind] ?? channel.kind}</p>
                        </div>
                        <div className="min-w-0">
                          <CellLabel>Destination</CellLabel>
                          <Destination channel={channel} />
                        </div>
                        <div className="min-w-0 text-sm text-ink">
                          <CellLabel>Severity floor</CellLabel>
                          {`alerts at or above ${channel.minSeverity}`}
                        </div>
                        <div className="min-w-0">
                          <CellLabel>State</CellLabel>
                          {channel.isActive ? (
                            <Badge className="border-success-border bg-success-fill text-success-text">
                              <Power className="mr-1 h-3 w-3" />
                              Active
                            </Badge>
                          ) : (
                            <Badge className="border-divider bg-glass-inset-gray text-ink-secondary">
                              <PowerOff className="mr-1 h-3 w-3" />
                              Inactive
                            </Badge>
                          )}
                        </div>
                        <div className="min-w-0 text-sm text-ink-secondary">
                          <CellLabel>Last delivery</CellLabel>
                          {lastDeliveryText(channel)}
                        </div>
                        <div className="flex flex-wrap gap-2 lg:justify-end">
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => handleTest(channel)}
                            disabled={testing}
                            aria-label={`Test ${channel.name}`}
                          >
                            <Send className="mr-1 h-3.5 w-3.5" />
                            {testing ? 'Testing' : 'Test'}
                          </Button>
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => setModalChannel(channel)}
                            aria-label={`Edit ${channel.name}`}
                          >
                            <Edit className="mr-1 h-3.5 w-3.5" />
                            Edit
                          </Button>
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => setDeleteTarget(channel)}
                            aria-label={`Delete ${channel.name}`}
                            className="text-danger-text"
                          >
                            <Trash2 className="mr-1 h-3.5 w-3.5" />
                            Delete
                          </Button>
                        </div>
                        {shown && (
                          <p
                            role="status"
                            data-testid="alert-channel-test-outcome"
                            className={`text-sm break-words lg:col-span-6 ${
                              shown.ok ? 'text-success-text' : 'text-danger-text'
                            }`}
                          >
                            {shown.text}
                          </p>
                        )}
                      </li>
                    );
                  })}
                </ul>
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      {modalChannel !== undefined && (
        <AlertChannelModal
          key={modalChannel?.id ?? 'new'}
          isOpen
          channel={modalChannel}
          onClose={() => setModalChannel(undefined)}
          onSaved={handleSaved}
        />
      )}

      <AlertDialog open={!!pagerDutyTestTarget} onOpenChange={(open) => !open && setPagerDutyTestTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Send a test event to PagerDuty</AlertDialogTitle>
            <AlertDialogDescription>
              A test opens an alert on the PagerDuty service this channel&apos;s integration key belongs to,
              which notifies people as that service is set up to. Resolve it in PagerDuty afterwards.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                const channel = pagerDutyTestTarget;
                setPagerDutyTestTarget(null);
                if (channel) runTest(channel);
              }}
            >
              Send test event
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog open={!!deleteTarget} onOpenChange={(open) => !open && setDeleteTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete alert channel</AlertDialogTitle>
            <AlertDialogDescription className="break-words">
              {deleteTarget ? `${deleteTarget.name} stops receiving alerts at once. This cannot be undone.` : ''}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleDelete}
              className="bg-danger-strong text-white shadow-none hover:bg-danger-strong hover:brightness-95"
            >
              Delete channel
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </AuthGuard>
  );
}

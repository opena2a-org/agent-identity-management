'use client';

import { useState, type FormEvent } from 'react';
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
import { AlertCircle, Loader2 } from 'lucide-react';
import {
  api,
  type AlertChannel,
  type AlertChannelConfig,
  type AlertChannelKind,
  type AlertChannelRequest,
  type AlertChannelSeverity,
  type ApiRequestError,
} from '@/lib/api';

export const ALERT_CHANNEL_KINDS: { value: AlertChannelKind; label: string }[] = [
  { value: 'slack', label: 'Slack' },
  { value: 'pagerduty', label: 'PagerDuty' },
  { value: 'teams', label: 'Microsoft Teams' },
  { value: 'email', label: 'Email' },
];

export const ALERT_CHANNEL_SEVERITIES: { value: AlertChannelSeverity; label: string }[] = [
  { value: 'info', label: 'Info' },
  { value: 'warning', label: 'Warning' },
  { value: 'high', label: 'High' },
  { value: 'critical', label: 'Critical' },
];

// The most recipients the server takes in an email channel.
export const MAX_EMAIL_RECIPIENTS = 20;

// The config field each kind takes, with the format the server accepts
// (docs/API.md § Alert channels).
const CONFIG_FIELDS: Record<AlertChannelKind, { label: string; hint: string; placeholder: string }> = {
  slack: {
    label: 'Slack incoming webhook URL',
    hint: "https://hooks.slack.com/services/ followed by the webhook's path, as Slack shows it when an incoming webhook is added to a channel.",
    placeholder: 'https://hooks.slack.com/services/...',
  },
  pagerduty: {
    label: 'PagerDuty integration key',
    hint: 'The 32-character integration key of an Events API v2 integration on the PagerDuty service.',
    placeholder: '32 characters',
  },
  teams: {
    label: 'Teams workflow URL',
    hint: 'The https URL that the workflow\'s "When a Teams webhook request is received" trigger shows.',
    placeholder: 'https://...',
  },
  email: {
    label: 'Email recipients',
    hint: `One bare address per line, such as ops@example.com, with no display name. 1 to ${MAX_EMAIL_RECIPIENTS} addresses.`,
    placeholder: 'ops@example.com\nsec@example.com',
  },
};

type Field = 'name' | 'kind' | 'config' | 'minSeverity' | 'form';
type FieldErrors = Partial<Record<Field, string>>;

// The form field a refusal's message names. The server starts each 400 reason
// with the field it refuses ("config.webhookUrl must be ...", "name is
// required", "unknown field: config.channel"), and answers a name in use with
// 409 "name is used by another alert channel of the organization".
export function fieldOfRefusal(message: string): Field {
  if (/^(unknown field: )?config\b/.test(message)) return 'config';
  if (/^name\b/.test(message)) return 'name';
  if (/^minSeverity\b/.test(message)) return 'minSeverity';
  if (/^(unsupported kind|kind is required)/.test(message) || /^email is not configured/.test(message)) {
    return 'kind';
  }
  return 'form';
}

function recipientsOf(text: string): string[] {
  return text
    .split('\n')
    .map((line) => line.trim())
    .filter((line) => line.length > 0);
}

function buildConfig(kind: AlertChannelKind, text: string): AlertChannelConfig {
  switch (kind) {
    case 'pagerduty':
      return { routingKey: text.trim() };
    case 'email':
      return { recipients: recipientsOf(text) };
    default:
      return { webhookUrl: text.trim() };
  }
}

// What the server answered as the channel's current destination: the masked
// URL or key, or the recipients as stored.
function currentValueOf(channel: AlertChannel): string {
  if (channel.kind === 'email') {
    return (channel.config.recipients ?? []).join(', ');
  }
  return (channel.kind === 'pagerduty' ? channel.config.routingKey : channel.config.webhookUrl) ?? '';
}

const SELECT_CLASS =
  'flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm text-foreground ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50';

interface AlertChannelModalProps {
  isOpen: boolean;
  // The channel to edit; null creates one.
  channel: AlertChannel | null;
  onClose: () => void;
  onSaved: (channel: AlertChannel) => void;
}

export function AlertChannelModal({ isOpen, channel, onClose, onSaved }: AlertChannelModalProps) {
  const editing = channel !== null;
  const [name, setName] = useState(channel?.name ?? '');
  const [kind, setKind] = useState<AlertChannelKind>(channel?.kind ?? 'slack');
  const [configText, setConfigText] = useState('');
  const [minSeverity, setMinSeverity] = useState<AlertChannelSeverity>(channel?.minSeverity ?? 'warning');
  const [errors, setErrors] = useState<FieldErrors>({});
  const [saving, setSaving] = useState(false);

  const configField = CONFIG_FIELDS[kind];
  // The stored destination stays only while the kind does: a new kind needs its own config.
  const keepsCurrent = editing && channel.kind === kind;

  const handleKindChange = (next: AlertChannelKind) => {
    setKind(next);
    setConfigText('');
    setErrors((prev) => ({ ...prev, kind: undefined, config: undefined }));
  };

  const validate = (): FieldErrors => {
    const found: FieldErrors = {};
    if (!name.trim()) {
      found.name = 'name is required';
    }
    if (!configText.trim()) {
      if (!keepsCurrent) {
        found.config = editing
          ? `${configField.label} is required when the kind changes`
          : `${configField.label} is required`;
      }
    } else if (kind === 'email' && recipientsOf(configText).length > MAX_EMAIL_RECIPIENTS) {
      found.config = `Enter at most ${MAX_EMAIL_RECIPIENTS} recipients, one per line.`;
    }
    return found;
  };

  const handleSubmit = async (event: FormEvent) => {
    event.preventDefault();
    const found = validate();
    setErrors(found);
    if (Object.keys(found).length > 0) {
      return;
    }

    setSaving(true);
    try {
      let saved: AlertChannel;
      if (editing) {
        const body: AlertChannelRequest = { name: name.trim(), minSeverity };
        // The server refuses kind for an email channel when it has no email
        // service, so an unchanged kind is left out.
        if (kind !== channel.kind) {
          body.kind = kind;
        }
        if (configText.trim()) {
          body.config = buildConfig(kind, configText);
        }
        saved = await api.updateAlertChannel(channel.id, body);
      } else {
        saved = await api.createAlertChannel({
          name: name.trim(),
          kind,
          config: buildConfig(kind, configText),
          minSeverity,
        });
      }
      onSaved(saved);
    } catch (err) {
      const requestError = err as ApiRequestError;
      const message = requestError?.message || 'The channel was not saved.';
      const field =
        requestError?.status === 400 || requestError?.status === 409 ? fieldOfRefusal(message) : 'form';
      setErrors({ [field]: message });
    } finally {
      setSaving(false);
    }
  };

  const describedBy = (field: Exclude<Field, 'form'>, hintId?: string) =>
    [hintId, errors[field] ? `alert-channel-${field}-error` : undefined].filter(Boolean).join(' ') ||
    undefined;

  const fieldError = (field: Exclude<Field, 'form'>) =>
    errors[field] ? (
      <p
        id={`alert-channel-${field}-error`}
        data-testid={`alert-channel-${field}-error`}
        className="text-xs text-danger-text break-words"
      >
        {errors[field]}
      </p>
    ) : null;

  return (
    <Dialog open={isOpen} onOpenChange={(open) => !open && !saving && onClose()}>
      <DialogContent className="max-w-lg max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle className="break-words">{editing ? `Edit ${channel.name}` : 'Add channel'}</DialogTitle>
          <DialogDescription>
            Alerts at or above the severity floor are sent to this destination in its own format.
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} noValidate className="space-y-5">
          {errors.form && (
            <div
              role="alert"
              className="flex items-start gap-2 rounded-lg border border-danger-border bg-danger-fill p-3 text-danger-text"
            >
              <AlertCircle className="mt-0.5 h-4 w-4 flex-shrink-0" />
              <p className="text-sm break-words">{errors.form}</p>
            </div>
          )}

          <div className="space-y-2">
            <Label htmlFor="alert-channel-name">Name</Label>
            <Input
              id="alert-channel-name"
              value={name}
              maxLength={100}
              placeholder="Security on-call"
              onChange={(e) => setName(e.target.value)}
              disabled={saving}
              aria-invalid={errors.name ? true : undefined}
              aria-describedby={describedBy('name', 'alert-channel-name-hint')}
            />
            <p id="alert-channel-name-hint" className="text-xs text-muted-foreground">
              1 to 100 characters, unique in the organization.
            </p>
            {fieldError('name')}
          </div>

          <div className="space-y-2">
            <Label htmlFor="alert-channel-kind">Kind</Label>
            <select
              id="alert-channel-kind"
              className={SELECT_CLASS}
              value={kind}
              onChange={(e) => handleKindChange(e.target.value as AlertChannelKind)}
              disabled={saving}
              aria-invalid={errors.kind ? true : undefined}
              aria-describedby={describedBy('kind')}
            >
              {ALERT_CHANNEL_KINDS.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
            {fieldError('kind')}
          </div>

          <div className="space-y-2">
            <Label htmlFor="alert-channel-config">{configField.label}</Label>
            {kind === 'email' ? (
              <Textarea
                id="alert-channel-config"
                rows={4}
                value={configText}
                placeholder={keepsCurrent ? (channel.config.recipients ?? []).join('\n') : configField.placeholder}
                onChange={(e) => setConfigText(e.target.value)}
                disabled={saving}
                spellCheck={false}
                aria-invalid={errors.config ? true : undefined}
                aria-describedby={describedBy('config', 'alert-channel-config-hint')}
              />
            ) : (
              <Input
                id="alert-channel-config"
                value={configText}
                placeholder={keepsCurrent ? currentValueOf(channel) : configField.placeholder}
                onChange={(e) => setConfigText(e.target.value)}
                disabled={saving}
                autoComplete="off"
                spellCheck={false}
                aria-invalid={errors.config ? true : undefined}
                aria-describedby={describedBy('config', 'alert-channel-config-hint')}
              />
            )}
            <div id="alert-channel-config-hint" className="space-y-1 text-xs text-muted-foreground">
              <p className="break-words">{configField.hint}</p>
              {keepsCurrent && (
                <p className="break-words">
                  Current value: <code className="rounded bg-muted px-1 break-all">{currentValueOf(channel)}</code>;
                  leave empty to keep the current value.
                </p>
              )}
            </div>
            {fieldError('config')}
          </div>

          <div className="space-y-2">
            <Label htmlFor="alert-channel-floor">Severity floor</Label>
            <select
              id="alert-channel-floor"
              className={SELECT_CLASS}
              value={minSeverity}
              onChange={(e) => setMinSeverity(e.target.value as AlertChannelSeverity)}
              disabled={saving}
              aria-invalid={errors.minSeverity ? true : undefined}
              aria-describedby={describedBy('minSeverity', 'alert-channel-floor-hint')}
            >
              {ALERT_CHANNEL_SEVERITIES.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
            <p id="alert-channel-floor-hint" className="text-xs text-muted-foreground">
              The channel is sent each alert at this severity or above. Severities rank info, warning,
              high, critical.
            </p>
            {fieldError('minSeverity')}
          </div>

          <DialogFooter className="gap-2 sm:gap-0">
            <Button type="button" variant="outline" onClick={onClose} disabled={saving}>
              Cancel
            </Button>
            <Button type="submit" disabled={saving}>
              {saving ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  Saving
                </>
              ) : editing ? (
                'Save changes'
              ) : (
                'Add channel'
              )}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

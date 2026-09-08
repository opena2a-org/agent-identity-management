'use client';

import { useEffect, useState } from 'react';
import { Activity, AlertTriangle, CheckCircle, HelpCircle, Info } from 'lucide-react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { Badge } from '@/components/ui/badge';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { api } from '@/lib/api';

interface DriftScoreCardProps {
  agentId: string;
}

type DriftStatus = 'clean' | 'mild' | 'active' | 'baseline';

// Sentinel value returned by trust_calculator.go:427/433 when the alert
// repository is unavailable (nil dep or query error). The drift factor
// otherwise lives in [0, 1] where 1.0 = no alerts in 30 days and each
// alert subtracts 0.02-0.30. 0.5 from real penalties is theoretically
// possible (0.30 + 0.15 + 0.05) but extremely unlikely; we prefer a
// false-baseline on that exact combo over a false "Active Drift" on
// every default-config deployment.
const NEUTRAL_SENTINEL = 0.5;
const NEUTRAL_EPSILON = 1e-9;

function isBaseline(factor: number): boolean {
  return Math.abs(factor - NEUTRAL_SENTINEL) < NEUTRAL_EPSILON;
}

function clamp01(x: number): number {
  if (!Number.isFinite(x)) return 0;
  if (x < 0) return 0;
  if (x > 1) return 1;
  return x;
}

// Convert the trust factor (higher = less drift) to a drift score
// percentage (higher = more drift). This is the same orientation as the
// `agent.drift_score` OTel gauge published for Grafana, so users reading
// "75% drift" here see the same direction as the gauge spike there.
function factorToDriftPct(factor: number): number {
  return Math.round((1 - clamp01(factor)) * 100);
}

function classifyByDriftPct(driftPct: number): DriftStatus {
  if (driftPct <= 5) return 'clean';
  if (driftPct <= 25) return 'mild';
  return 'active';
}

const STATUS_META: Record<DriftStatus, { label: string; badgeClass: string; scoreClass: string; icon: typeof CheckCircle; description: string }> = {
  clean: {
    label: 'Clean',
    badgeClass: 'bg-success-fill text-success-text border-success-border',
    scoreClass: 'text-success-text',
    icon: CheckCircle,
    description: 'No behavioral drift detected. Runtime configuration matches registered baseline.',
  },
  mild: {
    label: 'Mild Drift',
    badgeClass: 'bg-warning-fill text-warning-text border-warning-border',
    scoreClass: 'text-warning-text',
    icon: Activity,
    description: 'Minor deviations from registered configuration. Review recent drift alerts.',
  },
  active: {
    label: 'Active Drift',
    badgeClass: 'bg-danger-fill text-danger-text border-danger-border',
    scoreClass: 'text-danger-text',
    icon: AlertTriangle,
    description: 'Significant runtime deviation from registered configuration. Investigate immediately.',
  },
  baseline: {
    label: 'Baseline',
    badgeClass: 'bg-gray-500/10 text-gray-700 dark:text-gray-200 border-gray-500/20',
    scoreClass: 'text-gray-600 dark:text-gray-200',
    icon: HelpCircle,
    description: 'No drift signal available yet (alert backend unreachable or no history). Score is the neutral baseline.',
  },
};

export function DriftScoreCard({ agentId }: DriftScoreCardProps) {
  const [driftDetection, setDriftDetection] = useState<number | null>(null);
  const [calculatedAt, setCalculatedAt] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    api
      .getTrustScoreBreakdown(agentId)
      .then((data) => {
        if (cancelled) return;
        const factor = data?.factors?.driftDetection;
        // Reject both missing and non-finite payloads (NaN, ±Infinity).
        // Fail-loud as "Unknown" beats silently clamping a malformed value
        // to 100% drift and rendering a phantom Active-Drift alarm.
        if (typeof factor !== 'number' || !Number.isFinite(factor)) {
          setError('Drift factor missing or invalid in trust breakdown response');
          setDriftDetection(null);
        } else {
          setDriftDetection(factor);
        }
        setCalculatedAt(data?.calculatedAt ?? null);
      })
      .catch((err: Error) => {
        if (cancelled) return;
        setError(err.message || 'Failed to load drift score');
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [agentId]);

  if (loading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Activity className="h-5 w-5" />
            Drift Score
          </CardTitle>
          <CardDescription>Loading behavioral drift signal...</CardDescription>
        </CardHeader>
        <CardContent>
          <Skeleton className="h-10 w-32 mb-2" />
          <Skeleton className="h-4 w-48" />
        </CardContent>
      </Card>
    );
  }

  if (error || driftDetection === null) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Activity className="h-5 w-5" />
            Drift Score
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="text-center py-6 text-muted-foreground">
            <AlertTriangle className="h-8 w-8 mx-auto mb-2 text-yellow-600" />
            <p className="text-sm">{error || 'No drift data available'}</p>
          </div>
        </CardContent>
      </Card>
    );
  }

  const baseline = isBaseline(driftDetection);
  const driftPct = factorToDriftPct(driftDetection);
  const status: DriftStatus = baseline ? 'baseline' : classifyByDriftPct(driftPct);
  const meta = STATUS_META[status];
  const StatusIcon = meta.icon;
  const displayValue = baseline ? '—' : `${driftPct}%`;

  return (
    <TooltipProvider>
      <Card>
        <CardHeader>
          <div className="flex items-start justify-between">
            <div>
              <CardTitle className="flex items-center gap-2">
                <Activity className="h-5 w-5" />
                Drift Score
              </CardTitle>
              <CardDescription className="mt-1">
                Behavioral drift signal — runtime vs registered baseline (0% clean, 100% maximum drift)
              </CardDescription>
            </div>
            <Tooltip>
              <TooltipTrigger aria-label="About drift score">
                <Info className="h-4 w-4 text-muted-foreground hover:text-blue-600 transition-colors" />
              </TooltipTrigger>
              <TooltipContent side="left" className="max-w-xs">
                <p>
                  Derived from the Drift Detection trust factor: each configuration-drift
                  or MCP capability-drift alert in the last 30 days raises this score.
                  Real-time per-event telemetry is published as the{' '}
                  <code>agent.drift_score</code> OTel gauge for Grafana dashboards.
                </p>
              </TooltipContent>
            </Tooltip>
          </div>
        </CardHeader>
        <CardContent>
          <div className="flex items-center justify-between">
            <div>
              <div
                className={`text-4xl font-bold ${meta.scoreClass}`}
                aria-label={baseline ? 'No drift data available' : `Drift score ${driftPct} percent`}
              >
                {displayValue}
              </div>
              {calculatedAt && (
                <p className="text-xs text-muted-foreground mt-1">
                  Last calculated: {new Date(calculatedAt).toLocaleString()}
                </p>
              )}
            </div>
            <Badge className={`gap-1.5 ${meta.badgeClass}`}>
              <StatusIcon className="h-3.5 w-3.5" />
              {meta.label}
            </Badge>
          </div>
          <p className="text-sm text-muted-foreground mt-3">{meta.description}</p>
        </CardContent>
      </Card>
    </TooltipProvider>
  );
}

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, cleanup } from '@testing-library/react';
import { DriftScoreCard } from './drift-score-card';
import { api } from '@/lib/api';

vi.mock('@/lib/api', () => ({
  api: {
    getTrustScoreBreakdown: vi.fn(),
  },
}));

const makeBreakdown = (driftDetection: number, overrides: Record<string, unknown> = {}) => ({
  agentId: 'agent-1',
  agentName: 'test-agent',
  overall: 0.9,
  factors: {
    verificationStatus: 1,
    uptime: 1,
    successRate: 1,
    securityAlerts: 1,
    compliance: 1,
    age: 1,
    driftDetection,
    userFeedback: 1,
  },
  weights: {
    verificationStatus: 0.1, uptime: 0.1, successRate: 0.1, securityAlerts: 0.1,
    compliance: 0.1, age: 0.1, driftDetection: 0.2, userFeedback: 0.2,
  },
  contributions: {
    verificationStatus: 0.1, uptime: 0.1, successRate: 0.1, securityAlerts: 0.1,
    compliance: 0.1, age: 0.1, driftDetection: 0.2, userFeedback: 0.2,
  },
  confidence: 0.9,
  calculatedAt: '2026-05-25T20:00:00Z',
  ...overrides,
});

beforeEach(() => {
  vi.mocked(api.getTrustScoreBreakdown).mockReset();
});

afterEach(() => {
  cleanup();
});

describe('DriftScoreCard — scale and classification', () => {
  it('factor 1.0 (no alerts) renders as 0% drift, Clean', async () => {
    vi.mocked(api.getTrustScoreBreakdown).mockResolvedValue(makeBreakdown(1.0));
    render(<DriftScoreCard agentId="agent-1" />);
    await waitFor(() => expect(screen.getByText('0%')).toBeTruthy());
    expect(screen.getByText('Clean')).toBeTruthy();
  });

  it('factor 0.95 (boundary) renders as 5% drift, still Clean', async () => {
    vi.mocked(api.getTrustScoreBreakdown).mockResolvedValue(makeBreakdown(0.95));
    render(<DriftScoreCard agentId="agent-1" />);
    await waitFor(() => expect(screen.getByText('5%')).toBeTruthy());
    expect(screen.getByText('Clean')).toBeTruthy();
  });

  it('factor 0.80 renders as 20% drift, Mild', async () => {
    vi.mocked(api.getTrustScoreBreakdown).mockResolvedValue(makeBreakdown(0.80));
    render(<DriftScoreCard agentId="agent-1" />);
    await waitFor(() => expect(screen.getByText('20%')).toBeTruthy());
    expect(screen.getByText('Mild Drift')).toBeTruthy();
  });

  it('factor 0.75 (boundary) renders as 25% drift, Mild (not Active)', async () => {
    vi.mocked(api.getTrustScoreBreakdown).mockResolvedValue(makeBreakdown(0.75));
    render(<DriftScoreCard agentId="agent-1" />);
    await waitFor(() => expect(screen.getByText('25%')).toBeTruthy());
    expect(screen.getByText('Mild Drift')).toBeTruthy();
  });

  it('factor 0.945 (rounds drift to 6%) renders as 6% Mild — locks rounding behavior', async () => {
    vi.mocked(api.getTrustScoreBreakdown).mockResolvedValue(makeBreakdown(0.945));
    render(<DriftScoreCard agentId="agent-1" />);
    await waitFor(() => expect(screen.getByText('6%')).toBeTruthy());
    expect(screen.getByText('Mild Drift')).toBeTruthy();
  });

  it('factor 0.744 (rounds drift to 26%) renders as 26% Active — locks rounding behavior', async () => {
    vi.mocked(api.getTrustScoreBreakdown).mockResolvedValue(makeBreakdown(0.744));
    render(<DriftScoreCard agentId="agent-1" />);
    await waitFor(() => expect(screen.getByText('26%')).toBeTruthy());
    expect(screen.getByText('Active Drift')).toBeTruthy();
  });

  it('factor 0.40 renders as 60% drift, Active', async () => {
    vi.mocked(api.getTrustScoreBreakdown).mockResolvedValue(makeBreakdown(0.40));
    render(<DriftScoreCard agentId="agent-1" />);
    await waitFor(() => expect(screen.getByText('60%')).toBeTruthy());
    expect(screen.getByText('Active Drift')).toBeTruthy();
  });

  it('factor 0 renders as 100% drift, Active', async () => {
    vi.mocked(api.getTrustScoreBreakdown).mockResolvedValue(makeBreakdown(0));
    render(<DriftScoreCard agentId="agent-1" />);
    await waitFor(() => expect(screen.getByText('100%')).toBeTruthy());
    expect(screen.getByText('Active Drift')).toBeTruthy();
  });
});

describe('DriftScoreCard — neutral sentinel (alertRepo unavailable)', () => {
  it('factor exactly 0.5 (sentinel) renders as Baseline, never Active', async () => {
    vi.mocked(api.getTrustScoreBreakdown).mockResolvedValue(makeBreakdown(0.5));
    render(<DriftScoreCard agentId="agent-1" />);
    await waitFor(() => expect(screen.getByText('Baseline')).toBeTruthy());
    // No misleading percentage shown when there's no signal
    expect(screen.queryByText('50%')).toBeNull();
    expect(screen.getByText('—')).toBeTruthy();
    // Screen reader-friendly: em-dash is announced as "No drift data available"
    expect(screen.getByLabelText('No drift data available')).toBeTruthy();
  });

  it('factor 0.5 + epsilon is NOT treated as Baseline', async () => {
    vi.mocked(api.getTrustScoreBreakdown).mockResolvedValue(makeBreakdown(0.5 + 1e-6));
    render(<DriftScoreCard agentId="agent-1" />);
    await waitFor(() => expect(screen.getByText('50%')).toBeTruthy());
    expect(screen.queryByText('Baseline')).toBeNull();
  });
});

describe('DriftScoreCard — defensive input handling', () => {
  it('clamps factor > 1 to 0% drift, Clean (treated as best case)', async () => {
    vi.mocked(api.getTrustScoreBreakdown).mockResolvedValue(makeBreakdown(1.5));
    render(<DriftScoreCard agentId="agent-1" />);
    await waitFor(() => expect(screen.getByText('0%')).toBeTruthy());
    expect(screen.getByText('Clean')).toBeTruthy();
  });

  it('clamps factor < 0 to 100% drift, Active', async () => {
    vi.mocked(api.getTrustScoreBreakdown).mockResolvedValue(makeBreakdown(-0.5));
    render(<DriftScoreCard agentId="agent-1" />);
    await waitFor(() => expect(screen.getByText('100%')).toBeTruthy());
    expect(screen.getByText('Active Drift')).toBeTruthy();
  });

  it('treats NaN factor as Unknown (error path), never as 100% Active Drift', async () => {
    vi.mocked(api.getTrustScoreBreakdown).mockResolvedValue(makeBreakdown(NaN));
    render(<DriftScoreCard agentId="agent-1" />);
    await waitFor(() =>
      expect(screen.getByText(/drift factor missing or invalid/i)).toBeTruthy()
    );
    expect(screen.queryByText('100%')).toBeNull();
    expect(screen.queryByText('Active Drift')).toBeNull();
    expect(screen.queryByText('NaN%')).toBeNull();
  });

  it('treats +Infinity factor as Unknown (error path), never as 0% Clean', async () => {
    vi.mocked(api.getTrustScoreBreakdown).mockResolvedValue(makeBreakdown(Infinity));
    render(<DriftScoreCard agentId="agent-1" />);
    await waitFor(() =>
      expect(screen.getByText(/drift factor missing or invalid/i)).toBeTruthy()
    );
    expect(screen.queryByText('Clean')).toBeNull();
  });

  it('handles missing factors object with an error message, no crash', async () => {
    vi.mocked(api.getTrustScoreBreakdown).mockResolvedValue({
      ...makeBreakdown(1.0),
      factors: undefined as unknown as ReturnType<typeof makeBreakdown>['factors'],
    });
    render(<DriftScoreCard agentId="agent-1" />);
    await waitFor(() =>
      expect(screen.getByText(/drift factor missing/i)).toBeTruthy()
    );
  });

  it('handles missing calculatedAt without rendering Invalid Date', async () => {
    vi.mocked(api.getTrustScoreBreakdown).mockResolvedValue({
      ...makeBreakdown(1.0),
      calculatedAt: null as unknown as string,
    });
    render(<DriftScoreCard agentId="agent-1" />);
    await waitFor(() => expect(screen.getByText('0%')).toBeTruthy());
    expect(screen.queryByText(/last calculated/i)).toBeNull();
    expect(screen.queryByText(/invalid date/i)).toBeNull();
  });
});

describe('DriftScoreCard — async lifecycle', () => {
  it('shows error fallback when the API rejects', async () => {
    vi.mocked(api.getTrustScoreBreakdown).mockRejectedValue(new Error('boom'));
    render(<DriftScoreCard agentId="agent-1" />);
    await waitFor(() => expect(screen.getByText('boom')).toBeTruthy());
  });

  it('does not update state after unmount mid-fetch', async () => {
    let resolve!: (v: ReturnType<typeof makeBreakdown>) => void;
    const pending = new Promise<ReturnType<typeof makeBreakdown>>((r) => { resolve = r; });
    vi.mocked(api.getTrustScoreBreakdown).mockReturnValue(pending);
    const { unmount } = render(<DriftScoreCard agentId="agent-1" />);
    unmount();
    resolve(makeBreakdown(0.4));
    // Settle microtask queue; assert no thrown errors and nothing rendered.
    await new Promise((r) => setTimeout(r, 0));
    expect(screen.queryByText('60%')).toBeNull();
  });
});

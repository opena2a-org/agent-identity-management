import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, cleanup } from '@testing-library/react';
import { TrustScoreBreakdown } from './trust-score-breakdown';
import { api } from '@/lib/api';

vi.mock('@/lib/api', () => ({
  api: {
    getTrustScoreBreakdown: vi.fn(),
    getAgentTrustScoreHistory: vi.fn(),
  },
}));

vi.mock('@/components/charts', () => ({
  TrendChart: () => null,
  chartSeries: { brand: '#000' },
}));

beforeEach(() => {
  vi.mocked(api.getTrustScoreBreakdown).mockReset();
  vi.mocked(api.getAgentTrustScoreHistory).mockReset();
  vi.mocked(api.getAgentTrustScoreHistory).mockResolvedValue({ agentId: 'agent-1', history: [] } as never);
});

afterEach(() => {
  cleanup();
});

describe('TrustScoreBreakdown missing factors', () => {
  it('renders the empty state for a {} response', async () => {
    vi.mocked(api.getTrustScoreBreakdown).mockResolvedValue({} as never);
    render(<TrustScoreBreakdown agentId="agent-1" />);
    await waitFor(() => {
      expect(screen.getByText('No trust score data available')).toBeTruthy();
    });
  });

  it('renders the empty state when factors is an empty object', async () => {
    vi.mocked(api.getTrustScoreBreakdown).mockResolvedValue({
      overall: 0.5,
      factors: {},
    } as never);
    render(<TrustScoreBreakdown agentId="agent-1" />);
    await waitFor(() => {
      expect(screen.getByText('No trust score data available')).toBeTruthy();
    });
  });
});

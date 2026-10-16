import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';
import { ViolationsTab } from './violations-tab';
import { api } from '@/lib/api';

vi.mock('@/lib/api', () => ({
  api: {
    getAgentViolations: vi.fn(),
  },
}));

beforeEach(() => {
  vi.mocked(api.getAgentViolations).mockResolvedValue({ violations: [], total: 0 });
});

afterEach(() => {
  cleanup();
});

describe('ViolationsTab header', () => {
  // The title and the Refresh and Export CSV buttons share one row. On a 375px
  // screen the buttons need 253px beside the title and ran 12px past the
  // screen's edge; the row wraps them under the title instead.
  it('wraps its buttons under the title when the row is too narrow for both', async () => {
    render(<ViolationsTab agentId="agent-1" />);
    const title = await screen.findByRole('heading', { name: 'Capability Violations' });
    const row = title.parentElement?.parentElement;
    expect(row?.contains(screen.getByRole('button', { name: /Export CSV/ }))).toBe(true);
    expect(row?.classList.contains('flex-wrap')).toBe(true);
  });
});

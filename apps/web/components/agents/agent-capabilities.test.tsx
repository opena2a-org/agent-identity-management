import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';
import { AgentCapabilities } from './agent-capabilities';
import { api } from '@/lib/api';

vi.mock('@/lib/api', () => ({
  api: {
    getAgentCapabilities: vi.fn(),
    getLatestCapabilityReport: vi.fn(),
  },
}));

beforeEach(() => {
  vi.spyOn(console, 'log').mockImplementation(() => {});
  vi.mocked(api.getAgentCapabilities).mockResolvedValue([]);
  // GET .../capabilities/latest answers 204 for an agent that has sent no
  // report yet, and the API client returns a 204 as undefined.
  vi.mocked(api.getLatestCapabilityReport).mockResolvedValue(undefined);
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe('AgentCapabilities, an agent without a capability report', () => {
  it('shows how to declare capabilities, not an error', async () => {
    render(<AgentCapabilities agentId="agent-1" />);
    expect(await screen.findByText('No Capabilities Detected')).toBeTruthy();
    expect(screen.queryByText(/Failed to load/i)).toBeNull();
  });

  // The decorator line has no space to wrap at: on a 375px screen it needed
  // 340px in a 257px box and pushed the page sideways. Each code line may
  // break anywhere instead.
  it('lets each code line break inside its box', async () => {
    render(<AgentCapabilities agentId="agent-1" />);
    const decorator = await screen.findByText('@client.perform_action(capability="file:read")');
    const configFile = screen.getByText('~/.aim/capabilities.json');
    for (const code of [decorator, configFile]) {
      expect(code.tagName).toBe('CODE');
      expect(code.classList.contains('break-all')).toBe(true);
    }
  });
});

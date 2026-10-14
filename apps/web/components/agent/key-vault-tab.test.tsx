import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, cleanup, fireEvent, within } from '@testing-library/react';
import { KeyVaultTab } from './key-vault-tab';
import { api } from '@/lib/api';

vi.mock('@/lib/api', () => ({
  api: {
    getAgentKeyVault: vi.fn(),
    turnOffAgentHybridMode: vi.fn(),
  },
}));

const makeKeyVault = (overrides: Record<string, unknown> = {}) => ({
  agentId: 'agent-1',
  publicKey: 'ed25519-public-key',
  keyAlgorithm: 'Ed25519',
  keyCreatedAt: '2026-09-01T00:00:00Z',
  hasPreviousPublicKey: false,
  pqcPublicKey: 'ml-dsa-65-public-key',
  pqcKeyAlgorithm: 'ML-DSA-65',
  hybridModeEnabled: true,
  pqcKeyCreatedAt: '2026-09-01T00:00:00Z',
  pqcKeyExpiresAt: null,
  ...overrides,
});

const TURN_OFF = /turn off hybrid mode/i;

beforeEach(() => {
  vi.mocked(api.getAgentKeyVault).mockReset();
  vi.mocked(api.turnOffAgentHybridMode).mockReset();
});

afterEach(() => {
  cleanup();
});

async function renderLoaded(canManage: boolean) {
  render(<KeyVaultTab agentId="agent-1" canManage={canManage} />);
  await waitFor(() => expect(screen.getByText('Post-Quantum Companion Key')).toBeTruthy());
}

describe('KeyVaultTab — what the tab draws', () => {
  it('draws the Agent Public Key card and its copy button, and no future use cases or key management practices block', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });
    vi.mocked(api.getAgentKeyVault).mockResolvedValue(makeKeyVault());
    await renderLoaded(false);

    expect(screen.queryByText(/future use cases/i)).toBeNull();
    expect(screen.queryByText(/key management best practices/i)).toBeNull();

    const card = screen.getByRole('heading', { name: 'Agent Public Key' }).closest<HTMLElement>('.glass');
    expect(card).toBeTruthy();
    expect(within(card!).getByText('ed25519-public-key')).toBeTruthy();
    fireEvent.click(within(card!).getByRole('button'));
    expect(writeText).toHaveBeenCalledWith('ed25519-public-key');
  });
});

describe('KeyVaultTab — who is offered the hybrid mode turn-off', () => {
  it('offers it to an admin or manager when hybrid mode is on', async () => {
    vi.mocked(api.getAgentKeyVault).mockResolvedValue(makeKeyVault());
    await renderLoaded(true);
    expect(screen.getByRole('button', { name: TURN_OFF })).toBeTruthy();
  });

  it('renders no control at all for a member or viewer', async () => {
    vi.mocked(api.getAgentKeyVault).mockResolvedValue(makeKeyVault());
    await renderLoaded(false);
    expect(screen.getByText('Hybrid mode enabled')).toBeTruthy();
    expect(screen.queryByRole('button', { name: TURN_OFF })).toBeNull();
  });

  it('renders no control when the role is not passed', async () => {
    vi.mocked(api.getAgentKeyVault).mockResolvedValue(makeKeyVault());
    render(<KeyVaultTab agentId="agent-1" />);
    await waitFor(() => expect(screen.getByText('Hybrid mode enabled')).toBeTruthy());
    expect(screen.queryByRole('button', { name: TURN_OFF })).toBeNull();
  });

  it('renders no control when hybrid mode is already off', async () => {
    vi.mocked(api.getAgentKeyVault).mockResolvedValue(makeKeyVault({ hybridModeEnabled: false }));
    await renderLoaded(true);
    expect(screen.getByText('Registered')).toBeTruthy();
    expect(screen.queryByRole('button', { name: TURN_OFF })).toBeNull();
  });

  it('renders no control when no post-quantum key is registered', async () => {
    vi.mocked(api.getAgentKeyVault).mockResolvedValue(
      makeKeyVault({ pqcPublicKey: null, hybridModeEnabled: true }),
    );
    await renderLoaded(true);
    expect(screen.getByText('Not registered')).toBeTruthy();
    expect(screen.queryByRole('button', { name: TURN_OFF })).toBeNull();
  });

  it('never offers turning hybrid mode on', async () => {
    vi.mocked(api.getAgentKeyVault).mockResolvedValue(makeKeyVault({ hybridModeEnabled: false }));
    await renderLoaded(true);
    expect(screen.queryByRole('button', { name: /turn on hybrid mode|enable hybrid mode/i })).toBeNull();
  });
});

// The first Algorithm label is the classical key's; the second is the post-quantum key's.
function classicalAlgorithmRow() {
  return screen.getAllByText('Algorithm')[0].parentElement as HTMLElement;
}

describe('KeyVaultTab — the Algorithm row', () => {
  it('shows the stored algorithm', async () => {
    vi.mocked(api.getAgentKeyVault).mockResolvedValue(makeKeyVault({ keyAlgorithm: 'Ed25519' }));
    await renderLoaded(false);
    expect(within(classicalAlgorithmRow()).getByText('Ed25519')).toBeTruthy();
  });

  it('says Not recorded when no algorithm is stored, never Ed25519', async () => {
    vi.mocked(api.getAgentKeyVault).mockResolvedValue(makeKeyVault({ keyAlgorithm: '' }));
    await renderLoaded(false);
    const row = classicalAlgorithmRow();
    expect(within(row).getByText('Not recorded')).toBeTruthy();
    expect(row.textContent).not.toContain('Ed25519');
  });
});

describe('KeyVaultTab — turning hybrid mode off', () => {
  it('asks for confirmation before any request is sent', async () => {
    vi.mocked(api.getAgentKeyVault).mockResolvedValue(makeKeyVault());
    await renderLoaded(true);

    fireEvent.click(screen.getByRole('button', { name: TURN_OFF }));

    expect(await screen.findByRole('alertdialog')).toBeTruthy();
    expect(api.turnOffAgentHybridMode).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull());
    expect(api.turnOffAgentHybridMode).not.toHaveBeenCalled();
    expect(screen.getByText('Hybrid mode enabled')).toBeTruthy();
  });

  it('on success re-reads the key vault and shows the server value', async () => {
    vi.mocked(api.getAgentKeyVault)
      .mockResolvedValueOnce(makeKeyVault())
      .mockResolvedValueOnce(makeKeyVault({ hybridModeEnabled: false }));
    vi.mocked(api.turnOffAgentHybridMode).mockResolvedValue({
      agentId: 'agent-1',
      hybridModeEnabled: false,
      message: 'Hybrid mode disabled successfully',
    });
    await renderLoaded(true);

    fireEvent.click(screen.getByRole('button', { name: TURN_OFF }));
    const dialog = await screen.findByRole('alertdialog');
    fireEvent.click(
      Array.from(dialog.querySelectorAll('button')).find((b) => TURN_OFF.test(b.textContent ?? ''))!,
    );

    await waitFor(() => expect(screen.getByText('Registered')).toBeTruthy());
    expect(api.turnOffAgentHybridMode).toHaveBeenCalledWith('agent-1');
    expect(api.getAgentKeyVault).toHaveBeenCalledTimes(2);
    expect(screen.queryByText('Hybrid mode enabled')).toBeNull();
    expect(screen.queryByRole('button', { name: TURN_OFF })).toBeNull();
  });

  it('shows what the server stored, not an assumed value', async () => {
    // A 200 whose re-read still reports hybrid mode on keeps the badge on.
    vi.mocked(api.getAgentKeyVault).mockResolvedValue(makeKeyVault());
    vi.mocked(api.turnOffAgentHybridMode).mockResolvedValue({
      agentId: 'agent-1',
      hybridModeEnabled: false,
      message: 'Hybrid mode disabled successfully',
    });
    await renderLoaded(true);

    fireEvent.click(screen.getByRole('button', { name: TURN_OFF }));
    const dialog = await screen.findByRole('alertdialog');
    fireEvent.click(
      Array.from(dialog.querySelectorAll('button')).find((b) => TURN_OFF.test(b.textContent ?? ''))!,
    );

    await waitFor(() => expect(api.getAgentKeyVault).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(screen.getByText('Hybrid mode enabled')).toBeTruthy());
    expect(screen.queryByText('Registered')).toBeNull();
  });

  it("on a refusal shows the server's reason and leaves the badge unchanged", async () => {
    vi.mocked(api.getAgentKeyVault).mockResolvedValue(makeKeyVault());
    vi.mocked(api.turnOffAgentHybridMode).mockRejectedValue(new Error('Insufficient permissions'));
    await renderLoaded(true);

    fireEvent.click(screen.getByRole('button', { name: TURN_OFF }));
    const dialog = await screen.findByRole('alertdialog');
    fireEvent.click(
      Array.from(dialog.querySelectorAll('button')).find((b) => TURN_OFF.test(b.textContent ?? ''))!,
    );

    expect((await screen.findByRole('alert')).textContent).toContain('Insufficient permissions');
    expect(screen.getByText('Hybrid mode enabled')).toBeTruthy();
    expect(api.getAgentKeyVault).toHaveBeenCalledTimes(1);
  });
});

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { execSync } from 'child_process';
import { NetworkMonitor } from './network';
import type { EventEngine } from '../engine/event-engine';

vi.mock('child_process', () => ({ execSync: vi.fn() }));

interface ParsedConnection {
  protocol: string;
  localAddr: string;
  localPort: number;
  remoteAddr: string;
  remotePort: number;
  state: string;
  pid?: number;
}

// Captured `ss` outputs, Debian bookworm iproute2-6.1.0, 2026-09-01. Under the
// `state established` filter ss omits the State column, so the two shapes put
// Local/Peer Address:Port at different field indices.

/** `ss -tpn state established` — no State column. */
const FIXTURE_FILTERED =
  'Recv-Q Send-Q Local Address:Port  Peer Address:Port Process\n0      0          127.0.0.1:52430    127.0.0.1:41701 users:(("node",pid=1,fd=19))\n0      0          127.0.0.1:41701    127.0.0.1:52430 users:(("node",pid=1,fd=20))\n';

/** `ss -tpn`, same session — State column present. */
const FIXTURE_UNFILTERED =
  'State Recv-Q Send-Q Local Address:Port  Peer Address:Port Process\nESTAB 0      0          127.0.0.1:52430    127.0.0.1:41701 users:(("node",pid=1,fd=19))\nESTAB 0      0          127.0.0.1:41701    127.0.0.1:52430 users:(("node",pid=1,fd=20))\n';

/** Filtered shape without the `users:(...)` field — ss run without root. */
const FIXTURE_FILTERED_NO_PROCESS =
  'Recv-Q Send-Q Local Address:Port  Peer Address:Port Process\n0      0          127.0.0.1:52430    127.0.0.1:41701\n0      0          127.0.0.1:41701    127.0.0.1:52430\n';

const HEADER_ONLY_FILTERED = 'Recv-Q Send-Q Local Address:Port  Peer Address:Port Process\n';
const HEADER_ONLY_UNFILTERED = 'State Recv-Q Send-Q Local Address:Port  Peer Address:Port Process\n';

/** Feed a fixture to the ss-parsing path of NetworkMonitor. */
function parseSs(fixture: string): ParsedConnection[] {
  vi.mocked(execSync).mockReturnValue(fixture);
  const monitor = new NetworkMonitor(null as unknown as EventEngine);
  return (monitor as unknown as { parseSs(): ParsedConnection[] }).parseSs();
}

const FIRST_CONNECTION = {
  localAddr: '127.0.0.1',
  localPort: 52430,
  remoteAddr: '127.0.0.1',
  remotePort: 41701,
  state: 'ESTABLISHED',
};

describe('NetworkMonitor ss parser — column shift under the state filter', () => {
  beforeEach(() => {
    vi.mocked(execSync).mockReset();
  });

  it('AIM-04.AC1 filtered shape (ss -tpn state established) yields both connections with correct columns', () => {
    const connections = parseSs(FIXTURE_FILTERED);
    expect(connections).toHaveLength(2);
    expect(connections[0]).toMatchObject(FIRST_CONNECTION);
  });

  it('AIM-04.AC1 unfiltered shape (ss -tpn, State column present) yields both connections with correct columns', () => {
    const connections = parseSs(FIXTURE_UNFILTERED);
    expect(connections).toHaveLength(2);
    expect(connections[0]).toMatchObject(FIRST_CONNECTION);
  });

  it('AIM-04.AC2 filtered shape without a process column keeps both lines, pid undefined', () => {
    const connections = parseSs(FIXTURE_FILTERED_NO_PROCESS);
    expect(connections).toHaveLength(2);
    expect(connections[0]).toMatchObject(FIRST_CONNECTION);
    expect(connections[0].pid).toBeUndefined();
    expect(connections[1].pid).toBeUndefined();
  });

  it('AIM-04.AC3 header-only output (filtered shape) yields zero connections', () => {
    expect(parseSs(HEADER_ONLY_FILTERED)).toHaveLength(0);
  });

  it('AIM-04.AC3 header-only output (State-column shape) yields zero connections', () => {
    expect(parseSs(HEADER_ONLY_UNFILTERED)).toHaveLength(0);
  });
});

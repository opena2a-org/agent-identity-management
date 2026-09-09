/**
 * AIM-12 item 13: the correlated-events log is created with the process
 * default mode (0644 under the usual umask) while every other sensitive file
 * this SDK writes — credentials (oauth.ts), the sensor id — is created 0600.
 * The log carries agent ids, denied reasons, and resource paths; it must not
 * be group/world readable.
 */
import { it, expect, afterEach } from 'vitest';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import { writeCorrelatedRecord } from './local-writer';
import { buildCorrelatedRecord } from './correlated-record';

const tmpDirs: string[] = [];

afterEach(() => {
  for (const dir of tmpDirs.splice(0)) {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

it('AIM-12.AC3 item 13: a freshly created correlated-events log is not group/world readable', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'cde-mode-'));
  tmpDirs.push(dir);

  const ok = writeCorrelatedRecord(
    buildCorrelatedRecord({
      correlationId: 'cde_000000000_aabbccddeeff',
      agentId: 'agent-1',
      enforcement: {
        decision: 'deny',
        outcome: 'DENY_INTENT',
        capability: 'net:connect',
        resource: 'https://example.invalid/x',
        deniedReason: 'blocked',
        occurredAt: '2026-06-06T00:00:01.000Z',
        source: 'aim-pdp',
      },
    }),
    dir,
  );
  expect(ok).toBe(true);

  const mode = fs.statSync(path.join(dir, 'correlated-events.jsonl')).mode & 0o777;
  expect(
    mode & 0o077,
    `correlated-events.jsonl was created mode 0${mode.toString(8)}; ` +
      'group/other bits must be clear, matching the 0600 the credential writer uses',
  ).toBe(0);
});

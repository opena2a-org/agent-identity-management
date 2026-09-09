#!/usr/bin/env node

/**
 * `aim-arp` — the shipped telemetry consent CLI for @opena2a/aim-sdk.
 *
 * Registers ONLY the telemetry command group: this is the consent surface the
 * install-time disclosure points at (audit-log review, status, opt-out,
 * right-to-delete). The guard/proxy commands in ./index.ts are internal and do
 * not ship; sensor enrollment (`register`) is held pending a product decision
 * and must not appear here or in any string this bin emits.
 *
 * The bin is named `aim-arp`, never `arp`: `arp` is a system utility on macOS
 * and Linux (/usr/sbin/arp) and an unrelated npm package, and a bin that
 * shadows either is a trap. `npx @opena2a/aim-sdk` resolves to this file too
 * (single-bin resolution), so `npx @opena2a/aim-sdk telemetry status` works
 * without a global install.
 *
 * Thin auto-running entry, nothing importable: the dispatch lives in
 * ./aim-arp-main.ts and the command table in ./telemetry.ts, which is what
 * tests exercise.
 */

import { runAimArp } from './aim-arp-main';

runAimArp(process.argv.slice(2))
  .then((code) => {
    process.exitCode = code;
  })
  .catch((err) => {
    console.error(`Error: ${err instanceof Error ? err.message : String(err)}`);
    process.exitCode = 1;
  });

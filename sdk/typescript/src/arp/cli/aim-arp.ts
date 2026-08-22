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
 * Thin auto-running entry, nothing importable: the dispatch logic and command
 * table live in ./telemetry.ts, which is what tests exercise.
 */

import { SDK_VERSION } from '../../version';
import { loadConfig } from '../index';
import { runTelemetrySubcommand, TELEMETRY_SUBCOMMANDS } from './telemetry';

const argv = process.argv.slice(2);

function showHelp(): void {
  console.log(`
  aim-arp v${SDK_VERSION} — OpenA2A telemetry consent CLI

  USAGE
    aim-arp telemetry <subcommand>
    npx @opena2a/aim-sdk telemetry <subcommand>

  SUBCOMMANDS
    ${TELEMETRY_SUBCOMMANDS.join(', ')}

  Run \`aim-arp telemetry --help\` for what each one does.
`);
}

async function main(): Promise<number> {
  const command = argv[0];
  switch (command) {
    case 'telemetry': {
      // loadConfig never throws on a missing config file; the telemetry
      // commands only need the optional signatureTelemetry block.
      const config = loadConfig();
      return runTelemetrySubcommand(argv[1], argv.slice(2), config.signatureTelemetry);
    }
    case '--version':
    case '-v':
      console.log(`aim-arp v${SDK_VERSION}`);
      return 0;
    case '--help':
    case '-h':
    case undefined:
      showHelp();
      return 0;
    default:
      console.error(`Unknown command: ${command}`);
      showHelp();
      return 1;
  }
}

main()
  .then((code) => {
    process.exitCode = code;
  })
  .catch((err) => {
    console.error(`Error: ${err instanceof Error ? err.message : String(err)}`);
    process.exitCode = 1;
  });

/**
 * The shipped `aim-arp` bin's dispatch, split out of the entry file so tests
 * can drive it through its argv surface — the same split ./telemetry.ts made
 * for the subcommand table. The entry (./aim-arp.ts) stays a thin auto-running
 * shell around this function and exports nothing.
 *
 * Both the bare words `help`/`version` and their flag spellings are accepted:
 * the 1.3.1 release test typed the words (the spelling every modern CLI takes)
 * and was answered with "Unknown command" + exit 1 (AIM-12 item 1).
 */

import { SDK_VERSION } from '../../version';
import { loadConfig } from '../index';
import { runTelemetrySubcommand, TELEMETRY_SUBCOMMANDS } from './telemetry';

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

/** Dispatch one argv vector. Returns the process exit code. */
export async function runAimArp(argv: string[]): Promise<number> {
  const command = argv[0];
  switch (command) {
    case 'telemetry': {
      // loadConfig never throws on a missing config file; the telemetry
      // commands only need the optional signatureTelemetry block.
      const config = loadConfig();
      return runTelemetrySubcommand(argv[1], argv.slice(2), config.signatureTelemetry);
    }
    case 'version':
    case '--version':
    case '-v':
      console.log(`aim-arp v${SDK_VERSION}`);
      return 0;
    case 'help':
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

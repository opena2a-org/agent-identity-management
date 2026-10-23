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

import { cliVersion } from '../../build-version';
import { loadConfig } from '../index';
import { runTelemetrySubcommand, TELEMETRY_SUBCOMMANDS } from './telemetry';
import { runInit, scrubToken } from '../../cli/init';

function showHelp(): void {
  console.log(`
  aim-arp v${cliVersion()}
  OpenA2A AIM SDK CLI

  USAGE
    npx @opena2a/aim-sdk init [--url <url>] [--name <name>]
    aim-arp telemetry <subcommand>
    npx @opena2a/aim-sdk telemetry <subcommand>

  COMMANDS
    init       Register your first agent with a bootstrap token from the
               onboarding screen (run \`npx @opena2a/aim-sdk init --help\`)
    telemetry  Review or change signature telemetry consent

  SUBCOMMANDS
    ${TELEMETRY_SUBCOMMANDS.join(', ')}

  Run \`aim-arp telemetry --help\` for what each one does.
`);
}

/** Dispatch one argv vector. Returns the process exit code. */
export async function runAimArp(argv: string[]): Promise<number> {
  const command = argv[0];
  switch (command) {
    case 'init':
      return runInit(argv.slice(1));
    case 'telemetry': {
      // loadConfig never throws on a missing config file; the telemetry
      // commands only need the optional signatureTelemetry block.
      const config = loadConfig();
      return runTelemetrySubcommand(argv[1], argv.slice(2), config.signatureTelemetry);
    }
    case 'version':
    case '--version':
    case '-v':
    case '-V':
      console.log(`aim-arp v${cliVersion()}`);
      return 0;
    case 'help':
    case '--help':
    case '-h':
    case undefined:
      showHelp();
      return 0;
    default:
      // A bootstrap token pasted as the argument is printed redacted.
      console.error(`${command.startsWith('-') ? 'Unknown option' : 'Unknown command'}: ${scrubToken(command)}`);
      console.error('Run: aim-arp --help');
      return 1;
  }
}

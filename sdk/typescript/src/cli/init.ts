/**
 * `npx @opena2a/aim-sdk init` — register a first agent with an onboarding
 * bootstrap token, from a terminal, in one command.
 *
 * The onboarding screen mints a 15-minute, single-use token scoped to
 * `agents:register` (POST /api/v1/onboarding/bootstrap-tokens). This command:
 *
 *   1. reads the token from AIM_BOOTSTRAP_TOKEN or --token (the environment is
 *      preferred: a flag value is visible in `ps` and shell history);
 *   2. generates the agent's Ed25519 key pair locally, so the private key never
 *      leaves this machine;
 *   3. exchanges the token for an agent (POST
 *      /api/v1/onboarding/bootstrap-tokens/exchange), sending the token only in
 *      the X-AIM-Bootstrap-Token header and only the public key in the body;
 *   4. writes the credentials to ~/.aim/agents/<name>.json (0600, directory
 *      0700) in the layout the Python SDK reads, so either SDK can load them;
 *   5. prints the agent's dashboard URL, at the dashboard address the server
 *      reports (older servers report none, and it is derived from --url).
 *
 * The token is never printed, never logged, never written to disk and never
 * put in a URL. Server text is scrubbed of anything token-shaped before it is
 * shown. Redirects are not followed: a redirected request would carry the
 * token header to whatever host the redirect names.
 *
 * Everything the command touches (environment, home directory, fetch, output)
 * comes in through `InitDeps`, which is what tests drive.
 */

import { chmodSync, closeSync, existsSync, mkdirSync, openSync, renameSync, unlinkSync, writeSync } from 'fs';
import { homedir } from 'os';
import { join } from 'path';
import { generateKeyPair, toBase64 } from '../crypto/ed25519';

export const BOOTSTRAP_TOKEN_ENV = 'AIM_BOOTSTRAP_TOKEN';
export const BOOTSTRAP_TOKEN_HEADER = 'X-AIM-Bootstrap-Token';
export const BOOTSTRAP_TOKEN_PREFIX = 'aim_ob_';
export const EXCHANGE_PATH = '/api/v1/onboarding/bootstrap-tokens/exchange';
export const DEFAULT_AIM_URL = 'https://aim.opena2a.org';
export const DEFAULT_AGENT_NAME = 'my-first-agent';
/** Same on-disk schema the Python SDK stamps on agent credential files. */
export const CREDENTIALS_SCHEMA_VERSION = '1.0';

const REQUEST_TIMEOUT_MS = 30_000;
const TOKEN_PATTERN = /aim_ob_[A-Za-z0-9_-]+/g;

export interface InitDeps {
  env?: NodeJS.ProcessEnv;
  homeDir?: string;
  fetch?: typeof fetch;
  out?: (line: string) => void;
  err?: (line: string) => void;
  timeoutMs?: number;
}

interface InitArgs {
  token?: string;
  url?: string;
  name?: string;
  help: boolean;
}

type ParseResult = { ok: true; args: InitArgs } | { ok: false; error: string };

export function initHelpText(): string {
  return `
  Register your first agent with a bootstrap token from the onboarding screen.

  USAGE
    ${BOOTSTRAP_TOKEN_ENV}=<token> npx @opena2a/aim-sdk init [options]

  OPTIONS
    --token <token>  Bootstrap token (prefer the ${BOOTSTRAP_TOKEN_ENV} variable:
                     a flag value is visible in the process list and shell history)
    --url <url>      AIM server URL (default: AIM_URL, then ${DEFAULT_AIM_URL})
    --name <name>    Agent name (default: ${DEFAULT_AGENT_NAME})
    -h, --help       Show this help

  The agent's Ed25519 key pair is generated on this machine; only the public
  key is sent. Credentials are saved to ~/.aim/agents/<name>.json (owner
  read/write only).
`;
}

const VALUE_FLAGS = new Set(['--token', '--url', '--name']);

function parseArgs(argv: string[]): ParseResult {
  const args: InitArgs = { help: false };
  for (let i = 0; i < argv.length; i++) {
    const raw = argv[i];
    if (raw === '-h' || raw === '--help') {
      args.help = true;
      continue;
    }
    const eq = raw.indexOf('=');
    const flag = raw.startsWith('--') && eq > 0 ? raw.slice(0, eq) : raw;
    if (!VALUE_FLAGS.has(flag)) {
      // Never echo an unknown argument verbatim: a token pasted without its
      // flag would be printed back.
      const shown = raw.startsWith('-') ? scrubToken(raw.split('=')[0]) : 'a positional argument';
      return { ok: false, error: `Unknown option: ${shown}` };
    }
    let value: string | undefined;
    if (flag !== raw) {
      value = raw.slice(eq + 1);
    } else {
      value = argv[i + 1];
      if (value === undefined || (value.startsWith('-') && !value.startsWith(BOOTSTRAP_TOKEN_PREFIX))) {
        return { ok: false, error: `${flag} needs a value` };
      }
      i++;
    }
    if (flag === '--token') args.token = value;
    else if (flag === '--url') args.url = value;
    else args.name = value;
  }
  return { ok: true, args };
}

/** Replace anything token-shaped with a redacted marker. */
export function scrubToken(text: string): string {
  return text.replace(TOKEN_PATTERN, `${BOOTSTRAP_TOKEN_PREFIX}[REDACTED]`);
}

/** Why `url` cannot be an AIM server address, or null when it can. */
export function invalidServerUrlReason(url: string): string | null {
  const trimmed = url.trim();
  if (!trimmed) return '--url is empty';
  let parsed: URL;
  try {
    parsed = new URL(trimmed);
  } catch {
    return `--url ${JSON.stringify(scrubToken(trimmed))} is not a URL`;
  }
  if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
    return `--url ${JSON.stringify(scrubToken(trimmed))} is not an http(s) URL`;
  }
  if (!parsed.hostname) return `--url ${JSON.stringify(scrubToken(trimmed))} has no host`;
  if (parsed.search || parsed.hash || parsed.username || parsed.password) {
    return '--url must be the server address only, without credentials, a query string or a fragment';
  }
  return null;
}

/** Hosts of the local stack, which serves the API on port 8080 and the dashboard on 3000. */
const LOOPBACK_HOSTS = new Set(['localhost', '127.0.0.1', '[::1]']);

const HOSTED_DASHBOARDS: Record<string, string> = {
  'api.aim.opena2a.org': 'https://aim.opena2a.org',
  'api.community.opena2a.org': 'https://community.opena2a.org',
};

/**
 * The dashboard address for an API address, for a server whose exchange
 * answer reports none. A hosted API host maps to its dashboard; port 8080 maps
 * to 3000 only on a loopback host, where the local stack runs; a trailing
 * `/api` path segment is dropped. Any other address is taken to serve the
 * dashboard as well.
 */
export function dashboardUrlFor(aimUrl: string): string {
  let url: URL;
  try {
    url = new URL(aimUrl.trim());
  } catch {
    return aimUrl.trim().replace(/\/+$/, '');
  }
  const hosted = HOSTED_DASHBOARDS[url.hostname];
  if (hosted) return hosted;
  if (url.port === '8080' && LOOPBACK_HOSTS.has(url.hostname)) url.port = '3000';
  const path = url.pathname.replace(/\/+$/, '').replace(/\/api$/, '');
  return `${url.origin}${path}`;
}

/**
 * The dashboard address a server reported in its exchange answer, or null
 * when it reported none or one that cannot be used. A loopback address from a
 * server reached at another host is the server's unset FRONTEND_URL default,
 * not an address this machine can open.
 */
export function reportedDashboardUrl(value: unknown, aimUrl: string): string | null {
  if (typeof value !== 'string' || invalidServerUrlReason(value) !== null) return null;
  const url = new URL(value.trim());
  let aimHost = '';
  try {
    aimHost = new URL(aimUrl).hostname;
  } catch {
    // aimUrl was validated before the request; keep the reported address.
  }
  if (LOOPBACK_HOSTS.has(url.hostname) && aimHost && !LOOPBACK_HOSTS.has(aimHost)) return null;
  return `${url.origin}${url.pathname.replace(/\/+$/, '')}`;
}

/**
 * File name for an agent's credentials, sanitized as the Python SDK does it
 * (`c.isalnum() or c in "-_"`): letters and digits of any script, `-` and `_`
 * are kept, and every other character becomes one `_`.
 */
export function agentCredentialsFileName(name: string): string {
  return `${name.replace(/[^\p{L}\p{N}_-]/gu, '_')}.json`;
}

interface ServerAnswer {
  code?: string;
  message?: string;
}

function readServerAnswer(body: unknown): ServerAnswer {
  if (!body || typeof body !== 'object') return {};
  const b = body as Record<string, unknown>;
  const message = [b.error, b.message, b.errorDescription].find((v) => typeof v === 'string') as
    | string
    | undefined;
  return {
    code: typeof b.code === 'string' ? b.code : undefined,
    message: message ? scrubToken(message) : undefined,
  };
}

const REFUSAL_FIX: Record<string, string> = {
  bootstrap_token_invalid: 'The bootstrap token was not recognised.',
  bootstrap_token_expired: 'The bootstrap token has expired (tokens last 15 minutes).',
  bootstrap_token_used: 'The bootstrap token has already registered an agent (each token works once).',
  bootstrap_token_revoked: 'The bootstrap token was revoked (a newer token was generated, or it was exposed).',
  bootstrap_token_in_url: 'The bootstrap token was sent in a URL and has been revoked.',
};

function writeCredentialsFile(dir: string, file: string, content: string): void {
  mkdirSync(dir, { recursive: true, mode: 0o700 });
  // mkdir's mode applies only to directories it creates; tighten an existing one.
  chmodSync(dir, 0o700);
  const tmp = join(dir, `.${process.pid}.${Date.now()}.tmp`);
  // Created 0600 from the first byte, then moved into place.
  const fd = openSync(tmp, 'wx', 0o600);
  try {
    writeSync(fd, content);
  } finally {
    closeSync(fd);
  }
  try {
    chmodSync(tmp, 0o600);
    renameSync(tmp, file);
  } catch (e) {
    try {
      unlinkSync(tmp);
    } catch {
      // nothing left to clean up
    }
    throw e;
  }
}

/** Run `init`. Returns the process exit code. */
export async function runInit(argv: string[], deps: InitDeps = {}): Promise<number> {
  const env = deps.env ?? process.env;
  const out = deps.out ?? ((line: string) => console.log(line));
  const err = deps.err ?? ((line: string) => console.error(line));
  const doFetch = deps.fetch ?? globalThis.fetch;
  const home = deps.homeDir ?? homedir();

  const parsed = parseArgs(argv);
  if (!parsed.ok) {
    err(`Error: ${parsed.error}`);
    err('Run `npx @opena2a/aim-sdk init --help` for usage.');
    return 1;
  }
  const { args } = parsed;
  if (args.help) {
    out(initHelpText());
    return 0;
  }

  const token = (args.token ?? env[BOOTSTRAP_TOKEN_ENV] ?? '').trim();
  if (!token) {
    err('Error: no bootstrap token.');
    err('Copy the command from the onboarding screen, or set the token yourself:');
    err(`  ${BOOTSTRAP_TOKEN_ENV}=<token> npx @opena2a/aim-sdk init`);
    return 1;
  }
  if (args.token !== undefined) {
    // Said before the value is checked: a value that is not a bootstrap token,
    // such as another credential pasted by mistake, is in the history too.
    err(`Note: --token is visible in the process list and shell history; ${BOOTSTRAP_TOKEN_ENV} is not.`);
  }
  if (!token.startsWith(BOOTSTRAP_TOKEN_PREFIX)) {
    err(`Error: that is not a bootstrap token (bootstrap tokens start with ${BOOTSTRAP_TOKEN_PREFIX}).`);
    err('Generate one from the onboarding screen in the dashboard.');
    return 1;
  }
  if (args.token !== undefined) {
    err('The bootstrap token works once, so it is spent after this run.');
  }

  const rawUrl = args.url ?? env.AIM_URL ?? DEFAULT_AIM_URL;
  const urlProblem = invalidServerUrlReason(rawUrl);
  if (urlProblem) {
    err(`Error: ${urlProblem}.`);
    err('Pass the server address, for example: --url https://aim.example.com');
    return 1;
  }
  const aimUrl = rawUrl.trim().replace(/\/+$/, '');

  const name = (args.name ?? DEFAULT_AGENT_NAME).trim();
  if (!name) {
    err('Error: --name is empty.');
    return 1;
  }

  const agentsDir = join(home, '.aim', 'agents');
  const credentialsFile = join(agentsDir, agentCredentialsFileName(name));
  // Checked before the exchange so an existing agent's keys are never
  // overwritten and the single-use token is not spent on a run that cannot
  // save its result.
  if (existsSync(credentialsFile)) {
    err(`Error: credentials for an agent named "${name}" already exist at ${credentialsFile}.`);
    err('Pick another name with --name <name>; the token has not been used.');
    return 1;
  }

  const keyPair = await generateKeyPair();
  const publicKey = toBase64(keyPair.publicKey);
  // The 64-byte seed+public form, as the Python and Go code store it.
  const fullPrivateKey = new Uint8Array(64);
  fullPrivateKey.set(keyPair.privateKey, 0);
  fullPrivateKey.set(keyPair.publicKey, 32);
  const privateKey = toBase64(fullPrivateKey);

  const exchangeUrl = `${aimUrl}${EXCHANGE_PATH}`;
  out(`Registering agent "${name}" with ${aimUrl} ...`);

  let response: Response;
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), deps.timeoutMs ?? REQUEST_TIMEOUT_MS);
  try {
    response = await doFetch(exchangeUrl, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Accept: 'application/json',
        [BOOTSTRAP_TOKEN_HEADER]: token,
      },
      body: JSON.stringify({ name, agentType: 'custom', publicKey }),
      redirect: 'manual',
      signal: controller.signal,
    });
  } catch (e) {
    const reason = controller.signal.aborted
      ? `no answer within ${Math.round((deps.timeoutMs ?? REQUEST_TIMEOUT_MS) / 1000)}s`
      : scrubToken(e instanceof Error ? (e.cause instanceof Error ? e.cause.message : e.message) : String(e));
    err(`Error: could not reach the AIM server at ${aimUrl} (${reason}).`);
    err('Check the address and pass it with --url; the token has not been used.');
    return 1;
  } finally {
    clearTimeout(timer);
  }

  let body: unknown = null;
  try {
    const text = await response.text();
    body = text ? JSON.parse(text) : null;
  } catch {
    body = null;
  }

  if (response.status >= 300 && response.status < 400) {
    const location = response.headers.get('location');
    err(`Error: ${exchangeUrl} answered with a redirect (HTTP ${response.status})${location ? ` to ${scrubToken(location)}` : ''}.`);
    err('The token was not sent on to the new address. Pass the final server address with --url.');
    return 1;
  }

  if (response.status !== 201 && response.status !== 200) {
    const answer = readServerAnswer(body);
    const fix = answer.code ? REFUSAL_FIX[answer.code] : undefined;
    if (fix) {
      err(`Error: ${fix}`);
      err('Generate a new token from the onboarding screen and run the command again.');
    } else if (response.status === 409) {
      err(`Error: your organization already has an agent named "${name}".`);
      err('Run again with --name <another-name>; the token is still valid.');
    } else if (response.status === 404) {
      err(`Error: ${aimUrl} has no onboarding endpoint (HTTP 404).`);
      err('Check that --url is the AIM server address and that the server is up to date.');
    } else if (response.status === 429) {
      err('Error: too many attempts (HTTP 429). Wait a minute and run the command again.');
    } else {
      err(`Error: the server refused the registration (HTTP ${response.status})${answer.message ? `: ${answer.message}` : ''}.`);
    }
    return 1;
  }

  const agent = (body && typeof body === 'object' ? body : {}) as Record<string, unknown>;
  const agentId = typeof agent.agentId === 'string' ? agent.agentId : '';
  if (!agentId) {
    err(`Error: the server answered HTTP ${response.status} but returned no agent id; nothing was saved.`);
    return 1;
  }
  if (typeof agent.publicKey === 'string' && agent.publicKey !== '' && agent.publicKey !== publicKey) {
    err('Error: the server registered a different public key than the one generated here; nothing was saved.');
    err(`Remove agent ${agentId} from the dashboard and run the command again with a new token.`);
    return 1;
  }

  const credentials = {
    agent_id: agentId,
    public_key: publicKey,
    private_key: privateKey,
    aim_url: aimUrl,
    status: typeof agent.status === 'string' ? agent.status : 'unknown',
    trust_score: null,
    organization_id: typeof agent.organizationId === 'string' ? agent.organizationId : null,
    registered_at: new Date().toISOString(),
    schemaVersion: CREDENTIALS_SCHEMA_VERSION,
    type: 'agent',
    name,
  };

  try {
    writeCredentialsFile(agentsDir, credentialsFile, `${JSON.stringify(credentials, null, 2)}\n`);
  } catch (e) {
    err(`Error: agent ${agentId} was registered, but its credentials could not be saved to ${credentialsFile}`);
    err(`(${e instanceof Error ? e.message : String(e)}).`);
    err('Its private key exists only in this process and is now lost; remove the agent from the dashboard and run again.');
    return 1;
  }

  const dashboard = reportedDashboardUrl(agent.dashboardUrl, aimUrl) ?? dashboardUrlFor(aimUrl);
  out('');
  out(`Agent "${name}" is registered.`);
  out(`  Agent ID:     ${agentId}`);
  out(`  Status:       ${credentials.status}`);
  out(`  Credentials:  ${credentialsFile} (owner read/write only)`);
  out(`  Dashboard:    ${dashboard}/dashboard/agents/${agentId}`);
  out('');
  out('The private key was generated on this machine and has not been sent anywhere.');
  return 0;
}

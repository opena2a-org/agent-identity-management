/**
 * Fix sentences for a 401 answered to a request signed with
 * `agent-request-v1`.
 *
 * The refusal's message is the server's `error` string verbatim (capped as
 * parseAPIError caps it), then `\nFix: ` and the one sentence of the row that
 * string selects. Rows are static: no value from the server is interpolated.
 * Matching is case-sensitive; every exact string is tried before any prefix,
 * so the revoked row is chosen before the general status row. The table lists
 * the strings of the self-hosted AIM server and of the hosted deployment.
 *
 * @internal
 */

import { AuthenticationError, capAPIErrorMessage } from '../exceptions';

export interface AgentRequestFixRow {
  readonly id: string;
  /** Server `error` strings matched exactly. */
  readonly exact: readonly string[];
  /** Server `error` prefixes, tried only after every exact string. */
  readonly prefix: readonly string[];
  /** Server `reasonCode` values that select this row. */
  readonly reasonCodes: readonly string[];
  /** The one Fix sentence, without the `Fix: ` label. */
  readonly fix: string;
}

/** The SDK operation whose user-session refusal gets the agent-record row. */
const UPDATE_AGENT = 'updateAgent';

export const AGENT_REQUEST_FIX_ROWS: readonly AgentRequestFixRow[] = [
  {
    id: 'clock',
    exact: ['Request timestamp expired or invalid'],
    prefix: [],
    reasonCodes: [],
    fix:
      "Synchronize this host's clock with network time and send the request again; " +
      'AIM accepts a signed request only within 30 seconds of its own clock.',
  },
  {
    id: 'agentKey',
    exact: [
      'Invalid agent ID format',
      'Agent not found',
      'agent has no registered Ed25519 public key',
      'provided Ed25519 public key does not match registered key',
      'invalid public key format',
      'invalid Ed25519 public key size',
      'Agent has no registered public key. Register a key first using JWT authentication.',
      'Provided public key does not match registered key',
      'Invalid public key format',
    ],
    prefix: ['Invalid public key size: '],
    reasonCodes: [],
    fix:
      'Load credentials from one `registerAgent` result for the AIM server at `baseUrl`: ' +
      '`AIM_AGENT_ID`, `AIM_PUBLIC_KEY` and `AIM_PRIVATE_KEY` must belong to the same registered agent.',
  },
  {
    id: 'requestChanged',
    exact: [
      'invalid Ed25519 signature',
      'invalid signature format',
      'missing Ed25519 signature or public key',
      'Invalid timestamp format',
      'No authentication token provided',
      'Invalid signature',
      'Invalid signature format',
    ],
    prefix: [],
    reasonCodes: [],
    fix:
      'Make `baseUrl` reach AIM with the request unchanged: no path prefix that a proxy strips, ' +
      'and no proxy that rewrites the path, query, body or signature headers.',
  },
  {
    id: 'agentRevoked',
    exact: ['Agent is not permitted to authenticate (status: revoked)'],
    prefix: [],
    reasonCodes: [],
    fix: 'This agent is revoked: ask an AIM admin or manager to register a new agent to replace it.',
  },
  {
    id: 'agentStatus',
    exact: [],
    prefix: ['Agent is not permitted to authenticate (status: '],
    reasonCodes: [],
    fix: 'Ask an AIM admin or manager to reactivate this agent.',
  },
  {
    id: 'hybridMode',
    exact: ['Agent is in hybrid mode and the request is not hybrid-signed'],
    prefix: [],
    reasonCodes: [],
    fix:
      'This SDK signs with Ed25519 only, so ask an AIM admin or manager to turn off hybrid mode ' +
      'for this agent.',
  },
  {
    id: 'userSession',
    exact: ['Authentication required'],
    prefix: [],
    reasonCodes: ['userSessionRequired'],
    fix:
      'AIM accepts this call only from a signed-in AIM user, not from an agent, and this SDK has ' +
      'no user sign-in: ask an AIM admin, manager or member to make it in their own AIM session.',
  },
  {
    id: 'agentRecord',
    // Selected in place of userSession when the refused call is updateAgent.
    exact: [],
    prefix: [],
    reasonCodes: [],
    fix:
      "AIM lets a signed-in AIM user, not the agent itself, change this agent's record: " +
      'ask an AIM admin, manager or member to make the change.',
  },
  {
    id: 'other',
    // Any other 401 to a signed request.
    exact: [],
    prefix: [],
    reasonCodes: [],
    fix:
      'Check that the credentials come from one `registerAgent` result for the AIM server at ' +
      "`baseUrl`, that this host's clock is within 30 seconds of network time, and that no proxy " +
      'changes the request.',
  },
];

function row(id: string): AgentRequestFixRow {
  const found = AGENT_REQUEST_FIX_ROWS.find((r) => r.id === id);
  if (!found) throw new Error(`missing fix row ${id}`);
  return found;
}

/**
 * The row for a 401 to a signed request, chosen by the server's `error`
 * string and `reasonCode`. `operation` is the SDK method that sent the
 * request; it only separates the user-session and agent-record rows.
 */
export function agentRequestFixRow(
  serverError: unknown,
  reasonCode?: unknown,
  operation?: string
): AgentRequestFixRow {
  const error = typeof serverError === 'string' ? serverError : '';
  let chosen =
    AGENT_REQUEST_FIX_ROWS.find((r) => r.exact.includes(error)) ??
    AGENT_REQUEST_FIX_ROWS.find((r) => r.prefix.some((p) => error.startsWith(p))) ??
    (typeof reasonCode === 'string' && reasonCode.length > 0
      ? AGENT_REQUEST_FIX_ROWS.find((r) => r.reasonCodes.includes(reasonCode))
      : undefined) ??
    row('other');
  if (chosen.id === 'userSession' && operation === UPDATE_AGENT) {
    chosen = row('agentRecord');
  }
  return chosen;
}

/**
 * The `AuthenticationError` for a 401 answered to a signed request: the
 * server's `error` verbatim, then `\nFix: ` and the row's sentence.
 */
export function agentRequestAuthenticationError(
  body: unknown,
  operation?: string
): AuthenticationError {
  const errorBody = (body ?? {}) as Record<string, unknown>;
  const fixRow = agentRequestFixRow(errorBody.error, errorBody.reasonCode, operation);
  const message = `${capAPIErrorMessage(errorBody.error)}\nFix: ${fixRow.fix}`;
  return new AuthenticationError(message, errorBody);
}

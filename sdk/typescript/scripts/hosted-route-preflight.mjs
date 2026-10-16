#!/usr/bin/env node
/**
 * Asks a hosted AIM service whether it serves the routes the SDK's first-run
 * flows call, before a release that calls them is published.
 *
 * Usage:
 *   node scripts/hosted-route-preflight.mjs <base-url>
 *
 * The base URL is required; there is no default. The script sends one POST
 * with the body `{}` to two paths that cannot exist, then to each route in
 * FIRST_RUN_ROUTES. No request carries a credential of any kind: no
 * Authorization header, no bootstrap-token header, no cookie, and a base URL
 * with a user name or password is refused.
 *
 * A route counts as served when its answer (status and body) differs from the
 * answer to the unknown paths. Before comparing, each body has the path it was
 * asked for replaced with a placeholder (a not-found text may repeat it) and,
 * when it is JSON, every string value that is a date-time replaced with a
 * placeholder (the AIM backend's not-found answer carries the time of the
 * request). When the two unknown paths still get different answers, a served
 * route cannot be told from an unknown one and the script stops without
 * judging any route.
 *
 * It prints one line per route:
 *   served POST <path> (HTTP <status>)
 *   not served POST <path> (HTTP 404, same answer as an unknown path)
 * the second followed by a command that shows the route's answer and the
 * re-run that clears it.
 *
 * Exit codes: 0 every route is served; 1 at least one route is not served;
 * 2 nothing was judged (no or bad base URL, a network error naming the host,
 * or unknown paths that get different answers).
 *
 * tests/hosted-route-preflight.test.ts holds FIRST_RUN_ROUTES equal to the
 * paths the sources call and runs the script against a loopback server.
 */

import { randomBytes } from 'node:crypto';
import { realpathSync } from 'node:fs';
import { relative } from 'node:path';
import { fileURLToPath } from 'node:url';

/**
 * The routes a first run calls, in the order a new user meets them. Each
 * path is the one the source named beside it sends.
 */
export const FIRST_RUN_ROUTES = [
  // `aim-arp init`: EXCHANGE_PATH in src/cli/init.ts.
  { method: 'POST', path: '/api/v1/onboarding/bootstrap-tokens/exchange' },
  // AIMClient.registerAgent in src/client/AIMClient.ts.
  { method: 'POST', path: '/api/v1/agents' },
  // AIMClient.verifyAction: SDK_VERIFICATIONS_PATH in src/client/sdk-verification.ts.
  { method: 'POST', path: '/api/v1/sdk-api/verifications' },
];

const REQUEST_TIMEOUT_MS = 15_000;
const DATE_TIME = /^\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}(:\d{2}(\.\d+)?)?(Z|[+-]\d{2}:?\d{2})?$/;

class Unreachable extends Error {}

function shellQuote(s) {
  return `'${s.replace(/'/g, `'\\''`)}'`;
}

/** `s` as one shell word: as it is when it holds only URL and path characters the shell leaves alone. */
function shellArg(s) {
  return /^[A-Za-z0-9:/._~%@+=,-]+$/.test(s) ? s : shellQuote(s);
}

/** Every JSON string value that is a date-time, replaced with a placeholder. */
function withoutDateTimes(value) {
  if (typeof value === 'string') return DATE_TIME.test(value) ? '<date-time>' : value;
  if (Array.isArray(value)) return value.map(withoutDateTimes);
  if (value && typeof value === 'object') {
    return Object.fromEntries(Object.entries(value).map(([k, v]) => [k, withoutDateTimes(v)]));
  }
  return value;
}

/** The part of an answer that is compared: its status and its body without the request's own path and times. */
function fingerprint(answer, path) {
  const text = answer.body.split(path).join('<path>');
  let body = text;
  try {
    body = JSON.stringify(withoutDateTimes(JSON.parse(text)));
  } catch {
    // Not JSON: compared as text.
  }
  return `${answer.status} ${body}`;
}

function parseBaseUrl(raw) {
  let url;
  try {
    url = new URL(raw);
  } catch {
    return { error: `not a URL: ${raw}` };
  }
  if (url.protocol !== 'https:' && url.protocol !== 'http:') {
    return { error: `the base URL must be http or https, not ${url.protocol}` };
  }
  if (url.username || url.password) {
    return { error: 'the base URL carries a user name or password; this script sends no credential' };
  }
  if (url.search || url.hash) {
    return { error: 'the base URL takes no query or fragment' };
  }
  return { base: `${url.origin}${url.pathname.replace(/\/+$/, '')}`, host: url.host };
}

async function post(base, host, path) {
  let response;
  let body;
  try {
    response = await fetch(`${base}${path}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
      body: '{}',
      credentials: 'omit',
      redirect: 'manual',
      signal: AbortSignal.timeout(REQUEST_TIMEOUT_MS),
    });
    body = await response.text();
  } catch (e) {
    const cause = e instanceof Error && e.cause instanceof Error ? e.cause : e;
    const reason =
      cause instanceof Error && cause.name === 'TimeoutError'
        ? `no answer within ${REQUEST_TIMEOUT_MS / 1000}s`
        : (cause && (cause.code || cause.message)) || String(cause);
    throw new Unreachable(`could not reach ${host} (POST ${path}: ${reason})`);
  }
  return { status: response.status, body };
}

async function main(argv) {
  const script = shellArg(relative(process.cwd(), fileURLToPath(import.meta.url)));
  if (argv.length !== 1) {
    process.stderr.write(`usage: node ${script} <base-url>\n`);
    process.stderr.write(`example: node ${script} https://api.aim.opena2a.org\n`);
    return 2;
  }
  const parsed = parseBaseUrl(argv[0]);
  if (parsed.error) {
    process.stderr.write(`error: ${parsed.error}\n`);
    process.stderr.write(`usage: node ${script} <base-url>\n`);
    return 2;
  }
  const { base, host } = parsed;

  try {
    const unknownPaths = [0, 1].map(() => `/api/v1/hosted-route-preflight-unknown-${randomBytes(8).toString('hex')}`);
    const unknown = [];
    for (const path of unknownPaths) {
      const answer = await post(base, host, path);
      unknown.push({ path, answer, fingerprint: fingerprint(answer, path) });
    }
    if (unknown[0].fingerprint !== unknown[1].fingerprint) {
      process.stderr.write(
        `error: two unknown paths on ${host} got different answers ` +
          `(HTTP ${unknown[0].answer.status} and HTTP ${unknown[1].answer.status}), ` +
          'so a served route cannot be told from an unknown one; no route was judged\n',
      );
      process.stderr.write(
        `verify: curl -sS -X POST -H 'Content-Type: application/json' --data '{}' -w ' HTTP %{http_code}\\n' ${shellQuote(`${base}${unknown[0].path}`)}\n`,
      );
      return 2;
    }
    process.stdout.write(`an unknown path on ${host} answers HTTP ${unknown[0].answer.status}\n`);

    let notServed = 0;
    for (const route of FIRST_RUN_ROUTES) {
      const answer = await post(base, host, route.path);
      if (fingerprint(answer, route.path) === unknown[0].fingerprint) {
        notServed += 1;
        process.stdout.write(
          `not served ${route.method} ${route.path} (HTTP ${answer.status}, same answer as an unknown path)\n`,
        );
        process.stdout.write(
          `  verify: curl -sS -X ${route.method} -H 'Content-Type: application/json' --data '{}' -w ' HTTP %{http_code}\\n' ${shellQuote(`${base}${route.path}`)}\n`,
        );
        process.stdout.write(
          `  fix: deploy a backend that serves ${route.method} ${route.path} to ${host}, then re-run: node ${script} ${shellArg(argv[0])}\n`,
        );
      } else {
        process.stdout.write(`served ${route.method} ${route.path} (HTTP ${answer.status})\n`);
      }
    }

    if (notServed > 0) {
      process.stdout.write(`${notServed} of ${FIRST_RUN_ROUTES.length} first-run routes not served by ${host}\n`);
      return 1;
    }
    process.stdout.write(`all ${FIRST_RUN_ROUTES.length} first-run routes served by ${host}\n`);
    return 0;
  } catch (e) {
    if (e instanceof Unreachable) {
      process.stderr.write(`error: ${e.message}\n`);
      return 2;
    }
    throw e;
  }
}

// Run only when executed, not when the test imports the route table. The
// module's own path is a real path (macOS /tmp is /private/tmp), so the
// invoked path is compared as one too.
if (process.argv[1] && fileURLToPath(import.meta.url) === realpathSync(process.argv[1])) {
  process.exitCode = await main(process.argv.slice(2));
}

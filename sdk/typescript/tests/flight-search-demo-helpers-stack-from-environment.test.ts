import { describe, it, expect, beforeAll, afterAll } from 'vitest';
import { chmodSync, copyFileSync, existsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, rmSync, writeFileSync } from 'fs';
import { tmpdir } from 'os';
import { join } from 'path';
import { spawnSync } from 'child_process';

/**
 * The flight-search demo's helper scripts reach the stack the compose files
 * name.
 *
 * docker-compose.yml and docker-compose.quickstart.yml take the backend's host
 * port from AIM_BACKEND_PORT (default 8080), the dashboard's from
 * AIM_FRONTEND_PORT (3000) and PostgreSQL's from AIM_POSTGRES_PORT (5432).
 * get_fresh_sdk.py and verify_qa_complete.py called the backend on
 * localhost:8080 and printed dashboard URLs on localhost:3000, demo_search.py
 * and test_flight_agent.py printed the dashboard on localhost:3000, and
 * check_sdk_token.sh ran psql on the default port, so against a stack started
 * on other ports they reached nothing, or, beside a default stack on the same
 * host, that stack. flight_agent.py's hint for an unreachable backend named
 * `curl localhost:8080/healthz`, a path the backend does not serve, whatever
 * URL the client was using.
 *
 * Each case copies the example's scripts into a scratch tree shaped like the
 * repository (examples/flight-search-agent/ two levels below the .env) with
 * stand-ins for the AIM SDK, `requests`, psql and jq, so every backend call,
 * every psql call and every URL the scripts print is recorded.
 */

const REPO_ROOT = join(__dirname, '..', '..', '..');
const DEMO_DIR = join(REPO_ROOT, 'examples', 'flight-search-agent');
const SCRIPTS = [
  'stack-env.sh',
  'stack_env.py',
  'check_sdk_token.sh',
  'get_fresh_sdk.py',
  'verify_qa_complete.py',
  'flight_agent.py',
  'demo_search.py',
  'test_flight_agent.py',
];

/**
 * Records each call. A POST answers 401, or, with STUB_POST_OK set, 200 and a
 * new token pair, the answer a refresh of a live credential gets.
 */
const FAKE_REQUESTS = `import json, os


class _Response:
    def __init__(self, status_code, body):
        self.status_code = status_code
        self._body = body
        self.text = json.dumps(body)

    def json(self):
        return self._body


def _record(method, url):
    with open(os.environ['STUB_LOG'], 'a') as f:
        f.write(method + ' ' + url + '\\n')


def post(url, **kwargs):
    _record('POST', url)
    if os.environ.get('STUB_POST_OK'):
        return _Response(200, {'access_token': 'not-a-real-access-token', 'refresh_token': 'not-a-real-new-token'})
    return _Response(401, {})


def get(url, **kwargs):
    _record('GET', url)
    return _Response(200, [] if url.endswith('/verification-events') else {})
`;

const FAKE_SDK: Record<string, string> = {
  '__init__.py': 'def secure(*args, **kwargs):\n    return None\n\n\ndef register_agent(*args, **kwargs):\n    return None\n',
  'client.py': 'class AIMClient:\n    pass\n',
  'exceptions.py':
    'class AIMError(Exception):\n    pass\n\n\nclass ActionDeniedError(AIMError):\n    pass\n\n\nclass VerificationError(AIMError):\n    pass\n',
};

/** Stands in for the real agent in demo_search.py and test_flight_agent.py. */
const FAKE_AGENT = `class FlightAgent:
    def __init__(self):
        self.client = object()
        self.agent_id = 'stub-agent'

    def search_flights(self, destination):
        return [{'destination': destination}]

    def display_flights(self, flights):
        pass
`;

const PSQL_STUB = `#!/bin/sh
printf 'psql %s\\n' "$*" >> "$STUB_LOG"
cat > /dev/null
`;

const JQ_STUB = `#!/bin/sh
printf '%s\\n' "$STUB_TOKEN_ID"
`;

interface Stack {
  name: string;
  env?: Record<string, string>;
  dotenv?: string;
  backend: string;
  frontend: string;
  postgres: string;
}

const DOTENV = 'AIM_PROJECT=dotlab\nAIM_BACKEND_PORT=18081\nAIM_FRONTEND_PORT=13001\nAIM_POSTGRES_PORT=15433\n';

const STACKS: Stack[] = [
  { name: 'nothing set', backend: '8080', frontend: '3000', postgres: '5432' },
  {
    name: 'the environment',
    env: { AIM_BACKEND_PORT: '18080', AIM_FRONTEND_PORT: '13000', AIM_POSTGRES_PORT: '15432' },
    backend: '18080',
    frontend: '13000',
    postgres: '15432',
  },
  { name: "the repository's .env", dotenv: DOTENV, backend: '18081', frontend: '13001', postgres: '15433' },
  {
    name: 'the environment over .env',
    dotenv: DOTENV,
    env: { AIM_FRONTEND_PORT: '13002', AIM_POSTGRES_PORT: '15434' },
    backend: '18081',
    frontend: '13002',
    postgres: '15434',
  },
];

interface Run {
  status: number | null;
  output: string;
  calls: string[];
  /** ~/.aim/credentials.json after the run, or null when there is none. */
  credentialsAfter: string | null;
}

let scratch = '';
let caseCount = 0;

beforeAll(() => {
  scratch = mkdtempSync(join(tmpdir(), 'flight-demo-helpers-'));
  const bin = join(scratch, 'bin');
  mkdirSync(bin);
  for (const [name, text] of [
    ['psql', PSQL_STUB],
    ['jq', JQ_STUB],
  ] as const) {
    writeFileSync(join(bin, name), text, { mode: 0o755 });
    chmodSync(join(bin, name), 0o755);
  }
});

afterAll(() => {
  if (scratch) rmSync(scratch, { recursive: true, force: true });
});

/**
 * Runs one command in a fresh scratch tree. With fakeAgent, flight_agent.py is
 * the stand-in above instead of the real one; with postOk, a POST answers 200.
 */
function run(
  argv: string[],
  opts: { stack?: Stack; credentials?: Record<string, string>; fakeAgent?: boolean; postOk?: boolean } = {},
): Run {
  const root = join(scratch, `case-${++caseCount}`);
  const demo = join(root, 'examples', 'flight-search-agent');
  const home = join(root, 'home');
  const sdk = join(demo, 'aim-sdk-python', 'aim_sdk');
  mkdirSync(sdk, { recursive: true });
  mkdirSync(join(home, '.aim'), { recursive: true });
  for (const s of SCRIPTS) {
    // A script the example does not have yet is left out, so the cases below
    // fail on what the others do, not on a missing copy.
    if (!existsSync(join(DEMO_DIR, s))) continue;
    copyFileSync(join(DEMO_DIR, s), join(demo, s));
    chmodSync(join(demo, s), 0o755);
  }
  if (opts.fakeAgent) writeFileSync(join(demo, 'flight_agent.py'), FAKE_AGENT);
  writeFileSync(join(demo, 'requests.py'), FAKE_REQUESTS);
  for (const [name, text] of Object.entries(FAKE_SDK)) writeFileSync(join(sdk, name), text);
  const credentialsPath = join(home, '.aim', 'credentials.json');
  if (opts.credentials) writeFileSync(credentialsPath, JSON.stringify(opts.credentials));
  if (opts.stack?.dotenv !== undefined) writeFileSync(join(root, '.env'), opts.stack.dotenv);
  const log = join(root, 'calls.log');
  writeFileSync(log, '');

  const result = spawnSync(argv[0], argv.slice(1), {
    cwd: demo,
    encoding: 'utf-8',
    env: {
      PATH: `${join(scratch, 'bin')}:${process.env.PATH ?? ''}`,
      HOME: home,
      LANG: 'en_US.UTF-8',
      PYTHONIOENCODING: 'utf-8',
      PYTHONDONTWRITEBYTECODE: '1',
      STUB_LOG: log,
      STUB_TOKEN_ID: 'tok-1',
      ...(opts.postOk ? { STUB_POST_OK: '1' } : {}),
      ...(opts.stack?.env ?? {}),
    },
  });
  if (result.error) throw result.error;
  return {
    status: result.status,
    output: `${result.stdout}${result.stderr}`,
    calls: readFileSync(log, 'utf-8').split('\n').filter((l) => l !== ''),
    credentialsAfter: existsSync(credentialsPath) ? readFileSync(credentialsPath, 'utf-8') : null,
  };
}

/** Every localhost port the output names. */
function localhostPorts(output: string): string[] {
  return [...output.matchAll(/localhost:(\d+)/g)].map((m) => m[1]);
}

describe.each(STACKS)('the flight-search demo helpers with $name', (stack) => {
  const backend = `http://localhost:${stack.backend}`;
  const frontend = `http://localhost:${stack.frontend}`;

  it('get_fresh_sdk.py sends the operator to sign in and download at the dashboard port, and calls nothing', () => {
    const r = run(['python3', 'get_fresh_sdk.py'], { stack, credentials: { refresh_token: 'not-a-real-token' } });
    expect(r.status, r.output).toBe(0);
    expect(r.calls).toEqual([]);
    expect(r.output).toContain(`${frontend}/auth/login`);
    expect(r.output).toContain(`${frontend}/dashboard/sdk`);
    for (const port of localhostPorts(r.output)) expect(port).toBe(stack.frontend);
  });

  it('verify_qa_complete.py queries the backend port and points at the dashboard port', () => {
    const code = [
      'import time',
      'time.sleep = lambda seconds: None',
      'import verify_qa_complete as qa',
      "client = type('Client', (), {'agent_id': 'stub-agent'})()",
      'qa.verify_dashboard_data(client)',
      'qa.verify_capabilities(client)',
      "qa.print_final_summary({'all checks': True})",
    ].join('\n');
    const r = run(['python3', '-c', code], { stack, credentials: { access_token: 'not-a-real-token' } });
    expect(r.status, r.output).toBe(0);
    expect(r.calls).toEqual([
      `GET ${backend}/api/v1/agents/stub-agent/verification-events`,
      `GET ${backend}/api/v1/agents/stub-agent`,
    ]);
    expect(r.output).toContain(`${frontend}/dashboard/agents`);
    for (const port of localhostPorts(r.output)) expect(port).toBe(stack.frontend);
  });

  it('demo_search.py and test_flight_agent.py point at the dashboard port', () => {
    for (const script of ['demo_search.py', 'test_flight_agent.py']) {
      const r = run(['python3', script], { stack, fakeAgent: true });
      expect(r.status, `${script}: ${r.output}`).toBe(0);
      expect(r.output, script).toContain(`${frontend}/dashboard`);
      for (const port of localhostPorts(r.output)) expect(port, script).toBe(stack.frontend);
    }
  });

  it('check_sdk_token.sh runs psql on the PostgreSQL port', () => {
    const r = run(['bash', 'check_sdk_token.sh'], { stack, credentials: { sdk_token_id: 'tok-1' } });
    expect(r.status, r.output).toBe(0);
    expect(r.calls).toEqual([
      `psql -h localhost -p ${stack.postgres} -U postgres -d identity -v sdk_token_id=tok-1`,
    ]);
  });
});

/**
 * GET /api/v1/sdk/download mints a credential only for an access token from a
 * sign-in; an access token minted by refreshing an SDK-download token is
 * refused there. get_fresh_sdk.py refreshed the stored credential, wrote the
 * new refresh token back and posted to /api/v1/sdk/download/python, a path the
 * backend does not register, with the access token the refresh returned.
 */
describe('get_fresh_sdk.py', () => {
  it('neither refreshes nor rewrites a stored credential the backend would refresh', () => {
    const credentials = { refresh_token: 'not-a-real-token' };
    const r = run(['python3', 'get_fresh_sdk.py'], { credentials, postOk: true });
    expect(r.status, r.output).toBe(0);
    expect(r.calls).toEqual([]);
    expect(r.credentialsAfter).toBe(JSON.stringify(credentials));
    expect(r.output).toContain('http://localhost:3000/auth/login');
    expect(r.output).toContain('http://localhost:3000/dashboard/sdk');
    expect(r.output).toContain('Download Python SDK');
  });
});

describe('the flight-search agent', () => {
  it("names the client's backend URL and the /health path when a deny does not land", () => {
    const code = [
      'import flight_agent',
      'agent = flight_agent.FlightAgent.__new__(flight_agent.FlightAgent)',
      "agent.client = type('Client', (), {'aim_url': 'http://localhost:18080', 'verify_capability': lambda self, **kw: {'status': 'pending'}})()",
      "assert agent.inject_attack('data-exfil') is False",
    ].join('\n');
    const r = run(['python3', '-c', code]);
    expect(r.status, r.output).toBe(0);
    expect(r.output).toContain('curl -sf http://localhost:18080/health');
    expect(r.output).not.toContain('healthz');
    expect(localhostPorts(r.output)).toEqual(['18080']);
  });
});

describe('the flight-search example scripts', () => {
  it('name no fixed host port or container', () => {
    const scripts = readdirSync(DEMO_DIR).filter((f) => /\.(py|sh)$/.test(f));
    expect(scripts.length).toBeGreaterThan(0);
    for (const s of scripts) {
      const text = readFileSync(join(DEMO_DIR, s), 'utf-8');
      expect(text.match(/localhost:\d+/g), s).toBeNull();
      expect(text, s).not.toContain('aim-postgres');
      expect(text, s).not.toContain('/healthz');
    }
  });
});

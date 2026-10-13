import { describe, it, expect, beforeAll, afterAll } from 'vitest';
import { copyFileSync, existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'fs';
import { tmpdir } from 'os';
import { join } from 'path';
import { spawnSync } from 'child_process';

/**
 * The A2A multi-agent demo's Python scripts reach the stack the compose files
 * name.
 *
 * docker-compose.yml and docker-compose.quickstart.yml take the backend's host
 * port from AIM_BACKEND_PORT (default 8080) and the dashboard's from
 * AIM_FRONTEND_PORT (3000). run_demo.py defaulted --aim-url to localhost:8080
 * when AIM_URL was not set, and printed its dashboard links on the backend URL;
 * research_agent.py and analysis_agent.py fell back to localhost:8080 when given
 * an API token and no URL. Against a stack started on other ports they reached
 * nothing, or, beside a default stack on the same host, that stack.
 *
 * Each case copies the demo into a scratch tree shaped like the repository
 * (examples/a2a-multi-agent-demo/python/ three levels below the .env, the SDK
 * at sdk/python/) with stand-ins for the AIM SDK and requests, so every backend
 * URL the demo uses and every URL it prints is recorded.
 */

const REPO_ROOT = join(__dirname, '..', '..', '..');
const DEMO_DIR = join(REPO_ROOT, 'examples', 'a2a-multi-agent-demo', 'python');
const DEMO_FILES = ['run_demo.py', 'research_agent.py', 'analysis_agent.py', 'stack_env.py'];

/** Records each registration and the backend URL it used. */
const FAKE_SDK = `import os


def _log(line):
    with open(os.environ['STUB_LOG'], 'a') as f:
        f.write(line + '\\n')


class AIMClient:
    def __init__(self, agent_id, api_key=None, aim_url=None, **kwargs):
        self.agent_id = agent_id
        _log('client ' + agent_id + ' ' + str(aim_url))


class _Registered:
    def __init__(self, agent_id):
        self.agent_id = agent_id


def register_agent(name, aim_url=None, **kwargs):
    _log('register ' + name + ' ' + str(aim_url))
    return _Registered(name + '-stub-id')
`;

/** Every A2A call fails, so the demo takes its offline path through each step. */
const FAKE_A2A = `class A2ATrustScore:
    pass


class A2AClient:
    def __init__(self, client):
        self.client = client

    def __getattr__(self, name):
        def call(*args, **kwargs):
            raise RuntimeError('A2A stand-in: ' + name)
        return call
`;

/** Records each REST call; answers the two the token path makes. */
const FAKE_REQUESTS = `import json
import os


class _Response:
    def __init__(self, status_code, body):
        self.status_code = status_code
        self._body = body
        self.text = json.dumps(body)

    def json(self):
        return self._body


def _log(line):
    with open(os.environ['STUB_LOG'], 'a') as f:
        f.write(line + '\\n')


class Session:
    def __init__(self):
        self.headers = {}

    def get(self, url, **kwargs):
        _log('GET ' + url)
        return _Response(200, {'organizations': [{'id': 'org-stub'}]})

    def post(self, url, json=None, **kwargs):
        _log('POST ' + url)
        return _Response(201, {'id': json['name'] + '-stub-id', 'apiKey': {'key': 'not-a-real-key'}})
`;

interface Stack {
  name: string;
  env?: Record<string, string>;
  dotenv?: string;
  backend: string;
  frontend: string;
}

const DOTENV = 'AIM_PROJECT=dotlab\nAIM_BACKEND_PORT=18282\nAIM_FRONTEND_PORT=13232\n';

const STACKS: Stack[] = [
  { name: 'nothing set', backend: '8080', frontend: '3000' },
  {
    name: 'the environment',
    env: { AIM_BACKEND_PORT: '18181', AIM_FRONTEND_PORT: '13131' },
    backend: '18181',
    frontend: '13131',
  },
  { name: "the repository's .env", dotenv: DOTENV, backend: '18282', frontend: '13232' },
  {
    name: 'the environment over .env',
    dotenv: DOTENV,
    env: { AIM_FRONTEND_PORT: '13333' },
    backend: '18282',
    frontend: '13333',
  },
];

const API_TOKEN = { AIM_API_TOKEN: 'not-a-real-token' };

const DASHBOARD_TABS = ['', '?tab=cards', '?tab=consent', '?tab=tasks', '?tab=trust', '?tab=skills'];

interface Run {
  status: number | null;
  output: string;
  calls: string[];
}

let scratch = '';
let caseCount = 0;

beforeAll(() => {
  scratch = mkdtempSync(join(tmpdir(), 'a2a-demo-'));
});

afterAll(() => {
  if (scratch) rmSync(scratch, { recursive: true, force: true });
});

/** Runs one command in the demo directory of a fresh scratch tree. */
function run(argv: string[], opts: { stack?: Stack; env?: Record<string, string> } = {}): Run {
  const root = join(scratch, `case-${++caseCount}`);
  const demo = join(root, 'examples', 'a2a-multi-agent-demo', 'python');
  mkdirSync(demo, { recursive: true });
  // A file the demo does not have yet is left out.
  for (const f of DEMO_FILES) {
    if (existsSync(join(DEMO_DIR, f))) copyFileSync(join(DEMO_DIR, f), join(demo, f));
  }

  const sdk = join(root, 'sdk', 'python');
  mkdirSync(join(sdk, 'aim_sdk'), { recursive: true });
  writeFileSync(join(sdk, 'aim_sdk', '__init__.py'), FAKE_SDK);
  writeFileSync(join(sdk, 'aim_sdk', 'a2a.py'), FAKE_A2A);
  writeFileSync(join(demo, 'requests.py'), FAKE_REQUESTS);

  if (opts.stack?.dotenv !== undefined) writeFileSync(join(root, '.env'), opts.stack.dotenv);
  const log = join(root, 'calls.log');
  writeFileSync(log, '');

  const result = spawnSync(argv[0], argv.slice(1), {
    cwd: demo,
    encoding: 'utf-8',
    env: {
      PATH: process.env.PATH ?? '',
      HOME: join(root, 'home'),
      LANG: 'en_US.UTF-8',
      PYTHONIOENCODING: 'utf-8',
      PYTHONDONTWRITEBYTECODE: '1',
      STUB_LOG: log,
      ...(opts.stack?.env ?? {}),
      ...(opts.env ?? {}),
    },
  });
  if (result.error) throw result.error;
  return {
    status: result.status,
    output: `${result.stdout}${result.stderr}`,
    calls: readFileSync(log, 'utf-8').split('\n').filter((l) => l !== ''),
  };
}

/** Every localhost port the output names. */
function localhostPorts(output: string): string[] {
  return [...output.matchAll(/localhost:(\d+)/g)].map((m) => m[1]);
}

function expectDashboardLinks(output: string, frontend: string): void {
  for (const tab of DASHBOARD_TABS) expect(output).toContain(`${frontend}/dashboard/a2a${tab}\n`);
}

describe.each(STACKS)('the A2A demo with $name', (stack) => {
  const backend = `http://localhost:${stack.backend}`;
  const frontend = `http://localhost:${stack.frontend}`;

  it('run_demo.py registers both agents at the backend port and links the dashboard port', () => {
    const r = run(['python3', 'run_demo.py'], { stack });
    expect(r.status, r.output).toBe(0);
    expect(r.calls).toEqual([`register analysis-agent ${backend}`, `register research-agent ${backend}`]);
    expectDashboardLinks(r.output, frontend);
    for (const port of localhostPorts(r.output)) expect(port).toBe(stack.frontend);
  });

  it('run_demo.py with an API token registers both agents through the backend port', () => {
    const r = run(['python3', 'run_demo.py'], { stack, env: API_TOKEN });
    expect(r.status, r.output).toBe(0);
    const forAgent = (name: string) => [
      `GET ${backend}/api/v1/organizations`,
      `POST ${backend}/api/v1/agents`,
      expect.stringMatching(new RegExp(`^client ${name}-\\d+-stub-id ${backend}$`)),
    ];
    expect(r.calls).toEqual([...forAgent('analysis-agent'), ...forAgent('research-agent')]);
    expectDashboardLinks(r.output, frontend);
  });

  it('run_demo.py --help names the backend port as the default', () => {
    const r = run(['python3', 'run_demo.py', '--help'], { stack });
    expect(r.status, r.output).toBe(0);
    expect(r.output.replace(/\s+/g, ' ')).toContain(`${backend}`);
    for (const port of localhostPorts(r.output)) expect(port).toBe(stack.backend);
  });

  it.each(['AnalysisAgent', 'ResearchAgent'])('%s given an API token and no URL registers at the backend port', (cls) => {
    const module = cls === 'AnalysisAgent' ? 'analysis_agent' : 'research_agent';
    const r = run(['python3', '-c', `from ${module} import ${cls}; ${cls}(api_token='not-a-real-token')`], { stack });
    expect(r.status, r.output).toBe(0);
    expect(r.calls.slice(0, 2)).toEqual([`GET ${backend}/api/v1/organizations`, `POST ${backend}/api/v1/agents`]);
    expect(r.calls[2]).toMatch(new RegExp(` ${backend}$`));
  });
});

describe('the A2A demo with a backend URL given', () => {
  const env = { AIM_BACKEND_PORT: '18181', AIM_FRONTEND_PORT: '13131' };

  it('AIM_URL wins over AIM_BACKEND_PORT for the backend; the links use the dashboard port', () => {
    const r = run(['python3', 'run_demo.py'], { env: { ...env, AIM_URL: 'http://localhost:18999' } });
    expect(r.status, r.output).toBe(0);
    expect(r.calls).toEqual(['register analysis-agent http://localhost:18999', 'register research-agent http://localhost:18999']);
    expectDashboardLinks(r.output, 'http://localhost:13131');
    for (const port of localhostPorts(r.output)) expect(port).toBe('13131');
  });

  it('--aim-url wins over AIM_URL', () => {
    const r = run(['python3', 'run_demo.py', '--aim-url', 'http://localhost:18998'], {
      env: { ...env, AIM_URL: 'http://localhost:18999' },
    });
    expect(r.status, r.output).toBe(0);
    expect(r.calls).toEqual(['register analysis-agent http://localhost:18998', 'register research-agent http://localhost:18998']);
    expectDashboardLinks(r.output, 'http://localhost:13131');
  });
});

describe("the A2A demo's Python scripts", () => {
  it('name no fixed backend or dashboard port outside comments and docstrings', () => {
    for (const f of ['run_demo.py', 'research_agent.py', 'analysis_agent.py']) {
      const lines = readFileSync(join(DEMO_DIR, f), 'utf-8').split('\n');
      const fixed = lines.filter((l) => /["'][^"']*localhost:(8080|3000)\b/.test(l));
      expect(fixed, f).toEqual([]);
    }
  });
});

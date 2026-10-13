import { describe, it, expect, beforeAll, afterAll } from 'vitest';
import { chmodSync, copyFileSync, existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'fs';
import { tmpdir } from 'os';
import { join } from 'path';
import { spawnSync } from 'child_process';

/**
 * The LangChain CRUD demo and the MCP server demo reach the stack the compose
 * files name.
 *
 * docker-compose.yml and docker-compose.quickstart.yml take the backend's host
 * port from AIM_BACKEND_PORT (default 8080) and the dashboard's from
 * AIM_FRONTEND_PORT (3000). run-langchain-crud.sh checked the backend's health
 * on localhost:8080 whatever AIM_API_URL said, and printed dashboard links on
 * localhost:3000; langchain_crud_agent.py defaulted to localhost:8080 and built
 * its dashboard link by replacing "8080" with "3000" in the backend URL. Against
 * a stack started on other ports they reached nothing, or, beside a default
 * stack on the same host, that stack. mcp-server.py named the dashboard's MCP
 * page only in its docstring, on localhost:3000.
 *
 * Each case copies the demos into a scratch tree shaped like the repository
 * (examples/<demo>/ two levels below the .env) with stand-ins for the AIM SDK,
 * LangChain, Flask, PyNaCl, curl and pip, so every backend URL the demos use
 * and every URL they print is recorded.
 */

const REPO_ROOT = join(__dirname, '..', '..', '..');
const LANGCHAIN_DIR = join(REPO_ROOT, 'examples', 'langchain-crud-agent');
const MCP_DIR = join(REPO_ROOT, 'examples', 'mcp-server-demo');
const LANGCHAIN_FILES = ['run-langchain-crud.sh', 'langchain_crud_agent.py', 'stack_env.py'];
const MCP_FILES = ['mcp-server.py'];

/** Records the backend URL the agent registers against; every action runs. */
const FAKE_SDK = `import os


class _Client:
    agent_id = 'stub-agent'

    def perform_action(self, *args, **kwargs):
        return lambda f: f


def secure(name, aim_url=None, api_key=None, **kwargs):
    with open(os.environ['STUB_LOG'], 'a') as f:
        f.write('secure ' + name + ' ' + str(aim_url) + '\\n')
    return _Client()
`;

/** Keeps the agent in its direct mode, whether or not LangChain is installed. */
const NO_LANGCHAIN = "raise ImportError('LangChain stand-in: not installed')\n";

const FAKE_FLASK = `class Flask:
    def __init__(self, name):
        pass

    def route(self, *args, **kwargs):
        return lambda f: f

    def run(self, **kwargs):
        pass


request = None


def jsonify(*args, **kwargs):
    return None
`;

const FAKE_NACL_SIGNING = `class _Key:
    def __init__(self, raw):
        self._raw = raw

    def encode(self, encoder=None):
        return self._raw


class SigningKey(_Key):
    @classmethod
    def generate(cls):
        key = cls(b'not-a-real-signing-key')
        key.verify_key = _Key(b'not-a-real-public-key')
        return key
`;

const CURL_STUB = `#!/bin/sh
printf 'curl %s\\n' "$*" >> "$STUB_LOG"
exit "\${STUB_CURL_STATUS:-0}"
`;

const PIP_STUB = `#!/bin/sh
printf 'pip %s\\n' "$*" >> "$STUB_LOG"
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

const API_KEY = { AIM_API_KEY: 'not-a-real-key' };

interface Run {
  status: number | null;
  output: string;
  calls: string[];
}

let scratch = '';
let caseCount = 0;

beforeAll(() => {
  scratch = mkdtempSync(join(tmpdir(), 'langchain-mcp-demos-'));
  const bin = join(scratch, 'bin');
  mkdirSync(bin);
  for (const [name, text] of [
    ['curl', CURL_STUB],
    ['pip', PIP_STUB],
  ] as const) {
    writeFileSync(join(bin, name), text, { mode: 0o755 });
    chmodSync(join(bin, name), 0o755);
  }
});

afterAll(() => {
  if (scratch) rmSync(scratch, { recursive: true, force: true });
});

/** Copies the files a demo has into dest; one it does not have yet is left out. */
function copyDemo(from: string, files: string[], dest: string): void {
  mkdirSync(dest, { recursive: true });
  for (const f of files) {
    if (!existsSync(join(from, f))) continue;
    copyFileSync(join(from, f), join(dest, f));
    chmodSync(join(dest, f), 0o755);
  }
}

/** Runs one command in a demo directory of a fresh scratch tree. */
function run(demo: 'langchain' | 'mcp', argv: string[], opts: { stack?: Stack; env?: Record<string, string> } = {}): Run {
  const root = join(scratch, `case-${++caseCount}`);
  const langchain = join(root, 'examples', 'langchain-crud-agent');
  const mcp = join(root, 'examples', 'mcp-server-demo');

  copyDemo(LANGCHAIN_DIR, LANGCHAIN_FILES, langchain);
  const sdk = join(langchain, 'aim-sdk-python');
  mkdirSync(join(sdk, 'aim_sdk'), { recursive: true });
  mkdirSync(join(sdk, 'langchain'));
  writeFileSync(join(sdk, 'aim_sdk', '__init__.py'), FAKE_SDK);
  writeFileSync(join(sdk, 'langchain', '__init__.py'), NO_LANGCHAIN);
  // An existing venv whose activate changes nothing, so the stubs stay on PATH.
  mkdirSync(join(langchain, 'venv', 'bin'), { recursive: true });
  writeFileSync(join(langchain, 'venv', 'bin', 'activate'), '');

  copyDemo(MCP_DIR, MCP_FILES, mcp);
  mkdirSync(join(mcp, 'nacl'));
  writeFileSync(join(mcp, 'flask.py'), FAKE_FLASK);
  writeFileSync(join(mcp, 'nacl', '__init__.py'), '');
  writeFileSync(join(mcp, 'nacl', 'signing.py'), FAKE_NACL_SIGNING);
  writeFileSync(join(mcp, 'nacl', 'encoding.py'), 'class Base64Encoder:\n    pass\n');

  if (opts.stack?.dotenv !== undefined) writeFileSync(join(root, '.env'), opts.stack.dotenv);
  const log = join(root, 'calls.log');
  writeFileSync(log, '');

  const result = spawnSync(argv[0], argv.slice(1), {
    cwd: demo === 'langchain' ? langchain : mcp,
    encoding: 'utf-8',
    env: {
      PATH: `${join(scratch, 'bin')}:${process.env.PATH ?? ''}`,
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

describe.each(STACKS)('the LangChain CRUD demo with $name', (stack) => {
  const backend = `http://localhost:${stack.backend}`;
  const frontend = `http://localhost:${stack.frontend}`;

  it('run-langchain-crud.sh checks the backend port, runs the agent there and links the dashboard port', () => {
    const r = run('langchain', ['bash', 'run-langchain-crud.sh'], { stack, env: API_KEY });
    expect(r.status, r.output).toBe(0);
    expect(r.calls).toEqual([
      `curl -sf ${backend}/health`,
      'pip install -q langchain langchain-google-genai langchain-core requests',
      `secure langchain-crud-agent ${backend}`,
    ]);
    expect(r.output).toContain(`${frontend}/dashboard/agents/stub-agent`);
    expect(r.output).toContain(`${frontend}/dashboard/agents\n`);
    for (const port of localhostPorts(r.output)) expect([stack.backend, stack.frontend]).toContain(port);
  });

  it('run-langchain-crud.sh without an API key points at the dashboard port and suggests the backend URL', () => {
    const r = run('langchain', ['bash', 'run-langchain-crud.sh'], { stack });
    expect(r.status, r.output).toBe(1);
    expect(r.calls).toEqual([]);
    expect(r.output).toContain(`${frontend}/dashboard/settings/api-keys`);
    expect(r.output).toContain(`AIM_API_URL=${backend}`);
    for (const port of localhostPorts(r.output)) expect([stack.backend, stack.frontend]).toContain(port);
  });

  it('run-langchain-crud.sh names the health check it tried when the backend does not answer', () => {
    const r = run('langchain', ['bash', 'run-langchain-crud.sh'], { stack, env: { ...API_KEY, STUB_CURL_STATUS: '7' } });
    expect(r.status, r.output).toBe(1);
    expect(r.calls).toEqual([`curl -sf ${backend}/health`]);
    expect(r.output).toContain(`curl -sf ${backend}/health`);
  });

  it('langchain_crud_agent.py run on its own registers at the backend port and links the dashboard port', () => {
    const r = run('langchain', ['python3', 'langchain_crud_agent.py'], { stack, env: API_KEY });
    expect(r.status, r.output).toBe(0);
    expect(r.calls).toEqual([`secure langchain-crud-agent ${backend}`]);
    expect(r.output).toContain(`${frontend}/dashboard/agents/stub-agent`);
    for (const port of localhostPorts(r.output)) expect(port).toBe(stack.frontend);
  });

  it('langchain_crud_agent.py without an API key points at the dashboard port', () => {
    const r = run('langchain', ['python3', 'langchain_crud_agent.py'], { stack });
    expect(r.status, r.output).toBe(1);
    expect(r.calls).toEqual([]);
    expect(r.output).toContain(`${frontend}/dashboard/settings/api-keys`);
    for (const port of localhostPorts(r.output)) expect(port).toBe(stack.frontend);
  });
});

describe('the LangChain CRUD demo with AIM_API_URL set', () => {
  it('uses that URL for the health check and the agent, and the dashboard port for the links', () => {
    const env = { ...API_KEY, AIM_API_URL: 'http://localhost:18999', AIM_BACKEND_PORT: '18181', AIM_FRONTEND_PORT: '13131' };
    const r = run('langchain', ['bash', 'run-langchain-crud.sh'], { env });
    expect(r.status, r.output).toBe(0);
    expect(r.calls).toEqual([
      'curl -sf http://localhost:18999/health',
      'pip install -q langchain langchain-google-genai langchain-core requests',
      'secure langchain-crud-agent http://localhost:18999',
    ]);
    expect(r.output).toContain('http://localhost:13131/dashboard/agents/stub-agent');
    for (const port of localhostPorts(r.output)) expect(['18999', '13131']).toContain(port);
  });
});

describe.each(STACKS)('the MCP server demo with $name', (stack) => {
  it('mcp-server.py prints the dashboard MCP page on the dashboard port', () => {
    const r = run('mcp', ['python3', 'mcp-server.py'], { stack });
    expect(r.status, r.output).toBe(0);
    expect(r.output).toContain(`Open: http://localhost:${stack.frontend}/dashboard/mcp\n`);
    for (const port of localhostPorts(r.output)) expect([stack.frontend, '5151']).toContain(port);
  });
});

describe('the LangChain CRUD and MCP server demos', () => {
  it('print no fixed backend or dashboard port', () => {
    const files = [
      ...LANGCHAIN_FILES.map((f) => join(LANGCHAIN_DIR, f)),
      ...MCP_FILES.map((f) => join(MCP_DIR, f)),
    ];
    for (const file of files) {
      const lines = readFileSync(file, 'utf-8').split('\n');
      const fixed = lines.filter((l) => /\b(echo|print)\b/.test(l) && /localhost:(8080|3000)\b/.test(l));
      expect(fixed, file).toEqual([]);
    }
    const launcher = readFileSync(join(LANGCHAIN_DIR, 'run-langchain-crud.sh'), 'utf-8');
    expect(launcher.match(/localhost:\d+/g)).toBeNull();
  });
});

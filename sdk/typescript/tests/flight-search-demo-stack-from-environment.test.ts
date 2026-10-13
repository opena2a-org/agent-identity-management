import { describe, it, expect, beforeAll, afterAll } from 'vitest';
import { chmodSync, copyFileSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'fs';
import { tmpdir } from 'os';
import { join } from 'path';
import { spawnSync } from 'child_process';

/**
 * The flight-search demo's stage scripts reach the stack the compose files
 * name.
 *
 * docker-compose.yml and docker-compose.quickstart.yml take the container-name
 * prefix from AIM_PROJECT (default aim) and the backend's host port from
 * AIM_BACKEND_PORT (default 8080). examples/flight-search-agent/reset-demo.sh
 * and prep-stage.sh ran every query in the fixed container aim-postgres and
 * called the backend on localhost:8080, so against a stack started with other
 * values they failed with "No such container", or, beside a default stack,
 * reset and seeded that stack instead. They now source stack-env.sh, which
 * reads both variables from the environment, then the repository's .env, then
 * the defaults.
 *
 * Each case copies the three scripts into a scratch tree shaped like the
 * repository (examples/flight-search-agent/ two levels below the .env) and
 * runs prep-stage.sh end to end against docker and curl stubs, a fake
 * virtualenv and a fake FlightAgent, so every docker exec and every backend
 * call the scripts make is recorded.
 */

const REPO_ROOT = join(__dirname, '..', '..', '..');
const DEMO_DIR = join(REPO_ROOT, 'examples', 'flight-search-agent');
const SCRIPTS = ['stack-env.sh', 'reset-demo.sh', 'prep-stage.sh'];
const DEMO_README = join(DEMO_DIR, 'README.md');

const DOCKER_STUB = `#!/bin/sh
printf 'docker %s %s %s\\n' "$1" "$2" "$3" >> "$STUB_LOG"
`;

const CURL_STUB = `#!/bin/sh
out=""
url=""
while [ $# -gt 0 ]; do
  case "$1" in
    -o) out="$2"; shift ;;
    -H|-d|-X) shift ;;
    http://*|https://*) url="$1" ;;
  esac
  shift
done
printf 'curl %s\\n' "$url" >> "$STUB_LOG"
if [ -n "$out" ]; then cp "$STUB_ZIP" "$out"; else printf '%s' "$STUB_LOGIN_BODY"; fi
`;

/** Stands in for the real agent: registers nothing, writes the cache file. */
const FAKE_AGENT = `import json, pathlib


class FlightAgent:
    def __init__(self):
        p = pathlib.Path.home() / '.aim' / 'agents' / 'flight-search-agent.json'
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_text(json.dumps({'status': 'pending'}))
        self.client = object()
        self.agent_id = 'stub-agent'
`;

const LOGIN_OK = JSON.stringify({ accessToken: 'not-a-real-token' });

interface DemoRun {
  status: number | null;
  output: string;
  docker: string[];
  curl: string[];
}

let scratch = '';
let caseCount = 0;

beforeAll(() => {
  scratch = mkdtempSync(join(tmpdir(), 'flight-demo-stack-'));
  const bin = join(scratch, 'bin');
  mkdirSync(bin);
  for (const [name, text] of [
    ['docker', DOCKER_STUB],
    ['curl', CURL_STUB],
  ] as const) {
    writeFileSync(join(bin, name), text, { mode: 0o755 });
    chmodSync(join(bin, name), 0o755);
  }
  const zipped = spawnSync(
    'python3',
    [
      '-c',
      'import sys, zipfile\n' +
        'with zipfile.ZipFile(sys.argv[1], "w") as z:\n' +
        '    z.writestr("aim-sdk-python/.aim/sdk_credentials.json", "{}")\n',
      join(scratch, 'sdk.zip'),
    ],
    { encoding: 'utf-8' },
  );
  if (zipped.status !== 0) throw new Error(`could not build the SDK zip: ${zipped.stderr}`);
});

afterAll(() => {
  if (scratch) rmSync(scratch, { recursive: true, force: true });
});

function runDemo(opts: { env?: Record<string, string>; dotenv?: string; loginBody?: string }): DemoRun {
  const root = join(scratch, `case-${++caseCount}`);
  const demo = join(root, 'examples', 'flight-search-agent');
  const home = join(root, 'home');
  mkdirSync(join(demo, '.venv', 'bin'), { recursive: true });
  mkdirSync(join(home, '.aim'), { recursive: true });
  mkdirSync(join(root, 'tmp'));
  for (const s of SCRIPTS) {
    copyFileSync(join(DEMO_DIR, s), join(demo, s));
    chmodSync(join(demo, s), 0o755);
  }
  writeFileSync(join(demo, '.venv', 'bin', 'activate'), '');
  writeFileSync(join(demo, 'flight_agent.py'), FAKE_AGENT);
  if (opts.dotenv !== undefined) writeFileSync(join(root, '.env'), opts.dotenv);
  const log = join(root, 'calls.log');
  writeFileSync(log, '');

  const result = spawnSync('bash', [join(demo, 'prep-stage.sh')], {
    encoding: 'utf-8',
    env: {
      PATH: `${join(scratch, 'bin')}:${process.env.PATH ?? ''}`,
      HOME: home,
      TMPDIR: join(root, 'tmp'),
      LANG: 'en_US.UTF-8',
      PYTHONDONTWRITEBYTECODE: '1',
      STUB_LOG: log,
      STUB_ZIP: join(scratch, 'sdk.zip'),
      STUB_LOGIN_BODY: opts.loginBody ?? LOGIN_OK,
      ...(opts.env ?? {}),
    },
  });
  if (result.error) throw result.error;
  const calls = readFileSync(log, 'utf-8').split('\n').filter((l) => l !== '');
  return {
    status: result.status,
    output: `${result.stdout}${result.stderr}`,
    docker: calls.filter((c) => c.startsWith('docker ')),
    curl: calls.filter((c) => c.startsWith('curl ')),
  };
}

/** How many docker exec calls the two scripts make on a complete run: one per line that runs one. */
function expectedDockerCalls(): number {
  const text = ['reset-demo.sh', 'prep-stage.sh'].map((s) => readFileSync(join(DEMO_DIR, s), 'utf-8')).join('\n');
  return text.split('\n').filter((l) => l.includes('docker exec ') && !/^\s*(#|echo )/.test(l)).length;
}

function expectStack(r: DemoRun, project: string, port: string): void {
  expect(r.status, `exit status (output: ${r.output})`).toBe(0);
  expect(r.docker.length, 'one recorded call per docker exec in the scripts').toBe(expectedDockerCalls());
  for (const call of r.docker) expect(call).toBe(`docker exec ${project}-postgres psql`);
  expect(r.curl).toEqual([
    `curl http://localhost:${port}/api/v1/public/login`,
    `curl http://localhost:${port}/api/v1/sdk/download?sdk=python`,
  ]);
  expect(r.output, 'the strict-mode command names the same container').toContain(
    `docker exec ${project}-postgres psql -U postgres -d identity -c`,
  );
}

describe('the flight-search demo stage scripts reach the stack the compose files name', () => {
  it('use aim-postgres and port 8080 when nothing is set', () => {
    expectStack(runDemo({}), 'aim', '8080');
  });

  it('use AIM_PROJECT and AIM_BACKEND_PORT from the environment', () => {
    expectStack(runDemo({ env: { AIM_PROJECT: 'lab', AIM_BACKEND_PORT: '18080' } }), 'lab', '18080');
  });

  it("fall back to the repository's .env, as docker compose does", () => {
    const dotenv = 'AIM_FRONTEND_PORT=13000\nAIM_PROJECT=dotlab\nAIM_BACKEND_PORT=18081\n';
    expectStack(runDemo({ dotenv }), 'dotlab', '18081');
  });

  it('let the environment win over .env', () => {
    const dotenv = 'AIM_PROJECT=dotlab\nAIM_BACKEND_PORT=18081\n';
    expectStack(runDemo({ dotenv, env: { AIM_BACKEND_PORT: '18082' } }), 'dotlab', '18082');
  });

  it('name the backend URL they tried when the login gets no answer', () => {
    const r = runDemo({ env: { AIM_BACKEND_PORT: '18080' }, loginBody: '' });
    expect(r.status).toBe(1);
    expect(r.output).toContain('ERROR: admin login failed at http://localhost:18080.');
    expect(r.output).toContain('curl -sf http://localhost:18080/health');
    expect(r.output).not.toContain('Traceback');
    expect(r.curl).toEqual(['curl http://localhost:18080/api/v1/public/login']);
  });

  it('name no fixed container or backend port', () => {
    for (const s of ['reset-demo.sh', 'prep-stage.sh']) {
      const text = readFileSync(join(DEMO_DIR, s), 'utf-8');
      expect(text, s).not.toContain('aim-postgres');
      expect(text, s).not.toContain('localhost:8080');
      const execs = text.match(/docker exec \S+/g) ?? [];
      expect(execs.length, `${s} runs docker exec`).toBeGreaterThan(0);
      for (const e of execs) expect(e, s).toMatch(/^docker exec "?\$PG_CONTAINER"?$/);
    }
  });

  it('the demo README names both variables with their defaults', () => {
    const text = readFileSync(DEMO_README, 'utf-8');
    expect(text).toContain('| `AIM_PROJECT` | `aim` |');
    expect(text).toContain('| `AIM_BACKEND_PORT` | `8080` |');
    expect(text).toContain('reset-demo.sh');
    expect(text).toContain('prep-stage.sh');
  });
});

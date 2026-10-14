import { describe, it, expect } from 'vitest';
import { readdirSync, readFileSync } from 'fs';
import { join } from 'path';
import { load } from 'js-yaml';

/**
 * The quick-start takes its host ports and its container-name prefix from the
 * environment, with the values it always had as defaults.
 *
 * docker-compose.quickstart.yml fixed the container names (aim-postgres,
 * aim-redis, aim-backend, aim-frontend) and the host ports (5432, 6379, 8080,
 * 3000), so a second stack on one host, or a stack beside anything else on
 * 8080, could not start from it; scripts/test-quickstart.sh worked in one fixed
 * directory under /tmp and ran `down -v` on whatever it found there. This cell
 * keeps the variables, their defaults and the places that document them in
 * step:
 *
 *   - rendered with no variable set, the compose file yields the same names
 *     and host ports as before; rendered with the variables set, it yields
 *     those values, and the URLs the backend and dashboard are told follow
 *     the ports;
 *   - scripts/quickstart.sh probes and prints the configured ports;
 *   - scripts/test-quickstart.sh works in a directory from mktemp and names
 *     no fixed path;
 *   - the Docker section of docs/DEPLOYMENT.md and the release smoke contract
 *     name every variable with its default.
 *
 * The dev stack, docker-compose.yml, fixed the same names and ports and seven
 * more services' (aim-elasticsearch on 9200, aim-grafana on 3003, ...). It
 * takes the quick-start's variables with the same defaults and one port
 * variable per other service, and the commands the docs give to start part of
 * it name its services, not its containers (`docker compose up -d
 * aim-postgres` fails with "no such service").
 *
 * Interpolation follows the Compose file format: `${VAR}`, `${VAR:-default}`
 * with nested defaults, and `${VAR:?message}`; the rendering is checked
 * against the real `docker compose config` output in the compose file's
 * header comment and in docs/testing/release-smoke.md.
 */

const REPO_ROOT = join(__dirname, '..', '..', '..');
const COMPOSE_FILE = join(REPO_ROOT, 'docker-compose.quickstart.yml');
const QUICKSTART = join(REPO_ROOT, 'scripts', 'quickstart.sh');
const TEST_QUICKSTART = join(REPO_ROOT, 'scripts', 'test-quickstart.sh');
const DEPLOYMENT_GUIDE = join(REPO_ROOT, 'docs', 'DEPLOYMENT.md');
const SMOKE_CONTRACT = join(REPO_ROOT, 'docs', 'testing', 'release-smoke.md');

/** The variables the quick-start reads, with the defaults it always had. */
const VARIABLES: Record<string, string> = {
  AIM_PROJECT: 'aim',
  AIM_POSTGRES_PORT: '5432',
  AIM_REDIS_PORT: '6379',
  AIM_BACKEND_PORT: '8080',
  AIM_FRONTEND_PORT: '3000',
};

/** Secrets the compose file requires; any value renders it. */
const SECRETS: Record<string, string> = {
  POSTGRES_PASSWORD: 'not-a-real-password',
  REDIS_PASSWORD: 'not-a-real-password',
  JWT_SECRET: 'not-a-real-secret',
  KEYVAULT_MASTER_KEY: 'not-a-real-key',
};

/**
 * Resolves Compose variable references in `text` against `env`. Nested
 * defaults resolve innermost first, as `docker compose config` does.
 */
function interpolate(text: string, env: Record<string, string>): string {
  const inner = /\$\{([A-Za-z_][A-Za-z0-9_]*)(?::?([-?])((?:[^{}]|\$[^{])*))?\}/;
  let out = text;
  for (let guard = 0; guard < 100; guard++) {
    const m = inner.exec(out);
    if (m === null) return out;
    const [whole, name, operator, argument] = m;
    const value = env[name];
    let replacement: string;
    if (operator === '-') replacement = value === undefined || value === '' ? argument : value;
    else if (operator === '?') {
      if (value === undefined || value === '') throw new Error(`${name} is required: ${argument}`);
      replacement = value;
    } else replacement = value ?? '';
    out = out.slice(0, m.index) + replacement + out.slice(m.index + whole.length);
  }
  throw new Error('interpolation did not converge');
}

interface Service {
  container_name?: string;
  ports?: string[];
  environment?: string[] | Record<string, string>;
}

interface Rendered {
  names: Record<string, string>;
  /** Host side of each service's single published port, as written. */
  hostPorts: Record<string, string>;
  /** Container side of each service's single published port. */
  containerPorts: Record<string, string>;
  backendEnv: Record<string, string>;
  frontendEnv: Record<string, string>;
}

function render(env: Record<string, string>): Rendered {
  const doc = load(interpolate(readFileSync(COMPOSE_FILE, 'utf-8'), env)) as { services: Record<string, Service> };
  const names: Record<string, string> = {};
  const hostPorts: Record<string, string> = {};
  const containerPorts: Record<string, string> = {};
  for (const [service, spec] of Object.entries(doc.services)) {
    names[service] = spec.container_name ?? '';
    const ports = spec.ports ?? [];
    expect(ports.length, `${service} publishes one port`).toBe(1);
    const parts = ports[0].split(':');
    containerPorts[service] = parts[parts.length - 1];
    hostPorts[service] = parts[parts.length - 2];
  }
  const listEnv = (service: string): Record<string, string> => {
    const spec = doc.services[service].environment ?? [];
    if (!Array.isArray(spec)) return spec;
    return Object.fromEntries(spec.map((entry) => [entry.slice(0, entry.indexOf('=')), entry.slice(entry.indexOf('=') + 1)]));
  };
  return { names, hostPorts, containerPorts, backendEnv: listEnv('backend'), frontendEnv: listEnv('frontend') };
}

/** Lines of `text` that mention `variable`. */
function linesNaming(text: string, variable: string): string[] {
  return text.split('\n').filter((line) => line.includes(variable));
}

// ---------------------------------------------------------------------------

describe('the quick-start takes its host ports and container-name prefix from the environment', () => {
  it('the interpolation helper resolves plain, defaulted, nested and required references', () => {
    expect(interpolate('${A}-${B:-b}-${C:-${D:-d}}-${E:?set E}', { A: 'a', E: 'e' })).toBe('a-b-d-e');
    expect(interpolate('${C:-${D:-d}}', { D: 'given' })).toBe('given');
    expect(interpolate('${C:-${D:-d}}', { C: 'outer', D: 'given' })).toBe('outer');
    expect(() => interpolate('${E:?set E}', {})).toThrow('E is required: set E');
  });

  it('renders the names and host ports it always had when no variable is set', () => {
    const rendered = render(SECRETS);
    expect(rendered.names).toEqual({
      postgres: 'aim-postgres',
      redis: 'aim-redis',
      backend: 'aim-backend',
      frontend: 'aim-frontend',
    });
    expect(rendered.hostPorts).toEqual({ postgres: '5432', redis: '6379', backend: '8080', frontend: '3000' });
    expect(rendered.backendEnv.FRONTEND_URL).toBe('http://localhost:3000');
    expect(rendered.backendEnv.ALLOWED_ORIGINS).toBe('http://localhost:3000');
    expect(rendered.frontendEnv.NEXT_PUBLIC_API_URL).toBe('http://localhost:8080');
  });

  it('renders the configured names and host ports, and the URLs follow the ports', () => {
    const rendered = render({
      ...SECRETS,
      AIM_PROJECT: 'lab',
      AIM_POSTGRES_PORT: '15432',
      AIM_REDIS_PORT: '16379',
      AIM_BACKEND_PORT: '18080',
      AIM_FRONTEND_PORT: '13000',
    });
    expect(rendered.names).toEqual({
      postgres: 'lab-postgres',
      redis: 'lab-redis',
      backend: 'lab-backend',
      frontend: 'lab-frontend',
    });
    expect(rendered.hostPorts).toEqual({ postgres: '15432', redis: '16379', backend: '18080', frontend: '13000' });
    expect(rendered.backendEnv.FRONTEND_URL).toBe('http://localhost:13000');
    expect(rendered.backendEnv.ALLOWED_ORIGINS).toBe('http://localhost:13000');
    expect(rendered.frontendEnv.NEXT_PUBLIC_API_URL).toBe('http://localhost:18080');
  });

  it('keeps the container-side ports and the in-container service addresses fixed', () => {
    const rendered = render({ ...SECRETS, AIM_POSTGRES_PORT: '1', AIM_REDIS_PORT: '2', AIM_BACKEND_PORT: '3', AIM_FRONTEND_PORT: '4' });
    expect(rendered.containerPorts).toEqual({ postgres: '5432', redis: '6379', backend: '8080', frontend: '3000' });
    expect(rendered.backendEnv.POSTGRES_PORT).toBe('5432');
    expect(rendered.backendEnv.REDIS_PORT).toBe('6379');
  });

  it('an explicit FRONTEND_URL, ALLOWED_ORIGINS or FRONTEND_API_URL still wins over the ports', () => {
    const rendered = render({
      ...SECRETS,
      AIM_BACKEND_PORT: '18080',
      AIM_FRONTEND_PORT: '13000',
      FRONTEND_URL: 'https://aim.example.com',
      ALLOWED_ORIGINS: 'https://aim.example.com',
      FRONTEND_API_URL: 'https://api.aim.example.com',
    });
    expect(rendered.backendEnv.FRONTEND_URL).toBe('https://aim.example.com');
    expect(rendered.backendEnv.ALLOWED_ORIGINS).toBe('https://aim.example.com');
    expect(rendered.frontendEnv.NEXT_PUBLIC_API_URL).toBe('https://api.aim.example.com');
  });

  it('the postgres and redis ports stay bound to 127.0.0.1', () => {
    const text = interpolate(readFileSync(COMPOSE_FILE, 'utf-8'), SECRETS);
    const doc = load(text) as { services: Record<string, Service> };
    expect(doc.services.postgres.ports?.[0]).toMatch(/^127\.0\.0\.1:/);
    expect(doc.services.redis.ports?.[0]).toMatch(/^127\.0\.0\.1:/);
  });

  it('scripts/quickstart.sh probes and prints the configured ports, never a fixed one', () => {
    const script = readFileSync(QUICKSTART, 'utf-8');
    for (const line of script.split('\n').filter((l) => l.includes('localhost:'))) {
      expect(line, 'every localhost URL in quickstart.sh takes its port from a variable').toMatch(/localhost:(\$[a-z_]+|%s)/);
    }
    expect(script).toContain('AIM_BACKEND_PORT');
    expect(script).toContain('AIM_FRONTEND_PORT');
    expect(script).toContain('AIM_PROJECT');
  });

  it('scripts/test-quickstart.sh works in a directory from mktemp and names no fixed path', () => {
    const script = readFileSync(TEST_QUICKSTART, 'utf-8');
    expect(script).not.toContain('aim-quickstart-test');
    expect(script).toMatch(/DIR="?\$\(mktemp -d/);
    for (const line of script.split('\n').filter((l) => /^\s*docker compose\b/.test(l) && /\bdown\b/.test(l))) {
      expect(line, 'every teardown addresses the directory this run created').toContain('$DIR/');
    }
    for (const line of script.split('\n').filter((l) => l.includes('localhost:'))) {
      expect(line, 'every localhost URL in test-quickstart.sh takes its port from a variable').toMatch(/localhost:(\$[A-Z_]+|%s)/);
    }
    for (const [variable, fallback] of Object.entries(VARIABLES)) {
      if (variable === 'AIM_PROJECT') continue;
      expect(script, `${variable} defaults to ${fallback}`).toContain(`\${${variable}:-${fallback}}`);
    }
  });

  it('the deployment guide Docker section and the smoke contract name every variable with its default', () => {
    const guide = readFileSync(DEPLOYMENT_GUIDE, 'utf-8');
    const dockerSection = guide.slice(guide.indexOf('### Docker\n'), guide.indexOf('### From source'));
    expect(dockerSection.length).toBeGreaterThan(0);
    const contract = readFileSync(SMOKE_CONTRACT, 'utf-8');
    for (const [variable, fallback] of Object.entries(VARIABLES)) {
      for (const [label, text] of [
        ['docs/DEPLOYMENT.md Docker section', dockerSection],
        ['docs/testing/release-smoke.md', contract],
      ] as Array<[string, string]>) {
        const lines = linesNaming(text, variable);
        expect(lines.length, `${label} names ${variable}`).toBeGreaterThan(0);
        expect(
          lines.some((line) => line.includes(fallback)),
          `${label} states the default ${fallback} on a line naming ${variable}: ${JSON.stringify(lines)}`,
        ).toBe(true);
      }
    }
    expect(dockerSection).toContain('aim-backend');
  });

  it('the deployment guide says a run over an existing install leaves its .env as it is', () => {
    // scripts/test-quickstart-orphan-volume.sh measures both halves against
    // docker and curl stubs: a first run writes the variables to .env, and a
    // run over an existing install starts Compose with them and writes nothing.
    const guide = readFileSync(DEPLOYMENT_GUIDE, 'utf-8');
    const dockerSection = guide.slice(guide.indexOf('### Docker\n'), guide.indexOf('### From source'));
    expect(dockerSection.length).toBeGreaterThan(0);
    expect(dockerSection, 'recording is not claimed for every run').not.toContain('The script records the values it ran with');
    expect(dockerSection).toContain("A first run records the values it ran with in the install's `.env`");
    expect(dockerSection).toMatch(/over an existing install[^.]*leaves that `\.env` as it is/);

    const stubTest = readFileSync(join(REPO_ROOT, 'scripts', 'test-quickstart-orphan-volume.sh'), 'utf-8');
    expect(stubTest, 'the stub test runs the case the deployment guide describes').toContain('run_case existing-env-not-recorded');
  });

  it('the smoke contract rows for the quick-start are rows of the component smoke matrix', () => {
    const lines = readFileSync(SMOKE_CONTRACT, 'utf-8').split('\n');
    const header = lines.findIndex((line) => line.startsWith('| Component | What ships |'));
    expect(header, 'the component smoke matrix has its header row').toBeGreaterThan(-1);
    let end = header;
    while (end + 1 < lines.length && lines[end + 1].startsWith('|')) end++;
    const matrix = lines.slice(header, end + 1);
    for (const component of ['**Docker quick-start**', '**quickstart.sh without Docker**']) {
      expect(
        matrix.some((row) => row.startsWith(`| ${component} |`)),
        `${component} is a row of the table that starts at docs/testing/release-smoke.md:${header + 1}`,
      ).toBe(true);
    }
  });

  it('no pipe row of the smoke contract follows a blank line unless it opens a table', () => {
    // GitHub renders a pipe row as a table row only while the table runs on:
    // after a blank line it is a paragraph of pipes, unless a delimiter row
    // follows it and it opens a new table.
    const lines = readFileSync(SMOKE_CONTRACT, 'utf-8').split('\n');
    const strays: string[] = [];
    let fenced = false;
    lines.forEach((line, i) => {
      if (line.startsWith('```')) fenced = !fenced;
      if (fenced || !line.startsWith('|')) return;
      const continuesTable = i > 0 && lines[i - 1].startsWith('|');
      const opensTable = /^\|(\s*:?-{3,}:?\s*\|)+\s*$/.test(lines[i + 1] ?? '');
      if (!continuesTable && !opensTable) strays.push(`docs/testing/release-smoke.md:${i + 1}: ${line.slice(0, 48)}`);
    });
    expect(strays).toEqual([]);
  });
});

// ---------------------------------------------------------------------------

const DEV_COMPOSE_FILE = join(REPO_ROOT, 'docker-compose.yml');
const DEV_ENV_TEMPLATE = join(REPO_ROOT, '.env.example');

/** Host port variables of the dev stack: [service, container port, variable, default]. */
const DEV_PORTS: Array<[string, string, string, string]> = [
  ['postgres', '5432', 'AIM_POSTGRES_PORT', '5432'],
  ['redis', '6379', 'AIM_REDIS_PORT', '6379'],
  ['elasticsearch', '9200', 'AIM_ELASTICSEARCH_PORT', '9200'],
  ['elasticsearch', '9300', 'AIM_ELASTICSEARCH_TRANSPORT_PORT', '9300'],
  ['minio', '9000', 'AIM_MINIO_PORT', '9000'],
  ['minio', '9001', 'AIM_MINIO_CONSOLE_PORT', '9001'],
  ['nats', '4222', 'AIM_NATS_PORT', '4222'],
  ['nats', '8222', 'AIM_NATS_MONITOR_PORT', '8222'],
  ['prometheus', '9090', 'AIM_PROMETHEUS_PORT', '9090'],
  ['grafana', '3000', 'AIM_GRAFANA_PORT', '3003'],
  ['loki', '3100', 'AIM_LOKI_PORT', '3100'],
  ['backend', '8080', 'AIM_BACKEND_PORT', '8080'],
  ['frontend', '3000', 'AIM_FRONTEND_PORT', '3000'],
];

/** Services that run as a container of the stack (bootstrap is a one-shot profile service). */
const DEV_SERVICES = [
  'postgres',
  'redis',
  'elasticsearch',
  'minio',
  'nats',
  'prometheus',
  'grafana',
  'loki',
  'promtail',
  'backend',
  'frontend',
];

/** The two services the dev stack publishes on every interface, as before. */
const DEV_PUBLIC = new Set(['backend', 'frontend']);

const DEV_SECRETS: Record<string, string> = {
  ...SECRETS,
  MINIO_ROOT_PASSWORD: 'not-a-real-password',
  GRAFANA_ADMIN_PASSWORD: 'not-a-real-password',
  METRICS_AUTH_TOKEN: 'not-a-real-token',
};

interface DevService extends Service {
  profiles?: string[];
}

interface DevRendered {
  services: Record<string, DevService>;
  names: Record<string, string>;
  /** `<service>:<container port>` -> `<host address>:<host port>` ('' address when unbound). */
  published: Record<string, string>;
  env: (service: string) => Record<string, string>;
}

function renderDev(env: Record<string, string>): DevRendered {
  const doc = load(interpolate(readFileSync(DEV_COMPOSE_FILE, 'utf-8'), env)) as { services: Record<string, DevService> };
  const names: Record<string, string> = {};
  const published: Record<string, string> = {};
  for (const [service, spec] of Object.entries(doc.services)) {
    if (spec.container_name !== undefined) names[service] = spec.container_name;
    for (const port of spec.ports ?? []) {
      const parts = port.split(':');
      const address = parts.length === 3 ? parts[0] : '';
      published[`${service}:${parts[parts.length - 1]}`] = `${address}:${parts[parts.length - 2]}`;
    }
  }
  const envOf = (service: string): Record<string, string> => {
    const spec = doc.services[service].environment ?? [];
    if (!Array.isArray(spec)) return spec;
    return Object.fromEntries(spec.map((entry) => [entry.slice(0, entry.indexOf('=')), entry.slice(entry.indexOf('=') + 1)]));
  };
  return { services: doc.services, names, published, env: envOf };
}

function expectedPublished(hostPort: (variable: string, fallback: string) => string): Record<string, string> {
  return Object.fromEntries(
    DEV_PORTS.map(([service, containerPort, variable, fallback]) => [
      `${service}:${containerPort}`,
      `${DEV_PUBLIC.has(service) ? '' : '127.0.0.1'}:${hostPort(variable, fallback)}`,
    ]),
  );
}

/** Markdown files that start part of the dev stack by service name. */
function devStackDocs(): string[] {
  const files = [join(REPO_ROOT, 'README.md'), join(REPO_ROOT, 'CONTRIBUTING.md'), DEPLOYMENT_GUIDE];
  const guides = join(REPO_ROOT, 'docs', 'guides');
  for (const name of readdirSync(guides)) if (name.endsWith('.md')) files.push(join(guides, name));
  const examples = join(REPO_ROOT, 'examples');
  for (const entry of readdirSync(examples, { withFileTypes: true })) {
    if (!entry.isDirectory()) continue;
    if (readdirSync(join(examples, entry.name)).includes('README.md')) files.push(join(examples, entry.name, 'README.md'));
  }
  return files;
}

describe('the dev stack takes its host ports and container-name prefix from the environment', () => {
  it('renders the names and host ports it always had when no variable is set', () => {
    const rendered = renderDev(DEV_SECRETS);
    expect(rendered.names).toEqual(Object.fromEntries(DEV_SERVICES.map((service) => [service, `aim-${service}`])));
    expect(rendered.published).toEqual(expectedPublished((_, fallback) => fallback));
    expect(rendered.env('backend').FRONTEND_URL).toBe('http://localhost:3000');
    expect(rendered.env('backend').ALLOWED_ORIGINS).toBe('http://localhost:3000');
    expect(rendered.env('frontend').NEXT_PUBLIC_API_URL).toBe('http://localhost:8080');
    expect(rendered.env('grafana').GF_SERVER_ROOT_URL).toBe('http://localhost:3003');
  });

  it('renders the configured names and host ports, and the URLs follow the ports', () => {
    const configured: Record<string, string> = Object.fromEntries(
      DEV_PORTS.map(([, , variable], index) => [variable, String(20000 + index)]),
    );
    const rendered = renderDev({ ...DEV_SECRETS, ...configured, AIM_PROJECT: 'lab' });
    expect(rendered.names).toEqual(Object.fromEntries(DEV_SERVICES.map((service) => [service, `lab-${service}`])));
    expect(rendered.published).toEqual(expectedPublished((variable) => configured[variable]));
    expect(rendered.env('backend').FRONTEND_URL).toBe(`http://localhost:${configured.AIM_FRONTEND_PORT}`);
    expect(rendered.env('backend').ALLOWED_ORIGINS).toBe(`http://localhost:${configured.AIM_FRONTEND_PORT}`);
    expect(rendered.env('frontend').NEXT_PUBLIC_API_URL).toBe(`http://localhost:${configured.AIM_BACKEND_PORT}`);
    expect(rendered.env('grafana').GF_SERVER_ROOT_URL).toBe(`http://localhost:${configured.AIM_GRAFANA_PORT}`);
    expect(rendered.env('backend').POSTGRES_PORT).toBe('5432');
    expect(rendered.env('backend').REDIS_PORT).toBe('6379');
  });

  it('shares the quick-start variables with the same defaults', () => {
    const dev = readFileSync(DEV_COMPOSE_FILE, 'utf-8');
    for (const [variable, fallback] of Object.entries(VARIABLES)) {
      expect(dev, `docker-compose.yml reads ${variable} with the default ${fallback}`).toContain(`\${${variable}:-${fallback}}`);
    }
  });

  it('names a container and publishes a port only through a variable', () => {
    const dev = load(readFileSync(DEV_COMPOSE_FILE, 'utf-8')) as { services: Record<string, DevService> };
    for (const [service, spec] of Object.entries(dev.services)) {
      if (spec.container_name !== undefined) expect(spec.container_name).toBe(`\${AIM_PROJECT:-aim}-${service}`);
      for (const port of spec.ports ?? []) {
        expect(port, `${service} takes its host port from a variable`).toMatch(/^(127\.0\.0\.1:)?\$\{AIM_[A-Z_]+_PORT:-\d+\}:\d+$/);
      }
    }
    expect(dev.services.bootstrap.profiles).toEqual(['bootstrap']);
  });

  it('the From source section of docs/DEPLOYMENT.md and .env.example name every variable with its default', () => {
    const guide = readFileSync(DEPLOYMENT_GUIDE, 'utf-8');
    const start = guide.indexOf('### From source\n');
    expect(start).toBeGreaterThanOrEqual(0);
    const end = guide.indexOf('\n## ', start);
    const fromSource = guide.slice(start, end === -1 ? undefined : end);
    const template = readFileSync(DEV_ENV_TEMPLATE, 'utf-8');
    const all: Array<[string, string]> = [['AIM_PROJECT', 'aim'], ...DEV_PORTS.map(([, , variable, fallback]): [string, string] => [variable, fallback])];
    for (const [variable, fallback] of all) {
      expect(fromSource, `docs/DEPLOYMENT.md From source names ${variable} with its default`).toContain(`\`${variable}\` (\`${fallback}\``);
      expect(template, `.env.example names ${variable} with its default`).toContain(`# ${variable}=${fallback}\n`);
    }
  });

  it('every documented command that starts part of the dev stack names its services', () => {
    const services = Object.keys((load(readFileSync(DEV_COMPOSE_FILE, 'utf-8')) as { services: object }).services);
    const command = /docker compose(?: -f (\S+))? up(?: -d)?((?: [a-z][a-z0-9_-]*)+)/g;
    let checked = 0;
    for (const file of devStackDocs()) {
      for (const match of readFileSync(file, 'utf-8').matchAll(command)) {
        const [, composeFile, names] = match;
        if (composeFile !== undefined && !composeFile.endsWith('/docker-compose.yml') && composeFile !== 'docker-compose.yml') continue;
        for (const name of names.trim().split(/\s+/)) {
          expect(services, `${file.slice(REPO_ROOT.length + 1)}: "${match[0]}" starts ${name}`).toContain(name);
          checked++;
        }
      }
    }
    expect(checked).toBeGreaterThan(0);
  });
});

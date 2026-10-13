import { describe, it, expect } from 'vitest';
import { readFileSync } from 'fs';
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
 *   - the README quick-start section and the release smoke contract name
 *     every variable with its default.
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
const README = join(REPO_ROOT, 'README.md');
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

  it('the README quick-start section and the smoke contract name every variable with its default', () => {
    const readme = readFileSync(README, 'utf-8');
    const dockerSection = readme.slice(readme.indexOf('### Docker'), readme.indexOf('### From source'));
    expect(dockerSection.length).toBeGreaterThan(0);
    const contract = readFileSync(SMOKE_CONTRACT, 'utf-8');
    for (const [variable, fallback] of Object.entries(VARIABLES)) {
      for (const [label, text] of [
        ['README.md Docker section', dockerSection],
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
});

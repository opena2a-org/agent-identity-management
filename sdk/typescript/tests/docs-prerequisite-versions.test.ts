import { describe, it, expect } from 'vitest';
import { existsSync, readFileSync } from 'fs';
import { join } from 'path';

/**
 * The guides that set up, build or contribute to the platform state the Go
 * and Node.js versions that build and run it.
 *
 * They named Go 1.21+, 1.22+, 1.23+ and 1.23.1+ while apps/backend/go.mod
 * declares `go 1.25.0`, and Node.js 18+, 20+ and 22+ while the dashboard's
 * locked dependencies declare ranges that 18.x, 20.0 to 20.18 and 22.0 to
 * 22.12 fall outside of. Two guides listed pnpm 8+ and gave pnpm commands;
 * the repository installs with npm from package-lock.json.
 *
 * These cells run in the SDK suite because that job runs on every pull
 * request, so a dependency bump that raises the floor fails until the guides
 * say so.
 *
 *   C1  every Go version a guide states is at least the `go` directive of
 *       apps/backend/go.mod;
 *   C2  every Node.js version a guide states satisfies the `engines.node`
 *       range of the root package.json and of every package in the root and
 *       apps/web lockfiles that is not limited to one platform;
 *   C3  no guide names pnpm, every npm version a guide states satisfies the
 *       root package.json `engines.npm`, and the repository carries
 *       package-lock.json and no pnpm-lock.yaml;
 *   C4  the pre-fix statements, kept as literals, are refused and the fixed
 *       forms pass, through the same predicates as C1 and C2, against ranges
 *       kept as literals;
 *   C5  the range check itself, over literal versions and ranges;
 *   C6  a line stating Go 1.22+ and Node.js 18+, a pnpm command and a line
 *       stating npm 8+, added to the text of README.md or of a guide, one
 *       file at a time, are refused through the function C1 to C3 call.
 *
 * C1 to C3 read README.md as well as the guides. The first cell does not
 * require README.md to state a version: the prerequisites line is in
 * docs/DEPLOYMENT.md.
 */

const REPO_ROOT = join(__dirname, '..', '..', '..');

const GUIDES = [
  'CONTRIBUTING.md',
  'apps/backend/README.md',
  'docs/DEPLOYMENT.md',
  'docs/guides/CONTRIBUTING.md',
  'docs/guides/INSTALLATION.md',
  'docs/guides/QUICK_START.md',
  'docs/guides/SETUP_GUIDE.md',
];

// The files C1 to C3 read.
const READ = ['README.md', ...GUIDES];

const VERSION = String.raw`\d+(?:\.\d+){0,2}`;
const LABEL_END = String.raw`(?:\*\*)?:?(?:\*\*)?\s+`;
// "Go 1.25+", "**Go 1.25+**", "**Go** 1.25+", "**Go**: 1.25 or higher"
const GO_STATEMENT = new RegExp(String.raw`\bGo\b${LABEL_END}(${VERSION})`, 'g');
// "Node.js 20.19+ or 22.13+", "**Node.js**: 20.19+ or 22.13+", "Node 20+"
const NODE_STATEMENT = new RegExp(
  String.raw`\bNode(?:\.js)?\b${LABEL_END}(${VERSION}\+?(?:(?:,\s*|\s+or\s+)${VERSION}\+?)*)`,
  'g',
);
const NPM_STATEMENT = new RegExp(String.raw`\bnpm\b${LABEL_END}(${VERSION})\+`, 'g');

interface Statement {
  line: number;
  versions: string[];
}

function statements(text: string, pattern: RegExp): Statement[] {
  const out: Statement[] = [];
  text.split(/\r?\n/).forEach((line, i) => {
    for (const match of line.matchAll(pattern)) {
      out.push({ line: i + 1, versions: (match[1] ?? '').match(/\d+(?:\.\d+){0,2}/g) ?? [] });
    }
  });
  return out;
}

type Version = [number, number, number];

function parseVersion(text: string): Version {
  const parts = text.replace(/^v/, '').split('.').map(Number);
  return [parts[0] ?? 0, parts[1] ?? 0, parts[2] ?? 0];
}

function compare(a: Version, b: Version): number {
  return a[0] - b[0] || a[1] - b[1] || a[2] - b[2];
}

const COMPARATOR = /(>=|<=|>|<|\^|~|=)?\s*v?(\*|\d+(?:\.\d+){0,2})/g;

function satisfiesComparator(version: Version, op: string, text: string): boolean {
  if (text === '*') return true;
  const base = parseVersion(text);
  const given = text.split('.').length;
  const nextMinor: Version = given === 1 ? [base[0] + 1, 0, 0] : [base[0], base[1] + 1, 0];
  switch (op) {
    case '>=':
      return compare(version, base) >= 0;
    case '>':
      return compare(version, base) > 0;
    case '<=':
      return compare(version, base) <= 0;
    case '<':
      return compare(version, base) < 0;
    case '^': {
      const upper: Version =
        base[0] > 0 ? [base[0] + 1, 0, 0] : base[1] > 0 ? [0, base[1] + 1, 0] : [0, 0, base[2] + 1];
      return compare(version, base) >= 0 && compare(version, upper) < 0;
    }
    case '~':
      return compare(version, base) >= 0 && compare(version, nextMinor) < 0;
    default:
      // A bare or `=` version: X and X.Y cover their whole line, X.Y.Z is exact.
      if (given === 3) return compare(version, base) === 0;
      return compare(version, base) >= 0 && compare(version, nextMinor) < 0;
  }
}

/** Whether `version` satisfies the npm `engines` range; throws on a form it does not read. */
function satisfies(version: string, range: string): boolean {
  const v = parseVersion(version);
  return range.split('||').some((alternative) => {
    const comparators = [...alternative.matchAll(COMPARATOR)];
    if (comparators.length === 0 || alternative.replace(COMPARATOR, '').trim() !== '') {
      throw new Error(`engines range not read by this test: "${range}"`);
    }
    return comparators.every((m) => satisfiesComparator(v, m[1] ?? '', m[2] ?? ''));
  });
}

interface NamedRange {
  name: string;
  range: string;
}

interface LockEntry {
  engines?: unknown;
  os?: unknown;
  cpu?: unknown;
}

function lockfileNodeRanges(relPath: string): NamedRange[] {
  const lock = JSON.parse(readFileSync(join(REPO_ROOT, relPath), 'utf-8')) as {
    packages: Record<string, LockEntry>;
  };
  const out: NamedRange[] = [];
  for (const [key, entry] of Object.entries(lock.packages)) {
    // A package that names an os or cpu is installed only on that platform.
    if (entry.os !== undefined || entry.cpu !== undefined) continue;
    const engines = entry.engines as { node?: unknown } | undefined;
    if (engines && !Array.isArray(engines) && typeof engines.node === 'string') {
      out.push({ name: `${relPath} ${key || '(root)'}`, range: engines.node });
    }
  }
  return out;
}

function goProblems(text: string, required: string): string[] {
  return statements(text, GO_STATEMENT).flatMap(({ line, versions }) =>
    versions
      .filter((v) => compare(parseVersion(v), parseVersion(required)) < 0)
      .map((v) => `line ${line}: Go ${v} is below go ${required}`),
  );
}

function nodeProblems(text: string, ranges: NamedRange[]): string[] {
  return statements(text, NODE_STATEMENT).flatMap(({ line, versions }) =>
    versions.flatMap((v) => {
      const outside = ranges.find((r) => !satisfies(v, r.range));
      return outside ? [`line ${line}: Node.js ${v} is outside ${outside.name} "${outside.range}"`] : [];
    }),
  );
}

const rootPackage = JSON.parse(readFileSync(join(REPO_ROOT, 'package.json'), 'utf-8')) as {
  engines: { node: string; npm: string };
};
const goMod = readFileSync(join(REPO_ROOT, 'apps', 'backend', 'go.mod'), 'utf-8');
const goDirective = goMod.match(/^go\s+(\d+\.\d+(?:\.\d+)?)\s*$/m)?.[1];
const nodeRanges: NamedRange[] = [
  { name: 'package.json', range: rootPackage.engines.node },
  ...lockfileNodeRanges('package-lock.json'),
  ...lockfileNodeRanges('apps/web/package-lock.json'),
];
// Listed apart from READ, so C6 holds README.md's text when READ does not list it.
const onDisk = new Map(['README.md', ...GUIDES].map((f) => [f, readFileSync(join(REPO_ROOT, f), 'utf-8')]));

interface Problems {
  go: string[];
  node: string[];
  pnpm: string[];
  npm: string[];
}

/** What C1 to C3 refuse in the files of READ, given each file's text. */
function problemsIn(textOf: (file: string) => string): Problems {
  const out: Problems = { go: [], node: [], pnpm: [], npm: [] };
  for (const file of READ) {
    const text = textOf(file);
    out.go.push(...goProblems(text, goDirective!).map((p) => `${file} ${p}`));
    out.node.push(...nodeProblems(text, nodeRanges).map((p) => `${file} ${p}`));
    text.split(/\r?\n/).forEach((line, i) => {
      if (/\bpnpm\b/.test(line)) out.pnpm.push(`${file}:${i + 1}: ${line.trim()}`);
    });
    for (const { line, versions } of statements(text, NPM_STATEMENT)) {
      for (const v of versions) {
        if (!satisfies(v, rootPackage.engines.npm)) out.npm.push(`${file}:${line}: npm ${v}`);
      }
    }
  }
  return out;
}

describe('prerequisite versions in the setup and contributing guides', () => {
  it('every guide states a Go or Node.js version the checks read', () => {
    const silent = GUIDES.filter((g) => {
      const text = onDisk.get(g)!;
      return statements(text, GO_STATEMENT).length + statements(text, NODE_STATEMENT).length === 0;
    });
    expect(silent).toEqual([]);
  });

  it('C1: every Go version stated is at least the go directive of apps/backend/go.mod', () => {
    expect(goDirective, 'apps/backend/go.mod has no go directive').toBeDefined();
    expect(problemsIn((f) => onDisk.get(f)!).go).toEqual([]);
  });

  it('C2: every Node.js version stated satisfies every engines.node range the dashboard installs', () => {
    expect(nodeRanges.length).toBeGreaterThan(1);
    expect(problemsIn((f) => onDisk.get(f)!).node).toEqual([]);
  });

  it('C3: neither README.md nor a guide names pnpm, and stated npm versions satisfy engines.npm', () => {
    const found = problemsIn((f) => onDisk.get(f)!);
    expect(found.pnpm).toEqual([]);
    expect(found.npm).toEqual([]);
    expect(existsSync(join(REPO_ROOT, 'package-lock.json'))).toBe(true);
    expect(existsSync(join(REPO_ROOT, 'pnpm-lock.yaml'))).toBe(false);
  });

  it('C4: the pre-fix statements are refused and the fixed forms pass', () => {
    const go = '1.25.0';
    const ranges: NamedRange[] = [
      { name: 'package.json', range: '>=20.0.0' },
      { name: 'next', range: '>=20.9.0' },
      { name: 'vite', range: '^20.19.0 || >=22.12.0' },
      { name: 'eslint-visitor-keys', range: '^20.19.0 || ^22.13.0 || >=24' },
      { name: 'vitest', range: '^20.0.0 || ^22.0.0 || >=24.0.0' },
    ];
    const refusedGo = [
      'Prerequisites: Docker, Go 1.22+, Node 20+, Python 3.11+.',
      '- **Go 1.23.1+** - [Install Go](https://go.dev/doc/install)',
      '- **Go**: 1.23 or higher',
      '- Go 1.21+',
      '- **Go** 1.23+ ([Install](https://golang.org/dl/))',
      '- Go 1.24.9 installed',
    ];
    for (const text of refusedGo) expect(goProblems(text, go), text).not.toEqual([]);
    const refusedNode = [
      'Prerequisites: Docker, Go 1.22+, Node 20+, Python 3.11+.',
      '- **Node.js 20+** - [Install Node.js](https://nodejs.org/)',
      '- **Node.js**: 18 or higher',
      '- Node.js 18+',
      '- **Node.js** 22+ ([Install](https://nodejs.org/))',
      '- Node.js 20.19+ or 22.12+',
      '- Node.js 20.18+',
    ];
    for (const text of refusedNode) expect(nodeProblems(text, ranges), text).not.toEqual([]);
    const accepted = [
      'Prerequisites: Docker, Go 1.25+, Node.js 20.19+ or 22.13+, Python 3.11+.',
      '- **Go 1.25+** - [Install Go](https://go.dev/doc/install)',
      '- **Node.js 20.19+ or 22.13+** - [Install Node.js](https://nodejs.org/)',
      '- **Go**: 1.25 or higher',
      '- **Node.js**: 20.19+ or 22.13+',
      '- **Node.js** 20.19+ or 22.13+ and **npm** 10+ (for frontend development)',
      '- Go 1.26+',
    ];
    for (const text of accepted) {
      expect(goProblems(text, go), text).toEqual([]);
      expect(nodeProblems(text, ranges), text).toEqual([]);
      expect(
        statements(text, GO_STATEMENT).length + statements(text, NODE_STATEMENT).length,
        text,
      ).toBeGreaterThan(0);
    }
  });

  it('C5: the engines range check reads the forms the lockfiles use', () => {
    const cases: Array<[string, string, boolean]> = [
      ['20.19', '^20.19.0 || >=22.12.0', true],
      ['20.18', '^20.19.0 || >=22.12.0', false],
      ['22.12', '^20.19.0 || ^22.13.0 || >=24', false],
      ['22.13', '^20.19.0 || ^22.13.0 || >=24', true],
      ['23.0', '^20.0.0 || ^22.0.0 || >=24.0.0', false],
      ['20.9', '>=20.9.0', true],
      ['18', '>=20.9.0', false],
      ['20.19', '18 || 20 || >=22', true],
      ['21.0', '18 || 20 || >=22', false],
      ['14.16', '>=16 || 14 >=14.17', false],
      ['14.17', '>=16 || 14 >=14.17', true],
      ['20.19', '>= 0.8.0', true],
      ['20.19', '>=v12.22.7', true],
      ['20.19', '*', true],
      ['20.19', '~20.19.0', true],
      ['20.20', '~20.19.0', false],
    ];
    for (const [version, range, expected] of cases) {
      expect(satisfies(version, range), `${version} in "${range}"`).toBe(expected);
    }
    expect(() => satisfies('20.19.0', '1.2.3 - 2.3.4')).toThrow(/not read/);
  });

  it('C6: a stale Go, Node.js, pnpm or npm line added to README.md or to a guide is refused', () => {
    expect(goDirective, 'apps/backend/go.mod has no go directive').toBeDefined();
    const stale = ['Prerequisites: Docker, Go 1.22+, Node.js 18+, Python 3.11+.', 'pnpm install', '- npm 8+'];
    for (const file of ['README.md', ...GUIDES]) {
      const text = onDisk.get(file)!;
      const first = text.split(/\r?\n/).length + 1;
      const found = problemsIn((f) => (f === file ? `${text}\n${stale.join('\n')}\n` : onDisk.get(f)!));
      expect(found.go, file).toContain(`${file} line ${first}: Go 1.22 is below go ${goDirective}`);
      expect(
        found.node.filter((p) => p.startsWith(`${file} line ${first}: Node.js 18 is outside `)),
        file,
      ).toHaveLength(1);
      expect(found.pnpm, file).toContain(`${file}:${first + 1}: pnpm install`);
      expect(found.npm, file).toContain(`${file}:${first + 2}: npm 8`);
    }
  });
});

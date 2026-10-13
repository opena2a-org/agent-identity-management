/**
 * AIM-12 item 6: README.md claims "source is not shipped in the npm package",
 * but `tsup.config.ts` shipped `sourcemap: true` with esbuild's default of
 * embedding every input file into the maps' `sourcesContent` — ~2.6 MB of the
 * full TypeScript source, including doc comments (among them the deliberately
 * quoted `OpenA2A-ARP/0.2.0` docstring in src/arp/index.ts), in every tarball.
 *
 * The delivered invariant: source maps still ship (stack traces stay mapped),
 * but no `.map` in the packed tarball carries a `sourcesContent` array, so the
 * README sentence is true again and the docstring literal dies with the
 * embedded sources.
 *
 * This packs the real tarball: `npm run build` + `npm pack` (both offline),
 * extract, and inspect every `.map`.
 */
import { describe, it, expect, beforeAll, afterAll } from 'vitest';
import { execFileSync } from 'child_process';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';

const PKG_ROOT = path.join(__dirname, '..');

let workDir: string;
let packageDir: string;

function walk(dir: string, out: string[] = []): string[] {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, entry.name);
    if (entry.isDirectory()) walk(p, out);
    else out.push(p);
  }
  return out;
}

beforeAll(() => {
  workDir = fs.mkdtempSync(path.join(os.tmpdir(), 'aim-sdk-pack-'));
  execFileSync('npm', ['run', 'build'], { cwd: PKG_ROOT, stdio: 'pipe' });
  // npm pack writes staging files into the configured npm cache; point it at a
  // writable temp dir so the test also runs where the shared cache is read-only.
  const packOut = execFileSync(
    'npm',
    ['pack', '--pack-destination', workDir],
    {
      cwd: PKG_ROOT,
      stdio: 'pipe',
      env: { ...process.env, npm_config_cache: path.join(workDir, 'npm-cache') },
    },
  )
    .toString()
    .trim();
  const tarball = path.join(workDir, packOut.split('\n').pop()!.trim());
  execFileSync('tar', ['-xzf', tarball, '-C', workDir], { stdio: 'pipe' });
  packageDir = path.join(workDir, 'package');
}, 300_000);

afterAll(() => {
  fs.rmSync(workDir, { recursive: true, force: true });
});

describe('the packed tarball does not embed the TypeScript source', () => {
  it('AIM-12.AC2 no .map in the npm tarball carries a sourcesContent array', () => {
    const maps = walk(packageDir).filter((f) => f.endsWith('.map'));
    // Non-vacuity: maps must still ship at all — the fix strips the embedded
    // sources, it does not turn source maps off.
    expect(maps.length).toBeGreaterThan(0);

    const offenders: string[] = [];
    for (const map of maps) {
      const parsed = JSON.parse(fs.readFileSync(map, 'utf-8')) as {
        sourcesContent?: unknown[];
      };
      if (Array.isArray(parsed.sourcesContent) && parsed.sourcesContent.length > 0) {
        offenders.push(path.relative(packageDir, map));
      }
    }
    expect(
      offenders,
      'These shipped maps embed the full source via sourcesContent, ' +
        'contradicting README.md ("source is not shipped in the npm package"):\n' +
        offenders.map((o) => `  - ${o}`).join('\n'),
    ).toEqual([]);
  });

  it('AIM-12.AC2 the frozen OpenA2A-ARP/0.2.0 docstring literal does not ride into any shipped map', () => {
    // The literal is deliberately quoted in source docstrings
    // (src/arp/index.ts, version.test.ts documents the rule). It reached the
    // tarball only inside sourcesContent; with embedded sources gone it must
    // not appear in any map.
    const maps = walk(packageDir).filter((f) => f.endsWith('.map'));
    const carriers = maps.filter((m) =>
      fs.readFileSync(m, 'utf-8').includes('OpenA2A-ARP/0.2.0'),
    );
    expect(carriers.map((c) => path.relative(packageDir, c))).toEqual([]);
  });
});

// The documentation of createHybridRequestHeaders said its headers were
// compatible with the backend. AIM's request verification does not accept
// them, and the sentence reached every install through the type declarations.
// The same tarball is read here, so the package is built and packed once.
describe('the packed tarball does not say createHybridRequestHeaders is accepted by AIM', () => {
  const declarationsOfTheHelper = () =>
    walk(packageDir)
      .filter((f) => /\.d\.m?ts$/.test(f))
      .filter((f) => fs.readFileSync(f, 'utf-8').includes('declare function createHybridRequestHeaders'));

  it('no shipped file carries the compatibility sentence, the declarations included', () => {
    // Non-vacuity: the declarations that carried the sentence still ship.
    expect(declarationsOfTheHelper().length).toBeGreaterThan(0);

    const carriers = walk(packageDir).filter((f) =>
      fs.readFileSync(f, 'utf-8').includes('compatible with the Go backend'),
    );
    expect(carriers.map((c) => path.relative(packageDir, c))).toEqual([]);
  });

  it('every shipped declaration of the helper is deprecated and says AIM does not accept its headers', () => {
    const declarations = declarationsOfTheHelper();
    expect(declarations.length).toBeGreaterThan(0);

    for (const file of declarations) {
      const text = fs.readFileSync(file, 'utf-8');
      const declaredAt = text.indexOf('declare function createHybridRequestHeaders');
      const comment = text.slice(text.lastIndexOf('/**', declaredAt), declaredAt);
      const where = path.relative(packageDir, file);
      // The comment found is the helper's own: nothing but white space separates them.
      expect(comment.slice(comment.indexOf('*/') + 2).trim(), where).toBe('');
      expect(comment, where).toContain('@deprecated');
      expect(comment.replace(/\s*\n\s*\*\s*/g, ' '), where).toContain(
        "AIM's request verification does not accept these headers",
      );
    }
  });
});

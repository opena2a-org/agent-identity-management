import { describe, it, expect } from 'vitest';
import { readFileSync } from 'fs';
import { join } from 'path';

/**
 * The root README's TypeScript example is held to the package it names.
 *
 * The example imported `AIMClient` from `@opena2a/aim-core`, built it with
 * `{ agentId }` and called `verify({ capability, resource })`. That package
 * exports no `AIMClient`: its class is `AIMCore`, which reads `agentName`, and
 * its `verify` checks an Ed25519 signature. Copied as written, the example
 * stopped at the import.
 *
 * `@opena2a/aim-core` is published from another repository and is not a
 * dependency here, so these cells compare the README with the surface of the
 * version the README says the example ran on, recorded below from the
 * published tarball. A README that names a version with no record fails, which
 * is the prompt to run the example on that version and record its surface.
 *
 *   C1  the SDK table row and the example name the same package and class, and
 *       the README states the version the example ran on;
 *   C2  every name the example imports is an export of that version;
 *   C3  the class is built with an option it reads, in the example and in the
 *       table row, and every method called on the instance is one it has;
 *   C4  the pre-fix example and two near misses, kept as literals, are refused.
 */

const REPO_ROOT = join(__dirname, '..', '..', '..');
const readme = readFileSync(join(REPO_ROOT, 'README.md'), 'utf-8');

const PACKAGE = '@opena2a/aim-core';

interface ClassSurface {
  options: string[];
  methods: string[];
}

interface PackageSurface {
  exports: string[];
  classes: Record<string, ClassSurface>;
}

// Recorded from the published tarball, in an empty directory:
//   npm install @opena2a/aim-core@0.2.0
//   node -e 'const m = require("@opena2a/aim-core");
//            console.log(Object.keys(m).sort(), Object.getOwnPropertyNames(m.AIMCore.prototype))'
// Constructor options are the members of `AIMCoreOptions` in dist/types.d.ts;
// `defaultDataDir` is declared private there and is left out of the methods.
const NEWEST_RECORDED = '0.2.0';
const RECORDED_SURFACE: Record<string, PackageSurface> = {
  [NEWEST_RECORDED]: {
    exports: [
      'AIMCore',
      'AIMServerReporter',
      'ALL_PATTERNS',
      'EventAggregator',
      'PII_PATTERNS',
      'SECRET_PATTERNS',
      'VERSION',
      'calculateTrust',
      'checkCapability',
      'createIdentity',
      'defaultDLPPolicy',
      'getDLPAction',
      'getOrCreateIdentity',
      'hasAuditLog',
      'hasPolicy',
      'loadIdentity',
      'loadPolicy',
      'logEvent',
      'mask',
      'maskAll',
      'maskMetadata',
      'readAuditLog',
      'savePolicy',
      'scanText',
      'sign',
      'vault',
      'verify',
    ],
    classes: {
      AIMCore: {
        options: ['agentName', 'dataDir', 'serverUrl'],
        methods: [
          'getIdentity',
          'getOrCreateIdentity',
          'checkCapability',
          'loadPolicy',
          'savePolicy',
          'logEvent',
          'readAuditLog',
          'calculateTrust',
          'setTrustHints',
          'sign',
          'verify',
          'enableReporting',
          'enableAggregation',
          'shutdown',
          'getDataDir',
          'getVault',
        ],
      },
    },
  },
};

/** Text of the `### TypeScript` section, up to the next heading. */
function typescriptSection(text: string): string {
  const start = text.search(/^### TypeScript\s*$/m);
  if (start === -1) return '';
  const rest = text.slice(start).split('\n').slice(1);
  const end = rest.findIndex((line) => /^#{1,3} /.test(line));
  return (end === -1 ? rest : rest.slice(0, end)).join('\n');
}

/** Body of the first fenced `typescript` block in a section. */
function firstTypescriptBlock(section: string): string {
  return /```typescript\n([\s\S]*?)```/.exec(section)?.[1] ?? '';
}

/** Names a snippet imports from `pkg`, by the name the package exports them under. */
function importedNames(code: string, pkg: string): string[] {
  const names: string[] = [];
  for (const m of code.matchAll(/import\s*\{([^}]*)\}\s*from\s*["']([^"']+)["']/g)) {
    if (m[2] !== pkg) continue;
    for (const part of m[1].split(',')) {
      const name = part.trim().split(/\s+as\s+/)[0];
      if (name) names.push(name);
    }
  }
  return names;
}

/** `new Class({ key: ..., shorthand })` sites: the class and the option keys passed. */
function constructions(code: string): Array<{ className: string; options: string[] }> {
  return [...code.matchAll(/new\s+(\w+)\(\s*\{([^}]*)\}/g)].map((m) => ({
    className: m[1],
    options: m[2]
      .split(',')
      .map((entry) => entry.trim().split(':')[0].trim())
      .filter(Boolean),
  }));
}

/** Everything in `code` the recorded surface of `pkg` does not have. */
function problems(code: string, surface: PackageSurface): string[] {
  const found: string[] = [];
  const imported = importedNames(code, PACKAGE);

  for (const name of imported) {
    if (!surface.exports.includes(name)) {
      found.push(`${PACKAGE} exports no \`${name}\``);
    }
  }

  for (const { className, options } of constructions(code)) {
    const cls = surface.classes[className];
    if (!cls) {
      found.push(`\`new ${className}(...)\` is not a class ${PACKAGE} exports`);
      continue;
    }
    for (const option of options) {
      if (!cls.options.includes(option)) {
        found.push(`\`${className}\` reads no \`${option}\` option`);
      }
    }
  }

  for (const m of code.matchAll(/const\s+(\w+)\s*=\s*new\s+(\w+)\(/g)) {
    const [, instance, className] = m;
    const cls = surface.classes[className];
    if (!cls) continue;
    for (const call of code.matchAll(new RegExp(`\\b${instance}\\.(\\w+)\\(`, 'g'))) {
      if (!cls.methods.includes(call[1])) {
        found.push(`\`${className}\` has no \`${call[1]}\` method`);
      }
    }
  }

  return found;
}

const section = typescriptSection(readme);
const example = firstTypescriptBlock(section);
const tableRow = readme.split('\n').find((line) => /^\|\s*TypeScript\s*\|/.test(line)) ?? '';
const namedVersion = new RegExp(`\`${PACKAGE}\` (\\d+\\.\\d+\\.\\d+)`).exec(section)?.[1];

describe('root README TypeScript example', () => {
  it('C1 the SDK table row and the example name the same package and class, on a stated version', () => {
    expect(example, 'the README has a "### TypeScript" section with a typescript block').not.toBe('');
    expect(tableRow, 'the SDKs table has a TypeScript row').toContain(`npm install ${PACKAGE}`);

    const imported = importedNames(example, PACKAGE);
    expect(imported, `the example imports from ${PACKAGE}`).not.toEqual([]);

    const rowClasses = constructions(tableRow).map((c) => c.className);
    expect(rowClasses, 'the TypeScript row shows a constructor call').not.toEqual([]);
    for (const className of rowClasses) {
      expect(imported, `the example imports the class the table row constructs`).toContain(className);
    }

    expect(namedVersion, `the section states the ${PACKAGE} version the example ran on`).toBeDefined();
    expect(
      Object.keys(RECORDED_SURFACE),
      `no recorded surface for ${PACKAGE} ${namedVersion}: run the example on it and record its exports`
    ).toContain(namedVersion);
  });

  it('C2 C3 the example and the table row use only what that version has', () => {
    // With no version stated (C1 reports that), the newest record still shows
    // what the example gets wrong.
    const surface = RECORDED_SURFACE[namedVersion ?? NEWEST_RECORDED];
    expect(surface, `no recorded surface for ${PACKAGE} ${namedVersion}`).toBeDefined();
    expect(problems(example, surface)).toEqual([]);
    expect(problems(tableRow, surface)).toEqual([]);
  });

  it('C4 the pre-fix example and two near misses are refused', () => {
    const surface = RECORDED_SURFACE[NEWEST_RECORDED];

    const preFix = [
      'import { AIMClient } from "@opena2a/aim-core";',
      '',
      'const agent = new AIMClient({ agentId: "my-first-agent" });',
      'await agent.verify({ capability: "db:read", resource: "users_table" });',
    ].join('\n');
    expect(problems(preFix, surface)).toEqual([
      '@opena2a/aim-core exports no `AIMClient`',
      '`new AIMClient(...)` is not a class @opena2a/aim-core exports',
    ]);
    expect(problems('| TypeScript | `new AIMClient({ agentId })` |', surface)).toEqual([
      '`new AIMClient(...)` is not a class @opena2a/aim-core exports',
    ]);

    // The right class with the option the old example passed.
    const wrongOption = [
      'import { AIMCore } from "@opena2a/aim-core";',
      'const agent = new AIMCore({ agentId: "my-first-agent" });',
    ].join('\n');
    expect(problems(wrongOption, surface)).toEqual(['`AIMCore` reads no `agentId` option']);

    // The right class with the server client's method.
    const wrongMethod = [
      'import { AIMCore } from "@opena2a/aim-core";',
      'const agent = new AIMCore({ agentName: "my-first-agent" });',
      'await agent.verifyAction({ action: "db:read" });',
    ].join('\n');
    expect(problems(wrongMethod, surface)).toEqual(['`AIMCore` has no `verifyAction` method']);
  });
});

import { describe, it, expect, beforeAll, afterAll, afterEach, vi } from 'vitest';
import { mkdtempSync, rmSync, writeFileSync } from 'fs';
import { tmpdir } from 'os';
import { join } from 'path';
import { inspect } from 'util';

/**
 * A config file that cannot be parsed fails with an error that names the file
 * and carries the parser's own message: for JSON the SyntaxError's message, for
 * YAML js-yaml's reason with the line and column. js-yaml's own message ends
 * with an excerpt of the file, and the lines around a syntax error can hold a
 * configuration value such as `intelligence.adapterConfig.apiKey`, so no line
 * of the file reaches the thrown error. The install hint for js-yaml appears
 * only when js-yaml cannot be loaded, not when it loads and the YAML is wrong.
 */

const state = vi.hoisted(() => ({ hideYaml: false }));

vi.mock('../node-require', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../node-require')>();
  const nodeRequire = ((id: string) => {
    if (state.hideYaml && id === 'js-yaml') {
      const err = new Error("Cannot find module 'js-yaml'") as NodeJS.ErrnoException;
      err.code = 'MODULE_NOT_FOUND';
      throw err;
    }
    return actual.nodeRequire(id);
  }) as NodeRequire;
  return { nodeRequire };
});

import { loadConfig } from './loader';
import { nodeRequire } from '../node-require';

const INSTALL_HINT = 'Install js-yaml for YAML support.';

let scratch: string;

beforeAll(() => {
  scratch = mkdtempSync(join(tmpdir(), 'arp-config-parse-'));
});

afterAll(() => {
  rmSync(scratch, { recursive: true, force: true });
});

afterEach(() => {
  state.hideYaml = false;
});

function write(name: string, content: string): string {
  const filePath = join(scratch, name);
  writeFileSync(filePath, content);
  return filePath;
}

function thrownBy(fn: () => unknown): Error {
  try {
    fn();
  } catch (err) {
    return err as Error;
  }
  throw new Error('expected a throw');
}

function parserMessage(fn: () => unknown): string {
  return thrownBy(fn).message;
}

/** The fields of the error js-yaml throws that these tests read. */
interface YamlParserError extends Error {
  reason: string;
  mark: { line: number; column: number; snippet: string };
}

/** The error js-yaml itself throws for `content`, read directly from the parser. */
function yamlParserError(content: string): YamlParserError {
  const yaml = nodeRequire('js-yaml') as { load(s: string): unknown };
  const err = thrownBy(() => yaml.load(content)) as YamlParserError;
  expect(err.name).toBe('YAMLException');
  return err;
}

/** What loadConfig keeps of a js-yaml error: the reason and the 1-based line and column. */
function reasonAndPosition(err: YamlParserError): string {
  return `${err.reason} (${err.mark.line + 1}:${err.mark.column + 1})`;
}

describe('loadConfig: a YAML config with a syntax error', () => {
  it('names the file and gives the parser reason with line and column, without the install hint', () => {
    const content = 'monitors: [unclosed\n';
    const filePath = write('arp.yaml', content);
    const parserErr = yamlParserError(content);
    expect(reasonAndPosition(parserErr)).toBe('unexpected end of the stream within a flow collection (2:1)');

    const err = thrownBy(() => loadConfig(filePath));

    expect(err.message).toContain(filePath);
    expect(err.message).toContain(reasonAndPosition(parserErr));
    expect(err.message).not.toContain(INSTALL_HINT);
    expect(err.cause).toBeUndefined();
  });

  it('does the same for a .yml file', () => {
    const content = 'agentName: [a\n';
    const filePath = write('arp.yml', content);

    const err = thrownBy(() => loadConfig(filePath));

    expect(err.message).toContain(filePath);
    expect(err.message).toContain(reasonAndPosition(yamlParserError(content)));
    expect(err.message).not.toContain(INSTALL_HINT);
    expect(err.cause).toBeUndefined();
  });
});

describe('loadConfig: a YAML syntax error carries no line of the file', () => {
  const MARKER = 'FAKE-MARKER-VALUE-0001';
  const configWithKey = [
    'agentName: demo',
    'intelligence:',
    '  enabled: true',
    '  adapterConfig:',
    `    apiKey: ${MARKER}`,
  ];

  it.each([
    ['open-flow-sequence', 'an open flow sequence two lines below the key', [...configWithKey, 'rules: [unclosed', '']],
    ['bad-indentation', 'a bad indentation one line below the key', [...configWithKey, '   model: m', '']],
  ])('%s: with %s, the value next to the error is not in the thrown error', (slug, _shape, lines) => {
    const content = lines.join('\n');
    const filePath = write(`${slug}.yaml`, content);
    const parserErr = yamlParserError(content);

    // CONTROL: js-yaml's own message and excerpt hold the value, so a loader
    // that kept either of them, or the js-yaml error as the cause, would fail
    // the assertions below.
    expect(parserErr.message).toContain(MARKER);
    expect(parserErr.mark.snippet).toContain(MARKER);

    const err = thrownBy(() => loadConfig(filePath));

    expect(err.message).toContain(filePath);
    expect(err.message).toContain(reasonAndPosition(parserErr));
    for (const text of [err.message, String(err), inspect(err, { depth: Infinity }), err.stack ?? '']) {
      expect(text).not.toContain(MARKER);
    }
    for (const line of configWithKey) {
      expect(err.message).not.toContain(line);
    }
    expect(err.cause).toBeUndefined();
  });
});

describe('loadConfig: a JSON config with a syntax error', () => {
  it('names the file and gives the parser message', () => {
    const content = '{,}';
    const filePath = write('arp.json', content);
    const expected = parserMessage(() => JSON.parse(content));

    const err = thrownBy(() => loadConfig(filePath));

    expect(err.message).toContain(filePath);
    expect(err.message).toContain(expected);
    expect(err.message).not.toContain(INSTALL_HINT);
    expect(err.cause).toBeInstanceOf(SyntaxError);
  });
});

describe('loadConfig: js-yaml cannot be loaded', () => {
  it('names the file and gives the install hint for a YAML config', () => {
    state.hideYaml = true;
    const filePath = write('hidden.yaml', 'agentName: fine\n');

    const err = thrownBy(() => loadConfig(filePath));

    expect(err.message).toContain(filePath);
    expect(err.message).toContain(INSTALL_HINT);
  });

  it('still loads a JSON config', () => {
    state.hideYaml = true;
    const filePath = write('hidden.json', '{"agentName":"json-agent"}');

    expect(loadConfig(filePath).agentName).toBe('json-agent');
  });
});

describe('loadConfig: a config that parses', () => {
  it('loads a valid YAML config over the defaults', () => {
    const filePath = write('valid.yaml', 'agentName: yaml-agent\nrules: []\n');

    const config = loadConfig(filePath);

    expect(config.agentName).toBe('yaml-agent');
    expect(config.monitors?.process?.enabled).toBe(true);
  });

  it('loads a valid JSON config over the defaults', () => {
    const filePath = write('valid.json', '{"agentName":"json-agent"}');

    const config = loadConfig(filePath);

    expect(config.agentName).toBe('json-agent');
    expect(config.monitors?.process?.enabled).toBe(true);
  });
});

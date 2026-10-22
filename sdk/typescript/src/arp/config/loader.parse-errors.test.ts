import { describe, it, expect, beforeAll, afterAll, afterEach, vi } from 'vitest';
import { mkdtempSync, rmSync, writeFileSync } from 'fs';
import { tmpdir } from 'os';
import { join } from 'path';

/**
 * A config file that cannot be parsed fails with an error that names the file
 * and carries the parser's own message. The install hint for js-yaml appears
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

describe('loadConfig: a YAML config with a syntax error', () => {
  it('names the file and gives the parser message with line and column, without the install hint', () => {
    const content = 'monitors: [unclosed\n';
    const filePath = write('arp.yaml', content);
    const yaml = nodeRequire('js-yaml') as { load(s: string): unknown };
    const expected = parserMessage(() => yaml.load(content));
    expect(expected).toContain('(2:1)');

    const err = thrownBy(() => loadConfig(filePath));

    expect(err.message).toContain(filePath);
    expect(err.message).toContain(expected);
    expect(err.message).not.toContain(INSTALL_HINT);
    expect((err.cause as Error).name).toBe('YAMLException');
  });

  it('does the same for a .yml file', () => {
    const filePath = write('arp.yml', 'agentName: [a\n');

    const err = thrownBy(() => loadConfig(filePath));

    expect(err.message).toContain(filePath);
    expect(err.message).not.toContain(INSTALL_HINT);
    expect((err.cause as Error).name).toBe('YAMLException');
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

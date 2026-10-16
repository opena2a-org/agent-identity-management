import { describe, it, expect, vi, beforeEach } from 'vitest';

/**
 * The guard-socket annotator must send the same allowlisted `event.data` view the
 * L2 prompt sends, not the raw object.
 *
 * The L2 vendor channel passes `event.data` through an allowlist: identifiers such
 * as `pid` and `patternId` travel verbatim, everything else (`command`,
 * `matchedText`, arguments, paths) is replaced with a length bucket and character
 * classes. The classification annotator built its text with
 * `JSON.stringify(event.data)`, so the same command line and matched material the
 * vendor channel withholds travelled in full over the local guard socket.
 *
 * These tests drive the real provider and annotator; only the socket client is
 * replaced, by a fake that records the text it was handed and returns no result.
 * Each case has a positive control in the same run: the fake must have been called,
 * and an allowlisted value must still be present, so an empty or never-sent text
 * cannot read as a pass.
 */

const sent: string[] = [];

vi.mock('../guard/nanomind-guard-client', () => ({
  sendClassify: async (text: string) => {
    sent.push(text);
    return null;
  },
}));

import {
  ClassificationAnnotator,
  NanoMindGuardClassificationProvider,
} from './classification-annotator';
import { allowlistEventDataForL2 } from './coordinator';
import type { ARPEvent, NanoMindGuardVerifyOptions } from '../types';

// 40 characters each, built from fragments.
const PLANTED_COMMAND = 'PLANTED' + 'COMMANDLINE' + '-' + '0123456789abcdefghijk';
const PLANTED_MATCHED = 'PLANTED' + 'MATCHEDTEXT' + '-' + 'zyxwvutsrqponmlkjihgf';

// The provider returns null before verification is reached, so these options are
// never read; they exist to satisfy the constructor.
const VERIFY_OPTIONS = {} as NanoMindGuardVerifyOptions;

function event(overrides: Partial<ARPEvent>): ARPEvent {
  return {
    id: 'evt-allowlist',
    timestamp: new Date().toISOString(),
    source: 'process',
    category: 'anomaly',
    severity: 'high',
    description: 'New child process',
    data: {},
    classifiedBy: 'L0-rules',
    ...overrides,
  } as ARPEvent;
}

async function annotateAndCapture(ev: ARPEvent): Promise<string> {
  const annotator = new ClassificationAnnotator(
    new NanoMindGuardClassificationProvider(),
    VERIFY_OPTIONS,
  );
  const result = await annotator.annotate(ev);
  expect(result, 'the fake guard returns no result, so nothing is applied').toBeNull();
  expect(sent.length, 'no text reached the guard client — nothing was measured').toBe(1);
  return sent[0];
}

beforeEach(() => {
  sent.length = 0;
});

describe('guard-socket annotator sends the allowlisted event data', () => {
  it('A1: data.command is withheld, data.pid is not', async () => {
    expect(PLANTED_COMMAND).toHaveLength(40);
    const ev = event({
      source: 'process',
      data: { pid: 4242, command: PLANTED_COMMAND },
    });

    const text = await annotateAndCapture(ev);

    // CONTROL: an allowlisted value still travels.
    expect(text).toContain('4242');
    // The material must not be there.
    expect(text, 'the command line reached the guard socket').not.toContain(PLANTED_COMMAND);

    // The withheld rendering is the one the L2 prompt gives the same key.
    const view = allowlistEventDataForL2(ev.data);
    expect(view.command).toMatch(/^<withheld: /);
    expect(text).toContain(JSON.stringify(view.command));
    expect(text).toContain(JSON.stringify(view));
  });

  it('A2: data.matchedText is withheld, data.pid is not', async () => {
    expect(PLANTED_MATCHED).toHaveLength(40);
    const ev = event({
      source: 'prompt',
      category: 'threat',
      description: '[PI-001] Instruction override attempt',
      data: { pid: 4242, patternId: 'PI-001', matchedText: PLANTED_MATCHED },
    });

    const text = await annotateAndCapture(ev);

    expect(text).toContain('4242');
    expect(text).toContain('PI-001');
    expect(text, 'the matched text reached the guard socket').not.toContain(PLANTED_MATCHED);

    const view = allowlistEventDataForL2(ev.data);
    expect(view.matchedText).toMatch(/^<withheld: /);
    expect(text).toContain(JSON.stringify(view.matchedText));
    expect(text).toContain(JSON.stringify(view));
  });
});

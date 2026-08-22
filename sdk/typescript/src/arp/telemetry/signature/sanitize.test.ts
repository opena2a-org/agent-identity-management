/**
 * The TypeScript half of the #384 class: server-supplied text reaching a
 * terminal must not be able to drive it (ANSI escapes, C1 CSI, carriage-return
 * overwrite) or flood it (unbounded length).
 */

import { describe, expect, it } from 'vitest';
import { sanitizeTerminalText, TERMINAL_TEXT_MAX_CHARS } from './sanitize';

function controlBytesIn(text: string): string[] {
  return [...text].filter(
    (ch) => (ch < ' ' && ch !== '\n' && ch !== '\t') || (ch >= '\x7f' && ch <= '\x9f'),
  );
}

describe('sanitizeTerminalText', () => {
  it('strips ANSI escape sequences to inert text', () => {
    const out = sanitizeTerminalText('\x1b[2K\x1b[1Adenied \x1b[32mALLOWED\x1b[0m');
    expect(out).not.toContain('\x1b');
    expect(out).toContain('denied');
  });

  it('strips the single-byte C1 CSI', () => {
    const out = sanitizeTerminalText('bad\x9b31mactor');
    expect(controlBytesIn(out)).toEqual([]);
    expect(out).toContain('bad');
    expect(out).toContain('actor');
  });

  it('drops carriage return, the line-overwrite primitive', () => {
    expect(sanitizeTerminalText('real reason\rFAKE LINE')).not.toContain('\r');
  });

  it('strips DEL, NUL and BEL', () => {
    expect(sanitizeTerminalText('a\x7fb\x00c\x07d')).toBe('abcd');
  });

  it('keeps newline and tab', () => {
    expect(sanitizeTerminalText('line one\n\tindented')).toBe('line one\n\tindented');
  });

  it('truncates over the cap with an explicit marker', () => {
    const out = sanitizeTerminalText('x'.repeat(100_000));
    expect(out.length).toBeLessThanOrEqual(TERMINAL_TEXT_MAX_CHARS + ' [truncated]'.length);
    expect(out.endsWith('[truncated]')).toBe(true);
  });

  it('leaves text at the cap unmarked', () => {
    const at = 'y'.repeat(TERMINAL_TEXT_MAX_CHARS);
    expect(sanitizeTerminalText(at)).toBe(at);
  });

  it('leaves ordinary text byte-identical', () => {
    expect(sanitizeTerminalText('HTTP 503: registry unavailable')).toBe(
      'HTTP 503: registry unavailable',
    );
  });

  it('handles the empty string', () => {
    expect(sanitizeTerminalText('')).toBe('');
  });
});

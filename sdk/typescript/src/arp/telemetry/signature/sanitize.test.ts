/**
 * The TypeScript half of the #384 class: server-supplied text reaching a
 * terminal must not be able to drive it (ANSI escapes, C1 CSI, carriage-return
 * overwrite) or flood it (unbounded length).
 */

import { describe, expect, it } from 'vitest';
import { sanitizeTerminalLine, sanitizeTerminalText, TERMINAL_TEXT_MAX_CHARS } from './sanitize';

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

describe('sanitizeTerminalLine', () => {
  const R = '�';

  it('replaces every C0 character, newline, carriage return and tab included, with U+FFFD', () => {
    expect(sanitizeTerminalLine('a\nb\rc\td\x00e\x1b[31mf')).toBe(`a${R}b${R}c${R}d${R}e${R}[31mf`);
  });

  it('replaces DEL and the C1 range, NEL and the single-byte CSI included', () => {
    expect(sanitizeTerminalLine('a\x7fb\x85c\x9b31md')).toBe(`a${R}b${R}c${R}31md`);
  });

  it('replaces the bidirectional formatting characters', () => {
    const bidi = [
      '\u061c', '\u200e', '\u200f',
      '\u202a', '\u202b', '\u202c', '\u202d', '\u202e',
      '\u2066', '\u2067', '\u2068', '\u2069',
    ];
    for (const ch of bidi) {
      expect(sanitizeTerminalLine(`a${ch}b`), `U+${ch.codePointAt(0)!.toString(16)}`).toBe(`a${R}b`);
    }
  });

  it('replaces a lone surrogate and keeps a character outside the BMP', () => {
    expect(sanitizeTerminalLine('a\ud800b')).toBe(`a${R}b`);
    expect(sanitizeTerminalLine('a\u{1F916}b')).toBe('a\u{1F916}b');
  });

  it('keeps letters of any script and ordinary punctuation', () => {
    expect(sanitizeTerminalLine('HTTP 400: café — שלום 名前 (x/y?z#1%2.3:4)')).toBe(
      'HTTP 400: café — שלום 名前 (x/y?z#1%2.3:4)',
    );
  });

  it('caps the length in characters, with an explicit marker, without splitting a surrogate pair', () => {
    expect(sanitizeTerminalLine('x'.repeat(TERMINAL_TEXT_MAX_CHARS))).toBe('x'.repeat(TERMINAL_TEXT_MAX_CHARS));
    expect(sanitizeTerminalLine('x'.repeat(100_000))).toBe(`${'x'.repeat(TERMINAL_TEXT_MAX_CHARS)} [truncated]`);
    expect(sanitizeTerminalLine('\u{1F916}'.repeat(5), 3)).toBe(`${'\u{1F916}'.repeat(3)} [truncated]`);
  });

  it('handles the empty string', () => {
    expect(sanitizeTerminalLine('')).toBe('');
  });
});

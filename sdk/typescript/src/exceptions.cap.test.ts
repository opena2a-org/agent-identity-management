/**
 * AIM-12 item 12: parseAPIError lifted `message` straight out of the response
 * body with no cap and no type check, so a hostile or broken server could put
 * megabytes (or an object) into every thrown error's message — the string
 * that lands in logs, terminals, and error trackers verbatim.
 */
import { describe, it, expect } from 'vitest';
import { parseAPIError, AuthenticationError } from './exceptions';

describe('parseAPIError message hygiene', () => {
  it('AIM-12.AC3 item 12: a huge body message is capped, not passed through whole', () => {
    const err = parseAPIError(500, { message: 'x'.repeat(100_000) });
    expect(err.message.length).toBeLessThanOrEqual(600);
    expect(err.message).toContain('truncated');
  });

  it('AIM-12.AC3 item 12: a non-string message falls back to "Unknown error"', () => {
    const err = parseAPIError(500, { message: { nested: 'object' } });
    expect(err.message).toBe('Unknown error');
  });

  it('AIM-12.AC3 item 12: the cap applies on every status branch, 401 included', () => {
    const err = parseAPIError(401, { error: 'y'.repeat(100_000) });
    expect(err).toBeInstanceOf(AuthenticationError);
    expect(err.message.length).toBeLessThanOrEqual(600);
  });

  it('a normal short message is untouched', () => {
    const err = parseAPIError(500, { message: 'quota exceeded' });
    expect(err.message).toBe('quota exceeded');
  });
});

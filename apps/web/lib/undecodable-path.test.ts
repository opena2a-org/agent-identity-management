import { describe, expect, it } from "vitest";
import { UNDECODABLE_PATH_DESTINATION, UNDECODABLE_TEXT, undecodablePathRewrites } from "@/lib/undecodable-path";

/**
 * UNDECODABLE_TEXT has to match exactly the text decodeURIComponent throws on. Text it misses
 * reaches the dynamic route and is answered with a plain-text 500; text it matches wrongly
 * sends a page that would render to the not-found page. next-config.test.ts checks the rule
 * as the server applies it.
 */

const undecodable = new RegExp(`^(?:${UNDECODABLE_TEXT})$`);

function decodeThrows(text: string): boolean {
  try {
    decodeURIComponent(text);
    return false;
  } catch {
    return true;
  }
}

const hex = (byte: number) => byte.toString(16).toUpperCase().padStart(2, "0");

describe("UNDECODABLE_TEXT", () => {
  it("matches a % that two hex digits do not follow, and escapes that are not UTF-8", () => {
    for (const text of ["%E0%A4%A", "%", "%ff", "agents/%E0%A4%A", "%A", "%G0", "abc%", "%C3", "%C3%A9%"]) {
      expect(decodeThrows(text), text).toBe(true);
      expect(undecodable.test(text), text).toBe(true);
    }
  });

  it("does not match text that decodes", () => {
    for (const text of ["", "abc", "00000000-0000-4000-8000-000000000001", "%41", "%2F", "%c3%a9", "%E2%82%AC", "%F0%9F%98%80", "agents/x/y"]) {
      expect(decodeThrows(text), text).toBe(false);
      expect(undecodable.test(text), text).toBe(false);
    }
  });

  it("matches an overlong form, a surrogate and a code point above U+10FFFF", () => {
    for (const text of ["%C0%AF", "%C1%BF", "%E0%80%AF", "%ED%A0%80", "%ED%BF%BF", "%F0%80%80%AF", "%F4%90%80%80", "%F5%80%80%80"]) {
      expect(decodeThrows(text), text).toBe(true);
      expect(undecodable.test(text), text).toBe(true);
    }
  });

  it("agrees with decodeURIComponent on every escape of one and two bytes and on three and four byte sequences", () => {
    // The bounds the UTF-8 ranges turn on, and a byte on each side of them.
    const bounds = [0x00, 0x7f, 0x80, 0x8f, 0x90, 0x9f, 0xa0, 0xbf, 0xc0, 0xff];
    const disagree: string[] = [];
    let checked = 0;
    const check = (text: string) => {
      checked += 1;
      if (undecodable.test(text) !== decodeThrows(text)) disagree.push(text);
    };
    for (let a = 0; a < 0x100; a++) {
      check(`%${hex(a)}`);
      check(`x/%${hex(a).toLowerCase()}y`);
      for (let b = 0; b < 0x100; b++) check(`%${hex(a)}%${hex(b)}`);
    }
    for (let a = 0xe0; a < 0xf0; a++) {
      for (let b = 0; b < 0x100; b++) for (const c of bounds) check(`%${hex(a)}%${hex(b)}%${hex(c)}`);
    }
    for (let a = 0xf0; a < 0x100; a++) {
      for (const b of bounds) for (const c of bounds) for (const d of bounds) check(`%${hex(a)}%${hex(b)}%${hex(c)}%${hex(d)}`);
    }
    expect(checked).toBe(256 * 258 + 16 * 256 * 10 + 16 * 1000);
    expect(disagree).toEqual([]);
  });
});

describe("undecodablePathRewrites", () => {
  it("rewrites the dashboard's undecodable paths to one destination, with the pattern as an unnamed group", () => {
    expect(undecodablePathRewrites()).toEqual([
      { source: `/dashboard/(${UNDECODABLE_TEXT})`, destination: UNDECODABLE_PATH_DESTINATION },
    ]);
  });
});

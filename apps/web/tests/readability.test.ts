import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { GLOBAL_CSS_FONT_SIZES, countTextRuns, fontSizePx, hitAreaPx, textBelowFloor } from "./readability";

// The page tests trust these helpers to find small text and small hit areas; this
// file proves they do, so a page check cannot pass because the helper saw nothing.

function mount(html: string): HTMLElement {
  const root = document.createElement("div");
  root.innerHTML = html;
  return root;
}

describe("textBelowFloor", () => {
  it("flags each class that renders text below 14px", () => {
    const root = mount(
      `<p class="text-xs">a</p><p class="text-[13px]">b</p><p class="text-2xs">c</p><p class="text-overline">d</p><p class="text-xs/5">e</p>`,
    );
    expect(textBelowFloor(root).map((t) => t.text)).toEqual(["a", "b", "c", "d", "e"]);
  });

  it("passes 14px and larger, and unclassed text at the 16px default", () => {
    const root = mount(`<p class="text-sm">a</p><p class="text-[15px]">b</p><p>c</p><h1 class="text-3xl">d</h1>`);
    expect(textBelowFloor(root)).toEqual([]);
    expect(countTextRuns(root)).toBe(4);
  });

  it("takes the size from the nearest ancestor that sets one", () => {
    const root = mount(`<div class="text-xs"><p><span>inner</span></p></div><div class="text-xs"><p class="text-sm">ok</p></div>`);
    expect(textBelowFloor(root)).toEqual([{ text: "inner", px: 12 }]);
  });

  it("ignores variants that do not apply at 375px and counts the ones that do", () => {
    const root = mount(
      `<p class="text-sm sm:text-xs hover:text-xs">wide only</p><p class="text-sm max-sm:text-xs">narrow</p><p class="text-sm dark:text-xs">dark</p>`,
    );
    expect(textBelowFloor(root).map((t) => t.text)).toEqual(["narrow", "dark"]);
  });

  it("checks form controls, whose text is not a text node", () => {
    const root = mount(`<input id="a" class="text-xs" /><select id="b"></select>`);
    expect(textBelowFloor(root)).toEqual([{ text: '<input id="a">', px: 12 }]);
  });
});

describe("hitAreaPx", () => {
  it("reads width and height from sizing classes", () => {
    expect(hitAreaPx(mount(`<button class="h-11 w-11"></button>`).firstElementChild!)).toEqual({ width: 44, height: 44 });
    expect(hitAreaPx(mount(`<button class="size-11"></button>`).firstElementChild!)).toEqual({ width: 44, height: 44 });
    expect(hitAreaPx(mount(`<button class="min-h-[44px] w-8"></button>`).firstElementChild!)).toEqual({ width: 32, height: 44 });
    expect(hitAreaPx(mount(`<button class="right-3 text-ink"></button>`).firstElementChild!)).toEqual({ width: 0, height: 0 });
    expect(hitAreaPx(mount(`<button class="sm:h-11 sm:w-11"></button>`).firstElementChild!)).toEqual({ width: 0, height: 0 });
  });
});

describe("global CSS font sizes", () => {
  it("knows every component class in app/globals.css that sets a font-size", () => {
    const css = readFileSync(join(__dirname, "..", "app", "globals.css"), "utf8");
    const declared: Record<string, number> = {};
    for (const m of css.matchAll(/\.([a-z0-9-]+)\s*\{[^}]*?font-size:\s*([\d.]+)px/g)) {
      declared[m[1]] = Number(m[2]);
    }
    expect(Object.keys(declared).length).toBeGreaterThan(0);
    expect(declared).toEqual(GLOBAL_CSS_FONT_SIZES);
    const el = mount(`<span class="code-block">x</span>`).firstElementChild!;
    expect(fontSizePx(el)).toBe(GLOBAL_CSS_FONT_SIZES["code-block"]);
  });
});

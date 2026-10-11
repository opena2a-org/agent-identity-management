/**
 * Links and buttons whose height, read from their classes, is under the 24 CSS
 * px minimum target of WCAG 2.5.8.
 *
 * A control is at least as tall as the height or minimum height its classes set,
 * and at least as tall as one line of its text plus its vertical padding. When
 * its classes set neither a height nor a text size (an icon button sized by its
 * child, a control styled from elsewhere), its height is not read here: the
 * browser sweep (scripts/ux-sweep.mjs) measures every target on the loaded page
 * and gates the ones under 24x24.
 *
 * Only classes without a variant count, so the reading is the phone's. A class
 * list holds both arms of a conditional, so the smallest padding and the
 * smallest line are taken. A link is inline unless its classes say otherwise,
 * and an inline box ignores a height, so a link's height class counts only
 * beside a display class. A link inside running text is exempt, as in 2.5.8.
 */

import ts from "typescript";
import { baseClass, classNameOf, type JsxTag } from "./chart-surface";

export const TARGET_MIN_PX = 24;

const CONTROLS = new Set(["a", "button", "Link"]);
const LINKS = new Set(["a", "Link"]);
const DISPLAYS = new Set(["block", "inline-block", "flex", "inline-flex", "grid", "inline-grid", "flow-root", "table"]);
const SPACING_STEP_PX = 4;

// Font size and line height of each text size: Tailwind's defaults, and text-2xs
// from tailwind.config.js (10.5px at a line height of 1.3).
const TEXT_SIZES: Record<string, [fontPx: number, linePx: number]> = {
  "2xs": [10.5, 13.65],
  xs: [12, 16],
  sm: [14, 20],
  base: [16, 24],
  lg: [18, 28],
  xl: [20, 28],
  "2xl": [24, 32],
  "3xl": [30, 36],
  "4xl": [36, 40],
};
const LEADING: Record<string, number> = { none: 1, tight: 1.25, snug: 1.375, normal: 1.5, relaxed: 1.625, loose: 2 };

export interface UndersizedControl {
  file: string;
  line: number;
  /** The lowest height the classes allow, in CSS px. */
  heightPx: number;
  classes: string;
}

/** A spacing value in px: `2.5`, `px`, or an arbitrary `[22px]`; null for anything else. */
function spacingPx(value: string): number | null {
  if (value === "px") return 1;
  if (/^\d+(?:\.\d+)?$/.test(value)) return Number(value) * SPACING_STEP_PX;
  const arbitrary = /^\[(\d+(?:\.\d+)?)px\]$/.exec(value);
  return arbitrary ? Number(arbitrary[1]) : null;
}

/** Only the classes without a variant: `sm:` and `hover:` do not hold on a phone at rest. */
function unvaried(classes: string[]): string[] {
  return classes.filter((c) => baseClass(c) === c.replace(/^!/, "")).map(baseClass);
}

/**
 * The lowest height a class list allows, or null when it sets neither a height
 * nor a text size. `inline` is a box that ignores its height classes.
 */
export function minHeightPx(classes: string[], inline = false): number | null {
  const bases = unvaried(classes);
  let fixed = 0;
  const pad: Record<"t" | "b" | "y" | "all", number[]> = { t: [], b: [], y: [], all: [] };
  const lines: number[] = [];
  const fonts: number[] = [];
  let leading: { factor?: number; px?: number } | null = null;
  for (const c of bases) {
    let m = /^(?:h|min-h|size)-(.+)$/.exec(c);
    if (m) {
      const px = spacingPx(m[1]);
      if (px !== null && !inline) fixed = Math.max(fixed, px);
      continue;
    }
    m = /^p([tby]?)-(.+)$/.exec(c);
    if (m) {
      const px = spacingPx(m[2]);
      if (px !== null) pad[(m[1] || "all") as "t" | "b" | "y" | "all"].push(px);
      continue;
    }
    m = /^text-(2xs|xs|sm|base|lg|xl|2xl|3xl|4xl)$/.exec(c);
    if (m) {
      const [font, line] = TEXT_SIZES[m[1]];
      fonts.push(font);
      lines.push(line);
      continue;
    }
    m = /^leading-(.+)$/.exec(c);
    if (m) {
      if (m[1] in LEADING) leading = { factor: LEADING[m[1]] };
      else if (spacingPx(m[1]) !== null) leading = { px: spacingPx(m[1])! };
    }
  }
  const side = (own: number[]) => {
    const set = own.length ? own : pad.y.length ? pad.y : pad.all;
    return set.length ? Math.min(...set) : 0;
  };
  let line: number | null = lines.length ? Math.min(...lines) : null;
  if (leading?.px !== undefined) line = leading.px;
  else if (leading?.factor !== undefined && fonts.length) line = Math.min(...fonts) * leading.factor;
  if (line === null) return fixed > 0 ? fixed : null;
  return Math.max(fixed, side(pad.t) + side(pad.b) + line);
}

function inRunningText(tag: JsxTag): boolean {
  const element = ts.isJsxOpeningElement(tag) ? tag.parent : tag;
  const parent = element.parent;
  if (!ts.isJsxElement(parent) && !ts.isJsxFragment(parent)) return false;
  return parent.children.some((child) => ts.isJsxText(child) && child.text.trim() !== "");
}

/** Each link or button in `text` whose classes allow a height under 24px. */
export function undersizedControls(file: string, text: string): UndersizedControl[] {
  const source = ts.createSourceFile(file, text, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  const found: UndersizedControl[] = [];
  const check = (tag: JsxTag) => {
    const name = tag.tagName.getText(source);
    if (!CONTROLS.has(name)) return;
    const classes = classNameOf(tag);
    const inline = LINKS.has(name) && !unvaried(classes).some((c) => DISPLAYS.has(c));
    const height = minHeightPx(classes, inline);
    if (height === null || height >= TARGET_MIN_PX || inRunningText(tag)) return;
    const line = source.getLineAndCharacterOfPosition(tag.getStart(source)).line + 1;
    found.push({ file, line, heightPx: Math.round(height * 100) / 100, classes: classes.join(" ") });
  };
  const visit = (node: ts.Node) => {
    if (ts.isJsxOpeningElement(node) || ts.isJsxSelfClosingElement(node)) check(node);
    ts.forEachChild(node, visit);
  };
  visit(source);
  return found;
}

/**
 * Readability floor for pages rendered in jsdom: no text below 14px at a 375px
 * viewport, and controls with a hit area of at least 44 by 44 CSS pixels.
 *
 * jsdom applies no stylesheet, so sizes are read from the classes that set them:
 * Tailwind's font-size and sizing utilities, plus the component classes in
 * app/globals.css that declare a font-size (kept in step by readability.test.ts).
 * A class behind a variant (`sm:`, `hover:`, `placeholder:`) does not apply to
 * rendered text at 375px and is skipped; `max-*:` and `dark:` variants do apply
 * there, and when an element carries more than one size the smallest counts.
 */

export const BODY_TEXT_FLOOR_PX = 14;
export const HIT_AREA_FLOOR_PX = 44;

// No font-size is set on html or body, so unclassed text renders at the browser default.
const ROOT_FONT_SIZE_PX = 16;
const SPACING_STEP_PX = 4;

const TAILWIND_FONT_SIZES: Record<string, number> = {
  "2xs": 10.5, // tailwind.config.js theme.extend.fontSize
  xs: 12,
  sm: 14,
  base: 16,
  lg: 18,
  xl: 20,
  "2xl": 24,
  "3xl": 30,
  "4xl": 36,
  "5xl": 48,
  "6xl": 60,
  "7xl": 72,
  "8xl": 96,
  "9xl": 128,
};

/** Component classes in app/globals.css that declare a font-size, in px. */
export const GLOBAL_CSS_FONT_SIZES: Record<string, number> = {
  "glass-segment-item": 11.5,
  "code-block": 12,
  "text-headline": 24,
  "text-overline": 10.5,
  "text-kpi": 32,
};

/** The classes that apply at a 375px viewport, in the light or the dark theme. */
function classesAt375(el: Element): string[] {
  return Array.from(el.classList).flatMap((cls) => {
    const parts = cls.split(":");
    const utility = parts.pop() as string;
    return parts.every((variant) => variant === "dark" || variant.startsWith("max-")) ? [utility] : [];
  });
}

function arbitraryPx(value: string): number | null {
  const m = /^(\d+(?:\.\d+)?)(px|rem)$/.exec(value);
  if (!m) return null;
  return m[2] === "rem" ? Number(m[1]) * ROOT_FONT_SIZE_PX : Number(m[1]);
}

/** The font size an element's own classes set, or null when it inherits. */
export function ownFontSizePx(el: Element): number | null {
  let smallest: number | null = null;
  for (const cls of classesAt375(el)) {
    let px: number | null = null;
    const named = /^text-(2xs|xs|sm|base|lg|[2-9]?xl)(?:\/\S+)?$/.exec(cls);
    const arbitrary = /^text-\[([^\]]+)\]$/.exec(cls);
    if (cls in GLOBAL_CSS_FONT_SIZES) px = GLOBAL_CSS_FONT_SIZES[cls];
    else if (named) px = TAILWIND_FONT_SIZES[named[1]];
    else if (arbitrary) px = arbitraryPx(arbitrary[1]);
    if (px !== null && (smallest === null || px < smallest)) smallest = px;
  }
  return smallest;
}

/** The font size an element renders at: its own, else the nearest ancestor's. */
export function fontSizePx(el: Element): number {
  for (let node: Element | null = el; node; node = node.parentElement) {
    const px = ownFontSizePx(node);
    if (px !== null) return px;
  }
  return ROOT_FONT_SIZE_PX;
}

export interface SmallText {
  text: string;
  px: number;
}

/**
 * Every visible text run and form control under `root` that renders below the
 * floor. An empty array means the floor holds.
 */
export function textBelowFloor(root: ParentNode, floorPx = BODY_TEXT_FLOOR_PX): SmallText[] {
  const found: SmallText[] = [];
  const doc = (root as Node).ownerDocument ?? (root as Document);
  const walker = doc.createTreeWalker(root as Node, NodeFilter.SHOW_TEXT);
  for (let node = walker.nextNode(); node; node = walker.nextNode()) {
    const text = node.textContent?.trim() ?? "";
    const parent = node.parentElement;
    if (!text || !parent || parent.closest("script, style")) continue;
    const px = fontSizePx(parent);
    if (px < floorPx) found.push({ text, px });
  }
  root.querySelectorAll("input, select, textarea").forEach((control) => {
    const px = fontSizePx(control);
    if (px < floorPx) {
      found.push({ text: `<${control.tagName.toLowerCase()} id="${control.id}">`, px });
    }
  });
  return found;
}

/** How many text runs `textBelowFloor` would examine, so a check cannot pass on an empty page. */
export function countTextRuns(root: ParentNode): number {
  const doc = (root as Node).ownerDocument ?? (root as Document);
  const walker = doc.createTreeWalker(root as Node, NodeFilter.SHOW_TEXT);
  let count = 0;
  for (let node = walker.nextNode(); node; node = walker.nextNode()) {
    if (node.textContent?.trim()) count++;
  }
  return count;
}

function sizeFromClasses(el: Element, axis: "h" | "w"): number {
  let px = 0;
  for (const cls of classesAt375(el)) {
    const m = new RegExp(`^(?:${axis}|size|min-${axis})-(?:(\\d+(?:\\.\\d+)?)|\\[([^\\]]+)\\])$`).exec(cls);
    if (!m) continue;
    const value = m[1] !== undefined ? Number(m[1]) * SPACING_STEP_PX : arbitraryPx(m[2]);
    if (value !== null) px = Math.max(px, value);
  }
  return px;
}

/** The box a control's own sizing classes give it, in CSS pixels (0 when unsized). */
export function hitAreaPx(el: Element): { width: number; height: number } {
  return { width: sizeFromClasses(el, "w"), height: sizeFromClasses(el, "h") };
}

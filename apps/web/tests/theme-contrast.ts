/**
 * WCAG 2.x contrast for the theme tokens in app/globals.css, computed from the
 * declarations themselves: no browser, so it runs with the unit tests and fails
 * on the commit that moves a token below its floor.
 *
 * A theme is the custom properties that hold on the root element: every
 * unconditional rule whose selector list names `:root` (light), or `:root` or
 * `.dark` (dark, where the root element carries both and source order decides).
 * Rules inside @media or @supports are left out; a caller that needs one of
 * their fallbacks (the solid glass) names that token itself.
 */

export type Rgba = [number, number, number, number];
export type Theme = "light" | "dark";

interface Rule {
  selectors: string[];
  declarations: Array<[string, string]>;
  conditional: boolean;
}

function declarationsIn(body: string): Array<[string, string]> {
  const out: Array<[string, string]> = [];
  for (const part of body.split(";")) {
    const colon = part.indexOf(":");
    if (colon < 0) continue;
    const name = part.slice(0, colon).trim();
    if (name.startsWith("--")) out.push([name, part.slice(colon + 1).trim()]);
  }
  return out;
}

function rulesIn(css: string): Rule[] {
  const text = css.replace(/\/\*[\s\S]*?\*\//g, "");
  const rules: Rule[] = [];
  const open: Array<{ prelude: string; start: number }> = [];
  let preludeStart = 0;
  for (let i = 0; i < text.length; i++) {
    if (text[i] === "{") {
      // A statement such as `@tailwind base;` or a declaration can precede the prelude.
      const prelude = text.slice(preludeStart, i).split(";").pop()!.trim();
      open.push({ prelude, start: i + 1 });
      preludeStart = i + 1;
    } else if (text[i] === "}") {
      const block = open.pop();
      if (block && !block.prelude.startsWith("@")) {
        const body = text.slice(block.start, i);
        if (!body.includes("{")) {
          rules.push({
            selectors: block.prelude.split(",").map((s) => s.trim()),
            declarations: declarationsIn(body),
            conditional: open.some((b) => /^@(?:media|supports|container)\b/.test(b.prelude)),
          });
        }
      }
      preludeStart = i + 1;
    }
  }
  return rules;
}

/** The custom properties declared on the root element in `theme`, by name. */
export function themeTokens(css: string, theme: Theme): Map<string, string> {
  const roots = theme === "light" ? [":root"] : [":root", ".dark"];
  const tokens = new Map<string, string>();
  for (const rule of rulesIn(css)) {
    if (rule.conditional || !rule.selectors.some((s) => roots.includes(s))) continue;
    for (const [name, value] of rule.declarations) tokens.set(name, value);
  }
  return tokens;
}

/** `value` with every var() replaced by what it resolves to in `tokens`. */
export function resolveValue(tokens: Map<string, string>, value: string, depth = 0): string {
  if (depth > 20) throw new Error(`var() cycle resolving ${value}`);
  const next = value.replace(/var\(\s*(--[\w-]+)\s*(?:,\s*([^()]*))?\)/g, (_m, name: string, fallback?: string) => {
    const declared = tokens.get(name) ?? fallback?.trim();
    if (declared === undefined) throw new Error(`${name} is not declared`);
    return declared;
  });
  return next === value ? value : resolveValue(tokens, next, depth + 1);
}

const COLOR = /#[0-9a-fA-F]{3,8}\b|rgba?\([^)]*\)/g;

/** `hsl(h s% l%)` or `hsl(h, s%, l%)` (the shadcn triplets) as sRGB. */
function hslToRgb(h: number, s: number, l: number): [number, number, number] {
  const sat = s / 100;
  const light = l / 100;
  const k = (n: number) => (n + h / 30) % 12;
  const a = sat * Math.min(light, 1 - light);
  const f = (n: number) => light - a * Math.max(-1, Math.min(k(n) - 3, 9 - k(n), 1));
  return [f(0) * 255, f(8) * 255, f(4) * 255];
}

export function parseColor(value: string): Rgba | null {
  const v = value.trim();
  const hex = /^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$/.exec(v);
  if (hex) {
    let h = hex[1];
    if (h.length === 3) h = [...h].map((c) => c + c).join("");
    const n = (i: number) => parseInt(h.slice(i, i + 2), 16);
    return [n(0), n(2), n(4), h.length === 8 ? n(6) / 255 : 1];
  }
  const fn = /^rgba?\(([^)]*)\)$/.exec(v);
  if (fn) {
    const parts = fn[1].split(/[\s,/]+/).filter(Boolean).map(Number);
    if (parts.length < 3 || parts.some(Number.isNaN)) return null;
    return [parts[0], parts[1], parts[2], parts[3] ?? 1];
  }
  const hsl = /^hsla?\(([^)]*)\)$/.exec(v);
  if (hsl) {
    const parts = hsl[1].split(/[\s,/]+/).filter(Boolean).map((p) => parseFloat(p));
    if (parts.length < 3 || parts.some(Number.isNaN)) return null;
    return [...hslToRgb(parts[0], parts[1], parts[2]), parts[3] ?? 1];
  }
  return null;
}

/** The colour `value` resolves to in `tokens`; an error when it names no single colour. */
export function tokenColor(tokens: Map<string, string>, value: string): Rgba {
  const resolved = resolveValue(tokens, value);
  const c = parseColor(resolved);
  if (!c) throw new Error(`${value} resolves to ${resolved}, not a colour`);
  return c;
}

/**
 * The colours a translucent surface (`fill`) takes over the page: composited
 * over each stop of the page gradient, bare and under the peak of either radial
 * wash, plus `solid`, the opaque fill drawn under reduced transparency or
 * without backdrop-filter.
 */
export function translucentSurfaces(tokens: Map<string, string>, fill: string, solid: string): Rgba[] {
  const page = [...colorsIn(resolveValue(tokens, "var(--bg-page)")), tokenColor(tokens, "var(--bg-page-solid)")];
  const washes = ["var(--wash-1)", "var(--wash-2)"].map((w) => tokenColor(tokens, w));
  const grounds = page.flatMap((p) => [p, ...washes.map((w) => over(w, p))]);
  const surface = tokenColor(tokens, fill);
  return [...grounds.map((g) => over(surface, g)), tokenColor(tokens, solid)];
}

/** Every colour a value names, in order: one for a plain colour, each stop for a gradient. */
export function colorsIn(value: string): Rgba[] {
  return (value.match(COLOR) ?? []).map((c) => parseColor(c)).filter((c): c is Rgba => c !== null);
}

/** `top` painted over the opaque `bottom` (source-over). */
export function over(top: Rgba, bottom: Rgba): Rgba {
  const a = top[3];
  return [0, 1, 2].map((i) => top[i] * a + bottom[i] * (1 - a)).concat(1) as Rgba;
}

function channel(c: number): number {
  const s = c / 255;
  return s <= 0.04045 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
}

export function luminance([r, g, b]: Rgba): number {
  return 0.2126 * channel(r) + 0.7152 * channel(g) + 0.0722 * channel(b);
}

/** The WCAG 2.x contrast ratio of two opaque colours. */
export function contrastRatio(a: Rgba, b: Rgba): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
}

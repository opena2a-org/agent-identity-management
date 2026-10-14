/**
 * The colour a rendered element's text is drawn in, read from its classes
 * through the theme colours in tailwind.config.js, and the surfaces the shared
 * navigation and the overview's inset tiles are drawn on.
 *
 * tests/text-contrast.test.ts holds the tokens to the floor on these surfaces;
 * a component test renders the element and holds the colour it actually
 * carries to the same floor, so moving the element back to a colour under it
 * fails a unit test, not only the browser sweep.
 */
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { join } from "node:path";
import { contrastRatio, over, themeTokens, tokenColor, translucentSurfaces, type Rgba, type Theme } from "./theme-contrast";

export const TEXT_FLOOR = 4.5;
export const THEMES: Theme[] = ["light", "dark"];

type ColourTree = { [name: string]: string | ColourTree };

const require = createRequire(import.meta.url);
const tailwind = require("../tailwind.config.js") as {
  theme: { fontSize: Record<string, unknown>; extend: { colors: ColourTree } };
};
const css = readFileSync(join(__dirname, "..", "app", "globals.css"), "utf8");

function flatten(tree: ColourTree, prefix = ""): Array<[string, string]> {
  return Object.entries(tree).flatMap(([key, value]) => {
    const name = key === "DEFAULT" ? prefix : prefix ? `${prefix}-${key}` : key;
    return typeof value === "string" ? [[name, value] as [string, string]] : flatten(value, name);
  });
}

// The theme colours Tailwind generates a `text-*` utility for, by the utility's suffix.
const THEME_COLOURS = new Map(flatten(tailwind.theme.extend.colors));

// `text-*` utilities that set no colour: the font sizes (theme.fontSize is the
// whole scale), a set size, alignment and wrapping.
const SIZES = new Set(Object.keys(tailwind.theme.fontSize));
const NOT_A_COLOUR = /^text-(?:\[\d*\.?\d+(?:px|rem|em)\]|left|center|right|justify|start|end|wrap|nowrap|balance|pretty|ellipsis|clip)$/;

function describe(el: Element): string {
  return `<${el.tagName.toLowerCase()} class="${el.getAttribute("class") ?? ""}">`;
}

/**
 * The theme colour `el` is drawn in (a value such as `var(--text-secondary)`):
 * the `text-*` colour on the element, or on the nearest ancestor that sets one.
 * A `hover:` colour is not the resting colour and is not read. A `dark:` colour,
 * or a `text-*` class that is neither a theme colour nor a size or alignment, is
 * an error: the theme tokens do not measure it.
 */
export function drawnTextColour(el: Element): string {
  for (let node: Element | null = el; node; node = node.parentElement) {
    const classes = (node.getAttribute("class") ?? "").split(/\s+/);
    const darkOnly = classes.filter((c) => c.startsWith("dark:text-"));
    if (darkOnly.length) throw new Error(`${describe(node)} sets ${darkOnly.join(" ")}, a colour for one theme`);
    const texts = classes.filter((c) => c.startsWith("text-"));
    const colours = texts.filter((c) => THEME_COLOURS.has(c.slice("text-".length)));
    if (colours.length > 1) throw new Error(`${describe(node)} sets ${colours.join(" and ")}`);
    if (colours.length === 1) return THEME_COLOURS.get(colours[0].slice("text-".length))!;
    const unmeasured = texts.filter((c) => !SIZES.has(c.slice("text-".length)) && !NOT_A_COLOUR.test(c));
    if (unmeasured.length) throw new Error(`${describe(node)} sets ${unmeasured.join(" ")}, which is not a theme colour`);
  }
  throw new Error(`${describe(el)} and its ancestors set no text colour`);
}

/** The custom properties on the root element in `theme`, from app/globals.css. */
export function rootTokens(theme: Theme): Map<string, string> {
  return themeTokens(css, theme);
}

/**
 * The surfaces under the shared navigation's active link: its fill over the
 * glass chrome of the sidebar and over the opaque phone drawer.
 */
export function activeNavigationGrounds(tokens: Map<string, string>): Rgba[] {
  const grounds = [
    ...translucentSurfaces(tokens, "var(--glass-chrome-fill)", "var(--glass-chrome-solid)"),
    tokenColor(tokens, "hsl(var(--popover))"),
  ];
  const fill = tokenColor(tokens, "var(--nav-active-fill)");
  return grounds.map((g) => over(fill, g));
}

/** The surfaces under the text of an inset tile on a glass card. */
export function insetTileGrounds(tokens: Map<string, string>): Rgba[] {
  const inset = tokenColor(tokens, "var(--surface-inset-gray)");
  return translucentSurfaces(tokens, "var(--glass-fill)", "var(--glass-fill-solid)").map((g) => over(inset, g));
}

/** The lowest contrast `fg` reaches over any of the opaque `grounds`. */
export function lowestContrast(fg: Rgba, grounds: Rgba[]): number {
  return Math.min(...grounds.map((bg) => contrastRatio(over(fg, bg), bg)));
}

/**
 * Where the theme colour `colour` reads under the floor over the surfaces
 * `groundsOf` gives for each theme, as `<theme> <colour> <ratio>:1`; empty when
 * it holds the floor in both themes.
 */
export function shortfalls(colour: string, groundsOf: (tokens: Map<string, string>) => Rgba[]): string[] {
  return THEMES.flatMap((theme) => {
    const tokens = rootTokens(theme);
    const ratio = lowestContrast(tokenColor(tokens, colour), groundsOf(tokens));
    return ratio < TEXT_FLOOR ? [`${theme} ${colour} ${ratio.toFixed(2)}:1`] : [];
  });
}

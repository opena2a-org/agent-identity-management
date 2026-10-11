/**
 * Colours written into source instead of taken from the theme tokens in
 * app/globals.css: hex values, functional colour notation, and Tailwind palette
 * classes. Read line by line, so a class held in a lookup table or a variable
 * counts the same as one written in a className.
 */

export const HEX = /#[0-9a-fA-F]{3,6}/;
export const COLOR_FUNCTION = /\b(?:rgb|rgba|hsl|hsla)\(/;
export const PALETTE_CLASS =
  /\b(?:text|bg|stroke|fill|border|ring|outline|from|via|to|divide|shadow|accent|decoration)-(?:slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose)-\d{2,3}\b/;

/** Each line of `text` that matches one of `patterns`, as `<line>: <match>`. */
export function linesMatching(text: string, patterns: RegExp[]): string[] {
  const found: string[] = [];
  text.split("\n").forEach((line, i) => {
    for (const pattern of patterns) {
      const m = line.match(pattern);
      if (m) found.push(`${i + 1}: ${m[0]}`);
    }
  });
  return found;
}

/** Hex, rgb/hsl and palette-class colours, one entry per pattern a line matches. */
export function rawColours(text: string): string[] {
  return linesMatching(text, [HEX, COLOR_FUNCTION, PALETTE_CLASS]);
}

/** Tailwind palette classes only. */
export function paletteClasses(text: string): string[] {
  return linesMatching(text, [PALETTE_CLASS]);
}

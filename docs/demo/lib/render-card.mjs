// Render the title and end cards from lib/card.html to 1920x1080 PNGs.
// Usage (inside the browser image): node render-card.mjs <narration.md> <outdir> "<stamp>"
import fs from "node:fs";
import path from "node:path";
import { chromium } from "playwright";

const [narrationFile, outDir, stamp = ""] = process.argv.slice(2);
const lines = fs.readFileSync(narrationFile, "utf8").split("\n");
function card(id) {
  const i = lines.findIndex((l) => l.startsWith(`## ${id} `));
  const body = [];
  for (let j = i + 1; j < lines.length && !lines[j].startsWith("## "); j++) if (lines[j].trim()) body.push(lines[j].trim());
  return { title: body[0] || "", lines: body.slice(1) };
}
const html = "file://" + path.resolve(path.dirname(new URL(import.meta.url).pathname), "card.html");
fs.mkdirSync(outDir, { recursive: true });
const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: 1920, height: 1080 } });
for (const [id, kind, file] of [["s01", "title", "title.png"], ["s14", "end", "end.png"]]) {
  const c = card(id);
  const q = new URLSearchParams({ kind, title: c.title, lines: c.lines.join("\n"), stamp });
  await page.goto(`${html}?${q}`);
  await page.waitForTimeout(300);
  await page.screenshot({ path: path.join(outDir, file) });
}
await browser.close();
console.log(`cards rendered in ${outDir}`);

// Extract the typed lines of the README's Quick start: every line of every
// fenced code block under `## Quick start`, at the checked-out commit. The
// tape may type only these lines outside Hide (lint.mjs enforces it), so the
// video cannot drift from the documentation it records.
//
//   node docs/demo/lib/extract-readme.mjs [path/to/README.md]   -> JSON lines on stdout
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));

export function quickStartSection(md) {
  const lines = md.split("\n");
  const start = lines.findIndex((l) => /^## quick start\s*$/i.test(l));
  if (start < 0) throw new Error("no '## Quick start' heading in the README");
  let end = lines.findIndex((l, i) => i > start && /^## /.test(l));
  if (end < 0) end = lines.length;
  return lines.slice(start, end).join("\n");
}

export function codeLines(section) {
  const out = [];
  let inFence = false;
  for (const raw of section.split("\n")) {
    if (/^```/.test(raw)) { inFence = !inFence; continue; }
    if (!inFence) continue;
    const line = raw.replace(/\s+#.*$/, "").trimEnd(); // a trailing shell comment is not typed
    if (line.trim() === "") continue;
    out.push(line);
  }
  return out;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const readme = process.argv[2] || path.join(here, "..", "..", "..", "README.md");
  const section = quickStartSection(fs.readFileSync(readme, "utf8"));
  console.log(JSON.stringify({ readme: path.relative(process.cwd(), readme), lines: codeLines(section) }, null, 2));
}

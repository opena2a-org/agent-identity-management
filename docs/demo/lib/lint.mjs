// Lint a video's inputs before anything is recorded:
//   1. every visible `Type` line of the rendered tape is a Quick start line of the README
//      (lines typed inside Hide ... Show are preparation and are not checked);
//   2. narration.md follows the aim-demo-narration/1 grammar: scene headers, one
//      caption per line, at most 84 characters wrapped at 42, at most 15 characters
//      per second of the scene's target, sentence case, no exclamation marks, no emojis;
//   3. no forbidden string (internal names, credential shapes, hosts outside the allow-list)
//      in the tape, the narration or the walkthrough.
//
//   node docs/demo/lib/lint.mjs <slug> [--tape path/to/rendered.tape]   exit 0 = clean
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { quickStartSection, codeLines } from "./extract-readme.mjs";

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.join(here, "..", "..", "..");
const slug = process.argv[2];
if (!slug) { console.error("usage: lint.mjs <slug> [--tape file]"); process.exit(2); }
const tapeArg = process.argv.indexOf("--tape");
const tapePath = tapeArg > 0 ? process.argv[tapeArg + 1] : path.join(here, "..", slug, "tape.tape");
const dir = path.join(here, "..", slug);
const problems = [];

// 1. typed lines
const rehearsal = process.argv.includes("--rehearsal");
const readmeLines = new Set(codeLines(quickStartSection(fs.readFileSync(path.join(root, "README.md"), "utf8"))));
const rehearsalLines = new Set();
if (rehearsal) {
  // a rehearsal may also type the lines listed as README gaps (each one is a fix the README owes)
  const gaps = path.join(dir, "rehearsal-lines.txt");
  if (fs.existsSync(gaps)) for (const l of fs.readFileSync(gaps, "utf8").split("\n")) if (l.trim() && !l.startsWith("#")) rehearsalLines.add(l);
}
const rehearsalOnly = [];
const tape = fs.readFileSync(tapePath, "utf8").split("\n");
let hidden = false;
for (const [i, line] of tape.entries()) {
  const t = line.trim();
  if (/^Hide\b/.test(t)) hidden = true;
  if (/^Show\b/.test(t)) hidden = false;
  const m = t.match(/^Type\s+(["'])(.*)\1\s*$/);
  if (!m || hidden) continue;
  const typed = m[2];
  if (readmeLines.has(typed)) continue;
  if (rehearsalLines.has(typed)) { rehearsalOnly.push(typed); continue; }
  problems.push(`${path.relative(root, tapePath)}:${i + 1}: typed line is not in the README Quick start: ${typed}`);
}

// 2. narration grammar
const narrationPath = path.join(dir, "narration.md");
const narration = fs.readFileSync(narrationPath, "utf8").split("\n");
if (!/^format: aim-demo-narration\/1$/m.test(narration.join("\n"))) problems.push("narration.md: missing 'format: aim-demo-narration/1'");
let scene = null;
const avoid = /\b(seamless|seamlessly|revolutionary|cutting-edge|best-in-class|world-class|leverage|synerg|effortless|magic|product)\b/i;
for (const [i, line] of narration.entries()) {
  const h = line.match(/^## (s\d\d)\s+·\s+(card|terminal|browser)\s+·\s+(\d+(?:\.\d+)?) s\s+·\s+(.+)$/);
  if (h) { scene = { id: h[1], kind: h[2], seconds: Number(h[3]), chars: 0, line: i + 1 }; continue; }
  if (line.startsWith("## ")) { problems.push(`narration.md:${i + 1}: bad scene header: ${line}`); continue; }
  if (!scene || line.trim() === "" || /^[#a-z]+:/.test(line) && i < 6) continue;
  if (line.trim().startsWith("#")) continue;
  const caption = line.trim();
  if (caption.length > 84) problems.push(`narration.md:${i + 1}: caption longer than 84 characters`);
  if (/!/.test(caption)) problems.push(`narration.md:${i + 1}: exclamation mark`);
  if (/[\u{1F300}-\u{1FAFF}\u{2600}-\u{27BF}]/u.test(caption)) problems.push(`narration.md:${i + 1}: emoji`);
  if (avoid.test(caption)) problems.push(`narration.md:${i + 1}: vocabulary the builder persona avoids: ${caption.match(avoid)[0]}`);
  if (scene.kind !== "card" && /^[a-z]/.test(caption) && !/^`/.test(caption)) problems.push(`narration.md:${i + 1}: caption does not start with a capital or a literal`);
  scene.chars += caption.length;
  if (scene.kind !== "card" && scene.chars / scene.seconds > 15) problems.push(`narration.md:${i + 1}: ${scene.id} exceeds 15 characters per second (${scene.chars} chars in ${scene.seconds} s)`);
}

// 3. forbidden strings
const forbidden = [
  [/eyJ[A-Za-z0-9_-]{10,}/, "JWT fragment"], [/aim_live_/, "API key prefix"], [/Bearer\s+[A-Za-z0-9._-]{8,}/, "bearer token"],
  [/\/Users\/|\/home\/(?!demo\b)[a-z]|~\/workspace|\/private\/tmp/, "local path"],
  [/[A-Za-z0-9._%+-]+@(?!example\.com\b)[A-Za-z0-9.-]+\.[a-z]{2,}/, "email outside example.com"],
  [/https?:\/\/(?!localhost(?::\d+)?\b|github\.com\/opena2a-org\b|opena2a\.org\/docs\b|pypi\.org\b|files\.pythonhosted\.org\b)[a-z0-9.-]+/i, "host outside the allow-list"],
];
// Extra patterns kept outside this repository: `class<TAB>regex<TAB>canary` lines (--forbidden=<file> or $DEMO_FORBIDDEN_FILE).
const extraArg = process.argv.indexOf("--forbidden");
const extraFile = extraArg > 0 ? process.argv[extraArg + 1] : process.env.DEMO_FORBIDDEN_FILE;
if (extraFile && fs.existsSync(extraFile)) {
  for (const line of fs.readFileSync(extraFile, "utf8").split("\n")) {
    if (!line.trim() || line.startsWith("#")) continue;
    const [cls, re] = line.split("\t");
    if (cls && re) forbidden.push([new RegExp(re, "i"), cls]);
  }
}
for (const file of [tapePath, narrationPath, path.join(dir, "walkthrough.mjs")]) {
  if (!fs.existsSync(file)) continue;
  const content = fs.readFileSync(file, "utf8").split("\n");
  for (const [i, line] of content.entries()) {
    for (const [re, why] of forbidden) {
      if (re.test(line)) problems.push(`${path.relative(root, file)}:${i + 1}: ${why}`);
    }
  }
}

if (problems.length) { console.error(problems.join("\n")); console.error(`${problems.length} problem(s)`); process.exit(3); }
if (rehearsalOnly.length) console.log(`lint: rehearsal-only lines (README gaps): ${rehearsalOnly.join(" | ")}`);
console.log(`lint: ${slug} clean (${readmeLines.size} README lines, ${tape.length} tape lines, ${narration.length} narration lines)`);

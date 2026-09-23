// Deterministic census of everything a viewer could read in the video: the
// terminal transcript, the per-scene text snapshots of the dashboard, the
// captions, the card text and the listing metadata. It fails on any secret
// of the run (searched as fixed strings inside this process, never printed),
// any credential shape, any internal name, any email outside example.com,
// any host outside the allow-list and any public IP address.
//
// A positive control runs first: a copy of the transcript with one planted
// canary per class must be reported, otherwise the census is inconclusive and
// exits 4. Usage: node census.mjs --out=/work/out --scene=/work/scene --env=/work/run/.env [--metadata=file]
import fs from "node:fs";
import path from "node:path";

const args = Object.fromEntries(process.argv.slice(2).map((a) => { const m = a.match(/^--([a-z-]+)=(.*)$/); return m ? [m[1], m[2]] : [a, true]; }));
const OUT = args.out || "/work/out", SCENE = args.scene || "/work/scene";
const secrets = [];
if (args.env && fs.existsSync(args.env)) {
  for (const line of fs.readFileSync(args.env, "utf8").split("\n")) {
    const m = line.match(/^([A-Z0-9_]+)=(.*)$/);
    if (m && /PASSWORD|SECRET|KEY|TOKEN/.test(m[1]) && m[2].length >= 8) secrets.push({ name: m[1], value: m[2] });
  }
}
const classes = [
  ["jwt", /eyJ[A-Za-z0-9_-]{10,}/, "eyJhbGciOiJIUzI1NiJ9.canary"],
  ["api-key", /aim_live_[A-Za-z0-9]{6,}/, "aim_live_canary0000"],
  ["bearer", /Bearer\s+[A-Za-z0-9._-]{8,}/, "Bearer canarytoken00"],
  ["token-field", /"(refreshToken|accessToken)"\s*:\s*"[^"]{6,}"/, '"refreshToken": "canary000000"'],
  ["local-path", /\/Users\/|\/home\/(?!demo\b)[a-z]|~\/workspace|\/private\/tmp|\/tmp\/[a-z]+-[a-z0-9]{6,}/, "/Users/canary"],
  ["email", /[A-Za-z0-9._%+-]+@(?!example\.com\b)[A-Za-z0-9.-]+\.[a-z]{2,}/, "canary@canary.org"],
  ["own-fixture", /admin\.opena2a\.org|OpenA2A Admin|aim\.opena2a\.org/, "OpenA2A Admin"],
  ["host", /https?:\/\/(?!localhost(?::\d+)?(?:\/|$|\s)|github\.com\/opena2a-org|opena2a\.org\/docs|pypi\.org|files\.pythonhosted\.org)[a-z0-9.-]+/i, "https://canary.example.net/x"],
  ["public-ip", /\b(?!10\.|127\.|192\.168\.|172\.(1[6-9]|2\d|3[01])\.|0\.0\.0\.0)(\d{1,3}\.){3}\d{1,3}\b/, "203.0.113.7"],
];
// Extra classes, kept outside this repository: a tab-separated file of
// `class<TAB>regex<TAB>canary` lines (--forbidden=<file> or $DEMO_FORBIDDEN_FILE).
const extraFile = args.forbidden || process.env.DEMO_FORBIDDEN_FILE;
if (extraFile && fs.existsSync(extraFile)) {
  for (const line of fs.readFileSync(extraFile, "utf8").split("\n")) {
    if (!line.trim() || line.startsWith("#")) continue;
    const [cls, re, canary] = line.split("\t");
    if (cls && re && canary) classes.push([cls, new RegExp(re, "i"), canary]);
  }
}

function scan(name, text) {
  const hits = [];
  for (const s of secrets) if (text.includes(s.value)) hits.push({ source: name, class: "run-secret", detail: s.name });
  for (const [cls, re] of classes) { const m = text.match(re); if (m) hits.push({ source: name, class: cls, detail: m[0].slice(0, 40) }); }
  return hits;
}

const inputs = [];
const push = (label, file) => { if (fs.existsSync(file)) inputs.push([label, fs.readFileSync(file, "utf8").replace(/\x1b\[[0-9;?]*[A-Za-z]/g, "")]); };
push("transcript", path.join(OUT, "transcript.txt"));
push("captions", path.join(OUT, "captions.srt"));
push("narration", path.join(SCENE, "narration.md"));
if (args.metadata) push("metadata", args.metadata);
const textDir = path.join(OUT, "browser-text");
if (fs.existsSync(textDir)) for (const f of fs.readdirSync(textDir).sort()) push(`browser:${f}`, path.join(textDir, f));

// positive control: every class must be found in a planted copy
const planted = (inputs.find((i) => i[0] === "transcript")?.[1] || "") + "\n" + classes.map((c) => c[2]).join("\n") + (secrets[0] ? "\n" + secrets[0].value : "");
const control = scan("control", planted);
const missing = classes.map((c) => c[0]).filter((cls) => !control.some((h) => h.class === cls));
if (secrets.length && !control.some((h) => h.class === "run-secret")) missing.push("run-secret");
if (missing.length) { console.error(`census INCONCLUSIVE: the control did not report ${missing.join(", ")}`); process.exit(4); }

const hits = inputs.flatMap(([n, t]) => scan(n, t));
const report = { inputs: inputs.map((i) => i[0]), secretsChecked: secrets.map((s) => s.name), controlClasses: classes.length, hits };
fs.writeFileSync(path.join(OUT, "census.json"), JSON.stringify(report, null, 2));
if (hits.length) { console.error(JSON.stringify(hits, null, 2)); console.error(`census: ${hits.length} hit(s)`); process.exit(3); }
console.log(`census: clean (${inputs.length} inputs, ${secrets.length} run secrets, ${classes.length} classes, control reported all)`);

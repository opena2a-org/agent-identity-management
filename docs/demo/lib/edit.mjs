// Assemble the video from the recorded threads. Runs inside the terminal
// image (ffmpeg with libx264, libass and the fonts). Inputs under /work/out:
//   terminal.mp4          the VHS recording (1600x900)
//   browser/browser.webm  the Playwright recording (1600x900) + browser/timing.json
//   cue-*                 wall-clock stamps written by the tape and the walkthrough
//   transcript.timing     timing of the terminal transcript (script --log-timing)
//   cards/title.png, cards/end.png
// and /work/scene/narration.md. Output: the MP4 named by --output and
// recording.json next to it (scene boundaries, edits, hashes).
//
// Scene boundaries are read from the instrument's own markers: the terminal's
// screen clears (near-empty frames after each Hide block), the freeze that
// ends at the login cut, and the browser's event timestamps. Nothing is
// guessed from content.
import crypto from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import { execFileSync } from "node:child_process";

const OUT = process.env.DEMO_OUT_DIR || "/work/out";
const SCENE = process.env.DEMO_SCENE_DIR || "/work/scene";
const args = Object.fromEntries(process.argv.slice(2).map((a) => { const m = a.match(/^--([a-z0-9-]+)=(.*)$/); return m ? [m[1], m[2]] : [a, true]; }));
const output = args.output || path.join(OUT, "video.mp4");
const rehearsal = args.rehearsal === "1" || args.rehearsal === true;
const stamp = args.stamp || "";
const W = 1920, H = 1080, CW = 1600, CH = 900, FPS = 30, BAND_Y = 900;
const MIN_HOLD = 0.5;

const ff = (a) => execFileSync("ffmpeg", ["-hide_banner", "-loglevel", "error", "-y", ...a], { stdio: ["ignore", "pipe", "pipe"] });
const ffprobe = (file) => JSON.parse(execFileSync("ffprobe", ["-v", "error", "-show_streams", "-show_format", "-of", "json", file]).toString());
const duration = (file) => Number(ffprobe(file).format.duration);
const sha256 = (file) => crypto.createHash("sha256").update(fs.readFileSync(file)).digest("hex");
const readCue = (name) => { const f = path.join(OUT, `cue-${name}`); return fs.existsSync(f) ? Number(fs.readFileSync(f, "utf8").trim()) : null; };

// ---- narration -------------------------------------------------------------
function parseNarration(file) {
  const scenes = [];
  let cur = null;
  for (const line of fs.readFileSync(file, "utf8").split("\n")) {
    const h = line.match(/^## (s\d\d)\s+·\s+(card|terminal|browser)\s+·\s+(\d+(?:\.\d+)?) s\s+·\s+(.+)$/);
    if (h) { cur = { id: h[1], kind: h[2], target: Number(h[3]), chapter: h[4].trim(), captions: [] }; scenes.push(cur); continue; }
    if (!cur || !line.trim() || line.startsWith("#")) continue;
    cur.captions.push(line.trim());
  }
  return scenes;
}

// ---- terminal markers ------------------------------------------------------
function frameLuma(file) {
  // Average luma per frame: a cleared screen with only a prompt is the darkest frame class.
  const out = execFileSync("ffprobe", ["-v", "error", "-f", "lavfi", "-i", `movie=${file},signalstats`, "-show_entries", "frame=pts_time:frame_tags=lavfi.signalstats.YAVG", "-of", "csv=p=0"]).toString();
  return out.trim().split("\n").map((l) => { const [t, y] = l.split(","); return { t: Number(t), y: Number(y) }; });
}

function lumaJumps(frames) {
  // A screen clear is a sharp fall of the mean luma within one frame (content
  // to an empty prompt); the login cut is a sharp rise (the success text
  // replaces a frozen screen). Typing, output and scrolling change the mean
  // gradually, so a jump of more than 1.5 luma units in one frame is a marker.
  const falls = [], rises = [];
  for (let i = 1; i < frames.length; i++) {
    const d = frames[i].y - frames[i - 1].y;
    if (d < -1.5) falls.push({ t: frames[i].t, d });
    if (d > 1.5) rises.push({ t: frames[i].t, d });
  }
  return { falls, rises };
}

// ---- timeline --------------------------------------------------------------
const narration = parseNarration(path.join(SCENE, "narration.md"));
const byId = Object.fromEntries(narration.map((s) => [s.id, s]));
const term = path.join(OUT, "terminal.mp4");
const browserVideo = path.join(OUT, "browser", "browser.webm");
const timing = JSON.parse(fs.readFileSync(path.join(OUT, "browser", "timing.json"), "utf8"));
const termDur = duration(term);
const frames = frameLuma(term);
const { falls, rises } = lumaJumps(frames);   // s04, s09, s11, s12 begin at a cleared screen
const clears = falls.filter((f, i) => i === 0 || f.t - falls[i - 1].t > 2.0).map((f) => ({ start: f.t }));
if (clears.length < 4) throw new Error(`expected 4 screen clears in terminal.mp4, found ${clears.length}: ${JSON.stringify(clears)}`);
const [c04, c09, c11, c12] = clears.slice(0, 4);
// the login cut: the largest rise after the code box has been on screen for a while and before s09
const cut = rises.filter((r) => r.t > c04.start + 3.0 && r.t < c09.start - 1.0).sort((a, b) => b.d - a.d)[0];
if (!cut) throw new Error("no luma rise marks the login cut between s04 and s09");
const s08start = cut.t;
const tb = { s03: [0, c04.start], s04: [c04.start, s08start], s08: [s08start, c09.start], s09: [c09.start, c11.start], s11: [c11.start, c12.start], s12: [c12.start, termDur] };
const bs = (id) => [timing.scenes[id].start, timing.scenes[id].end];

// measured hidden waits (wall clock), labelled on the scene that follows them
const cue = { s04: readCue("s04"), approved: readCue("approved"), registered: readCue("registered"), verified: readCue("verified"), s11: readCue("s11") };
const skipped = {};
if (cue.s04 && cue.approved) skipped.s08 = Math.max(0, Math.round((cue.approved - cue.s04) - (tb.s04[1] - tb.s04[0])));
if (cue.registered && cue.s11) skipped.s11 = Math.max(0, Math.round(cue.s11 - cue.registered));

const labels = { s02: "terminal · from the end of this video", s03: "terminal", s04: "terminal", s08: "terminal", s09: "terminal", s11: "terminal", s12: "terminal",
  s05: "localhost:3000/device", s06: "localhost:3000/auth/login", s07: "localhost:3000/device", s10: "localhost:3000/dashboard/agents", s13: "localhost:3000/dashboard/agents/…" };
for (const id of Object.keys(skipped)) labels[id] += ` · ${skipped[id]} s skipped`;

// segments in join order; each is {id, src, from, to, still}
const order = ["s01", "s02", "s03", "s04", "s05", "s06", "s07", "s08", "s09", "s10", "s11", "s12", "s13", "s14"];
const segs = [];
for (const id of order) {
  const n = byId[id]; if (!n) throw new Error(`narration lacks ${id}`);
  if (id === "s01") segs.push({ id, still: path.join(OUT, "cards", "title.png"), len: n.target });
  else if (id === "s14") segs.push({ id, still: path.join(OUT, "cards", "end.png"), len: n.target });
  else if (id === "s02") segs.push({ id, src: term, from: Math.max(0, termDur - 1.5), to: termDur - 1.0, hold: n.target });
  else if (n.kind === "terminal") segs.push({ id, src: term, from: tb[id][0], to: tb[id][1] });
  else segs.push({ id, src: browserVideo, from: bs(id)[0], to: bs(id)[1] });
}

// reading time: hold the last frame until the captions fit 15 characters per second
for (const s of segs) {
  const n = byId[s.id];
  const chars = n.captions.join(" ").length;
  const need = n.kind === "card" ? n.target : Math.max(chars / 15 + MIN_HOLD, 2.0);
  const actual = s.still ? s.len : (s.hold ? s.hold : s.to - s.from);
  s.len = Math.max(actual, need);
  s.holdFor = Math.max(0, s.len - (s.still ? s.len : (s.hold ? (s.to - s.from) : (s.to - s.from))));
}

// ---- intermediates ---------------------------------------------------------
const work = path.join(OUT, "edit"); fs.mkdirSync(work, { recursive: true });
const parts = [];
const canvas = `scale=${CW}:${CH}:force_original_aspect_ratio=decrease,pad=${CW}:${CH}:(ow-iw)/2:(oh-ih)/2:color=0x1e1e2e,pad=${W}:${H}:160:0:color=0x1e1e2e,fps=${FPS},format=yuv444p`;
for (const [i, s] of segs.entries()) {
  const file = path.join(work, `${String(i).padStart(2, "0")}-${s.id}.mkv`);
  if (s.still) {
    ff(["-loop", "1", "-framerate", String(FPS), "-t", String(s.len), "-i", s.still, "-vf", `scale=${W}:${H},fps=${FPS},format=yuv444p`, "-c:v", "libx264", "-qp", "0", file]);
  } else {
    const clip = s.hold ? s.to - s.from : s.to - s.from;
    const hold = s.hold ? s.len - clip : s.holdFor;
    const vf = canvas + (hold > 0.01 ? `,tpad=stop_mode=clone:stop_duration=${hold.toFixed(3)}` : "");
    ff(["-ss", s.from.toFixed(3), "-to", s.to.toFixed(3), "-i", s.src, "-vf", vf, "-an", "-c:v", "libx264", "-qp", "0", file]);
  }
  s.file = file; s.len = duration(file);
  parts.push(file);
}
let t = 0; for (const s of segs) { s.start = t; t += s.len; s.end = t; }
const total = t;

// ---- captions (ASS) --------------------------------------------------------
const esc = (s) => s.replace(/\\/g, "/").replace(/\{/g, "(").replace(/\}/g, ")");
const mono = (s) => s.replace(/`([^`]+)`/g, "{\\fnDejaVu Sans Mono}$1{\\fnDejaVu Sans}");
const ts = (sec) => { const h = Math.floor(sec / 3600), m = Math.floor((sec % 3600) / 60), s = (sec % 60).toFixed(2).padStart(5, "0"); return `${h}:${String(m).padStart(2, "0")}:${s}`; };
let ass = `[Script Info]\nScriptType: v4.00+\nPlayResX: ${W}\nPlayResY: ${H}\nWrapStyle: 0\n\n[V4+ Styles]\nFormat: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n` +
  `Style: Caption,DejaVu Sans,42,&H00F4D6CD,&H00F4D6CD,&H002E1E1E,&H002E1E1E,0,0,0,0,100,100,0,0,1,0,0,2,180,180,22,1\n` +
  `Style: Label,DejaVu Sans Mono,26,&H00C8ADA6,&H00C8ADA6,&H002E1E1E,&H002E1E1E,0,0,0,0,100,100,0,0,1,0,0,2,160,160,126,1\n` +
  `Style: Watermark,DejaVu Sans,26,&H0000A5FF,&H0000A5FF,&H002E1E1E,&H002E1E1E,1,0,0,0,100,100,0,0,1,0,0,8,0,0,902,1\n\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n`;
const captionTimes = [];
for (const s of segs) {
  const n = byId[s.id];
  if (n.kind === "card") continue;
  if (labels[s.id]) ass += `Dialogue: 0,${ts(s.start)},${ts(s.end)},Label,,0,0,0,,${esc(labels[s.id])}\n`;
  const share = s.len / n.captions.length;
  n.captions.forEach((c, i) => {
    const a = s.start + i * share, b = s.start + (i + 1) * share;
    captionTimes.push({ scene: s.id, start: a, end: b, text: c });
    ass += `Dialogue: 0,${ts(a)},${ts(b)},Caption,,0,0,0,,${mono(esc(c))}\n`;
  });
}
if (rehearsal) ass += `Dialogue: 1,${ts(0)},${ts(total)},Watermark,,0,0,0,,REHEARSAL: SDK built from source ${esc(args.sha7 || "?")}, not a release\n`;
fs.writeFileSync(path.join(work, "captions.ass"), ass);
// an SRT of the same lines, for the voice unit and for review
fs.writeFileSync(path.join(OUT, "captions.srt"), captionTimes.map((c, i) => `${i + 1}\n${ts(c.start).replace(".", ",")}0 --> ${ts(c.end).replace(".", ",")}0\n${c.text}\n`).join("\n"));

// ---- join and final encode -------------------------------------------------
const list = path.join(work, "concat.txt");
fs.writeFileSync(list, parts.map((p) => `file '${p}'`).join("\n") + "\n");
ff(["-f", "concat", "-safe", "0", "-i", list, "-vf", `subtitles=${path.join(work, "captions.ass")}:fontsdir=/usr/share/fonts,format=yuv420p`, "-c:v", "libx264", "-preset", "medium", "-crf", "18", "-profile:v", "high", "-r", String(FPS), "-movflags", "+faststart", "-an", output]);

const info = ffprobe(output);
const v = info.streams.find((s) => s.codec_type === "video");
const sidecar = {
  format: { container: "mp4", codec: v.codec_name, profile: v.profile, pixFmt: v.pix_fmt, width: v.width, height: v.height, fps: v.r_frame_rate, durationSeconds: Number(info.format.duration), bytes: Number(info.format.size), audio: info.streams.some((s) => s.codec_type === "audio") },
  sha256: sha256(output),
  rehearsal,
  scenes: segs.map((s) => ({ id: s.id, kind: byId[s.id].kind, chapter: byId[s.id].chapter, start: Number(s.start.toFixed(2)), end: Number(s.end.toFixed(2)), source: s.still ? "card" : (s.src.endsWith(".webm") ? "browser" : "terminal"), sourceFrom: s.from, sourceTo: s.to, label: labels[s.id] || null })),
  terminalMarkers: { clears: clears.map((c) => c.start), loginCut: s08start, terminalDuration: termDur },
  browserTiming: timing.scenes,
  skippedSeconds: skipped,
  edits: ["hard cuts at scene boundaries", "last frame held where captions need reading time", ...Object.entries(skipped).map(([id, n]) => `${n} s of waiting removed before ${id} (labelled in the frame)`), "s02 is a still of s12's final frame", ...(rehearsal ? ["rehearsal watermark on every frame"] : [])],
  captions: { sha256: crypto.createHash("sha256").update(ass).digest("hex"), narrationSha256: sha256(path.join(SCENE, "narration.md")), count: captionTimes.length },
  chapters: (() => { const ch = []; for (const s of segs) { const c = byId[s.id].chapter; if (!ch.length || ch[ch.length - 1].title !== c) ch.push({ title: c, start: Number(s.start.toFixed(2)) }); } return ch; })(),
};
fs.writeFileSync(path.join(OUT, "recording.json"), JSON.stringify(sidecar, null, 2));
console.log(JSON.stringify({ result: "edited", mp4: output, durationSeconds: sidecar.format.durationSeconds, scenes: segs.length, skipped }));

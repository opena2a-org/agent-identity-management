// The browser thread of the quick-start walkthrough, recorded by Playwright
// inside docs/demo/browser. It follows the terminal thread through the
// transcript and the cue files the tape writes, performs every step the
// viewer would perform (approve the device code, sign in, authorize, verify
// the agent, read the activity and the violations), and writes its scene
// timestamps and a text snapshot of every scene for the review census.
//
// The administrator's password is read from the run's generated env file
// inside this process and typed into the login form only. Nothing here is
// traced or logged with fill values.
import fs from "node:fs";
import path from "node:path";
import { chromium } from "playwright";

const OUT = "/work/out";
const RUN_ENV = process.env.DEMO_RUN_ENV || "/work/run/.env";
const DASHBOARD = "http://localhost:3000";
const TIMEOUT = 300_000;

function readEnv(file) {
  const values = {};
  for (const line of fs.readFileSync(file, "utf8").split("\n")) {
    const m = line.match(/^([A-Z0-9_]+)=(.*)$/);
    if (m) values[m[1]] = m[2];
  }
  return values;
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function waitForFile(name, timeout = TIMEOUT) {
  const file = path.join(OUT, name);
  const deadline = Date.now() + timeout;
  while (!fs.existsSync(file)) {
    if (Date.now() > deadline) throw new Error(`timed out waiting for ${name}`);
    await sleep(500);
  }
  return fs.readFileSync(file, "utf8");
}

async function waitForTranscript(re, timeout = TIMEOUT) {
  const file = path.join(OUT, "transcript.txt");
  const deadline = Date.now() + timeout;
  for (;;) {
    if (fs.existsSync(file)) {
      const m = fs.readFileSync(file, "utf8").replace(/\x1b\[[0-9;?]*[A-Za-z]/g, "").match(re);
      if (m) return m;
    }
    if (Date.now() > deadline) throw new Error(`timed out waiting for ${re} in the transcript`);
    await sleep(500);
  }
}

const env = readEnv(RUN_ENV);
const email = env.ADMIN_EMAIL || "admin@example.com";
const password = env.ADMIN_PASSWORD;
if (!password) throw new Error("ADMIN_PASSWORD missing from the run env");

fs.mkdirSync(path.join(OUT, "browser"), { recursive: true });
fs.mkdirSync(path.join(OUT, "browser-text"), { recursive: true });
const browser = await chromium.launch();
// Playwright records the CSS viewport at its CSS size, so the viewport is the
// frame (1600x900) and the page is zoomed 1.25x so it lays out as it would at
// 1280x720: the same legibility as a device scale factor, without letterboxing.
const context = await browser.newContext({
  viewport: { width: 1600, height: 900 },
  colorScheme: "dark",
  recordVideo: { dir: path.join(OUT, "browser"), size: { width: 1600, height: 900 } },
});
await context.addInitScript(() => {
  const style = document.createElement("style");
  style.textContent = "html { zoom: 1.25; }";
  document.documentElement.appendChild(style);
});
const t0 = Date.now();
const page = await context.newPage();
const timing = { scenes: {}, cues: {}, t0 };
const now = () => (Date.now() - t0) / 1000;

function scene(id) {
  timing.scenes[id] = { start: now() };
  return async () => {
    timing.scenes[id].end = now();
    const text = await page.evaluate(() => document.body.innerText);
    fs.writeFileSync(path.join(OUT, "browser-text", `${id}.txt`), text);
  };
}

async function ring(locator) {
  // A 24-px ring at the click point for 400 ms, so the viewer sees where the click lands.
  const box = await locator.boundingBox();
  if (!box) return;
  const x = box.x + box.width / 2, y = box.y + box.height / 2;
  await page.evaluate(([x, y]) => {
    const d = document.createElement("div");
    d.style.cssText = `position:fixed;left:${x - 12}px;top:${y - 12}px;width:24px;height:24px;border:3px solid #f59e0b;border-radius:50%;pointer-events:none;z-index:2147483647;box-shadow:0 0 0 2px rgba(0,0,0,.35)`;
    document.body.appendChild(d);
    setTimeout(() => d.remove(), 400);
  }, [x, y]);
  await sleep(400);
}

async function click(locator) {
  await locator.waitFor({ state: "visible", timeout: 60_000 });
  await ring(locator);
  await locator.click();
}

function cue(name) {
  fs.writeFileSync(path.join(OUT, `cue-${name}`), String(Date.now() / 1000));
  timing.cues[name] = now();
}

try {
  // s05 · the device page shows the code the terminal printed
  const code = (await waitForTranscript(/Code:\s*([A-Z0-9]{4}-[A-Z0-9]{4})/))[1];
  timing.cues["code-seen"] = now();
  let end = scene("s05");
  await page.goto(`${DASHBOARD}/device?user_code=${code}`, { waitUntil: "networkidle" });
  await page.getByText(code, { exact: false }).first().waitFor({ timeout: 60_000 });
  await sleep(3000);
  await click(page.getByRole("button", { name: /sign in/i }).first());
  await end();

  // s06 · sign in as the administrator
  end = scene("s06");
  await page.waitForURL(/\/auth\/login/, { timeout: 60_000 });
  await sleep(800);
  const emailBox = page.getByLabel(/email/i).first();
  await emailBox.waitFor({ timeout: 60_000 });
  await ring(emailBox);
  await emailBox.click();
  await emailBox.pressSequentially(email, { delay: 60 });
  const passwordBox = page.getByLabel(/password/i).first();
  await ring(passwordBox);
  await passwordBox.click();
  await passwordBox.pressSequentially(password, { delay: 35 });
  await sleep(500);
  await click(page.getByRole("button", { name: /sign in|log in/i }).first());
  await end();

  // s07 · back on the device page: authorize
  end = scene("s07");
  await page.waitForURL(/\/device/, { timeout: 60_000 });
  await sleep(1500);
  await click(page.getByRole("button", { name: /authorize/i }).first());
  await page.getByText(/signed in/i).first().waitFor({ timeout: 60_000 });
  await sleep(3000);
  await end();
  cue("approved");

  // s10 · verify the agent
  await waitForFile("cue-registered");
  end = scene("s10");
  await page.goto(`${DASHBOARD}/dashboard/agents`, { waitUntil: "networkidle" });
  await sleep(2500);
  await click(page.getByRole("link", { name: /my-first-agent/ }).or(page.getByText("my-first-agent", { exact: true })).first());
  await page.waitForURL(/\/dashboard\/agents\/[^/]+/, { timeout: 60_000 });
  await sleep(2500);
  await click(page.getByRole("button", { name: /verify agent|verify/i }).first());
  await page.getByText(/^verified$/i).first().waitFor({ timeout: 60_000 });
  await sleep(3000);
  await end();
  cue("verified");

  // s13 · where the refusal is recorded
  await waitForFile("cue-denied");
  await sleep(2000);
  end = scene("s13");
  await page.reload({ waitUntil: "networkidle" });
  await sleep(1500);
  const activity = page.locator("section[aria-label='Recent activity']").first();
  if (await activity.count()) { await activity.scrollIntoViewIfNeeded(); }
  await sleep(6000);
  const security = page.getByRole("tab", { name: /security/i }).or(page.getByRole("button", { name: /security/i })).or(page.getByRole("link", { name: /security/i })).first();
  if (await security.count()) { await click(security); }
  const violations = page.locator("section[aria-label='Violations']").first();
  if (await violations.count()) { await violations.scrollIntoViewIfNeeded(); }
  await sleep(6000);
  await end();
} catch (err) {
  timing.error = String(err && err.message || err);
  try { await page.screenshot({ path: path.join(OUT, "browser", "failure.png") }); } catch {}
  console.error("walkthrough failed:", timing.error);
} finally {
  timing.end = now();
  await context.close();
  const video = fs.readdirSync(path.join(OUT, "browser")).find((f) => f.endsWith(".webm"));
  if (video) fs.renameSync(path.join(OUT, "browser", video), path.join(OUT, "browser", "browser.webm"));
  fs.writeFileSync(path.join(OUT, "browser", "timing.json"), JSON.stringify(timing, null, 2));
  await browser.close();
  console.log(JSON.stringify({ result: timing.error ? "failed" : "recorded", scenes: Object.keys(timing.scenes), seconds: timing.end }));
}
process.exit(timing.error ? 1 : 0);

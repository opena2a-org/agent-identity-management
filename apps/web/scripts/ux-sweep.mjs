#!/usr/bin/env node
/**
 * Readability sweep over every page route of the dashboard.
 *
 * For each `page.tsx` under `app/` (enumerated from the tree; a dynamic segment
 * such as `[id]` gets a fixed sample value) the route is loaded from a running
 * dashboard at 375x812 and 1280x800, in the light and the dark theme, as a
 * signed-in admin. Every `/api/v1/**` request is answered by the route mock in
 * this file, so no backend is needed. Per route and variant it records:
 *
 *   - the minimum text contrast ratio (WCAG 2.x relative luminance) with the
 *     background composited src-over up the ancestor chain, split into body
 *     text (needs 4.5:1) and large text (24px, or 18.66px bold; needs 3:1).
 *     A gradient background is scored against its worst colour stop; text over
 *     a url() image is reported as unmeasured rather than guessed
 *   - the count of visible text nodes rendered under 14px
 *   - the count of interactive targets whose box is under 44x44 CSS px
 *     (inline links inside running text are exempt, as in WCAG 2.5.8)
 *   - whether a <main> landmark exists
 *   - horizontal overflow of the document in CSS px
 *   - console errors and uncaught page errors, verbatim
 *
 * Usage:
 *   node apps/web/scripts/ux-sweep.mjs --base http://localhost:3000 --out result.json
 *
 * Exit code 1 when any route has body-text contrast under 4.5:1, large-text
 * contrast under 3:1, horizontal overflow, a console error, or could not be
 * loaded; 0 otherwise. The result file is written in both cases.
 */

import { chromium } from "@playwright/test";
import { promises as fs } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const DEFAULT_APP_DIR = path.resolve(HERE, "..", "app");

const VIEWPORTS = [
  { name: "375x812", width: 375, height: 812 },
  { name: "1280x800", width: 1280, height: 800 },
];
const THEMES = ["light", "dark"];
const BODY_MIN_RATIO = 4.5;
const LARGE_MIN_RATIO = 3;
const BODY_TEXT_FLOOR_PX = 14;
const HIT_AREA_FLOOR_PX = 44;
const SAMPLE_SEGMENT_VALUES = { id: "00000000-0000-4000-8000-000000000001" };
const SAMPLE_UUID = SAMPLE_SEGMENT_VALUES.id;

function parseArgs(argv) {
  const args = { base: "http://localhost:3000", out: null, app: DEFAULT_APP_DIR, settleMs: 1500 };
  for (let i = 0; i < argv.length; i += 1) {
    const a = argv[i];
    if (a === "--base") args.base = argv[++i];
    else if (a === "--out") args.out = argv[++i];
    else if (a === "--app") args.app = path.resolve(argv[++i]);
    else if (a === "--settle-ms") args.settleMs = Number(argv[++i]);
    else if (a === "--help" || a === "-h") {
      console.log("usage: ux-sweep.mjs --out <result.json> [--base <url>] [--app <app dir>] [--settle-ms <n>]");
      process.exit(0);
    } else throw new Error(`unknown argument: ${a}`);
  }
  if (!args.out) throw new Error("--out <result.json> is required");
  return args;
}

/** Every page.tsx under the app dir, as a URL path. Route groups `(name)` are dropped. */
async function enumerateRoutes(appDir) {
  const files = [];
  async function walk(dir) {
    for (const entry of await fs.readdir(dir, { withFileTypes: true })) {
      const full = path.join(dir, entry.name);
      if (entry.isDirectory()) await walk(full);
      else if (entry.name === "page.tsx") files.push(full);
    }
  }
  await walk(appDir);
  return files
    .map((file) => {
      const rel = path.relative(appDir, path.dirname(file));
      const segments = rel
        .split(path.sep)
        .filter((s) => s && !/^\(.*\)$/.test(s))
        .map((s) => {
          const m = /^\[(?:\.\.\.)?(.+)\]$/.exec(s);
          if (!m) return s;
          return SAMPLE_SEGMENT_VALUES[m[1]] ?? SAMPLE_UUID;
        });
      return { file: path.relative(path.resolve(appDir, ".."), file), route: `/${segments.join("/")}`.replace(/\/+$/, "") || "/" };
    })
    .sort((a, b) => a.route.localeCompare(b.route));
}

/** Unsigned session token: the dashboard only decodes the payload (lib/jwt-payload.ts); the API that would verify it is mocked. */
function sessionToken() {
  const b64url = (o) => Buffer.from(JSON.stringify(o)).toString("base64url");
  const now = Math.floor(Date.now() / 1000);
  return `${b64url({ alg: "none", typ: "JWT" })}.${b64url({
    user_id: SAMPLE_UUID,
    organization_id: SAMPLE_UUID,
    email: "admin@example.com",
    role: "admin",
    iat: now,
    exp: now + 8 * 3600,
  })}.`;
}

const NOW = new Date().toISOString();
const AGENT = {
  id: SAMPLE_UUID,
  organizationId: SAMPLE_UUID,
  name: "sample-agent",
  displayName: "Sample agent",
  description: "Fixture agent for the readability sweep",
  agentType: "claude",
  status: "verified",
  version: "1.0.0",
  trustScore: 0.82,
  talksTo: [],
  capabilities: [],
  metadata: {},
  createdAt: NOW,
  updatedAt: NOW,
};
const MCP_SERVER = {
  id: SAMPLE_UUID,
  organizationId: SAMPLE_UUID,
  name: "sample-mcp",
  url: "https://mcp.example.com",
  description: "Fixture MCP server for the readability sweep",
  status: "verified",
  isVerified: true,
  lastVerifiedAt: NOW,
  createdAt: NOW,
  updatedAt: NOW,
};
const TRUST_FACTORS = { verificationStatus: 1, uptime: 0.9, successRate: 0.9, securityAlerts: 1, compliance: 0.8, age: 0.5, driftDetection: 1, userFeedback: 0.5, executionIsolation: 0.7 };
const TRUST_WEIGHTS = { verificationStatus: 0.2, uptime: 0.1, successRate: 0.15, securityAlerts: 0.15, compliance: 0.1, age: 0.05, driftDetection: 0.1, userFeedback: 0.05, executionIsolation: 0.1 };
const COUNTS = { allCount: 0, acknowledgedCount: 0, unacknowledgedCount: 0, criticalCount: 0, highCount: 0, mediumCount: 0, lowAndInfoCount: 0 };

/** Route mock: [method, path regex, body]. First match wins; an unmatched request gets `{}` and is recorded. */
const MOCKS = [
  ["GET", /^\/api\/v1\/auth\/me$/, { id: SAMPLE_UUID, organizationId: SAMPLE_UUID, organizationName: "Sample org", email: "admin@example.com", name: "Sample Admin", role: "admin", status: "active", createdAt: NOW }],
  ["GET", /^\/api\/v1\/organizations\/current$/, { id: SAMPLE_UUID, name: "Sample org", maxAgents: 100, isActive: true, createdAt: NOW, updatedAt: NOW }],
  ["GET", /^\/api\/v1\/agents$/, { agents: [AGENT] }],
  ["GET", /^\/api\/v1\/agents\/[^/]+$/, AGENT],
  ["GET", /^\/api\/v1\/agents\/[^/]+\/alerts/, { agentId: SAMPLE_UUID, alerts: [], total: 0 }],
  ["GET", /^\/api\/v1\/agents\/[^/]+\/mcp-servers$/, { mcpServers: [], total: 0 }],
  ["GET", /^\/api\/v1\/agents\/[^/]+\/trust-score\/history$/, { agentId: SAMPLE_UUID, history: [] }],
  ["GET", /^\/api\/v1\/agents\/[^/]+\/trust-score$/, { agentId: SAMPLE_UUID, trustScore: 0.82, calculatedAt: NOW }],
  ["GET", /^\/api\/v1\/trust-score\/agents\/[^/]+\/breakdown$/, { agentId: SAMPLE_UUID, agentName: AGENT.displayName, overall: 0.82, factors: { ...TRUST_FACTORS }, weights: { ...TRUST_WEIGHTS }, contributions: { ...TRUST_FACTORS }, confidence: 0.9, calculatedAt: NOW }],
  ["GET", /^\/api\/v1\/trust-score\/agents\/[^/]+$/, { agentId: SAMPLE_UUID, trustScore: 0.82, calculatedAt: NOW }],
  ["GET", /^\/api\/v1\/verification-events\/statistics/, { totalVerifications: 0, successCount: 0, failedCount: 0, pendingCount: 0, timeoutCount: 0, successRate: 0, avgDurationMs: 0, avgConfidence: 0, avgTrustScore: 0, verificationsPerMinute: 0, uniqueAgentsVerified: 0, protocolDistribution: {}, typeDistribution: {}, initiatorDistribution: {} }],
  ["GET", /^\/api\/v1\/verification-events\/recent/, { events: [], total: 0 }],
  ["POST", /^\/api\/v1\/compliance\/check$/, { checkType: "all", passed: 0, failed: 0, total: 0, complianceRate: 100, checks: [] }],
  ["GET", /^\/api\/v1\/agents\/[^/]+\/activity/, { activities: [], total: 0 }],
  ["GET", /^\/api\/v1\/agents\/[^/]+\/capabilities/, []],
  ["GET", /^\/api\/v1\/capabilities$/, { capabilities: [], reservedNamespaces: [], validationPattern: "^[a-z]+:[a-z_]+$" }],
  ["GET", /^\/api\/v1\/agents\/[^/]+\/tags$/, { tags: [] }],
  ["GET", /^\/api\/v1\/detection\/agents\/[^/]+\/status$/, { agentId: SAMPLE_UUID, sdkInstalled: false, autoDetectEnabled: false, detectedMCPs: [] }],
  ["GET", /^\/api\/v1\/detection\/agents\/[^/]+\/capabilities\/latest$/, { agentId: SAMPLE_UUID, capabilities: [] }],
  ["GET", /^\/api\/v1\/analytics\/dashboard$/, { totalAgents: 1, verifiedAgents: 1, pendingAgents: 0, verificationRate: 100, avgTrustScore: 82, totalMcpServers: 0, activeMcpServers: 0, totalUsers: 1, activeUsers: 1, activeAlerts: 0, criticalAlerts: 0, securityIncidents: 0, totalVerifications: 0, successfulVerifications: 0, failedVerifications: 0, avgResponseTime: 0, organizationId: SAMPLE_UUID }],
  ["GET", /^\/api\/v1\/analytics\/verification-activity/, { period: "6 months", activity: [], currentStats: { totalVerified: 1, totalPending: 0, agentsVerified: 1, agentsPending: 0, mcpVerified: 0, mcpPending: 0 } }],
  ["GET", /^\/api\/v1\/analytics\/activity/, { period: { startDate: NOW, endDate: NOW, days: 30 }, summary: { totalAgents: 1, totalMcpServers: 0, verificationCount: 0, attestationCount: 0, totalActivityEvents: 0 }, activityByDay: [], topAgents: [], topActions: [] }],
  ["GET", /^\/api\/v1\/analytics\/agents\/activity/, { activities: [], summary: { totalActivities: 0, successCount: 0, failureCount: 0, successRate: 0 } }],
  ["GET", /^\/api\/v1\/analytics\/trends/, { trends: [], distribution: { excellent: 0, good: 0, fair: 0, poor: 0 }, summary: { overallAvg: 0, trend: "stable", changePercentage: 0 } }],
  ["GET", /^\/api\/v1\/analytics\//, { data: [], total: 0 }],
  ["GET", /^\/api\/v1\/security\/metrics$/, { securityScore: 90, securityGrade: "A", securityStatus: "healthy", lastIncidentAt: NOW, actionsBlocked: 0, actionsBlockedToday: 0, agentsMonitored: 1, agentsTrusted: 1, trustPercentage: 100, actionsToday: 0, requiresAttention: 0, averageTrustScore: 82, mcpServersTotal: 0, mcpServersVerified: 0, mcpTrustPercentage: 0, totalThreats: 0, activeThreats: 0, blockedThreats: 0, totalAnomalies: 0, highSeverityCount: 0, openIncidents: 0, protectionTimeline: [], riskByCategory: [] }],
  ["GET", /^\/api\/v1\/security\/violations/, { violations: [], total: 0, limit: 50, offset: 0 }],
  ["GET", /^\/api\/v1\/security\/threats/, { threats: [], total: 0 }],
  ["GET", /^\/api\/v1\/compliance\/status$/, { complianceLevel: "high", totalAgents: 1, verifiedAgents: 1, verificationRate: 100, averageTrustScore: 82, recentAuditCount: 0 }],
  ["GET", /^\/api\/v1\/compliance\/metrics$/, { startDate: NOW, endDate: NOW, interval: "day", metrics: { period: { start: NOW, end: NOW, interval: "day" }, agentVerificationTrend: [], trustScoreTrend: [], auditActivityTrend: [], complianceScoreTrend: [] } }],
  ["GET", /^\/api\/v1\/compliance\/access-review$/, { users: [], total: 0 }],
  ["GET", /^\/api\/v1\/compliance\//, { violations: [], evidence: [], data: [], total: 0 }],
  ["GET", /^\/api\/v1\/admin\/alerts/, { alerts: [], total: 0, ...COUNTS }],
  ["GET", /^\/api\/v1\/admin\/capability-requests/, []],
  ["GET", /^\/api\/v1\/admin\/enforcement-settings$/, { enforcementMode: "monitoring", description: "Monitoring mode", explanation: "Requests are recorded and allowed.", impact: "No request is blocked." }],
  ["GET", /^\/api\/v1\/admin\/security-policies$/, []],
  ["GET", /^\/api\/v1\/admin\/users/, { users: [], total: 0 }],
  ["GET", /^\/api\/v1\/admin\/audit-logs/, { logs: [], total: 0 }],
  ["GET", /^\/api\/v1\/admin\/registration-requests/, { requests: [], total: 0 }],
  ["GET", /^\/api\/v1\/admin\/verifications\/pending/, { verifications: [], pagination: { page: 1, pageSize: 20, total: 0, totalPages: 0 }, statusCounts: { pending: 0, approved: 0, denied: 0, expired: 0 } }],
  ["GET", /^\/api\/v1\/admin\/dashboard\/stats$/, { totalUsers: 1, activeUsers: 1, pendingUsers: 0 }],
  ["GET", /^\/api\/v1\/api-keys/, { apiKeys: [] }],
  ["GET", /^\/api\/v1\/webhooks$/, { webhooks: [] }],
  ["GET", /^\/api\/v1\/tags/, []],
  ["GET", /^\/api\/v1\/users\/me\/sdk-tokens\/count$/, { count: 0 }],
  ["GET", /^\/api\/v1\/users\/me\/sdk-tokens/, { tokens: [] }],
  ["GET", /^\/api\/v1\/mcp-servers\/discovered$/, { discovered: [], totalUnmapped: 0, totalAgents: 1 }],
  ["GET", /^\/api\/v1\/mcp-servers\/graph$/, { nodes: [], edges: [] }],
  ["GET", /^\/api\/v1\/mcp-servers$/, { mcpServers: [], total: 0 }],
  ["GET", /^\/api\/v1\/mcp-servers\/[^/]+$/, MCP_SERVER],
  ["GET", /^\/api\/v1\/mcp-servers\/[^/]+\/agents$/, { agents: [], total: 0 }],
  ["GET", /^\/api\/v1\/mcp-servers\/[^/]+\/attestations$/, { attestations: [], total: 0, confidenceScore: 0, lastAttestedAt: NOW }],
  ["GET", /^\/api\/v1\/mcp-servers\/[^/]+\/audit-logs/, { logs: [], total: 0 }],
  ["GET", /^\/api\/v1\/mcp-servers\/[^/]+\/capabilities$/, { capabilities: [], total: 0 }],
  ["GET", /^\/api\/v1\/mcp-servers\/[^/]+\/connections$/, { connections: [], total: 0 }],
  ["GET", /^\/api\/v1\/supply-chain\/analytics/, { stats: { totalConnections: 0, activeConnections: 0, totalAttestations: 0, attestationsLast24h: 0, uniqueAgents: 0, uniqueMCPServers: 0 }, attestationTrend: [], connections: [], topMCPServers: [], topAgents: [] }],
  ["GET", /^\/api\/v1\/supply-chain\/drift-alerts/, { stats: { totalAlerts: 0, addedCapabilities: 0, removedCapabilities: 0, highSeverity: 0, mediumSeverity: 0, lowSeverity: 0, unacknowledgedCount: 0 }, alerts: [] }],
  ["GET", /^\/api\/v1\/a2a\/cards/, { cards: [], limit: 100, offset: 0, total: 0 }],
  ["GET", /^\/api\/v1\/a2a\/tasks/, { tasks: [], total: 0 }],
  ["GET", /^\/api\/v1\/a2a\/consents/, { consents: [], total: 0 }],
  ["GET", /^\/api\/v1\/a2a\/consent\//, { consents: [], count: 0 }],
  ["GET", /^\/api\/v1\/a2a\/trust-scores/, { scores: [], total: 0 }],
  ["GET", /^\/api\/v1\/a2a\/skills\/search/, { skills: [], total: 0 }],
  ["GET", /^\/api\/v1\/oauth\/device\/verify/, { userCode: "ABCD-EFGH", clientId: "aim-sdk", scope: "agent:register", status: "pending", expiresAt: NOW }],
  ["POST", /^\/api\/v1\/auth\/refresh$/, { token: sessionToken(), refreshToken: "refresh" }],
];

function findMock(method, pathname) {
  for (const [m, re, body] of MOCKS) if (m === method && re.test(pathname)) return body;
  return null;
}

/** Runs inside the page. Returns the per-variant measurement. */
function measureInPage({ bodyMin, largeMin, textFloor, hitFloor }) {
  const canvas = document.createElement("canvas");
  canvas.width = canvas.height = 1;
  const ctx = canvas.getContext("2d", { willReadFrequently: true });
  const colorCache = new Map();
  function parseColor(str) {
    if (!str) return [0, 0, 0, 0];
    if (colorCache.has(str)) return colorCache.get(str);
    let out;
    const m = /^rgba?\(\s*([\d.]+)[,\s]+([\d.]+)[,\s]+([\d.]+)(?:\s*[,/]\s*([\d.]+%?))?\s*\)$/.exec(str);
    if (m) {
      let a = m[4] === undefined ? 1 : m[4].endsWith("%") ? parseFloat(m[4]) / 100 : parseFloat(m[4]);
      out = [+m[1], +m[2], +m[3], a];
    } else if (str === "transparent") {
      out = [0, 0, 0, 0];
    } else {
      ctx.clearRect(0, 0, 1, 1);
      ctx.fillStyle = "#000";
      ctx.fillStyle = str;
      ctx.fillRect(0, 0, 1, 1);
      const d = ctx.getImageData(0, 0, 1, 1).data;
      out = [d[0], d[1], d[2], d[3] / 255];
    }
    colorCache.set(str, out);
    return out;
  }
  function over(top, bottom) {
    const a = top[3];
    if (a >= 1) return [top[0], top[1], top[2], 1];
    if (a <= 0) return bottom;
    return [
      top[0] * a + bottom[0] * (1 - a),
      top[1] * a + bottom[1] * (1 - a),
      top[2] * a + bottom[2] * (1 - a),
      1,
    ];
  }
  function luminance([r, g, b]) {
    const f = (c) => {
      c /= 255;
      return c <= 0.03928 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4);
    };
    return 0.2126 * f(r) + 0.7152 * f(g) + 0.0722 * f(b);
  }
  function ratio(fg, bg) {
    const l1 = luminance(fg);
    const l2 = luminance(bg);
    return (Math.max(l1, l2) + 0.05) / (Math.min(l1, l2) + 0.05);
  }
  /**
   * Background candidates for an element: every ancestor's background-color is
   * composited src-over from the document (white) down to the element. A gradient
   * background contributes each of its colour stops as a candidate, so the ratio
   * is taken against the worst stop; an opaque layer closer to the element resets
   * the candidates. A url() image cannot be reduced to a colour, so the node is
   * reported as unmeasured instead of guessed.
   */
  function compositedBackgrounds(el) {
    const chain = [];
    for (let n = el; n && n.nodeType === 1; n = n.parentElement) chain.unshift(n);
    let candidates = [[255, 255, 255, 1]];
    let note = null;
    for (const n of chain) {
      const cs = getComputedStyle(n);
      const c = parseColor(cs.backgroundColor);
      if (c[3] >= 1) { candidates = [[c[0], c[1], c[2], 1]]; note = null; }
      else if (c[3] > 0) candidates = candidates.map((b) => over(c, b));
      const img = cs.backgroundImage;
      if (img && img !== "none") {
        if (/url\(/.test(img)) note = "image";
        else if (/gradient\(/.test(img)) {
          const stops = [...img.matchAll(/rgba?\([^)]*\)|#[0-9a-f]{3,8}\b|\b(?:transparent|white|black)\b/gi)].map((m) => parseColor(m[0])).filter((c) => c[3] > 0);
          if (stops.length) {
            const next = [];
            for (const b of candidates) for (const st of stops) next.push(over(st, b));
            const seen = new Set();
            candidates = next.filter((b) => { const k = b.slice(0, 3).map(Math.round).join(","); if (seen.has(k)) return false; seen.add(k); return true; }).slice(0, 32);
            if (note !== "image") note = "gradient";
          }
        }
      }
    }
    return { candidates, note };
  }
  function describe(el) {
    const parts = [];
    for (let n = el, i = 0; n && n.nodeType === 1 && i < 4; n = n.parentElement, i += 1) {
      let s = n.tagName.toLowerCase();
      if (n.id) s += `#${n.id}`;
      else if (typeof n.className === "string" && n.className.trim()) s += "." + n.className.trim().split(/\s+/).slice(0, 2).join(".");
      parts.unshift(s);
    }
    return parts.join(" > ");
  }
  function isRendered(el) {
    for (let n = el; n && n.nodeType === 1; n = n.parentElement) {
      const cs = getComputedStyle(n);
      if (cs.display === "none" || cs.visibility === "hidden") return false;
    }
    return true;
  }
  const SKIP = new Set(["SCRIPT", "STYLE", "NOSCRIPT", "TEMPLATE", "TITLE", "SVG", "OPTION"]);

  const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
  let textNodes = 0;
  let unmeasuredTextNodes = 0;
  const unmeasuredSamples = [];
  let smallTextNodes = 0;
  const smallTextSamples = [];
  let minBody = Infinity;
  let minLarge = Infinity;
  let worstBody = null;
  let worstLarge = null;
  const belowFloor = [];
  for (let node = walker.nextNode(); node; node = walker.nextNode()) {
    const text = node.nodeValue.replace(/\s+/g, " ").trim();
    if (!text) continue;
    const el = node.parentElement;
    if (!el || SKIP.has(el.tagName) || el.closest("svg")) continue;
    if (!isRendered(el)) continue;
    const range = document.createRange();
    range.selectNodeContents(node);
    const rect = range.getBoundingClientRect();
    if (rect.width === 0 || rect.height === 0) continue;
    const cs = getComputedStyle(el);
    const size = parseFloat(cs.fontSize);
    const weight = parseInt(cs.fontWeight, 10) || (cs.fontWeight === "bold" ? 700 : 400);
    const large = size >= 24 || (size >= 18.66 && weight >= 700);
    textNodes += 1;
    if (size < textFloor) {
      smallTextNodes += 1;
      if (smallTextSamples.length < 5) smallTextSamples.push({ text: text.slice(0, 60), fontSizePx: size, where: describe(el) });
    }
    const { candidates, note } = compositedBackgrounds(el);
    if (note === "image") {
      unmeasuredTextNodes += 1;
      if (unmeasuredSamples.length < 5) unmeasuredSamples.push({ text: text.slice(0, 60), reason: "background-image url()", where: describe(el) });
      continue;
    }
    const fgColor = parseColor(cs.color);
    let r = Infinity;
    let bg = candidates[0];
    let fg = fgColor;
    for (const b of candidates) {
      const f = over(fgColor, b);
      const rr = ratio(f, b);
      if (rr < r) { r = rr; bg = b; fg = f; }
    }
    const sample = { text: text.slice(0, 60), ratio: +r.toFixed(2), fontSizePx: size, fontWeight: weight, large, fg: fg.slice(0, 3).map(Math.round), bg: bg.slice(0, 3).map(Math.round), bgNote: note, where: describe(el) };
    if (large) {
      if (r < minLarge) { minLarge = r; worstLarge = sample; }
      if (r < largeMin && belowFloor.length < 25) belowFloor.push(sample);
    } else {
      if (r < minBody) { minBody = r; worstBody = sample; }
      if (r < bodyMin && belowFloor.length < 25) belowFloor.push(sample);
    }
  }

  const TARGETS = 'a[href], button, input:not([type="hidden"]), select, textarea, summary, [role="button"], [role="link"], [role="tab"], [role="menuitem"], [role="checkbox"], [role="radio"], [role="switch"], [tabindex]:not([tabindex="-1"])';
  let interactiveTargets = 0;
  let smallTargets = 0;
  const smallTargetSamples = [];
  for (const el of document.querySelectorAll(TARGETS)) {
    if (SKIP.has(el.tagName) || !isRendered(el)) continue;
    const rect = el.getBoundingClientRect();
    if (rect.width === 0 || rect.height === 0) continue;
    const cs = getComputedStyle(el);
    if (el.tagName === "A" && cs.display === "inline") {
      const parentText = (el.parentElement?.textContent ?? "").trim();
      if (parentText.length > (el.textContent ?? "").trim().length) continue; // link inside running text
    }
    interactiveTargets += 1;
    if (rect.width < hitFloor || rect.height < hitFloor) {
      smallTargets += 1;
      if (smallTargetSamples.length < 5) smallTargetSamples.push({ text: (el.getAttribute("aria-label") || el.textContent || el.getAttribute("placeholder") || "").replace(/\s+/g, " ").trim().slice(0, 40), widthPx: +rect.width.toFixed(1), heightPx: +rect.height.toFixed(1), where: describe(el) });
    }
  }

  const doc = document.documentElement;
  const overflowPx = Math.max(0, doc.scrollWidth - doc.clientWidth, document.body.scrollWidth - doc.clientWidth);
  return {
    themeClassDark: doc.classList.contains("dark"),
    hasMain: !!document.querySelector('main, [role="main"]'),
    overflowPx,
    textNodes,
    unmeasuredTextNodes,
    unmeasuredSamples,
    smallTextNodes,
    smallTextSamples,
    interactiveTargets,
    smallTargets,
    smallTargetSamples,
    minBodyContrast: Number.isFinite(minBody) ? +minBody.toFixed(2) : null,
    minLargeContrast: Number.isFinite(minLarge) ? +minLarge.toFixed(2) : null,
    worstBody,
    worstLarge,
    belowFloor,
    title: document.title,
  };
}

async function sweepRoute(context, base, entry, settleMs) {
  const page = await context.newPage();
  const consoleErrors = [];
  const unmocked = new Set();
  page.on("console", (msg) => {
    if (msg.type() === "error") consoleErrors.push(msg.text().slice(0, 500));
  });
  page.on("pageerror", (err) => consoleErrors.push(`pageerror: ${String(err.message || err).slice(0, 500)}`));
  await page.route("**/api/v1/**", async (route) => {
    const req = route.request();
    const url = new URL(req.url());
    const body = findMock(req.method(), url.pathname);
    if (body === null) unmocked.add(`${req.method()} ${url.pathname}`);
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(body ?? {}) });
  });
  const result = { route: entry.route, file: entry.file, requestedUrl: base + entry.route, finalUrl: null, redirected: false, loadError: null };
  try {
    await page.goto(base + entry.route, { waitUntil: "networkidle", timeout: 45_000 });
    await page.waitForTimeout(settleMs);
    try { await page.waitForLoadState("networkidle", { timeout: 10_000 }); } catch { /* keep going with what rendered */ }
    result.finalUrl = page.url();
    result.redirected = new URL(result.finalUrl).pathname !== entry.route;
    Object.assign(result, await page.evaluate(measureInPage, { bodyMin: BODY_MIN_RATIO, largeMin: LARGE_MIN_RATIO, textFloor: BODY_TEXT_FLOOR_PX, hitFloor: HIT_AREA_FLOOR_PX }));
  } catch (err) {
    result.loadError = String(err.message || err).slice(0, 500);
  }
  result.consoleErrors = consoleErrors;
  result.unmockedRequests = [...unmocked].sort();
  await page.close();
  return result;
}

function summarize(variants, routes) {
  const byRoute = new Map();
  for (const v of variants) {
    const s = byRoute.get(v.route) ?? { route: v.route, file: v.file, minBodyContrast: null, minLargeContrast: null, smallTextNodes: 0, smallTargets: 0, hasMain: true, overflowPx: 0, consoleErrors: 0, loadErrors: 0, redirectedTo: null, variants: 0 };
    s.variants += 1;
    if (v.loadError) s.loadErrors += 1;
    if (v.minBodyContrast !== null && v.minBodyContrast !== undefined && (s.minBodyContrast === null || v.minBodyContrast < s.minBodyContrast)) s.minBodyContrast = v.minBodyContrast;
    if (v.minLargeContrast !== null && v.minLargeContrast !== undefined && (s.minLargeContrast === null || v.minLargeContrast < s.minLargeContrast)) s.minLargeContrast = v.minLargeContrast;
    s.smallTextNodes = Math.max(s.smallTextNodes, v.smallTextNodes ?? 0);
    s.smallTargets = Math.max(s.smallTargets, v.smallTargets ?? 0);
    if (v.hasMain === false) s.hasMain = false;
    s.overflowPx = Math.max(s.overflowPx, v.overflowPx ?? 0);
    s.consoleErrors += v.consoleErrors.length;
    if (v.redirected) s.redirectedTo = new URL(v.finalUrl).pathname;
    byRoute.set(v.route, s);
  }
  const rows = routes.map((r) => byRoute.get(r.route));
  const fails = {
    bodyContrast: rows.filter((r) => r.minBodyContrast !== null && r.minBodyContrast < BODY_MIN_RATIO).map((r) => r.route),
    largeContrast: rows.filter((r) => r.minLargeContrast !== null && r.minLargeContrast < LARGE_MIN_RATIO).map((r) => r.route),
    overflow: rows.filter((r) => r.overflowPx > 0).map((r) => r.route),
    consoleErrors: rows.filter((r) => r.consoleErrors > 0).map((r) => r.route),
    loadErrors: rows.filter((r) => r.loadErrors > 0).map((r) => r.route),
    smallText: rows.filter((r) => r.smallTextNodes > 0).map((r) => r.route),
    smallTargets: rows.filter((r) => r.smallTargets > 0).map((r) => r.route),
    noMain: rows.filter((r) => !r.hasMain).map((r) => r.route),
  };
  return { rows, fails };
}

async function main() {
  const args = parseArgs(process.argv.slice(2));
  const routes = await enumerateRoutes(args.app);
  const browser = await chromium.launch();
  const variants = [];
  const startedAt = new Date().toISOString();
  for (const viewport of VIEWPORTS) {
    for (const theme of THEMES) {
      const context = await browser.newContext({ viewport: { width: viewport.width, height: viewport.height }, colorScheme: theme });
      await context.addInitScript(({ token, theme, now }) => {
        window.__RUNTIME_CONFIG__ = { apiUrl: window.location.origin };
        localStorage.setItem("auth_token", token);
        localStorage.setItem("session_start", now);
        localStorage.setItem("last_activity", now);
        localStorage.setItem("theme", theme);
      }, { token: sessionToken(), theme, now: String(Date.now()) });
      for (const entry of routes) {
        const r = await sweepRoute(context, args.base, entry, args.settleMs);
        variants.push({ viewport: viewport.name, theme, ...r });
        const flag = r.loadError ? "LOAD-ERROR" : `body ${r.minBodyContrast ?? "-"} large ${r.minLargeContrast ?? "-"} small-text ${r.smallTextNodes} small-targets ${r.smallTargets} main ${r.hasMain} overflow ${r.overflowPx} errors ${r.consoleErrors.length}`;
        console.log(`${viewport.name} ${theme.padEnd(5)} ${entry.route.padEnd(44)} ${flag}`);
      }
      await context.close();
    }
  }
  await browser.close();
  const { rows, fails } = summarize(variants, routes);
  const gateFailed = fails.bodyContrast.length + fails.largeContrast.length + fails.overflow.length + fails.consoleErrors.length + fails.loadErrors.length > 0;
  const result = {
    instrument: path.relative(process.cwd(), fileURLToPath(import.meta.url)),
    startedAt,
    finishedAt: new Date().toISOString(),
    base: args.base,
    appDir: args.app,
    thresholds: { bodyContrast: BODY_MIN_RATIO, largeContrast: LARGE_MIN_RATIO, textFloorPx: BODY_TEXT_FLOOR_PX, hitAreaFloorPx: HIT_AREA_FLOOR_PX },
    viewports: VIEWPORTS.map((v) => v.name),
    themes: THEMES,
    routeCount: routes.length,
    variantCount: variants.length,
    gate: { passed: !gateFailed, failing: fails },
    routes: rows,
    variants,
  };
  await fs.mkdir(path.dirname(path.resolve(args.out)), { recursive: true });
  await fs.writeFile(args.out, JSON.stringify(result, null, 2) + "\n");
  console.log(`\n${routes.length} routes x ${variants.length / routes.length} variants -> ${args.out}`);
  for (const [k, v] of Object.entries(fails)) console.log(`  ${k.padEnd(14)} ${v.length} route(s)${v.length ? ": " + v.join(", ") : ""}`);
  console.log(gateFailed ? "GATE: FAIL" : "GATE: PASS");
  process.exit(gateFailed ? 1 : 0);
}

main().catch((err) => {
  console.error(err);
  process.exit(2);
});

import { describe, expect, it } from "vitest";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";

// A browser alert carries no link or action, ignores the theme, and has shown the
// server's raw error text. Outcomes render inline through components/ui/action-outcome.
// This counts the lines in app/ that call one, the same lines as
//   git grep -n -E 'alert\(' -- apps/web/app ':!*.fmt' ':!*.final' ':!*.bkp' ':!*.bak'
// and fails when the count rises. Lower CEILING in the change that converts a page.
const CEILING = 16;

const root = join(__dirname, "..");
const SKIPPED_SUFFIXES = [".fmt", ".final", ".bkp", ".bak"];
const BROWSER_ALERT = /alert\(/;

function files(dir: string, out: string[] = []): string[] {
  for (const entry of readdirSync(join(root, dir))) {
    const rel = join(dir, entry);
    if (statSync(join(root, rel)).isDirectory()) {
      files(rel, out);
    } else {
      out.push(rel);
    }
  }
  return out;
}

function browserAlertLines(sources: { path: string; text: string }[]): string[] {
  return sources
    .filter(({ path }) => !SKIPPED_SUFFIXES.some((suffix) => path.endsWith(suffix)))
    .flatMap(({ path, text }) =>
      text
        .split("\n")
        .map((line, i) => (BROWSER_ALERT.test(line) ? `${path}:${i + 1}: ${line.trim()}` : null))
        .filter((hit): hit is string => hit !== null)
    );
}

const appLines = browserAlertLines(
  files("app").map((path) => ({ path, text: readFileSync(join(root, path), "utf8") }))
);

describe("no browser alerts in the dashboard", () => {
  it("counts a planted call, and skips the stale copies the git command skips", () => {
    const planted = 'export function f() {\n  window.alert("done");\n  alert(`failed`);\n}\n';
    expect(browserAlertLines([{ path: "app/x/page.tsx", text: planted }])).toEqual([
      'app/x/page.tsx:2: window.alert("done");',
      "app/x/page.tsx:3: alert(`failed`);",
    ]);
    expect(browserAlertLines([{ path: "app/x/page.tsx.bak", text: planted }])).toEqual([]);
    expect(browserAlertLines([{ path: "app/x/page.tsx", text: 'toast.error("failed");\n' }])).toEqual([]);
  });

  it(`has no more than ${CEILING} lines in app/ that open one`, () => {
    expect(appLines.length, appLines.join("\n")).toBeLessThanOrEqual(CEILING);
  });

  it("has none on the agent page", () => {
    const page = `${join("app", "dashboard", "agents", "[id]", "page.tsx")}:`;
    expect(appLines.filter((line) => line.startsWith(page))).toEqual([]);
  });
});

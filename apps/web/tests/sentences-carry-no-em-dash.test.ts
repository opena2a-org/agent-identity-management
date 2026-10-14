import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { join } from "node:path";

// A sentence the dashboard shows is punctuated with a full stop, a colon or a
// comma, never an em dash. The one em dash these screens may hold is the
// placeholder the drift score card prints in place of a value it has no
// number for.

const root = join(__dirname, "..");

const SCREENS = [
  "components/overview/live-checkin-panel.tsx",
  "components/agent/drift-score-card.tsx",
  "components/agent/trust-score-breakdown.tsx",
];

const EM_DASH = "—";
const PLACEHOLDER = `'${EM_DASH}'`;

function emDashLines(source: string): string[] {
  return source
    .split("\n")
    .map((line, i) => ({ line, n: i + 1 }))
    .filter(({ line }) => line.includes(EM_DASH) || line.includes("\\u2014"))
    .filter(({ line }) => !line.includes(PLACEHOLDER))
    .map(({ line, n }) => `${n}: ${line.trim()}`);
}

describe("dashboard sentences", () => {
  it("carry no em dash outside the empty-value placeholder", () => {
    const offenders = SCREENS.flatMap((rel) =>
      emDashLines(readFileSync(join(root, rel), "utf8")).map((hit) => `${rel}:${hit}`),
    );
    expect(offenders).toEqual([]);
  });

  it("still lets the drift score card print its placeholder", () => {
    expect(emDashLines(`const displayValue = baseline ? ${PLACEHOLDER} : \`\${driftPct}%\`;`)).toEqual([]);
    expect(emDashLines(`Behavioral drift signal ${EM_DASH} runtime`)).toHaveLength(1);
    expect(emDashLines(`"Token rotated \\u2014 revoked"`)).toHaveLength(1);
  });
});

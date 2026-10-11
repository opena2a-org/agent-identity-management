import { describe, expect, it } from "vitest";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import { chartPlacements, offGlass, paintOf, type GlassComponent } from "./chart-surface";

// The chart standard's contrast floors are measured against the glass card
// (tests/chart-contrast.test.ts). This holds every chart to that card: a chart
// on a card painted any other way has text and marks nobody measured, which is
// how an axis label reached 4.05:1 on a dark:bg-gray-800 card.

const root = join(__dirname, "..");
const CHARTS_DIR = "components/charts";

// components/ui/card.tsx renders the glass card.
const GLASS_COMPONENTS: GlassComponent[] = [{ module: "components/ui/card", name: "Card" }];

function files(dir: string, out: string[] = []): string[] {
  for (const entry of readdirSync(join(root, dir))) {
    const rel = join(dir, entry);
    if (statSync(join(root, rel)).isDirectory()) {
      if (entry === "node_modules" || entry === ".next") continue;
      files(rel, out);
    } else {
      out.push(rel);
    }
  }
  return out;
}

function appSources(): Record<string, string> {
  const sources: Record<string, string> = {};
  for (const rel of ["app", "components"].flatMap((d) => files(d))) {
    if (rel.startsWith(`${CHARTS_DIR}/`) || !/\.(?:tsx|jsx)$/.test(rel) || /\.test\.(?:tsx|jsx)$/.test(rel)) continue;
    sources[rel] = readFileSync(join(root, rel), "utf8");
  }
  return sources;
}

const CHART_IMPORT = 'import { TrendChart } from "@/components/charts";\n';

function surfacesOf(sources: Record<string, string>) {
  return chartPlacements(sources, GLASS_COMPONENTS).map((p) => p.surfaces.map((s) => s.kind));
}

describe("chart surface", () => {
  const sources = appSources();
  const placements = chartPlacements(sources, GLASS_COMPONENTS);

  // Found a second way, by the layer's component names in the text, so a chart the
  // walk cannot see (imported some way it does not resolve) fails here.
  it("finds every chart the app draws", () => {
    const index = readFileSync(join(root, CHARTS_DIR, "index.ts"), "utf8");
    const components = [...index.matchAll(/export \{([^}]*)\}/g)]
      .flatMap((m) => m[1].split(","))
      .map((name) => name.trim())
      .filter((name) => /^[A-Z]\w*$/.test(name));
    expect(components.length).toBeGreaterThan(0);
    const tag = new RegExp(`<(?:${components.join("|")})[\\s/>]`, "g");
    const written = Object.entries(sources).flatMap(([file, text]) => (text.match(tag) ?? []).map(() => file));
    expect(written.length).toBeGreaterThan(0);
    const placed = new Set(placements.map((p) => p.file));
    expect([...new Set(written)].filter((file) => !placed.has(file))).toEqual([]);
    expect(placements.every((p) => p.surfaces.length > 0)).toBe(true);
  });

  it("draws every chart on the glass card", () => {
    expect(offGlass(placements)).toEqual([]);
  });

  it("renders the glass card from the Card component", () => {
    const card = readFileSync(join(root, "components/ui/card.tsx"), "utf8");
    const classes = /const Card = [\s\S]*?className=\{cn\("([^"]*)"/.exec(card)?.[1].split(/\s+/) ?? [];
    expect(paintOf(classes)).toEqual({ kind: "glass" });
  });

  it("reads what a class list paints", () => {
    expect(paintOf(["glass", "p-6"])).toEqual({ kind: "glass" });
    expect(paintOf(["bg-glass", "rounded-card"])).toEqual({ kind: "glass" });
    expect(paintOf(["h-64", "flex", "bg-clip-text", "bg-transparent"])).toBeNull();
    expect(paintOf(["bg-white", "dark:bg-gray-800", "rounded-xl"])).toEqual({
      kind: "painted",
      paint: "bg-white dark:bg-gray-800",
    });
    // A glass card repainted in one theme or state is not the card the tokens were measured on.
    expect(paintOf(["glass", "dark:bg-gray-800"])).toEqual({ kind: "painted", paint: "dark:bg-gray-800" });
    expect(paintOf(["glass", "hover:bg-[rgba(0,0,0,0.5)]"])?.kind).toBe("painted");
    // The other glass surfaces have their own fills.
    expect(paintOf(["glass-inset"])?.kind).toBe("painted");
    expect(paintOf(["bg-glass/50"])?.kind).toBe("painted");
  });

  it("recognises a chart on a card that is not glass", () => {
    const page = (card: string) => ({
      "app/x/page.tsx": `${CHART_IMPORT}export default function Page() {
  return (
    <div className="space-y-6">
      <div className=${card}>
        <div className="h-64">
          <TrendChart title="t" data={[]} xKey="d" series={[]} />
        </div>
      </div>
    </div>
  );
}`,
    });
    expect(surfacesOf(page('"glass p-6"'))).toEqual([["glass"]]);
    expect(surfacesOf(page('{cn("glass", wide && "p-8")}'))).toEqual([["glass"]]);
    expect(surfacesOf(page('"bg-white dark:bg-gray-800 rounded-xl border p-6"'))).toEqual([["painted"]]);
    expect(surfacesOf(page("{`rounded-xl ${dark ? \"bg-gray-800\" : \"glass\"}`}"))).toEqual([["painted"]]);

    const plain = page('"bg-white dark:bg-gray-800 rounded-xl border p-6"');
    expect(offGlass(chartPlacements(plain, GLASS_COMPONENTS))).toEqual([
      'app/x/page.tsx:7 TrendChart is drawn on "bg-white dark:bg-gray-800" (app/x/page.tsx:5)',
    ]);
  });

  it("takes the nearest painted element, not the outer card", () => {
    const sources = {
      "app/x/page.tsx": `${CHART_IMPORT}export default function Page() {
  return (
    <div className="glass p-6">
      <div className="bg-surface-inset h-64">
        <TrendChart title="t" data={[]} xKey="d" series={[]} />
      </div>
    </div>
  );
}`,
    };
    expect(surfacesOf(sources)).toEqual([["painted"]]);
  });

  it("follows a component that draws a chart to every place it is rendered", () => {
    const wrapper = `${CHART_IMPORT}export function Activity() {
  return <TrendChart title="t" data={[]} xKey="d" series={[]} />;
}`;
    const sources = {
      "components/overview/shared.tsx": wrapper,
      "components/overview/lens.tsx": `import { Activity as Verifications } from "./shared";
export function Lens() {
  return <div className="glass p-5"><Verifications /></div>;
}`,
      "app/dashboard/page.tsx": `import { Activity } from "@/components/overview/shared";
function Panel() {
  return <div className="flex"><Activity /></div>;
}
export default function Page() {
  return <section className="bg-white p-4"><Panel /></section>;
}`,
      // Same name, another module: not a place this chart is drawn.
      "app/other/page.tsx": `import { Activity } from "@/components/feed";
export default function Page() {
  return <div className="bg-black"><Activity /></div>;
}`,
    };
    const [placement] = chartPlacements(sources, GLASS_COMPONENTS);
    expect(placement.file).toBe("components/overview/shared.tsx");
    expect(placement.surfaces.map((s) => `${s.kind} ${s.file}:${s.line}`).sort()).toEqual([
      "glass components/overview/lens.tsx:3",
      "painted app/dashboard/page.tsx:6",
    ]);
  });

  it("follows a default export to where it is imported", () => {
    const sources = {
      "components/activity.tsx": `${CHART_IMPORT}function Activity() {
  return <TrendChart title="t" data={[]} xKey="d" series={[]} />;
}
export default Activity;`,
      "app/x/page.tsx": `import Verifications from "@/components/activity";
export default function Page() {
  return <div className="glass-inset p-4"><Verifications /></div>;
}`,
    };
    expect(offGlass(chartPlacements(sources, GLASS_COMPONENTS))).toEqual([
      'components/activity.tsx:3 TrendChart is drawn on "glass-inset" (app/x/page.tsx:3)',
    ]);
  });

  it("counts the Card component as glass unless it is repainted", () => {
    const page = (card: string) => ({
      "components/agent/history.tsx": `${CHART_IMPORT}import { Card, CardContent } from "@/components/ui/card";
export function History() {
  return (
    ${card}
      <CardContent><TrendChart title="t" data={[]} xKey="d" series={[]} /></CardContent>
    </Card>
  );
}`,
    });
    expect(surfacesOf(page("<Card>"))).toEqual([["glass"]]);
    expect(surfacesOf(page('<Card className="bg-white">'))).toEqual([["painted"]]);
    // A Card from anywhere else is not known to be glass, and nothing renders History.
    const elsewhere = page("<Card>");
    elsewhere["components/agent/history.tsx"] = elsewhere["components/agent/history.tsx"].replace(
      "@/components/ui/card",
      "@/components/legacy/card",
    );
    expect(surfacesOf(elsewhere)).toEqual([["none"]]);
  });

  it("reports a chart with no card around it", () => {
    const sources = {
      "app/x/page.tsx": `${CHART_IMPORT}export default function Page() {
  return <main className="p-6"><TrendChart title="t" data={[]} xKey="d" series={[]} /></main>;
}`,
    };
    expect(offGlass(chartPlacements(sources, GLASS_COMPONENTS))).toEqual([
      "app/x/page.tsx:3 TrendChart reaches the top of Page (app/x/page.tsx:3) with no card around it",
    ]);
  });
});

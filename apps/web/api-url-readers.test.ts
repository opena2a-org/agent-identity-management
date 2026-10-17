// @vitest-environment node
import { describe, expect, it } from "vitest";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import ts from "typescript";

// The build replaces process.env.NEXT_PUBLIC_API_URL with the value it read. The client
// resolves its API URL in its own order (getApiUrl in lib/api.ts), where that value
// comes after the "-backend" host a "-frontend" hostname derives, so a component that
// builds an API URL from the value can call another API than the one the client signs
// in to. Components read apiBaseUrl() from lib/api-base-url.ts instead. Root-level on
// purpose: it walks the whole app.

// The files allowed to read it: the resolver and the build configuration.
const ALLOWED = ["lib/api.ts", "next.config.ts"];

const VARIABLE = "NEXT_PUBLIC_API_URL";

function sourceFiles(dir = __dirname, out: string[] = []): string[] {
  for (const entry of readdirSync(dir)) {
    if (entry === "node_modules" || entry.startsWith(".")) continue;
    const path = join(dir, entry);
    if (statSync(path).isDirectory()) sourceFiles(path, out);
    else if (/\.[cm]?[jt]sx?$/.test(entry) && !/\.test\.[jt]sx?$/.test(entry)) out.push(relative(__dirname, path));
  }
  return out;
}

function isProcessEnv(node: ts.Expression): boolean {
  return (
    ts.isPropertyAccessExpression(node) &&
    node.name.text === "env" &&
    ts.isIdentifier(node.expression) &&
    node.expression.text === "process"
  );
}

// True when the source reads process.env.NEXT_PUBLIC_API_URL, the form the build
// replaces, or process.env["NEXT_PUBLIC_API_URL"]. Comments and other lookups do not count.
function readsVariable(text: string, path = "source.tsx"): boolean {
  const source = ts.createSourceFile(path, text, ts.ScriptTarget.Latest, true);
  let found = false;
  const visit = (node: ts.Node): void => {
    if (found) return;
    if (ts.isPropertyAccessExpression(node) && node.name.text === VARIABLE && isProcessEnv(node.expression)) {
      found = true;
    } else if (
      ts.isElementAccessExpression(node) &&
      ts.isStringLiteralLike(node.argumentExpression) &&
      node.argumentExpression.text === VARIABLE &&
      isProcessEnv(node.expression)
    ) {
      found = true;
    } else {
      ts.forEachChild(node, visit);
    }
  };
  visit(source);
  return found;
}

describe("NEXT_PUBLIC_API_URL readers", () => {
  it("are found as a property or a string index of process.env, and not in a comment or another lookup", () => {
    expect(readsVariable("const a = process.env.NEXT_PUBLIC_API_URL;")).toBe(true);
    expect(readsVariable('const a = process.env["NEXT_PUBLIC_API_URL"];')).toBe(true);
    expect(readsVariable("const a = <p>{process.env.NEXT_PUBLIC_API_URL}</p>;")).toBe(true);
    expect(readsVariable("// process.env.NEXT_PUBLIC_API_URL\nconst a = 1;")).toBe(false);
    expect(readsVariable("const a = env.NEXT_PUBLIC_API_URL;")).toBe(false);
  });

  it("are the resolver and the build configuration only", () => {
    const files = sourceFiles();
    // Control: the walk reaches the app, the components and the library.
    expect(files).toEqual(
      expect.arrayContaining(["app/dashboard/developers/page.tsx", "components/modals/mcp-detail-modal.tsx", "lib/agent-sdk.ts"]),
    );

    const readers = files.filter((file) => readsVariable(readFileSync(join(__dirname, file), "utf8"), file));
    expect(readers.sort()).toEqual(ALLOWED);
  });
});

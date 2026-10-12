// @vitest-environment node
import { describe, expect, it } from "vitest";
import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import ts from "typescript";

// hackmyagent's LOG-002 ("Sensitive Data in Logs") matches a `console.log` call whose first
// argument begins with `password`, `apiKey`, `secret` or `token`, but it reads only the .ts
// and .js files directly in the directory it scans. The repository root has none, so
// `secure . --ci --format json` records LOG-002 as not applicable and reads no file of this
// app. This test makes the same match, for each method in CONSOLE_METHODS, over every tracked
// source file of the dashboard. It parses each file, so a string or a comment that quotes the call
// is text, not a call.
const root = join(__dirname, "..");
const SOURCE = /\.(?:[cm]?[jt]s|[jt]sx)$/;
const CONSOLE_METHODS = new Set(["log", "info", "warn", "error", "debug", "trace"]);
const SENSITIVE_FIRST_ARGUMENT = /^(?:password|apikey|secret|token)(?![A-Za-z0-9_$])/i;

function sensitiveLogCalls(path: string, text: string): string[] {
  const source = ts.createSourceFile(path, text, ts.ScriptTarget.Latest, true);
  const hits: string[] = [];
  const visit = (node: ts.Node) => {
    if (
      ts.isCallExpression(node) &&
      ts.isPropertyAccessExpression(node.expression) &&
      ts.isIdentifier(node.expression.expression) &&
      node.expression.expression.text === "console" &&
      CONSOLE_METHODS.has(node.expression.name.text) &&
      node.arguments.length > 0 &&
      SENSITIVE_FIRST_ARGUMENT.test(node.arguments[0].getText(source))
    ) {
      const { line } = source.getLineAndCharacterOfPosition(node.getStart(source));
      hits.push(`${path}:${line + 1}: ${node.getText(source)}`);
    }
    ts.forEachChild(node, visit);
  };
  visit(source);
  return hits;
}

const tracked = execFileSync("git", ["ls-files", "-z"], { cwd: root, encoding: "utf8" })
  .split("\0")
  .filter((path) => SOURCE.test(path));

describe("no secret-named value logged to the console", () => {
  it("finds a planted call for each console method, and not a string, a comment or a longer name", () => {
    const planted = [
      "export function f(token: string, apiKey: string, Password: string) {",
      "  console.log(token);",
      "  console.error(apiKey.slice(0, 4));",
      "  console.warn(Password, 1);",
      "}",
      "",
    ].join("\n");
    expect(sensitiveLogCalls("lib/x.ts", planted)).toEqual([
      "lib/x.ts:2: console.log(token)",
      "lib/x.ts:3: console.error(apiKey.slice(0, 4))",
      "lib/x.ts:4: console.warn(Password, 1)",
    ]);

    const quoted = [
      "export function Help({ tokenCount }: { tokenCount: number }) {",
      "  // console.log(token) prints the token",
      '  const hint = "console.log(secret)";',
      "  console.log(tokenCount);",
      '  console.log("token refreshed");',
      "  logger.log(token);",
      "  return <p>{hint} console.log(password)</p>;",
      "}",
      "",
    ].join("\n");
    expect(sensitiveLogCalls("components/help.tsx", quoted)).toEqual([]);
  });

  it("reads the dashboard's tracked source, the tab bar included", () => {
    expect(tracked).toContain("components/mobile-tab-bar.tsx");
    expect(tracked).toContain("app/dashboard/layout.tsx");
  });

  it("has no console call whose first argument is a password, API key, secret or token", () => {
    const hits = tracked.flatMap((path) => sensitiveLogCalls(path, readFileSync(join(root, path), "utf8")));
    expect(hits).toEqual([]);
  });
});

// @vitest-environment node
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import { LOCAL_API_ORIGIN, assertNoLocalApiOrigin, filesHoldingLocalApiOrigin } from "./local-api-origin";

const made: string[] = [];

// A build directory whose static/ holds the given files, keyed by their path under it.
function buildDir(files: Record<string, string | Buffer>): string {
  const dir = mkdtempSync(join(tmpdir(), "aim-built-"));
  made.push(dir);
  mkdirSync(join(dir, "static"));
  for (const [path, content] of Object.entries(files)) {
    mkdirSync(dirname(join(dir, "static", path)), { recursive: true });
    writeFileSync(join(dir, "static", path), content);
  }
  return dir;
}

afterEach(() => {
  for (const dir of made.splice(0)) rmSync(dir, { recursive: true, force: true });
});

describe("filesHoldingLocalApiOrigin", () => {
  it("lists every file under static/ that holds http://localhost:8080, at any depth", () => {
    // The shape a build that read no URL wrote under the previous next.config.ts.
    const built = 'return console.log("[API] Using NEXT_PUBLIC_API_URL:","http://localhost:8080"),"http://localhost:8080"';
    const dir = buildDir({
      "chunks/0jo04432x72wh.js": built,
      "chunks/app/dashboard/page.js": 'fetch("http://localhost:8080/api/v1/agents")',
      "chunks/default-host.js": "const t=`${e}//${n}:8080`",
      "chunks/other-port.js": '"http://localhost:8081"',
      "css/app.css": "body{margin:0}",
      "media/font.woff2": Buffer.from([0x77, 0x4f, 0x46, 0x32, 0x00, 0xff, 0xfe]),
    });

    expect(filesHoldingLocalApiOrigin(dir)).toEqual(["static/chunks/0jo04432x72wh.js", "static/chunks/app/dashboard/page.js"]);
  });

  it("lists nothing for browser code that holds no copy", () => {
    expect(filesHoldingLocalApiOrigin(buildDir({ "chunks/a.js": '"https://api.example.com"' }))).toEqual([]);
  });

  it("fails when the build directory has no static/ to check", () => {
    const dir = mkdtempSync(join(tmpdir(), "aim-built-"));
    made.push(dir);
    expect(() => filesHoldingLocalApiOrigin(dir)).toThrow(/No browser code to check/);
  });
});

describe("assertNoLocalApiOrigin", () => {
  it("fails, naming the files, when the build was given no NEXT_PUBLIC_API_URL that holds the address", () => {
    const dir = buildDir({ "chunks/a.js": `"${LOCAL_API_ORIGIN}"`, "chunks/b.js": "1" });
    for (const builtApiUrl of [undefined, "", "http://localhost:8081", "https://api.example.com", "http://localhost:808"]) {
      expect(() => assertNoLocalApiOrigin(dir, builtApiUrl), String(builtApiUrl)).toThrow(
        "The dashboard's browser code holds http://localhost:8080, and the build was given no NEXT_PUBLIC_API_URL " +
          "that holds it: static/chunks/a.js",
      );
    }
  });

  it("passes a build that was given a NEXT_PUBLIC_API_URL holding the address", () => {
    const dir = buildDir({ "chunks/a.js": `"${LOCAL_API_ORIGIN}"` });
    for (const builtApiUrl of ["http://localhost:8080", "http://localhost:8080/"]) {
      expect(() => assertNoLocalApiOrigin(dir, builtApiUrl), builtApiUrl).not.toThrow();
    }
  });

  it("passes browser code that holds no copy, whatever the build was given", () => {
    const dir = buildDir({ "chunks/a.js": "const t=`${e}//${n}:8080`" });
    for (const builtApiUrl of [undefined, "http://localhost:8081"]) {
      expect(() => assertNoLocalApiOrigin(dir, builtApiUrl), String(builtApiUrl)).not.toThrow();
    }
  });
});

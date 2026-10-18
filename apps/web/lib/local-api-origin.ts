import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";

// The check next.config.ts runs once next build has written the browser's code: that
// code may hold http://localhost:8080 only when the build was given a NEXT_PUBLIC_API_URL
// that holds it.
//
// next build writes the NEXT_PUBLIC_API_URL it reads into the browser's code where the
// code reads the variable (lib/api.ts), so a build given that address holds it by
// request. Any other copy is an API address fixed into the dashboard image, which no
// setting of the running container changes. The browser's code is every file under
// <distDir>/static.

export const LOCAL_API_ORIGIN = "http://localhost:8080";

// The files under <distDir>/static that hold LOCAL_API_ORIGIN, relative to distDir and
// sorted. Throws when distDir has no static directory, so a moved directory does not
// pass as one that holds no copy.
export function filesHoldingLocalApiOrigin(distDir: string): string[] {
  const staticDir = join(distDir, "static");
  if (!statSync(staticDir, { throwIfNoEntry: false })?.isDirectory()) {
    throw new Error(`No browser code to check: ${staticDir} is not a directory`);
  }
  const found: string[] = [];
  const walk = (dir: string): void => {
    for (const entry of readdirSync(dir, { withFileTypes: true })) {
      const path = join(dir, entry.name);
      if (entry.isDirectory()) walk(path);
      // latin1 maps each byte to one character, so a font or an image reads too, and the
      // ASCII address matches in UTF-8 text.
      else if (entry.isFile() && readFileSync(path, "latin1").includes(LOCAL_API_ORIGIN)) {
        found.push(relative(distDir, path));
      }
    }
  };
  walk(staticDir);
  return found.sort();
}

// Throws, naming the files, when the browser's code under distDir holds
// LOCAL_API_ORIGIN and builtApiUrl, the NEXT_PUBLIC_API_URL the build read, does not.
export function assertNoLocalApiOrigin(distDir: string, builtApiUrl: string | undefined): void {
  if (builtApiUrl?.includes(LOCAL_API_ORIGIN)) return;
  const files = filesHoldingLocalApiOrigin(distDir);
  if (files.length > 0) {
    throw new Error(
      `The dashboard's browser code holds ${LOCAL_API_ORIGIN}, and the build was given no ` +
        `NEXT_PUBLIC_API_URL that holds it: ${files.join(", ")}`,
    );
  }
}

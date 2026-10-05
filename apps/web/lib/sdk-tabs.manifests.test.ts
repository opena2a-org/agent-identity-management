import { describe, it, expect } from "vitest";
import { existsSync, readFileSync } from "fs";
import path from "path";
import { SDK_TABS, type SdkLang } from "./sdk-tabs";

// Every package name, CLI command, flag and import the dashboard cites is read
// back from the SDK's own manifest in this repository, so renaming an SDK fails
// here instead of leaving a developer with an install line that does not run.

const SDK_ROOT = path.resolve(__dirname, "..", "..", "..", "sdk");
const ORIGIN = "https://aim.example.test";

const read = (...parts: string[]) => readFileSync(path.join(SDK_ROOT, ...parts), "utf8");
const tab = (key: SdkLang) => SDK_TABS.find((t) => t.key === key)!;

function firstMatch(text: string, re: RegExp, what: string): string {
  const m = text.match(re);
  if (!m) throw new Error(`${what} not found`);
  return m[1];
}

describe("SDK_TABS cite the names in each SDK's manifest", () => {
  describe("Python (sdk/python/setup.py)", () => {
    const setup = read("python", "setup.py");
    const distName = firstMatch(setup, /setup\(\s*name="([^"]+)"/, "setup.py name");
    const [script, module] = firstMatch(setup, /"console_scripts":\s*\[\s*"([^"]+)"/, "console_scripts entry")
      .split("=")
      .map((s) => s.trim());
    const importName = module.split(".")[0];

    it("installs the distribution setup.py publishes", () => {
      for (const origin of ["", ORIGIN]) {
        expect(tab("python").install(origin)).toMatch(new RegExp(`^pip install ${distName} && `));
      }
    });

    it("runs the console script setup.py installs, with a flag its login parser defines", () => {
      expect(tab("python").install("")).toContain(`&& ${script} login`);
      expect(tab("python").install(ORIGIN)).toContain(`&& ${script} login --url ${ORIGIN}`);
      const cli = read("python", `${module.split(":")[0].replace(/\./g, "/")}.py`);
      expect(cli).toMatch(/add_parser\(\s*'login'/);
      expect(cli).toMatch(/login_parser\.add_argument\(\s*'--url'/);
    });

    it("imports secure from the package the console script lives in", () => {
      expect(tab("python").code("")).toContain(`from ${importName} import secure`);
      expect(read("python", importName, "__init__.py")).toMatch(/^\s*"secure",$/m);
    });
  });

  describe("TypeScript (sdk/typescript/package.json)", () => {
    const pkgName = JSON.parse(read("typescript", "package.json")).name as string;

    it("installs and imports the package package.json names", () => {
      expect(tab("typescript").install("")).toBe(`npm install ${pkgName}`);
      expect(tab("typescript").code("")).toContain(`import { AIMClient, AgentType } from "${pkgName}";`);
    });

    it("imports only symbols the package entry point exports", () => {
      const index = read("typescript", "src", "index.ts");
      expect(index).toMatch(/^export \{ AIMClient\b/m);
      expect(index).toMatch(/^\s*AgentType,$/m);
    });
  });

  describe("Java (sdk/java/pom.xml)", () => {
    const pom = read("java", "pom.xml");
    const groupId = firstMatch(pom, /<\/modelVersion>\s*<groupId>([^<]+)<\/groupId>/, "pom groupId");
    const artifactId = firstMatch(pom, /<\/groupId>\s*<artifactId>([^<]+)<\/artifactId>/, "pom artifactId");
    const version = firstMatch(pom, /<\/artifactId>\s*<version>([^<]+)<\/version>/, "pom version");

    it("builds the directory that holds pom.xml from a fresh clone", () => {
      const install = tab("java").install("");
      const clone = firstMatch(install, /^git clone \S+\/([^/\s]+)\.git && /, "clone target");
      expect(install).toContain(`mvn -f ${clone}/sdk/java `);
      expect(install).toMatch(/ install$/);
      expect(existsSync(path.join(SDK_ROOT, "java", "pom.xml"))).toBe(true);
    });

    it("says the artifact is built from source and names the coordinates pom.xml installs", () => {
      const note = tab("java").note ?? "";
      expect(note).toContain("not published to Maven Central");
      expect(note).toContain(`${groupId}:${artifactId}:${version}`);
    });

    it("imports a class the SDK defines, with the secure() factory the example calls", () => {
      const fqcn = firstMatch(tab("java").code(""), /^import ([\w.]+);$/m, "Java import");
      const source = read("java", "src", "main", "java", ...fqcn.split(".").slice(0, -1), `${fqcn.split(".").pop()}.java`);
      expect(source).toMatch(/public static AIMClient secure\(String \w+\)/);
      expect(source).toMatch(/public <T> T performAction\(String \w+, String \w+, Supplier<T> \w+\)/);
    });
  });
});

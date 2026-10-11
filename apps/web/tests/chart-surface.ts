/**
 * Where each chart is drawn, read from the source: for every chart element in
 * app/ and components/, the nearest element above it that paints a background.
 * The --chart-* tokens hold their contrast floors against the glass card
 * (tests/chart-contrast.test.ts), so a chart on any other surface is outside
 * what that test measures.
 *
 * The walk goes up the JSX a chart is written in. When it reaches the top of a
 * component without meeting a painted element, it continues from every place
 * that component is rendered, in the same file or in a file that imports it by
 * name. A component rendered nowhere the walk can follow (a route's page, or a
 * component loaded through a dynamic import) has no surface.
 *
 * What is behind a translucent card, and anything painted from a stylesheet
 * rather than a class name, is the browser sweep's to measure
 * (scripts/ux-sweep.mjs).
 */

import ts from "typescript";

/** The folder whose exports are the charts, as an import specifier and as a path. */
const CHART_LAYER = "components/charts";

export type Surface =
  /** The glass card: `glass`, `bg-glass`, or a component that renders one. */
  | { kind: "glass"; file: string; line: number }
  /** Any other background. */
  | { kind: "painted"; file: string; line: number; paint: string }
  /** Nothing painted between the chart and the page. */
  | { kind: "none"; file: string; line: number; component: string };

export interface ChartPlacement {
  file: string;
  line: number;
  /** The chart component's name as written at the call site. */
  chart: string;
  /** One per place the chart ends up: several when a wrapper is rendered more than once. */
  surfaces: Surface[];
}

export interface GlassComponent {
  /** The file that defines it, relative to the web app, without extension. */
  module: string;
  name: string;
}

// Background utilities that set no colour.
const NOT_A_PAINT =
  /^bg-(?:transparent|none|clip-|origin-|repeat|no-repeat|cover|contain|auto|center|top|bottom|left|right|fixed|local|scroll|blend-|opacity-)/;

/** A class with its variants (`dark:`, `hover:`, `md:`) and `!` removed. */
function baseClass(token: string): string {
  let depth = 0;
  let start = 0;
  for (let i = 0; i < token.length; i++) {
    const c = token[i];
    if (c === "[" || c === "(") depth++;
    else if (c === "]" || c === ")") depth--;
    else if (c === ":" && depth === 0) start = i + 1;
  }
  return token.slice(start).replace(/^!/, "");
}

/**
 * What a class list paints: "glass" for the glass card alone, the offending
 * classes when anything else sets a background in any theme or state, null when
 * it sets none.
 */
export function paintOf(classes: string[]): { kind: "glass" } | { kind: "painted"; paint: string } | null {
  let glass = false;
  const other: string[] = [];
  for (const token of classes) {
    const base = baseClass(token);
    if (base === "glass" || base === "bg-glass") glass = true;
    else if (base.startsWith("glass-")) other.push(token);
    else if (base.startsWith("bg-") && !NOT_A_PAINT.test(base)) other.push(token);
  }
  if (other.length > 0) return { kind: "painted", paint: other.join(" ") };
  return glass ? { kind: "glass" } : null;
}

// Every class a className attribute can produce: each string in the expression,
// so both arms of a conditional and every argument of cn() count.
function classesIn(attribute: ts.JsxAttribute): string[] {
  const out: string[] = [];
  const add = (text: string) => out.push(...text.split(/\s+/).filter(Boolean));
  const visit = (node: ts.Node) => {
    if (ts.isStringLiteralLike(node)) add(node.text);
    else if (ts.isTemplateHead(node) || ts.isTemplateMiddle(node) || ts.isTemplateTail(node)) add(node.text);
    ts.forEachChild(node, visit);
  };
  if (attribute.initializer) visit(attribute.initializer);
  return out;
}

type JsxTag = ts.JsxOpeningElement | ts.JsxSelfClosingElement;

function classNameOf(tag: JsxTag): string[] {
  for (const property of tag.attributes.properties) {
    if (ts.isJsxAttribute(property) && property.name.getText() === "className") return classesIn(property);
  }
  return [];
}

interface Parsed {
  file: string;
  source: ts.SourceFile;
  /** Local name to the module it was imported from and its name there. */
  imports: Map<string, { module: string; name: string }>;
  /** The identifier in `export default <identifier>`, when the file has one. */
  defaultExport?: string;
  tags: JsxTag[];
}

/** An import specifier as a path relative to the web app, without extension. */
function moduleOf(specifier: string, fromFile: string): string | null {
  if (specifier.startsWith("@/")) return specifier.slice(2);
  if (!specifier.startsWith(".")) return null;
  const parts = fromFile.split("/").slice(0, -1);
  for (const part of specifier.split("/")) {
    if (part === "..") parts.pop();
    else if (part !== ".") parts.push(part);
  }
  return parts.join("/");
}

function withoutExtension(file: string): string {
  return file.replace(/\.(?:tsx|ts|jsx|js)$/, "").replace(/\/index$/, "");
}

function parse(file: string, text: string): Parsed {
  const source = ts.createSourceFile(file, text, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  const imports: Parsed["imports"] = new Map();
  const tags: JsxTag[] = [];
  let defaultExport: string | undefined;
  const visit = (node: ts.Node) => {
    if (ts.isExportAssignment(node) && !node.isExportEquals && ts.isIdentifier(node.expression)) {
      defaultExport = node.expression.text;
    }
    if (ts.isImportDeclaration(node) && ts.isStringLiteral(node.moduleSpecifier)) {
      const from = moduleOf(node.moduleSpecifier.text, file);
      const clause = node.importClause;
      // A type is never rendered, so `import type` and `{ type X }` are left out.
      if (from && clause && !clause.isTypeOnly) {
        if (clause.name) imports.set(clause.name.text, { module: from, name: "default" });
        if (clause.namedBindings && ts.isNamedImports(clause.namedBindings)) {
          for (const element of clause.namedBindings.elements) {
            if (element.isTypeOnly) continue;
            imports.set(element.name.text, { module: from, name: (element.propertyName ?? element.name).text });
          }
        }
      }
    }
    if (ts.isJsxOpeningElement(node) || ts.isJsxSelfClosingElement(node)) tags.push(node);
    ts.forEachChild(node, visit);
  };
  visit(source);
  return { file, source, imports, defaultExport, tags };
}

function lineOf(parsed: Parsed, node: ts.Node): number {
  return parsed.source.getLineAndCharacterOfPosition(node.getStart(parsed.source)).line + 1;
}

/**
 * The name of the component a node is written in, and whether that is the
 * file's default export. A lower-case helper inside a component (a render
 * function) is passed over, so a chart it returns is placed where the component
 * is rendered, not where the helper is called.
 */
function componentAround(node: ts.Node, defaultExport: string | undefined): { name: string; isDefault: boolean } | null {
  for (let at: ts.Node | undefined = node.parent; at; at = at.parent) {
    if (ts.isFunctionDeclaration(at) && at.name && /^[A-Z]/.test(at.name.text)) {
      const modifiers = ts.getModifiers(at) ?? [];
      const isDefault = modifiers.some((m) => m.kind === ts.SyntaxKind.DefaultKeyword) || at.name.text === defaultExport;
      return { name: at.name.text, isDefault };
    }
    if (ts.isVariableDeclaration(at) && ts.isIdentifier(at.name) && /^[A-Z]/.test(at.name.text)) {
      return { name: at.name.text, isDefault: at.name.text === defaultExport };
    }
  }
  return null;
}

/**
 * Every chart element in `sources` (path relative to the web app, to its text)
 * with the surface or surfaces it is drawn on. Files inside the chart layer and
 * test files are not places a chart is drawn for a reader, so leave them out.
 */
export function chartPlacements(sources: Record<string, string>, glassComponents: GlassComponent[] = []): ChartPlacement[] {
  const files = Object.entries(sources).map(([file, text]) => parse(file, text));

  const isGlassComponent = (parsed: Parsed, tag: string) => {
    const imported = parsed.imports.get(tag);
    return imported !== undefined && glassComponents.some((g) => g.module === imported.module && g.name === imported.name);
  };

  // The surface `node` is drawn on, where `node` is a chart or a component that
  // draws one: its own className first (it lands on the element it renders),
  // then each element around it.
  const surfacesAbove = (parsed: Parsed, node: JsxTag, seen: Set<string>): Surface[] => {
    const element = ts.isJsxOpeningElement(node) ? node.parent : node;
    const around: JsxTag[] = [node];
    for (let at: ts.Node | undefined = element.parent; at; at = at.parent) {
      if (ts.isJsxElement(at)) around.push(at.openingElement);
    }
    for (const tag of around) {
      const paint = paintOf(classNameOf(tag));
      const where = { file: parsed.file, line: lineOf(parsed, tag) };
      if (paint?.kind === "painted") return [{ kind: "painted", ...where, paint: paint.paint }];
      if (paint?.kind === "glass" || isGlassComponent(parsed, tag.tagName.getText())) return [{ kind: "glass", ...where }];
    }

    const component = componentAround(node, parsed.defaultExport);
    const nowhere: Surface = {
      kind: "none",
      file: parsed.file,
      line: lineOf(parsed, node),
      component: component?.name ?? "(module scope)",
    };
    if (!component) return [nowhere];
    const key = `${parsed.file}#${component.name}`;
    if (seen.has(key)) return [];
    const next = new Set(seen).add(key);

    const home = withoutExtension(parsed.file);
    const rendered = files.flatMap((other) =>
      other.tags
        .filter((tag) => {
          const name = tag.tagName.getText();
          if (other.file === parsed.file) return name === component.name;
          const imported = other.imports.get(name);
          if (!imported || imported.module !== home) return false;
          return imported.name === component.name || (imported.name === "default" && component.isDefault);
        })
        .flatMap((tag) => surfacesAbove(other, tag, next)),
    );
    return rendered.length > 0 ? rendered : [nowhere];
  };

  return files.flatMap((parsed) =>
    parsed.tags
      .filter((tag) => {
        const imported = parsed.imports.get(tag.tagName.getText());
        return imported !== undefined && (imported.module === CHART_LAYER || imported.module.startsWith(`${CHART_LAYER}/`));
      })
      .map((tag) => ({
        file: parsed.file,
        line: lineOf(parsed, tag),
        chart: tag.tagName.getText(),
        surfaces: surfacesAbove(parsed, tag, new Set()),
      })),
  );
}

/** One line per chart that is drawn anywhere but on the glass card. */
export function offGlass(placements: ChartPlacement[]): string[] {
  return placements.flatMap((p) =>
    p.surfaces
      .filter((s) => s.kind !== "glass")
      .map((s) =>
        s.kind === "painted"
          ? `${p.file}:${p.line} ${p.chart} is drawn on "${s.paint}" (${s.file}:${s.line})`
          : `${p.file}:${p.line} ${p.chart} reaches the top of ${s.component} (${s.file}:${s.line}) with no card around it`,
      ),
  );
}

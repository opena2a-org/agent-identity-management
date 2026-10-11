/**
 * Growing flex items that cannot shrink, read from the source.
 *
 * A flex item is never narrower than its content unless told otherwise
 * (`min-width: auto`). A `flex-1` box around a `truncate` line is therefore as
 * wide as the whole line before truncation: the ellipsis never shows, and on a
 * 375px screen the row pushes the page sideways. Every growing box between the
 * truncated text and the nearest ancestor that clips its overflow needs
 * `min-w-0` (or a clipping overflow of its own) to shrink.
 *
 * The walk goes up the JSX of one component and through the callbacks inside it
 * (a `.map()` that renders rows). Text truncated inside a component rendered
 * from somewhere else is the browser sweep's to measure: scripts/ux-sweep.mjs
 * reports horizontal overflow at 375x812.
 */

import ts from "typescript";
import { baseClass, classNameOf } from "./chart-surface";

const GROWS = new Set(["flex-1", "flex-auto", "grow", "flex-grow"]);
const CLIPS = /^overflow-(?:[xy]-)?(?:hidden|clip|auto|scroll)$/;

export interface UnshrinkableItem {
  file: string;
  /** The line of the growing box that cannot shrink. */
  line: number;
  /** The line of the truncated text inside it. */
  truncatedAt: number;
  classes: string;
}

function lineOf(source: ts.SourceFile, node: ts.Node): number {
  return source.getLineAndCharacterOfPosition(node.getStart(source)).line + 1;
}

/** Each growing box in `text` that holds truncated text and cannot shrink, once per box. */
export function unshrinkableFlexItems(file: string, text: string): UnshrinkableItem[] {
  const source = ts.createSourceFile(file, text, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  const found = new Map<number, UnshrinkableItem>();

  const check = (tag: ts.JsxOpeningElement | ts.JsxSelfClosingElement) => {
    if (!classNameOf(tag).some((c) => baseClass(c) === "truncate")) return;
    const truncatedAt = lineOf(source, tag);
    const start = ts.isJsxOpeningElement(tag) ? tag.parent : tag;
    for (let at: ts.Node | undefined = start.parent; at; at = at.parent) {
      // A component's JSX ends at its declaration; where it is rendered is another walk.
      if (ts.isFunctionDeclaration(at) || ts.isVariableDeclaration(at) || ts.isClassDeclaration(at)) break;
      if (!ts.isJsxElement(at)) continue;
      const classes = classNameOf(at.openingElement);
      const bases = classes.map(baseClass);
      const clips = bases.some((b) => CLIPS.test(b));
      if (bases.some((b) => GROWS.has(b)) && !clips && !bases.includes("min-w-0")) {
        const line = lineOf(source, at.openingElement);
        if (!found.has(line)) found.set(line, { file, line, truncatedAt, classes: classes.join(" ") });
      }
      if (clips) break;
    }
  };

  const visit = (node: ts.Node) => {
    if (ts.isJsxOpeningElement(node) || ts.isJsxSelfClosingElement(node)) check(node);
    ts.forEachChild(node, visit);
  };
  visit(source);
  return [...found.values()].sort((a, b) => a.line - b.line);
}

import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "./dialog";
import { minHeightPx } from "@/tests/target-size";

// 44x44 CSS px: the size scripts/ux-sweep.mjs counts every target against (it
// gates at the 24x24 of WCAG 2.5.8). The Close button set no size of its own
// and drew only its 16px icon (#731).
const CLOSE_TARGET_PX = 44;
const SPACING_STEP_PX = 4;

/** The class without its variants: `sm:h-4` is `h-4`. */
const base = (c: string) => c.slice(c.lastIndexOf(":") + 1);

/** Each px value the classes set on one axis, with or without a variant. */
function sizesPx(classes: string[], axis: "h" | "w"): number[] {
  const pattern = new RegExp(`^(?:${axis}|min-${axis}|size)-(\\d+(?:\\.\\d+)?)$`);
  return classes.flatMap((c) => {
    const m = pattern.exec(base(c));
    return m ? [Number(m[1]) * SPACING_STEP_PX] : [];
  });
}

function renderOpenDialog() {
  render(
    <Dialog open>
      <DialogContent>
        <DialogTitle>Detect MCP servers</DialogTitle>
        <DialogDescription>Run this on the machine where the agent runs.</DialogDescription>
      </DialogContent>
    </Dialog>,
  );
  return screen.getByRole("button", { name: "Close" });
}

afterEach(cleanup);

describe("DialogContent Close button", () => {
  it("is at least 44px tall on a phone", () => {
    const classes = renderOpenDialog().className.split(/\s+/);
    expect(minHeightPx(classes)).toBeGreaterThanOrEqual(CLOSE_TARGET_PX);
  });

  it("is at least 44px wide on a phone", () => {
    const classes = renderOpenDialog().className.split(/\s+/);
    const unvaried = classes.filter((c) => !c.includes(":"));
    expect(Math.max(0, ...sizesPx(unvaried, "w"))).toBeGreaterThanOrEqual(CLOSE_TARGET_PX);
  });

  it("sets no height or width under 44px at any breakpoint", () => {
    const classes = renderOpenDialog().className.split(/\s+/);
    const small = [...sizesPx(classes, "h"), ...sizesPx(classes, "w")].filter((px) => px < CLOSE_TARGET_PX);
    expect(small).toEqual([]);
  });

  it("centres its icon in the target", () => {
    const close = renderOpenDialog();
    const classes = close.className.split(/\s+/);
    expect(classes).toEqual(expect.arrayContaining(["inline-flex", "items-center", "justify-center"]));
    expect(close.querySelector("svg")).not.toBeNull();
  });
});

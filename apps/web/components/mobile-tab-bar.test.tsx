import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { MobileTabBar } from "./mobile-tab-bar";

vi.mock("next/navigation", () => ({ usePathname: () => "/dashboard" }));
vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));

afterEach(cleanup);

// jsdom lays nothing out, so this file reads class names, not drawn boxes;
// tests/e2e/mobile-tab-bar.spec.ts measures the boxes in a browser. The classes it looks for
// are a negative margin, offset or translate, which draw an element outside its parent's box,
// in each spelling Tailwind accepts: the `-` prefix (`-mt-3.5`), a bracketed negative value
// (`mt-[-14px]`, `translate-y-[-14px]`), and either of those behind variants
// (`max-lg:-mt-3.5`) or the `!` modifier (`!-mt-3.5`).
const MOVES_ITS_BOX = /^(?:m[trblxyse]?|top|right|bottom|left|start|end|inset(?:-[xy])?|translate-[xy])-/;
const NEGATIVE_BRACKETED_VALUE = /-\[\s*(?:-|calc\(\s*-)/;

function pullsOut(className: string): boolean {
  // The utility follows the last `:` outside brackets; what comes before it is variants.
  let depth = 0;
  let start = 0;
  for (let i = 0; i < className.length; i++) {
    if (className[i] === "[") depth++;
    else if (className[i] === "]") depth--;
    else if (className[i] === ":" && depth === 0) start = i + 1;
  }
  const utility = className.slice(start).replace(/^!/, "");
  if (utility.startsWith("-")) return MOVES_ITS_BOX.test(utility.slice(1));
  return MOVES_ITS_BOX.test(utility) && NEGATIVE_BRACKETED_VALUE.test(utility);
}

describe("the bottom tab bar", () => {
  it("draws every tab inside its own link or button, the raised Secure icon included", () => {
    render(<MobileTabBar role="admin" onOpenMenu={() => {}} />);
    const bar = screen.getByRole("navigation", { name: "Primary" });
    expect(screen.getByRole("link", { name: "Secure" })).toBeTruthy();

    const controls = [...bar.querySelectorAll("a, button")];
    expect(controls.map((c) => c.textContent)).toEqual(["Overview", "Agents", "Secure", "Security", "Menu"]);
    const pulledOut = controls.flatMap((control) =>
      [...control.querySelectorAll("*")].flatMap((part) =>
        (part.getAttribute("class") ?? "")
          .split(/\s+/)
          .filter(pullsOut)
          .map((c) => `${control.textContent}: <${part.tagName.toLowerCase()}> ${c}`),
      ),
    );
    expect(pulledOut).toEqual([]);
  });

  it("reads a negative margin, offset or translate in each spelling as drawing outside the parent's box", () => {
    const outside = [
      "-mt-3.5",
      "mt-[-14px]",
      "translate-y-[-14px]",
      "max-lg:-mt-3.5",
      "max-lg:mt-[-14px]",
      "!-mt-3.5",
      "max-lg:!-mt-3.5",
      "[&:hover]:-mt-1",
      "-mx-2",
      "-top-1",
      "top-[-14px]",
      "-inset-y-1",
      "-start-2",
      "-translate-y-1/2",
      "mt-[calc(-1*14px)]",
    ];
    const inside = ["mt-1", "mx-auto", "max-w-full", "translate-y-0", "top-[14px]", "-z-10", "-rotate-45", "text-[10px]"];
    expect(outside.filter((c) => !pullsOut(c))).toEqual([]);
    expect(inside.filter(pullsOut)).toEqual([]);
  });
});

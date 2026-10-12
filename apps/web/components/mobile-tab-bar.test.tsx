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

// A class that draws an element outside its parent's box: a negative margin, a
// negative offset or a negative translate.
const PULLED_OUT = /^-(?:m[trblxy]?|top|right|bottom|left|inset(?:-[xy])?|translate-[xy])-/;

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
          .filter((c) => PULLED_OUT.test(c))
          .map((c) => `${control.textContent}: <${part.tagName.toLowerCase()}> ${c}`),
      ),
    );
    expect(pulledOut).toEqual([]);
  });
});

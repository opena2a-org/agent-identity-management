import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen, fireEvent, cleanup } from "@testing-library/react";
import type { ReactNode } from "react";
import RegisterPage from "./page";
import { HIT_AREA_FLOOR_PX, countTextRuns, hitAreaPx, textBelowFloor } from "@/tests/readability";

// Sign-up is read on a phone: no text on it renders below 14px at 375px, and each
// "Show password" control is a 44 by 44 hit area around an unchanged icon.
vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn() }) }));
vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("@/lib/api", () => ({ api: { register: vi.fn() } }));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("register page readability", () => {
  it("renders no text below 14px, before and after a submit with every field missing", () => {
    const { container } = render(<RegisterPage />);
    expect(screen.getByText(/Must be 8\+ characters/)).toBeTruthy();
    expect(countTextRuns(container)).toBeGreaterThan(20);
    expect(textBelowFloor(container)).toEqual([]);

    fireEvent.click(screen.getByRole("button", { name: "Create account" }));
    expect(screen.getByText("Email is required")).toBeTruthy();
    expect(screen.getByText("Password is required")).toBeTruthy();
    expect(textBelowFloor(container)).toEqual([]);
  });

  it("gives both Show password controls a 44px square hit area, the same icon and their name", () => {
    render(<RegisterPage />);
    const toggles = screen.getAllByRole("button", { name: "Show password" });
    expect(toggles).toHaveLength(2);
    for (const toggle of toggles) {
      const area = hitAreaPx(toggle);
      expect(area.width).toBeGreaterThanOrEqual(HIT_AREA_FLOOR_PX);
      expect(area.height).toBeGreaterThanOrEqual(HIT_AREA_FLOOR_PX);
      const icon = toggle.querySelector("svg");
      expect(icon?.classList.contains("h-5")).toBe(true);
      expect(icon?.classList.contains("w-5")).toBe(true);
    }

    fireEvent.click(toggles[0]);
    expect((document.getElementById("password") as HTMLInputElement).type).toBe("text");
    const hide = screen.getByRole("button", { name: "Hide password" });
    expect(hide).toBe(toggles[0]);
    expect(hitAreaPx(hide).height).toBeGreaterThanOrEqual(HIT_AREA_FLOOR_PX);
  });
});

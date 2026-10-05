import { describe, it, expect, vi, afterEach, beforeEach } from "vitest";
import { render, screen, fireEvent, cleanup } from "@testing-library/react";
import type { ReactNode } from "react";
import ChangePasswordPage from "./page";
import { HIT_AREA_FLOOR_PX, hitAreaPx } from "@/tests/readability";

// Each "Show password" control on the forced password change is a 44 by 44 hit
// area around an unchanged icon, and keeps its accessible name in both states.
vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn() }) }));
vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("@/components/sidebar", () => ({ AimLogo: () => null }));
vi.mock("@/lib/api", () => ({ api: { changePassword: vi.fn() } }));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

beforeEach(() => {
  localStorage.setItem("temp_user_email", "user@example.test");
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
  localStorage.clear();
});

describe("change password page tap targets", () => {
  it("gives all three Show password controls a 44px square hit area, the same icon and their name", () => {
    render(<ChangePasswordPage />);
    const toggles = screen.getAllByRole("button", { name: "Show password" });
    expect(toggles).toHaveLength(3);
    for (const toggle of toggles) {
      const area = hitAreaPx(toggle);
      expect(area.width).toBeGreaterThanOrEqual(HIT_AREA_FLOOR_PX);
      expect(area.height).toBeGreaterThanOrEqual(HIT_AREA_FLOOR_PX);
      const icon = toggle.querySelector("svg");
      expect(icon?.classList.contains("h-4")).toBe(true);
      expect(icon?.classList.contains("w-4")).toBe(true);
    }

    fireEvent.click(toggles[1]);
    expect((screen.getByLabelText("New password") as HTMLInputElement).type).toBe("text");
    const hide = screen.getByRole("button", { name: "Hide password" });
    expect(hide).toBe(toggles[1]);
    expect(hitAreaPx(hide).height).toBeGreaterThanOrEqual(HIT_AREA_FLOOR_PX);
  });
});

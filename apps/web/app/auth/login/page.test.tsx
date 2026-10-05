import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen, fireEvent, cleanup } from "@testing-library/react";
import type { ReactNode } from "react";
import LoginPage from "./page";
import { HIT_AREA_FLOOR_PX, hitAreaPx } from "@/tests/readability";

// The "Show password" control on sign-in is a 44 by 44 hit area around an
// unchanged icon, and keeps its accessible name in both states.
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
  useSearchParams: () => new URLSearchParams(),
}));
vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("@/components/sidebar", () => ({ AimLogo: () => null }));
vi.mock("@/lib/api", () => ({ api: { loginWithPassword: vi.fn() } }));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("login page tap targets", () => {
  it("gives Show password a 44px square hit area, the same icon and its name", async () => {
    render(<LoginPage />);
    const toggle = await screen.findByRole("button", { name: "Show password" });
    const area = hitAreaPx(toggle);
    expect(area.width).toBeGreaterThanOrEqual(HIT_AREA_FLOOR_PX);
    expect(area.height).toBeGreaterThanOrEqual(HIT_AREA_FLOOR_PX);
    const icon = toggle.querySelector("svg");
    expect(icon?.classList.contains("h-4")).toBe(true);
    expect(icon?.classList.contains("w-4")).toBe(true);

    fireEvent.click(toggle);
    expect((screen.getByLabelText("Password") as HTMLInputElement).type).toBe("text");
    const hide = screen.getByRole("button", { name: "Hide password" });
    expect(hide).toBe(toggle);
    expect(hitAreaPx(hide).width).toBeGreaterThanOrEqual(HIT_AREA_FLOOR_PX);
  });
});

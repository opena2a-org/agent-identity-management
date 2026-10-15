import { describe, it, expect, vi, afterEach, beforeEach, type Mock } from "vitest";
import { render, screen, fireEvent, cleanup, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import ResetPasswordPage from "./page";
import { api } from "@/lib/api";
import { HIT_AREA_FLOOR_PX, countTextRuns, hitAreaPx, textBelowFloor } from "@/tests/readability";

// The reset page reads the token from the link and posts it with the new
// password. Whatever else a link carries (here an email address, which no
// link of ours has) stays out of the request and out of the console.
const linkQuery = "token=fixture-token&email=reset.probe%40example.test";
const push = vi.fn();
vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(linkQuery),
  useRouter: () => ({ push }),
}));
vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("@/components/sidebar", () => ({ AimLogo: () => null }));
vi.mock("@/lib/api", () => ({ api: { resetPassword: vi.fn() } }));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const consoleMethods = ["log", "info", "warn", "error", "debug"] as const;
let spies: Mock[] = [];

beforeEach(() => {
  window.history.replaceState({}, "", "/auth/reset-password?" + linkQuery);
  spies = consoleMethods.map((m) => vi.spyOn(console, m).mockImplementation(() => {}) as unknown as Mock);
  (api.resetPassword as Mock).mockResolvedValue({ success: true });
});

afterEach(() => {
  cleanup();
  spies.forEach((s) => s.mockRestore());
  vi.clearAllMocks();
  window.history.replaceState({}, "", "/");
});

describe("reset password page", () => {
  it("posts the token and the new password, nothing else from the link", async () => {
    const newPassword = "Aa1!aaaa";
    render(<ResetPasswordPage />);
    fireEvent.change(screen.getByLabelText("New password"), { target: { value: newPassword } });
    fireEvent.change(screen.getByLabelText("Confirm password"), { target: { value: newPassword } });
    fireEvent.click(screen.getByRole("button", { name: "Set new password" }));

    await waitFor(() => expect(api.resetPassword).toHaveBeenCalledTimes(1));
    expect((api.resetPassword as Mock).mock.calls[0][0]).toEqual({
      resetToken: "fixture-token",
      newPassword,
      confirmPassword: newPassword,
    });
    const serialized = JSON.stringify((api.resetPassword as Mock).mock.calls);
    expect(serialized).not.toContain("reset.probe");
    expect(serialized).not.toContain("%40");

    const consoleArgs = JSON.stringify(spies.flatMap((s) => s.mock.calls));
    expect(consoleArgs).not.toContain("fixture-token");
    expect(consoleArgs).not.toContain("reset.probe");
  });
});

// The reset page is opened from an email, usually on a phone: no text in any of
// its three states renders below 14px at 375px, and each "Show password" control
// is a 44 by 44 hit area around an unchanged icon.
describe("reset password page readability", () => {
  it("renders no text below 14px on the form, before and after a submit with both fields empty", () => {
    const { container } = render(<ResetPasswordPage />);
    expect(countTextRuns(container)).toBeGreaterThan(5);
    expect(textBelowFloor(container)).toEqual([]);

    fireEvent.click(screen.getByRole("button", { name: "Set new password" }));
    expect(screen.getByText("Password is required")).toBeTruthy();
    expect(screen.getByText("Please confirm your password")).toBeTruthy();
    expect(textBelowFloor(container)).toEqual([]);
  });

  it("renders no text below 14px once the password is updated", async () => {
    const { container } = render(<ResetPasswordPage />);
    fireEvent.change(screen.getByLabelText("New password"), { target: { value: "Aa1!aaaa" } });
    fireEvent.change(screen.getByLabelText("Confirm password"), { target: { value: "Aa1!aaaa" } });
    fireEvent.click(screen.getByRole("button", { name: "Set new password" }));

    expect(await screen.findByText("Taking you to sign in in a few seconds.")).toBeTruthy();
    expect(textBelowFloor(container)).toEqual([]);
  });

  it("renders no text below 14px when the link no longer works", async () => {
    (api.resetPassword as Mock).mockRejectedValue({ message: "Reset token is invalid or expired" });
    const { container } = render(<ResetPasswordPage />);
    fireEvent.change(screen.getByLabelText("New password"), { target: { value: "Aa1!aaaa" } });
    fireEvent.change(screen.getByLabelText("Confirm password"), { target: { value: "Aa1!aaaa" } });
    fireEvent.click(screen.getByRole("button", { name: "Set new password" }));

    expect(await screen.findByText("This reset link no longer works")).toBeTruthy();
    expect(screen.getByText("Use the link within 24 hours of receiving it.")).toBeTruthy();
    expect(textBelowFloor(container)).toEqual([]);
  });

  it("gives both Show password controls a 44px square hit area, the same icon and their name", () => {
    render(<ResetPasswordPage />);
    const toggles = screen.getAllByRole("button", { name: "Show password" });
    expect(toggles).toHaveLength(2);
    for (const toggle of toggles) {
      const area = hitAreaPx(toggle);
      expect(area.width).toBeGreaterThanOrEqual(HIT_AREA_FLOOR_PX);
      expect(area.height).toBeGreaterThanOrEqual(HIT_AREA_FLOOR_PX);
      const icon = toggle.querySelector("svg");
      expect(icon?.classList.contains("h-4")).toBe(true);
      expect(icon?.classList.contains("w-4")).toBe(true);
    }

    fireEvent.click(toggles[0]);
    expect((screen.getByLabelText("New password") as HTMLInputElement).type).toBe("text");
    const hide = screen.getByRole("button", { name: "Hide password" });
    expect(hide).toBe(toggles[0]);
    expect(hitAreaPx(hide).width).toBeGreaterThanOrEqual(HIT_AREA_FLOOR_PX);
  });
});

import { describe, it, expect, vi, afterEach, beforeEach, type Mock } from "vitest";
import { render, screen, fireEvent, cleanup, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import ResetPasswordPage from "./page";
import { api } from "@/lib/api";

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

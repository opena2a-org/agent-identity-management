import { describe, it, expect, vi, afterEach, beforeEach, type Mock } from "vitest";
import { render, screen, fireEvent, cleanup } from "@testing-library/react";
import type { ReactNode } from "react";
import ForgotPasswordPage from "./page";
import { api } from "@/lib/api";
import { countTextRuns, textBelowFloor } from "@/tests/readability";

// The reset request is read on a phone: no text on it, in any of its states,
// renders below 14px at 375px.
vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("@/components/sidebar", () => ({ AimLogo: () => null }));
vi.mock("@/lib/api", () => ({ api: { forgotPassword: vi.fn() } }));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

beforeEach(() => {
  (api.forgotPassword as Mock).mockResolvedValue({ success: true });
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("forgot password page readability", () => {
  it("renders no text below 14px on the form, with its hint and with an error", () => {
    const { container } = render(<ForgotPasswordPage />);
    expect(screen.getByText("The address you sign in with.")).toBeTruthy();
    expect(screen.getByText(/does not say whether an account exists/)).toBeTruthy();
    expect(countTextRuns(container)).toBeGreaterThan(5);
    expect(textBelowFloor(container)).toEqual([]);

    const email = screen.getByLabelText("Email address");
    fireEvent.change(email, { target: { value: "not-an-address" } });
    fireEvent.blur(email);
    expect(screen.getByText("Invalid email address")).toBeTruthy();
    expect(textBelowFloor(container)).toEqual([]);
  });

  it("renders no text below 14px once the link is sent", async () => {
    const { container } = render(<ForgotPasswordPage />);
    fireEvent.change(screen.getByLabelText("Email address"), { target: { value: "user@example.test" } });
    fireEvent.click(screen.getByRole("button", { name: "Send reset link" }));
    expect(await screen.findByText("Next steps")).toBeTruthy();
    expect(screen.getByText(/Check your spam folder/)).toBeTruthy();
    expect(textBelowFloor(container)).toEqual([]);
  });
});

import { describe, it, expect, vi, afterEach, type Mock } from "vitest";
import { render, screen, fireEvent, cleanup, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import LoginPage from "./page";
import { api, type ApiRequestError } from "@/lib/api";
import { toast } from "sonner";
import { SIGN_IN_NEUTRAL_MESSAGE } from "@/lib/sign-in-failure";

// A sign-in failure is shown once, in the alert at the top of the card: no toast, no
// server text under the password field. Client-side validation keeps its field errors.
const push = vi.fn();
vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(""),
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
vi.mock("@/lib/api", () => ({ api: { loginWithPassword: vi.fn() } }));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() } }));

function refusal(status: number, message: string): ApiRequestError {
  const err = new Error(message) as ApiRequestError;
  err.status = status;
  return err;
}

function submit(email = "person@example.test", password = "not-the-password") {
  fireEvent.change(screen.getByLabelText("Email address"), { target: { value: email } });
  fireEvent.change(screen.getByLabelText("Password"), { target: { value: password } });
  fireEvent.click(screen.getByRole("button", { name: "Sign in" }));
}

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("sign-in failures", () => {
  it("shows a wrong password once, in the form-level alert, and marks both inputs", async () => {
    (api.loginWithPassword as Mock).mockRejectedValue(refusal(401, "Invalid email or password"));
    render(<LoginPage />);
    submit();

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain("Invalid email or password");
    expect(screen.getAllByText("Invalid email or password")).toHaveLength(1);
    expect(document.activeElement).toBe(alert);
    expect(toast.error).not.toHaveBeenCalled();
    expect(document.getElementById("password-error")).toBeNull();
    expect(screen.getByLabelText("Email address").getAttribute("aria-invalid")).toBe("true");
    expect(screen.getByLabelText("Password").getAttribute("aria-invalid")).toBe("true");
    expect(document.body.textContent).not.toMatch(/session expired/i);
  });

  it("shows any other server refusal in the alert without marking the inputs", async () => {
    const message = "Your account has been deactivated. Please contact your administrator for assistance.";
    (api.loginWithPassword as Mock).mockRejectedValue(refusal(401, message));
    render(<LoginPage />);
    submit();

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(message);
    expect(toast.error).not.toHaveBeenCalled();
    expect(screen.getByLabelText("Email address").getAttribute("aria-invalid")).toBe("false");
    expect(screen.getByLabelText("Password").getAttribute("aria-invalid")).toBe("false");
  });

  it("uses the neutral default when the server sent no words, and never claims a wrong password", async () => {
    (api.loginWithPassword as Mock).mockRejectedValueOnce(new TypeError("Failed to fetch"));
    render(<LoginPage />);
    submit();

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(SIGN_IN_NEUTRAL_MESSAGE);
    expect(document.body.textContent).not.toContain("Invalid email or password");
    expect(screen.getByLabelText("Password").getAttribute("aria-invalid")).toBe("false");

    cleanup();
    (api.loginWithPassword as Mock).mockRejectedValueOnce(refusal(502, "HTTP 502"));
    render(<LoginPage />);
    submit();
    expect((await screen.findByRole("alert")).textContent).toContain(SIGN_IN_NEUTRAL_MESSAGE);
  });

  it("keeps the alert until the next submit", async () => {
    (api.loginWithPassword as Mock).mockRejectedValueOnce(refusal(401, "Invalid email or password"));
    (api.loginWithPassword as Mock).mockReturnValueOnce(new Promise(() => {}));
    render(<LoginPage />);
    submit();
    await screen.findByRole("alert");

    fireEvent.change(screen.getByLabelText("Password"), { target: { value: "another-try" } });
    expect(screen.queryByRole("alert")).not.toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Sign in" }));
    await waitFor(() => expect(screen.queryByRole("alert")).toBeNull());
  });

  it("keeps client-side validation as field errors", async () => {
    render(<LoginPage />);
    submit("person@example.test", "");

    expect(await screen.findByText("Password is required")).toBeTruthy();
    expect(screen.queryByRole("alert")).toBeNull();
    expect(api.loginWithPassword).not.toHaveBeenCalled();
  });
});

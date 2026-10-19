import { describe, it, expect, vi, afterEach, beforeEach, type Mock } from "vitest";
import { render, screen, fireEvent, cleanup, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import DeleteAccountPage from "./page";
import { api, type ApiRequestError } from "@/lib/api";
import {
  INVALID_PASSWORD_CODE,
  INVALID_PASSWORD_MESSAGE,
  LAST_ACTIVE_ADMIN_CODE,
  LAST_ACTIVE_ADMIN_MESSAGE,
  SESSION_ENDED_MESSAGE,
} from "@/lib/account-deletion";
import { formatDateTime } from "@/lib/date-utils";
import { HIT_AREA_FLOOR_PX, countTextRuns, hitAreaPx, textBelowFloor } from "@/tests/readability";

// The page a signed-in user asks for the deletion of their own account on. It sends the
// password through api.requestAccountDeletion, says what each refusal means, and after an
// accepted request shows when the account is due for deletion. It is read on a phone: no
// text renders below 14px at 375px, and the Show password control is a 44 by 44 hit area.

const router = vi.hoisted(() => ({ replace: vi.fn(), push: vi.fn() }));
const session = vi.hoisted(() => ({ token: "stored-session" as string | null }));

vi.mock("next/navigation", () => ({ useRouter: () => router }));
vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("@/components/sidebar", () => ({ AimLogo: () => null }));
vi.mock("@/lib/jwt-payload", () => ({ decodeJwtPayload: () => ({ email: "person@example.test" }) }));
vi.mock("@/lib/api", () => ({
  api: { getToken: vi.fn(() => session.token), requestAccountDeletion: vi.fn() },
}));

const requestAccountDeletion = api.requestAccountDeletion as Mock;

const refusal = (status: number, message: string, code?: string) => {
  const err = new Error(message) as ApiRequestError;
  err.status = status;
  if (code) err.code = code;
  return err;
};

const accepted = {
  status: "deactivated" as const,
  deletionRequestedAt: "2026-10-19T12:30:00Z",
  deletionDueAt: "2026-11-17T12:30:00Z",
};

function submit(password: string) {
  fireEvent.change(screen.getByLabelText("Password"), { target: { value: password } });
  fireEvent.click(screen.getByRole("button", { name: "Delete my account" }));
}

beforeEach(() => {
  session.token = "stored-session";
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("delete account page", () => {
  it("names the signed-in account and sends a visitor without a session to sign-in, which returns here", () => {
    render(<DeleteAccountPage />);
    expect(screen.getByText("Signed in as person@example.test")).toBeTruthy();
    expect(router.replace).not.toHaveBeenCalled();
    cleanup();

    session.token = null;
    render(<DeleteAccountPage />);
    expect(router.replace).toHaveBeenCalledWith("/auth/login?returnUrl=%2Fauth%2Fdelete-account");
  });

  it("says that the organization is deleted with the account when no other user is left in it", () => {
    render(<DeleteAccountPage />);
    expect(
      screen.getByText(
        "If no other user is left in your organization then, the organization is deleted too, with its agents and MCP servers."
      )
    ).toBeTruthy();
  });

  it("asks for the password before it sends anything", () => {
    render(<DeleteAccountPage />);
    fireEvent.click(screen.getByRole("button", { name: "Delete my account" }));
    expect(screen.getByText("Enter your password")).toBeTruthy();
    expect(screen.getByLabelText("Password").getAttribute("aria-invalid")).toBe("true");
    expect(requestAccountDeletion).not.toHaveBeenCalled();
  });

  it("shows a refused password in the alert, marks the input and keeps the form", async () => {
    requestAccountDeletion.mockRejectedValueOnce(refusal(401, "Invalid password", INVALID_PASSWORD_CODE));
    render(<DeleteAccountPage />);
    submit("not-a-real-password");

    expect(await screen.findByText(INVALID_PASSWORD_MESSAGE)).toBeTruthy();
    expect(requestAccountDeletion).toHaveBeenCalledWith("not-a-real-password");
    expect(screen.getByRole("alert").textContent).toContain("Could not request the deletion");
    expect(screen.getByLabelText("Password").getAttribute("aria-invalid")).toBe("true");
    expect(screen.getByRole("button", { name: "Delete my account" })).toBeTruthy();
  });

  it("links the organization's only active admin to the Users page", async () => {
    requestAccountDeletion.mockRejectedValueOnce(
      refusal(409, "You are the only active admin of your organization", LAST_ACTIVE_ADMIN_CODE)
    );
    render(<DeleteAccountPage />);
    submit("not-a-real-password");

    expect(await screen.findByText(LAST_ACTIVE_ADMIN_MESSAGE)).toBeTruthy();
    expect(screen.getByRole("link", { name: "Open Users" }).getAttribute("href")).toBe("/dashboard/admin/users");
    expect(screen.getByLabelText("Password").getAttribute("aria-invalid")).toBe("false");
  });

  it("says the session has ended when the client signed it out", async () => {
    requestAccountDeletion.mockImplementationOnce(async () => {
      session.token = null;
      throw new Error("Unauthorized");
    });
    render(<DeleteAccountPage />);
    submit("not-a-real-password");

    expect(await screen.findByText(SESSION_ENDED_MESSAGE)).toBeTruthy();
  });

  it("shows when the account is due for deletion after an accepted request", async () => {
    requestAccountDeletion.mockResolvedValueOnce(accepted);
    render(<DeleteAccountPage />);
    submit("not-a-real-password");

    expect(await screen.findByRole("heading", { name: "Your account is deactivated" })).toBeTruthy();
    expect(screen.getByRole("status").textContent).toContain(
      `It is scheduled for deletion on ${formatDateTime(accepted.deletionDueAt)}, and you are signed out.`
    );
    expect(screen.getByRole("link", { name: "Go to sign in" }).getAttribute("href")).toBe("/auth/login");
    expect(screen.queryByLabelText("Password")).toBeNull();
  });
});

describe("delete account page readability", () => {
  it("renders no text below 14px on the form, with a refusal showing, and after an accepted request", async () => {
    const { container } = render(<DeleteAccountPage />);
    expect(countTextRuns(container)).toBeGreaterThan(5);
    expect(textBelowFloor(container)).toEqual([]);

    fireEvent.click(screen.getByRole("button", { name: "Delete my account" }));
    expect(screen.getByText("Enter your password")).toBeTruthy();
    expect(textBelowFloor(container)).toEqual([]);

    requestAccountDeletion.mockRejectedValueOnce(
      refusal(409, "You are the only active admin of your organization", LAST_ACTIVE_ADMIN_CODE)
    );
    submit("not-a-real-password");
    await screen.findByRole("link", { name: "Open Users" });
    expect(textBelowFloor(container)).toEqual([]);

    requestAccountDeletion.mockResolvedValueOnce(accepted);
    submit("not-a-real-password");
    await waitFor(() => expect(screen.getByRole("status")).toBeTruthy());
    expect(textBelowFloor(container)).toEqual([]);
  });

  it("gives the Show password control a 44px square hit area around an unchanged icon, in both states", () => {
    render(<DeleteAccountPage />);
    const toggle = screen.getByRole("button", { name: "Show password" });
    const area = hitAreaPx(toggle);
    expect(area.width).toBeGreaterThanOrEqual(HIT_AREA_FLOOR_PX);
    expect(area.height).toBeGreaterThanOrEqual(HIT_AREA_FLOOR_PX);
    const icon = toggle.querySelector("svg");
    expect(icon?.classList.contains("h-4")).toBe(true);
    expect(icon?.classList.contains("w-4")).toBe(true);

    fireEvent.click(toggle);
    expect((screen.getByLabelText("Password") as HTMLInputElement).type).toBe("text");
    expect(screen.getByRole("button", { name: "Hide password" })).toBe(toggle);
  });
});

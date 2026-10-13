import { describe, it, expect, vi, afterEach, beforeEach } from "vitest";
import { render, screen, cleanup, fireEvent, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import CapabilityRequestsPage from "./page";
import { api } from "@/lib/api";

const router = { replace: vi.fn(), push: vi.fn() };
vi.mock("next/navigation", () => ({
  useRouter: () => router,
  usePathname: () => "/dashboard/admin/capability-requests",
}));
vi.mock("@/components/auth-guard", () => ({
  AuthGuard: ({ children }: { children: ReactNode }) => <>{children}</>,
}));
vi.mock("@/lib/api", () => ({
  api: {
    getToken: vi.fn(),
    getCapabilityRequests: vi.fn(),
    getEnforcementSettings: vi.fn(),
    approveCapabilityRequest: vi.fn(),
    rejectCapabilityRequest: vi.fn(),
  },
}));

const base64Url = (value: object) =>
  btoa(JSON.stringify(value)).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
const ADMIN_TOKEN = `${base64Url({ alg: "none" })}.${base64Url({ role: "admin", sub: "admin-1" })}.sig`;

const pending = {
  id: "request-1",
  agentId: "agent-1",
  agentName: "billing-bot",
  agentDisplayName: "Billing Bot",
  capabilityType: "file:write",
  reason: "Writes the monthly invoice export",
  status: "pending" as const,
  requestedBy: "user-1",
  requestedByEmail: "dana@example.com",
  requestedAt: "2026-10-01T12:00:00Z",
};

function requestError(message: string, status: number, extra: object = {}) {
  return Object.assign(new Error(message), { status }, extra);
}

async function openApproveDialog() {
  render(<CapabilityRequestsPage />);
  fireEvent.click(await screen.findByRole("button", { name: "Approve" }));
  return screen.getByRole("alertdialog");
}

beforeEach(() => {
  vi.mocked(api.getToken).mockReturnValue(ADMIN_TOKEN);
  vi.mocked(api.getCapabilityRequests).mockResolvedValue([pending]);
  vi.mocked(api.getEnforcementSettings).mockResolvedValue({
    enforcementMode: "strict",
    description: "",
    explanation: "",
    impact: "",
  });
  vi.spyOn(console, "error").mockImplementation(() => {});
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
  vi.restoreAllMocks();
});

describe("CapabilityRequestsPage approve dialog", () => {
  it("states an approval the record path refused once, in a neutral alert inside the dialog, with when to try again", async () => {
    vi.mocked(api.approveCapabilityRequest).mockRejectedValueOnce(
      requestError("server line", 503, { code: "recordUnavailable", retryAfterSeconds: 300 })
    );
    const dialog = await openApproveDialog();

    fireEvent.click(screen.getByRole("button", { name: "Confirm" }));

    const refusal = await screen.findByRole("alert");
    expect(screen.getAllByRole("alert")).toHaveLength(1);
    expect(screen.getByRole("alertdialog")).toBe(dialog);
    expect(dialog.contains(refusal)).toBe(true);
    expect(refusal.querySelector("h5")?.textContent).toBe("Nothing was changed");
    expect(refusal.textContent).toContain("AIM did not approve this capability request");
    expect(refusal.textContent).toContain("the record could not be written");
    expect(refusal.textContent).toContain("Try again in 5 minutes.");
    expect(refusal.textContent).not.toContain("server line");
    // The default variant: the person did nothing wrong.
    expect(refusal.className).not.toContain("destructive");
    expect(document.body.textContent).not.toContain("Action Failed");
    expect(dialog.contains(document.activeElement)).toBe(true);
    expect(api.getCapabilityRequests).toHaveBeenCalledTimes(1);

    // The alert stays until the next attempt, which clears it.
    vi.mocked(api.approveCapabilityRequest).mockResolvedValueOnce({ message: "approved" });
    fireEvent.click(screen.getByRole("button", { name: "Confirm" }));
    await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
    expect(screen.queryByText("Nothing was changed")).toBeNull();
    expect(api.approveCapabilityRequest).toHaveBeenCalledTimes(2);
    expect(api.approveCapabilityRequest).toHaveBeenLastCalledWith("request-1");
    expect((await screen.findByRole("alert")).textContent).toContain(
      "Capability request for Billing Bot approved."
    );
  });

  it("clears the refusal when the dialog is closed and opened again", async () => {
    vi.mocked(api.approveCapabilityRequest).mockRejectedValueOnce(
      requestError("server line", 503, { code: "recordUnavailable" })
    );
    await openApproveDialog();
    fireEvent.click(screen.getByRole("button", { name: "Confirm" }));
    expect((await screen.findByRole("alert")).textContent).toContain("Try again in a few minutes.");

    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
    fireEvent.click(screen.getByRole("button", { name: "Approve" }));

    expect(screen.getByRole("alertdialog")).toBeTruthy();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("keeps any other failure on the page, as before", async () => {
    vi.mocked(api.approveCapabilityRequest).mockRejectedValueOnce(
      requestError("An internal error occurred. Please try again later.", 500)
    );
    await openApproveDialog();

    fireEvent.click(screen.getByRole("button", { name: "Confirm" }));

    await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
    const failure = await screen.findByRole("alert");
    expect(failure.textContent).toContain("Action Failed");
    expect(screen.queryByText("Nothing was changed")).toBeNull();
  });
});

import { describe, it, expect, vi, afterEach, beforeEach, type Mock } from "vitest";
import { render, screen, fireEvent, cleanup, act, waitFor } from "@testing-library/react";
import DevicePage from "./page";
import { api } from "@/lib/api";

/**
 * The consent page for the CLI device login (RFC 8628). A code on the URL is a
 * hint that the CLI printed, not an authorization: the page never approves on
 * load, names the client only from an allow-list, and approves on one explicit
 * click through the api client (bearer in localStorage, cookies HttpOnly).
 */
let searchParams = new URLSearchParams();
const push = vi.fn();
vi.mock("next/navigation", () => ({
  useSearchParams: () => searchParams,
  useRouter: () => ({ push }),
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() } }));
vi.mock("@/components/sidebar", () => ({ AimLogo: () => <span data-testid="logo" /> }));
vi.mock("@/lib/api", () => ({
  api: {
    getToken: vi.fn(),
    getDeviceVerification: vi.fn(),
    approveDevice: vi.fn(),
  },
}));

const pending = (clientId: string) => ({
  userCode: "BCDF-GHJK",
  clientId,
  scope: "",
  status: "pending",
  expiresAt: new Date(Date.now() + 600_000).toISOString(),
});

beforeEach(() => {
  searchParams = new URLSearchParams("user_code=BCDF-GHJK");
  (api.getToken as Mock).mockReturnValue("a.bearer.token");
  (api.getDeviceVerification as Mock).mockResolvedValue(pending("aim-sdk"));
  (api.approveDevice as Mock).mockResolvedValue({ message: "Device authorized successfully" });
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("device consent page", () => {
  it("shows the code from the URL and approves nothing on load", async () => {
    render(<DevicePage />);
    expect(await screen.findByText("BCDF-GHJK")).toBeTruthy();
    await waitFor(() => expect(api.getDeviceVerification).toHaveBeenCalledWith("BCDF-GHJK"));
    expect(api.approveDevice).not.toHaveBeenCalled();
    expect(screen.getByText(/only if you started this login yourself/i)).toBeTruthy();
  });

  it("approves only on the explicit click, through the api client", async () => {
    render(<DevicePage />);
    const button = await screen.findByRole("button", { name: /authorize aim-sdk/i });
    expect(api.approveDevice).not.toHaveBeenCalled();
    await act(async () => {
      fireEvent.click(button);
    });
    expect(api.approveDevice).toHaveBeenCalledTimes(1);
    expect(api.approveDevice).toHaveBeenCalledWith("BCDF-GHJK");
    expect(await screen.findByText(/signed in/i)).toBeTruthy();
  });

  it("sends a visitor without a session to login and keeps the code on the return URL", async () => {
    (api.getToken as Mock).mockReturnValue(null);
    render(<DevicePage />);
    const button = await screen.findByRole("button", { name: /sign in to authorize/i });
    fireEvent.click(button);
    expect(push).toHaveBeenCalledWith("/auth/login?returnUrl=%2Fdevice%3Fuser_code%3DBCDF-GHJK");
    expect(api.approveDevice).not.toHaveBeenCalled();
  });

  it("never renders a client name it does not know", async () => {
    (api.getDeviceVerification as Mock).mockResolvedValue(pending("<img src=x onerror=alert(1)>"));
    render(<DevicePage />);
    expect(await screen.findByText(/an unrecognised client/i)).toBeTruthy();
    expect(document.body.innerHTML).not.toContain("onerror");
    expect(screen.queryByText(/img src/)).toBeNull();
  });

  it("asks for the code when the URL carries none, and never approves from typing alone", async () => {
    searchParams = new URLSearchParams();
    render(<DevicePage />);
    const input = await screen.findByLabelText(/verification code/i);
    fireEvent.change(input, { target: { value: "bcdfghjk" } });
    expect((input as HTMLInputElement).value).toBe("BCDF-GHJK");
    expect(api.approveDevice).not.toHaveBeenCalled();
  });
});

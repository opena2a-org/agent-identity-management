import { describe, it, expect, vi, afterEach, beforeEach, type Mock } from "vitest";
import { render, screen, fireEvent, cleanup, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import LoginPage from "./page";
import { api, type ApiRequestError } from "@/lib/api";
import {
  OIDC_PROVIDER_ADDRESS_REFUSED_MESSAGE,
  OIDC_SIGN_IN_STORAGE_KEY,
  OIDC_SIGN_IN_UNAVAILABLE_MESSAGE,
} from "@/lib/oidc-sign-in";
import { HIT_AREA_FLOOR_PX, hitAreaPx, textBelowFloor } from "@/tests/readability";

// An organization's sign-in link, /auth/login?organization=<id>, adds sign-in through the
// organization's identity provider beside the password form. The button keeps the flow
// the server answers in this tab's sessionStorage, then sends the browser to the provider.

let params = new URLSearchParams();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
  useSearchParams: () => params,
}));
vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("@/components/sidebar", () => ({ AimLogo: () => null }));
vi.mock("@/lib/api", () => ({ api: { loginWithPassword: vi.fn(), startOidcSignIn: vi.fn() } }));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() } }));

const ORG = "2f1c9a4e-8b7d-4c3a-9e1f-0a2b3c4d5e6f";
const AUTHORIZATION_URL = "https://idp.example.test/authorize?client_id=aim&state=the-state";
const start = api.startOidcSignIn as Mock;
const assign = vi.fn();

const providerButton = () => screen.queryByRole("button", { name: "Sign in with your identity provider" });

function refusal(status: number, message: string): ApiRequestError {
  const err = new Error(message) as ApiRequestError;
  err.status = status;
  return err;
}

beforeEach(() => {
  sessionStorage.clear();
  vi.stubGlobal("location", { href: "http://localhost:3000/auth/login", assign });
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.clearAllMocks();
  params = new URLSearchParams();
});

describe("the sign-in page without an organization's sign-in link", () => {
  it("offers the password form only", () => {
    render(<LoginPage />);
    expect(screen.getByRole("button", { name: "Sign in" })).toBeTruthy();
    expect(providerButton()).toBeNull();
  });

  it("offers nothing more for an organization value that is not an id", () => {
    params = new URLSearchParams("organization=acme");
    render(<LoginPage />);
    expect(providerButton()).toBeNull();
  });
});

describe("the sign-in page with an organization's sign-in link", () => {
  it("keeps the flow in this tab and sends the browser to the provider", async () => {
    params = new URLSearchParams(`organization=${ORG}&returnUrl=%2Fdashboard%2Fagents`);
    start.mockResolvedValue({ authorizationUrl: AUTHORIZATION_URL, flow: `${ORG}.sealed-flow`, expiresAt: "2026-10-19T08:10:00Z" });

    render(<LoginPage />);
    fireEvent.click(providerButton()!);

    await waitFor(() => expect(assign).toHaveBeenCalledWith(AUTHORIZATION_URL));
    expect(start).toHaveBeenCalledWith(ORG);
    expect(JSON.parse(sessionStorage.getItem(OIDC_SIGN_IN_STORAGE_KEY) ?? "null")).toEqual({
      organizationId: ORG,
      flow: `${ORG}.sealed-flow`,
      returnUrl: "/dashboard/agents",
    });
    expect(screen.getByRole("button", { name: "Opening your identity provider..." })).toHaveProperty("disabled", true);
    // The password form stays.
    expect(screen.getByRole("button", { name: "Sign in" })).toBeTruthy();
  });

  it("gives the button back to a page the browser restores on the way back from the provider", async () => {
    params = new URLSearchParams(`organization=${ORG}`);
    start.mockResolvedValue({ authorizationUrl: AUTHORIZATION_URL, flow: `${ORG}.sealed-flow`, expiresAt: "2026-10-19T08:10:00Z" });

    render(<LoginPage />);
    fireEvent.click(providerButton()!);
    await waitFor(() => expect(assign).toHaveBeenCalledTimes(1));
    expect(providerButton()).toBeNull();

    // A page shown from the back-forward cache keeps its state; a fresh load (persisted false) is not one.
    fireEvent(window, Object.assign(new Event("pageshow"), { persisted: false }));
    expect(providerButton()).toBeNull();
    fireEvent(window, Object.assign(new Event("pageshow"), { persisted: true }));
    expect(providerButton()).toHaveProperty("disabled", false);
  });

  it("says so when the organization has no provider sign-in, and leaves the page", async () => {
    params = new URLSearchParams(`organization=${ORG}`);
    start.mockRejectedValue(refusal(404, "The organization has no OIDC sign-in"));

    render(<LoginPage />);
    fireEvent.click(providerButton()!);

    expect(await screen.findByText(OIDC_SIGN_IN_UNAVAILABLE_MESSAGE)).toBeTruthy();
    expect(assign).not.toHaveBeenCalled();
    expect(sessionStorage.getItem(OIDC_SIGN_IN_STORAGE_KEY)).toBeNull();
    expect(providerButton()).toHaveProperty("disabled", false);
  });

  it("sends the browser nowhere when the answered address is not an https URL", async () => {
    params = new URLSearchParams(`organization=${ORG}`);
    start.mockResolvedValue({ authorizationUrl: "javascript:void(0)", flow: `${ORG}.sealed-flow`, expiresAt: "2026-10-19T08:10:00Z" });

    render(<LoginPage />);
    fireEvent.click(providerButton()!);

    expect(await screen.findByText(OIDC_PROVIDER_ADDRESS_REFUSED_MESSAGE)).toBeTruthy();
    expect(assign).not.toHaveBeenCalled();
    expect(sessionStorage.getItem(OIDC_SIGN_IN_STORAGE_KEY)).toBeNull();
  });

  it("gives the button a 44px tall hit area and renders no text below 14px", () => {
    params = new URLSearchParams(`organization=${ORG}`);
    const { container } = render(<LoginPage />);
    expect(hitAreaPx(providerButton()!).height).toBeGreaterThanOrEqual(HIT_AREA_FLOOR_PX);
    expect(screen.getByText("or with your email and password")).toBeTruthy();
    expect(textBelowFloor(container)).toEqual([]);
  });
});

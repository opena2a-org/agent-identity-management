import { describe, it, expect, vi, afterEach, beforeEach, type Mock } from "vitest";
import { render, screen, fireEvent, cleanup, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import LoginPage from "./page";
import { api, type ApiRequestError } from "@/lib/api";
import { OIDC_SIGN_IN_STORAGE_KEY, PROVIDER_SIGN_IN_REQUIRED_MESSAGE } from "@/lib/oidc-sign-in";

// An organization can require its users who are not administrators to sign in through its
// identity provider. Their own password is then refused with 403, the code
// provider_sign_in_required and the organization's id, and the sign-in page offers that
// organization's provider in place of a dead end.

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
const LINK_ORG = "7d3b5e1a-4c2f-4a8b-9d6e-1f0a2b3c4d5e";
const SERVER_LINE =
  "Your organization requires sign-in through its identity provider. Sign in from your organization's sign-in link.";
const AUTHORIZATION_URL = "https://idp.example.test/authorize?client_id=aim&state=the-state";
const login = api.loginWithPassword as Mock;
const start = api.startOidcSignIn as Mock;
const assign = vi.fn();

const providerButton = () => screen.queryByRole("button", { name: "Sign in with your identity provider" });

// What lib/api.ts throws for the refusal: the server's line, its status and code, and the
// organization the body names.
function providerSignInRequired(organizationId?: string): ApiRequestError {
  const err = new Error(SERVER_LINE) as ApiRequestError;
  err.status = 403;
  err.code = "provider_sign_in_required";
  if (organizationId !== undefined) err.organizationId = organizationId;
  return err;
}

function submitPassword() {
  fireEvent.change(screen.getByLabelText("Email address"), { target: { value: "member@example.test" } });
  fireEvent.change(screen.getByLabelText("Password"), { target: { value: "the-members-password" } });
  fireEvent.click(screen.getByRole("button", { name: "Sign in" }));
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

describe("a password sign-in the organization's provider requirement refuses", () => {
  it("says so and offers the organization's provider, which starts its sign-in", async () => {
    params = new URLSearchParams("returnUrl=%2Fdashboard%2Fagents");
    login.mockRejectedValue(providerSignInRequired(ORG.toUpperCase()));
    start.mockResolvedValue({ authorizationUrl: AUTHORIZATION_URL, flow: `${ORG}.sealed-flow`, expiresAt: "2026-10-19T08:10:00Z" });

    render(<LoginPage />);
    expect(providerButton()).toBeNull();
    submitPassword();

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(PROVIDER_SIGN_IN_REQUIRED_MESSAGE);
    await waitFor(() => expect(document.activeElement).toBe(alert));
    // The password was right, so neither input is marked.
    expect(screen.getByLabelText("Email address").getAttribute("aria-invalid")).toBe("false");
    expect(screen.getByLabelText("Password").getAttribute("aria-invalid")).toBe("false");

    fireEvent.click(providerButton()!);
    await waitFor(() => expect(assign).toHaveBeenCalledWith(AUTHORIZATION_URL));
    expect(start).toHaveBeenCalledWith(ORG);
    expect(JSON.parse(sessionStorage.getItem(OIDC_SIGN_IN_STORAGE_KEY) ?? "null")).toEqual({
      organizationId: ORG,
      flow: `${ORG}.sealed-flow`,
      returnUrl: "/dashboard/agents",
    });
  });

  it("offers the refused account's organization in place of the one the link names", async () => {
    params = new URLSearchParams(`organization=${LINK_ORG}`);
    login.mockRejectedValue(providerSignInRequired(ORG));
    start.mockResolvedValue({ authorizationUrl: AUTHORIZATION_URL, flow: `${ORG}.sealed-flow`, expiresAt: "2026-10-19T08:10:00Z" });

    render(<LoginPage />);
    submitPassword();
    await screen.findByText(PROVIDER_SIGN_IN_REQUIRED_MESSAGE);
    expect(screen.getAllByRole("button", { name: "Sign in with your identity provider" })).toHaveLength(1);

    fireEvent.click(providerButton()!);
    await waitFor(() => expect(start).toHaveBeenCalledTimes(1));
    expect(start).toHaveBeenCalledWith(ORG);
  });

  it("shows the server's line and offers no provider when the refusal names no organization id", async () => {
    for (const organizationId of [undefined, "", "acme"]) {
      login.mockRejectedValueOnce(providerSignInRequired(organizationId));
      render(<LoginPage />);
      submitPassword();

      const alert = await screen.findByRole("alert");
      expect(alert.textContent).toContain(SERVER_LINE);
      expect(providerButton()).toBeNull();
      cleanup();
    }
  });

  it("offers no provider for a refusal with another code", async () => {
    const err = new Error("Refused for another reason") as ApiRequestError;
    err.status = 403;
    err.code = "another_refusal";
    err.organizationId = ORG;
    login.mockRejectedValue(err);

    render(<LoginPage />);
    submitPassword();

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain("Refused for another reason");
    expect(providerButton()).toBeNull();
  });
});

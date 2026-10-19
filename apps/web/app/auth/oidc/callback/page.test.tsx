import { describe, it, expect, vi, afterEach, beforeEach, type Mock } from "vitest";
import { StrictMode, type ReactNode } from "react";
import { render, screen, cleanup, waitFor } from "@testing-library/react";
import OidcCallbackPage from "./page";
import { api, type ApiRequestError } from "@/lib/api";
import { toast } from "sonner";
import { OIDC_SIGN_IN_NOT_STARTED_MESSAGE, OIDC_SIGN_IN_STORAGE_KEY, rememberOidcSignIn } from "@/lib/oidc-sign-in";
import { SIGN_IN_NEUTRAL_MESSAGE } from "@/lib/sign-in-failure";
import { countTextRuns, textBelowFloor } from "@/tests/readability";

// The page the organization's identity provider redirects the browser to. It completes
// only the sign-in this tab started: the flow the sign-in page kept in sessionStorage
// goes to the server with the code and state of the redirect, once.

const replace = vi.fn();
let params = new URLSearchParams();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace, push: vi.fn() }),
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
vi.mock("@/lib/api", () => ({ api: { completeOidcSignIn: vi.fn() } }));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() } }));

const ORG = "2f1c9a4e-8b7d-4c3a-9e1f-0a2b3c4d5e6f";
const FLOW = `${ORG}.sealed-flow`;
const complete = api.completeOidcSignIn as Mock;

// The provider's redirect: the page's address carries it, and useSearchParams reads it.
function arriveWith(query: string) {
  params = new URLSearchParams(query);
  window.history.replaceState(null, "", `/auth/oidc/callback?${query}`);
}

function refusal(status: number, message: string, code?: string): ApiRequestError {
  const err = new Error(message) as ApiRequestError;
  err.status = status;
  if (code) err.code = code;
  return err;
}

const signedIn = {
  accessToken: "oidc.access.value",
  refreshToken: "oidc.refresh.value",
  user: { id: "u-1", email: "person@example.test", name: "Person", role: "viewer", organizationId: ORG },
};

beforeEach(() => {
  sessionStorage.clear();
  localStorage.clear();
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
  window.history.replaceState(null, "", "/");
});

describe("the identity provider's redirect", () => {
  it("completes the sign-in this tab started and goes to the page it started from", async () => {
    rememberOidcSignIn({ organizationId: ORG, flow: FLOW, returnUrl: "/dashboard/agents" });
    arriveWith("code=the-code&state=the-state");
    complete.mockResolvedValue(signedIn);

    render(<OidcCallbackPage />);

    await waitFor(() => expect(replace).toHaveBeenCalledWith("/dashboard/agents"));
    expect(complete).toHaveBeenCalledTimes(1);
    expect(complete).toHaveBeenCalledWith({ code: "the-code", state: "the-state", flow: FLOW });
    expect(toast.success).toHaveBeenCalledWith("Signed in");
    expect(sessionStorage.getItem(OIDC_SIGN_IN_STORAGE_KEY)).toBeNull();
    // The spent code and state are gone from the address bar.
    expect(window.location.pathname).toBe("/auth/oidc/callback");
    expect(window.location.search).toBe("");
  });

  it("sends the flow once when the development build runs the effect twice", async () => {
    rememberOidcSignIn({ organizationId: ORG, flow: FLOW, returnUrl: "/dashboard" });
    arriveWith("code=the-code&state=the-state");
    complete.mockResolvedValue(signedIn);

    render(
      <StrictMode>
        <OidcCallbackPage />
      </StrictMode>
    );

    await waitFor(() => expect(replace).toHaveBeenCalledWith("/dashboard"));
    expect(complete).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("completes nothing in a tab that did not start a sign-in", async () => {
    // A link to this page carrying someone else's code and state signs no one in here.
    arriveWith("code=another-code&state=another-state");

    render(<OidcCallbackPage />);

    expect(await screen.findByText(OIDC_SIGN_IN_NOT_STARTED_MESSAGE)).toBeTruthy();
    expect(complete).not.toHaveBeenCalled();
    expect(replace).not.toHaveBeenCalled();
    expect(screen.getByRole("link", { name: "Back to sign in" }).getAttribute("href")).toBe("/auth/login");
  });

  it("shows the server's refusal and links the organization's sign-in page", async () => {
    rememberOidcSignIn({ organizationId: ORG, flow: FLOW, returnUrl: "/dashboard/agents" });
    arriveWith("code=the-code&state=the-state");
    complete.mockRejectedValue(refusal(401, "Sign-in through the identity provider was refused", "no_user"));

    render(<OidcCallbackPage />);

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(
      "No account of this organization has your email address. Ask your administrator to add you."
    );
    await waitFor(() => expect(document.activeElement).toBe(alert));
    expect(screen.getByRole("heading", { level: 1 }).textContent).toBe("Sign-in did not complete");
    expect(screen.getByRole("link", { name: "Back to sign in" }).getAttribute("href")).toBe(
      `/auth/login?organization=${ORG}&returnUrl=%2Fdashboard%2Fagents`
    );
    expect(replace).not.toHaveBeenCalled();
    expect(sessionStorage.getItem(OIDC_SIGN_IN_STORAGE_KEY)).toBeNull();
  });

  it("shows a fixed line for the provider's error, never the description in the address", async () => {
    rememberOidcSignIn({ organizationId: ORG, flow: FLOW, returnUrl: "/dashboard" });
    arriveWith("error=access_denied&error_description=Call%20555-0100%20to%20unlock%20your%20account&state=the-state");

    const { container } = render(<OidcCallbackPage />);

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain("The sign-in was cancelled or refused at your identity provider.");
    expect(container.textContent).not.toContain("555-0100");
    expect(complete).not.toHaveBeenCalled();
    expect(sessionStorage.getItem(OIDC_SIGN_IN_STORAGE_KEY)).toBeNull();
    expect(screen.getByRole("link", { name: "Back to sign in" }).getAttribute("href")).toBe(
      `/auth/login?organization=${ORG}`
    );
  });

  it("opens nothing when the answer carries no login pair", async () => {
    rememberOidcSignIn({ organizationId: ORG, flow: FLOW, returnUrl: "/dashboard" });
    arriveWith("code=the-code&state=the-state");
    complete.mockResolvedValue({});

    render(<OidcCallbackPage />);

    expect(await screen.findByText(SIGN_IN_NEUTRAL_MESSAGE)).toBeTruthy();
    expect(replace).not.toHaveBeenCalled();
    expect(toast.success).not.toHaveBeenCalled();
  });

  it("does not send a redirect that carries no code", async () => {
    rememberOidcSignIn({ organizationId: ORG, flow: FLOW, returnUrl: "/dashboard" });
    arriveWith("state=the-state");

    render(<OidcCallbackPage />);

    expect(await screen.findByText("The identity provider's answer carries no sign-in code. Start the sign-in again.")).toBeTruthy();
    expect(complete).not.toHaveBeenCalled();
  });

  it("takes an account that must change its password to the password change", async () => {
    rememberOidcSignIn({ organizationId: ORG, flow: FLOW, returnUrl: "/dashboard" });
    arriveWith("code=the-code&state=the-state");
    complete.mockResolvedValue({ ...signedIn, user: { ...signedIn.user, forcePasswordChange: true } });

    render(<OidcCallbackPage />);

    await waitFor(() => expect(replace).toHaveBeenCalledWith("/auth/change-password"));
    expect(localStorage.getItem("temp_user_id")).toBe("u-1");
    expect(localStorage.getItem("temp_user_email")).toBe("person@example.test");
    expect(toast.success).not.toHaveBeenCalled();
  });

  it("renders no text below 14px, while it works and after a refusal", async () => {
    rememberOidcSignIn({ organizationId: ORG, flow: FLOW, returnUrl: "/dashboard" });
    arriveWith("code=the-code&state=the-state");
    let refuse: (err: unknown) => void = () => {};
    complete.mockReturnValue(new Promise((_, reject) => (refuse = reject)));

    const { container } = render(<OidcCallbackPage />);

    expect(await screen.findByRole("status")).toBeTruthy();
    expect(countTextRuns(container)).toBeGreaterThan(2);
    expect(textBelowFloor(container)).toEqual([]);

    refuse(refusal(401, "Sign-in through the identity provider was refused", "flow_expired"));
    expect(await screen.findByText("The sign-in expired before it completed. Start the sign-in again.")).toBeTruthy();
    expect(textBelowFloor(container)).toEqual([]);
  });
});

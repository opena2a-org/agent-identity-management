import { describe, it, expect, vi, afterEach, beforeEach, type Mock } from "vitest";
import { render, screen, fireEvent, cleanup, waitFor, within } from "@testing-library/react";
import type { ReactNode } from "react";
import IdentityProviderPage from "./page";
import { api, type ApiRequestError, type OidcSignInConfiguration } from "@/lib/api";
import { decodeJwtPayload } from "@/lib/jwt-payload";
import { HIT_AREA_FLOOR_PX, hitAreaPx, textBelowFloor } from "@/tests/readability";

// The administrator's page for the organization's OpenID Connect provider, over
// GET, PUT and DELETE /api/v1/admin/oidc-sign-in.

vi.mock("@/components/auth-guard", () => ({ AuthGuard: ({ children }: { children: ReactNode }) => <>{children}</> }));
vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("@/lib/api", () => ({
  api: {
    getOidcSignInConfiguration: vi.fn(),
    saveOidcSignInConfiguration: vi.fn(),
    removeOidcSignInConfiguration: vi.fn(),
    getToken: vi.fn(() => "the-session-token"),
  },
}));
vi.mock("@/lib/jwt-payload", () => ({ decodeJwtPayload: vi.fn() }));

const ORG = "2f1c9a4e-8b7d-4c3a-9e1f-0a2b3c4d5e6f";
const REDIRECT = "https://aim.example.test/auth/oidc/callback";
const ISSUER = "https://idp.example.test";
const SIGN_IN_LINK = `http://localhost:3000/auth/login?organization=${ORG}`;

const notConfigured: OidcSignInConfiguration = { configured: false, redirectUrl: REDIRECT };
const stored: OidcSignInConfiguration = {
  configured: true,
  issuer: ISSUER,
  clientId: "aim-dashboard",
  clientSecretSet: true,
  provisioning: { enabled: false },
  requireProviderSignIn: false,
  redirectUrl: REDIRECT,
  updatedBy: "5c8e1f3a-2d4b-4e6f-9a7c-0b1d2e3f4a5b",
  updatedAt: "2026-10-19T08:00:00Z",
};

const get = api.getOidcSignInConfiguration as Mock;
const save = api.saveOidcSignInConfiguration as Mock;
const remove = api.removeOidcSignInConfiguration as Mock;

const field = (label: string) => screen.getByLabelText(label) as HTMLInputElement;
const type = (label: string, value: string) => fireEvent.change(field(label), { target: { value } });
const saveButton = () => screen.getByRole("button", { name: "Save" });

function refusal(status: number, message: string): ApiRequestError {
  const err = new Error(message) as ApiRequestError;
  err.status = status;
  return err;
}

beforeEach(() => {
  (decodeJwtPayload as Mock).mockReturnValue({ organization_id: ORG, role: "admin" });
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
  Reflect.deleteProperty(navigator, "clipboard");
});

describe("the identity provider page", () => {
  it("stores the provider of an organization that has none, and then offers its sign-in link", async () => {
    get.mockResolvedValue(notConfigured);
    save.mockResolvedValue({
      ...stored,
      provisioning: { enabled: true, role: "member", emailDomains: ["example.com", "example.org"] },
      requireProviderSignIn: true,
    });
    render(<IdentityProviderPage />);

    expect(await screen.findByText(REDIRECT)).toBeTruthy();
    expect(screen.queryByText(SIGN_IN_LINK)).toBeNull();
    expect(screen.queryByRole("button", { name: "Remove identity provider" })).toBeNull();

    type("Issuer URL", ` ${ISSUER} `);
    type("Client ID", "aim-dashboard");
    type("Client secret", "not-a-real-client-secret");
    fireEvent.click(screen.getByLabelText("Create an account at the first sign-in"));
    const role = screen.getByLabelText("Role of a created account") as HTMLSelectElement;
    expect(Array.from(role.options).map((o) => o.value)).toEqual(["viewer", "member", "manager"]);
    expect(role.value).toBe("viewer");
    fireEvent.change(role, { target: { value: "member" } });
    type("Email domains", "Example.com, example.org");
    fireEvent.click(screen.getByLabelText("Require sign-in through the provider"));
    fireEvent.click(saveButton());

    await waitFor(() => expect(save).toHaveBeenCalledTimes(1));
    expect(save).toHaveBeenCalledWith({
      issuer: ISSUER,
      clientId: "aim-dashboard",
      clientSecret: "not-a-real-client-secret",
      provisioning: { enabled: true, role: "member", emailDomains: ["Example.com", "example.org"] },
      requireProviderSignIn: true,
    });

    expect(await screen.findByText("The identity provider is saved.")).toBeTruthy();
    // The form now holds the stored provider, and never the secret.
    expect(field("Client secret").value).toBe("");
    expect(field("Email domains").value).toBe("example.com, example.org");
    expect(screen.getByText("A client secret is stored. Leave this empty to keep it, or enter a new one to replace it.")).toBeTruthy();
    expect(screen.getByText(SIGN_IN_LINK)).toBeTruthy();
    expect(screen.getByRole("button", { name: "Remove identity provider" })).toBeTruthy();
  });

  it("changes a setting of the stored provider and leaves the client secret empty, which keeps it", async () => {
    get.mockResolvedValue(stored);
    save.mockResolvedValue({ ...stored, requireProviderSignIn: true });
    render(<IdentityProviderPage />);

    expect(await screen.findByDisplayValue(ISSUER)).toBeTruthy();
    expect(field("Client ID").value).toBe("aim-dashboard");
    expect(field("Client secret").value).toBe("");
    expect(field("Client secret").placeholder).toBe("Stored");
    expect(screen.getByText(SIGN_IN_LINK)).toBeTruthy();

    fireEvent.click(screen.getByLabelText("Require sign-in through the provider"));
    fireEvent.click(saveButton());

    await waitFor(() => expect(save).toHaveBeenCalledTimes(1));
    expect(save).toHaveBeenCalledWith({
      issuer: ISSUER,
      clientId: "aim-dashboard",
      clientSecret: "",
      provisioning: { enabled: false },
      requireProviderSignIn: true,
    });
    expect(await screen.findByText("The identity provider is saved.")).toBeTruthy();
    expect((screen.getByLabelText("Require sign-in through the provider") as HTMLInputElement).checked).toBe(true);
  });

  it("asks for the client secret when the client ID or the issuer changes", async () => {
    get.mockResolvedValue(stored);
    render(<IdentityProviderPage />);
    await screen.findByDisplayValue(ISSUER);

    type("Client ID", "another-client");
    expect(field("Client secret").placeholder).toBe("");
    expect(
      screen.getByText(
        "The stored client secret is kept only with the saved issuer and client ID. Enter the client secret for the new ones."
      )
    ).toBeTruthy();
    fireEvent.click(saveButton());

    expect(await screen.findByText("Enter the client secret. The stored one is kept only for the same issuer and client ID.")).toBeTruthy();
    expect(save).not.toHaveBeenCalled();
    expect(document.activeElement).toBe(field("Client secret"));
    expect(field("Client secret").getAttribute("aria-invalid")).toBe("true");

    type("Client ID", "aim-dashboard");
    type("Issuer URL", `${ISSUER}/tenant`);
    fireEvent.click(saveButton());
    expect(save).not.toHaveBeenCalled();

    type("Client secret", "not-a-real-client-secret");
    save.mockResolvedValue({ ...stored, issuer: `${ISSUER}/tenant` });
    fireEvent.click(saveButton());
    await waitFor(() => expect(save).toHaveBeenCalledTimes(1));
    expect(save.mock.calls[0][0]).toMatchObject({ issuer: `${ISSUER}/tenant`, clientSecret: "not-a-real-client-secret" });
  });

  it("checks the fields before it saves and moves focus to the first one to correct", async () => {
    get.mockResolvedValue(notConfigured);
    render(<IdentityProviderPage />);
    await screen.findByText(REDIRECT);

    fireEvent.click(saveButton());
    expect(await screen.findByText("Enter your provider's issuer URL.")).toBeTruthy();
    expect(screen.getByText("Enter the client ID your provider registered for this deployment.")).toBeTruthy();
    expect(screen.getByText("Enter the client secret your provider issued.")).toBeTruthy();
    expect(document.activeElement).toBe(field("Issuer URL"));
    expect(field("Issuer URL").getAttribute("aria-describedby")).toBe("idp-issuer-error idp-issuer-hint");

    type("Issuer URL", "http://idp.example.test");
    type("Client ID", "aim-dashboard");
    type("Client secret", "not-a-real-client-secret");
    fireEvent.click(screen.getByLabelText("Create an account at the first sign-in"));
    fireEvent.click(saveButton());
    expect(await screen.findByText("The issuer must be an https URL, such as https://idp.example.com.")).toBeTruthy();
    expect(screen.getByText("Enter at least one email domain, such as example.com.")).toBeTruthy();

    type("Issuer URL", ISSUER);
    type("Email domains", Array.from({ length: 21 }, (_, i) => `d${i}.example.com`).join(", "));
    fireEvent.click(saveButton());
    expect(await screen.findByText("Enter at most 20 email domains.")).toBeTruthy();
    expect(document.activeElement).toBe(field("Email domains"));
    expect(save).not.toHaveBeenCalled();
  });

  it("shows the server's refusal of a setting as it is, and keeps what was typed", async () => {
    get.mockResolvedValue(notConfigured);
    save.mockRejectedValue(
      refusal(400, "The issuer's configuration document could not be read, or names an endpoint that is not an https URL")
    );
    render(<IdentityProviderPage />);
    await screen.findByText(REDIRECT);

    type("Issuer URL", ISSUER);
    type("Client ID", "aim-dashboard");
    type("Client secret", "not-a-real-client-secret");
    fireEvent.click(saveButton());

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(
      "The issuer's configuration document could not be read, or names an endpoint that is not an https URL"
    );
    expect(alert.textContent).toContain("Correct the setting and save again.");
    expect(field("Issuer URL").value).toBe(ISSUER);
    expect(field("Client secret").value).toBe("not-a-real-client-secret");
    expect(screen.queryByText(SIGN_IN_LINK)).toBeNull();
  });

  it("describes a failure that is not a refusal of a setting from its status alone", async () => {
    get.mockResolvedValue(stored);
    save.mockRejectedValue(refusal(500, "the database said something internal"));
    render(<IdentityProviderPage />);
    await screen.findByDisplayValue(ISSUER);

    fireEvent.click(saveButton());
    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain("AIM could not save the identity provider.");
    expect(alert.textContent).not.toContain("the database said something internal");
  });

  it("removes the provider after a confirmation, and offers the empty form", async () => {
    get.mockResolvedValue({ ...stored, requireProviderSignIn: true });
    remove.mockResolvedValue(undefined);
    render(<IdentityProviderPage />);
    await screen.findByDisplayValue(ISSUER);

    fireEvent.click(screen.getByRole("button", { name: "Remove identity provider" }));
    const confirm = await screen.findByRole("alertdialog");
    expect(confirm.textContent).toContain("the requirement to sign in through it is removed");
    expect(confirm.textContent).toContain("Accounts created at sign-in have no password.");
    expect(remove).not.toHaveBeenCalled();

    fireEvent.click(within(confirm).getByRole("button", { name: "Remove" }));
    await waitFor(() => expect(remove).toHaveBeenCalledTimes(1));
    expect(await screen.findByText("The identity provider is removed.")).toBeTruthy();
    expect(field("Issuer URL").value).toBe("");
    expect((screen.getByLabelText("Require sign-in through the provider") as HTMLInputElement).checked).toBe(false);
    expect(screen.queryByRole("button", { name: "Remove identity provider" })).toBeNull();
    expect(screen.queryByText(SIGN_IN_LINK)).toBeNull();
    expect(screen.getByText(REDIRECT)).toBeTruthy();
  });

  it("removes nothing when the confirmation is cancelled", async () => {
    get.mockResolvedValue(stored);
    render(<IdentityProviderPage />);
    await screen.findByDisplayValue(ISSUER);

    fireEvent.click(screen.getByRole("button", { name: "Remove identity provider" }));
    const confirm = await screen.findByRole("alertdialog");
    fireEvent.click(within(confirm).getByRole("button", { name: "Cancel" }));
    await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
    expect(remove).not.toHaveBeenCalled();
    expect(field("Issuer URL").value).toBe(ISSUER);
  });

  it("says why the settings did not load, and loads them again", async () => {
    get.mockRejectedValueOnce(new TypeError("Failed to fetch")).mockResolvedValueOnce(notConfigured);
    render(<IdentityProviderPage />);

    expect(await screen.findByText("AIM could not be reached, so it did not load the identity provider settings.")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(await screen.findByText(REDIRECT)).toBeTruthy();
    expect(get).toHaveBeenCalledTimes(2);
  });

  it("copies the redirect URI and the sign-in link, and says when the browser does not", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
    get.mockResolvedValue(stored);
    render(<IdentityProviderPage />);
    await screen.findByDisplayValue(ISSUER);

    fireEvent.click(screen.getByRole("button", { name: "Copy the redirect URI" }));
    expect(await screen.findByText("The redirect URI is copied.")).toBeTruthy();
    expect(writeText).toHaveBeenCalledWith(REDIRECT);

    fireEvent.click(screen.getByRole("button", { name: "Copy the sign-in link" }));
    expect(await screen.findByText("The sign-in link is copied.")).toBeTruthy();
    expect(writeText).toHaveBeenLastCalledWith(SIGN_IN_LINK);

    writeText.mockRejectedValueOnce(new Error("denied"));
    fireEvent.click(screen.getByRole("button", { name: "Copy the redirect URI" }));
    expect(await screen.findByText("The browser did not copy it. Select the text and copy it.")).toBeTruthy();
  });

  it("offers no sign-in link when the session names no organization", async () => {
    (decodeJwtPayload as Mock).mockReturnValue(null);
    get.mockResolvedValue(stored);
    render(<IdentityProviderPage />);
    await screen.findByDisplayValue(ISSUER);

    expect(screen.queryByRole("heading", { name: "Sign-in link" })).toBeNull();
    expect(screen.getByRole("button", { name: "Remove identity provider" })).toBeTruthy();
  });

  it("renders no text below 14px and gives every button a 44px tall hit area", async () => {
    get.mockResolvedValue({ ...stored, provisioning: { enabled: true, role: "viewer", emailDomains: ["example.com"] } });
    const { container } = render(<IdentityProviderPage />);
    await screen.findByDisplayValue(ISSUER);

    expect(textBelowFloor(container)).toEqual([]);
    const buttons = screen.getAllByRole("button");
    expect(buttons.map((b) => b.textContent)).toEqual(["Copy", "Save", "Remove identity provider", "Copy"]);
    for (const button of buttons) {
      expect(hitAreaPx(button).height).toBeGreaterThanOrEqual(HIT_AREA_FLOOR_PX);
    }
  });
});

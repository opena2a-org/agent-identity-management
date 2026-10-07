import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor, cleanup } from "@testing-library/react";
import type { ReactNode } from "react";
import { FirstRunPanel, isFirstRun } from "./first-run-panel";
import { SDK_TABS } from "@/lib/sdk-tabs";
import type { Agent } from "@/lib/api";

vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));

const AGENT_ID = "7b1d2c7e-58a4-4f0e-9a51-0c2f6d1e8a11";
const PUBLIC_KEY = "mC4sAbPp0Vt1x2Yq3Zr4Ws5Xt6Yu7Zv8Aw9Bx0Cy1D=";

const agent = (overrides: Partial<Agent> = {}): Agent =>
  ({
    id: AGENT_ID,
    name: "billing-bot",
    displayName: "Billing Bot",
    status: "pending",
    publicKey: PUBLIC_KEY,
    lastActive: null,
    ...overrides,
  }) as Agent;

afterEach(() => {
  vi.clearAllMocks();
  cleanup();
});

describe("isFirstRun", () => {
  it("holds from registration until the agent's first authenticated call", () => {
    expect(isFirstRun({ status: "pending", lastActive: null })).toBe(true);
    expect(isFirstRun({ status: "verified" })).toBe(true);
    expect(isFirstRun({ status: "verified", lastActive: "2026-10-01T09:30:00Z" })).toBe(false);
  });

  it("never holds for an agent that cannot connect", () => {
    expect(isFirstRun({ status: "suspended", lastActive: null })).toBe(false);
    expect(isFirstRun({ status: "revoked", lastActive: null })).toBe(false);
  });
});

describe("FirstRunPanel, first run", () => {
  it("shows the identity, the commands that connect the agent, and where to go next", () => {
    const { container } = render(<FirstRunPanel agent={agent()} />);

    expect(screen.getByRole("region", { name: "Agent registered. Next, connect it from your code" })).toBeTruthy();
    expect(screen.getByText("Billing Bot")).toBeTruthy();

    // Identity: the identifier and the public key, each with a copy control.
    expect(screen.getByText(AGENT_ID)).toBeTruthy();
    expect(screen.getByText(PUBLIC_KEY)).toBeTruthy();
    expect(screen.getByRole("button", { name: "Copy Agent ID" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Copy Public key (Ed25519)" })).toBeTruthy();

    // The next commands, with this dashboard's origin and this agent's identifier filled in.
    expect(screen.getByText("pip install aim-sdk")).toBeTruthy();
    expect(screen.getByText(`aim-sdk login --url ${window.location.origin}`)).toBeTruthy();
    expect(container.textContent).toContain(`agent = secure("${AGENT_ID}")`);
    expect(container.textContent).toContain('@agent.perform_action("read_database", resource="users_table")');

    // Where the credentials are.
    expect(container.textContent).toContain("stores it under ~/.aim/ and replaces the key created at registration");

    // What to do next.
    expect(screen.getByRole("link", { name: "View documentation" }).getAttribute("href")).toBe("https://opena2a.org/docs");
    expect(screen.getByRole("link", { name: "View all agents" }).getAttribute("href")).toBe("/dashboard/agents");
    expect(screen.getByRole("link", { name: "Go to dashboard" }).getAttribute("href")).toBe("/dashboard");
  });

  it("lets each identity row shrink below its value, so the value truncates and the copy button stays on screen", () => {
    // A grid item defaults to min-width: auto, which holds the track at the unbroken value's
    // width and pushes the copy button off a phone screen; jsdom has no layout, so the class
    // that releases it is the observable part.
    render(<FirstRunPanel agent={agent()} />);
    for (const label of ["Agent ID", "Public key (Ed25519)"]) {
      const row = screen.getByRole("button", { name: `Copy ${label}` }).parentElement!;
      expect(row.parentElement!.className, label).toContain("grid");
      expect(row.className.split(/\s+/), label).toContain("min-w-0");
      expect(row.querySelector(".truncate")?.textContent, label).toBeTruthy();
    }
  });

  it("leaves the public key row out when the agent has none", () => {
    render(<FirstRunPanel agent={agent({ publicKey: null })} />);
    expect(screen.getByRole("button", { name: "Copy Agent ID" })).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Copy Public key (Ed25519)" })).toBeNull();
  });

  it("copies the agent ID", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
    render(<FirstRunPanel agent={agent()} />);

    fireEvent.click(screen.getByRole("button", { name: "Copy Agent ID" }));
    await waitFor(() => expect(writeText).toHaveBeenCalledWith(AGENT_ID));
  });

  it("switches to the Java steps: the shared quickstart, connected by this agent's name", () => {
    const java = SDK_TABS.find((tab) => tab.key === "java")!;
    const { container } = render(<FirstRunPanel agent={agent()} />);

    expect(screen.getByRole("button", { name: "Python" }).getAttribute("aria-pressed")).toBe("true");
    expect(screen.queryByText(java.install(""))).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Java" }));
    expect(screen.getByRole("button", { name: "Java" }).getAttribute("aria-pressed")).toBe("true");
    expect(screen.queryByText("pip install aim-sdk")).toBeNull();
    expect(screen.getByText(java.install(""))).toBeTruthy();
    expect(screen.getByText('AIMClient agent = AIMClient.secure("billing-bot");')).toBeTruthy();
    expect(container.textContent).toContain(java.code(""));
    expect(screen.getByRole("link", { name: java.docsLabel }).getAttribute("href")).toBe(java.docsHref);

    // A clone carries no credentials, so a step names where AIMClient.secure() gets them.
    expect(screen.getByRole("heading", { name: "Get the SDK credentials" })).toBeTruthy();
    expect(container.textContent).toContain("AIM_REFRESH_TOKEN");
    expect(container.textContent).toContain("~/.aim/sdk_credentials.json");
    expect(screen.getByRole("link", { name: "Open the SDK page" }).getAttribute("href")).toBe("/dashboard/sdk");

    // No control offers a per-agent Java download: the server has no such package.
    expect(screen.queryByRole("button", { name: /download/i })).toBeNull();
  });
});

describe("FirstRunPanel, not first run", () => {
  it("renders nothing once the agent has connected", () => {
    const { container } = render(<FirstRunPanel agent={agent({ status: "verified", lastActive: "2026-10-01T09:30:00Z" })} />);
    expect(container.firstChild).toBeNull();
  });

  it("renders nothing for a suspended agent that never connected", () => {
    const { container } = render(<FirstRunPanel agent={agent({ status: "suspended" })} />);
    expect(container.firstChild).toBeNull();
  });
});

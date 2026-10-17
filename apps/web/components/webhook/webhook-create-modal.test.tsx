import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { readFileSync } from "fs";
import path from "path";
import { fileURLToPath } from "url";

const api = vi.hoisted(() => ({ createWebhook: vi.fn() }));
vi.mock("@/lib/api", () => ({ api }));

import { AVAILABLE_EVENTS, WebhookCreateModal } from "./webhook-create-modal";

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, "../../../..");

// The events docs/API.md § Webhook Events says the server sends, with the
// text of the table's "Sent when" column, backticks removed.
function documentedEvents(): Array<{ id: string; description: string }> {
  const doc = readFileSync(path.join(repoRoot, "docs/API.md"), "utf8");
  const section = doc.split("### Webhook Events")[1].split("\n### ")[0];
  return section
    .split("\n")
    .map((line) => line.match(/^\| `([a-z_.]+)` \| (.+) \|$/))
    .filter((match): match is RegExpMatchArray => match !== null)
    .map((match) => ({ id: match[1], description: match[2].replace(/`/g, "") }));
}

// Every event name the server defines (domain.AllWebhookEvents).
function serverEventNames(): string[] {
  const source = readFileSync(path.join(repoRoot, "apps/backend/internal/domain/webhook.go"), "utf8");
  return Array.from(source.matchAll(/WebhookEvent = "([a-z_.]+)"/g), (match) => match[1]);
}

describe("webhook create modal: events offered", () => {
  it("offers exactly the events the API reference says the server sends, with their descriptions", () => {
    const documented = documentedEvents();
    expect(documented.length).toBeGreaterThan(0);
    expect(AVAILABLE_EVENTS.map(({ id, description }) => ({ id, description }))).toEqual(documented);
  });

  it("offers no name the server does not define", () => {
    const known = new Set(serverEventNames());
    expect(known.size).toBeGreaterThan(0);
    for (const event of AVAILABLE_EVENTS) {
      expect(known, event.id).toContain(event.id);
    }
    for (const unsent of ["agent.reactivated", "trust_score.critical", "alert.resolved", "verification.failed"]) {
      expect(AVAILABLE_EVENTS.map((event) => event.id)).not.toContain(unsent);
    }
  });
});

describe("webhook create modal: the secret after create", () => {
  const writeText = vi.fn(async () => {});
  const secret = "e".repeat(64);

  beforeEach(() => {
    api.createWebhook.mockReset();
    writeText.mockClear();
    Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
  });
  afterEach(cleanup);

  function fillForm() {
    fireEvent.change(screen.getByLabelText("Webhook Name *"), { target: { value: "receiver" } });
    fireEvent.change(screen.getByLabelText("Webhook URL *"), { target: { value: "https://hooks.example.com/aim" } });
    fireEvent.click(screen.getByLabelText("Agent Created"));
  }

  it("shows the returned secret once, with a copy control, until the user is done", async () => {
    api.createWebhook.mockResolvedValue({ id: "w1", name: "receiver", secret });
    const onSuccess = vi.fn();
    const onClose = vi.fn();
    render(<WebhookCreateModal isOpen onClose={onClose} onSuccess={onSuccess} />);

    fillForm();
    fireEvent.click(screen.getByRole("button", { name: "Create Webhook" }));

    expect((await screen.findByTestId("webhook-created-secret")).textContent).toBe(secret);
    expect(screen.getByText(/This secret will not be shown again/)).toBeTruthy();
    expect(api.createWebhook).toHaveBeenCalledWith({
      name: "receiver",
      url: "https://hooks.example.com/aim",
      events: ["agent.created"],
      secret: undefined,
    });
    expect(onSuccess).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Copy secret" }));
    await screen.findByText("Copied");
    expect(writeText).toHaveBeenCalledWith(secret);

    fireEvent.click(screen.getByRole("button", { name: "Done" }));
    expect(onSuccess).toHaveBeenCalledTimes(1);
    expect(onClose).not.toHaveBeenCalled();
    expect(screen.queryByText(secret)).toBeNull();
  });

  it("sends a chosen secret, and shows the one the server answers", async () => {
    const chosen = "c".repeat(40);
    api.createWebhook.mockResolvedValue({ id: "w1", name: "receiver", secret: chosen });
    render(<WebhookCreateModal isOpen onClose={vi.fn()} onSuccess={vi.fn()} />);

    fillForm();
    fireEvent.change(screen.getByLabelText("Webhook Secret (Optional)"), { target: { value: chosen } });
    fireEvent.click(screen.getByRole("button", { name: "Create Webhook" }));

    expect((await screen.findByTestId("webhook-created-secret")).textContent).toBe(chosen);
    expect(api.createWebhook.mock.calls[0][0].secret).toBe(chosen);
  });

  it("refuses a chosen secret shorter than 32 characters before sending it", () => {
    render(<WebhookCreateModal isOpen onClose={vi.fn()} onSuccess={vi.fn()} />);

    fillForm();
    fireEvent.change(screen.getByLabelText("Webhook Secret (Optional)"), { target: { value: "s".repeat(31) } });
    fireEvent.click(screen.getByRole("button", { name: "Create Webhook" }));

    expect(screen.getByText("Webhook secret must be at least 32 characters")).toBeTruthy();
    expect(api.createWebhook).not.toHaveBeenCalled();
  });
});

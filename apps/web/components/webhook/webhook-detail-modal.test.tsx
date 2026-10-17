import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";

const api = vi.hoisted(() => ({ getWebhook: vi.fn(), rotateWebhookSecret: vi.fn() }));
vi.mock("@/lib/api", () => ({ api }));

import { WebhookDetailModal } from "./webhook-detail-modal";

// The webhook as GET /api/v1/webhooks/:id answers it: no secret.
const webhook = {
  id: "0c6f3c1e-8f0e-4a63-9a43-2f1d8b7e5a10",
  organizationId: "7d9f2b4a-1c3e-4f5a-8b6c-9d0e1f2a3b4c",
  name: "receiver",
  url: "https://hooks.example.com/aim",
  events: ["agent.created"],
  isActive: true,
  createdAt: "2026-10-16T00:00:00Z",
  successCount: 0,
  failureCount: 0,
};

describe("webhook detail modal: secret", () => {
  const rotated = "a".repeat(64);
  const writeText = vi.fn(async () => {});

  beforeEach(() => {
    api.getWebhook.mockReset().mockResolvedValue(webhook);
    api.rotateWebhookSecret.mockReset().mockResolvedValue({ id: webhook.id, secret: rotated });
    writeText.mockClear();
    Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
  });
  afterEach(cleanup);

  function renderModal() {
    render(<WebhookDetailModal isOpen webhookId={webhook.id} onClose={vi.fn()} onRefresh={vi.fn()} />);
  }

  it("shows no secret, since the server never returns one, and offers rotation", async () => {
    renderModal();
    await screen.findByText(webhook.url);

    expect(screen.queryByTestId("webhook-rotated-secret")).toBeNull();
    expect(screen.queryByText(/•/)).toBeNull();
    expect(screen.getByText(/shown only when the webhook is created or its secret is rotated/)).toBeTruthy();
    expect(screen.getByRole("button", { name: "Rotate secret" })).toBeTruthy();
  });

  it("rotates after a confirmation and shows the new secret once", async () => {
    renderModal();
    await screen.findByText(webhook.url);

    fireEvent.click(screen.getByRole("button", { name: "Rotate secret" }));
    expect(api.rotateWebhookSecret).not.toHaveBeenCalled();
    expect(screen.getByText(/Rotating replaces the secret at once/)).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "Confirm rotation" }));
    expect((await screen.findByTestId("webhook-rotated-secret")).textContent).toBe(rotated);
    expect(api.rotateWebhookSecret).toHaveBeenCalledWith(webhook.id);
    expect(api.rotateWebhookSecret).toHaveBeenCalledTimes(1);
    expect(screen.getByText(/This secret will not be shown again/)).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "Copy secret" }));
    await vi.waitFor(() => expect(writeText).toHaveBeenCalledWith(rotated));
  });

  it("cancelling the confirmation rotates nothing", async () => {
    renderModal();
    await screen.findByText(webhook.url);

    fireEvent.click(screen.getByRole("button", { name: "Rotate secret" }));
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));

    expect(api.rotateWebhookSecret).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Rotate secret" })).toBeTruthy();
  });

  it("says why a rotation failed and shows no secret", async () => {
    api.rotateWebhookSecret.mockRejectedValue(new Error("Webhook not found"));
    renderModal();
    await screen.findByText(webhook.url);

    fireEvent.click(screen.getByRole("button", { name: "Rotate secret" }));
    fireEvent.click(screen.getByRole("button", { name: "Confirm rotation" }));

    expect(await screen.findByText("Webhook not found")).toBeTruthy();
    expect(screen.queryByTestId("webhook-rotated-secret")).toBeNull();
  });
});

import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen, cleanup, fireEvent, waitFor, within } from "@testing-library/react";
import type { ReactNode } from "react";
import AlertChannelsPage from "./page";
import { api } from "@/lib/api";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: vi.fn(), push: vi.fn() }),
  useSearchParams: () => new URLSearchParams(),
}));
vi.mock("@/components/auth-guard", () => ({
  AuthGuard: ({ children }: { children: ReactNode }) => <>{children}</>,
}));
vi.mock("@/lib/api", () => ({
  api: {
    listAlertChannels: vi.fn(),
    getAlertChannel: vi.fn(),
    createAlertChannel: vi.fn(),
    updateAlertChannel: vi.fn(),
    deleteAlertChannel: vi.fn(),
    testAlertChannel: vi.fn(),
  },
}));

const base = {
  isActive: true,
  lastDeliveryStatus: null,
  lastDeliveryCode: null,
  lastDeliveryAt: null,
  createdAt: "2026-10-16T00:00:00Z",
  updatedAt: "2026-10-16T00:00:00Z",
};

// One channel of each kind, with config as the server answers it: the Slack,
// PagerDuty and Teams values masked to their last four characters, the email
// recipients as stored.
const slack = {
  ...base,
  id: "3f1c2b7a-5d4e-4c3b-9a8f-1e2d3c4b5a69",
  name: "Security on-call",
  kind: "slack" as const,
  minSeverity: "warning" as const,
  config: { webhookUrl: "****Q7fz" },
  lastDeliveryStatus: "delivered",
  lastDeliveryCode: 200,
  lastDeliveryAt: new Date(Date.now() - 5 * 60 * 1000).toISOString(),
};
const pagerduty = {
  ...base,
  id: "9b2e4c1a-7f3d-4e6b-8a5c-2d1f0e9b8a7c",
  name: "PagerDuty security service",
  kind: "pagerduty" as const,
  minSeverity: "high" as const,
  config: { routingKey: "****a1b2" },
};
const teams = {
  ...base,
  id: "5a6b7c8d-1e2f-4a3b-9c4d-5e6f7a8b9c0d",
  name: "Security channel in Teams",
  kind: "teams" as const,
  minSeverity: "info" as const,
  isActive: false,
  config: { webhookUrl: "****9x8w" },
};
const email = {
  ...base,
  id: "1d2c3b4a-5f6e-4d7c-8b9a-0f1e2d3c4b5a",
  name: "Security team mailbox",
  kind: "email" as const,
  minSeverity: "critical" as const,
  config: { recipients: ["ops@example.com", "sec@example.com"] },
};

function serve(channels: object[]) {
  vi.mocked(api.listAlertChannels).mockResolvedValue({
    alertChannels: channels,
    total: channels.length,
  } as never);
}

function rowOf(name: string): HTMLElement {
  const row = screen.getByText(name).closest<HTMLElement>("[data-alert-channel-row]");
  expect(row).not.toBeNull();
  return row!;
}

// A URL of the Slack incoming webhook form, put together at run time so that no
// file holds one.
const SLACK_WEBHOOK_URL = ["https://hooks.slack.com/services/", "T000", "/B000/", "XXXX"].join("");

function requestError(status: number, message: string) {
  return Object.assign(new Error(message), { status });
}

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("AlertChannelsPage list", () => {
  it("shows each channel's name, kind, floor and config exactly as the server answers it", async () => {
    serve([slack, pagerduty, teams, email]);
    render(<AlertChannelsPage />);

    await screen.findByText(slack.name);
    for (const channel of [slack, pagerduty, teams, email]) {
      const row = rowOf(channel.name);
      expect(row.textContent).toContain(`alerts at or above ${channel.minSeverity}`);
    }
    expect(rowOf(slack.name).textContent).toContain("Slack");
    expect(rowOf(pagerduty.name).textContent).toContain("PagerDuty");
    expect(rowOf(teams.name).textContent).toContain("Microsoft Teams");
    expect(rowOf(email.name).textContent).toContain("Email");

    expect(within(rowOf(slack.name)).getByText("****Q7fz")).toBeTruthy();
    expect(within(rowOf(pagerduty.name)).getByText("****a1b2")).toBeTruthy();
    expect(within(rowOf(teams.name)).getByText("****9x8w")).toBeTruthy();
    expect(within(rowOf(email.name)).getByText("ops@example.com")).toBeTruthy();
    expect(within(rowOf(email.name)).getByText("sec@example.com")).toBeTruthy();
  });

  it("shows the active state and the last delivery, or never when there was none", async () => {
    serve([slack, teams]);
    render(<AlertChannelsPage />);

    await screen.findByText(slack.name);
    const slackRow = rowOf(slack.name);
    expect(slackRow.textContent).toContain("Active");
    expect(slackRow.textContent).toContain("Delivered (200)");
    expect(slackRow.textContent).toContain("5 minutes ago");

    const teamsRow = rowOf(teams.name);
    expect(teamsRow.textContent).toContain("Inactive");
    expect(teamsRow.textContent).toContain("never");
  });

  it("offers Add channel from the empty state, which says what a channel does", async () => {
    serve([]);
    render(<AlertChannelsPage />);

    const empty = await screen.findByTestId("alert-channels-empty");
    expect(empty.textContent).toContain("severity floor");
    fireEvent.click(within(empty).getByRole("button", { name: "Add channel" }));
    expect(await screen.findByLabelText("Slack incoming webhook URL")).toBeTruthy();
  });

  it("stacks each channel below the lg breakpoint so a 375px screen does not scroll sideways", async () => {
    serve([email]);
    render(<AlertChannelsPage />);

    await screen.findByText(email.name);
    const row = rowOf(email.name);
    // A single column until lg, where the row takes the list's columns, shared
    // with the header and every other row.
    expect(row.className).toContain("grid");
    expect(row.className).toContain("lg:grid-cols-subgrid");
    expect(row.className).not.toMatch(/(^|\s)grid-cols-/);
    // Long recipient addresses wrap rather than widen the page.
    expect(within(row).getByText("ops@example.com").className).toContain("break-words");
  });
});

describe("AlertChannelsPage create and edit", () => {
  it("creates a Slack channel with the field names the server's request takes", async () => {
    serve([]);
    vi.mocked(api.createAlertChannel).mockResolvedValue({ ...slack } as never);
    render(<AlertChannelsPage />);

    const empty = await screen.findByTestId("alert-channels-empty");
    fireEvent.click(within(empty).getByRole("button", { name: "Add channel" }));

    const dialog = await screen.findByRole("dialog");
    fireEvent.change(within(dialog).getByLabelText("Name"), { target: { value: "  Security on-call " } });
    fireEvent.change(within(dialog).getByLabelText("Kind"), { target: { value: "slack" } });
    fireEvent.change(within(dialog).getByLabelText("Slack incoming webhook URL"), {
      target: { value: SLACK_WEBHOOK_URL },
    });
    fireEvent.change(within(dialog).getByLabelText("Severity floor"), { target: { value: "high" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Add channel" }));

    await waitFor(() => expect(api.createAlertChannel).toHaveBeenCalledTimes(1));
    expect(vi.mocked(api.createAlertChannel).mock.calls[0][0]).toEqual({
      name: "Security on-call",
      kind: "slack",
      config: { webhookUrl: SLACK_WEBHOOK_URL },
      minSeverity: "high",
    });
  });

  it("creates an email channel with one recipient per line", async () => {
    serve([]);
    vi.mocked(api.createAlertChannel).mockResolvedValue({ ...email } as never);
    render(<AlertChannelsPage />);

    const empty = await screen.findByTestId("alert-channels-empty");
    fireEvent.click(within(empty).getByRole("button", { name: "Add channel" }));

    const dialog = await screen.findByRole("dialog");
    fireEvent.change(within(dialog).getByLabelText("Name"), { target: { value: "Security team mailbox" } });
    fireEvent.change(within(dialog).getByLabelText("Kind"), { target: { value: "email" } });
    fireEvent.change(within(dialog).getByLabelText("Email recipients"), {
      target: { value: " ops@example.com\n\nsec@example.com \n" },
    });
    fireEvent.click(within(dialog).getByRole("button", { name: "Add channel" }));

    await waitFor(() => expect(api.createAlertChannel).toHaveBeenCalledTimes(1));
    expect(vi.mocked(api.createAlertChannel).mock.calls[0][0]).toEqual({
      name: "Security team mailbox",
      kind: "email",
      config: { recipients: ["ops@example.com", "sec@example.com"] },
      minSeverity: "warning",
    });
  });

  it("refuses more than 20 recipients beside the field without calling the server", async () => {
    serve([]);
    render(<AlertChannelsPage />);

    const empty = await screen.findByTestId("alert-channels-empty");
    fireEvent.click(within(empty).getByRole("button", { name: "Add channel" }));

    const dialog = await screen.findByRole("dialog");
    fireEvent.change(within(dialog).getByLabelText("Name"), { target: { value: "Everyone" } });
    fireEvent.change(within(dialog).getByLabelText("Kind"), { target: { value: "email" } });
    const many = Array.from({ length: 21 }, (_, i) => `user${i}@example.com`).join("\n");
    fireEvent.change(within(dialog).getByLabelText("Email recipients"), { target: { value: many } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Add channel" }));

    const fieldError = await within(dialog).findByTestId("alert-channel-config-error");
    expect(fieldError.textContent).toContain("at most 20");
    expect(api.createAlertChannel).not.toHaveBeenCalled();
  });

  it("shows a 400 naming config.webhookUrl beside the URL field", async () => {
    serve([]);
    const message =
      "config.webhookUrl must be a Slack incoming webhook URL: https://hooks.slack.com/services/ followed by the webhook's path";
    vi.mocked(api.createAlertChannel).mockRejectedValue(requestError(400, message));
    render(<AlertChannelsPage />);

    const empty = await screen.findByTestId("alert-channels-empty");
    fireEvent.click(within(empty).getByRole("button", { name: "Add channel" }));

    const dialog = await screen.findByRole("dialog");
    fireEvent.change(within(dialog).getByLabelText("Name"), { target: { value: "Security on-call" } });
    const url = within(dialog).getByLabelText("Slack incoming webhook URL");
    fireEvent.change(url, { target: { value: "https://example.com/hook" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Add channel" }));

    const fieldError = await within(dialog).findByTestId("alert-channel-config-error");
    expect(fieldError.textContent).toBe(message);
    expect(url.getAttribute("aria-invalid")).toBe("true");
    expect(url.getAttribute("aria-describedby")).toContain(fieldError.id);
    expect(within(dialog).queryByTestId("alert-channel-name-error")).toBeNull();
  });

  it("shows a name the organization already uses beside the name field", async () => {
    serve([]);
    vi.mocked(api.createAlertChannel).mockRejectedValue(
      requestError(409, "name is used by another alert channel of the organization"),
    );
    render(<AlertChannelsPage />);

    const empty = await screen.findByTestId("alert-channels-empty");
    fireEvent.click(within(empty).getByRole("button", { name: "Add channel" }));

    const dialog = await screen.findByRole("dialog");
    fireEvent.change(within(dialog).getByLabelText("Name"), { target: { value: "Security on-call" } });
    fireEvent.change(within(dialog).getByLabelText("Slack incoming webhook URL"), {
      target: { value: SLACK_WEBHOOK_URL },
    });
    fireEvent.click(within(dialog).getByRole("button", { name: "Add channel" }));

    const fieldError = await within(dialog).findByTestId("alert-channel-name-error");
    expect(fieldError.textContent).toBe("name is used by another alert channel of the organization");
  });

  it("edits with an empty config field by sending no config key, and shows the masked value", async () => {
    serve([slack]);
    vi.mocked(api.updateAlertChannel).mockResolvedValue({ ...slack, minSeverity: "critical" } as never);
    render(<AlertChannelsPage />);

    await screen.findByText(slack.name);
    fireEvent.click(within(rowOf(slack.name)).getByRole("button", { name: `Edit ${slack.name}` }));

    const dialog = await screen.findByRole("dialog");
    const url = within(dialog).getByLabelText("Slack incoming webhook URL") as HTMLInputElement;
    expect(url.value).toBe("");
    expect(dialog.textContent).toContain("****Q7fz");
    expect(dialog.textContent).toContain("leave empty to keep the current value");
    expect((within(dialog).getByLabelText("Name") as HTMLInputElement).value).toBe(slack.name);

    fireEvent.change(within(dialog).getByLabelText("Severity floor"), { target: { value: "critical" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Save changes" }));

    await waitFor(() => expect(api.updateAlertChannel).toHaveBeenCalledTimes(1));
    const [id, body] = vi.mocked(api.updateAlertChannel).mock.calls[0];
    expect(id).toBe(slack.id);
    expect(body).toEqual({ name: slack.name, minSeverity: "critical" });
    expect(Object.prototype.hasOwnProperty.call(body, "config")).toBe(false);
    expect(Object.prototype.hasOwnProperty.call(body, "kind")).toBe(false);
  });

  it("edits with a new config value by sending it whole", async () => {
    serve([pagerduty]);
    vi.mocked(api.updateAlertChannel).mockResolvedValue({ ...pagerduty } as never);
    render(<AlertChannelsPage />);

    await screen.findByText(pagerduty.name);
    fireEvent.click(within(rowOf(pagerduty.name)).getByRole("button", { name: `Edit ${pagerduty.name}` }));

    const dialog = await screen.findByRole("dialog");
    const key = "k".repeat(32);
    fireEvent.change(within(dialog).getByLabelText("PagerDuty integration key"), { target: { value: key } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Save changes" }));

    await waitFor(() => expect(api.updateAlertChannel).toHaveBeenCalledTimes(1));
    expect(vi.mocked(api.updateAlertChannel).mock.calls[0][1]).toEqual({
      name: pagerduty.name,
      minSeverity: "high",
      config: { routingKey: key },
    });
  });
});

describe("AlertChannelsPage test and delete", () => {
  it("shows Delivered (200) when the test message was delivered", async () => {
    serve([slack]);
    vi.mocked(api.testAlertChannel).mockResolvedValue({ delivered: true, statusCode: 200 });
    render(<AlertChannelsPage />);

    await screen.findByText(slack.name);
    fireEvent.click(within(rowOf(slack.name)).getByRole("button", { name: `Test ${slack.name}` }));

    const outcome = await within(rowOf(slack.name)).findByTestId("alert-channel-test-outcome");
    expect(outcome.textContent).toContain("Delivered (200)");
    expect(api.testAlertChannel).toHaveBeenCalledWith(slack.id);
  });

  it("shows the error class and the next step for a 502 answer, and no URL", async () => {
    serve([slack]);
    vi.mocked(api.testAlertChannel).mockResolvedValue({
      delivered: false,
      statusCode: 404,
      error: "status 404",
    });
    render(<AlertChannelsPage />);

    await screen.findByText(slack.name);
    fireEvent.click(within(rowOf(slack.name)).getByRole("button", { name: `Test ${slack.name}` }));

    const outcome = await within(rowOf(slack.name)).findByTestId("alert-channel-test-outcome");
    expect(outcome.textContent).toContain("status 404");
    expect(outcome.textContent).toContain("check the URL or key, then test again");
    expect(outcome.textContent).not.toMatch(/https?:\/\//);
  });

  it("asks before a PagerDuty test, saying it opens an alert on the PagerDuty service", async () => {
    serve([pagerduty]);
    vi.mocked(api.testAlertChannel).mockResolvedValue({ delivered: true, statusCode: 202 });
    render(<AlertChannelsPage />);

    await screen.findByText(pagerduty.name);
    fireEvent.click(within(rowOf(pagerduty.name)).getByRole("button", { name: `Test ${pagerduty.name}` }));

    const confirm = await screen.findByRole("alertdialog");
    expect(confirm.textContent).toContain("opens an alert on the PagerDuty service");
    expect(api.testAlertChannel).not.toHaveBeenCalled();

    fireEvent.click(within(confirm).getByRole("button", { name: "Send test event" }));
    await waitFor(() => expect(api.testAlertChannel).toHaveBeenCalledWith(pagerduty.id));
    const outcome = await within(rowOf(pagerduty.name)).findByTestId("alert-channel-test-outcome");
    expect(outcome.textContent).toContain("Delivered (202)");
  });

  it("deletes a channel only after the confirmation", async () => {
    serve([slack]);
    vi.mocked(api.deleteAlertChannel).mockResolvedValue(undefined);
    render(<AlertChannelsPage />);

    await screen.findByText(slack.name);
    fireEvent.click(within(rowOf(slack.name)).getByRole("button", { name: `Delete ${slack.name}` }));

    const confirm = await screen.findByRole("alertdialog");
    expect(confirm.textContent).toContain(slack.name);
    expect(api.deleteAlertChannel).not.toHaveBeenCalled();

    fireEvent.click(within(confirm).getByRole("button", { name: "Delete channel" }));
    await waitFor(() => expect(api.deleteAlertChannel).toHaveBeenCalledWith(slack.id));
  });

  it("does not delete when the confirmation is cancelled", async () => {
    serve([slack]);
    render(<AlertChannelsPage />);

    await screen.findByText(slack.name);
    fireEvent.click(within(rowOf(slack.name)).getByRole("button", { name: `Delete ${slack.name}` }));

    const confirm = await screen.findByRole("alertdialog");
    fireEvent.click(within(confirm).getByRole("button", { name: "Cancel" }));
    await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
    expect(api.deleteAlertChannel).not.toHaveBeenCalled();
  });
});

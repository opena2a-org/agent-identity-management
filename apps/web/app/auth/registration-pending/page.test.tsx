import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen, cleanup } from "@testing-library/react";
import type { ReactNode } from "react";
import RegistrationPendingPage from "./page";
import { countTextRuns, textBelowFloor } from "@/tests/readability";

// The page a new account lands on after sign-up is read on a phone: no text on
// it, the request ID included, renders below 14px at 375px.
let query = "request_id=req-fixture-0001";
vi.mock("next/navigation", () => ({ useSearchParams: () => new URLSearchParams(query) }));
vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("@/components/sidebar", () => ({ AimLogo: () => null }));

afterEach(() => {
  cleanup();
  query = "request_id=req-fixture-0001";
});

describe("registration pending page readability", () => {
  it("renders no text below 14px with a request ID", () => {
    const { container } = render(<RegistrationPendingPage />);
    expect(screen.getByText("req-fixture-0001")).toBeTruthy();
    expect(countTextRuns(container)).toBeGreaterThan(10);
    expect(textBelowFloor(container)).toEqual([]);
  });

  it("renders no text below 14px without a request ID", () => {
    query = "";
    const { container } = render(<RegistrationPendingPage />);
    expect(screen.queryByText(/Request ID/)).toBeNull();
    expect(screen.getByText("Registration submitted")).toBeTruthy();
    expect(textBelowFloor(container)).toEqual([]);
  });
});

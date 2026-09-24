import { describe, expect, it } from "vitest";
import { GET } from "./route";

// /login forwards to the sign-in page with a 307 and carries nothing from its own
// URL: a returnUrl planted on the alias must not reach /auth/login.

describe("/login", () => {
  it("answers exactly 307 with Location /auth/login and no query carried", () => {
    const response = GET();
    expect(response.status).toBe(307);
    expect(response.headers.get("location")).toBe("/auth/login");
  });

  it("carries no returnUrl from the alias, even a crafted one", () => {
    // The handler takes no argument on purpose: whatever the request carried, the
    // answer is the same. This cell pins that a crafted query has no effect.
    const request = new Request("http://localhost:3000/login?returnUrl=%2F%09%2Fevil.example");
    const response = (GET as unknown as (req: Request) => Response)(request);
    expect(response.status).toBe(307);
    expect(response.headers.get("location")).toBe("/auth/login");
    expect(response.headers.get("location")).not.toContain("returnUrl");
  });
});

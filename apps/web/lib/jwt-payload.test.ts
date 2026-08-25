import { describe, expect, it } from "vitest";
import { decodeJwtPayload } from "@/lib/jwt-payload";

const token = (payload: Record<string, unknown>, encoding: "base64" | "base64url" = "base64url") =>
  `header.${Buffer.from(JSON.stringify(payload)).toString(encoding).replace(/=+$/, "")}.signature`;

describe("decodeJwtPayload", () => {
  it("decodes the base64url alphabet real JWTs use, with and without padding", () => {
    let t = "";
    for (let n = 1; n <= 3 && !/[-_]/.test(t); n++) t = token({ role: "admin", name: "?".repeat(n) });
    expect(t).toMatch(/[-_]/);
    expect(decodeJwtPayload(t)?.role).toBe("admin");
    expect(decodeJwtPayload(token({ role: "member", exp: 1 }, "base64"))).toEqual({ role: "member", exp: 1 });
  });

  it("returns null for anything that is not a decodable JWT payload", () => {
    for (const bad of ["not.a-jwt.at-all", "onlyonesegment", "", "a..b", "h.!!!.s"]) {
      expect(decodeJwtPayload(bad), bad).toBeNull();
    }
    expect(decodeJwtPayload(null)).toBeNull();
    expect(decodeJwtPayload(undefined)).toBeNull();
    // A payload that decodes to a non-object (a JSON string) is not a claims set.
    expect(decodeJwtPayload(`h.${Buffer.from('"just a string"').toString("base64url")}.s`)).toBeNull();
  });
});

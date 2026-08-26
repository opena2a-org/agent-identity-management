import { describe, it, expect } from "vitest";
import { trustDisplay } from "./shared";

describe("trustDisplay", () => {
  it("renders whole numbers on the 0-100 scale, never decimals", () => {
    expect(trustDisplay(0.71)).toBe("71");
    expect(trustDisplay(0.845)).toBe("85");
    expect(trustDisplay(1)).toBe("100");
    expect(trustDisplay(0)).toBe("0");
  });

  it("passes through scores already on the 0-100 scale, rounded", () => {
    expect(trustDisplay(84)).toBe("84");
    expect(trustDisplay(59.6)).toBe("60");
  });

  it("returns null for missing or invalid scores", () => {
    expect(trustDisplay(null)).toBeNull();
    expect(trustDisplay(undefined)).toBeNull();
    expect(trustDisplay(Number.NaN)).toBeNull();
  });
});

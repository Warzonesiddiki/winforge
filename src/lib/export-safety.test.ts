import { describe, expect, it } from "vitest";
import { csvEscape, escapeHtml } from "./export-safety";

describe("escapeHtml", () => {
  it("escapes markup and quoted attributes", () => {
    expect(escapeHtml(`<img src=x onerror="alert('x')"> &`)).toBe(
      "&lt;img src=x onerror=&quot;alert(&#39;x&#39;)&quot;&gt; &amp;"
    );
  });

  it("stringifies non-string values", () => {
    expect(escapeHtml(42)).toBe("42");
  });
});

describe("csvEscape", () => {
  it.each(["=1+1", "+cmd", "-2+3", "@SUM(A1:A2)", " \t=1+1"])(
    "neutralizes spreadsheet formula %j",
    (value) => {
      expect(csvEscape(value)).toMatch(/^'?/);
      expect(csvEscape(value)).toContain(`'${value}`);
    }
  );

  it("quotes commas, quotes, and newlines", () => {
    expect(csvEscape('one,"two"\nthree')).toBe('"one,""two""\nthree"');
  });

  it("leaves ordinary values unchanged", () => {
    expect(csvEscape("tweak-applied")).toBe("tweak-applied");
  });
});

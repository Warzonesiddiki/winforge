import { describe, it, expect } from "vitest";
import { t, getLocales, type Locale } from "./i18n";

describe("i18n", () => {
  it("returns the English string for a known key", () => {
    expect(t("nav.dashboard")).toBe("Dashboard");
    expect(t("common.apply")).toBe("Apply");
  });

  it("translates known keys into every supported locale", () => {
    const locales = getLocales().map((l) => l.id);
    expect(locales).toEqual(["en-US", "es-ES", "fr-FR", "de-DE", "zh-CN"]);
    for (const locale of locales) {
      // Every locale must translate the nav labels — these are the core
      // chrome strings the dashboard renders.
      const value = t("nav.dashboard", locale as Locale);
      expect(value).not.toBe("nav.dashboard");
      expect(value.length).toBeGreaterThan(0);
    }
  });

  it("falls back to English for an unrecognized locale", () => {
    // @ts-expect-error deliberately invalid locale to test fallback
    const value = t("common.apply", "xx-XX");
    expect(value).toBe("Apply");
  });

  it("returns the key itself for an unknown translation key", () => {
    const value = t("nonexistent.key");
    expect(value).toBe("nonexistent.key");
  });

  it("exposes locale display names for all five languages", () => {
    const names = getLocales().map((l) => l.name);
    expect(names).toContain("English (US)");
    expect(names).toContain("Español");
    expect(names).toContain("Français");
    expect(names).toContain("Deutsch");
    expect(names).toContain("中文 (简体)");
  });
});

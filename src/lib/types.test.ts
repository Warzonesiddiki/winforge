import { describe, it, expect } from "vitest";
import { healthColor, RISK_ORDER, RISK_LABEL, type RiskLevel } from "./types";

describe("healthColor", () => {
  it("returns Excellent for scores >= 80", () => {
    expect(healthColor(80).label).toBe("Excellent");
    expect(healthColor(100).label).toBe("Excellent");
    expect(healthColor(95).hex).toBe("#4CAF50");
  });

  it("returns Good for 60-79", () => {
    expect(healthColor(60).label).toBe("Good");
    expect(healthColor(79).label).toBe("Good");
    expect(healthColor(70).hex).toBe("#FFC107");
  });

  it("returns Needs Attention for 40-59", () => {
    expect(healthColor(40).label).toBe("Needs Attention");
    expect(healthColor(59).label).toBe("Needs Attention");
    expect(healthColor(50).hex).toBe("#FF9800");
  });

  it("returns Critical for < 40", () => {
    expect(healthColor(39).label).toBe("Critical");
    expect(healthColor(0).label).toBe("Critical");
    expect(healthColor(10).hex).toBe("#F44336");
  });
});

describe("risk level constants", () => {
  it("orders risk levels from least to most severe", () => {
    expect(RISK_ORDER.low).toBeLessThan(RISK_ORDER.medium);
    expect(RISK_ORDER.medium).toBeLessThan(RISK_ORDER.high);
    expect(RISK_ORDER.high).toBeLessThan(RISK_ORDER.expert);
  });

  it("provides a human label for every risk level", () => {
    const levels: RiskLevel[] = ["low", "medium", "high", "expert"];
    for (const level of levels) {
      expect(RISK_LABEL[level]).toBeTruthy();
      expect(RISK_LABEL[level].length).toBeGreaterThan(0);
    }
  });
});

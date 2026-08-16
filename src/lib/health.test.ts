import { describe, it, expect, vi, beforeEach } from "vitest";

// Mock @/db before importing health. The mock inspects the Drizzle table
// object's columns to decide which fixture rows to return, and always
// provides a .where() so both `db.select().from(t)` and
// `db.select().from(t).where(...)` work.
const fixtures = vi.hoisted(() => ({
  tweaks: [] as unknown[],
  bloat: [] as unknown[],
  privacy: [] as unknown[],
  updates: [] as unknown[],
}));

function rowsForTable(table: { [k: string]: unknown }): unknown[] {
  const keys = Object.keys(table);
  if (keys.includes("operations")) return fixtures.tweaks;
  if (keys.includes("provisionedRemoved")) return fixtures.bloat;
  if (keys.includes("kb") && keys.includes("severity")) return fixtures.updates;
  if (keys.includes("enabled")) return fixtures.privacy;
  return [];
}

vi.mock("@/db", () => ({
  db: {
    select: vi.fn(() => ({
      from: (table: { [k: string]: unknown }) => {
        const rows = rowsForTable(table);
        const query = Promise.resolve(rows);
        return Object.assign(query, {
          where: () => Promise.resolve(rows),
        });
      },
    })),
  },
}));

import { computeHealthReport } from "./health";

interface TweakRow {
  id: string;
  applied: boolean;
  risk: "low" | "medium" | "high" | "expert";
  defaultEnabled: boolean;
}
interface BloatRow {
  category: string;
  status: string;
}
interface PrivacyRow {
  enabled: boolean;
}
interface UpdateRow {
  installed: boolean;
  hidden: boolean;
  severity: string;
}

function setFixtures(f: {
  tweaks?: TweakRow[];
  bloat?: BloatRow[];
  privacy?: PrivacyRow[];
  updates?: UpdateRow[];
}) {
  fixtures.tweaks = f.tweaks ?? [];
  fixtures.bloat = f.bloat ?? [];
  fixtures.privacy = f.privacy ?? [];
  fixtures.updates = f.updates ?? [];
}

describe("computeHealthReport", () => {
  beforeEach(() => setFixtures({}));

  it("computes a baseline score on an empty catalog", async () => {
    // Empty catalog: privacyScore defaults to 100 (privacyBonus=15),
    // telemetry enabled (-5). The default-enabled bonus requires at least
    // one default-enabled tweak, so it does not fire here.
    // 50 + 15 - 5 = 60.
    const report = await computeHealthReport();
    expect(report.score).toBe(60);
    expect(report.bloatwareCount).toBe(0);
    expect(report.telemetryEnabled).toBe(true);
    expect(report.privacyScore).toBe(100);
  });

  it("clamps score to [0, 100] under maximum penalties", async () => {
    const tweaks: TweakRow[] = Array.from({ length: 30 }, (_, i) => ({
      id: `t${i}`,
      applied: false,
      risk: "high",
      defaultEnabled: false,
    }));
    const updates: UpdateRow[] = Array.from({ length: 10 }, () => ({
      installed: false,
      hidden: false,
      severity: "Critical",
    }));
    setFixtures({
      tweaks,
      bloat: Array.from({ length: 60 }, () => ({ category: "Bloat", status: "installed" })),
      updates,
    });
    const report = await computeHealthReport();
    expect(report.score).toBeGreaterThanOrEqual(0);
    expect(report.score).toBeLessThanOrEqual(100);
  });

  it("rewards applied tweaks up to +20", async () => {
    setFixtures({
      tweaks: Array.from({ length: 10 }, (_, i) => ({
        id: `a${i}`,
        applied: true,
        risk: "low",
        defaultEnabled: false,
      })),
    });
    const report = await computeHealthReport();
    // baseline 50 + min(20, 10*2)=20 + privacyBonus 15 - telemetry 5 = 80
    expect(report.appliedTweaksCount).toBe(10);
    expect(report.score).toBe(80);
  });

  it("caps the tweak bonus at +20", async () => {
    setFixtures({
      tweaks: Array.from({ length: 50 }, (_, i) => ({
        id: `a${i}`,
        applied: true,
        risk: "low",
        defaultEnabled: false,
      })),
    });
    const report = await computeHealthReport();
    // Same as 10 applied because bonus is capped at 20.
    expect(report.score).toBe(80);
  });

  it("penalizes pending security updates up to -10", async () => {
    setFixtures({
      updates: [
        { installed: false, hidden: false, severity: "Critical" },
        { installed: false, hidden: false, severity: "Important" },
        { installed: false, hidden: false, severity: "Critical" },
      ],
    });
    const report = await computeHealthReport();
    expect(report.pendingSecurityUpdates).toBe(3);
    // baseline 50 + privacyBonus 15 - min(10,15)=10 - telemetry 5 + default 5 = 55
    expect(report.score).toBe(50); // 50 + privacy(15) - security(10) - telemetry(5)
  });

  it("penalizes heavy bloatware (>50 packages) by 5", async () => {
    setFixtures({
      bloat: Array.from({ length: 60 }, () => ({ category: "Bloat", status: "installed" })),
    });
    const report = await computeHealthReport();
    expect(report.bloatwareCount).toBe(60);
    // baseline 50 + privacyBonus 15 - telemetry 5 + default 5 - 5 heavy bloat = 60
    expect(report.score).toBe(55); // 50 + privacy(15) - telemetry(5) - heavy bloat(5)
  });

  it("counts removed bloatware for the bonus", async () => {
    setFixtures({
      bloat: [
        ...Array.from({ length: 30 }, () => ({ category: "Bloat", status: "installed" })),
        ...Array.from({ length: 30 }, () => ({ category: "Bloat", status: "removed" })),
      ],
    });
    const report = await computeHealthReport();
    expect(report.bloatwareCount).toBe(30);
    expect(report.removedBloatCount).toBe(30);
    // 30 removed → min(15, 30*0.5)=15 debloat bonus
    // baseline 50 + debloat 15 + privacy 15 - telemetry 5 = 75
    expect(report.score).toBe(75);
  });

  it("awards +5 only when all default-enabled tweaks are applied", async () => {
    setFixtures({
      tweaks: [
        { id: "d1", applied: true, risk: "low", defaultEnabled: true },
        { id: "d2", applied: true, risk: "medium", defaultEnabled: true },
      ],
    });
    const report = await computeHealthReport();
    // baseline 50 + min(20, 2*2)=4 + privacy 15 - telemetry 5 + default bonus 5 = 69
    expect(report.score).toBe(69);
  });

  it("does not award the default bonus when a default-enabled tweak is unapplied", async () => {
    setFixtures({
      tweaks: [
        { id: "d1", applied: true, risk: "low", defaultEnabled: true },
        { id: "d2", applied: false, risk: "medium", defaultEnabled: true },
      ],
    });
    const report = await computeHealthReport();
    // baseline 50 + min(20, 1*2)=2 + privacy 15 - telemetry 5 (no default bonus) = 62
    expect(report.score).toBe(62);
  });

  it("computes privacy score as a percentage", async () => {
    setFixtures({
      privacy: [
        { enabled: true },
        { enabled: true },
        { enabled: false },
        { enabled: false },
      ],
    });
    const report = await computeHealthReport();
    expect(report.privacyScore).toBe(50);
    // baseline 50 + privacyBonus round(50*0.15)=8 - telemetry 5 + default 5 = 58
    expect(report.score).toBe(53); // 50 + privacy(8) - telemetry(5)
  });

  it("treats telemetry as disabled when tel-disable-telemetry is applied", async () => {
    setFixtures({
      tweaks: [{ id: "tel-disable-telemetry", applied: true, risk: "medium", defaultEnabled: false }],
    });
    const report = await computeHealthReport();
    expect(report.telemetryEnabled).toBe(false);
    // baseline 50 + min(20,1*2)=2 + privacy 15 - 0 (telemetry off) + default 5 = 72
    expect(report.score).toBe(67); // 50 + tweak(2) + privacy(15), telemetry off
  });
});

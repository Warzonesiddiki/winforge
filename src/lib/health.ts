import { db } from "@/db";
import {
  debloatPackages,
  privacyRules,
  tweaks,
  windowsUpdates,
} from "@/db/schema";
import { eq, and, ne } from "drizzle-orm";

export interface SystemHealthReport {
  score: number;
  bloatwareCount: number;
  unappliedLowRiskTweaks: number;
  unappliedMedRiskTweaks: number;
  unappliedHighRiskTweaks: number;
  pendingUpdates: number;
  pendingSecurityUpdates: number;
  privacyScore: number;
  telemetryEnabled: boolean;
  quickWins: string[];
  warnings: string[];
  appliedTweaksCount: number;
  totalTweaksCount: number;
  removedBloatCount: number;
}

/**
 * Implements the WEB CONTROL CENTER health-score algorithm against live DB state.
 *
 * The algorithm balances penalties (bloatware, unapplied tweaks) with bonuses
 * (applied tweaks, removed bloat, privacy hardening) to give a fair score.
 *
 * NOTE: This is intentionally different from the Go engine's formula in
 * internal/tweak/health.go (baseline 100, linear per-risk penalties). The two
 * surfaces score different inputs — the web app reads a simulated PostgreSQL
 * catalog, the engine reads real registry/service state — so they are not
 * expected to produce identical numbers. Each has tests pinning its behavior.
 * Do not "align" them without updating both implementations together.
 */
export async function computeHealthReport(): Promise<SystemHealthReport> {
  const allTweaks = await db.select().from(tweaks);
  const allBloat = await db.select().from(debloatPackages).where(ne(debloatPackages.category, "Protected"));
  const bloatInstalled = allBloat.filter((p) => p.status === "installed");
  const bloatRemoved = allBloat.filter((p) => p.status === "removed");
  const privacy = await db.select().from(privacyRules);
  const updates = await db
    .select()
    .from(windowsUpdates)
    .where(and(eq(windowsUpdates.installed, false), eq(windowsUpdates.hidden, false)));

  const appliedTweaks = allTweaks.filter((x) => x.applied);
  const unappliedLowRiskTweaks = allTweaks.filter((x) => !x.applied && x.risk === "low").length;
  const unappliedMedRiskTweaks = allTweaks.filter((x) => !x.applied && x.risk === "medium").length;
  const unappliedHighRiskTweaks = allTweaks.filter((x) => !x.applied && (x.risk === "high" || x.risk === "expert")).length;

  const bloatwareCount = bloatInstalled.length;
  const removedBloatCount = bloatRemoved.length;

  const pendingSecurityUpdates = updates.filter((u) => u.severity === "Critical" || u.severity === "Important").length;
  const pendingOptionalUpdates = updates.filter((u) => u.severity === "Optional" || u.severity === "Feature").length;

  const enabledPrivacy = privacy.filter((p) => p.enabled).length;
  const privacyScore = privacy.length === 0 ? 100 : Math.round((enabledPrivacy / privacy.length) * 100);

  const telemetryTweak = allTweaks.find((x) => x.id === "tel-disable-telemetry");
  const telemetryEnabled = telemetryTweak ? !telemetryTweak.applied : true;

  // New balanced scoring algorithm:
  // Start at 50 (neutral baseline) and adjust based on actions taken
  let score = 50;

  // Bonuses for positive actions (max +50)
  const tweakBonus = Math.min(20, appliedTweaks.length * 2); // Up to +20 for applied tweaks
  const debloatBonus = Math.min(15, removedBloatCount * 0.5); // Up to +15 for removed bloat
  const privacyBonus = Math.round(privacyScore * 0.15); // Up to +15 for privacy score
  score += tweakBonus + debloatBonus + privacyBonus;

  // Penalties for issues (max -50)
  score -= Math.min(10, pendingSecurityUpdates * 5); // Max -10 for security updates
  score -= Math.min(5, pendingOptionalUpdates * 1); // Max -5 for optional updates
  score -= telemetryEnabled ? 5 : 0; // -5 if telemetry still enabled

  // Light penalty for high bloatware (encourages cleanup but doesn't destroy score)
  if (bloatwareCount > 50) score -= 5;
  else if (bloatwareCount > 30) score -= 3;

  // Bonus for having most default-enabled tweaks applied
  const defaultEnabledTweaks = allTweaks.filter((t) => t.defaultEnabled);
  const defaultApplied = defaultEnabledTweaks.filter((t) => t.applied).length;
  if (defaultEnabledTweaks.length > 0 && defaultApplied === defaultEnabledTweaks.length) {
    score += 5; // Bonus for applying all recommended tweaks
  }

  score = Math.max(0, Math.min(100, Math.round(score)));

  // Generate actionable quick wins
  const quickWins: string[] = [];

  if (telemetryEnabled) {
    quickWins.push("Disable Telemetry to reduce data collection");
  }
  if (bloatwareCount > 20) {
    quickWins.push(`Remove ${bloatwareCount} detected bloatware packages`);
  }
  if (pendingSecurityUpdates > 0) {
    quickWins.push(`Install ${pendingSecurityUpdates} pending security update(s)`);
  }
  if (unappliedLowRiskTweaks > 5) {
    quickWins.push(`Apply ${unappliedLowRiskTweaks} safe low-risk tweaks`);
  }
  if (privacyScore < 60) {
    quickWins.push("Run Privacy → Harden All to raise your privacy score");
  }
  if (appliedTweaks.length < 5) {
    quickWins.push("Apply a preset to quickly optimize your system");
  }

  // Fill with positive messages if system is well-optimized
  if (quickWins.length === 0) {
    quickWins.push("Your system is well optimized!");
  }
  if (quickWins.length < 2 && removedBloatCount > 10) {
    quickWins.push(`Great progress! ${removedBloatCount} bloatware packages removed`);
  }
  if (quickWins.length < 3 && appliedTweaks.length > 10) {
    quickWins.push(`${appliedTweaks.length} optimizations active — check History for details`);
  }

  // Generate warnings for critical issues
  const warnings: string[] = [];
  if (bloatwareCount > 50) {
    warnings.push(`${bloatwareCount} bloatware packages still installed`);
  }
  if (pendingSecurityUpdates > 0) {
    warnings.push(`${pendingSecurityUpdates} pending security update(s) — install soon`);
  }
  if (telemetryEnabled) {
    warnings.push("Windows telemetry is currently enabled");
  }
  if (privacyScore < 30) {
    warnings.push("Privacy score is critically low");
  }

  return {
    score,
    bloatwareCount,
    unappliedLowRiskTweaks,
    unappliedMedRiskTweaks,
    unappliedHighRiskTweaks,
    pendingUpdates: updates.length,
    pendingSecurityUpdates,
    privacyScore,
    telemetryEnabled,
    quickWins: quickWins.slice(0, 3),
    warnings,
    appliedTweaksCount: appliedTweaks.length,
    totalTweaksCount: allTweaks.length,
    removedBloatCount,
  };
}

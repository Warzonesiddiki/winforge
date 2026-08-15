import { NextResponse } from "next/server";
import { ensureSeeded } from "@/db/seed";
import { computeHealthReport } from "@/lib/health";
import { db } from "@/db";
import { applications, appSettings, debloatPackages, operationHistory, privacyRules, restorePoints, scheduledTasks, services, snapshots, tweaks, windowsUpdates } from "@/db/schema";
import { and, desc, eq, gte, ne } from "drizzle-orm";

export const dynamic = "force-dynamic";

/**
 * Comprehensive status endpoint for CLI/automation integration.
 * Returns full system health report and statistics.
 */
export async function GET() {
  await ensureSeeded();

  const health = await computeHealthReport();

  const allTweaks = await db.select().from(tweaks);
  const appliedTweaks = allTweaks.filter((t) => t.applied);
  const bloatInstalled = await db.select().from(debloatPackages).where(and(eq(debloatPackages.status, "installed"), ne(debloatPackages.category, "Protected")));
  const bloatRemoved = await db.select().from(debloatPackages).where(eq(debloatPackages.status, "removed"));
  const privacyEnabled = await db.select().from(privacyRules).where(eq(privacyRules.enabled, true));
  const privacyTotal = await db.select().from(privacyRules);
  const appsInstalled = await db.select().from(applications).where(eq(applications.installed, true));
  const appsTotal = await db.select().from(applications);
  const pendingUpdates = await db.select().from(windowsUpdates).where(and(eq(windowsUpdates.installed, false), eq(windowsUpdates.hidden, false)));
  const restorePointsList = await db.select().from(restorePoints).orderBy(desc(restorePoints.sequenceNumber)).limit(5);
  const allServices = await db.select().from(services);
  const allTasks = await db.select().from(scheduledTasks);
  const snapshotRows = await db.select().from(snapshots);
  const [settingsRow] = await db.select().from(appSettings).limit(1);
  const today = new Date();
  today.setHours(0, 0, 0, 0);
  const recentOps = await db.select().from(operationHistory).where(gte(operationHistory.timestamp, today)).orderBy(desc(operationHistory.timestamp)).limit(10);

  return NextResponse.json({
    version: "2.0.0",
    timestamp: new Date().toISOString(),
    settings: settingsRow ? {
      theme: settingsRow.theme,
      language: settingsRow.language,
      backdrop: settingsRow.backdrop,
      restorePointBeforeMutation: settingsRow.restorePointBeforeMutation,
      showExpertTweaks: settingsRow.showExpertTweaks,
    } : undefined,
    health: {
      score: health.score,
      label: health.score >= 80 ? "Excellent" : health.score >= 60 ? "Good" : health.score >= 40 ? "Needs Attention" : "Critical",
      privacyScore: health.privacyScore,
      telemetryEnabled: health.telemetryEnabled,
      quickWins: health.quickWins,
      warnings: health.warnings,
    },
    statistics: {
      tweaks: {
        applied: appliedTweaks.length,
        total: allTweaks.length,
        byCategory: Object.fromEntries(
          Array.from(new Set(allTweaks.map((t) => t.category))).map((cat) => [
            cat,
            { applied: appliedTweaks.filter((t) => t.category === cat).length, total: allTweaks.filter((t) => t.category === cat).length },
          ])
        ),
      },
      debloat: {
        installed: bloatInstalled.length,
        removed: bloatRemoved.length,
      },
      privacy: {
        enabled: privacyEnabled.length,
        total: privacyTotal.length,
        score: health.privacyScore,
      },
      applications: {
        installed: appsInstalled.length,
        available: appsTotal.length,
      },
      updates: {
        pending: pendingUpdates.length,
        pendingCritical: pendingUpdates.filter((u) => u.severity === "Critical").length,
      },
    },
    restorePoints: restorePointsList.map((rp) => ({
      sequenceNumber: rp.sequenceNumber,
      description: rp.description,
      createdAt: rp.createdAt.toISOString(),
    })),
    services: {
      total: allServices.length,
      protected: allServices.filter((s) => s.protected).length,
      disabled: allServices.filter((s) => s.startType === "Disabled").length,
      running: allServices.filter((s) => s.status === "Running").length,
    },
    scheduledTasks: {
      total: allTasks.length,
      disabled: allTasks.filter((t) => !t.enabled).length,
    },
    snapshots: {
      total: snapshotRows.length,
    },
    recentOperations: recentOps.map((op) => ({
      id: op.id,
      timestamp: op.timestamp.toISOString(),
      type: op.operationType,
      target: op.target,
      success: op.success,
      canUndo: op.canUndo && !op.undone,
    })),
  });
}

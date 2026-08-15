import { sql } from "drizzle-orm";
import { db } from "@/db";
import {
  appSettings,
  applications,
  communityPacks,
  contextMenuItems,
  debloatPackages,
  healthHistory,
  presets,
  privacyRules,
  scheduledTasks,
  services,
  startupItems,
  tweaks,
  windowsUpdates,
} from "@/db/schema";
import {
  appsSeed,
  communityPacksSeed,
  contextMenuItemsSeed,
  debloatSeed,
  healthHistorySeed,
  presetsSeed,
  privacySeed,
  servicesSeed,
  startupItemsSeed,
  tasksSeed,
  tweaksSeed,
  updatesSeed,
} from "@/db/seed-data";

let seeded = false;

/**
 * Idempotently merges the catalog tables with the latest seed data.
 *
 * Every call re-issues `INSERT ... ON CONFLICT DO NOTHING`, which:
 *  - inserts newly-added catalog entries (new tweaks, new bloatware
 *    packages, new privacy rules) into already-provisioned databases, and
 *  - never touches existing rows, so user-applied state (applied toggles,
 *    package statuses, enabled rules) is preserved.
 */
export async function ensureSeeded(): Promise<void> {
  if (seeded) return;

  await db.insert(tweaks).values(
    tweaksSeed.map((tw) => ({
      id: tw.id,
      name: tw.name,
      description: tw.description,
      category: tw.category,
      risk: tw.risk,
      defaultEnabled: tw.defaultEnabled,
      applied: tw.defaultEnabled,
      tags: tw.tags,
      warningMessage: tw.warningMessage ?? null,
      breaksFeatures: tw.breaksFeatures ?? [],
      operations: tw.operations,
      undoOperations: tw.undoOperations,
    }))
  ).onConflictDoNothing();

  await db.insert(debloatPackages).values(
    debloatSeed.map((p) => ({
      packageName: p.packageName,
      displayName: p.displayName,
      category: p.category,
      risk: p.risk,
      canReinstall: p.canReinstall,
      storeId: p.storeId ?? null,
      status: (p.category === "Protected" ? "protected" : "installed") as
        | "protected"
        | "installed",
    }))
  ).onConflictDoNothing();

  await db.insert(privacyRules).values(
    privacySeed.map((r) => ({
      id: r.id,
      name: r.name,
      description: r.description,
      category: r.category,
      risk: r.risk,
      defaultEnabled: r.defaultEnabled,
      enabled: r.defaultEnabled,
    }))
  ).onConflictDoNothing();

  await db.insert(applications).values(
    appsSeed.map((a) => ({
      id: a.id,
      name: a.name,
      publisher: a.publisher,
      category: a.category,
      version: a.version,
      installed: a.installed ?? false,
    }))
  ).onConflictDoNothing();

  await db.insert(presets).values(presetsSeed).onConflictDoNothing();

  await db.insert(windowsUpdates).values(updatesSeed).onConflictDoNothing();

  await db.insert(startupItems).values(startupItemsSeed).onConflictDoNothing();

  await db.insert(services).values(servicesSeed).onConflictDoNothing();

  await db.insert(scheduledTasks).values(tasksSeed).onConflictDoNothing();

  await db.insert(contextMenuItems).values(contextMenuItemsSeed).onConflictDoNothing();

  await db.insert(communityPacks).values(communityPacksSeed).onConflictDoNothing();

  // Health history uses a serial PK — guard against duplicate rows per boot.
  const [{ count }] = await db
    .execute<{ count: string }>(sql`select count(*)::text as count from ${healthHistory}`)
    .then((r) => r.rows as { count: string }[]);
  if (Number(count) === 0) {
    await db.insert(healthHistory).values(healthHistorySeed);
  }

  await db
    .insert(appSettings)
    .values({ id: 1 })
    .onConflictDoNothing();

  seeded = true;
}

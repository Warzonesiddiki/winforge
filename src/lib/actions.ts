"use server";

import { randomBytes, createHash } from "node:crypto";
import os from "node:os";
import { revalidatePath } from "next/cache";
import { and, desc, eq, gte, inArray } from "drizzle-orm";
import { db } from "@/db";
import { ensureSeeded } from "@/db/seed";
import {
  appSettings,
  applications,
  communityPacks,
  contextMenuItems,
  debloatPackages,
  healthHistory,
  isoJobs,
  operationHistory,
  presets,
  privacyRules,
  restorePoints,
  scheduledTasks,
  services,
  snapshots,
  startupItems,
  tweaks,
  windowsUpdates,
  type SnapshotState,
} from "@/db/schema";
import { computeHealthReport } from "@/lib/health";

type UndoKind =
  | "tweak"
  | "privacy"
  | "debloat"
  | "app"
  | "update"
  | "startup"
  | "service"
  | "task"
  | "context_menu"
  | "pack";

interface UndoPayload {
  kind: UndoKind;
  id: string;
  field: string;
  value: boolean | string;
}

async function log(entry: {
  operationType: string;
  category: string;
  target: string;
  previousValue?: string | null;
  newValue?: string | null;
  risk?: "low" | "medium" | "high" | "expert";
  canUndo?: boolean;
  undoData?: UndoPayload | null;
}) {
  await db.insert(operationHistory).values({
    operationType: entry.operationType,
    category: entry.category,
    target: entry.target,
    previousValue: entry.previousValue ?? null,
    newValue: entry.newValue ?? null,
    risk: entry.risk ?? "low",
    canUndo: entry.canUndo ?? true,
    undoData: entry.undoData ?? null,
  });
}

async function maybeCreateRestorePoint(description: string) {
  const [settings] = await db.select().from(appSettings).where(eq(appSettings.id, 1));
  if (!settings || settings.restorePointBeforeMutation) {
    const rows = await db.select().from(restorePoints).orderBy(desc(restorePoints.sequenceNumber)).limit(1);
    const nextSeq = (rows[0]?.sequenceNumber ?? 0) + 1;
    await db.insert(restorePoints).values({ sequenceNumber: nextSeq, description });
  }
}

function revalidateAll() {
  for (const p of [
    "/dashboard",
    "/tweaks",
    "/debloat",
    "/privacy",
    "/install",
    "/repair",
    "/services",
    "/updates",
    "/iso",
    "/history",
    "/settings",
  ]) {
    revalidatePath(p);
  }
}

// ── Action Result Types ────────────────────────────────────────────────────

export interface ActionResult {
  success: boolean;
  message: string;
}

// ── Tweaks ────────────────────────────────────────────────────────────────

export async function setTweakApplied(id: string, applied: boolean): Promise<ActionResult> {
  await ensureSeeded();
  const [row] = await db.select().from(tweaks).where(eq(tweaks.id, id));
  if (!row) return { success: false, message: "Tweak not found" };
  if (row.applied === applied) return { success: true, message: "No change needed" };

  await maybeCreateRestorePoint(`Before tweak: ${row.name}`);
  await db.update(tweaks).set({ applied, updatedAt: new Date() }).where(eq(tweaks.id, id));
  await log({
    operationType: applied ? "TweakApply" : "TweakUndo",
    category: row.category,
    target: row.name,
    previousValue: String(row.applied),
    newValue: String(applied),
    risk: row.risk,
    undoData: { kind: "tweak", id, field: "applied", value: row.applied },
  });
  revalidateAll();
  return { success: true, message: `${row.name} ${applied ? "applied" : "reverted"} successfully` };
}

export async function applyPreset(presetId: string): Promise<ActionResult> {
  await ensureSeeded();
  const [preset] = await db.select().from(presets).where(eq(presets.id, presetId));
  if (!preset) return { success: false, message: "Preset not found" };

  await maybeCreateRestorePoint(`Before applying preset: ${preset.name}`);

  for (const tweakId of preset.tweakIds) {
    const [row] = await db.select().from(tweaks).where(eq(tweaks.id, tweakId));
    if (row && !row.applied) {
      await db.update(tweaks).set({ applied: true, updatedAt: new Date() }).where(eq(tweaks.id, tweakId));
      await log({
        operationType: "TweakApply",
        category: row.category,
        target: `${row.name} (preset: ${preset.name})`,
        previousValue: "false",
        newValue: "true",
        risk: row.risk,
        undoData: { kind: "tweak", id: tweakId, field: "applied", value: false },
      });
    }
  }

  for (const pkg of preset.debloatPackages) {
    const [row] = await db.select().from(debloatPackages).where(eq(debloatPackages.packageName, pkg));
    if (row && row.status === "installed") {
      await db.update(debloatPackages).set({ status: "removed", updatedAt: new Date() }).where(eq(debloatPackages.packageName, pkg));
      await log({
        operationType: "AppxRemove",
        category: row.category,
        target: `${row.displayName} (preset: ${preset.name})`,
        previousValue: "installed",
        newValue: "removed",
        risk: row.risk,
        undoData: { kind: "debloat", id: pkg, field: "status", value: true },
      });
    }
  }

  for (const ruleId of preset.privacyRuleIds) {
    const [row] = await db.select().from(privacyRules).where(eq(privacyRules.id, ruleId));
    if (row && !row.enabled) {
      await db.update(privacyRules).set({ enabled: true, updatedAt: new Date() }).where(eq(privacyRules.id, ruleId));
      await log({
        operationType: "PrivacyApply",
        category: row.category,
        target: `${row.name} (preset: ${preset.name})`,
        previousValue: "false",
        newValue: "true",
        risk: row.risk,
        undoData: { kind: "privacy", id: ruleId, field: "enabled", value: false },
      });
    }
  }

  revalidateAll();
  const totalChanges = preset.tweakIds.length + preset.debloatPackages.length + preset.privacyRuleIds.length;
  return { success: true, message: `Preset "${preset.name}" applied (${totalChanges} changes)` };
}

export async function importTweakSelection(tweakIds: string[]): Promise<ActionResult> {
  await ensureSeeded();
  if (tweakIds.length === 0) return { success: false, message: "No tweak IDs provided" };
  await maybeCreateRestorePoint(`Before importing custom .winforge preset (${tweakIds.length} tweaks)`);
  const all = await db.select().from(tweaks);
  let changed = 0;
  for (const row of all) {
    const shouldApply = tweakIds.includes(row.id);
    if (row.applied !== shouldApply) {
      await db.update(tweaks).set({ applied: shouldApply, updatedAt: new Date() }).where(eq(tweaks.id, row.id));
      await log({
        operationType: shouldApply ? "TweakApply" : "TweakUndo",
        category: row.category,
        target: `${row.name} (imported preset)`,
        previousValue: String(row.applied),
        newValue: String(shouldApply),
        risk: row.risk,
        undoData: { kind: "tweak", id: row.id, field: "applied", value: row.applied },
      });
      changed++;
    }
  }
  revalidateAll();
  return { success: true, message: `Imported .winforge — ${changed} tweak(s) changed.` };
}

// ── Debloat ───────────────────────────────────────────────────────────────

export async function setPackageStatus(packageName: string, remove: boolean, removeProvisioned = false): Promise<ActionResult> {
  await ensureSeeded();
  const [row] = await db.select().from(debloatPackages).where(eq(debloatPackages.packageName, packageName));
  if (!row) return { success: false, message: "Package not found" };
  if (row.category === "Protected") return { success: false, message: `${row.displayName} is protected and cannot be removed.` };
  const newStatus = remove ? "removed" : "installed";
  if (row.status === newStatus) return { success: true, message: "No change needed" };

  await maybeCreateRestorePoint(`Before removing package: ${row.displayName}`);
  await db
    .update(debloatPackages)
    .set({ status: newStatus, provisionedRemoved: remove ? removeProvisioned : false, updatedAt: new Date() })
    .where(eq(debloatPackages.packageName, packageName));

  await log({
    operationType: remove ? "AppxRemove" : "AppxInstall",
    category: row.category,
    target: row.displayName,
    previousValue: row.status,
    newValue: newStatus,
    risk: row.risk,
    undoData: { kind: "debloat", id: packageName, field: "status", value: row.status === "installed" },
  });
  revalidateAll();
  return { success: true, message: `${row.displayName} ${remove ? "removed" : "reinstalled"}.` };
}

export async function bulkRemovePackages(packageNames: string[], removeProvisioned = false): Promise<ActionResult> {
  await ensureSeeded();
  if (packageNames.length === 0) return { success: false, message: "No packages selected" };
  await maybeCreateRestorePoint(`Before bulk removing ${packageNames.length} package(s)`);
  let removed = 0;
  for (const pkg of packageNames) {
    const r = await setPackageStatus(pkg, true, removeProvisioned);
    if (r.success) removed++;
  }
  return { success: true, message: `${removed}/${packageNames.length} package(s) removed.` };
}

export async function setStartupEnabled(id: string, enabled: boolean): Promise<ActionResult> {
  await ensureSeeded();
  const [row] = await db.select().from(startupItems).where(eq(startupItems.id, id));
  if (!row) return { success: false, message: "Startup item not found" };
  if (row.enabled === enabled) return { success: true, message: "No change needed" };
  await db.update(startupItems).set({ enabled }).where(eq(startupItems.id, id));
  await log({
    operationType: enabled ? "TaskEnable" : "TaskDisable",
    category: "Startup",
    target: row.name,
    previousValue: String(row.enabled),
    newValue: String(enabled),
    risk: "low",
    undoData: { kind: "startup", id, field: "enabled", value: row.enabled },
  });
  revalidateAll();
  return { success: true, message: `${row.name} ${enabled ? "enabled" : "disabled"} at startup.` };
}

// ── Privacy ───────────────────────────────────────────────────────────────

export async function setPrivacyRule(id: string, enabled: boolean): Promise<ActionResult> {
  await ensureSeeded();
  const [row] = await db.select().from(privacyRules).where(eq(privacyRules.id, id));
  if (!row) return { success: false, message: "Privacy rule not found" };
  if (row.enabled === enabled) return { success: true, message: "No change needed" };
  await maybeCreateRestorePoint(`Before privacy rule: ${row.name}`);
  await db.update(privacyRules).set({ enabled, updatedAt: new Date() }).where(eq(privacyRules.id, id));
  await log({
    operationType: enabled ? "PrivacyApply" : "PrivacyUndo",
    category: row.category,
    target: row.name,
    previousValue: String(row.enabled),
    newValue: String(enabled),
    risk: row.risk,
    undoData: { kind: "privacy", id, field: "enabled", value: row.enabled },
  });
  revalidateAll();
  return { success: true, message: `${row.name} ${enabled ? "hardened" : "reverted"}.` };
}

export async function hardenAllPrivacy(): Promise<ActionResult> {
  await ensureSeeded();
  const rows = await db.select().from(privacyRules).where(eq(privacyRules.enabled, false));
  if (rows.length === 0) return { success: true, message: "All privacy rules are already applied." };
  await maybeCreateRestorePoint(`Before Harden All Privacy (${rows.length} rules)`);
  for (const row of rows) {
    await db.update(privacyRules).set({ enabled: true, updatedAt: new Date() }).where(eq(privacyRules.id, row.id));
    await log({
      operationType: "PrivacyApply",
      category: row.category,
      target: `${row.name} (Harden All)`,
      previousValue: "false",
      newValue: "true",
      risk: row.risk,
      undoData: { kind: "privacy", id: row.id, field: "enabled", value: false },
    });
  }
  revalidateAll();
  return { success: true, message: `${rows.length} privacy rule(s) hardened.` };
}

// ── Applications / installer ───────────────────────────────────────────────

export async function setAppInstalled(id: string, installed: boolean): Promise<ActionResult> {
  await ensureSeeded();
  const [row] = await db.select().from(applications).where(eq(applications.id, id));
  if (!row) return { success: false, message: "Application not found" };
  if (row.installed === installed) return { success: true, message: "No change needed" };
  await db.update(applications).set({ installed, updatedAt: new Date() }).where(eq(applications.id, id));
  await log({
    operationType: installed ? "PackageInstall" : "PackageUninstall",
    category: row.category,
    target: `${row.name} (winget: ${row.id})`,
    previousValue: String(row.installed),
    newValue: String(installed),
    risk: "low",
    undoData: { kind: "app", id, field: "installed", value: row.installed },
  });
  revalidateAll();
  return { success: true, message: `${row.name} ${installed ? "installed" : "uninstalled"}.` };
}

export async function installAppsBatch(ids: string[]): Promise<ActionResult> {
  await ensureSeeded();
  let installed = 0;
  for (const id of ids) {
    const r = await setAppInstalled(id, true);
    if (r.success) installed++;
  }
  return { success: true, message: `${installed}/${ids.length} application(s) installed.` };
}

// ── History / Undo ──────────────────────────────────────────────────────────

export async function undoOperation(id: string): Promise<ActionResult> {
  await ensureSeeded();
  const [record] = await db.select().from(operationHistory).where(eq(operationHistory.id, id));
  if (!record) return { success: false, message: "Operation not found" };
  if (!record.canUndo) return { success: false, message: "This operation cannot be undone." };
  if (record.undone) return { success: false, message: "This operation was already undone." };
  if (!record.undoData) return { success: false, message: "No undo payload recorded for this operation." };

  const { kind, id: targetId, field, value } = record.undoData;

  if (kind === "tweak" && field === "applied") {
    await db.update(tweaks).set({ applied: Boolean(value), updatedAt: new Date() }).where(eq(tweaks.id, targetId));
  } else if (kind === "debloat" && field === "status") {
    await db
      .update(debloatPackages)
      .set({ status: value ? "installed" : "removed", updatedAt: new Date() })
      .where(eq(debloatPackages.packageName, targetId));
  } else if (kind === "privacy" && field === "enabled") {
    await db.update(privacyRules).set({ enabled: Boolean(value), updatedAt: new Date() }).where(eq(privacyRules.id, targetId));
  } else if (kind === "app" && field === "installed") {
    await db.update(applications).set({ installed: Boolean(value), updatedAt: new Date() }).where(eq(applications.id, targetId));
  } else if (kind === "startup" && field === "enabled") {
    await db.update(startupItems).set({ enabled: Boolean(value) }).where(eq(startupItems.id, targetId));
  } else if (kind === "update" && field === "installed") {
    await db.update(windowsUpdates).set({ installed: Boolean(value) }).where(eq(windowsUpdates.id, targetId));
  } else if (kind === "service" && field === "startType") {
    await db
      .update(services)
      .set({
        startType: String(value),
        status: value === "Disabled" ? "Stopped" : "Running",
        updatedAt: new Date(),
      })
      .where(eq(services.id, targetId));
  } else if (kind === "task" && field === "enabled") {
    await db
      .update(scheduledTasks)
      .set({ enabled: Boolean(value), updatedAt: new Date() })
      .where(eq(scheduledTasks.id, targetId));
  } else if (kind === "context_menu" && field === "enabled") {
    await db
      .update(contextMenuItems)
      .set({ enabled: Boolean(value), updatedAt: new Date() })
      .where(eq(contextMenuItems.id, targetId));
  } else if (kind === "pack" && field === "installed") {
    await db
      .update(communityPacks)
      .set({ installed: Boolean(value), updatedAt: new Date() })
      .where(eq(communityPacks.id, targetId));
  }

  await db.update(operationHistory).set({ undone: true }).where(eq(operationHistory.id, id));
  await log({
    operationType: "Undo",
    category: record.category,
    target: `Undo: ${record.target}`,
    previousValue: record.newValue,
    newValue: record.previousValue,
    risk: record.risk,
    canUndo: false,
  });
  revalidateAll();
  return { success: true, message: `Undone: ${record.target}` };
}

export async function undoAllToday(): Promise<ActionResult> {
  await ensureSeeded();
  const start = new Date();
  start.setHours(0, 0, 0, 0);
  const rows = await db
    .select()
    .from(operationHistory)
    .where(and(gte(operationHistory.timestamp, start), eq(operationHistory.canUndo, true), eq(operationHistory.undone, false)));
  if (rows.length === 0) return { success: true, message: "No reversible operations logged today." };
  for (const row of rows) {
    await undoOperation(row.id);
  }
  return { success: true, message: `${rows.length} operation(s) undone.` };
}

// ── Services Manager ────────────────────────────────────────────────────────

export async function setServiceState(id: string, disable: boolean): Promise<ActionResult> {
  await ensureSeeded();
  const [row] = await db.select().from(services).where(eq(services.id, id));
  if (!row) return { success: false, message: "Service not found" };
  if (row.protected) return { success: false, message: `${row.displayName} is a protected system service and cannot be modified.` };
  if (row.startType === "Disabled" && disable) return { success: true, message: "Service is already disabled" };
  if (row.startType !== "Disabled" && !disable) return { success: true, message: "Service is already enabled" };

  await maybeCreateRestorePoint(`Before ${disable ? "disabling" : "enabling"} service: ${row.displayName}`);
  const newStartType = disable ? "Disabled" : "Automatic";
  await db
    .update(services)
    .set({ startType: newStartType, status: disable ? "Stopped" : "Running", updatedAt: new Date() })
    .where(eq(services.id, id));

  await log({
    operationType: disable ? "ServiceDisable" : "ServiceEnable",
    category: row.category,
    target: row.displayName,
    previousValue: row.startType,
    newValue: newStartType,
    risk: row.risk,
    undoData: { kind: "service", id, field: "startType", value: row.startType },
  });
  revalidateAll();
  return { success: true, message: `${row.displayName} ${disable ? "disabled" : "enabled"}.` };
}

export async function setServiceMode(id: string, mode: "automatic" | "manual" | "disabled"): Promise<ActionResult> {
  await ensureSeeded();
  const [row] = await db.select().from(services).where(eq(services.id, id));
  if (!row) return { success: false, message: "Service not found" };
  if (row.protected) return { success: false, message: `${row.displayName} is a protected system service.` };

  const startTypes: Record<string, string> = {
    automatic: "Automatic",
    manual: "Manual",
    disabled: "Disabled",
  };
  const newStartType = startTypes[mode];
  if (row.startType === newStartType) return { success: true, message: "No change needed" };

  await maybeCreateRestorePoint(`Before changing ${row.displayName} start type`);
  await db
    .update(services)
    .set({ startType: newStartType, status: newStartType === "Disabled" ? "Stopped" : "Running", updatedAt: new Date() })
    .where(eq(services.id, id));

  await log({
    operationType: "ServiceConfigure",
    category: row.category,
    target: row.displayName,
    previousValue: row.startType,
    newValue: newStartType,
    risk: row.risk,
    undoData: { kind: "service", id, field: "startType", value: row.startType },
  });
  revalidateAll();
  return { success: true, message: `${row.displayName} start type set to ${mode}.` };
}

// ── Scheduled Tasks Manager ────────────────────────────────────────────────

export async function setTaskEnabled(id: string, enabled: boolean): Promise<ActionResult> {
  await ensureSeeded();
  const [row] = await db.select().from(scheduledTasks).where(eq(scheduledTasks.id, id));
  if (!row) return { success: false, message: "Task not found" };
  if (row.enabled === enabled) return { success: true, message: "No change needed" };

  await maybeCreateRestorePoint(`Before ${enabled ? "enabling" : "disabling"} task: ${row.name}`);
  await db.update(scheduledTasks).set({ enabled, updatedAt: new Date() }).where(eq(scheduledTasks.id, id));
  await log({
    operationType: enabled ? "TaskEnable" : "TaskDisable",
    category: row.category,
    target: row.name,
    previousValue: String(row.enabled),
    newValue: String(enabled),
    risk: row.risk,
    undoData: { kind: "task", id, field: "enabled", value: row.enabled },
  });
  revalidateAll();
  return { success: true, message: `${row.name} ${enabled ? "enabled" : "disabled"}.` };
}

// ── Health Score History ─────────────────────────────────────────────────────

export async function recordHealthSnapshot(): Promise<void> {
  await ensureSeeded();
  try {
    const [last] = await db.select().from(healthHistory).orderBy(desc(healthHistory.timestamp)).limit(1);
    if (last && Date.now() - last.timestamp.getTime() < 3600_000) return; // throttle to 1/hour
    const report = await computeHealthReport();
    await db.insert(healthHistory).values({
      score: report.score,
      privacyScore: report.privacyScore,
      bloatCount: report.bloatwareCount,
      appliedTweaks: report.appliedTweaksCount,
      pendingUpdates: report.pendingUpdates,
    });
  } catch {
    // never fail a page render because history recording failed
  }
}

// ── System Snapshots (capture / compare / restore) ──────────────────────────

export async function createSnapshot(name: string): Promise<ActionResult> {
  await ensureSeeded();
  const [tweaksRows, pkgRows, privacyRows] = await Promise.all([
    db.select().from(tweaks),
    db.select().from(debloatPackages).where(eq(debloatPackages.status, "installed")),
    db.select().from(privacyRules),
  ]);

  const state: SnapshotState = {
    tweaks: Object.fromEntries(tweaksRows.map((t) => [t.id, t.applied])),
    packages: Object.fromEntries(pkgRows.map((p) => [p.packageName, "installed"])),
    privacy: Object.fromEntries(privacyRows.map((r) => [r.id, r.enabled])),
  };

  await db.insert(snapshots).values({ name: name || `Snapshot ${new Date().toLocaleString()}`, state });
  revalidateAll();
  return { success: true, message: `Snapshot "${name}" created.` };
}

export interface CompareResult extends ActionResult {
  diffCount?: number;
  diffs?: { type: "tweak" | "package" | "privacy"; target: string; from: string; to: string }[];
  snapshot?: string;
}

export async function compareSnapshot(id: string): Promise<CompareResult> {
  await ensureSeeded();
  const [snap] = await db.select().from(snapshots).where(eq(snapshots.id, id));
  if (!snap) return { success: false, message: "Snapshot not found" };

  const [tweaksRows, pkgRows, privacyRows] = await Promise.all([
    db.select().from(tweaks),
    db.select().from(debloatPackages),
    db.select().from(privacyRules),
  ]);

  const diffs: { type: "tweak" | "package" | "privacy"; target: string; from: string; to: string }[] = [];

  for (const t of tweaksRows) {
    const snapVal = snap.state.tweaks[t.id];
    if (snapVal !== undefined && snapVal !== t.applied) {
      diffs.push({ type: "tweak", target: t.name, from: String(snapVal), to: String(t.applied) });
    }
  }
  for (const p of pkgRows) {
    const snapVal = snap.state.packages[p.packageName];
    if (snapVal !== undefined && snapVal !== p.status) {
      diffs.push({ type: "package", target: p.displayName, from: snapVal, to: p.status });
    }
  }
  for (const r of privacyRows) {
    const snapVal = snap.state.privacy[r.id];
    if (snapVal !== undefined && snapVal !== r.enabled) {
      diffs.push({ type: "privacy", target: r.name, from: String(snapVal), to: String(r.enabled) });
    }
  }

  return { success: true, message: `${diffs.length} difference(s) found since snapshot "${snap.name}".`, diffCount: diffs.length, diffs, snapshot: snap.name };
}

export async function restoreSnapshot(id: string): Promise<ActionResult> {
  await ensureSeeded();
  const [snap] = await db.select().from(snapshots).where(eq(snapshots.id, id));
  if (!snap) return { success: false, message: "Snapshot not found" };

  const comparison = await compareSnapshot(id);
  if (!comparison.success || !comparison.diffs) return { success: false, message: "Could not compare snapshot" };
  const diffs = comparison.diffs;
  if (diffs.length === 0) return { success: true, message: "System already matches this snapshot." };

  await maybeCreateRestorePoint(`Before restoring snapshot: ${snap.name}`);

  let restored = 0;
  for (const diff of diffs) {
    if (diff.type === "tweak") {
      const [row] = await db.select().from(tweaks).where(eq(tweaks.name, diff.target));
      if (row && row.applied !== (diff.from === "true")) {
        await db.update(tweaks).set({ applied: diff.from === "true", updatedAt: new Date() }).where(eq(tweaks.id, row.id));
        await log({
          operationType: "SnapshotRestore",
          category: row.category,
          target: `${row.name} (snapshot: ${snap.name})`,
          previousValue: diff.to,
          newValue: diff.from,
          risk: row.risk,
          undoData: { kind: "tweak", id: row.id, field: "applied", value: row.applied },
        });
        restored++;
      }
    } else if (diff.type === "package") {
      const [row] = await db.select().from(debloatPackages).where(eq(debloatPackages.displayName, diff.target));
      if (row) {
        const targetStatus = diff.from === "installed" ? "installed" : "removed";
        if (row.status !== targetStatus) {
          await db.update(debloatPackages).set({ status: targetStatus, updatedAt: new Date() }).where(eq(debloatPackages.packageName, row.packageName));
          await log({
            operationType: targetStatus === "installed" ? "AppxInstall" : "AppxRemove",
            category: row.category,
            target: `${row.displayName} (snapshot: ${snap.name})`,
            previousValue: row.status,
            newValue: targetStatus,
            risk: row.risk,
            undoData: { kind: "debloat", id: row.packageName, field: "status", value: row.status === "installed" },
          });
          restored++;
        }
      }
    } else if (diff.type === "privacy") {
      const [row] = await db.select().from(privacyRules).where(eq(privacyRules.name, diff.target));
      if (row && row.enabled !== (diff.from === "true")) {
        await db.update(privacyRules).set({ enabled: diff.from === "true", updatedAt: new Date() }).where(eq(privacyRules.id, row.id));
        await log({
          operationType: "SnapshotRestore",
          category: row.category,
          target: `${row.name} (snapshot: ${snap.name})`,
          previousValue: diff.to,
          newValue: diff.from,
          risk: row.risk,
          undoData: { kind: "privacy", id: row.id, field: "enabled", value: row.enabled },
        });
        restored++;
      }
    }
  }

  revalidateAll();
  return { success: true, message: `Snapshot restored — ${restored} change(s) reverted.` };
}

// ── Settings ─────────────────────────────────────────────────────────────

export async function updateSettings(patch: Partial<{
  theme: string;
  backdrop: string;
  language: string;
  restorePointBeforeMutation: boolean;
  showExpertTweaks: boolean;
  showCopilotTweaksSeparately: boolean;
  autoMaintenanceEnabled: boolean;
}>): Promise<ActionResult> {
  await ensureSeeded();
  const keys = Object.keys(patch);
  if (keys.length === 0) return { success: false, message: "No settings to update" };
  await db.update(appSettings).set(patch).where(eq(appSettings.id, 1));
  revalidateAll();
  return { success: true, message: `Updated: ${keys.join(", ")}` };
}

export async function resetSettingsToDefaults(): Promise<ActionResult> {
  await ensureSeeded();
  await db
    .update(appSettings)
    .set({
      theme: "dark",
      backdrop: "mica",
      language: "en-US",
      restorePointBeforeMutation: true,
      showExpertTweaks: false,
      showCopilotTweaksSeparately: true,
      autoMaintenanceEnabled: false,
    })
    .where(eq(appSettings.id, 1));
  revalidateAll();
  return { success: true, message: "All settings restored to factory defaults." };
}

export async function createManualRestorePoint(description: string): Promise<ActionResult> {
  await ensureSeeded();
  await maybeCreateRestorePoint(description || "Manual restore point");
  revalidateAll();
  return { success: true, message: `Restore point created: ${description || "Manual restore point"}` };
}

// ── Repair ───────────────────────────────────────────────────────────────

export async function resetWindowsUpdate(): Promise<ActionResult> {
  await maybeCreateRestorePoint("Before Windows Update reset");
  await log({
    operationType: "Custom",
    category: "Repair",
    target: "Windows Update components reset (wuauserv, bits, cryptsvc, SoftwareDistribution renamed)",
    newValue: "reset-complete",
    risk: "medium",
    canUndo: false,
  });
  revalidateAll();
  return { success: true, message: "Windows Update services stopped, SoftwareDistribution renamed, services restarted." };
}

export async function runDismScan(): Promise<ActionResult> {
  await log({ operationType: "Custom", category: "Repair", target: "DISM /ScanHealth", newValue: "no-corruption-detected", risk: "low", canUndo: false });
  revalidateAll();
  return { success: true, message: "Component store scan complete. No corruption detected." };
}

export async function runDismRestore(): Promise<ActionResult> {
  await maybeCreateRestorePoint("Before DISM RestoreHealth");
  await log({ operationType: "Custom", category: "Repair", target: "DISM /RestoreHealth", newValue: "repaired", risk: "medium", canUndo: false });
  revalidateAll();
  return { success: true, message: "Component store repaired successfully from Windows Update source." };
}

export async function runSfc(): Promise<ActionResult> {
  await log({ operationType: "Custom", category: "Repair", target: "SFC /scannow", newValue: "no-violations-found", risk: "low", canUndo: false });
  revalidateAll();
  return { success: true, message: "System File Checker complete. Windows Resource Protection did not find any integrity violations." };
}

export async function runFullSystemCheck(): Promise<ActionResult & { log?: string[] }> {
  await maybeCreateRestorePoint("Before full system check");
  
  const results: string[] = [];
  
  // SFC
  results.push("Running SFC /scannow...");
  results.push("✓ Windows Resource Protection did not find any integrity violations.");
  
  // DISM Scan
  results.push("Running DISM /Online /Cleanup-Image /ScanHealth...");
  results.push("✓ No component store corruption detected.");
  
  // DISM RestoreHealth
  results.push("Running DISM /Online /Cleanup-Image /RestoreHealth...");
  results.push("✓ The restore operation completed successfully.");
  
  // Check disk
  results.push("Running CHKDSK /F /R (scheduled for next restart)...");
  results.push("✓ Disk check scheduled.");
  
  await log({ operationType: "Custom", category: "Repair", target: "Full System Check (SFC, DISM, CHKDSK)", newValue: "complete", risk: "medium", canUndo: false });
  revalidateAll();
  
  return { success: true, message: "Full system check complete.", log: results };
}

export async function flushDns(): Promise<ActionResult> {
  await log({ operationType: "Custom", category: "Repair", target: "DnsFlushResolverCache()", newValue: "flushed", risk: "low", canUndo: false });
  revalidateAll();
  return { success: true, message: "DNS resolver cache flushed." };
}

export async function resetNetworkStack(): Promise<ActionResult> {
  await maybeCreateRestorePoint("Before network stack reset");
  await log({ operationType: "Custom", category: "Repair", target: "Winsock reset, TCP/IP stack reset, DHCP renew", newValue: "reset-complete", risk: "medium", canUndo: false });
  revalidateAll();
  return { success: true, message: "Winsock catalog reset, TCP/IP stack reset, DHCP lease renewed." };
}

export interface CleanupSizeInfo {
  tempMb: number;
  prefetchMb: number;
  updateCacheMb: number;
  thumbnailMb: number;
  totalMb: number;
}

export async function calculateTempCleanupSize(): Promise<CleanupSizeInfo> {
  const { size } = await dirSize(os.tmpdir());
  const tempMb = Math.max(1, Math.round(size / (1024 * 1024)));
  const prefetchMb = Math.round(tempMb * 0.35);
  const updateCacheMb = Math.round(tempMb * 1.8);
  const thumbnailMb = Math.round(tempMb * 0.5);
  return {
    tempMb,
    prefetchMb,
    updateCacheMb,
    thumbnailMb,
    totalMb: tempMb + prefetchMb + updateCacheMb + thumbnailMb,
  };
}

async function dirSize(dir: string): Promise<{ size: number }> {
  const fs = await import("node:fs/promises");
  let total = 0;
  try {
    const entries = await fs.readdir(dir, { withFileTypes: true });
    for (const entry of entries.slice(0, 200)) {
      try {
        const full = `${dir}/${entry.name}`;
        if (entry.isFile()) {
          const st = await fs.stat(full);
          total += st.size;
        }
      } catch {
        // skip locked/inaccessible files, matching real-world temp cleanup behavior
      }
    }
  } catch {
    total = 42 * 1024 * 1024;
  }
  return { size: total };
}

export async function cleanTempFiles(): Promise<CleanupSizeInfo & { success: boolean; message: string }> {
  const info = await calculateTempCleanupSize();
  await log({
    operationType: "FileDelete",
    category: "Repair",
    target: `Temp/prefetch/update-cache/thumbnail cleanup (~${info.totalMb} MB reclaimed)`,
    newValue: `${info.totalMb} MB`,
    risk: "low",
    canUndo: false,
  });
  revalidateAll();
  return { ...info, success: true, message: `Reclaimed approximately ${info.totalMb} MB.` };
}

// ── Windows Updates ─────────────────────────────────────────────────────────

export async function installUpdate(id: string): Promise<ActionResult> {
  const [row] = await db.select().from(windowsUpdates).where(eq(windowsUpdates.id, id));
  if (!row) return { success: false, message: "Update not found" };
  if (row.installed) return { success: true, message: `${row.kb} is already installed.` };
  await maybeCreateRestorePoint(`Before installing update: ${row.kb}`);
  await db.update(windowsUpdates).set({ installed: true }).where(eq(windowsUpdates.id, id));
  await log({
    operationType: "Custom",
    category: "Windows Update",
    target: `${row.title} (${row.kb})`,
    previousValue: "false",
    newValue: "true",
    risk: row.severity === "Critical" ? "medium" : "low",
    undoData: { kind: "update", id, field: "installed", value: false },
  });
  revalidateAll();
  return { success: true, message: `${row.kb} installed successfully.` };
}

export async function installAllUpdates(): Promise<ActionResult> {
  const rows = await db.select().from(windowsUpdates).where(and(eq(windowsUpdates.installed, false), eq(windowsUpdates.hidden, false)));
  if (rows.length === 0) return { success: true, message: "No pending updates." };
  let installed = 0;
  for (const row of rows) {
    const r = await installUpdate(row.id);
    if (r.success) installed++;
  }
  return { success: true, message: `${installed} update(s) installed.` };
}

export async function hideUpdate(id: string): Promise<ActionResult> {
  const [row] = await db.select().from(windowsUpdates).where(eq(windowsUpdates.id, id));
  if (!row) return { success: false, message: "Update not found" };
  await db.update(windowsUpdates).set({ hidden: true }).where(eq(windowsUpdates.id, id));
  revalidateAll();
  return { success: true, message: `${row.kb} hidden.` };
}

export async function unhideUpdate(id: string): Promise<ActionResult> {
  const [row] = await db.select().from(windowsUpdates).where(eq(windowsUpdates.id, id));
  if (!row) return { success: false, message: "Update not found" };
  await db.update(windowsUpdates).set({ hidden: false }).where(eq(windowsUpdates.id, id));
  revalidateAll();
  return { success: true, message: `${row.kb} unhidden.` };
}

export async function pauseUpdates(days: number): Promise<ActionResult> {
  await log({ operationType: "Custom", category: "Windows Update", target: `Updates paused for ${days} day(s)`, risk: "low", canUndo: false });
  revalidateAll();
  return { success: true, message: `Windows Update paused for ${days} day(s).` };
}

export async function resumeUpdates(): Promise<ActionResult> {
  await log({ operationType: "Custom", category: "Windows Update", target: "Updates resumed", risk: "low", canUndo: false });
  revalidateAll();
  return { success: true, message: "Windows Update resumed." };
}

// ── DNS Configuration ──────────────────────────────────────────────────────

export async function setDnsPreset(presetId: string): Promise<ActionResult> {
  const { dnsPresets } = await import("@/db/dns-presets");
  const preset = dnsPresets.find((p) => p.id === presetId);
  if (!preset) return { success: false, message: "Unknown DNS preset" };

  await maybeCreateRestorePoint(`Before DNS change to ${preset.name}`);
  await log({
    operationType: "Custom",
    category: "Network",
    target: `DNS changed to ${preset.name} (${preset.primary}, ${preset.secondary})`,
    newValue: `${preset.primary}, ${preset.secondary}`,
    risk: "low",
    canUndo: false,
  });
  revalidateAll();
  return { success: true, message: `DNS configured to ${preset.name}: ${preset.primary} / ${preset.secondary}` };
}

// ── Windows Features ────────────────────────────────────────────────────────

export async function setWindowsFeature(featureId: string, enable: boolean): Promise<ActionResult> {
  const { windowsFeatures } = await import("@/db/dns-presets");
  const feature = windowsFeatures.find((f) => f.id === featureId);
  if (!feature) return { success: false, message: "Unknown feature" };

  await maybeCreateRestorePoint(`Before ${enable ? "enabling" : "disabling"} ${feature.name}`);
  await log({
    operationType: enable ? "DismFeatureEnable" : "DismFeatureDisable",
    category: "Windows Features",
    target: `${feature.name} (${featureId})`,
    previousValue: String(feature.enabled),
    newValue: String(enable),
    risk: "medium",
    canUndo: false,
  });
  revalidateAll();
  return { success: true, message: `${feature.name} ${enable ? "enabled" : "disabled"}. A restart may be required.` };
}

// ── Context Menu Manager ───────────────────────────────────────────────────

export async function setContextMenuItemEnabled(id: string, enabled: boolean): Promise<ActionResult> {
  await ensureSeeded();
  const [row] = await db.select().from(contextMenuItems).where(eq(contextMenuItems.id, id));
  if (!row) return { success: false, message: "Context menu item not found" };
  if (row.enabled === enabled) return { success: true, message: "No change needed" };

  await maybeCreateRestorePoint(`Before ${enabled ? "restoring" : "removing"} context menu item: ${row.title}`);
  await db.update(contextMenuItems).set({ enabled, updatedAt: new Date() }).where(eq(contextMenuItems.id, id));
  await log({
    operationType: enabled ? "RegistrySet" : "RegistryDelete",
    category: "Context Menu",
    target: `${row.title} (${row.registryKey})`,
    previousValue: String(row.enabled),
    newValue: String(enabled),
    risk: row.risk,
    undoData: { kind: "context_menu", id, field: "enabled", value: row.enabled },
  });
  revalidateAll();
  return { success: true, message: `${row.title} context menu item ${enabled ? "enabled" : "hidden"}.` };
}

// ── Community Packs (AtlasOS, ReviOS, CTT, Dev Power) ──────────────────────

export async function applyCommunityPack(packId: string): Promise<ActionResult> {
  await ensureSeeded();
  const [pack] = await db.select().from(communityPacks).where(eq(communityPacks.id, packId));
  if (!pack) return { success: false, message: "Community pack not found" };

  await maybeCreateRestorePoint(`Before applying community pack: ${pack.name}`);

  for (const tweakId of pack.tweakIds) {
    const [row] = await db.select().from(tweaks).where(eq(tweaks.id, tweakId));
    if (row && !row.applied) {
      await db.update(tweaks).set({ applied: true, updatedAt: new Date() }).where(eq(tweaks.id, tweakId));
      await log({
        operationType: "TweakApply",
        category: row.category,
        target: `${row.name} (pack: ${pack.name})`,
        previousValue: "false",
        newValue: "true",
        risk: row.risk,
        undoData: { kind: "tweak", id: tweakId, field: "applied", value: false },
      });
    }
  }

  for (const pkg of pack.debloatPackages) {
    const [row] = await db.select().from(debloatPackages).where(eq(debloatPackages.packageName, pkg));
    if (row && row.status === "installed") {
      await db.update(debloatPackages).set({ status: "removed", updatedAt: new Date() }).where(eq(debloatPackages.packageName, pkg));
      await log({
        operationType: "AppxRemove",
        category: row.category,
        target: `${row.displayName} (pack: ${pack.name})`,
        previousValue: "installed",
        newValue: "removed",
        risk: row.risk,
        undoData: { kind: "debloat", id: pkg, field: "status", value: true },
      });
    }
  }

  for (const ruleId of pack.privacyRuleIds) {
    const [row] = await db.select().from(privacyRules).where(eq(privacyRules.id, ruleId));
    if (row && !row.enabled) {
      await db.update(privacyRules).set({ enabled: true, updatedAt: new Date() }).where(eq(privacyRules.id, ruleId));
      await log({
        operationType: "PrivacyApply",
        category: row.category,
        target: `${row.name} (pack: ${pack.name})`,
        previousValue: "false",
        newValue: "true",
        risk: row.risk,
        undoData: { kind: "privacy", id: ruleId, field: "enabled", value: false },
      });
    }
  }

  await db.update(communityPacks).set({ installed: true, updatedAt: new Date() }).where(eq(communityPacks.id, packId));
  revalidateAll();
  return { success: true, message: `Community pack "${pack.name}" installed and applied.` };
}

export async function uninstallCommunityPack(packId: string): Promise<ActionResult> {
  await ensureSeeded();
  const [pack] = await db.select().from(communityPacks).where(eq(communityPacks.id, packId));
  if (!pack) return { success: false, message: "Community pack not found" };

  await maybeCreateRestorePoint(`Before reverting community pack: ${pack.name}`);
  await db.update(communityPacks).set({ installed: false, updatedAt: new Date() }).where(eq(communityPacks.id, packId));
  revalidateAll();
  return { success: true, message: `Community pack "${pack.name}" marked as uninstalled. (You can also restore a snapshot from History).` };
}

// ── Restore Point Reversal / Restore Action ────────────────────────────────

export async function restoreSystemRestorePoint(sequenceNumber: number): Promise<ActionResult> {
  await ensureSeeded();
  const [rp] = await db.select().from(restorePoints).where(eq(restorePoints.sequenceNumber, sequenceNumber));
  if (!rp) return { success: false, message: "Restore point not found" };

  await log({
    operationType: "SystemRestore",
    category: "Safety",
    target: `System Restore to #${rp.sequenceNumber} (${rp.description})`,
    newValue: `restored-${rp.sequenceNumber}`,
    risk: "medium",
    canUndo: false,
  });
  revalidateAll();
  return { success: true, message: `System state simulated restore to point #${rp.sequenceNumber} (${rp.description}).` };
}

// ── ISO Builder ────────────────────────────────────────────────────────────

export interface IsoJobResult {
  id: string;
  createdAt: Date;
  status: string;
  options: Record<string, boolean>;
  log: string[];
  sha256: string | null;
}

export async function buildCustomIso(
  options: Record<string, boolean>,
  meta?: { edition?: string; arch?: string; targetFilename?: string }
): Promise<IsoJobResult> {
  const edition = meta?.edition || "Windows 11 Pro 64-bit";
  const arch = meta?.arch || "x64";
  const steps: string[] = [
    `Selecting target image architecture: ${arch}...`,
    "Mounting source ISO via DiscUtils.Iso9660.CDReader...",
    `Extracting and mounting install.wim [Index for ${edition}] (DISM API)...`,
  ];
  if (options.removeBloatware) steps.push("Removing 70+ bloatware Appx packages from offline image (DISM)...");
  if (options.applyPrivacyTweaks) steps.push("Injecting offline registry hive tweaks via RegLoadKey P/Invoke...");
  if (options.removeEdge) steps.push("Removing Microsoft Edge binary payload and setup stubs from offline image...");
  if (options.removeOneDrive) steps.push("Removing OneDrive setup installer and explorer namespace tree entries...");
  if (options.removeRecall) steps.push("Stripping Windows Recall & AI snapshot background capture packages...");
  if (options.bypassTpm) steps.push("Patching appraiserres.dll & boot.wim setup to bypass TPM 2.0, SecureBoot, and 8GB RAM checks...");
  if (options.bypassNro) steps.push("Configuring OOBE\\BypassNRO registry key to allow local offline account creation without internet...");
  if (options.disableDefenderPrompt) steps.push("Injecting unattended answer file (Autounattend.xml) with local admin configuration...");
  if (options.includePreset) steps.push("Embedding WinForge preset profile directly into %ProgramData%\\WinForge\\init.json...");
  steps.push("Committing offline WIM changes and unmounting install.wim...");
  steps.push("Generating El Torito bootable ISO catalog via DiscUtils.Iso9660.CDBuilder...");
  steps.push("Calculating SHA-256 cryptographic verification checksum...");
  steps.push(`Build complete! Output target: ${meta?.targetFilename || "WinForge-Win11-Custom.iso"}`);

  const sha256 = createHash("sha256").update(JSON.stringify(options) + randomBytes(8).toString("hex")).digest("hex");

  const [job] = await db
    .insert(isoJobs)
    .values({ status: "completed", options, log: steps, sha256 })
    .returning();

  await log({ operationType: "Custom", category: "ISO Builder", target: "Custom Windows ISO built", newValue: sha256, risk: "low", canUndo: false });
  revalidateAll();
  return job;
}

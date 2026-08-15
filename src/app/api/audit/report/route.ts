import { db } from "@/db";
import { ensureSeeded } from "@/db/seed";
import {
  debloatPackages,
  operationHistory,
  privacyRules,
  restorePoints,
  scheduledTasks,
  services,
  startupItems,
  tweaks,
  windowsUpdates,
} from "@/db/schema";
import { computeHealthReport } from "@/lib/health";
import { and, desc, eq, ne } from "drizzle-orm";

export const dynamic = "force-dynamic";

function table(headers: string[], rows: string[][]) {
  return `
  <table>
    <thead><tr>${headers.map((h) => `<th>${h}</th>`).join("")}</tr></thead>
    <tbody>${rows.map((r) => `<tr>${r.map((c) => `<td>${c}</td>`).join("")}</tr>`).join("")}</tbody>
  </table>`;
}

function statusBadge(ok: boolean, onText: string, offText: string) {
  return `<span style="color:${ok ? "#10b981" : "#ef4444"};font-weight:600">${ok ? onText : offText}</span>`;
}

export async function GET() {
  await ensureSeeded();
  const health = await computeHealthReport();

  const allTweaks = await db.select().from(tweaks).orderBy(tweaks.category, tweaks.name);
  const pkgs = await db.select().from(debloatPackages).where(ne(debloatPackages.category, "Protected")).orderBy(debloatPackages.category, debloatPackages.displayName);
  const privacy = await db.select().from(privacyRules).orderBy(privacyRules.category, privacyRules.name);
  const allServices = await db.select().from(services).orderBy(services.category, services.displayName);
  const tasks = await db.select().from(scheduledTasks).orderBy(scheduledTasks.category, scheduledTasks.name);
  const startup = await db.select().from(startupItems);
  const updates = await db
    .select()
    .from(windowsUpdates)
    .where(and(eq(windowsUpdates.installed, false), eq(windowsUpdates.hidden, false)));
  const restorePointRows = await db.select().from(restorePoints).orderBy(desc(restorePoints.sequenceNumber)).limit(10);
  const history = await db.select().from(operationHistory).orderBy(desc(operationHistory.timestamp)).limit(50);

  const scoreColor = health.score >= 80 ? "#4CAF50" : health.score >= 60 ? "#FFC107" : health.score >= 40 ? "#FF9800" : "#F44336";

  const html = `<!doctype html>
<html><head><meta charset="utf-8"><title>WinForge Elite — Full System Audit Report</title>
<style>
  body { font-family: Segoe UI, Arial, sans-serif; background:#0b0f17; color:#e2e8f0; padding:32px; max-width:1100px; margin:0 auto; }
  h1 { color:#38bdf8; } h2 { margin-top:36px; border-bottom:1px solid #334155; padding-bottom:6px; color:#f1f5f9; }
  table { width:100%; border-collapse:collapse; margin-top:12px; font-size:13px; }
  th, td { text-align:left; padding:7px 10px; border-bottom:1px solid #1e293b; }
  th { color:#94a3b8; text-transform:uppercase; font-size:11px; }
  .score { font-size:52px; font-weight:bold; color:${scoreColor}; }
  .grid { display:grid; grid-template-columns:repeat(auto-fit,minmax(160px,1fr)); gap:12px; margin:16px 0; }
  .card { border:1px solid #1e293b; border-radius:10px; padding:14px; }
  .card b { display:block; font-size:24px; } .card span { font-size:11px; color:#94a3b8; text-transform:uppercase; }
  .muted { color:#94a3b8; }
</style></head><body>
<h1>WinForge Elite — Full System Audit Report</h1>
<p class="muted">Generated ${new Date().toISOString()}</p>

<div class="score">${health.score}/100</div>
<p>${health.quickWins.map((w) => `• ${w}`).join("<br>")}</p>

<div class="grid">
  <div class="card"><b>${health.bloatwareCount}</b><span>Bloatware</span></div>
  <div class="card"><b>${health.privacyScore}/100</b><span>Privacy Score</span></div>
  <div class="card"><b>${health.appliedTweaksCount}/${health.totalTweaksCount}</b><span>Tweaks Applied</span></div>
  <div class="card"><b>${health.pendingSecurityUpdates}</b><span>Security Updates Pending</span></div>
  <div class="card"><b>${startup.filter((s) => s.enabled).length}</b><span>Startup Items</span></div>
  <div class="card"><b>${restorePointRows.length}</b><span>Restore Points</span></div>
</div>

<h2>Tweaks (${allTweaks.length})</h2>
${table(
  ["Name", "Category", "Risk", "State", "Operations"],
  allTweaks.map((t) => [
    t.name,
    t.category,
    t.risk,
    statusBadge(t.applied, "Applied", "Not applied"),
    `${t.operations.length} op(s)`,
  ])
)}

<h2>Bloatware Packages (${pkgs.length})</h2>
${table(
  ["Package", "Category", "Risk", "Status"],
  pkgs.map((p) => [p.displayName, p.category, p.risk, statusBadge(p.status === "installed", "Installed", "Removed")])
)}

<h2>Privacy Rules (${privacy.length})</h2>
${table(
  ["Rule", "Category", "Risk", "Status"],
  privacy.map((r) => [r.name, r.category, r.risk, statusBadge(r.enabled, "Hardened", "Not applied")])
)}

<h2>Services (${allServices.length})</h2>
${table(
  ["Service", "Category", "Start Type", "Status", "Protected"],
  allServices.map((s) => [
    s.displayName,
    s.category,
    s.startType,
    s.status,
    s.protected ? "🔒 Yes" : "No",
  ])
)}

<h2>Scheduled Tasks (${tasks.length})</h2>
${table(
  ["Task", "Path", "Category", "Risk", "Enabled"],
  tasks.map((t) => [t.name, t.path, t.category, t.risk, statusBadge(t.enabled, "Enabled", "Disabled")])
)}

<h2>Pending Updates (${updates.length})</h2>
${table(
  ["Title", "KB", "Severity", "Size"],
  updates.map((u) => [u.title, u.kb, u.severity, `${u.sizeMb} MB`])
)}

<h2>Restore Points</h2>
${restorePointRows.length === 0 ? '<p class="muted">None yet</p>' : table(["#", "Description", "Created"], restorePointRows.map((rp) => [String(rp.sequenceNumber), rp.description, rp.createdAt.toISOString()]))}

<h2>Recent Operations (${history.length})</h2>
${history.length === 0 ? '<p class="muted">No operations logged yet</p>' : table(
  ["Time", "Type", "Target", "Result"],
  history.map((h) => [h.timestamp.toISOString(), h.operationType, h.target, statusBadge(h.success, "Success", "Failed")])
)}

<p class="muted" style="margin-top:32px">WinForge Elite v2.0 — Simulation control center. All changes are reversible via History → Undo or snapshots.</p>
</body></html>`;

  return new Response(html, { headers: { "Content-Type": "text/html; charset=utf-8" } });
}

import { db } from "@/db";
import { ensureSeeded } from "@/db/seed";
import { computeHealthReport } from "@/lib/health";
import { recordHealthSnapshot } from "@/lib/actions";
import { debloatPackages, presets, tweaks, windowsUpdates } from "@/db/schema";
import { and, eq, ne } from "drizzle-orm";
import { MetricsPanel } from "@/components/MetricsPanel";
import { SystemInfo } from "@/components/SystemInfo";
import { QuickScan } from "@/components/QuickScan";
import { HealthPanel } from "@/components/HealthPanel";
import { Banner } from "@/components/ui";
import { PresetButtons } from "./PresetButtons";
import { PageHeader } from "@/components/PageHeader";

export const dynamic = "force-dynamic";

export default async function DashboardPage() {
  await ensureSeeded();
  const health = await computeHealthReport();
  await recordHealthSnapshot(); // throttled to 1/hour
  const allPresets = await db.select().from(presets);
  const bloatCount = (
    await db
      .select()
      .from(debloatPackages)
      .where(and(eq(debloatPackages.status, "installed"), ne(debloatPackages.category, "Protected")))
  ).length;
  const allTweaks = await db.select().from(tweaks);
  const appliedTweaks = allTweaks.filter((t) => t.applied).length;
  const totalTweaks = allTweaks.length;
  const pendingUpdates = await db
    .select()
    .from(windowsUpdates)
    .where(and(eq(windowsUpdates.installed, false), eq(windowsUpdates.hidden, false)));

  return (
    <div className="mx-auto max-w-7xl px-4 py-6 sm:px-8 sm:py-8">
      <PageHeader
        title="Dashboard"
        subtitle="Live overview of system health, bloatware, privacy posture, and pending updates."
      />

      <div className="mt-6">
        <HealthPanel
          initialScore={health.score}
          initialPrivacy={health.privacyScore}
          initialBloat={bloatCount}
          initialApplied={appliedTweaks}
          initialTotal={totalTweaks}
          initialPending={pendingUpdates.length}
        />
      </div>

      <div className="mt-6 space-y-2">
        {health.warnings.length === 0 ? (
          <Banner>✅ No critical alerts — your system looks healthy.</Banner>
        ) : (
          health.warnings.map((w, i) => (
            <Banner key={i} tone="warn">
              ⚠️ {w}
            </Banner>
          ))
        )}
      </div>

      <div className="mt-6">
        <MetricsPanel />
      </div>

      <div className="mt-6">
        <h3 className="mb-3 text-sm font-semibold text-white">System Information</h3>
        <SystemInfo />
      </div>

      <div className="mt-6">
        <QuickScan />
      </div>

      <div className="mt-6 grid grid-cols-1 gap-6 lg:grid-cols-2">
        <div className="rounded-2xl border border-white/10 bg-white/[0.03] p-6">
          <h3 className="mb-3 text-sm font-semibold text-white">Quick Wins</h3>
          <ul className="space-y-2">
            {health.quickWins.map((qw, i) => (
              <li key={i} className="flex items-start gap-2 text-sm text-slate-300">
                <span className="mt-0.5 text-sky-400">→</span>
                {qw}
              </li>
            ))}
          </ul>
        </div>

        <div className="rounded-2xl border border-white/10 bg-white/[0.03] p-6">
          <h3 className="mb-3 text-sm font-semibold text-white">One-Click Presets</h3>
          <p className="mb-4 text-xs text-slate-500">
            Applies a curated bundle of tweaks, debloat targets, and privacy rules. A restore point
            is created automatically before changes are made.
          </p>
          <PresetButtons presets={allPresets.map((p) => ({ id: p.id, name: p.name, description: p.description }))} />
        </div>
      </div>
    </div>
  );
}

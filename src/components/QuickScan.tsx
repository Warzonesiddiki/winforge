"use client";

import { useState, useTransition } from "react";
import Link from "next/link";
import { useToast } from "@/components/Toast";

interface ScanResult {
  category: string;
  status: "good" | "warning" | "critical";
  message: string;
  action?: string;
  href?: string;
}

interface StatusPayload {
  health: {
    score: number;
    privacyScore: number;
    quickWins: string[];
  };
  statistics: {
    tweaks: { applied: number; total: number };
    debloat: { installed: number; removed: number };
    privacy: { enabled: number; total: number; score: number };
    updates: { pending: number; pendingCritical: number };
  };
  restorePoints: { sequenceNumber: number }[];
}

export function QuickScan() {
  const [scanning, setScanning] = useState(false);
  const [results, setResults] = useState<ScanResult[] | null>(null);
  const [pending, startTransition] = useTransition();
  const { addToast } = useToast();

  async function runScan() {
    setScanning(true);
    setResults(null);

    let status: StatusPayload | null = null;
    try {
      const res = await fetch("/api/status", { cache: "no-store" });
      status = await res.json();
    } catch {
      // fall back to unknown state
    }

    const scanResults: ScanResult[] = [];

    await delay(250);
    const pendingUpdates = status?.statistics.updates.pending ?? 0;
    const pendingCritical = status?.statistics.updates.pendingCritical ?? 0;
    scanResults.push({
      category: "Updates",
      status: pendingCritical > 0 ? "critical" : pendingUpdates > 0 ? "warning" : "good",
      message:
        pendingUpdates === 0
          ? "System is up to date"
          : `${pendingUpdates} update(s) pending${pendingCritical > 0 ? ` (${pendingCritical} critical)` : ""}`,
      action: pendingUpdates > 0 ? "View Updates" : undefined,
      href: pendingUpdates > 0 ? "/updates" : undefined,
    });

    await delay(200);
    const bloat = status?.statistics.debloat.installed ?? 0;
    const bloatRemoved = status?.statistics.debloat.removed ?? 0;
    scanResults.push({
      category: "Bloatware",
      status: bloat > 30 ? "warning" : "good",
      message:
        bloat === 0
          ? "No bloatware detected — clean!"
          : `${bloat} removable package(s) detected${bloatRemoved > 0 ? `, ${bloatRemoved} already removed` : ""}`,
      action: bloat > 0 ? "Review Bloat" : undefined,
      href: bloat > 0 ? "/debloat" : undefined,
    });

    await delay(200);
    const privacyScore = status?.health.privacyScore ?? 0;
    scanResults.push({
      category: "Privacy",
      status: privacyScore >= 80 ? "good" : privacyScore >= 40 ? "warning" : "critical",
      message: `Privacy score ${privacyScore}/100`,
      action: privacyScore < 80 ? "Harden Privacy" : undefined,
      href: privacyScore < 80 ? "/privacy" : undefined,
    });

    await delay(150);
    const tweaksApplied = status?.statistics.tweaks.applied ?? 0;
    const tweaksTotal = status?.statistics.tweaks.total ?? 0;
    scanResults.push({
      category: "Optimization",
      status: tweaksApplied >= Math.ceil(tweaksTotal * 0.4) ? "good" : "warning",
      message: `${tweaksApplied}/${tweaksTotal} tweaks applied`,
      action: tweaksApplied < tweaksTotal * 0.4 ? "View Tweaks" : undefined,
      href: tweaksApplied < tweaksTotal * 0.4 ? "/tweaks" : undefined,
    });

    await delay(150);
    const restorePoints = status?.restorePoints?.length ?? 0;
    scanResults.push({
      category: "Safety Net",
      status: restorePoints > 0 ? "good" : "warning",
      message:
        restorePoints > 0
          ? `${restorePoints} restore point(s) available`
          : "No restore points yet — create one in Settings",
      action: restorePoints === 0 ? "Open Settings" : undefined,
      href: restorePoints === 0 ? "/settings" : undefined,
    });

    await delay(150);
    const healthScore = status?.health.score ?? 0;
    scanResults.push({
      category: "Overall Health",
      status: healthScore >= 60 ? "good" : "warning",
      message: `System health score ${healthScore}/100`,
    });

    await delay(100);
    setResults(scanResults);
    setScanning(false);
    const passed = scanResults.filter((r) => r.status === "good").length;
    addToast(
      "info",
      "Quick Scan Complete",
      `${passed}/${scanResults.length} checks passed — health score ${healthScore}/100.`
    );
  }

  const statusIcon = {
    good: "✓",
    warning: "⚠",
    critical: "✕",
  };

  const statusColor = {
    good: "text-emerald-400",
    warning: "text-amber-400",
    critical: "text-red-400",
  };

  return (
    <div className="rounded-2xl border border-white/10 bg-white/[0.03] p-5">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-sm font-semibold text-white">Quick System Scan</h3>
          <p className="mt-0.5 text-xs text-slate-500">Live check of updates, bloatware, privacy, and safety state</p>
        </div>
        <button
          onClick={() => startTransition(() => runScan())}
          disabled={scanning || pending}
          className="rounded-lg bg-sky-500 px-4 py-2 text-sm font-medium text-white hover:bg-sky-400 disabled:opacity-50"
        >
          {scanning ? "Scanning..." : "Run Scan"}
        </button>
      </div>

      {scanning && (
        <div className="mt-4">
          <div className="h-2 overflow-hidden rounded-full bg-white/10">
            <div className="h-full w-1/3 animate-pulse rounded-full bg-sky-500" />
          </div>
          <p className="mt-2 text-xs text-slate-400">Analyzing system configuration...</p>
        </div>
      )}

      {results && (
        <div className="mt-4 space-y-2">
          {results.map((r, i) => (
            <div key={i} className="flex flex-wrap items-center justify-between gap-2 rounded-lg bg-black/20 px-3 py-2">
              <div className="flex items-center gap-2">
                <span className={statusColor[r.status]}>{statusIcon[r.status]}</span>
                <span className="text-sm text-slate-300">{r.category}</span>
              </div>
              <div className="flex items-center gap-3">
                <span className="text-xs text-slate-500">{r.message}</span>
                {r.href && r.action && (
                  <Link
                    href={r.href}
                    className="rounded bg-sky-500/15 px-2 py-0.5 text-[11px] font-medium text-sky-300 hover:bg-sky-500/25"
                  >
                    {r.action} →
                  </Link>
                )}
              </div>
            </div>
          ))}
          <div className="mt-3 flex items-center justify-between rounded-lg border border-sky-500/20 bg-sky-500/5 px-3 py-2">
            <span className="text-sm font-medium text-sky-300">
              {results.filter((r) => r.status === "good").length}/{results.length} checks passed
            </span>
            <span className="text-xs text-slate-400">
              {results.filter((r) => r.status !== "good").length} items need attention
            </span>
          </div>
        </div>
      )}
    </div>
  );
}

function delay(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

"use client";

import { useEffect, useState } from "react";
import { HealthGauge } from "@/components/HealthGauge";

interface HealthPoint {
  timestamp: string;
  score: number;
  privacyScore: number;
  bloatCount: number;
  appliedTweaks: number;
  pendingUpdates: number;
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
    updates: { pending: number; pendingCritical: number };
  };
}

function TrendChart({ points }: { points: HealthPoint[] }) {
  if (points.length < 2) {
    return <p className="py-6 text-center text-xs text-slate-600">Collecting trend data…</p>;
  }
  const width = 320;
  const height = 64;
  const scores = points.map((p) => p.score);
  const min = Math.min(...scores, 0);
  const max = Math.max(...scores, 100);
  const range = max - min || 1;

  const coords = points.map((p, i) => {
    const x = (i / (points.length - 1)) * width;
    const y = height - ((p.score - min) / range) * height;
    return `${x.toFixed(1)},${y.toFixed(1)}`;
  });

  const last = points[points.length - 1];
  const first = points[0];
  const delta = last.score - first.score;

  return (
    <div>
      <svg viewBox={`0 0 ${width} ${height}`} className="h-16 w-full" preserveAspectRatio="none">
        <polyline points={coords.join(" ")} fill="none" stroke="#38bdf8" strokeWidth={2} />
        <circle
          cx={((points.length - 1) / (points.length - 1)) * width}
          cy={height - ((last.score - min) / range) * height}
          r={3.5}
          fill="#38bdf8"
        />
      </svg>
      <div className="mt-1 flex items-center justify-between text-[11px] text-slate-500">
        <span>{first.score} → {last.score}</span>
        <span className={delta >= 0 ? "text-emerald-400" : "text-red-400"}>
          {delta >= 0 ? "+" : ""}{delta} this period
        </span>
        <span>{new Date(points[0].timestamp).toLocaleDateString()} → {new Date(last.timestamp).toLocaleDateString()}</span>
      </div>
    </div>
  );
}

export function HealthPanel({
  initialScore,
  initialPrivacy,
  initialBloat,
  initialApplied,
  initialTotal,
  initialPending,
}: {
  initialScore: number;
  initialPrivacy: number;
  initialBloat: number;
  initialApplied: number;
  initialTotal: number;
  initialPending: number;
}) {
  const [status, setStatus] = useState<StatusPayload | null>(null);
  const [history, setHistory] = useState<HealthPoint[]>([]);

  useEffect(() => {
    let mounted = true;
    async function refresh() {
      try {
        const res = await fetch("/api/status", { cache: "no-store" });
        if (!mounted) return;
        const data: StatusPayload = await res.json();
        setStatus(data);
      } catch {
        // keep initial values
      }
    }
    async function loadHistory() {
      try {
        const res = await fetch("/api/health/history", { cache: "no-store" });
        if (!mounted) return;
        setHistory(await res.json());
      } catch {
        // ignore
      }
    }
    refresh();
    loadHistory();
    const id = setInterval(refresh, 30000);
    return () => {
      mounted = false;
      clearInterval(id);
    };
  }, []);

  const score = status?.health.score ?? initialScore;
  const privacy = status?.health.privacyScore ?? initialPrivacy;
  const bloat = status?.statistics.debloat.installed ?? initialBloat;
  const applied = status?.statistics.tweaks.applied ?? initialApplied;
  const total = status?.statistics.tweaks.total ?? initialTotal;
  const pending = status?.statistics.updates.pending ?? initialPending;

  return (
    <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
      <div className="rounded-2xl border border-white/10 bg-white/[0.03] p-6">
        <HealthGauge score={score} size={180} />
        <div className="mt-4 grid grid-cols-2 gap-2 text-center text-xs text-slate-400">
          <div>
            <p className="font-semibold text-white">{privacy}</p>
            <p>Privacy Score</p>
          </div>
          <div>
            <p className="font-semibold text-white">{bloat}</p>
            <p>Bloat Detected</p>
          </div>
        </div>
        <p className="mt-3 text-center text-[11px] text-slate-600">Auto-refreshes every 30s</p>
      </div>

      <div className="lg:col-span-2">
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
          <Stat value={`${applied}/${total}`} label="Tweaks Applied" />
          <Stat value={String(bloat)} label="Bloatware" tone={bloat > 10 ? "amber" : "green"} />
          <Stat value={String(pending)} label="Pending Updates" tone={pending > 0 ? "amber" : "green"} />
          <Stat value={`${privacy}/100`} label="Privacy Score" tone={privacy < 60 ? "red" : "green"} />
        </div>

        <div className="mt-4 rounded-2xl border border-white/10 bg-white/[0.03] p-4">
          <h4 className="mb-2 text-xs font-semibold uppercase tracking-wide text-slate-400">Health Score Trend</h4>
          <TrendChart points={history} />
        </div>
      </div>
    </div>
  );
}

function Stat({ value, label, tone = "neutral" }: { value: string; label: string; tone?: "neutral" | "green" | "amber" | "red" }) {
  const toneClass: Record<string, string> = {
    neutral: "text-white",
    green: "text-emerald-400",
    amber: "text-amber-400",
    red: "text-red-400",
  };
  return (
    <div className="rounded-2xl border border-white/10 bg-white/[0.03] p-5">
      <p className="text-xs uppercase tracking-wide text-slate-400">{label}</p>
      <p className={`mt-2 text-2xl font-semibold ${toneClass[tone]}`}>{value}</p>
    </div>
  );
}

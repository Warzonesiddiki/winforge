"use client";

import { useEffect, useState } from "react";

// Shapes returned by the Go engine's HTTP API (internal/httpapi/server.go):
// GET /api/status and GET /api/health, reached through the /engine/* rewrite
// (docs/ADR-001-ui-engine-bridge.md). When no engine is running the fetch
// fails and the card renders the offline state — the simulation is unaffected.

interface EngineStatus {
  version: string;
  elevated: boolean;
  tweakCount: number;
  pluginCount: number;
  bloatwareCount: number;
  os?: { caption?: string; version?: string };
}

interface EngineHealth {
  score: number;
  totalTweaks: number;
  appliedTweaks: number;
  unappliedLow: number;
  unappliedMedium: number;
  unappliedHigh: number;
  unverifiableTweaks?: number;
  bloatwareCount: number;
}

type EngineState =
  | { kind: "checking" }
  | { kind: "offline" }
  | { kind: "online"; status: EngineStatus; health: EngineHealth };

export function EngineStatusCard() {
  const [state, setState] = useState<EngineState>({ kind: "checking" });

  useEffect(() => {
    let cancelled = false;
    const probe = async () => {
      let next: EngineState;
      try {
        const [statusRes, healthRes] = await Promise.all([
          fetch("/engine/api/status", { cache: "no-store" }),
          fetch("/engine/api/health", { cache: "no-store" }),
        ]);
        if (!statusRes.ok || !healthRes.ok) throw new Error("engine unreachable");
        const status = (await statusRes.json()) as EngineStatus;
        const health = (await healthRes.json()) as EngineHealth;
        next = { kind: "online", status, health };
      } catch {
        next = { kind: "offline" };
      }
      if (!cancelled) setState(next);
    };
    probe();
    const timer = setInterval(probe, 15000);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, []);

  if (state.kind === "checking") {
    return <div className="h-20 animate-pulse rounded-2xl bg-white/5" />;
  }

  if (state.kind === "offline") {
    return (
      <div className="rounded-2xl border border-white/10 bg-white/[0.03] p-4">
        <div className="flex items-center gap-2">
          <span className="h-2 w-2 rounded-full bg-slate-500" />
          <h3 className="text-sm font-semibold text-white">Native engine</h3>
          <span className="text-xs text-slate-500">offline — simulation mode</span>
        </div>
        <p className="mt-2 text-xs text-slate-400">
          Start the WinForge engine with <code className="rounded bg-white/10 px-1">winforge serve</code> on
          this machine to see live system state here.
        </p>
      </div>
    );
  }

  const { status, health } = state;
  return (
    <div className="rounded-2xl border border-emerald-400/20 bg-emerald-400/[0.04] p-4">
      <div className="flex flex-wrap items-center gap-2">
        <span className="h-2 w-2 rounded-full bg-emerald-400" />
        <h3 className="text-sm font-semibold text-white">Native engine connected</h3>
        <span className="text-xs text-slate-400">
          v{status.version}
          {status.elevated ? " · elevated" : " · standard user"}
        </span>
      </div>
      <dl className="mt-3 grid grid-cols-2 gap-3 text-xs sm:grid-cols-4">
        <div>
          <dt className="text-slate-500">Health score</dt>
          <dd className="text-lg font-semibold text-emerald-300">{health.score}</dd>
        </div>
        <div>
          <dt className="text-slate-500">Tweaks applied</dt>
          <dd className="text-lg font-semibold text-white">
            {health.appliedTweaks}
            <span className="text-xs font-normal text-slate-500"> / {status.tweakCount}</span>
          </dd>
        </div>
        <div>
          <dt className="text-slate-500">Bloatware detected</dt>
          <dd className="text-lg font-semibold text-white">{health.bloatwareCount}</dd>
        </div>
        <div>
          <dt className="text-slate-500">Plugins</dt>
          <dd className="text-lg font-semibold text-white">{status.pluginCount}</dd>
        </div>
      </dl>
    </div>
  );
}

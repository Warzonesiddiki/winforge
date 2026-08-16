"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { EngineError, engineClient } from "@/lib/engine-client";

// Live tweak control for the native engine (ADR-002 phase 2). Every mutation
// goes through engineClient, which attaches the per-instance session token
// (X-WinForge-Token) required by internal/httpapi/server.go. Reads need no
// token. When no engine is running this renders nothing — the Next.js
// simulation on the rest of the dashboard is unaffected.

interface EngineTweak {
  id: string;
  name: string;
  category: string;
  description: string;
  risk: string;
  reversible: boolean;
  verifiable: boolean;
  applied: boolean;
}

// internal/tweak Result
interface TweakResult {
  tweakId: string;
  dryRun: boolean;
  succeeded: number;
  failed: number;
  changed: number;
  warnings?: string[];
}

type Load =
  | { kind: "loading" }
  | { kind: "offline" }
  | { kind: "ready"; tweaks: EngineTweak[] };

const RISK_TONE: Record<string, string> = {
  low: "border-emerald-400/30 bg-emerald-400/10 text-emerald-300",
  medium: "border-amber-400/30 bg-amber-400/10 text-amber-300",
  high: "border-rose-400/30 bg-rose-400/10 text-rose-300",
};

export function EngineTweaks() {
  const [load, setLoad] = useState<Load>({ kind: "loading" });
  const [busy, setBusy] = useState<string | null>(null);
  const [notice, setNotice] = useState<{ tone: "ok" | "err"; text: string } | null>(null);
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState("all");
  const [appliedOnly, setAppliedOnly] = useState(false);

  // Bumping this re-runs the loader effect. Keeping the fetch inside the
  // effect (rather than a callback that setStates directly) avoids the
  // cascading-render pattern the react-hooks lint rule guards against.
  const [reloadKey, setReloadKey] = useState(0);
  const refresh = useCallback(() => setReloadKey((k) => k + 1), []);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const tweaks = await engineClient.get<EngineTweak[]>("/api/tweaks");
        if (!cancelled) setLoad({ kind: "ready", tweaks });
      } catch {
        if (!cancelled) setLoad({ kind: "offline" });
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [reloadKey]);

  const categories = useMemo(() => {
    if (load.kind !== "ready") return [];
    return Array.from(new Set(load.tweaks.map((t) => t.category))).sort();
  }, [load]);

  const visible = useMemo(() => {
    if (load.kind !== "ready") return [];
    const q = query.trim().toLowerCase();
    return load.tweaks.filter((t) => {
      if (appliedOnly && !t.applied) return false;
      if (category !== "all" && t.category !== category) return false;
      if (!q) return true;
      return (
        t.name.toLowerCase().includes(q) ||
        t.id.toLowerCase().includes(q) ||
        t.description.toLowerCase().includes(q)
      );
    });
  }, [load, query, category, appliedOnly]);

  async function mutate(tweak: EngineTweak, action: "apply" | "undo") {
    setBusy(tweak.id);
    setNotice(null);
    try {
      const res = await engineClient.post<TweakResult>(`/api/tweaks/${action}`, { id: tweak.id });
      const verb = action === "apply" ? "Applied" : "Reverted";
      const warn = res.warnings?.length ? ` (${res.warnings.length} warning(s))` : "";
      setNotice({
        tone: "ok",
        text: `${verb} “${tweak.name}” — ${res.changed} change(s)${warn}.`,
      });
      refresh();
    } catch (err) {
      // A 401 means the engine rotated its token (restart); engineClient has
      // already dropped its cache, so an immediate retry would succeed.
      const msg =
        err instanceof EngineError
          ? err.status === 401
            ? "Engine session expired (it was restarted). Try again."
            : err.message
          : "Engine unreachable.";
      setNotice({ tone: "err", text: `Could not ${action} “${tweak.name}”: ${msg}` });
      if (err instanceof EngineError && err.status !== 401) refresh();
    } finally {
      setBusy(null);
    }
  }

  if (load.kind === "loading" || load.kind === "offline") {
    // Offline is already communicated by EngineStatusCard; stay quiet here.
    return null;
  }

  const appliedCount = load.tweaks.filter((t) => t.applied).length;

  return (
    <div className="rounded-2xl border border-white/10 bg-white/[0.03] p-4 sm:p-6">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h3 className="text-sm font-semibold text-white">
          Native engine tweaks
          <span className="ml-2 text-xs font-normal text-slate-500">
            {appliedCount} of {load.tweaks.length} applied
          </span>
        </h3>
        <button
          type="button"
          onClick={refresh}
          className="rounded-lg border border-white/10 bg-white/5 px-3 py-1 text-xs text-slate-300 transition hover:bg-white/10"
        >
          Refresh
        </button>
      </div>
      <p className="mt-1 text-xs text-slate-500">
        These act on the <strong className="font-medium text-slate-400">real machine</strong> via
        the local engine, not the simulation. Each change is audited and can be undone.
      </p>

      <div className="mt-4 flex flex-wrap gap-2">
        <input
          type="search"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Search tweaks…"
          className="min-w-0 flex-1 rounded-lg border border-white/10 bg-slate-950/60 px-3 py-1.5 text-sm text-slate-200 placeholder:text-slate-600 focus:border-sky-400/40 focus:outline-none"
        />
        <select
          value={category}
          onChange={(e) => setCategory(e.target.value)}
          className="rounded-lg border border-white/10 bg-slate-950/60 px-3 py-1.5 text-sm text-slate-200 focus:border-sky-400/40 focus:outline-none"
        >
          <option value="all">All categories</option>
          {categories.map((c) => (
            <option key={c} value={c}>
              {c}
            </option>
          ))}
        </select>
        <label className="flex items-center gap-2 rounded-lg border border-white/10 bg-slate-950/60 px-3 py-1.5 text-sm text-slate-300">
          <input
            type="checkbox"
            checked={appliedOnly}
            onChange={(e) => setAppliedOnly(e.target.checked)}
            className="accent-sky-400"
          />
          Applied only
        </label>
      </div>

      {notice && (
        <p
          role="status"
          className={`mt-3 rounded-lg border px-3 py-2 text-xs ${
            notice.tone === "ok"
              ? "border-emerald-400/30 bg-emerald-400/10 text-emerald-200"
              : "border-rose-400/30 bg-rose-400/10 text-rose-200"
          }`}
        >
          {notice.text}
        </p>
      )}

      <ul className="mt-4 max-h-96 space-y-2 overflow-y-auto pr-1">
        {visible.length === 0 && (
          <li className="py-6 text-center text-xs text-slate-500">No tweaks match those filters.</li>
        )}
        {visible.map((t) => (
          <li
            key={t.id}
            className="flex flex-wrap items-center gap-3 rounded-xl border border-white/5 bg-white/[0.02] px-3 py-2"
          >
            <div className="min-w-0 flex-1">
              <div className="flex flex-wrap items-center gap-2">
                <span className="truncate text-sm text-slate-200">{t.name}</span>
                <span
                  className={`rounded-full border px-1.5 py-px text-[10px] uppercase ${
                    RISK_TONE[t.risk] ?? "border-white/10 bg-white/5 text-slate-400"
                  }`}
                >
                  {t.risk}
                </span>
                {t.applied && (
                  <span className="rounded-full border border-sky-400/30 bg-sky-400/10 px-1.5 py-px text-[10px] uppercase text-sky-300">
                    applied
                  </span>
                )}
                {!t.verifiable && (
                  <span
                    title="One-way operation: the engine cannot verify this tweak's state."
                    className="rounded-full border border-white/10 bg-white/5 px-1.5 py-px text-[10px] uppercase text-slate-500"
                  >
                    unverifiable
                  </span>
                )}
              </div>
              <p className="truncate text-xs text-slate-500">{t.description || t.id}</p>
            </div>
            {t.applied ? (
              <button
                type="button"
                disabled={busy !== null || !t.reversible}
                title={t.reversible ? undefined : "This tweak is not reversible."}
                onClick={() => void mutate(t, "undo")}
                className="rounded-lg border border-white/10 bg-white/5 px-3 py-1.5 text-xs font-medium text-slate-200 transition hover:bg-white/10 disabled:opacity-40"
              >
                {busy === t.id ? "Working…" : "Undo"}
              </button>
            ) : (
              <button
                type="button"
                disabled={busy !== null}
                onClick={() => void mutate(t, "apply")}
                className="rounded-lg border border-sky-400/30 bg-sky-400/10 px-3 py-1.5 text-xs font-medium text-sky-200 transition hover:bg-sky-400/20 disabled:opacity-40"
              >
                {busy === t.id ? "Working…" : "Apply"}
              </button>
            )}
          </li>
        ))}
      </ul>
    </div>
  );
}

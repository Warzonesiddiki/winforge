"use client";

import { useEffect, useRef, useState } from "react";

interface Metrics {
  timestamp: number;
  cpuPercent: number;
  memPercent: number;
  diskPercent: number;
  netKbps: number;
  totalMemGb: number;
  freeMemGb: number;
  diskTotalGb: number;
  diskUsedGb: number;
  cpuModel: string;
  cpuCores: number;
  platform: string;
  hostname: string;
  uptimeHours: number;
}

const SERIES_KEYS = ["cpuPercent", "memPercent", "diskPercent", "netKbps"] as const;

function Sparkline({ data, max, color }: { data: number[]; max: number; color: string }) {
  const width = 260;
  const height = 56;
  const points = data
    .map((v, i) => {
      const x = (i / Math.max(1, data.length - 1)) * width;
      const y = height - (Math.min(v, max) / max) * height;
      return `${x.toFixed(1)},${y.toFixed(1)}`;
    })
    .join(" ");
  return (
    <svg width="100%" viewBox={`0 0 ${width} ${height}`} preserveAspectRatio="none" className="h-14 w-full">
      <polyline points={points} fill="none" stroke={color} strokeWidth={2} />
    </svg>
  );
}

export function MetricsPanel() {
  const [history, setHistory] = useState<Record<(typeof SERIES_KEYS)[number], number[]>>({
    cpuPercent: [],
    memPercent: [],
    diskPercent: [],
    netKbps: [],
  });
  const [latest, setLatest] = useState<Metrics | null>(null);
  const mounted = useRef(true);

  useEffect(() => {
    mounted.current = true;
    async function poll() {
      try {
        const res = await fetch("/api/metrics", { cache: "no-store" });
        const data: Metrics = await res.json();
        if (!mounted.current) return;
        setLatest(data);
        setHistory((prev) => {
          const next = { ...prev };
          for (const key of SERIES_KEYS) {
            next[key] = [...prev[key], data[key]].slice(-60);
          }
          return next;
        });
      } catch {
        // ignore transient failures
      }
    }
    poll();
    const id = setInterval(poll, 2000);
    return () => {
      mounted.current = false;
      clearInterval(id);
    };
  }, []);

  const cards = [
    { key: "cpuPercent" as const, label: "CPU Load", color: "#38bdf8", max: 100, unit: "%" },
    { key: "memPercent" as const, label: "Memory", color: "#a78bfa", max: 100, unit: "%" },
    { key: "diskPercent" as const, label: "Disk", color: "#fb923c", max: 100, unit: "%" },
    { key: "netKbps" as const, label: "Network", color: "#4ade80", max: 200, unit: " KB/s" },
  ];

  return (
    <div className="rounded-2xl border border-white/10 bg-white/[0.03] p-5">
      <div className="mb-4 flex items-center justify-between">
        <h3 className="text-sm font-semibold text-white">Live System Telemetry</h3>
        {latest && (
          <p className="text-xs text-slate-500">
            {latest.cpuModel} · {latest.cpuCores} cores · {latest.hostname}
          </p>
        )}
      </div>
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {cards.map((c) => (
          <div key={c.key} className="rounded-xl border border-white/5 bg-black/20 p-3">
            <div className="mb-1 flex items-baseline justify-between">
              <span className="text-xs uppercase tracking-wide text-slate-400">{c.label}</span>
              <span className="text-sm font-semibold text-white">
                {latest ? latest[c.key] : "--"}
                {c.unit}
              </span>
            </div>
            <Sparkline data={history[c.key]} max={c.max} color={c.color} />
          </div>
        ))}
      </div>
      {latest && (
        <p className="mt-3 text-[11px] text-slate-500">
          {latest.freeMemGb} GB free of {latest.totalMemGb} GB RAM · {latest.diskUsedGb} GB used of {latest.diskTotalGb} GB disk ·
          uptime {latest.uptimeHours}h
        </p>
      )}
    </div>
  );
}

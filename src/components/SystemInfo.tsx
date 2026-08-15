"use client";

import { useEffect, useState } from "react";

interface SysInfo {
  cpuModel: string;
  cpuCores: number;
  totalMemGb: number;
  freeMemGb: number;
  diskTotalGb: number;
  diskUsedGb: number;
  platform: string;
  hostname: string;
  uptimeHours: number;
}

export function SystemInfo() {
  const [info, setInfo] = useState<SysInfo | null>(null);

  useEffect(() => {
    fetch("/api/metrics")
      .then((r) => r.json())
      .then(setInfo)
      .catch(() => {});
  }, []);

  const osInfo = {
    edition: "Windows 11 Pro",
    version: "24H2",
    build: "26100.2605",
    experience: "Windows Feature Experience Pack 1000.26100.32.0",
    activation: "Windows is activated with a digital license",
  };

  if (!info) return <div className="h-32 animate-pulse rounded-xl bg-white/5" />;

  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
      <div className="rounded-xl border border-white/10 bg-white/[0.03] p-4">
        <h4 className="mb-3 text-xs font-semibold uppercase tracking-wide text-slate-500">Operating System</h4>
        <div className="space-y-2 text-sm">
          <Row label="Edition" value={osInfo.edition} />
          <Row label="Version" value={osInfo.version} />
          <Row label="OS Build" value={osInfo.build} />
          <Row label="Experience" value={osInfo.experience} small />
          <Row label="Activation" value={osInfo.activation} tone="green" />
        </div>
      </div>
      <div className="rounded-xl border border-white/10 bg-white/[0.03] p-4">
        <h4 className="mb-3 text-xs font-semibold uppercase tracking-wide text-slate-500">Hardware</h4>
        <div className="space-y-2 text-sm">
          <Row label="Processor" value={info.cpuModel} small />
          <Row label="CPU Cores" value={`${info.cpuCores} logical processors`} />
          <Row label="RAM" value={`${info.totalMemGb} GB total (${info.freeMemGb} GB free)`} />
          <Row label="System Disk" value={`${info.diskUsedGb} GB used of ${info.diskTotalGb} GB`} />
          <Row label="Uptime" value={`${info.uptimeHours} hours`} />
        </div>
      </div>
    </div>
  );
}

function Row({ label, value, tone, small }: { label: string; value: string; tone?: "green" | "amber" | "red"; small?: boolean }) {
  const toneClass = tone === "green" ? "text-emerald-400" : tone === "amber" ? "text-amber-400" : tone === "red" ? "text-red-400" : "text-white";
  return (
    <div className="flex justify-between gap-2">
      <span className="text-slate-400">{label}</span>
      <span className={`text-right ${toneClass} ${small ? "max-w-[180px] truncate text-xs" : ""}`} title={value}>{value}</span>
    </div>
  );
}

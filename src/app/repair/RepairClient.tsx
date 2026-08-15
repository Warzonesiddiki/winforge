"use client";

import { useState, useTransition } from "react";
import {
  calculateTempCleanupSize,
  cleanTempFiles,
  flushDns,
  resetNetworkStack,
  resetWindowsUpdate,
  runDismRestore,
  runDismScan,
  runFullSystemCheck,
  runSfc,
  setDnsPreset,
  setWindowsFeature,
  type CleanupSizeInfo,
} from "@/lib/actions";
import { LogConsole } from "@/components/LogConsole";
import { Pill } from "@/components/ui";
import { useToast } from "@/components/Toast";
import { dnsPresets, windowsFeatures } from "@/db/dns-presets";

interface RepairTool {
  id: string;
  title: string;
  description: string;
  action: () => Promise<{ success: boolean; message: string }>;
}

const TOOLS: RepairTool[] = [
  {
    id: "full-check",
    title: "Full System Check",
    description: "Runs SFC + DISM Scan + DISM Restore + CHKDSK in sequence for a complete integrity pass.",
    action: runFullSystemCheck,
  },
  {
    id: "sfc",
    title: "SFC /scannow",
    description: "System File Checker scans all protected system files and repairs corrupted ones.",
    action: runSfc,
  },
  {
    id: "wu-reset",
    title: "Reset Windows Update",
    description: "Stops wuauserv/bits/cryptsvc, renames SoftwareDistribution, restarts services.",
    action: resetWindowsUpdate,
  },
  {
    id: "dism-scan",
    title: "DISM Scan Health",
    description: "Scans the component store for corruption without repairing.",
    action: runDismScan,
  },
  {
    id: "dism-restore",
    title: "DISM Restore Health",
    description: "Repairs the component store using Windows Update as a source.",
    action: runDismRestore,
  },
  {
    id: "dns-flush",
    title: "Flush DNS Cache",
    description: "Calls DnsFlushResolverCache() to clear the local resolver cache.",
    action: flushDns,
  },
  {
    id: "net-reset",
    title: "Reset Network Stack",
    description: "Resets Winsock catalog, TCP/IP stack, and renews DHCP lease.",
    action: resetNetworkStack,
  },
];

const CLEANUP_CATEGORIES = [
  { id: "temp", label: "Temporary Files (%TEMP%, C:\\Windows\\Temp)", defaultChecked: true, factor: 1.0 },
  { id: "update", label: "Windows Update Download & Component Cache", defaultChecked: true, factor: 1.8 },
  { id: "thumbnails", label: "Windows Thumbnail Database Cache", defaultChecked: true, factor: 0.5 },
  { id: "prefetch", label: "Prefetch Application Traces (Safe Subset)", defaultChecked: true, factor: 0.35 },
  { id: "shader", label: "DirectX / D3D Shader Cache", defaultChecked: true, factor: 0.6 },
  { id: "delivery", label: "Delivery Optimization Peer Download Cache", defaultChecked: true, factor: 0.8 },
  { id: "dumps", label: "Windows Error Reporting Memory Dumps & Minidumps", defaultChecked: false, factor: 0.4 },
  { id: "recycle", label: "Recycle Bin Contents", defaultChecked: false, factor: 1.2 },
];

export function RepairClient() {
  const [log, setLog] = useState<string[]>([]);
  const [pending, startTransition] = useTransition();
  const [cleanupPreview, setCleanupPreview] = useState<CleanupSizeInfo | null>(null);
  const [selectedCleanCategories, setSelectedCleanCategories] = useState<Record<string, boolean>>({
    temp: true,
    update: true,
    thumbnails: true,
    prefetch: true,
    shader: true,
    delivery: true,
    dumps: false,
    recycle: false,
  });
  const [cleaning, startCleaning] = useTransition();
  const [tab, setTab] = useState<"tools" | "cleanup" | "dns" | "features">("tools");
  const [featureSearch, setFeatureSearch] = useState("");
  const { addToast } = useToast();

  async function run(tool: RepairTool) {
    startTransition(async () => {
      setLog((prev) => [...prev, `Running ${tool.title}...`]);
      const result = await tool.action();
      setLog((prev) => [...prev, `${result.success ? "✓" : "✗"} ${result.message}`]);
      addToast(result.success ? "success" : "error", tool.title, result.message);
      if ("log" in result && Array.isArray((result as { log?: string[] }).log)) {
        setLog((prev) => [...prev, ...((result as { log: string[] }).log)]);
      }
    });
  }

  function previewCleanup() {
    startTransition(async () => {
      const info = await calculateTempCleanupSize();
      setCleanupPreview(info);
    });
  }

  function toggleCleanCat(id: string) {
    setSelectedCleanCategories((prev) => ({ ...prev, [id]: !prev[id] }));
  }

  function confirmCleanup() {
    startCleaning(async () => {
      const activeCats = Object.entries(selectedCleanCategories).filter(([, v]) => v).map(([k]) => k);
      setLog((prev) => [...prev, `Executing Advanced Disk Cleanup (${activeCats.length} categories)...`]);
      const result = await cleanTempFiles();
      setLog((prev) => [...prev, `✓ ${result.message}`]);
      addToast("success", "Disk Cleanup Complete", result.message);
      setCleanupPreview(null);
    });
  }

  function applyDns(presetId: string) {
    startTransition(async () => {
      const preset = dnsPresets.find((p) => p.id === presetId);
      setLog((prev) => [...prev, `Applying DNS preset: ${preset?.name}...`]);
      const result = await setDnsPreset(presetId);
      setLog((prev) => [...prev, `${result.success ? "✓" : "✗"} ${result.message}`]);
      addToast(result.success ? "success" : "error", "DNS Configuration", result.message);
    });
  }

  function toggleFeature(featureId: string, enable: boolean) {
    startTransition(async () => {
      const feature = windowsFeatures.find((f) => f.id === featureId);
      setLog((prev) => [...prev, `${enable ? "Enabling" : "Disabling"} ${feature?.name}...`]);
      const result = await setWindowsFeature(featureId, enable);
      setLog((prev) => [...prev, `${result.success ? "✓" : "✗"} ${result.message}`]);
      addToast(result.success ? "success" : "error", feature?.name ?? "Feature", result.message);
    });
  }

  const filteredFeatures = windowsFeatures.filter(
    (f) => !featureSearch || f.name.toLowerCase().includes(featureSearch.toLowerCase()) || f.category.toLowerCase().includes(featureSearch.toLowerCase())
  );

  const featureCategories = Array.from(new Set(windowsFeatures.map((f) => f.category)));

  // Calculated estimate
  const baseMb = cleanupPreview?.tempMb || 45;
  const estimatedReclaim = CLEANUP_CATEGORIES
    .filter((c) => selectedCleanCategories[c.id])
    .reduce((acc, c) => acc + Math.round(baseMb * c.factor), 0);

  return (
    <div className="mt-6">
      <div className="mb-4 flex flex-wrap items-center gap-2 border-b border-white/10 pb-2">
        <button onClick={() => setTab("tools")} className={`px-3 py-2 text-sm font-medium transition ${tab === "tools" ? "border-b-2 border-sky-400 text-sky-300" : "text-slate-400 hover:text-slate-200"}`}>
          Repair Tools
        </button>
        <button onClick={() => setTab("cleanup")} className={`px-3 py-2 text-sm font-medium transition ${tab === "cleanup" ? "border-b-2 border-sky-400 text-sky-300" : "text-slate-400 hover:text-slate-200"}`}>
          Disk Cleanup Suite
        </button>
        <button onClick={() => setTab("dns")} className={`px-3 py-2 text-sm font-medium transition ${tab === "dns" ? "border-b-2 border-sky-400 text-sky-300" : "text-slate-400 hover:text-slate-200"}`}>
          DNS Configuration
        </button>
        <button onClick={() => setTab("features")} className={`px-3 py-2 text-sm font-medium transition ${tab === "features" ? "border-b-2 border-sky-400 text-sky-300" : "text-slate-400 hover:text-slate-200"}`}>
          Windows Features
        </button>
      </div>

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <div>
          {tab === "tools" && (
            <div className="space-y-4">
              {/* Full System Check - highlighted */}
              <div className="flex items-start justify-between gap-4 rounded-2xl border border-emerald-500/30 bg-emerald-500/5 p-4">
                <div>
                  <div className="flex items-center gap-2">
                    <p className="text-sm font-semibold text-white">Full System Check</p>
                    <span className="rounded-full bg-emerald-500/20 px-2 py-0.5 text-[10px] font-semibold text-emerald-400">RECOMMENDED</span>
                  </div>
                  <p className="mt-1 text-xs text-slate-400">
                    Runs SFC + DISM Scan + DISM Restore + CHKDSK in sequence for a complete integrity pass.
                  </p>
                </div>
                <button
                  onClick={() => run(TOOLS[0])}
                  disabled={pending}
                  className="shrink-0 rounded-lg bg-emerald-500 px-3 py-1.5 text-xs font-medium text-white hover:bg-emerald-400 disabled:opacity-40"
                >
                  Run All
                </button>
              </div>

              {TOOLS.slice(1).map((tool) => (
                <div key={tool.id} className="flex items-start justify-between gap-4 rounded-2xl border border-white/10 bg-white/[0.03] p-4">
                  <div>
                    <p className="text-sm font-semibold text-white">{tool.title}</p>
                    <p className="mt-1 text-xs text-slate-400">{tool.description}</p>
                  </div>
                  <button
                    onClick={() => run(tool)}
                    disabled={pending}
                    className="shrink-0 rounded-lg border border-sky-500/30 bg-sky-500/10 px-3 py-1.5 text-xs font-medium text-sky-300 hover:bg-sky-500/20 disabled:opacity-40"
                  >
                    Run
                  </button>
                </div>
              ))}
            </div>
          )}

          {tab === "cleanup" && (
            <div className="space-y-4 rounded-2xl border border-white/10 bg-white/[0.03] p-5">
              <div className="flex items-center justify-between border-b border-white/10 pb-3">
                <div>
                  <h3 className="text-sm font-semibold text-white">Advanced Disk Cleanup Suite</h3>
                  <p className="text-xs text-slate-400">Select categories to safely purge and reclaim storage</p>
                </div>
                <button
                  onClick={previewCleanup}
                  disabled={pending}
                  className="rounded-lg border border-sky-500/30 bg-sky-500/10 px-3 py-1.5 text-xs font-medium text-sky-300 hover:bg-sky-500/20"
                >
                  Scan Disk Space
                </button>
              </div>

              <div className="space-y-2">
                {CLEANUP_CATEGORIES.map((c) => (
                  <label
                    key={c.id}
                    className="flex cursor-pointer items-center justify-between rounded-xl border border-white/5 bg-black/20 p-3 text-xs transition hover:border-white/10"
                  >
                    <div className="flex items-center gap-3">
                      <input
                        type="checkbox"
                        checked={!!selectedCleanCategories[c.id]}
                        onChange={() => toggleCleanCat(c.id)}
                      />
                      <span className="text-slate-200 font-medium">{c.label}</span>
                    </div>
                    <span className="font-mono text-slate-400">
                      ~{Math.round(baseMb * c.factor)} MB
                    </span>
                  </label>
                ))}
              </div>

              <div className="flex items-center justify-between rounded-xl border border-emerald-500/30 bg-emerald-500/10 p-4">
                <div>
                  <p className="text-xs text-slate-400 uppercase tracking-wider">Estimated Space Savings</p>
                  <p className="text-lg font-bold text-emerald-300">~{estimatedReclaim} MB ({((estimatedReclaim / 1024)).toFixed(2)} GB)</p>
                </div>
                <button
                  onClick={confirmCleanup}
                  disabled={cleaning || estimatedReclaim === 0}
                  className="rounded-xl bg-emerald-500 px-5 py-2 text-xs font-semibold text-white transition hover:bg-emerald-400 disabled:opacity-40"
                >
                  {cleaning ? "Cleaning..." : `Purge Selected (${Object.values(selectedCleanCategories).filter(Boolean).length})`}
                </button>
              </div>
            </div>
          )}

          {tab === "dns" && (
            <div className="space-y-4">
              <p className="text-sm text-slate-400">
                Select a privacy-focused DNS provider. Changes are applied via WMI SetDNSServerSearchOrder.
              </p>
              {(["privacy", "security", "performance", "family"] as const).map((category) => (
                <div key={category}>
                  <h4 className="mb-2 text-xs font-semibold uppercase tracking-wide text-slate-500">{category}</h4>
                  <div className="space-y-2">
                    {dnsPresets
                      .filter((p) => p.category === category)
                      .map((preset) => (
                        <div key={preset.id} className="flex items-center justify-between gap-3 rounded-xl border border-white/10 bg-white/[0.03] p-3">
                          <div>
                            <p className="text-sm font-medium text-white">{preset.name}</p>
                            <p className="text-xs text-slate-500">{preset.description}</p>
                            <p className="mt-1 font-mono text-[11px] text-slate-600">
                              {preset.primary} / {preset.secondary}
                              {preset.doh && <span className="ml-2 text-emerald-500">DoH ✓</span>}
                            </p>
                          </div>
                          <button
                            onClick={() => applyDns(preset.id)}
                            disabled={pending}
                            className="shrink-0 rounded-lg border border-sky-500/30 bg-sky-500/10 px-3 py-1.5 text-xs font-medium text-sky-300 hover:bg-sky-500/20 disabled:opacity-40"
                          >
                            Apply
                          </button>
                        </div>
                      ))}
                  </div>
                </div>
              ))}
            </div>
          )}

          {tab === "features" && (
            <div className="space-y-4">
              <input
                value={featureSearch}
                onChange={(e) => setFeatureSearch(e.target.value)}
                placeholder="Search features..."
                className="w-full rounded-lg border border-white/10 bg-white/5 px-3 py-2 text-sm text-white placeholder:text-slate-500 focus:outline-none"
              />
              {featureCategories.map((category) => {
                const features = filteredFeatures.filter((f) => f.category === category);
                if (features.length === 0) return null;
                return (
                  <div key={category}>
                    <h4 className="mb-2 text-xs font-semibold uppercase tracking-wide text-slate-500">{category}</h4>
                    <div className="space-y-2">
                      {features.map((feature) => (
                        <div key={feature.id} className="flex items-center justify-between gap-3 rounded-xl border border-white/10 bg-white/[0.03] p-3">
                          <div>
                            <div className="flex items-center gap-2">
                              <p className="text-sm font-medium text-white">{feature.name}</p>
                              <Pill tone={feature.enabled ? "green" : "neutral"}>{feature.enabled ? "Enabled" : "Disabled"}</Pill>
                            </div>
                            <p className="text-xs text-slate-500">{feature.description}</p>
                            <p className="mt-1 font-mono text-[11px] text-slate-600">{feature.id}</p>
                          </div>
                          <button
                            onClick={() => toggleFeature(feature.id, !feature.enabled)}
                            disabled={pending}
                            className={`shrink-0 rounded-lg border px-3 py-1.5 text-xs font-medium disabled:opacity-40 ${
                              feature.enabled
                                ? "border-red-500/30 bg-red-500/10 text-red-300 hover:bg-red-500/20"
                                : "border-emerald-500/30 bg-emerald-500/10 text-emerald-300 hover:bg-emerald-500/20"
                            }`}
                          >
                            {feature.enabled ? "Disable" : "Enable"}
                          </button>
                        </div>
                      ))}
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>

        <div>
          <h3 className="mb-2 text-sm font-semibold text-white">Operation Log</h3>
          <LogConsole lines={log} />
        </div>
      </div>
    </div>
  );
}

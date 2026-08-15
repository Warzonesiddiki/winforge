"use client";

import { useState, useTransition } from "react";
import { buildCustomIso } from "@/lib/actions";
import { LogConsole } from "@/components/LogConsole";
import { useToast } from "@/components/Toast";
import { Pill } from "@/components/ui";

const EDITIONS = [
  { id: "win11-pro", name: "Windows 11 Pro", recommended: true },
  { id: "win11-home", name: "Windows 11 Home", recommended: false },
  { id: "win11-enterprise", name: "Windows 11 Enterprise", recommended: false },
  { id: "win11-iot-ltsc", name: "Windows 11 IoT Enterprise LTSC 2024", recommended: true },
  { id: "win10-pro", name: "Windows 10 Pro (22H2)", recommended: false },
];

const OPTIONS: { key: string; label: string; desc: string; warning?: boolean; category: string }[] = [
  { key: "removeBloatware", label: "Remove bloatware Appx packages", desc: "Strips 70+ preloaded consumer apps from the offline install.wim", category: "Debloat" },
  { key: "applyPrivacyTweaks", label: "Inject privacy registry hive tweaks", desc: "Pre-configures offline telemetry locks and privacy permissions", category: "Privacy" },
  { key: "removeEdge", label: "Remove Microsoft Edge", desc: "Completely strips Edge binary payload and setup stubs", warning: true, category: "Components" },
  { key: "removeOneDrive", label: "Remove OneDrive setup", desc: "Removes OneDrive installer and Explorer namespace tree integration", category: "Components" },
  { key: "removeRecall", label: "Remove Recall & AI components", desc: "Strips AI background capture runtime and Recall packages", category: "Privacy" },
  { key: "bypassTpm", label: "Bypass TPM 2.0 & SecureBoot check", desc: "Patches setup checks for compatibility with legacy hardware", warning: true, category: "Compatibility" },
  { key: "bypassNro", label: "Bypass Network Requirement (OOBE\\BypassNRO)", desc: "Allows creating offline local account during Windows setup without Wi-Fi", category: "Setup & OOBE" },
  { key: "disableDefenderPrompt", label: "Inject Autounattend.xml local admin answer file", desc: "Pre-answers setup questions and sets up initial offline user", category: "Setup & OOBE" },
  { key: "includePreset", label: "Include WinForge preset profile", desc: "Embeds WinForge standalone preset inside %ProgramData%\\WinForge", category: "WinForge" },
];

export function IsoClient() {
  const [edition, setEdition] = useState("win11-pro");
  const [arch, setArch] = useState<"x64" | "ARM64">("x64");
  const [options, setOptions] = useState<Record<string, boolean>>({
    removeBloatware: true,
    applyPrivacyTweaks: true,
    removeEdge: false,
    removeOneDrive: true,
    removeRecall: true,
    bypassTpm: false,
    bypassNro: true,
    disableDefenderPrompt: true,
    includePreset: true,
  });
  const [log, setLog] = useState<string[]>([]);
  const [sha, setSha] = useState<string | null>(null);
  const [pending, startTransition] = useTransition();
  const [copied, setCopied] = useState(false);
  const { addToast } = useToast();

  function toggle(key: string) {
    setOptions((prev) => ({ ...prev, [key]: !prev[key] }));
  }

  function selectAll() {
    const next: Record<string, boolean> = {};
    for (const opt of OPTIONS) next[opt.key] = true;
    setOptions(next);
  }

  function resetOptions() {
    setOptions({
      removeBloatware: true,
      applyPrivacyTweaks: true,
      removeEdge: false,
      removeOneDrive: true,
      removeRecall: true,
      bypassTpm: false,
      bypassNro: true,
      disableDefenderPrompt: true,
      includePreset: true,
    });
  }

  function build() {
    const selectedEdition = EDITIONS.find((e) => e.id === edition)?.name || "Windows 11 Pro";
    const filename = `WinForge-${edition}-${arch}.iso`;
    setSha(null);
    setLog([`Initializing MicroWin ISO pipeline for ${selectedEdition} (${arch})...`]);
    startTransition(async () => {
      const job = await buildCustomIso(options, {
        edition: selectedEdition,
        arch,
        targetFilename: filename,
      });
      for (const line of job.log) {
        await new Promise((r) => setTimeout(r, 180));
        setLog((prev) => [...prev, line]);
      }
      setSha(job.sha256 ?? null);
      addToast("success", "Custom ISO Built", `Generated ${filename} (${(job.sha256 ?? "").slice(0, 12)}...)`);
    });
  }

  function copySha() {
    if (!sha) return;
    navigator.clipboard.writeText(sha);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
    addToast("info", "Copied", "SHA-256 checksum copied to clipboard.");
  }

  const enabledCount = Object.values(options).filter(Boolean).length;

  return (
    <div className="mt-6 grid grid-cols-1 gap-6 lg:grid-cols-2">
      <div className="space-y-4">
        {/* Source Image & Edition card */}
        <div className="rounded-2xl border border-white/10 bg-white/[0.03] p-5">
          <div className="mb-3 flex items-center justify-between">
            <h3 className="text-sm font-semibold text-white">Target Edition & Architecture</h3>
            <Pill tone="blue">MicroWin Pipeline</Pill>
          </div>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <div>
              <label className="text-xs uppercase tracking-wide text-slate-400">Windows Edition</label>
              <select
                value={edition}
                onChange={(e) => setEdition(e.target.value)}
                className="mt-1.5 w-full rounded-lg border border-white/10 bg-white/5 px-3 py-2 text-sm text-white focus:outline-none"
              >
                {EDITIONS.map((e) => (
                  <option key={e.id} value={e.id}>
                    {e.name} {e.recommended ? "★ (Recommended)" : ""}
                  </option>
                ))}
              </select>
            </div>
            <div>
              <label className="text-xs uppercase tracking-wide text-slate-400">Architecture</label>
              <select
                value={arch}
                onChange={(e) => setArch(e.target.value as "x64" | "ARM64")}
                className="mt-1.5 w-full rounded-lg border border-white/10 bg-white/5 px-3 py-2 text-sm text-white focus:outline-none"
              >
                <option value="x64">x64 (Intel / AMD 64-bit)</option>
                <option value="ARM64">ARM64 (Snapdragon / Surface Pro)</option>
              </select>
            </div>
          </div>
        </div>

        {/* Customization Options */}
        <div className="rounded-2xl border border-white/10 bg-white/[0.03] p-5">
          <div className="mb-3 flex items-center justify-between">
            <div>
              <h3 className="text-sm font-semibold text-white">Offline Customizations ({enabledCount}/{OPTIONS.length})</h3>
              <p className="text-xs text-slate-500">Modifications applied directly to offline install.wim image</p>
            </div>
            <div className="flex gap-2">
              <button onClick={selectAll} className="text-xs text-sky-400 hover:text-sky-300">Select All</button>
              <span className="text-slate-600">·</span>
              <button onClick={resetOptions} className="text-xs text-slate-400 hover:text-slate-300">Reset</button>
            </div>
          </div>

          <div className="space-y-2.5">
            {OPTIONS.map((opt) => (
              <label
                key={opt.key}
                className="flex cursor-pointer items-start gap-3 rounded-xl border border-white/5 bg-black/20 p-3 transition hover:border-white/10 hover:bg-white/[0.02]"
              >
                <input
                  type="checkbox"
                  checked={!!options[opt.key]}
                  onChange={() => toggle(opt.key)}
                  className="mt-0.5"
                />
                <div className="flex-1">
                  <div className="flex items-center gap-2">
                    <span className={`text-sm font-medium ${opt.warning ? "text-amber-300" : "text-white"}`}>
                      {opt.label}
                    </span>
                    <span className="rounded bg-white/5 px-1.5 py-0.5 text-[10px] uppercase tracking-wider text-slate-400">
                      {opt.category}
                    </span>
                  </div>
                  <p className="mt-0.5 text-xs text-slate-400">{opt.desc}</p>
                </div>
              </label>
            ))}
          </div>

          <button
            onClick={build}
            disabled={pending}
            className="mt-5 w-full rounded-xl bg-sky-500 py-2.5 text-sm font-semibold text-white transition hover:bg-sky-400 disabled:opacity-40"
          >
            {pending ? "Executing MicroWin Pipeline..." : `Build Custom ISO (${enabledCount} mods)`}
          </button>
        </div>

        {sha && (
          <div className="rounded-2xl border border-emerald-500/30 bg-emerald-500/5 p-4">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <span className="text-emerald-400">✓</span>
                <p className="text-sm font-semibold text-white">ISO Generation Complete</p>
              </div>
              <button
                onClick={copySha}
                className="rounded-lg border border-emerald-500/30 bg-emerald-500/10 px-2.5 py-1 text-xs font-medium text-emerald-300 hover:bg-emerald-500/20"
              >
                {copied ? "Copied! ✓" : "Copy Checksum"}
              </button>
            </div>
            <p className="mt-2 text-xs text-slate-400">Cryptographic Verification Checksum (SHA-256):</p>
            <p className="mt-1 break-all rounded-lg bg-black/40 p-2 font-mono text-xs text-emerald-300">{sha}</p>
          </div>
        )}
      </div>

      <div>
        <div className="mb-2 flex items-center justify-between">
          <h3 className="text-sm font-semibold text-white">Pipeline Execution Log</h3>
          {pending && <span className="animate-pulse text-xs text-sky-400">● Live Building...</span>}
        </div>
        <LogConsole lines={log} />
      </div>
    </div>
  );
}

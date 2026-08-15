"use client";

import { useTransition, useState } from "react";
import {
  applyCommunityPack,
  createManualRestorePoint,
  resetSettingsToDefaults,
  restoreSystemRestorePoint,
  uninstallCommunityPack,
  updateSettings,
} from "@/lib/actions";
import { Toggle, Pill } from "@/components/ui";
import { useToast } from "@/components/Toast";
import { useLocale } from "@/components/LocaleProvider";
import { ConfirmModal } from "@/components/Modal";

interface Settings {
  id: number;
  theme: string;
  backdrop: string;
  language: string;
  restorePointBeforeMutation: boolean;
  showExpertTweaks: boolean;
  showCopilotTweaksSeparately: boolean;
  autoMaintenanceEnabled: boolean;
}

interface RestorePoint {
  id: number;
  sequenceNumber: number;
  description: string;
  createdAt: string;
}

interface CommunityPack {
  id: string;
  name: string;
  author: string;
  description: string;
  version: string;
  category: string;
  icon: string;
  tweakIds: string[];
  debloatPackages: string[];
  privacyRuleIds: string[];
  installed: boolean;
}

export function SettingsClient({
  settings,
  restorePoints,
  communityPacks = [],
}: {
  settings: Settings;
  restorePoints: RestorePoint[];
  communityPacks?: CommunityPack[];
}) {
  const [pending, startTransition] = useTransition();
  const [activeTab, setActiveTab] = useState<"settings" | "packs" | "restore">("settings");
  const [restoreTarget, setRestoreTarget] = useState<RestorePoint | null>(null);
  const { addToast } = useToast();
  const { setLocale, locale } = useLocale();

  function patch(fields: Partial<Settings>) {
    startTransition(async () => { await updateSettings(fields); });
  }

  function patchTheme(theme: string) {
    try {
      localStorage.setItem("wf-theme", theme);
      document.documentElement.style.colorScheme =
        theme === "light" ? "light" : theme === "system" ? "light dark" : "dark";
    } catch {
      // localStorage unavailable — persist server-side only
    }
    patch({ theme });
    addToast("info", "Theme Updated", `Theme set to ${theme.charAt(0).toUpperCase() + theme.slice(1)}.`);
  }

  function patchLang(language: string) {
    setLocale(language as "en-US" | "es-ES" | "fr-FR" | "de-DE" | "zh-CN");
    patch({ language });
    addToast("info", "Language Updated", `Locale set to ${language}. Navigation labels translated live.`);
  }

  function handleApplyPack(pack: CommunityPack) {
    startTransition(async () => {
      const r = await applyCommunityPack(pack.id);
      addToast(r.success ? "success" : "error", pack.name, r.message);
    });
  }

  function handleUninstallPack(pack: CommunityPack) {
    startTransition(async () => {
      const r = await uninstallCommunityPack(pack.id);
      addToast(r.success ? "info" : "error", pack.name, r.message);
    });
  }

  function handleRestoreSystem(rp: RestorePoint) {
    startTransition(async () => {
      const r = await restoreSystemRestorePoint(rp.sequenceNumber);
      addToast(r.success ? "success" : "error", "System Restore", r.message);
      setRestoreTarget(null);
    });
  }

  return (
    <div className="mt-6 space-y-6">
      {/* Tabs */}
      <div className="flex flex-wrap items-center gap-2 border-b border-white/10 pb-2">
        <button
          onClick={() => setActiveTab("settings")}
          className={`px-3 py-2 text-sm font-medium transition ${
            activeTab === "settings" ? "border-b-2 border-sky-400 text-sky-300" : "text-slate-400 hover:text-slate-200"
          }`}
        >
          General & Safety
        </button>
        <button
          onClick={() => setActiveTab("packs")}
          className={`px-3 py-2 text-sm font-medium transition ${
            activeTab === "packs" ? "border-b-2 border-sky-400 text-sky-300" : "text-slate-400 hover:text-slate-200"
          }`}
        >
          Community Profiles ({communityPacks.length} packs)
        </button>
        <button
          onClick={() => setActiveTab("restore")}
          className={`px-3 py-2 text-sm font-medium transition ${
            activeTab === "restore" ? "border-b-2 border-sky-400 text-sky-300" : "text-slate-400 hover:text-slate-200"
          }`}
        >
          Restore Points ({restorePoints.length})
        </button>
      </div>

      {activeTab === "settings" && (
        <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
          <div className="space-y-6">
            <section className="rounded-2xl border border-white/10 bg-white/[0.03] p-5">
              <h3 className="mb-4 text-sm font-semibold text-white">Appearance</h3>
              <div className="space-y-3">
                <div className="flex items-center justify-between">
                  <span className="text-sm text-slate-300">Theme</span>
                  <select defaultValue={settings.theme} onChange={(e) => patchTheme(e.target.value)} className="rounded-lg border border-white/10 bg-white/5 px-2 py-1 text-sm text-white">
                    <option value="light">Light</option>
                    <option value="dark">Dark</option>
                    <option value="system">System</option>
                  </select>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-sm text-slate-300">Backdrop</span>
                  <select defaultValue={settings.backdrop} onChange={(e) => patch({ backdrop: e.target.value })} className="rounded-lg border border-white/10 bg-white/5 px-2 py-1 text-sm text-white">
                    <option value="mica">Mica</option>
                    <option value="acrylic">Acrylic</option>
                    <option value="none">None</option>
                  </select>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-sm text-slate-300">Language</span>
                  <select defaultValue={settings.language} onChange={(e) => patchLang(e.target.value)} className="rounded-lg border border-white/10 bg-white/5 px-2 py-1 text-sm text-white">
                    <option value="en-US">English (US)</option>
                    <option value="es-ES">Español (Spanish)</option>
                    <option value="fr-FR">Français (French)</option>
                    <option value="de-DE">Deutsch (German)</option>
                    <option value="zh-CN">中文 (Simplified Chinese)</option>
                  </select>
                </div>
              </div>
            </section>

            <section className="rounded-2xl border border-white/10 bg-white/[0.03] p-5">
              <h3 className="mb-4 text-sm font-semibold text-white">Safety & Risk Guardrails</h3>
              <div className="space-y-4">
                <Row label="Always create restore point before mutations" desc="Creates shadow restore point before any change">
                  <Toggle checked={settings.restorePointBeforeMutation} onToggle={() => patch({ restorePointBeforeMutation: !settings.restorePointBeforeMutation })} />
                </Row>
                <Row label="Show expert / high-risk tweaks" desc="Off by default to protect stability">
                  <Toggle checked={settings.showExpertTweaks} onToggle={() => patch({ showExpertTweaks: !settings.showExpertTweaks })} />
                </Row>
                <Row label="Show AI / Copilot tweaks separately" desc="Dedicated Copilot & Recall badges">
                  <Toggle checked={settings.showCopilotTweaksSeparately} onToggle={() => patch({ showCopilotTweaksSeparately: !settings.showCopilotTweaksSeparately })} />
                </Row>
                <Row label="Scheduled maintenance re-verification" desc="Periodically verifies tweak state integrity">
                  <Toggle checked={settings.autoMaintenanceEnabled} onToggle={() => patch({ autoMaintenanceEnabled: !settings.autoMaintenanceEnabled })} />
                </Row>
              </div>
              <button
                onClick={() =>
                  startTransition(async () => {
                    await resetSettingsToDefaults();
                    addToast("warning", "Settings Reset", "All settings restored to defaults.");
                  })
                }
                disabled={pending}
                className="mt-5 w-full rounded-lg border border-red-500/30 bg-red-500/10 py-2 text-sm font-medium text-red-300 hover:bg-red-500/20"
              >
                Reset Settings to Defaults
              </button>
            </section>
          </div>

          <section className="rounded-2xl border border-white/10 bg-white/[0.03] p-5">
            <h3 className="mb-2 text-sm font-semibold text-white">About WinForge Elite</h3>
            <p className="text-sm font-medium text-sky-400">Version 2.0.0 — Unified Windows Suite</p>
            <p className="text-xs text-slate-500">Build {new Date().toISOString().slice(0, 10)} · Administrator Elevated</p>

            <div className="mt-4 space-y-3 text-xs text-slate-300 leading-relaxed border-t border-white/10 pt-4">
              <p>
                <strong className="text-white">Architecture:</strong> Native C# with Win32 P/Invoke, WMI System.Management, dynamic COM, and Appx PackageManager APIs — zero PowerShell scripts.
              </p>
              <p>
                <strong className="text-white">Community Heritage:</strong> Curated optimizations inspired by AtlasOS, ReviOS, and Chris Titus Tech Windows Utility.
              </p>
              <p>
                <strong className="text-white">Safety Guarantee:</strong> Every mutation is logged with full undo payload, guarded by restore points, and snapshot-reversible.
              </p>
            </div>
          </section>
        </div>
      )}

      {activeTab === "packs" && (
        <div className="space-y-4">
          <div className="rounded-2xl border border-white/10 bg-white/[0.02] p-4">
            <h3 className="text-sm font-semibold text-white">Community Optimization Packs</h3>
            <p className="text-xs text-slate-400">
              One-click install pre-configured bundles curated by leading Windows enthusiast communities.
            </p>
          </div>

          <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
            {communityPacks.map((pack) => (
              <div
                key={pack.id}
                className={`flex flex-col justify-between rounded-2xl border p-5 transition ${
                  pack.installed
                    ? "border-emerald-500/30 bg-emerald-500/[0.04]"
                    : "border-white/10 bg-white/[0.03]"
                }`}
              >
                <div>
                  <div className="flex items-start justify-between gap-3">
                    <div className="flex items-center gap-2.5">
                      <span className="text-2xl">{pack.icon}</span>
                      <div>
                        <h4 className="font-semibold text-white">{pack.name}</h4>
                        <p className="text-xs text-slate-500">By {pack.author} · v{pack.version}</p>
                      </div>
                    </div>
                    {pack.installed ? (
                      <Pill tone="green">Applied ✓</Pill>
                    ) : (
                      <Pill tone="blue">{pack.category}</Pill>
                    )}
                  </div>
                  <p className="mt-3 text-xs text-slate-300 leading-relaxed">{pack.description}</p>

                  <div className="mt-4 flex flex-wrap gap-2 text-[11px] text-slate-400">
                    <span className="rounded bg-white/5 px-2 py-0.5">{pack.tweakIds.length} Tweaks</span>
                    <span className="rounded bg-white/5 px-2 py-0.5">{pack.debloatPackages.length} Debloat</span>
                    <span className="rounded bg-white/5 px-2 py-0.5">{pack.privacyRuleIds.length} Privacy</span>
                  </div>
                </div>

                <div className="mt-5 border-t border-white/10 pt-3 flex gap-2">
                  {pack.installed ? (
                    <button
                      onClick={() => handleUninstallPack(pack)}
                      disabled={pending}
                      className="w-full rounded-xl border border-white/10 py-2 text-xs font-semibold text-slate-300 hover:bg-white/5 disabled:opacity-40"
                    >
                      Mark Uninstalled
                    </button>
                  ) : (
                    <button
                      onClick={() => handleApplyPack(pack)}
                      disabled={pending}
                      className="w-full rounded-xl bg-sky-500 py-2 text-xs font-semibold text-white hover:bg-sky-400 disabled:opacity-40"
                    >
                      Apply Community Pack
                    </button>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {activeTab === "restore" && (
        <section className="rounded-2xl border border-white/10 bg-white/[0.03] p-5">
          <div className="mb-4 flex items-center justify-between">
            <div>
              <h3 className="text-sm font-semibold text-white">System Restore Points</h3>
              <p className="text-xs text-slate-400">Captured before system mutations or created manually</p>
            </div>
            <button
              onClick={() =>
                startTransition(async () => {
                  await createManualRestorePoint(`Manual point ${new Date().toLocaleTimeString()}`);
                  addToast("success", "Restore Point Created", "A new system restore point has been saved.");
                })
              }
              disabled={pending}
              className="rounded-lg border border-sky-500/30 bg-sky-500/10 px-3.5 py-1.5 text-xs font-medium text-sky-300 hover:bg-sky-500/20"
            >
              + Create Manual Point
            </button>
          </div>
          <div className="space-y-2">
            {restorePoints.length === 0 && <p className="text-sm text-slate-500">No restore points yet.</p>}
            {restorePoints.map((rp) => (
              <div key={rp.id} className="flex items-center justify-between rounded-xl border border-white/5 bg-black/20 px-4 py-3">
                <div>
                  <p className="text-sm font-medium text-white">
                    #{rp.sequenceNumber} — {rp.description}
                  </p>
                  <p className="text-xs text-slate-500">{new Date(rp.createdAt).toLocaleString()}</p>
                </div>
                <button
                  onClick={() => setRestoreTarget(rp)}
                  className="rounded-lg border border-amber-500/30 bg-amber-500/10 px-3 py-1 text-xs font-medium text-amber-300 hover:bg-amber-500/20"
                >
                  Restore to Point
                </button>
              </div>
            ))}
          </div>
        </section>
      )}

      <ConfirmModal
        open={!!restoreTarget}
        onClose={() => setRestoreTarget(null)}
        onConfirm={() => {
          if (restoreTarget) handleRestoreSystem(restoreTarget);
        }}
        title="Restore System Restore Point"
        message={`Simulate system restore to point #${restoreTarget?.sequenceNumber} ("${restoreTarget?.description}")? This action will be logged in History.`}
        confirmLabel="Restore"
        danger
      />
    </div>
  );
}

function Row({ label, desc, children }: { label: string; desc: string; children: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-4">
      <div>
        <p className="text-sm text-slate-200">{label}</p>
        <p className="text-xs text-slate-500">{desc}</p>
      </div>
      {children}
    </div>
  );
}

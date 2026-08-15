"use client";

import { useMemo, useRef, useState } from "react";
import { applyPreset, importTweakSelection, setTweakApplied } from "@/lib/actions";
import { RiskBadge, Toggle, ActionButton, Pill } from "@/components/ui";
import { useToast } from "@/components/Toast";
import { Modal, WarningBox, ConfirmModal } from "@/components/Modal";
import { Pagination, usePagination } from "@/components/Pagination";
import type { RiskLevel } from "@/lib/types";

interface TweakRow {
  id: string;
  name: string;
  description: string;
  category: string;
  risk: RiskLevel;
  defaultEnabled: boolean;
  applied: boolean;
  tags: string[];
  warningMessage: string | null;
  breaksFeatures: string[];
  operations: string[];
  undoOperations: string[];
}

export function TweaksClient({
  tweaks,
  presets,
  showExpert,
  initialSearch = "",
}: {
  tweaks: TweakRow[];
  presets: { id: string; name: string }[];
  showExpert: boolean;
  initialSearch?: string;
}) {
  const categories = useMemo(() => Array.from(new Set(tweaks.map((t) => t.category))).sort(), [tweaks]);
  const [activeCategory, setActiveCategory] = useState<string>("All");
  const [filterState, setFilterState] = useState<"all" | "applied" | "unapplied" | "recommended">("all");
  const [search, setSearch] = useState(initialSearch);
  const [selectedTweak, setSelectedTweak] = useState<TweakRow | null>(tweaks[0] || null);
  const [dryRun, setDryRun] = useState<TweakRow | null>(null);
  const [presetConfirm, setPresetConfirm] = useState<{ id: string; name: string } | null>(null);
  const fileRef = useRef<HTMLInputElement>(null);
  const { addToast } = useToast();

  const filteredAll = tweaks.filter((t) => {
    if (!showExpert && t.risk === "expert") return false;
    if (activeCategory !== "All" && t.category !== activeCategory) return false;
    if (filterState === "applied" && !t.applied) return false;
    if (filterState === "unapplied" && t.applied) return false;
    if (filterState === "recommended" && !t.defaultEnabled) return false;
    if (search && !`${t.name} ${t.description} ${t.tags.join(" ")} ${t.category}`.toLowerCase().includes(search.toLowerCase())) return false;
    return true;
  });

  const { items: visible, page, setPage, totalPages, totalItems, perPage } = usePagination(filteredAll, 12);

  const hiddenExpertCount = tweaks.filter((t) => t.risk === "expert").length;
  const totalApplied = tweaks.filter((t) => t.applied).length;

  function handleExport() {
    const applied = tweaks.filter((t) => t.applied).map((t) => t.id);
    const blob = new Blob([JSON.stringify({ tweakIds: applied, exportedAt: new Date().toISOString() }, null, 2)], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "custom-preset.winforge";
    a.click();
    URL.revokeObjectURL(url);
    addToast("info", "Preset Exported", `${applied.length} tweak(s) exported to custom-preset.winforge`);
  }

  function handleImportFile(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = () => {
      try {
        const parsed = JSON.parse(String(reader.result));
        const ids: string[] = parsed.tweakIds ?? [];
        importTweakSelection(ids);
        addToast("success", "Preset Imported", `${ids.length} tweak(s) imported and applied.`);
      } catch {
        addToast("error", "Import Failed", "Invalid .winforge file format.");
      }
    };
    reader.readAsText(file);
    e.target.value = "";
  }

  async function handleToggle(t: TweakRow) {
    const result = await setTweakApplied(t.id, !t.applied);
    addToast(result.success ? "success" : "error", t.name, result.message);
    if (selectedTweak?.id === t.id) {
      setSelectedTweak({ ...t, applied: !t.applied });
    }
  }

  async function handlePreset(presetId: string, presetName: string) {
    const result = await applyPreset(presetId);
    addToast(result.success ? "success" : "error", `Preset: ${presetName}`, result.message);
  }

  const inspector = selectedTweak || visible[0] || null;

  return (
    <div className="mt-6 space-y-4">
      {/* Top Toolbar */}
      <div className="flex flex-wrap items-center gap-3 rounded-2xl border border-white/10 bg-white/[0.02] p-4">
        <input
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Search 60+ tweaks by name, tag, or description..."
          className="w-72 rounded-lg border border-white/10 bg-white/5 px-3 py-2 text-sm text-white placeholder:text-slate-500 focus:border-sky-500/50 focus:outline-none"
        />

        {/* Quick Filter Pills */}
        <div className="flex items-center gap-1.5">
          {[
            { id: "all" as const, label: `All (${tweaks.length})` },
            { id: "recommended" as const, label: "Recommended" },
            { id: "applied" as const, label: `Applied (${totalApplied})` },
            { id: "unapplied" as const, label: `Pending (${tweaks.length - totalApplied})` },
          ].map((f) => (
            <button
              key={f.id}
              onClick={() => setFilterState(f.id)}
              className={`rounded-lg px-2.5 py-1.5 text-xs font-medium transition ${
                filterState === f.id
                  ? "bg-sky-500/20 text-sky-300 border border-sky-500/40"
                  : "border border-white/5 text-slate-400 hover:bg-white/5"
              }`}
            >
              {f.label}
            </button>
          ))}
        </div>

        {/* Preset & Import/Export */}
        <div className="ml-auto flex flex-wrap items-center gap-2">
          <select
            defaultValue=""
            onChange={(e) => {
              const val = e.target.value;
              if (val) {
                const presetName = presets.find((p) => p.id === val)?.name ?? val;
                setPresetConfirm({ id: val, name: presetName });
              }
              e.target.value = "";
            }}
            className="rounded-lg border border-white/10 bg-white/5 px-3 py-2 text-sm text-white focus:outline-none"
          >
            <option value="">Apply preset profile…</option>
            {presets.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
              </option>
            ))}
          </select>
          <ActionButton label="Export .winforge" onClick={async () => handleExport()} />
          <ActionButton label="Import .winforge" onClick={async () => fileRef.current?.click()} />
          <input ref={fileRef} type="file" accept=".json,.winforge" onChange={handleImportFile} className="hidden" />
        </div>
      </div>

      {/* 3-Panel Architecture: [Panel 1: Category Tree] | [Panel 2: Tweak Cards] | [Panel 3: Live State Inspector] */}
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-12">
        {/* PANEL 1: Category Tree (2 Cols) */}
        <div className="space-y-1 lg:col-span-2">
          <button
            onClick={() => setActiveCategory("All")}
            className={`flex w-full items-center justify-between rounded-xl px-3 py-2.5 text-left text-sm font-medium transition ${
              activeCategory === "All" ? "bg-sky-500/15 text-sky-300 border border-sky-500/30" : "text-slate-400 hover:bg-white/5"
            }`}
          >
            <span>All Categories</span>
            <span className="text-xs text-slate-500">{tweaks.length}</span>
          </button>
          {categories.map((c) => {
            const count = tweaks.filter((t) => t.category === c).length;
            const appliedInCat = tweaks.filter((t) => t.category === c && t.applied).length;
            return (
              <button
                key={c}
                onClick={() => setActiveCategory(c)}
                className={`flex w-full items-center justify-between rounded-xl px-3 py-2.5 text-left text-sm font-medium transition ${
                  activeCategory === c ? "bg-sky-500/15 text-sky-300 border border-sky-500/30" : "text-slate-400 hover:bg-white/5"
                }`}
              >
                <span>{c}</span>
                <span className="text-xs text-slate-500">{appliedInCat}/{count}</span>
              </button>
            );
          })}
          {!showExpert && hiddenExpertCount > 0 && (
            <div className="mt-4 rounded-xl border border-purple-500/20 bg-purple-500/10 p-3">
              <p className="text-xs font-semibold text-purple-300">Expert Mode Off</p>
              <p className="mt-1 text-[11px] text-purple-300/80">
                {hiddenExpertCount} high-impact tweaks hidden. Reveal in Settings.
              </p>
            </div>
          )}
        </div>

        {/* PANEL 2: Tweak Cards Grid (6 Cols) */}
        <div className="space-y-4 lg:col-span-6">
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            {visible.map((t) => {
              const isSelected = selectedTweak?.id === t.id;
              return (
                <div
                  key={t.id}
                  onClick={() => setSelectedTweak(t)}
                  className={`cursor-pointer flex flex-col justify-between rounded-2xl border p-4 transition ${
                    isSelected
                      ? "border-sky-500/50 bg-sky-500/[0.07] ring-1 ring-sky-500/30"
                      : t.applied
                      ? "border-emerald-500/20 bg-emerald-500/[0.02] hover:border-white/20"
                      : "border-white/10 bg-white/[0.02] hover:border-white/20"
                  }`}
                >
                  <div>
                    <div className="flex items-start justify-between gap-2">
                      <div className="min-w-0 flex-1">
                        <p className="truncate text-sm font-semibold text-white" title={t.name}>{t.name}</p>
                        <p className="text-[11px] text-slate-500">{t.category}</p>
                      </div>
                      <div onClick={(e) => e.stopPropagation()}>
                        <Toggle checked={t.applied} onToggle={() => handleToggle(t)} size="sm" />
                      </div>
                    </div>
                    <p className="mt-2 line-clamp-2 text-xs text-slate-400">{t.description}</p>
                  </div>

                  <div className="mt-3 flex items-center justify-between gap-1 pt-2 border-t border-white/5">
                    <RiskBadge risk={t.risk} />
                    <div className="flex items-center gap-1.5" onClick={(e) => e.stopPropagation()}>
                      <button
                        onClick={() => setDryRun(t)}
                        className="text-[11px] text-sky-400 hover:text-sky-300"
                      >
                        Dry Run
                      </button>
                    </div>
                  </div>
                </div>
              );
            })}
          </div>

          {visible.length === 0 && (
            <div className="rounded-2xl border border-dashed border-white/10 p-8 text-center text-sm text-slate-500">
              No tweaks match your active filters.
            </div>
          )}

          {totalPages > 1 && (
            <div className="border-t border-white/10 pt-3">
              <Pagination
                currentPage={page}
                totalPages={totalPages}
                onPageChange={setPage}
                totalItems={totalItems}
                itemsPerPage={perPage}
              />
            </div>
          )}
        </div>

        {/* PANEL 3: Live State Inspector (4 Cols) */}
        <div className="lg:col-span-4">
          <div className="sticky top-20 rounded-2xl border border-white/10 bg-white/[0.03] p-5">
            <div className="mb-3 flex items-center justify-between border-b border-white/10 pb-3">
              <div>
                <span className="text-[10px] font-semibold uppercase tracking-wider text-sky-400">Live State Inspector</span>
                <h3 className="text-sm font-semibold text-white">
                  {inspector ? inspector.name : "Select a tweak"}
                </h3>
              </div>
              {inspector && (
                <Pill tone={inspector.applied ? "green" : "neutral"}>
                  {inspector.applied ? "Applied ✓" : "Pending"}
                </Pill>
              )}
            </div>

            {inspector ? (
              <div className="space-y-4 text-xs">
                <div>
                  <span className="text-slate-500 uppercase tracking-wider text-[10px]">Description</span>
                  <p className="mt-1 text-slate-300 leading-relaxed">{inspector.description}</p>
                </div>

                {inspector.warningMessage && (
                  <div className="rounded-lg border border-amber-500/30 bg-amber-500/10 p-2.5 text-amber-200">
                    ⚠️ <span className="font-medium">Warning:</span> {inspector.warningMessage}
                  </div>
                )}

                {inspector.breaksFeatures.length > 0 && (
                  <div className="rounded-lg border border-red-500/30 bg-red-500/10 p-2.5 text-red-200">
                    🚨 <span className="font-medium">May Affect:</span> {inspector.breaksFeatures.join(", ")}
                  </div>
                )}

                <div>
                  <span className="text-slate-500 uppercase tracking-wider text-[10px]">Registry & Subsystem Mutations</span>
                  <ul className="mt-1.5 space-y-1 rounded-lg bg-black/40 p-2.5 font-mono text-[11px] text-emerald-300">
                    {inspector.operations.map((op, i) => (
                      <li key={i} className="break-all">+ {op}</li>
                    ))}
                  </ul>
                </div>

                {inspector.undoOperations.length > 0 && (
                  <div>
                    <span className="text-slate-500 uppercase tracking-wider text-[10px]">Undo Reversal Payload</span>
                    <ul className="mt-1.5 space-y-1 rounded-lg bg-black/40 p-2.5 font-mono text-[11px] text-slate-400">
                      {inspector.undoOperations.map((op, i) => (
                        <li key={i} className="break-all">- {op}</li>
                      ))}
                    </ul>
                  </div>
                )}

                <div>
                  <span className="text-slate-500 uppercase tracking-wider text-[10px]">Tags</span>
                  <div className="mt-1.5 flex flex-wrap gap-1">
                    {inspector.tags.map((tag) => (
                      <span key={tag} className="rounded-md bg-white/5 px-2 py-0.5 text-[10px] text-slate-400">
                        #{tag}
                      </span>
                    ))}
                  </div>
                </div>

                <div className="pt-2 border-t border-white/10 flex gap-2">
                  <button
                    onClick={() => handleToggle(inspector)}
                    className={`flex-1 rounded-xl py-2 font-semibold transition ${
                      inspector.applied
                        ? "border border-amber-500/40 bg-amber-500/10 text-amber-300 hover:bg-amber-500/20"
                        : "bg-sky-500 text-white hover:bg-sky-400"
                    }`}
                  >
                    {inspector.applied ? "Revert / Undo Tweak" : "Apply Tweak Now"}
                  </button>
                  <button
                    onClick={() => setDryRun(inspector)}
                    className="rounded-xl border border-white/10 px-3 py-2 text-slate-300 hover:bg-white/5"
                  >
                    Dry Run
                  </button>
                </div>
              </div>
            ) : (
              <p className="py-8 text-center text-xs text-slate-500">
                Click any tweak card to inspect its exact Win32 & registry mutation payload.
              </p>
            )}
          </div>
        </div>
      </div>

      {/* Dry Run Modal */}
      <Modal
        open={!!dryRun}
        onClose={() => setDryRun(null)}
        title={dryRun ? `${dryRun.name} — Dry Run Preview` : ""}
        footer={
          <div className="flex justify-end gap-2">
            <button onClick={() => setDryRun(null)} className="rounded-lg border border-white/10 px-3 py-1.5 text-sm text-slate-300 hover:bg-white/5">
              Close
            </button>
            <button
              onClick={() => {
                if (dryRun) handleToggle(dryRun);
                setDryRun(null);
              }}
              className="rounded-lg bg-sky-500 px-3 py-1.5 text-sm font-medium text-white hover:bg-sky-400"
            >
              {dryRun?.applied ? "Undo Now" : "Apply Now"}
            </button>
          </div>
        }
      >
        {dryRun && (
          <div>
            {dryRun.warningMessage && (
              <div className="mb-3">
                <WarningBox title="Warning">{dryRun.warningMessage}</WarningBox>
              </div>
            )}
            <p className="text-xs text-slate-500">The following operations would be executed:</p>
            <ul className="mt-3 space-y-1.5 rounded-lg bg-black/40 p-3 font-mono text-xs text-emerald-300">
              {dryRun.operations.map((op, i) => (
                <li key={i} className="break-all">+ {op}</li>
              ))}
            </ul>
            {dryRun.breaksFeatures.length > 0 && (
              <div className="mt-3">
                <WarningBox title="May Affect These Features" tone="danger">
                  {dryRun.breaksFeatures.join(", ")}
                </WarningBox>
              </div>
            )}
            {dryRun.undoOperations.length > 0 && (
              <>
                <p className="mt-3 text-xs text-slate-500">Undo would restore:</p>
                <ul className="mt-1 space-y-1.5 rounded-lg bg-black/40 p-3 font-mono text-xs text-slate-400">
                  {dryRun.undoOperations.map((op, i) => (
                    <li key={i} className="break-all">- {op}</li>
                  ))}
                </ul>
              </>
            )}
          </div>
        )}
      </Modal>

      <ConfirmModal
        open={!!presetConfirm}
        onClose={() => setPresetConfirm(null)}
        onConfirm={() => {
          if (presetConfirm) handlePreset(presetConfirm.id, presetConfirm.name);
          setPresetConfirm(null);
        }}
        title={`Apply "${presetConfirm?.name}" Preset`}
        message={`This will apply the ${presetConfirm?.name} preset — changing multiple tweak states, removing bloatware packages, and enabling privacy rules. A restore point will be created automatically. Continue?`}
        confirmLabel="Apply Preset"
        loading={false}
      />
    </div>
  );
}

"use client";

import { useMemo, useState } from "react";
import { bulkRemovePackages, setContextMenuItemEnabled, setPackageStatus, setStartupEnabled } from "@/lib/actions";
import { RiskBadge, Pill, ActionButton, Toggle } from "@/components/ui";
import { useToast } from "@/components/Toast";
import { Pagination, usePagination } from "@/components/Pagination";
import { ConfirmModal } from "@/components/Modal";
import type { RiskLevel } from "@/lib/types";

interface PackageRow {
  packageName: string;
  displayName: string;
  category: string;
  risk: RiskLevel;
  canReinstall: boolean;
  storeId: string | null;
  status: "installed" | "removed" | "protected";
  provisionedRemoved: boolean;
}

interface StartupRow {
  id: string;
  name: string;
  publisher: string;
  command: string;
  impact: string;
  enabled: boolean;
}

interface ContextMenuRow {
  id: string;
  title: string;
  description: string;
  registryKey: string;
  targetExtension: string;
  enabled: boolean;
  risk: RiskLevel;
  category: string;
}

export function DebloatClient({
  packages,
  startupItems,
  contextMenus = [],
  initialSearch = "",
}: {
  packages: PackageRow[];
  startupItems: StartupRow[];
  contextMenus?: ContextMenuRow[];
  initialSearch?: string;
}) {
  const categories = useMemo(() => Array.from(new Set(packages.map((p) => p.category))).sort(), [packages]);
  const [activeCategory, setActiveCategory] = useState("All");
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [tab, setTab] = useState<"packages" | "startup" | "context">("packages");
  const [search, setSearch] = useState(initialSearch);
  const [confirmBulk, setConfirmBulk] = useState(false);
  const { addToast } = useToast();

  const visibleAll = packages.filter((p) => {
    if (activeCategory !== "All" && p.category !== activeCategory) return false;
    if (search && !`${p.displayName} ${p.packageName}`.toLowerCase().includes(search.toLowerCase())) return false;
    return true;
  });

  const { items: visible, page, setPage, totalPages, totalItems, perPage } = usePagination(visibleAll, 15);

  const contextAll = contextMenus.filter((c) => {
    if (search && !`${c.title} ${c.description} ${c.registryKey}`.toLowerCase().includes(search.toLowerCase())) return false;
    return true;
  });
  const { items: visibleContext, page: cPage, setPage: setCPage, totalPages: cPages, totalItems: cTotal, perPage: cPerPage } = usePagination(contextAll, 10);

  function toggleSelect(name: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(name)) next.delete(name);
      else next.add(name);
      return next;
    });
  }

  function selectCategory() {
    setSelected((prev) => {
      const next = new Set(prev);
      for (const p of visibleAll) {
        if (p.status === "installed") next.add(p.packageName);
      }
      return next;
    });
  }

  async function removeSelected() {
    if (selected.size === 0) return;
    setConfirmBulk(true);
  }

  async function confirmRemoveSelected() {
    const count = selected.size;
    await bulkRemovePackages(Array.from(selected), true);
    addToast("success", "Packages Removed", `${count} package(s) removed. Undo from History.`);
    setSelected(new Set());
    setConfirmBulk(false);
  }

  async function handleRemove(pkg: PackageRow) {
    await setPackageStatus(pkg.packageName, true, true);
    addToast("success", "Package Removed", `${pkg.displayName} removed successfully.`);
  }

  async function handleReinstall(pkg: PackageRow) {
    await setPackageStatus(pkg.packageName, false);
    addToast("success", "Package Reinstalled", `${pkg.displayName} reinstalled.`);
  }

  async function handleToggleContextMenu(cm: ContextMenuRow) {
    const result = await setContextMenuItemEnabled(cm.id, !cm.enabled);
    addToast(result.success ? "success" : "error", cm.title, result.message);
  }

  const installedCount = packages.filter((p) => p.status === "installed").length;
  const hiddenContextCount = contextMenus.filter((c) => !c.enabled).length;

  return (
    <div className="mt-6">
      {/* Module Tabs */}
      <div className="mb-4 flex flex-wrap items-center gap-2 border-b border-white/10 pb-2">
        <button
          onClick={() => setTab("packages")}
          className={`px-3 py-2 text-sm font-medium transition ${tab === "packages" ? "border-b-2 border-sky-400 text-sky-300" : "text-slate-400 hover:text-slate-200"}`}
        >
          Appx Packages ({installedCount} installed · {packages.length - installedCount} removed)
        </button>
        <button
          onClick={() => setTab("context")}
          className={`px-3 py-2 text-sm font-medium transition ${tab === "context" ? "border-b-2 border-sky-400 text-sky-300" : "text-slate-400 hover:text-slate-200"}`}
        >
          Context Menu Cleaner ({contextMenus.length} items · {hiddenContextCount} cleaned)
        </button>
        <button
          onClick={() => setTab("startup")}
          className={`px-3 py-2 text-sm font-medium transition ${tab === "startup" ? "border-b-2 border-sky-400 text-sky-300" : "text-slate-400 hover:text-slate-200"}`}
        >
          Startup Manager ({startupItems.filter((s) => s.enabled).length} enabled)
        </button>
      </div>

      {tab === "packages" ? (
        <div className="grid grid-cols-1 gap-6 lg:grid-cols-[220px_1fr]">
          <div className="space-y-1">
            <button
              onClick={() => setActiveCategory("All")}
              className={`block w-full rounded-lg px-3 py-2 text-left text-sm transition ${activeCategory === "All" ? "bg-sky-500/15 text-sky-300 border border-sky-500/30" : "text-slate-400 hover:bg-white/5"}`}
            >
              All Categories ({packages.length})
            </button>
            {categories.map((c) => (
              <button
                key={c}
                onClick={() => setActiveCategory(c)}
                className={`block w-full rounded-lg px-3 py-2 text-left text-sm transition ${activeCategory === c ? "bg-sky-500/15 text-sky-300 border border-sky-500/30" : "text-slate-400 hover:bg-white/5"}`}
              >
                {c} ({packages.filter((p) => p.category === c).length})
              </button>
            ))}
          </div>

          <div>
            <div className="mb-4 flex flex-wrap items-center gap-2">
              <input
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder="Search packages..."
                className="w-56 rounded-lg border border-white/10 bg-white/5 px-3 py-1.5 text-sm text-white placeholder:text-slate-500 focus:outline-none"
              />
              <ActionButton label="Select all in view" onClick={async () => selectCategory()} />
              <ActionButton label={`Clear selection (${selected.size})`} onClick={async () => setSelected(new Set())} />
              <button
                onClick={removeSelected}
                disabled={selected.size === 0}
                className="ml-auto rounded-lg border border-red-500/40 bg-red-500/10 px-3.5 py-1.5 text-sm font-medium text-red-300 hover:bg-red-500/20 disabled:opacity-30"
              >
                Remove Selected ({selected.size})
              </button>
            </div>

            <div className="overflow-hidden rounded-2xl border border-white/10">
              <table className="w-full text-sm">
                <thead className="bg-white/5 text-left text-xs uppercase tracking-wide text-slate-400">
                  <tr>
                    <th className="w-10 px-4 py-3"></th>
                    <th className="px-4 py-3">Package</th>
                    <th className="px-4 py-3">Category</th>
                    <th className="px-4 py-3">Risk</th>
                    <th className="px-4 py-3">Status</th>
                    <th className="px-4 py-3 text-right">Action</th>
                  </tr>
                </thead>
                <tbody>
                  {visible.map((p) => (
                    <tr key={p.packageName} className="border-t border-white/5 hover:bg-white/[0.02]">
                      <td className="px-4 py-3">
                        {p.status !== "protected" && (
                          <input
                            type="checkbox"
                            checked={selected.has(p.packageName)}
                            onChange={() => toggleSelect(p.packageName)}
                            disabled={p.status === "removed"}
                          />
                        )}
                      </td>
                      <td className="px-4 py-3">
                        <p className="font-medium text-white">{p.displayName}</p>
                        <p className="text-xs text-slate-500">{p.packageName}</p>
                      </td>
                      <td className="px-4 py-3 text-slate-400">{p.category}</td>
                      <td className="px-4 py-3">
                        <RiskBadge risk={p.risk} />
                      </td>
                      <td className="px-4 py-3">
                        {p.status === "installed" && <Pill tone="blue">Installed</Pill>}
                        {p.status === "removed" && <Pill tone="green">Removed</Pill>}
                        {p.status === "protected" && <Pill tone="red">Protected</Pill>}
                      </td>
                      <td className="px-4 py-3 text-right">
                        {p.status === "installed" && (
                          <ActionButton label="Remove" onClick={async () => handleRemove(p)} />
                        )}
                        {p.status === "removed" && p.canReinstall && (
                          <ActionButton label="Reinstall" onClick={async () => handleReinstall(p)} />
                        )}
                        {p.status === "protected" && <span className="text-xs text-slate-600">🔒 Locked</span>}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
              <div className="border-t border-white/10 px-4 py-3">
                <Pagination
                  currentPage={page}
                  totalPages={totalPages}
                  onPageChange={setPage}
                  totalItems={totalItems}
                  itemsPerPage={perPage}
                />
              </div>
            </div>
          </div>
        </div>
      ) : tab === "context" ? (
        <div className="space-y-4">
          <div className="flex items-center justify-between rounded-xl border border-white/10 bg-white/[0.02] p-4">
            <div>
              <h3 className="text-sm font-semibold text-white">Explorer Right-Click Context Menu Cleaner</h3>
              <p className="text-xs text-slate-400">
                Disable bloated third-party and legacy Microsoft shell extensions from cluttering the Explorer right-click menu.
              </p>
            </div>
            <input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search context entries..."
              className="w-56 rounded-lg border border-white/10 bg-white/5 px-3 py-1.5 text-sm text-white placeholder:text-slate-500 focus:outline-none"
            />
          </div>

          <div className="overflow-hidden rounded-2xl border border-white/10">
            <table className="w-full text-sm">
              <thead className="bg-white/5 text-left text-xs uppercase tracking-wide text-slate-400">
                <tr>
                  <th className="px-4 py-3">Menu Entry</th>
                  <th className="px-4 py-3">Target Scope</th>
                  <th className="px-4 py-3">Category</th>
                  <th className="px-4 py-3">Status</th>
                  <th className="px-4 py-3 text-right">Visibility</th>
                </tr>
              </thead>
              <tbody>
                {visibleContext.map((cm) => (
                  <tr key={cm.id} className="border-t border-white/5 hover:bg-white/[0.02]">
                    <td className="px-4 py-3">
                      <p className="font-medium text-white">{cm.title}</p>
                      <p className="text-xs font-mono text-slate-500">{cm.registryKey}</p>
                    </td>
                    <td className="px-4 py-3 font-mono text-xs text-slate-400">{cm.targetExtension}</td>
                    <td className="px-4 py-3 text-slate-400">{cm.category}</td>
                    <td className="px-4 py-3">
                      <Pill tone={cm.enabled ? "blue" : "green"}>
                        {cm.enabled ? "Visible" : "Cleaned ✓"}
                      </Pill>
                    </td>
                    <td className="px-4 py-3 text-right">
                      <button
                        onClick={() => handleToggleContextMenu(cm)}
                        className={`rounded-lg border px-3 py-1 text-xs font-medium transition ${
                          cm.enabled
                            ? "border-red-500/30 bg-red-500/10 text-red-300 hover:bg-red-500/20"
                            : "border-emerald-500/30 bg-emerald-500/10 text-emerald-300 hover:bg-emerald-500/20"
                        }`}
                      >
                        {cm.enabled ? "Clean (Hide)" : "Restore"}
                      </button>
                    </td>
                  </tr>
                ))}
                {visibleContext.length === 0 && (
                  <tr>
                    <td colSpan={5} className="px-4 py-6 text-center text-sm text-slate-500">
                      No context menu items match search.
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
            {cPages > 1 && (
              <div className="border-t border-white/10 px-4 py-3">
                <Pagination
                  currentPage={cPage}
                  totalPages={cPages}
                  onPageChange={setCPage}
                  totalItems={cTotal}
                  itemsPerPage={cPerPage}
                />
              </div>
            )}
          </div>
        </div>
      ) : (
        <div className="overflow-hidden rounded-2xl border border-white/10">
          <table className="w-full text-sm">
            <thead className="bg-white/5 text-left text-xs uppercase tracking-wide text-slate-400">
              <tr>
                <th className="px-4 py-3">Program</th>
                <th className="px-4 py-3">Command</th>
                <th className="px-4 py-3">Startup Impact</th>
                <th className="px-4 py-3 text-right">Enabled</th>
              </tr>
            </thead>
            <tbody>
              {startupItems.map((s) => (
                <tr key={s.id} className="border-t border-white/5">
                  <td className="px-4 py-3">
                    <p className="font-medium text-white">{s.name}</p>
                    <p className="text-xs text-slate-500">{s.publisher}</p>
                  </td>
                  <td className="max-w-md truncate px-4 py-3 font-mono text-xs text-slate-500">{s.command}</td>
                  <td className="px-4 py-3">
                    <Pill tone={s.impact === "high" ? "red" : s.impact === "medium" ? "amber" : "green"}>{s.impact}</Pill>
                  </td>
                  <td className="px-4 py-3 text-right">
                    <Toggle checked={s.enabled} onToggle={() => setStartupEnabled(s.id, !s.enabled)} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <ConfirmModal
        open={confirmBulk}
        onClose={() => setConfirmBulk(false)}
        onConfirm={confirmRemoveSelected}
        title="Bulk Remove Packages"
        message={`Remove ${selected.size} selected package(s)? Each removal is logged and individually undoable from the History & Undo module.`}
        confirmLabel={`Remove ${selected.size} Package(s)`}
        danger
      />
    </div>
  );
}

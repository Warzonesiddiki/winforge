"use client";

import { useMemo, useState } from "react";
import { setAppInstalled } from "@/lib/actions";
import { Pill } from "@/components/ui";
import { useToast } from "@/components/Toast";
import { Pagination, usePagination } from "@/components/Pagination";
import { LogConsole } from "@/components/LogConsole";

interface AppRow {
  id: string;
  name: string;
  publisher: string;
  category: string;
  version: string;
  source: string;
  installed: boolean;
}

export function InstallClient({ apps, initialSearch = "" }: { apps: AppRow[]; initialSearch?: string }) {
  const categories = useMemo(() => Array.from(new Set(apps.map((a) => a.category))), [apps]);
  const [activeCategory, setActiveCategory] = useState("All");
  const [search, setSearch] = useState(initialSearch);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [installing, setInstalling] = useState<Set<string>>(new Set());
  const [log, setLog] = useState<string[]>([]);
  const [queueRunning, setQueueRunning] = useState(false);
  const { addToast } = useToast();

  const visibleAll = apps.filter((a) => {
    if (activeCategory !== "All" && a.category !== activeCategory) return false;
    if (search && !`${a.name} ${a.publisher}`.toLowerCase().includes(search.toLowerCase())) return false;
    return true;
  });

  const { items: visible, page, setPage, totalPages, totalItems, perPage } = usePagination(visibleAll, 24);

  function appendLog(line: string) {
    setLog((prev) => [...prev, line]);
  }

  async function installOne(app: AppRow, notify = false) {
    setInstalling((prev) => new Set(prev).add(app.id));
    appendLog(`winget install --id ${app.id} --source ${app.source} --accept-package-agreements`);
    await new Promise((r) => setTimeout(r, 700 + Math.random() * 600));
    appendLog(`  Downloading ${app.name} ${app.version}...`);
    await new Promise((r) => setTimeout(r, 500 + Math.random() * 500));
    await setAppInstalled(app.id, true);
    appendLog(`  Successfully installed ${app.name}.`);
    if (notify) {
      addToast("success", "Installed", `${app.name} has been installed.`);
    }
    setInstalling((prev) => {
      const next = new Set(prev);
      next.delete(app.id);
      return next;
    });
  }

  async function handleUninstall(app: AppRow) {
    await setAppInstalled(app.id, false);
    appendLog(`Uninstalled ${app.name}.`);
    addToast("info", "Uninstalled", `${app.name} was removed.`);
  }

  async function installSelected() {
    if (selected.size === 0) return;
    setQueueRunning(true);
    const toInstall = apps.filter((a) => selected.has(a.id) && !a.installed);
    appendLog(`Starting batch install of ${toInstall.length} app(s)...`);
    for (const app of toInstall) {
      await installOne(app);
    }
    appendLog("Batch install complete.");
    addToast("success", "Batch Install Complete", `${toInstall.length} application(s) installed successfully.`);
    setSelected(new Set());
    setQueueRunning(false);
  }

  function toggleSelect(id: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  return (
    <div className="mt-6">
      <div className="mb-4 flex flex-wrap items-center gap-2">
        <button
          onClick={() => setActiveCategory("All")}
          className={`rounded-full border px-3 py-1.5 text-xs font-medium ${activeCategory === "All" ? "border-sky-500/50 bg-sky-500/15 text-sky-300" : "border-white/10 text-slate-400"}`}
        >
          All
        </button>
        {categories.map((c) => (
          <button
            key={c}
            onClick={() => setActiveCategory(c)}
            className={`rounded-full border px-3 py-1.5 text-xs font-medium ${activeCategory === c ? "border-sky-500/50 bg-sky-500/15 text-sky-300" : "border-white/10 text-slate-400"}`}
          >
            {c}
          </button>
        ))}
        <input
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Search apps…"
          className="ml-2 w-48 rounded-lg border border-white/10 bg-white/5 px-3 py-1.5 text-sm text-white placeholder:text-slate-500 focus:outline-none"
        />
        <button
          onClick={installSelected}
          disabled={selected.size === 0 || queueRunning}
          className="ml-auto rounded-lg bg-sky-500 px-3 py-1.5 text-sm font-medium text-white hover:bg-sky-400 disabled:opacity-30"
        >
          Install Selected ({selected.size})
        </button>
      </div>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {visible.map((app) => {
          const busy = installing.has(app.id);
          return (
            <div key={app.id} className={`rounded-2xl border p-4 ${app.installed ? "border-emerald-500/20 bg-emerald-500/5" : "border-white/10 bg-white/[0.03]"}`}>
              <div className="flex items-start justify-between gap-2">
                <div className="flex items-center gap-2">
                  {!app.installed && (
                    <input type="checkbox" checked={selected.has(app.id)} onChange={() => toggleSelect(app.id)} disabled={busy} />
                  )}
                  <div>
                    <p className="text-sm font-semibold text-white">{app.name}</p>
                    <p className="text-xs text-slate-500">{app.publisher}</p>
                  </div>
                </div>
                {app.installed ? <Pill tone="green">Installed ✓</Pill> : <Pill tone="blue">{app.source}</Pill>}
              </div>
              <p className="mt-2 text-xs text-slate-500">v{app.version}</p>
              <div className="mt-3">
                {busy ? (
                  <div className="h-1.5 w-full overflow-hidden rounded-full bg-white/10">
                    <div className="h-full w-1/2 animate-pulse rounded-full bg-sky-400" />
                  </div>
                ) : app.installed ? (
                  <button
                    onClick={() => handleUninstall(app)}
                    className="w-full rounded-lg border border-white/10 py-1.5 text-xs text-slate-300 hover:bg-white/5"
                  >
                    Uninstall
                  </button>
                ) : (
                  <button
                    onClick={() => installOne(app, true)}
                    className="w-full rounded-lg border border-sky-500/30 bg-sky-500/10 py-1.5 text-xs font-medium text-sky-300 hover:bg-sky-500/20"
                  >
                    Install
                  </button>
                )}
              </div>
            </div>
          );
        })}
      </div>

      <div className="mt-4">
        <Pagination
          currentPage={page}
          totalPages={totalPages}
          onPageChange={setPage}
          totalItems={totalItems}
          itemsPerPage={perPage}
        />
      </div>

      <div className="mt-6">
        <h3 className="mb-2 text-sm font-semibold text-white">Install Log</h3>
        <LogConsole lines={log} />
      </div>
    </div>
  );
}

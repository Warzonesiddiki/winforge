"use client";

import { useMemo, useState, useTransition } from "react";
import { setServiceMode, setServiceState, setTaskEnabled } from "@/lib/actions";
import { RiskBadge, Pill } from "@/components/ui";
import { useToast } from "@/components/Toast";
import { Pagination, usePagination } from "@/components/Pagination";
import type { RiskLevel } from "@/lib/types";

interface ServiceRow {
  id: string;
  displayName: string;
  description: string;
  category: string;
  startType: string;
  status: string;
  risk: RiskLevel;
  protected: boolean;
  recommended: string;
}

interface TaskRow {
  id: string;
  name: string;
  path: string;
  description: string;
  enabled: boolean;
  risk: RiskLevel;
  category: string;
}

export function ServicesClient({ services, tasks }: { services: ServiceRow[]; tasks: TaskRow[] }) {
  const [tab, setTab] = useState<"services" | "tasks">("services");
  const [search, setSearch] = useState("");
  const [category, setCategory] = useState("All");
  const [pending, startTransition] = useTransition();
  const { addToast } = useToast();

  const serviceCategories = useMemo(() => Array.from(new Set(services.map((s) => s.category))).sort(), [services]);
  const taskCategories = useMemo(() => Array.from(new Set(tasks.map((t) => t.category))).sort(), [tasks]);

  const servicesAll = services.filter((s) => {
    if (category !== "All" && s.category !== category) return false;
    if (search && !`${s.displayName} ${s.description}`.toLowerCase().includes(search.toLowerCase())) return false;
    return true;
  });
  const { items: visibleServices, page: sPage, setPage: setSPage, totalPages: sPages, totalItems: sTotal, perPage: sPerPage } = usePagination(servicesAll, 15);

  const tasksAll = tasks.filter((t) => {
    if (category !== "All" && t.category !== category) return false;
    if (search && !`${t.name} ${t.path} ${t.description}`.toLowerCase().includes(search.toLowerCase())) return false;
    return true;
  });
  const { items: visibleTasks, page: tPage, setPage: setTPage, totalPages: tPages, totalItems: tTotal, perPage: tPerPage } = usePagination(tasksAll, 15);

  function changeService(s: ServiceRow, mode: "automatic" | "manual" | "disabled") {
    startTransition(async () => {
      const r = await setServiceMode(s.id, mode);
      addToast(r.success ? "success" : "error", s.displayName, r.message);
    });
  }

  function changeTask(t: TaskRow) {
    startTransition(async () => {
      const r = await setTaskEnabled(t.id, !t.enabled);
      addToast(r.success ? "success" : "error", t.name, r.message);
    });
  }

  const disabledServices = services.filter((s) => s.startType === "Disabled").length;
  const disabledTasks = tasks.filter((t) => !t.enabled).length;

  return (
    <div className="mt-6">
      <div className="mb-4 flex flex-wrap items-center gap-2 border-b border-white/10 pb-2">
        <button onClick={() => { setTab("services"); setCategory("All"); }} className={`px-3 py-2 text-sm font-medium ${tab === "services" ? "border-b-2 border-sky-400 text-sky-300" : "text-slate-400"}`}>
          Services ({services.length} · {disabledServices} disabled)
        </button>
        <button onClick={() => { setTab("tasks"); setCategory("All"); }} className={`px-3 py-2 text-sm font-medium ${tab === "tasks" ? "border-b-2 border-sky-400 text-sky-300" : "text-slate-400"}`}>
          Scheduled Tasks ({tasks.length} · {disabledTasks} disabled)
        </button>
        <div className="ml-auto flex items-center gap-2">
          <input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search…"
            className="w-44 rounded-lg border border-white/10 bg-white/5 px-3 py-1.5 text-sm text-white placeholder:text-slate-500 focus:outline-none"
          />
          <select value={category} onChange={(e) => setCategory(e.target.value)} className="rounded-lg border border-white/10 bg-white/5 px-2 py-1.5 text-sm text-white">
            <option value="All">All categories</option>
            {(tab === "services" ? serviceCategories : taskCategories).map((c) => (
              <option key={c} value={c}>{c}</option>
            ))}
          </select>
        </div>
      </div>

      {tab === "services" ? (
        <div className="overflow-hidden rounded-2xl border border-white/10">
          <table className="w-full text-sm">
            <thead className="bg-white/5 text-left text-xs uppercase tracking-wide text-slate-400">
              <tr>
                <th className="px-4 py-3">Service</th>
                <th className="px-4 py-3">Category</th>
                <th className="px-4 py-3">State</th>
                <th className="px-4 py-3">Risk</th>
                <th className="px-4 py-3">Recommendation</th>
                <th className="px-4 py-3 text-right">Start Type</th>
              </tr>
            </thead>
            <tbody>
              {visibleServices.map((s) => (
                <tr key={s.id} className={`border-t border-white/5 ${s.protected ? "bg-amber-500/[0.03]" : ""}`}>
                  <td className="px-4 py-3">
                    <p className="font-medium text-white">
                      {s.displayName}
                      {s.protected && <span className="ml-2 text-amber-400">🔒</span>}
                    </p>
                    <p className="max-w-sm truncate text-xs text-slate-500" title={s.description}>{s.description}</p>
                  </td>
                  <td className="px-4 py-3 text-slate-400">{s.category}</td>
                  <td className="px-4 py-3">
                    <Pill tone={s.status === "Running" ? "green" : "neutral"}>{s.status}</Pill>
                  </td>
                  <td className="px-4 py-3"><RiskBadge risk={s.risk} /></td>
                  <td className="px-4 py-3">
                    {s.protected ? (
                      <span className="text-xs text-amber-400">Protected</span>
                    ) : (
                      <Pill tone={s.recommended === "disable" ? "amber" : "blue"}>{s.recommended}</Pill>
                    )}
                  </td>
                  <td className="px-4 py-3 text-right">
                    {s.protected ? (
                      <span className="text-xs text-slate-600">{s.startType} 🔒</span>
                    ) : (
                      <select
                        value={s.startType}
                        onChange={(e) => changeService(s, e.target.value as "automatic" | "manual" | "disabled")}
                        disabled={pending}
                        className="rounded-lg border border-white/10 bg-white/5 px-2 py-1 text-xs text-white disabled:opacity-40"
                      >
                        <option value="Automatic">Automatic</option>
                        <option value="Manual">Manual</option>
                        <option value="Disabled">Disabled</option>
                      </select>
                    )}
                  </td>
                </tr>
              ))}
              {visibleServices.length === 0 && (
                <tr><td colSpan={6} className="px-4 py-6 text-center text-sm text-slate-500">No services match.</td></tr>
              )}
            </tbody>
          </table>
          {sPages > 1 && (
            <div className="border-t border-white/10 px-4 py-3">
              <Pagination currentPage={sPage} totalPages={sPages} onPageChange={setSPage} totalItems={sTotal} itemsPerPage={sPerPage} />
            </div>
          )}
        </div>
      ) : (
        <div className="overflow-hidden rounded-2xl border border-white/10">
          <table className="w-full text-sm">
            <thead className="bg-white/5 text-left text-xs uppercase tracking-wide text-slate-400">
              <tr>
                <th className="px-4 py-3">Task</th>
                <th className="px-4 py-3">Path</th>
                <th className="px-4 py-3">Category</th>
                <th className="px-4 py-3">Risk</th>
                <th className="px-4 py-3 text-right">Enabled</th>
              </tr>
            </thead>
            <tbody>
              {visibleTasks.map((t) => (
                <tr key={t.id} className="border-t border-white/5">
                  <td className="px-4 py-3">
                    <p className="font-medium text-white">{t.name}</p>
                    <p className="max-w-sm truncate text-xs text-slate-500" title={t.description}>{t.description}</p>
                  </td>
                  <td className="max-w-xs truncate px-4 py-3 font-mono text-xs text-slate-500" title={t.path}>{t.path}</td>
                  <td className="px-4 py-3 text-slate-400">{t.category}</td>
                  <td className="px-4 py-3"><RiskBadge risk={t.risk} /></td>
                  <td className="px-4 py-3 text-right">
                    <button
                      onClick={() => changeTask(t)}
                      disabled={pending}
                      className={`rounded-lg border px-3 py-1 text-xs font-medium disabled:opacity-40 ${
                        t.enabled
                          ? "border-emerald-500/30 bg-emerald-500/10 text-emerald-300 hover:bg-emerald-500/20"
                          : "border-white/10 text-slate-400 hover:bg-white/5"
                      }`}
                    >
                      {t.enabled ? "Enabled" : "Disabled"}
                    </button>
                  </td>
                </tr>
              ))}
              {visibleTasks.length === 0 && (
                <tr><td colSpan={5} className="px-4 py-6 text-center text-sm text-slate-500">No tasks match.</td></tr>
              )}
            </tbody>
          </table>
          {tPages > 1 && (
            <div className="border-t border-white/10 px-4 py-3">
              <Pagination currentPage={tPage} totalPages={tPages} onPageChange={setTPage} totalItems={tTotal} itemsPerPage={tPerPage} />
            </div>
          )}
        </div>
      )}
    </div>
  );
}

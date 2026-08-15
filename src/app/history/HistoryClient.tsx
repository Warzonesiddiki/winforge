"use client";

import { useMemo, useState, useTransition } from "react";
import { undoAllToday, undoOperation } from "@/lib/actions";
import { RiskBadge, Pill, ConfirmButton } from "@/components/ui";
import { useToast } from "@/components/Toast";
import { Pagination, usePagination } from "@/components/Pagination";
import { SnapshotsPanel } from "./SnapshotsPanel";
import type { RiskLevel } from "@/lib/types";
import type { SnapshotState } from "@/db/schema";

interface HistoryRow {
  id: string;
  timestamp: string;
  operationType: string;
  category: string;
  target: string;
  previousValue: string | null;
  newValue: string | null;
  risk: RiskLevel;
  success: boolean;
  canUndo: boolean;
  undone: boolean;
}

export function HistoryClient({ rows, snapshots }: { rows: HistoryRow[]; snapshots: { id: string; name: string; createdAt: string; state: SnapshotState }[] }) {
  const categories = useMemo(() => Array.from(new Set(rows.map((r) => r.category))), [rows]);
  const [category, setCategory] = useState("All");
  const [statusFilter, setStatusFilter] = useState<"all" | "success" | "failed">("all");
  const [pending, startTransition] = useTransition();
  const { addToast } = useToast();

  const filteredAll = rows.filter((r) => {
    if (category !== "All" && r.category !== category) return false;
    if (statusFilter === "success" && !r.success) return false;
    if (statusFilter === "failed" && r.success) return false;
    return true;
  });

  const { items: filtered, page, setPage, totalPages, totalItems, perPage } = usePagination(filteredAll, 15);

  function exportCsv() {
    window.open("/api/history/export", "_blank");
    addToast("info", "Export Started", "Downloading CSV export of operation history.");
  }

  async function handleUndo(id: string, target: string) {
    await undoOperation(id);
    addToast("success", "Operation Undone", `Reversed: ${target}`);
  }

  async function handleUndoAll() {
    await undoAllToday();
    addToast("success", "Bulk Undo Complete", "All today's reversible operations have been undone.");
  }

  return (
    <div className="mt-6">
      <div className="mb-4 flex flex-wrap items-center gap-2">
        <select value={category} onChange={(e) => setCategory(e.target.value)} className="rounded-lg border border-white/10 bg-white/5 px-3 py-1.5 text-sm text-white">
          <option value="All">All categories</option>
          {categories.map((c) => (
            <option key={c} value={c}>
              {c}
            </option>
          ))}
        </select>
        <select value={statusFilter} onChange={(e) => setStatusFilter(e.target.value as typeof statusFilter)} className="rounded-lg border border-white/10 bg-white/5 px-3 py-1.5 text-sm text-white">
          <option value="all">All statuses</option>
          <option value="success">Success</option>
          <option value="failed">Failed</option>
        </select>
        <div className="ml-auto flex gap-2">
          <button onClick={exportCsv} className="rounded-lg border border-white/10 bg-white/5 px-3 py-1.5 text-sm font-medium text-slate-200 hover:bg-white/10">
            Export CSV
          </button>
          <ConfirmButton
            label="Undo All Today"
            confirmMessage="Undo every reversible operation logged today? This cannot be re-applied automatically."
            onConfirm={() => handleUndoAll()}
            danger
          />
        </div>
      </div>

      <div className="overflow-hidden rounded-2xl border border-white/10">
        <table className="w-full text-sm">
          <thead className="bg-white/5 text-left text-xs uppercase tracking-wide text-slate-400">
            <tr>
              <th className="px-4 py-3">Timestamp</th>
              <th className="px-4 py-3">Type</th>
              <th className="px-4 py-3">Target</th>
              <th className="px-4 py-3">Change</th>
              <th className="px-4 py-3">Risk</th>
              <th className="px-4 py-3">Status</th>
              <th className="px-4 py-3 text-right">Undo</th>
            </tr>
          </thead>
          <tbody>
            {filtered.map((r) => (
              <tr key={r.id} className="border-t border-white/5">
                <td className="whitespace-nowrap px-4 py-3 text-xs text-slate-400">{new Date(r.timestamp).toLocaleString()}</td>
                <td className="px-4 py-3 text-xs text-slate-300">{r.operationType}</td>
                <td className="max-w-xs truncate px-4 py-3 text-white">{r.target}</td>
                <td className="px-4 py-3 text-xs font-mono text-slate-500">
                  {r.previousValue ?? "—"} → {r.newValue ?? "—"}
                </td>
                <td className="px-4 py-3">
                  <RiskBadge risk={r.risk} />
                </td>
                <td className="px-4 py-3">
                  {r.success ? <Pill tone="green">Success</Pill> : <Pill tone="red">Failed</Pill>}
                  {r.undone && <Pill tone="amber">Undone</Pill>}
                </td>
                <td className="px-4 py-3 text-right">
                  {r.canUndo && !r.undone ? (
                    <button
                      onClick={() => startTransition(() => handleUndo(r.id, r.target))}
                      disabled={pending}
                      className="rounded-lg border border-white/10 px-2.5 py-1 text-xs text-slate-300 hover:bg-white/5"
                    >
                      Undo
                    </button>
                  ) : (
                    <span className="text-xs text-slate-600">—</span>
                  )}
                </td>
              </tr>
            ))}
            {filtered.length === 0 && (
              <tr>
                <td colSpan={7} className="px-4 py-6 text-center text-sm text-slate-500">
                  No operations logged yet.
                </td>
              </tr>
            )}
          </tbody>
        </table>
        {totalPages > 1 && (
          <div className="border-t border-white/10 px-4 py-3">
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

      <SnapshotsPanel snapshots={snapshots} />
    </div>
  );
}

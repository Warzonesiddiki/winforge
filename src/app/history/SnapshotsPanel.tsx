"use client";

import { useState, useTransition } from "react";
import { compareSnapshot, createSnapshot, restoreSnapshot } from "@/lib/actions";
import { useToast } from "@/components/Toast";
import { Modal, ConfirmModal } from "@/components/Modal";
import type { SnapshotState } from "@/db/schema";

interface SnapshotRow {
  id: string;
  name: string;
  createdAt: string;
  state: SnapshotState;
}

interface DiffItem {
  type: "tweak" | "package" | "privacy";
  target: string;
  from: string;
  to: string;
}

export function SnapshotsPanel({ snapshots }: { snapshots: SnapshotRow[] }) {
  const [name, setName] = useState("");
  const [pending, startTransition] = useTransition();
  const [compareOpen, setCompareOpen] = useState<{ snapshot: SnapshotRow; diffs: DiffItem[] } | null>(null);
  const [restoreTarget, setRestoreTarget] = useState<SnapshotRow | null>(null);
  const [comparing, startComparing] = useTransition();
  const { addToast } = useToast();

  function handleCreate() {
    startTransition(async () => {
      const r = await createSnapshot(name.trim() || `Snapshot ${new Date().toLocaleString()}`);
      addToast(r.success ? "success" : "error", "Snapshot", r.message);
      if (r.success) setName("");
    });
  }

  function handleCompare(snap: SnapshotRow) {
    startComparing(async () => {
      const r = await compareSnapshot(snap.id);
      if (r.success) {
        setCompareOpen({ snapshot: snap, diffs: r.diffs ?? [] });
      } else {
        addToast("error", "Compare Failed", r.message);
      }
    });
  }

  function handleRestore() {
    if (!restoreTarget) return;
    startTransition(async () => {
      const r = await restoreSnapshot(restoreTarget.id);
      addToast(r.success ? "success" : "error", "Snapshot Restore", r.message);
      setRestoreTarget(null);
    });
  }

  return (
    <div className="mt-8">
      <div className="mb-4 flex flex-wrap items-center gap-3">
        <h3 className="text-sm font-semibold text-white">System Snapshots</h3>
        <p className="text-xs text-slate-500">Capture the current state, compare, and restore any point in time — the ultimate reversibility guarantee.</p>
        <div className="ml-auto flex gap-2">
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Snapshot name…"
            className="w-56 rounded-lg border border-white/10 bg-white/5 px-3 py-1.5 text-sm text-white placeholder:text-slate-500 focus:outline-none"
          />
          <button
            onClick={handleCreate}
            disabled={pending}
            className="rounded-lg bg-sky-500 px-3 py-1.5 text-sm font-medium text-white hover:bg-sky-400 disabled:opacity-40"
          >
            Capture Snapshot
          </button>
        </div>
      </div>

      {snapshots.length === 0 ? (
        <div className="rounded-2xl border border-dashed border-white/10 p-6 text-center text-sm text-slate-500">
          No snapshots yet. Capture one before making major changes — you&apos;ll be able to restore the exact state later.
        </div>
      ) : (
        <div className="space-y-2">
          {snapshots.map((s) => (
            <div key={s.id} className="flex flex-wrap items-center gap-3 rounded-xl border border-white/10 bg-white/[0.03] px-4 py-3">
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-medium text-white">{s.name}</p>
                <p className="text-xs text-slate-500">{new Date(s.createdAt).toLocaleString()}</p>
              </div>
              <div className="flex gap-2">
                <button
                  onClick={() => handleCompare(s)}
                  disabled={comparing}
                  className="rounded-lg border border-white/10 px-2.5 py-1 text-xs text-slate-300 hover:bg-white/5"
                >
                  Compare
                </button>
                <button
                  onClick={() => setRestoreTarget(s)}
                  className="rounded-lg border border-amber-500/30 bg-amber-500/10 px-2.5 py-1 text-xs text-amber-300 hover:bg-amber-500/20"
                >
                  Restore
                </button>
              </div>
            </div>
          ))}
        </div>
      )}

      <Modal
        open={!!compareOpen}
        onClose={() => setCompareOpen(null)}
        title={compareOpen ? `Compare: ${compareOpen.snapshot.name}` : ""}
        size="lg"
      >
        {compareOpen && (
          <div>
            {compareOpen.diffs.length === 0 ? (
              <p className="py-4 text-center text-sm text-emerald-400">✓ System state matches this snapshot exactly.</p>
            ) : (
              <>
                <p className="mb-3 text-sm text-slate-400">{compareOpen.diffs.length} difference(s) since this snapshot:</p>
                <div className="max-h-72 space-y-1.5 overflow-y-auto">
                  {compareOpen.diffs.map((d, i) => (
                    <div key={i} className="flex items-center justify-between gap-3 rounded-lg bg-black/30 px-3 py-2 text-xs">
                      <span className="min-w-0 flex-1 truncate text-slate-300">
                        <span className="mr-1 font-mono uppercase text-sky-400">{d.type}</span>
                        {d.target}
                      </span>
                      <span className="shrink-0 font-mono text-slate-500">{d.from} → <span className="text-amber-400">{d.to}</span></span>
                    </div>
                  ))}
                </div>
              </>
            )}
          </div>
        )}
      </Modal>

      <ConfirmModal
        open={!!restoreTarget}
        onClose={() => setRestoreTarget(null)}
        onConfirm={handleRestore}
        title="Restore Snapshot"
        message={`Restore "${restoreTarget?.name}"? All changes made since this snapshot (tweaks, packages, privacy rules) will be reverted. Each reversal is logged and individually undoable.`}
        confirmLabel="Restore Snapshot"
        danger
      />
    </div>
  );
}

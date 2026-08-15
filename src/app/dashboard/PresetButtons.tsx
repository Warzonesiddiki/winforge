"use client";

import { useTransition, useState } from "react";
import { applyPreset } from "@/lib/actions";
import { useToast } from "@/components/Toast";
import { ConfirmModal } from "@/components/Modal";

export function PresetButtons({ presets }: { presets: { id: string; name: string; description: string }[] }) {
  const [pending, startTransition] = useTransition();
  const [confirmTarget, setConfirmTarget] = useState<{ id: string; name: string } | null>(null);
  const { addToast } = useToast();

  async function handleApply(presetId: string, presetName: string) {
    startTransition(async () => {
      const result = await applyPreset(presetId);
      if (result.success) {
        addToast("success", "Preset Applied", result.message);
      } else {
        addToast("error", "Failed", result.message);
      }
      setConfirmTarget(null);
    });
  }

  return (
    <>
      <div className="grid grid-cols-2 gap-3">
        {presets.map((p) => (
          <button
            key={p.id}
            disabled={pending}
            onClick={() => setConfirmTarget(p)}
            className="group rounded-xl border border-white/10 bg-white/[0.03] p-4 text-left transition hover:border-sky-500/40 hover:bg-sky-500/5 disabled:opacity-40"
            title={p.description}
          >
            <p className="text-sm font-semibold text-white group-hover:text-sky-300">{p.name}</p>
            <p className="mt-1 line-clamp-2 text-xs text-slate-500">{p.description}</p>
          </button>
        ))}
      </div>

      <ConfirmModal
        open={!!confirmTarget}
        onClose={() => setConfirmTarget(null)}
        onConfirm={() => {
          if (confirmTarget) handleApply(confirmTarget.id, confirmTarget.name);
        }}
        title={`Apply "${confirmTarget?.name}" Preset`}
        message={`This will apply ${confirmTarget?.name} — changing multiple tweaks, removing bloatware packages, and enabling privacy rules. A restore point will be created first. Continue?`}
        confirmLabel="Apply Preset"
        loading={pending}
      />
    </>
  );
}

"use client";

import { useMemo, useState } from "react";
import { hardenAllPrivacy, setPrivacyRule } from "@/lib/actions";
import { RiskBadge, Toggle, ConfirmButton } from "@/components/ui";
import { useToast } from "@/components/Toast";
import type { RiskLevel } from "@/lib/types";

interface RuleRow {
  id: string;
  name: string;
  description: string;
  category: string;
  risk: RiskLevel;
  defaultEnabled: boolean;
  enabled: boolean;
}

export function PrivacyClient({ rules }: { rules: RuleRow[] }) {
  const categories = useMemo(() => Array.from(new Set(rules.map((r) => r.category))), [rules]);
  const [activeCategory, setActiveCategory] = useState("All");
  const visible = rules.filter((r) => activeCategory === "All" || r.category === activeCategory);
  const disabledCount = rules.filter((r) => !r.enabled).length;
  const { addToast } = useToast();

  async function handleToggle(r: RuleRow) {
    await setPrivacyRule(r.id, !r.enabled);
    addToast("success", r.name, `${r.enabled ? "Disabled" : "Enabled"} successfully.`);
  }

  return (
    <div>
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
        <div className="ml-auto flex gap-2">
          <a
            href="/api/audit/report"
            target="_blank"
            rel="noreferrer"
            className="rounded-lg border border-white/10 bg-white/5 px-3 py-1.5 text-sm font-medium text-slate-200 hover:bg-white/10"
          >
            Full Audit Report
          </a>
          <a
            href="/api/privacy/audit"
            target="_blank"
            rel="noreferrer"
            className="rounded-lg border border-white/10 bg-white/5 px-3 py-1.5 text-sm font-medium text-slate-200 hover:bg-white/10"
          >
            Privacy Report
          </a>
          <ConfirmButton
            label={`Harden All (${disabledCount})`}
            confirmMessage={`This will enable ${disabledCount} privacy rule(s) across all categories. A restore point will be created first. Continue?`}
            onConfirm={async () => {
              await hardenAllPrivacy();
              addToast("success", "Privacy Hardened", `${disabledCount} privacy rule(s) enabled.`);
            }}
            disabled={disabledCount === 0}
          />
        </div>
      </div>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        {visible.map((r) => (
          <div key={r.id} className="flex items-start justify-between gap-3 rounded-2xl border border-white/10 bg-white/[0.03] p-4">
            <div>
              <p className="text-sm font-semibold text-white">{r.name}</p>
              <p className="mt-1 text-xs text-slate-400">{r.description}</p>
              <div className="mt-2 flex items-center gap-2">
                <RiskBadge risk={r.risk} />
                <span className="text-[11px] text-slate-500">{r.category}</span>
              </div>
            </div>
            <Toggle checked={r.enabled} onToggle={() => handleToggle(r)} />
          </div>
        ))}
      </div>
    </div>
  );
}

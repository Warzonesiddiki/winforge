"use client";

import { useState, useTransition, type ReactNode } from "react";
import { RISK_CLASSES, RISK_LABEL, type RiskLevel } from "@/lib/types";

export function RiskBadge({ risk }: { risk: RiskLevel }) {
  return (
    <span
      className={`inline-flex items-center rounded-full border px-2.5 py-0.5 text-[11px] font-semibold uppercase tracking-wide ${RISK_CLASSES[risk]}`}
    >
      {RISK_LABEL[risk]}
    </span>
  );
}

export function Pill({ children, tone = "neutral" }: { children: ReactNode; tone?: "neutral" | "green" | "amber" | "red" | "blue" }) {
  const tones: Record<string, string> = {
    neutral: "bg-white/5 text-slate-300 border-white/10",
    green: "bg-emerald-500/15 text-emerald-400 border-emerald-500/30",
    amber: "bg-amber-500/15 text-amber-400 border-amber-500/30",
    red: "bg-red-500/15 text-red-400 border-red-500/30",
    blue: "bg-sky-500/15 text-sky-400 border-sky-500/30",
  };
  return (
    <span className={`inline-flex items-center gap-1 rounded-full border px-2.5 py-0.5 text-xs font-medium ${tones[tone]}`}>
      {children}
    </span>
  );
}

export function Banner({ children, tone = "info" }: { children: ReactNode; tone?: "info" | "warn" }) {
  const cls =
    tone === "warn"
      ? "border-amber-500/30 bg-amber-500/10 text-amber-200"
      : "border-sky-500/30 bg-sky-500/10 text-sky-200";
  return <div className={`rounded-xl border px-4 py-3 text-sm ${cls}`}>{children}</div>;
}

export function StatCard({ label, value, sub, tone = "neutral" }: { label: string; value: string; sub?: string; tone?: "neutral" | "green" | "amber" | "red" }) {
  const toneClass: Record<string, string> = {
    neutral: "text-white",
    green: "text-emerald-400",
    amber: "text-amber-400",
    red: "text-red-400",
  };
  return (
    <div className="rounded-2xl border border-white/10 bg-white/[0.03] p-5">
      <p className="text-xs uppercase tracking-wide text-slate-400">{label}</p>
      <p className={`mt-2 text-2xl font-semibold ${toneClass[tone]}`}>{value}</p>
      {sub && <p className="mt-1 text-xs text-slate-500">{sub}</p>}
    </div>
  );
}

export function Toggle({
  checked,
  onToggle,
  disabled,
  size = "md",
}: {
  checked: boolean;
  onToggle: () => Promise<unknown> | void;
  disabled?: boolean;
  size?: "sm" | "md";
}) {
  const [pending, startTransition] = useTransition();
  const dims = size === "sm" ? "h-5 w-9" : "h-6 w-11";
  const knob = size === "sm" ? "h-3.5 w-3.5" : "h-4.5 w-4.5";
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      disabled={disabled || pending}
      onClick={() => startTransition(async () => { await onToggle(); })}
      className={`relative inline-flex ${dims} shrink-0 items-center rounded-full transition-colors disabled:opacity-40 ${
        checked ? "bg-emerald-500" : "bg-white/15"
      } ${pending ? "animate-pulse" : ""}`}
    >
      <span
        className={`inline-block ${knob} transform rounded-full bg-white shadow transition-transform ${
          checked ? (size === "sm" ? "translate-x-4" : "translate-x-5") : "translate-x-1"
        }`}
      />
    </button>
  );
}

export function ConfirmButton({
  label,
  confirmMessage,
  onConfirm,
  className,
  danger,
  disabled,
}: {
  label: string;
  confirmMessage: string;
  onConfirm: () => Promise<void> | void;
  className?: string;
  danger?: boolean;
  disabled?: boolean;
}) {
  const [pending, startTransition] = useTransition();
  const [open, setOpen] = useState(false);

  return (
    <>
      <button
        type="button"
        disabled={disabled || pending}
        onClick={() => setOpen(true)}
        className={
          className ??
          `rounded-lg border px-3 py-1.5 text-sm font-medium transition disabled:opacity-40 ${
            danger
              ? "border-red-500/40 bg-red-500/10 text-red-300 hover:bg-red-500/20"
              : "border-white/10 bg-white/5 text-slate-200 hover:bg-white/10"
          }`
        }
      >
        {pending ? "Working…" : label}
      </button>
      {open && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4">
          <div className="w-full max-w-sm rounded-2xl border border-white/10 bg-[#0b0f17] p-5 shadow-2xl">
            <p className="text-sm font-semibold text-white">Confirm Action</p>
            <p className="mt-2 text-sm text-slate-300">{confirmMessage}</p>
            <div className="mt-4 flex justify-end gap-2">
              <button
                onClick={() => setOpen(false)}
                className="rounded-lg border border-white/10 px-3 py-1.5 text-sm text-slate-300 hover:bg-white/5"
              >
                Cancel
              </button>
              <button
                onClick={() => {
                  setOpen(false);
                  startTransition(async () => { await onConfirm(); });
                }}
                className={`rounded-lg px-3 py-1.5 text-sm font-medium text-white ${
                  danger ? "bg-red-500 hover:bg-red-400" : "bg-sky-500 hover:bg-sky-400"
                }`}
              >
                Confirm
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}

export function ActionButton({
  label,
  onClick,
  className,
  disabled,
}: {
  label: string;
  onClick: () => Promise<void> | void;
  className?: string;
  disabled?: boolean;
}) {
  const [pending, startTransition] = useTransition();
  return (
    <button
      type="button"
      disabled={disabled || pending}
      onClick={() => startTransition(() => onClick())}
      className={
        className ??
        "rounded-lg border border-white/10 bg-white/5 px-3 py-1.5 text-sm font-medium text-slate-200 transition hover:bg-white/10 disabled:opacity-40"
      }
    >
      {pending ? "Working…" : label}
    </button>
  );
}

export function useAsyncAction() {
  const [message, setMessage] = useState<string | null>(null);
  const [pending, startTransition] = useTransition();
  function run(fn: () => Promise<{ message?: string } | void>) {
    startTransition(async () => {
      const result = await fn();
      if (result && "message" in result && result.message) {
        setMessage(result.message);
      }
    });
  }
  return { message, setMessage, pending, run };
}

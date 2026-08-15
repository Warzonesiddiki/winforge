"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Modal } from "./Modal";

const SHORTCUTS = [
  { keys: ["g", "d"], description: "Go to Dashboard", action: "/dashboard" },
  { keys: ["g", "b"], description: "Go to Debloat", action: "/debloat" },
  { keys: ["g", "t"], description: "Go to Tweaks", action: "/tweaks" },
  { keys: ["g", "p"], description: "Go to Privacy", action: "/privacy" },
  { keys: ["g", "i"], description: "Go to Install", action: "/install" },
  { keys: ["g", "r"], description: "Go to Repair", action: "/repair" },
  { keys: ["g", "v"], description: "Go to Services", action: "/services" },
  { keys: ["g", "u"], description: "Go to Updates", action: "/updates" },
  { keys: ["g", "o"], description: "Go to ISO Builder", action: "/iso" },
  { keys: ["g", "h"], description: "Go to History", action: "/history" },
  { keys: ["g", "s"], description: "Go to Settings", action: "/settings" },
  { keys: ["?"], description: "Show keyboard shortcuts", action: "help" },
  { keys: ["Escape"], description: "Close modal / Cancel", action: "close" },
];

export function KeyboardShortcuts() {
  const router = useRouter();
  const [showHelp, setShowHelp] = useState(false);
  const [pendingKey, setPendingKey] = useState<string | null>(null);

  useEffect(() => {
    let timeout: NodeJS.Timeout;

    const handleKeyDown = (e: KeyboardEvent) => {
      // Ignore if typing in an input
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement || e.target instanceof HTMLSelectElement) {
        return;
      }

      const key = e.key.toLowerCase();

      // Show help
      if (key === "?" || (e.shiftKey && key === "/")) {
        e.preventDefault();
        setShowHelp(true);
        return;
      }

      // Handle "g" prefix shortcuts
      if (pendingKey === "g") {
        const shortcut = SHORTCUTS.find((s) => s.keys[0] === "g" && s.keys[1] === key);
        if (shortcut && typeof shortcut.action === "string" && shortcut.action.startsWith("/")) {
          e.preventDefault();
          router.push(shortcut.action);
        }
        setPendingKey(null);
        return;
      }

      if (key === "g") {
        setPendingKey("g");
        timeout = setTimeout(() => setPendingKey(null), 1500);
        return;
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => {
      window.removeEventListener("keydown", handleKeyDown);
      clearTimeout(timeout);
    };
  }, [pendingKey, router]);

  return (
    <>
      {pendingKey && (
        <div className="fixed bottom-4 left-1/2 z-40 -translate-x-1/2 rounded-lg border border-sky-500/30 bg-sky-500/10 px-3 py-1.5 text-sm text-sky-300">
          Press a key: d=Dashboard, b=Debloat, t=Tweaks, p=Privacy...
        </div>
      )}
      <Modal open={showHelp} onClose={() => setShowHelp(false)} title="Keyboard Shortcuts" size="md">
        <div className="space-y-2">
          {SHORTCUTS.filter((s) => s.action !== "help" && s.action !== "close").map((s) => (
            <div key={s.description} className="flex items-center justify-between py-1">
              <span className="text-sm text-slate-300">{s.description}</span>
              <div className="flex gap-1">
                {s.keys.map((k) => (
                  <kbd key={k} className="rounded border border-white/20 bg-white/10 px-2 py-0.5 font-mono text-xs text-slate-200">
                    {k}
                  </kbd>
                ))}
              </div>
            </div>
          ))}
        </div>
        <p className="mt-4 text-xs text-slate-500">Press ? anytime to show this help</p>
      </Modal>
    </>
  );
}

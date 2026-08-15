"use client";

import { useEffect, useRef } from "react";

export function LogConsole({ lines }: { lines: string[] }) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (ref.current) ref.current.scrollTop = ref.current.scrollHeight;
  }, [lines]);

  function colorFor(line: string) {
    if (/error|fail/i.test(line)) return "text-red-400";
    if (/warn/i.test(line)) return "text-amber-400";
    return "text-slate-300";
  }

  return (
    <div
      ref={ref}
      className="h-64 overflow-y-auto rounded-xl border border-white/10 bg-black/60 p-4 font-mono text-xs leading-relaxed"
    >
      {lines.length === 0 ? (
        <p className="text-slate-600">No output yet…</p>
      ) : (
        lines.map((line, i) => (
          <p key={i} className={colorFor(line)}>
            <span className="text-slate-600">[{String(i + 1).padStart(2, "0")}]</span> {line}
          </p>
        ))
      )}
    </div>
  );
}

"use client";

import { useEffect, useRef, useState, useTransition } from "react";
import { useRouter } from "next/navigation";

interface SearchResult {
  type: "tweak" | "package" | "privacy" | "app" | "page";
  id: string;
  title: string;
  subtitle: string;
  href: string;
}

export function GlobalSearch() {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<SearchResult[]>([]);
  const [selectedIndex, setSelectedIndex] = useState(0);
  const [pending, startTransition] = useTransition();
  const inputRef = useRef<HTMLInputElement>(null);
  const router = useRouter();

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault();
        setOpen(true);
      }
      if (e.key === "Escape") {
        setOpen(false);
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, []);

  useEffect(() => {
    if (open && inputRef.current) {
      inputRef.current.focus();
    }
  }, [open]);

  useEffect(() => {
    if (!query.trim()) {
      return;
    }

    startTransition(async () => {
      const res = await fetch(`/api/search?q=${encodeURIComponent(query)}`);
      const data = await res.json();
      setResults(data.results || []);
      setSelectedIndex(0);
    });
  }, [query]);

  function handleSelect(result: SearchResult) {
    router.push(result.href);
    setOpen(false);
    setQuery("");
  }

  function handleKeyDown(e: React.KeyboardEvent) {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setSelectedIndex((i) => Math.min(i + 1, results.length - 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setSelectedIndex((i) => Math.max(i - 1, 0));
    } else if (e.key === "Enter" && results[selectedIndex]) {
      handleSelect(results[selectedIndex]);
    }
  }

  const typeIcon = {
    tweak: "⚙️",
    package: "📦",
    privacy: "🛡️",
    app: "💿",
    page: "📄",
  };

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center pt-[15vh]">
      <div className="fixed inset-0 bg-black/70 backdrop-blur-sm" onClick={() => setOpen(false)} />
      <div className="relative w-full max-w-xl rounded-2xl border border-white/10 bg-[#0b0f17] shadow-2xl">
        <div className="flex items-center gap-3 border-b border-white/10 px-4 py-3">
          <svg className="h-5 w-5 text-slate-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
          <input
            ref={inputRef}
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Search tweaks, packages, apps, settings..."
            className="flex-1 bg-transparent text-sm text-white placeholder:text-slate-500 focus:outline-none"
          />
          <kbd className="rounded border border-white/20 bg-white/5 px-1.5 py-0.5 text-[10px] text-slate-500">ESC</kbd>
        </div>

        <div className="max-h-[50vh] overflow-y-auto">
          {pending && (
            <div className="px-4 py-8 text-center">
              <div className="mx-auto h-6 w-6 animate-spin rounded-full border-2 border-sky-500 border-t-transparent" />
            </div>
          )}

          {!pending && query && results.length === 0 && (
            <div className="px-4 py-8 text-center text-sm text-slate-500">No results found for &quot;{query}&quot;</div>
          )}

          {!pending && query && results.length > 0 && (
            <div className="py-2">
              {results.map((r, i) => (
                <button
                  key={`${r.type}-${r.id}`}
                  onClick={() => handleSelect(r)}
                  className={`flex w-full items-center gap-3 px-4 py-2.5 text-left ${
                    i === selectedIndex ? "bg-sky-500/10" : "hover:bg-white/5"
                  }`}
                >
                  <span className="text-lg">{typeIcon[r.type]}</span>
                  <div className="flex-1 min-w-0">
                    <p className="truncate text-sm font-medium text-white">{r.title}</p>
                    <p className="truncate text-xs text-slate-500">{r.subtitle}</p>
                  </div>
                  <span className="shrink-0 rounded bg-white/5 px-1.5 py-0.5 text-[10px] uppercase text-slate-500">
                    {r.type}
                  </span>
                </button>
              ))}
            </div>
          )}

          {!query && (
            <div className="space-y-1 p-3">
              <p className="px-2 text-[11px] uppercase tracking-wide text-slate-600">Quick Links</p>
              {[
                { label: "Dashboard", href: "/dashboard" },
                { label: "Apply Standard Preset", href: "/tweaks" },
                { label: "Remove Bloatware", href: "/debloat" },
                { label: "Harden Privacy", href: "/privacy" },
                { label: "Repair System", href: "/repair" },
              ].map((link) => (
                <button
                  key={link.href}
                  onClick={() => { router.push(link.href); setOpen(false); }}
                  className="flex w-full items-center gap-2 rounded-lg px-2 py-1.5 text-sm text-slate-300 hover:bg-white/5"
                >
                  <span className="text-slate-500">→</span>
                  {link.label}
                </button>
              ))}
            </div>
          )}
        </div>

        <div className="flex items-center justify-between border-t border-white/10 px-4 py-2 text-[11px] text-slate-600">
          <span>↑↓ to navigate</span>
          <span>↵ to select</span>
          <span>esc to close</span>
        </div>
      </div>
    </div>
  );
}

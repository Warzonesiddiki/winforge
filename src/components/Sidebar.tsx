"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState } from "react";
import { useLocale } from "@/components/LocaleProvider";

const NAV = [
  { href: "/dashboard", key: "nav.dashboard", icon: "📊" },
  { href: "/debloat", key: "nav.debloat", icon: "🧹" },
  { href: "/tweaks", key: "nav.tweaks", icon: "⚙️" },
  { href: "/privacy", key: "nav.privacy", icon: "🛡️" },
  { href: "/install", key: "nav.install", icon: "📦" },
  { href: "/repair", key: "nav.repair", icon: "🩹" },
  { href: "/services", key: "nav.services", icon: "🛠️" },
  { href: "/updates", key: "nav.updates", icon: "⬆️" },
  { href: "/iso", key: "nav.iso", icon: "💿" },
  { href: "/history", key: "nav.history", icon: "🕘" },
  { href: "/settings", key: "nav.settings", icon: "🔧" },
];

export function Sidebar() {
  const pathname = usePathname();
  const [mobileOpen, setMobileOpen] = useState(false);

  return (
    <>
      {/* Mobile top bar */}
      <div className="sticky top-0 z-30 flex items-center gap-3 border-b border-white/10 bg-[#0b0f17] px-4 py-3 lg:hidden">
        <button
          onClick={() => setMobileOpen(true)}
          className="rounded-lg border border-white/10 p-2 text-slate-300 hover:bg-white/5"
          aria-label="Open menu"
        >
          <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h16" />
          </svg>
        </button>
        <div className="flex items-center gap-2">
          <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-gradient-to-br from-sky-500 to-indigo-600 text-sm font-bold text-white">
            W
          </div>
          <span className="text-sm font-bold text-white">WinForge Elite</span>
        </div>
      </div>

      {/* Desktop sidebar */}
      <nav className="hidden h-full w-64 shrink-0 flex-col border-r border-white/10 bg-[#0b0f17] px-3 py-5 lg:flex">
        <SidebarContent pathname={pathname} />
      </nav>

      {/* Mobile drawer */}
      {mobileOpen && (
        <div className="fixed inset-0 z-50 lg:hidden">
          <div className="fixed inset-0 bg-black/60" onClick={() => setMobileOpen(false)} />
          <nav className="fixed left-0 top-0 flex h-full w-72 flex-col border-r border-white/10 bg-[#0b0f17] px-3 py-5">
            <div className="mb-4 flex items-center justify-between px-2">
              <span className="text-sm font-bold text-white">WinForge Elite</span>
              <button onClick={() => setMobileOpen(false)} className="rounded-lg p-1.5 text-slate-400 hover:bg-white/5">
                <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
            <div onClick={() => setMobileOpen(false)}>
              <SidebarContent pathname={pathname} />
            </div>
          </nav>
        </div>
      )}
    </>
  );
}

function SidebarContent({ pathname }: { pathname: string | null }) {
  const { t } = useLocale();
  return (
    <>
      <div className="mb-4 hidden items-center gap-2 px-2 lg:flex">
        <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-gradient-to-br from-sky-500 to-indigo-600 text-lg font-bold text-white">
          W
        </div>
        <div>
          <p className="text-sm font-bold leading-none text-white">WinForge Elite</p>
          <p className="text-[11px] text-slate-500">Control Center</p>
        </div>
      </div>
      <button
        onClick={() => {
          const event = new KeyboardEvent("keydown", { key: "k", ctrlKey: true });
          window.dispatchEvent(event);
        }}
        className="mb-4 flex w-full items-center gap-2 rounded-lg border border-white/10 bg-white/[0.02] px-3 py-2 text-left text-sm text-slate-500 hover:bg-white/5"
      >
        <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
        </svg>
        <span className="flex-1">Search...</span>
        <kbd className="rounded border border-white/20 bg-white/5 px-1.5 py-0.5 text-[10px]">⌘K</kbd>
      </button>
      <div className="flex flex-1 flex-col gap-1">
        {NAV.map((item) => {
          const active = pathname === item.href || pathname?.startsWith(item.href + "/");
          return (
            <Link
              key={item.href}
              href={item.href}
              className={`flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition ${
                active ? "bg-sky-500/15 text-sky-300" : "text-slate-400 hover:bg-white/5 hover:text-slate-200"
              }`}
            >
              <span className="text-base">{item.icon}</span>
              {t(item.key)}
            </Link>
          );
        })}
      </div>
      <div className="rounded-lg border border-white/10 bg-white/[0.03] p-3 text-[11px] leading-relaxed text-slate-500">
        Running in <span className="font-semibold text-slate-300">Simulation Mode</span> — all system actions are safely
        modeled, logged, and reversible.
      </div>
    </>
  );
}

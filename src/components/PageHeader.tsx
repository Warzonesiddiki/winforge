export function PageHeader({ title, subtitle, badge }: { title: string; subtitle?: string; badge?: string }) {
  return (
    <div>
      <nav className="mb-1 flex items-center gap-1.5 text-xs text-slate-500" aria-label="Breadcrumb">
        <span>WinForge</span>
        <span className="text-slate-600">/</span>
        <span className="font-medium text-sky-400">{title}</span>
      </nav>
      <div className="flex items-center gap-3">
        <h1 className="text-2xl font-bold text-white">{title}</h1>
        {badge && (
          <span className="rounded-full border border-sky-500/30 bg-sky-500/10 px-2.5 py-0.5 text-[11px] font-semibold uppercase tracking-wide text-sky-300">
            {badge}
          </span>
        )}
      </div>
      {subtitle && <p className="mt-1 text-sm text-slate-400">{subtitle}</p>}
    </div>
  );
}

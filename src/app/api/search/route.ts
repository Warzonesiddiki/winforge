import { NextRequest, NextResponse } from "next/server";
import { db } from "@/db";
import { ensureSeeded } from "@/db/seed";
import { tweaks, debloatPackages, privacyRules, applications } from "@/db/schema";
import { ilike, or } from "drizzle-orm";

export const dynamic = "force-dynamic";

interface SearchResult {
  type: "tweak" | "package" | "privacy" | "app" | "page";
  id: string;
  title: string;
  subtitle: string;
  href: string;
}

const PAGES: SearchResult[] = [
  { type: "page", id: "dashboard", title: "Dashboard", subtitle: "System health overview and quick actions", href: "/dashboard" },
  { type: "page", id: "debloat", title: "Debloat", subtitle: "Remove bloatware and manage startup items", href: "/debloat" },
  { type: "page", id: "tweaks", title: "Tweaks", subtitle: "System optimization settings", href: "/tweaks" },
  { type: "page", id: "privacy", title: "Privacy", subtitle: "Privacy hardening controls", href: "/privacy" },
  { type: "page", id: "install", title: "Install", subtitle: "Software installer", href: "/install" },
  { type: "page", id: "repair", title: "Repair", subtitle: "System repair tools", href: "/repair" },
  { type: "page", id: "services", title: "Services & Tasks", subtitle: "Windows services and scheduled tasks", href: "/services" },
  { type: "page", id: "updates", title: "Updates", subtitle: "Windows Update management", href: "/updates" },
  { type: "page", id: "iso", title: "ISO Builder", subtitle: "Custom Windows image builder", href: "/iso" },
  { type: "page", id: "history", title: "History", subtitle: "Operation history and undo", href: "/history" },
  { type: "page", id: "settings", title: "Settings", subtitle: "Application settings", href: "/settings" },
];

export async function GET(request: NextRequest) {
  const q = request.nextUrl.searchParams.get("q")?.toLowerCase().trim();
  if (!q || q.length < 2) {
    return NextResponse.json({ results: [] });
  }

  await ensureSeeded();
  const results: SearchResult[] = [];
  const pattern = `%${q}%`;

  // Search pages first
  for (const page of PAGES) {
    if (page.title.toLowerCase().includes(q) || page.subtitle.toLowerCase().includes(q)) {
      results.push(page);
    }
  }

  // Search tweaks
  const tweakResults = await db
    .select()
    .from(tweaks)
    .where(or(ilike(tweaks.name, pattern), ilike(tweaks.description, pattern), ilike(tweaks.category, pattern)))
    .limit(10);

  for (const t of tweakResults) {
    results.push({
      type: "tweak",
      id: t.id,
      title: t.name,
      subtitle: `${t.category} · ${t.applied ? "Applied" : "Not applied"}`,
      href: `/tweaks?search=${encodeURIComponent(t.name)}`,
    });
  }

  // Search packages
  const packageResults = await db
    .select()
    .from(debloatPackages)
    .where(or(ilike(debloatPackages.displayName, pattern), ilike(debloatPackages.packageName, pattern), ilike(debloatPackages.category, pattern)))
    .limit(10);

  for (const p of packageResults) {
    results.push({
      type: "package",
      id: p.packageName,
      title: p.displayName,
      subtitle: `${p.category} · ${p.status}`,
      href: `/debloat?search=${encodeURIComponent(p.displayName)}`,
    });
  }

  // Search privacy rules
  const privacyResults = await db
    .select()
    .from(privacyRules)
    .where(or(ilike(privacyRules.name, pattern), ilike(privacyRules.description, pattern), ilike(privacyRules.category, pattern)))
    .limit(10);

  for (const r of privacyResults) {
    results.push({
      type: "privacy",
      id: r.id,
      title: r.name,
      subtitle: `${r.category} · ${r.enabled ? "Enabled" : "Disabled"}`,
      href: `/privacy`,
    });
  }

  // Search applications
  const appResults = await db
    .select()
    .from(applications)
    .where(or(ilike(applications.name, pattern), ilike(applications.publisher, pattern), ilike(applications.category, pattern)))
    .limit(10);

  for (const a of appResults) {
    results.push({
      type: "app",
      id: a.id,
      title: a.name,
      subtitle: `${a.publisher} · ${a.installed ? "Installed" : "Available"}`,
      href: `/install?search=${encodeURIComponent(a.name)}`,
    });
  }

  return NextResponse.json({ results: results.slice(0, 20) });
}

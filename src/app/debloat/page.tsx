import { db } from "@/db";
import { ensureSeeded } from "@/db/seed";
import { contextMenuItems, debloatPackages, startupItems } from "@/db/schema";
import { PageHeader } from "@/components/PageHeader";
import { DebloatClient } from "./DebloatClient";

export const dynamic = "force-dynamic";

export default async function DebloatPage({ searchParams }: { searchParams: Promise<{ search?: string }> }) {
  await ensureSeeded();
  const params = await searchParams;
  const initialSearch = params.search ?? "";
  const packages = await db.select().from(debloatPackages).orderBy(debloatPackages.category, debloatPackages.displayName);
  const startup = await db.select().from(startupItems).orderBy(startupItems.name);
  const contextMenus = await db.select().from(contextMenuItems).orderBy(contextMenuItems.category, contextMenuItems.title);

  return (
    <div className="mx-auto max-w-7xl px-4 py-6 sm:px-8 sm:py-8">
      <PageHeader
        title="Debloat"
        badge="debloat.json"
        subtitle="Remove Microsoft bloat, OEM junk, advertising apps, AI/Copilot components, startup clutter, and context menu entries."
      />
      <DebloatClient
        packages={packages}
        startupItems={startup}
        contextMenus={contextMenus}
        initialSearch={initialSearch}
      />
    </div>
  );
}

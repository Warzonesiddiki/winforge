import { db } from "@/db";
import { ensureSeeded } from "@/db/seed";
import { appSettings, communityPacks, restorePoints } from "@/db/schema";
import { desc, eq } from "drizzle-orm";
import { PageHeader } from "@/components/PageHeader";
import { SettingsClient } from "./SettingsClient";

export const dynamic = "force-dynamic";

export default async function SettingsPage() {
  await ensureSeeded();
  const [settings] = await db.select().from(appSettings).where(eq(appSettings.id, 1));
  const points = await db.select().from(restorePoints).orderBy(desc(restorePoints.sequenceNumber)).limit(30);
  const packs = await db.select().from(communityPacks).orderBy(communityPacks.category, communityPacks.name);

  return (
    <div className="mx-auto max-w-5xl px-4 py-6 sm:px-8 sm:py-8">
      <PageHeader
        title="Settings & Community Packs"
        subtitle="Theme, language, safety defaults, restore points, and community optimization profiles (AtlasOS, ReviOS, CTT)."
      />
      <SettingsClient
        settings={settings}
        restorePoints={points.map((p) => ({ id: p.id, sequenceNumber: p.sequenceNumber, description: p.description, createdAt: p.createdAt.toISOString() }))}
        communityPacks={packs.map((p) => ({
          id: p.id,
          name: p.name,
          author: p.author,
          description: p.description,
          version: p.version,
          category: p.category,
          icon: p.icon,
          tweakIds: p.tweakIds,
          debloatPackages: p.debloatPackages,
          privacyRuleIds: p.privacyRuleIds,
          installed: p.installed,
        }))}
      />
    </div>
  );
}

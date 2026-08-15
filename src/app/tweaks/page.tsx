import { db } from "@/db";
import { ensureSeeded } from "@/db/seed";
import { appSettings, presets, tweaks } from "@/db/schema";
import { eq } from "drizzle-orm";
import { PageHeader } from "@/components/PageHeader";
import { TweaksClient } from "./TweaksClient";

export const dynamic = "force-dynamic";

export default async function TweaksPage({ searchParams }: { searchParams: Promise<{ search?: string }> }) {
  await ensureSeeded();
  const params = await searchParams;
  const initialSearch = params.search ?? "";
  const allTweaks = await db.select().from(tweaks).orderBy(tweaks.category, tweaks.name);
  const allPresets = await db.select().from(presets);
  const [settings] = await db.select().from(appSettings).where(eq(appSettings.id, 1));

  return (
    <div className="mx-auto max-w-7xl px-4 py-6 sm:px-8 sm:py-8">
      <PageHeader title="Tweaks" subtitle="30+ granular system tweaks across performance, telemetry, UI, network, services, gaming, power, and Explorer." />
      <TweaksClient
        tweaks={allTweaks}
        presets={allPresets.map((p) => ({ id: p.id, name: p.name }))}
        showExpert={settings?.showExpertTweaks ?? false}
        initialSearch={initialSearch}
      />
    </div>
  );
}

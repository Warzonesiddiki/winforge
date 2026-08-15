import { db } from "@/db";
import { ensureSeeded } from "@/db/seed";
import { privacyRules } from "@/db/schema";
import { PageHeader } from "@/components/PageHeader";
import { HealthGauge } from "@/components/HealthGauge";
import { PrivacyClient } from "./PrivacyClient";

export const dynamic = "force-dynamic";

export default async function PrivacyPage() {
  await ensureSeeded();
  const rules = await db.select().from(privacyRules).orderBy(privacyRules.category, privacyRules.name);
  const score = rules.length === 0 ? 100 : Math.round((rules.filter((r) => r.enabled).length / rules.length) * 100);

  return (
    <div className="mx-auto max-w-7xl px-4 py-6 sm:px-8 sm:py-8">
      <PageHeader title="Privacy Hardening" subtitle="Data collection, app permissions, advertising, Microsoft account, browser, and network privacy controls." />
      <div className="mt-6 grid grid-cols-1 gap-6 lg:grid-cols-[220px_1fr]">
        <div className="rounded-2xl border border-white/10 bg-white/[0.03] p-6">
          <HealthGauge score={score} size={150} label="Privacy Score" />
        </div>
        <PrivacyClient rules={rules} />
      </div>
    </div>
  );
}

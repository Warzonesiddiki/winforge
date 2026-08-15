import { db } from "@/db";
import { ensureSeeded } from "@/db/seed";
import { applications } from "@/db/schema";
import { PageHeader } from "@/components/PageHeader";
import { InstallClient } from "./InstallClient";

export const dynamic = "force-dynamic";

export default async function InstallPage({ searchParams }: { searchParams: Promise<{ search?: string }> }) {
  await ensureSeeded();
  const params = await searchParams;
  const initialSearch = params.search ?? "";
  const apps = await db.select().from(applications).orderBy(applications.category, applications.name);

  return (
    <div className="mx-auto max-w-7xl px-4 py-6 sm:px-8 sm:py-8">
      <PageHeader title="Software Installer" subtitle="60+ curated apps across browsers, dev tools, media, utilities, comms, security, and gaming." />
      <InstallClient apps={apps} initialSearch={initialSearch} />
    </div>
  );
}

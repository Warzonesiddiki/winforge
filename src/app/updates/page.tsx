import { db } from "@/db";
import { ensureSeeded } from "@/db/seed";
import { windowsUpdates } from "@/db/schema";
import { PageHeader } from "@/components/PageHeader";
import { UpdatesClient } from "./UpdatesClient";

export const dynamic = "force-dynamic";

export default async function UpdatesPage() {
  await ensureSeeded();
  const updates = await db.select().from(windowsUpdates).orderBy(windowsUpdates.releaseDate);

  return (
    <div className="mx-auto max-w-7xl px-4 py-6 sm:px-8 sm:py-8">
      <PageHeader title="Windows Updates" subtitle="Search, selectively install, hide, and review update history via Microsoft.Update.Session semantics." />
      <UpdatesClient updates={updates} />
    </div>
  );
}

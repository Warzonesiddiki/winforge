import { db } from "@/db";
import { ensureSeeded } from "@/db/seed";
import { scheduledTasks, services } from "@/db/schema";
import { PageHeader } from "@/components/PageHeader";
import { ServicesClient } from "./ServicesClient";

export const dynamic = "force-dynamic";

export default async function ServicesPage() {
  await ensureSeeded();
  const allServices = await db.select().from(services).orderBy(services.category, services.displayName);
  const tasks = await db.select().from(scheduledTasks).orderBy(scheduledTasks.category, scheduledTasks.name);

  return (
    <div className="mx-auto max-w-7xl px-4 py-6 sm:px-8 sm:py-8">
      <PageHeader
        title="Services & Tasks"
        badge="services.json"
        subtitle="Windows service and scheduled task management with protected-resource locking and full undo support."
      />
      <ServicesClient services={allServices} tasks={tasks} />
    </div>
  );
}

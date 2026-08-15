import { db } from "@/db";
import { ensureSeeded } from "@/db/seed";
import { operationHistory, snapshots } from "@/db/schema";
import { desc } from "drizzle-orm";
import { PageHeader } from "@/components/PageHeader";
import { HistoryClient } from "./HistoryClient";

export const dynamic = "force-dynamic";

export default async function HistoryPage() {
  await ensureSeeded();
  const rows = await db.select().from(operationHistory).orderBy(desc(operationHistory.timestamp)).limit(300);
  const snapshotRows = await db.select().from(snapshots).orderBy(desc(snapshots.createdAt)).limit(20);

  return (
    <div className="mx-auto max-w-7xl px-4 py-6 sm:px-8 sm:py-8">
      <PageHeader title="History & Undo" subtitle="Full audit trail of every logged operation, fully reversible via undo payloads and system snapshots." />
      <HistoryClient
        rows={rows.map((r) => ({
          id: r.id,
          timestamp: r.timestamp.toISOString(),
          operationType: r.operationType,
          category: r.category,
          target: r.target,
          previousValue: r.previousValue,
          newValue: r.newValue,
          risk: r.risk,
          success: r.success,
          canUndo: r.canUndo,
          undone: r.undone,
        }))}
        snapshots={snapshotRows.map((s) => ({
          id: s.id,
          name: s.name,
          createdAt: s.createdAt.toISOString(),
          state: s.state,
        }))}
      />
    </div>
  );
}

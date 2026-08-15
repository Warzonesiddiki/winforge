import { db } from "@/db";
import { ensureSeeded } from "@/db/seed";
import { operationHistory } from "@/db/schema";
import { desc } from "drizzle-orm";

export const dynamic = "force-dynamic";

function csvEscape(value: string) {
  if (value.includes(",") || value.includes('"') || value.includes("\n")) {
    return `"${value.replace(/"/g, '""')}"`;
  }
  return value;
}

export async function GET() {
  await ensureSeeded();
  const rows = await db.select().from(operationHistory).orderBy(desc(operationHistory.timestamp));

  const header = ["Timestamp", "Type", "Category", "Target", "Previous", "New", "Risk", "Success", "Undone"];
  const lines = [header.join(",")];
  for (const r of rows) {
    lines.push(
      [
        r.timestamp.toISOString(),
        r.operationType,
        r.category,
        r.target,
        r.previousValue ?? "",
        r.newValue ?? "",
        r.risk,
        String(r.success),
        String(r.undone),
      ]
        .map((v) => csvEscape(String(v)))
        .join(",")
    );
  }

  return new Response(lines.join("\n"), {
    headers: {
      "Content-Type": "text/csv; charset=utf-8",
      "Content-Disposition": "attachment; filename=winforge-history.csv",
    },
  });
}

import { NextResponse } from "next/server";
import { db } from "@/db";
import { ensureSeeded } from "@/db/seed";
import { healthHistory } from "@/db/schema";
import { asc } from "drizzle-orm";

export const dynamic = "force-dynamic";

/** Returns the persistent health-score trend series for charting. */
export async function GET() {
  await ensureSeeded();
  const rows = await db.select().from(healthHistory).orderBy(asc(healthHistory.timestamp)).limit(168); // last week at hourly resolution
  return NextResponse.json(
    rows.map((r) => ({
      timestamp: r.timestamp.toISOString(),
      score: r.score,
      privacyScore: r.privacyScore,
      bloatCount: r.bloatCount,
      appliedTweaks: r.appliedTweaks,
      pendingUpdates: r.pendingUpdates,
    }))
  );
}

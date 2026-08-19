import { db } from "@/db";
import { ensureSeeded } from "@/db/seed";
import { privacyRules } from "@/db/schema";
import { escapeHtml } from "@/lib/export-safety";

export const dynamic = "force-dynamic";

export async function GET() {
  await ensureSeeded();
  const rules = await db.select().from(privacyRules).orderBy(privacyRules.category, privacyRules.name);
  const score = rules.length === 0 ? 100 : Math.round((rules.filter((r) => r.enabled).length / rules.length) * 100);
  const byCategory = new Map<string, typeof rules>();
  for (const r of rules) {
    byCategory.set(r.category, [...(byCategory.get(r.category) ?? []), r]);
  }

  const rows = Array.from(byCategory.entries())
    .map(
      ([category, items]) => `
      <h2>${escapeHtml(category)}</h2>
      <table>
        <thead><tr><th>Rule</th><th>Description</th><th>Risk</th><th>Status</th></tr></thead>
        <tbody>
          ${items
            .map(
              (r) => `<tr>
                <td>${escapeHtml(r.name)}</td>
                <td>${escapeHtml(r.description)}</td>
                <td>${escapeHtml(r.risk)}</td>
                <td style="color:${r.enabled ? "#10b981" : "#ef4444"}">${r.enabled ? "Hardened" : "Not Applied"}</td>
              </tr>`
            )
            .join("")}
        </tbody>
      </table>`
    )
    .join("\n");

  const html = `<!doctype html>
  <html><head><meta charset="utf-8"><title>WinForge Elite — Privacy Audit Report</title>
  <style>
    body { font-family: Segoe UI, Arial, sans-serif; background:#0b0f17; color:#e2e8f0; padding:32px; }
    h1 { color:#38bdf8; }
    h2 { margin-top:32px; border-bottom:1px solid #334155; padding-bottom:6px; }
    table { width:100%; border-collapse:collapse; margin-top:12px; font-size:14px; }
    th, td { text-align:left; padding:8px 10px; border-bottom:1px solid #1e293b; }
    th { color:#94a3b8; text-transform:uppercase; font-size:11px; }
    .score { font-size:48px; font-weight:bold; color:${score >= 80 ? "#4CAF50" : score >= 60 ? "#FFC107" : score >= 40 ? "#FF9800" : "#F44336"}; }
  </style>
  </head><body>
  <h1>WinForge Elite — Privacy Audit Report</h1>
  <p>Generated ${new Date().toISOString()}</p>
  <p class="score">${score}/100</p>
  ${rows}
  </body></html>`;

  return new Response(html, {
    headers: {
      "Content-Type": "text/html; charset=utf-8",
      "Content-Security-Policy": "default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; frame-ancestors 'none'",
      "X-Content-Type-Options": "nosniff",
    },
  });
}

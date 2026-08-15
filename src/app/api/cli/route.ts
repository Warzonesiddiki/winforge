import { NextRequest, NextResponse } from "next/server";
import { ensureSeeded } from "@/db/seed";
import { db } from "@/db";
import { presets, tweaks } from "@/db/schema";
import {
  applyPreset,
  createManualRestorePoint,
  flushDns,
  installAppsBatch,
  resetNetworkStack,
  resetWindowsUpdate,
  restoreSnapshot,
  runDismRestore,
  runDismScan,
  runFullSystemCheck,
  setTweakApplied,
  undoAllToday,
} from "@/lib/actions";
import { computeHealthReport } from "@/lib/health";
import { eq } from "drizzle-orm";
import { services, scheduledTasks } from "@/db/schema";

export const dynamic = "force-dynamic";

const HELP = `
WinForge Elite — Command Line Interface
=======================================

Usage: GET /api/cli?cmd=<command>[&param=value][&format=json]

Commands:
  help                         Show this help
  status                       Current health score + statistics
  apply-preset&preset=<id>     Apply preset (standard|gaming|privacy|work)
  apply-tweak&id=<id>          Apply a tweak by id
  undo-all                     Undo all today's operations
  restore-snapshot&id=<uuid>   Restore a system snapshot
  repair&tool=<name>           Run repair tool (full|wu|dism-scan|dism-restore|network|dns)
  install&apps=a,b,c           Mark applications installed (winget ids)
  debloat&package=<name>       Remove a bloatware package
  create-restore-point&desc=   Create a restore point
  list-presets                 List all preset ids
  list-tweaks                  List tweak ids (optionally &category=)
  list-services                List all services with start types
  list-tasks                   List all scheduled tasks
  export-report                Full HTML audit report (browser)
  export-history               CSV operation history (browser)

Options:
  format=json                  JSON output instead of plain text

Exit codes are modeled as HTTP status codes (200 = success, 400 = bad args).
`.trim();

interface CmdResult {
  status: number;
  text: string;
  json?: unknown;
}

export async function GET(request: NextRequest) {
  await ensureSeeded();
  const cmd = request.nextUrl.searchParams.get("cmd") ?? "help";
  const format = request.nextUrl.searchParams.get("format") ?? "text";
  const param = (key: string) => request.nextUrl.searchParams.get(key) ?? "";

  let result: CmdResult;

  try {
    switch (cmd) {
      case "help":
        result = { status: 200, text: HELP, json: { commands: ["help", "status", "apply-preset", "apply-tweak", "undo-all", "restore-snapshot", "repair", "install", "debloat", "create-restore-point", "list-presets", "list-tweaks"] } };
        break;

      case "status": {
        const health = await computeHealthReport();
        const allPresets = await db.select().from(presets);
        result = {
          status: 200,
          text: `WinForge Elite Status\nHealth score: ${health.score}/100 (${health.score >= 80 ? "Excellent" : health.score >= 60 ? "Good" : health.score >= 40 ? "Needs Attention" : "Critical"})\nPrivacy score: ${health.privacyScore}/100\nBloatware: ${health.bloatwareCount} installed\nTweaks: ${health.appliedTweaksCount}/${health.totalTweaksCount} applied\nPending updates: ${health.pendingUpdates}\nQuick wins:\n${health.quickWins.map((w) => `  - ${w}`).join("\n")}\nPresets: ${allPresets.map((p) => p.id).join(", ")}`,
          json: { health },
        };
        break;
      }

      case "apply-preset": {
        const presetId = param("preset");
        if (!presetId) {
          result = { status: 400, text: "Missing required param: preset", json: { error: "preset required" } };
          break;
        }
        const presetList = await db.select().from(presets).where(eq(presets.id, presetId));
        if (presetList.length === 0) {
          result = { status: 400, text: `Unknown preset: ${presetId}`, json: { error: `unknown preset ${presetId}` } };
          break;
        }
        const r = await applyPreset(presetId);
        result = { status: r.success ? 200 : 500, text: r.message, json: r };
        break;
      }

      case "apply-tweak": {
        const id = param("id");
        if (!id) {
          result = { status: 400, text: "Missing required param: id", json: { error: "id required" } };
          break;
        }
        const r = await setTweakApplied(id, true);
        result = { status: r.success ? 200 : 400, text: r.message, json: r };
        break;
      }

      case "undo-all": {
        await undoAllToday();
        result = { status: 200, text: "All today's reversible operations undone.", json: { ok: true } };
        break;
      }

      case "restore-snapshot": {
        const id = param("id");
        if (!id) {
          result = { status: 400, text: "Missing required param: id", json: { error: "id required" } };
          break;
        }
        const r = await restoreSnapshot(id);
        result = { status: r.success ? 200 : 400, text: r.message, json: r };
        break;
      }

      case "repair": {
        const tool = param("tool") || "full";
        const tools: Record<string, () => Promise<{ success: boolean; message: string }>> = {
          full: runFullSystemCheck,
          wu: resetWindowsUpdate,
          "dism-scan": runDismScan,
          "dism-restore": runDismRestore,
          network: resetNetworkStack,
          dns: flushDns,
        };
        const fn = tools[tool];
        if (!fn) {
          result = { status: 400, text: `Unknown repair tool: ${tool}. Valid: ${Object.keys(tools).join(", ")}`, json: { error: `unknown tool ${tool}` } };
          break;
        }
        const r = await fn();
        result = { status: r.success ? 200 : 500, text: r.message, json: r };
        break;
      }

      case "install": {
        const apps = param("apps").split(",").map((s) => s.trim()).filter(Boolean);
        if (apps.length === 0) {
          result = { status: 400, text: "Missing required param: apps (comma-separated winget ids)", json: { error: "apps required" } };
          break;
        }
        await installAppsBatch(apps);
        result = { status: 200, text: `Installed ${apps.length} app(s): ${apps.join(", ")}`, json: { installed: apps } };
        break;
      }

      case "debloat": {
        const pkg = param("package");
        if (!pkg) {
          result = { status: 400, text: "Missing required param: package", json: { error: "package required" } };
          break;
        }
        const { setPackageStatus } = await import("@/lib/actions");
        await setPackageStatus(pkg, true, true);
        result = { status: 200, text: `Package ${pkg} removed.`, json: { removed: pkg } };
        break;
      }

      case "create-restore-point": {
        const desc = param("desc") || "CLI restore point";
        await createManualRestorePoint(desc);
        result = { status: 200, text: `Restore point created: ${desc}`, json: { ok: true, description: desc } };
        break;
      }

      case "list-presets": {
        const allPresets = await db.select().from(presets);
        result = {
          status: 200,
          text: allPresets.map((p) => `${p.id}\t${p.name}\t${p.description}`).join("\n"),
          json: allPresets.map((p) => ({ id: p.id, name: p.name })),
        };
        break;
      }

      case "list-tweaks": {
        const category = param("category");
        const all = await db.select().from(tweaks);
        const filtered = category ? all.filter((t) => t.category.toLowerCase() === category.toLowerCase()) : all;
        result = {
          status: 200,
          text: filtered.map((t) => `${t.id}\t${t.applied ? "applied" : "pending"}\t${t.name}`).join("\n"),
          json: filtered.map((t) => ({ id: t.id, applied: t.applied, name: t.name, category: t.category, risk: t.risk })),
        };
        break;
      }

      case "list-services": {
        const allServices = await db.select().from(services);
        result = {
          status: 200,
          text: allServices.map((s) => `${s.id}\t${s.startType}\t${s.status}\t${s.protected ? "protected" : "managed"}\t${s.displayName}`).join("\n"),
          json: allServices.map((s) => ({ id: s.id, displayName: s.displayName, startType: s.startType, status: s.status, protected: s.protected, recommended: s.recommended })),
        };
        break;
      }

      case "list-tasks": {
        const allTasks = await db.select().from(scheduledTasks);
        result = {
          status: 200,
          text: allTasks.map((t) => `${t.id}\t${t.enabled ? "enabled" : "disabled"}\t${t.name}`).join("\n"),
          json: allTasks.map((t) => ({ id: t.id, name: t.name, enabled: t.enabled, category: t.category, risk: t.risk })),
        };
        break;
      }

      case "export-report":
        return NextResponse.redirect(new URL("/api/audit/report", request.url));

      case "export-history":
        return NextResponse.redirect(new URL("/api/history/export", request.url));

      default:
        result = { status: 400, text: `Unknown command: ${cmd}\n\n${HELP}`, json: { error: `unknown command ${cmd}` } };
    }
  } catch (error) {
    const message = error instanceof Error ? error.message : "Unknown error";
    result = { status: 500, text: `Error: ${message}`, json: { error: message } };
  }

  const headers: Record<string, string> = {
    "Content-Type": format === "json" ? "application/json; charset=utf-8" : "text/plain; charset=utf-8",
    "Cache-Control": "no-store",
  };

  const body = format === "json" ? JSON.stringify(result.json ?? { ok: result.status === 200 }, null, 2) : result.text;
  return new NextResponse(body, { status: result.status, headers });
}

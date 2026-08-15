"use client";

import { useState, useTransition } from "react";
import { hideUpdate, installAllUpdates, installUpdate, pauseUpdates, resumeUpdates, unhideUpdate } from "@/lib/actions";
import { Pill, Toggle } from "@/components/ui";
import { useToast } from "@/components/Toast";

interface UpdateRow {
  id: string;
  title: string;
  kb: string;
  sizeMb: number;
  severity: string;
  releaseDate: string;
  installed: boolean;
  hidden: boolean;
}

export function UpdatesClient({ updates }: { updates: UpdateRow[] }) {
  const [tab, setTab] = useState<"available" | "history" | "hidden">("available");
  const [excludeDrivers, setExcludeDrivers] = useState(true);
  const [targetVersion, setTargetVersion] = useState("24H2");
  const [pending, startTransition] = useTransition();
  const { addToast } = useToast();

  const available = updates.filter((u) => !u.installed && !u.hidden);
  const history = updates.filter((u) => u.installed);
  const hidden = updates.filter((u) => u.hidden);
  const list = tab === "available" ? available : tab === "history" ? history : hidden;

  function toggleDriverExclusion() {
    setExcludeDrivers((prev) => {
      const next = !prev;
      addToast(
        "info",
        "Driver Policy Updated",
        next
          ? "Windows Update will now exclude driver updates (ExcludeWUDriversInQualityUpdate=1)."
          : "Driver updates included in quality updates."
      );
      return next;
    });
  }

  function handleVersionLock(ver: string) {
    setTargetVersion(ver);
    addToast("info", "Target Version Locked", `TargetReleaseVersionInfo locked to Windows 11 ${ver}.`);
  }

  return (
    <div className="mt-6 space-y-4">
      {/* Update Policy Toolbar */}
      <div className="grid grid-cols-1 gap-4 rounded-2xl border border-white/10 bg-white/[0.03] p-4 md:grid-cols-2">
        <div className="flex items-center justify-between gap-3">
          <div>
            <p className="text-sm font-medium text-white">Exclude Driver Updates in Quality Updates</p>
            <p className="text-xs text-slate-500">Prevents Windows from overwriting GPU / audio vendor drivers</p>
          </div>
          <Toggle checked={excludeDrivers} onToggle={toggleDriverExclusion} />
        </div>
        <div className="flex items-center justify-between gap-3">
          <div>
            <p className="text-sm font-medium text-white">Target Feature Release Version</p>
            <p className="text-xs text-slate-500">Locks Windows update version milestone</p>
          </div>
          <select
            value={targetVersion}
            onChange={(e) => handleVersionLock(e.target.value)}
            className="rounded-lg border border-white/10 bg-white/5 px-2.5 py-1.5 text-xs text-white focus:outline-none"
          >
            <option value="24H2">Windows 11 24H2</option>
            <option value="23H2">Windows 11 23H2</option>
            <option value="22H2">Windows 11 22H2</option>
            <option value="none">No Version Lock (Latest)</option>
          </select>
        </div>
      </div>

      {/* Tabs & Actions */}
      <div className="flex flex-wrap items-center gap-2 border-b border-white/10 pb-2">
        <button
          onClick={() => setTab("available")}
          className={`px-3 py-2 text-sm font-medium transition ${
            tab === "available" ? "border-b-2 border-sky-400 text-sky-300" : "text-slate-400 hover:text-slate-200"
          }`}
        >
          Available Updates ({available.length})
        </button>
        <button
          onClick={() => setTab("history")}
          className={`px-3 py-2 text-sm font-medium transition ${
            tab === "history" ? "border-b-2 border-sky-400 text-sky-300" : "text-slate-400 hover:text-slate-200"
          }`}
        >
          Update History ({history.length})
        </button>
        <button
          onClick={() => setTab("hidden")}
          className={`px-3 py-2 text-sm font-medium transition ${
            tab === "hidden" ? "border-b-2 border-sky-400 text-sky-300" : "text-slate-400 hover:text-slate-200"
          }`}
        >
          Hidden Updates ({hidden.length})
        </button>
        <div className="ml-auto flex gap-2">
          <button
            onClick={() =>
              startTransition(async () => {
                await pauseUpdates(7);
                addToast("info", "Updates Paused", "Windows Update paused for 7 days via Registry policy.");
              })
            }
            disabled={pending}
            className="rounded-lg border border-white/10 bg-white/5 px-3 py-1.5 text-xs font-medium text-slate-200 hover:bg-white/10"
          >
            Pause 7 Days
          </button>
          <button
            onClick={() =>
              startTransition(async () => {
                await resumeUpdates();
                addToast("info", "Updates Resumed", "Windows Update service resumed.");
              })
            }
            disabled={pending}
            className="rounded-lg border border-white/10 bg-white/5 px-3 py-1.5 text-xs font-medium text-slate-200 hover:bg-white/10"
          >
            Resume
          </button>
          {tab === "available" && available.length > 0 && (
            <button
              onClick={() =>
                startTransition(async () => {
                  await installAllUpdates();
                  addToast("success", "Updates Installed", "All available updates installed successfully.");
                })
              }
              disabled={pending}
              className="rounded-lg bg-sky-500 px-3.5 py-1.5 text-xs font-semibold text-white hover:bg-sky-400"
            >
              Install All ({available.length})
            </button>
          )}
        </div>
      </div>

      <div className="overflow-hidden rounded-2xl border border-white/10">
        <table className="w-full text-sm">
          <thead className="bg-white/5 text-left text-xs uppercase tracking-wide text-slate-400">
            <tr>
              <th className="px-4 py-3">Title</th>
              <th className="px-4 py-3">KB Identifier</th>
              <th className="px-4 py-3">Size</th>
              <th className="px-4 py-3">Severity</th>
              <th className="px-4 py-3">Release Date</th>
              <th className="px-4 py-3 text-right">Action</th>
            </tr>
          </thead>
          <tbody>
            {list.map((u) => (
              <tr key={u.id} className="border-t border-white/5 hover:bg-white/[0.02]">
                <td className="px-4 py-3 text-white font-medium">{u.title}</td>
                <td className="px-4 py-3 font-mono text-xs text-slate-400">{u.kb}</td>
                <td className="px-4 py-3 text-slate-400">{u.sizeMb} MB</td>
                <td className="px-4 py-3">
                  <Pill tone={u.severity === "Critical" ? "red" : u.severity === "Important" ? "amber" : "blue"}>{u.severity}</Pill>
                </td>
                <td className="px-4 py-3 text-slate-400">{u.releaseDate}</td>
                <td className="px-4 py-3 text-right">
                  {tab === "available" && (
                    <div className="flex justify-end gap-2">
                      <button
                        onClick={() =>
                          startTransition(async () => {
                            await installUpdate(u.id);
                            addToast("success", "Update Installed", `${u.kb} installed successfully.`);
                          })
                        }
                        className="rounded-lg border border-sky-500/30 bg-sky-500/10 px-2.5 py-1 text-xs text-sky-300 hover:bg-sky-500/20"
                      >
                        Install
                      </button>
                      <button
                        onClick={() =>
                          startTransition(async () => {
                            await hideUpdate(u.id);
                            addToast("info", "Update Hidden", `${u.kb} hidden from available updates.`);
                          })
                        }
                        className="rounded-lg border border-white/10 px-2.5 py-1 text-xs text-slate-300 hover:bg-white/5"
                      >
                        Hide
                      </button>
                    </div>
                  )}
                  {tab === "hidden" && (
                    <button
                      onClick={() =>
                        startTransition(async () => {
                          await unhideUpdate(u.id);
                          addToast("info", "Update Unhidden", `${u.kb} is now visible.`);
                        })
                      }
                      className="rounded-lg border border-white/10 px-2.5 py-1 text-xs text-slate-300 hover:bg-white/5"
                    >
                      Unhide
                    </button>
                  )}
                  {tab === "history" && <span className="text-xs text-emerald-400">Installed ✓</span>}
                </td>
              </tr>
            ))}
            {list.length === 0 && (
              <tr>
                <td colSpan={6} className="px-4 py-8 text-center text-sm text-slate-500">
                  No updates in this view.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}

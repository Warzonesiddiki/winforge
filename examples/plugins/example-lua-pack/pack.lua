-- Example WinForge Lua pack.
--
-- A pack runs once at startup (during plugin discovery, non-elevated) and
-- PROPOSES tweaks. It never touches the machine directly: every call below
-- builds a config.Operation that is validated through the exact same strict
-- loader as tweaks.json and executed later by the orchestrator, with audit
-- logging and undo.
--
-- Whitelisted API:
--   local t = winforge.tweak{ id="...", name="...", category="...",
--                             description="...", risk="low|medium|high",
--                             reversible=true|false }
--   local op = winforge.registry.set("HKCU", "Key\\Sub", "ValueName",
--                                    "dword"|"qword"|"string", value)
--   local op = winforge.registry.delete("HKCU", "Key\\Sub", "ValueName")
--   local op = winforge.service.set_start_mode("ServiceName",
--                                              "auto"|"manual"|"disabled")
--   winforge.revert(op)          -- add a proposed op to the tweak's revert list
--   winforge.log("message")      -- bounded diagnostic line (also available as print)
--   t:commit()                   -- finalize the tweak (one open tweak at a time)
--
-- Everything else (os, io, debug, package, loadfile, require) is removed.
-- A runaway loop is aborted by an instruction-count hook.
-- Only registry (dword/qword/string set, value delete) and service start-mode
-- operations are permitted. Commands, Appx removal, and scheduled-task changes
-- are reserved for the curated catalog and cannot be proposed by a script.

local t = winforge.tweak{
  id          = "example-lua-tweak",
  name        = "Example Lua Tweak",
  category    = "Customization",
  description = "Sets a marker value written by the example Lua pack.",
  risk        = "low",
  reversible  = true,
}

local apply = winforge.registry.set(
  "HKCU", "Software\\WinForge\\Example", "LuaMarker", "dword", 1
)
local revert = winforge.registry.delete(
  "HKCU", "Software\\WinForge\\Example", "LuaMarker"
)
winforge.revert(revert)

t:commit()

winforge.log("example pack proposed 1 tweak")

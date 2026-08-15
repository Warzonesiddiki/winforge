const std = @import("std");

// WinForge native core — Zig.
// The same source builds three artifacts (see build.sh):
//   - Linux  .so  → tested locally through the Bun FFI bridge (bun:ffi)
//   - Windows .dll → embedded in / loaded by the Bun Windows EXE
//   - Windows .exe → standalone core CLI for power users
// All exported functions use the C ABI so any host (Bun, Node, C, PowerShell
// via P/Invoke) can call the core.

/// Writes the core version string into `buf` (NUL-terminated).
/// Returns the written length (excluding NUL), or 0 when `len` is too small.
export fn winforge_core_version(buf: [*]u8, len: usize) usize {
    const version = "winforge-core 0.2.0 (zig core)";
    if (len < version.len + 1) return 0;
    @memcpy(buf[0..version.len], version);
    buf[version.len] = 0;
    return version.len;
}

/// Health score formula (same algorithm as the web app's src/lib/health.ts and
/// the C# HealthService): baseline 50 + tweak bonus (max 20) + bloat bonus
/// (max 15) + privacy bonus (max 15) − telemetry penalty, clamped to [0, 100].
/// Pure function → fully testable on Linux.
export fn winforge_health_score(
    applied_tweaks: i32,
    removed_bloat: i32,
    privacy_percent: f64,
    telemetry_enabled: bool,
) i32 {
    const bonus_tweaks: i32 = @min(20, applied_tweaks * 2);
    const bonus_bloat: i32 = @min(15, removed_bloat);
    const bonus_privacy: f64 = @round(privacy_percent * 0.15);

    var score: f64 = 50.0
        + @as(f64, @floatFromInt(bonus_tweaks))
        + @as(f64, @floatFromInt(bonus_bloat))
        + bonus_privacy;
    if (telemetry_enabled) score -= 5.0;

    const clamped = @max(0.0, @min(100.0, score));
    return @intFromFloat(@round(clamped));
}

pub fn main() !void {
    std.debug.print("WinForge core {s}\n", .{"0.2.0"});
}

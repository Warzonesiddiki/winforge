using System.Runtime.InteropServices;
using WinForge.Elite.Models;

namespace WinForge.Elite.Services
{
    /// <summary>
    /// Samples live system telemetry from native Windows APIs (no external
    /// dependencies): CPU load via GetSystemTimes, memory via GlobalMemoryStatusEx,
    /// system drive capacity via DriveInfo, and uptime via Environment.TickCount64.
    /// </summary>
    public sealed class SystemInfoService
    {
        [StructLayout(LayoutKind.Sequential)]
        private struct FileTime
        {
            public uint Low;
            public uint High;

            public ulong ToUInt64() => ((ulong)High << 32) | Low;
        }

        [StructLayout(LayoutKind.Sequential)]
        private struct MemoryStatusEx
        {
            public uint Length;
            public uint MemoryLoad;
            public ulong TotalPhys;
            public ulong AvailPhys;
            public ulong TotalPageFile;
            public ulong AvailPageFile;
            public ulong TotalVirtual;
            public ulong AvailVirtual;
            public ulong AvailExtendedVirtual;
        }

        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern bool GetSystemTimes(out FileTime idleTime, out FileTime kernelTime, out FileTime userTime);

        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern bool GlobalMemoryStatusEx(ref MemoryStatusEx status);

        private const double BytesPerGb = 1024.0 * 1024.0 * 1024.0;

        private readonly object _gate = new();
        private ulong _lastIdle;
        private ulong _lastKernel;
        private ulong _lastUser;
        private bool _hasSample;

        /// <summary>
        /// Samples the current system state. CPU percentage is computed as the delta
        /// between consecutive samples, so the first call always reports 0%.
        /// Thread-safe; never throws — unavailable readings degrade to zero values.
        /// </summary>
        public SystemTelemetry Sample()
        {
            lock (_gate)
            {
                var cpuPercent = 0.0;
                if (GetSystemTimes(out var idle, out var kernel, out var user))
                {
                    var idleNow = idle.ToUInt64();
                    var kernelNow = kernel.ToUInt64();
                    var userNow = user.ToUInt64();
                    if (_hasSample)
                    {
                        var idleDelta = idleNow - _lastIdle;
                        var totalDelta = (kernelNow + userNow) - (_lastKernel + _lastUser);
                        if (totalDelta > 0)
                        {
                            cpuPercent = Math.Clamp(100.0 * (1.0 - (double)idleDelta / totalDelta), 0.0, 100.0);
                        }
                    }

                    _lastIdle = idleNow;
                    _lastKernel = kernelNow;
                    _lastUser = userNow;
                    _hasSample = true;
                }

                var memory = new MemoryStatusEx { Length = (uint)Marshal.SizeOf<MemoryStatusEx>() };
                var totalRamGb = 0.0;
                var availableRamGb = 0.0;
                var usedRamGb = 0.0;
                if (GlobalMemoryStatusEx(ref memory))
                {
                    totalRamGb = memory.TotalPhys / BytesPerGb;
                    availableRamGb = memory.AvailPhys / BytesPerGb;
                    usedRamGb = (memory.TotalPhys - memory.AvailPhys) / BytesPerGb;
                }

                var systemDrive = Path.GetPathRoot(Environment.SystemDirectory) ?? @"C:\";
                var systemDriveFreeGb = 0.0;
                var systemDriveTotalGb = 0.0;
                try
                {
                    var drive = new DriveInfo(systemDrive);
                    systemDriveFreeGb = drive.AvailableFreeSpace / BytesPerGb;
                    systemDriveTotalGb = drive.TotalSize / BytesPerGb;
                }
                catch (Exception ex)
                {
                    Logging.Logger.GetLogger<SystemInfoService>()
                        .Debug(ex, "Failed to read system drive information for {Drive}", systemDrive);
                }

                var uptime = TimeSpan.FromMilliseconds(Environment.TickCount64);
                return new SystemTelemetry(
                    Environment.MachineName,
                    Environment.OSVersion.VersionString,
                    $"{uptime.Days}d {uptime.Hours}h {uptime.Minutes}m",
                    cpuPercent,
                    usedRamGb,
                    totalRamGb,
                    availableRamGb,
                    systemDrive,
                    systemDriveFreeGb,
                    systemDriveTotalGb);
            }
        }
    }
}

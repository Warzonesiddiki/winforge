using Dapper;
using System.ComponentModel;
using System.Runtime.InteropServices;
using WinForge.Elite.Data;
using WinForge.Elite.Helpers;
using WinForge.Elite.Models;

namespace WinForge.Elite.Services
{
    /// <summary>
    /// Creates Windows System Restore points via the native srclient.dll
    /// SRSetRestorePointW API (BEGIN_SYSTEM_CHANGE / END_SYSTEM_CHANGE protocol).
    /// Every successfully created restore point is also recorded in the local
    /// database so the UI can list and account for it.
    /// </summary>
    public sealed class RestorePointService
    {
        private static readonly Serilog.ILogger Log = Logging.Logger.GetLogger<RestorePointService>();

        private const int BeginSystemChange = 100;
        private const int EndSystemChange = 101;
        private const int ModifySettings = 12; // A change to system settings was made.
        private const uint SuccessStatus = 0;

        [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
        private struct RestorePointInfo
        {
            public int EventType;
            public int RestorePointType;
            public long SequenceNumber;
            [MarshalAs(UnmanagedType.ByValTStr, SizeConst = 256)]
            public string Description;
        }

        [StructLayout(LayoutKind.Sequential)]
        private struct StateManagerStatus
        {
            public uint Status;
            public long SequenceNumber;
        }

        [DllImport("srclient.dll", SetLastError = true)]
        private static extern bool SRSetRestorePointW(ref RestorePointInfo info, ref StateManagerStatus status);

        /// <summary>
        /// Creates a system restore point. Failure (for example when restore points are
        /// disabled by group policy) is reported in the result rather than thrown, so
        /// callers can decide whether to warn and continue.
        /// </summary>
        public Task<RestorePointResult> CreateAsync(string description, CancellationToken ct = default)
        {
            return Task.Run(() =>
            {
                var info = new RestorePointInfo
                {
                    EventType = BeginSystemChange,
                    RestorePointType = ModifySettings,
                    SequenceNumber = 0,
                    Description = Truncate(description, 250)
                };
                var status = new StateManagerStatus();

                if (!SRSetRestorePointW(ref info, ref status) || status.Status != SuccessStatus)
                {
                    var win32 = new Win32Exception(Marshal.GetLastWin32Error()).Message;
                    var message = $"System restore point creation failed (status {status.Status}): {win32}";
                    Log.Warning(message);
                    return new RestorePointResult(false, 0, message);
                }

                var sequence = status.SequenceNumber;

                // Close the system-change transaction opened above.
                var end = info;
                end.EventType = EndSystemChange;
                end.SequenceNumber = sequence;
                var endStatus = new StateManagerStatus();
                if (!SRSetRestorePointW(ref end, ref endStatus))
                {
                    Log.Warning("Failed to end restore point transaction {Sequence}: {Error}",
                        sequence, new Win32Exception(Marshal.GetLastWin32Error()).Message);
                }

                int? restorePointId = null;
                try
                {
                    using var connection = DbConnectionFactory.CreateConnection();
                    connection.Open();
                    var now = DateTime.UtcNow.ToString("o");
                    connection.Execute(
                        @"INSERT INTO RestorePoints (Name, Description, CreatedAt, SnapshotPath, IsValid, DiskSpaceUsed)
                          VALUES (@Name, @Description, @CreatedAt, @SnapshotPath, 1, 0)",
                        new
                        {
                            Name = description,
                            Description = "WinForge Elite restore point",
                            CreatedAt = now,
                            SnapshotPath = PathHelper.GetBackupPath($"restore_{sequence}")
                        });
                    restorePointId = connection.ExecuteScalar<int>("SELECT last_insert_rowid();");
                }
                catch (Exception ex)
                {
                    Log.Warning(ex, "Restore point created but failed to record it in the local database");
                }

                Log.Information("Restore point created: {Description} (sequence {Sequence})", description, sequence);
                return new RestorePointResult(true, sequence, $"Restore point created (sequence {sequence})", restorePointId);
            }, ct);
        }

        private static string Truncate(string value, int maxLength)
        {
            return value.Length <= maxLength ? value : value.Substring(0, maxLength);
        }
    }
}

namespace WinForge.Elite.Models
{
    /// <summary>Outcome of a mutating operation (tweak, preset, debloat, privacy rule).</summary>
    public sealed record OperationResult(bool Success, string Message, int? RestorePointId = null, string? SnapshotPath = null);

    /// <summary>Outcome of attempting to create a Windows System Restore point.</summary>
    public sealed record RestorePointResult(bool Success, long SequenceNumber, string Message, int? RestorePointId = null);

    /// <summary>Live system telemetry sampled from real Windows APIs.</summary>
    public sealed record SystemTelemetry(
        string MachineName,
        string OsVersion,
        string UptimeText,
        double CpuPercent,
        double UsedRamGb,
        double TotalRamGb,
        double AvailableRamGb,
        string SystemDrive,
        double SystemDriveFreeGb,
        double SystemDriveTotalGb);

    /// <summary>Overall + category health scores computed from live system state.</summary>
    public sealed record HealthSnapshot(
        int OverallScore,
        int SecurityScore,
        int PerformanceScore,
        int CleanlinessScore,
        int PrivacyScore,
        int CriticalIssues,
        int WarningIssues,
        int InfoIssues,
        double PrivacyPercent);
}

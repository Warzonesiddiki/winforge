using Dapper;
using WinForge.Elite.Data;
using WinForge.Elite.Models;

namespace WinForge.Elite.Services
{
    /// <summary>
    /// Computes the WinForge Elite health score (0-100) from live system state:
    /// the local catalog database plus real registry telemetry settings.
    ///
    /// Algorithm:
    ///   Start at 50 (neutral baseline)
    ///   + min(20, appliedTweaks * 2)                      (up to +20)
    ///   + min(15, removedBloatPackages)                   (up to +15)
    ///   + round(privacyPercent * 0.15)                    (up to +15)
    ///   + 5 when every default-enabled tweak is applied
    ///   - 5 when Windows telemetry is still enabled
    ///   - 5 when more than 15 bloat packages remain, -3 when more than 10
    ///   Clamp to [0, 100].
    ///
    /// Category sub-scores (Security/Performance/Cleanliness/Privacy) are computed
    /// from the same inputs and stored alongside the overall score in HealthHistory.
    /// </summary>
    public sealed class HealthService
    {
        private static readonly Serilog.ILogger Log = Logging.Logger.GetLogger<HealthService>();

        private readonly RegistryService _registry;

        public HealthService(RegistryService registry)
        {
            _registry = registry ?? throw new ArgumentNullException(nameof(registry));
        }

        public Task<HealthSnapshot> EvaluateAsync(CancellationToken ct = default)
        {
            return Task.Run(() =>
            {
                ct.ThrowIfCancellationRequested();
                using var connection = DbConnectionFactory.CreateConnection();
                connection.Open();

                long Count(string sql) => connection.ExecuteScalar<long>(sql);

                var appliedTweaks = (int)Count("SELECT COUNT(*) FROM Tweaks WHERE Applied = 1");
                var defaultTweaks = (int)Count("SELECT COUNT(*) FROM Tweaks WHERE DefaultEnabled = 1");
                var appliedDefaultTweaks = (int)Count("SELECT COUNT(*) FROM Tweaks WHERE DefaultEnabled = 1 AND Applied = 1");
                var perfTweaks = (int)Count("SELECT COUNT(*) FROM Tweaks WHERE Category = 'Performance'");
                var appliedPerfTweaks = (int)Count("SELECT COUNT(*) FROM Tweaks WHERE Category = 'Performance' AND Applied = 1");

                var removedBloat = (int)Count("SELECT COUNT(*) FROM DebloatPackages WHERE Status = 1");
                var installedBloat = (int)Count("SELECT COUNT(*) FROM DebloatPackages WHERE Status = 0 AND Category != 'Protected'");

                var privacyTotal = (int)Count("SELECT COUNT(*) FROM PrivacyRules");
                var privacyEnabled = (int)Count("SELECT COUNT(*) FROM PrivacyRules WHERE Enabled = 1");
                var highRiskRules = (int)Count("SELECT COUNT(*) FROM PrivacyRules WHERE Risk = 2");
                var enabledHighRiskRules = (int)Count("SELECT COUNT(*) FROM PrivacyRules WHERE Risk = 2 AND Enabled = 1");
                var mediumRiskRules = (int)Count("SELECT COUNT(*) FROM PrivacyRules WHERE Risk = 1");
                var enabledMediumRiskRules = (int)Count("SELECT COUNT(*) FROM PrivacyRules WHERE Risk = 1 AND Enabled = 1");

                var telemetryEnabled = IsTelemetryEnabled();
                var privacyPercent = privacyTotal > 0 ? 100.0 * privacyEnabled / privacyTotal : 0.0;

                var score = 50.0
                    + Math.Min(20, appliedTweaks * 2)
                    + Math.Min(15, removedBloat)
                    + Math.Round(privacyPercent * 0.15)
                    + (defaultTweaks > 0 && appliedDefaultTweaks == defaultTweaks ? 5 : 0)
                    - (telemetryEnabled ? 5 : 0)
                    - (installedBloat > 15 ? 5 : installedBloat > 10 ? 3 : 0);

                var overall = ClampToScore(score);

                // Sub-scores: honest, documented derivations of the same signals.
                var security = ClampToScore(50 + Math.Round(privacyPercent * 0.5) - (telemetryEnabled ? 25 : 0));
                var performance = ClampToScore(
                    Math.Round(perfTweaks > 0 ? appliedPerfTweaks * 70.0 / perfTweaks : 0)
                    + Math.Min(30, removedBloat * 2));
                var cleanliness = ClampToScore(removedBloat + installedBloat > 0
                    ? Math.Round(removedBloat * 100.0 / (removedBloat + installedBloat))
                    : 0);
                var privacy = ClampToScore(privacyPercent);

                var criticalIssues = highRiskRules - enabledHighRiskRules;
                var warningIssues = (telemetryEnabled ? 1 : 0) + (mediumRiskRules - enabledMediumRiskRules);
                var infoIssues = (int)Count("SELECT COUNT(*) FROM Tweaks WHERE Applied = 0");

                var snapshot = new HealthSnapshot(
                    overall,
                    security,
                    performance,
                    cleanliness,
                    privacy,
                    criticalIssues,
                    warningIssues,
                    infoIssues,
                    Math.Round(privacyPercent, 1));

                try
                {
                    var now = DateTime.UtcNow.ToString("o");
                    connection.Execute(
                        @"INSERT INTO HealthHistory
                              (OverallScore, SecurityScore, PerformanceScore, CleanlinessScore, PrivacyScore,
                               CriticalIssues, WarningIssues, InfoIssues, RecordedAt)
                          VALUES
                              (@OverallScore, @SecurityScore, @PerformanceScore, @CleanlinessScore, @PrivacyScore,
                               @CriticalIssues, @WarningIssues, @InfoIssues, @RecordedAt)",
                        new
                        {
                            OverallScore = overall,
                            SecurityScore = security,
                            PerformanceScore = performance,
                            CleanlinessScore = cleanliness,
                            PrivacyScore = privacy,
                            CriticalIssues = criticalIssues,
                            WarningIssues = warningIssues,
                            InfoIssues = infoIssues,
                            RecordedAt = now
                        });
                }
                catch (Exception ex)
                {
                    Log.Warning(ex, "Failed to persist health snapshot");
                }

                return snapshot;
            }, ct);
        }

        private bool IsTelemetryEnabled()
        {
            // Policy override takes precedence; the legacy location is the fallback.
            var policy = _registry.ReadDWord("HKLM", @"SOFTWARE\Policies\Microsoft\Windows\DataCollection", "AllowTelemetry");
            if (policy is not null)
            {
                return policy.Value > 0;
            }

            var legacy = _registry.ReadDWord("HKLM", @"SOFTWARE\Microsoft\Windows\CurrentVersion\Policies\DataCollection", "AllowTelemetry");
            return legacy is not null && legacy.Value > 0;
        }

        private static int ClampToScore(double value)
        {
            return (int)Math.Clamp(Math.Round(value), 0, 100);
        }
    }
}

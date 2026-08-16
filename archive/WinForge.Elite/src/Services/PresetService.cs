using Dapper;
using WinForge.Elite.Data;
using WinForge.Elite.Models;

namespace WinForge.Elite.Services
{
    /// <summary>
    /// Applies preset profiles: every included tweak and privacy rule is applied
    /// under a single system restore point. Items that are already applied/enabled
    /// are skipped. The per-tweak audit trail is preserved by the underlying
    /// TweakService/PrivacyService calls.
    /// </summary>
    public sealed class PresetService
    {
        private static readonly Serilog.ILogger Log = Logging.Logger.GetLogger<PresetService>();

        private readonly TweakService _tweaks;
        private readonly PrivacyService _privacy;
        private readonly RestorePointService _restorePoints;

        public PresetService(TweakService tweaks, PrivacyService privacy, RestorePointService restorePoints)
        {
            _tweaks = tweaks ?? throw new ArgumentNullException(nameof(tweaks));
            _privacy = privacy ?? throw new ArgumentNullException(nameof(privacy));
            _restorePoints = restorePoints ?? throw new ArgumentNullException(nameof(restorePoints));
        }

        public async Task<OperationResult> ApplyAsync(
            Preset preset,
            IReadOnlyList<Tweak> tweaks,
            IReadOnlyList<PrivacyRule> privacyRules,
            CancellationToken ct = default)
        {
            if (preset is null)
            {
                throw new ArgumentNullException(nameof(preset));
            }

            var tweakIds = preset.IncludedTweakIds;
            var ruleIds = preset.IncludedPrivacyRuleIds;
            if (tweakIds.Count == 0 && ruleIds.Count == 0)
            {
                return new OperationResult(false, $"Preset '{preset.Name}' does not include any tweaks or privacy rules.");
            }

            var restorePoint = await _restorePoints.CreateAsync($"WinForge Elite: applying preset '{preset.Name}'", ct).ConfigureAwait(false);
            if (!restorePoint.Success)
            {
                Log.Warning("Restore point skipped for preset {PresetId}: {Message}", preset.Id, restorePoint.Message);
            }

            var applied = 0;
            var skipped = 0;
            string? firstError = null;

            foreach (var tweakId in tweakIds)
            {
                var tweak = tweaks.FirstOrDefault(t => t.Id == tweakId);
                if (tweak is null)
                {
                    Log.Warning("Preset {PresetId} references missing tweak {TweakId}", preset.Id, tweakId);
                    skipped++;
                    continue;
                }

                if (tweak.Applied)
                {
                    skipped++;
                    continue;
                }

                var result = await _tweaks.ApplyAsync(tweak, createRestorePoint: false, ct).ConfigureAwait(false);
                if (result.Success)
                {
                    applied++;
                }
                else
                {
                    firstError ??= result.Message;
                }
            }

            foreach (var ruleId in ruleIds)
            {
                var rule = privacyRules.FirstOrDefault(r => r.Id == ruleId);
                if (rule is null)
                {
                    Log.Warning("Preset {PresetId} references missing privacy rule {RuleId}", preset.Id, ruleId);
                    skipped++;
                    continue;
                }

                if (rule.Enabled)
                {
                    skipped++;
                    continue;
                }

                var result = await _privacy.SetRuleAsync(rule, enable: true, createRestorePoint: false, ct).ConfigureAwait(false);
                if (result.Success)
                {
                    applied++;
                }
                else
                {
                    firstError ??= result.Message;
                }
            }

            using (var connection = DbConnectionFactory.CreateConnection())
            {
                connection.Open();
                AuditService.Log(
                    connection,
                    null,
                    OperationType.Tweak,
                    $"Preset: {preset.Name}",
                    $"Applied preset '{preset.Name}': {applied} item(s) applied, {skipped} skipped.",
                    success: firstError is null,
                    errorMessage: firstError,
                    restorePointId: restorePoint.RestorePointId);
            }

            var errorSuffix = firstError is null ? string.Empty : $" First failure: {firstError}";
            return new OperationResult(
                firstError is null,
                $"Preset '{preset.Name}': {applied} applied, {skipped} skipped.{errorSuffix}",
                restorePoint.RestorePointId);
        }
    }
}

using Dapper;
using WinForge.Elite.Data;
using WinForge.Elite.Models;

namespace WinForge.Elite.Services
{
    /// <summary>
    /// Applies and reverts privacy rules. Each rule carries structured operations
    /// and undo operations (the same JSON format the TweakService uses), so enabling
    /// a rule writes real registry values and disabling it restores the defaults.
    /// </summary>
    public sealed class PrivacyService
    {
        private static readonly Serilog.ILogger Log = Logging.Logger.GetLogger<PrivacyService>();

        private readonly RestorePointService _restorePoints;
        private readonly TweakService _tweaks;

        public PrivacyService(TweakService tweaks, RestorePointService restorePoints)
        {
            _tweaks = tweaks ?? throw new ArgumentNullException(nameof(tweaks));
            _restorePoints = restorePoints ?? throw new ArgumentNullException(nameof(restorePoints));
        }

        public async Task<OperationResult> SetRuleAsync(PrivacyRule rule, bool enable, bool createRestorePoint = true, CancellationToken ct = default)
        {
            if (rule is null)
            {
                throw new ArgumentNullException(nameof(rule));
            }

            try
            {
                var operations = TweakService.ParseOperations(enable ? rule.Operations : rule.UndoOperations);
                if (operations.Count == 0)
                {
                    return new OperationResult(false, $"Privacy rule '{rule.Name}' defines no {(enable ? "operations" : "undo operations")}.");
                }

                int? restorePointId = null;
                if (createRestorePoint)
                {
                    var restorePoint = await _restorePoints.CreateAsync($"WinForge Elite: {(enable ? "enabling" : "disabling")} '{rule.Name}'", ct).ConfigureAwait(false);
                    restorePointId = restorePoint.RestorePointId;
                    if (!restorePoint.Success)
                    {
                        Log.Warning("Restore point skipped for privacy rule {RuleId}: {Message}", rule.Id, restorePoint.Message);
                    }
                }

                for (var index = 0; index < operations.Count; index++)
                {
                    var step = await _tweaks.ExecuteOperationAsync(operations[index], ct).ConfigureAwait(false);
                    if (!step.Success)
                    {
                        return new OperationResult(false, $"Operation {index + 1} of {operations.Count} failed: {step.Message}", restorePointId);
                    }
                }

                using var connection = DbConnectionFactory.CreateConnection();
                connection.Open();
                using var transaction = connection.BeginTransaction();
                var now = DateTime.UtcNow.ToString("o");
                connection.Execute(
                    "UPDATE PrivacyRules SET Enabled = @Enabled, UpdatedAt = @Now WHERE Id = @Id",
                    new { Enabled = enable ? 1 : 0, Now = now, Id = rule.Id },
                    transaction);
                AuditService.Log(
                    connection,
                    transaction,
                    OperationType.Privacy,
                    $"{(enable ? "Enable" : "Disable")}: {rule.Name}",
                    $"{(enable ? "Enabled" : "Disabled")} privacy rule '{rule.Name}' ({operations.Count} operation(s)).",
                    success: true,
                    restorePointId: restorePointId);
                transaction.Commit();

                return new OperationResult(true, $"{(enable ? "Enabled" : "Disabled")}: {rule.Name}", restorePointId);
            }
            catch (Exception ex)
            {
                Log.Error(ex, "SetRuleAsync failed for rule {RuleId}", rule.Id);
                return new OperationResult(false, ex.Message);
            }
        }

        /// <summary>Enables every currently-disabled rule under a single restore point.</summary>
        public async Task<OperationResult> HardenAllAsync(IReadOnlyList<PrivacyRule> rules, CancellationToken ct = default)
        {
            if (rules is null)
            {
                throw new ArgumentNullException(nameof(rules));
            }

            var targets = rules.Where(r => !r.Enabled).ToList();
            if (targets.Count == 0)
            {
                return new OperationResult(true, "All privacy rules are already enabled.");
            }

            var restorePoint = await _restorePoints.CreateAsync("WinForge Elite: Harden All privacy rules", ct).ConfigureAwait(false);
            if (!restorePoint.Success)
            {
                Log.Warning("Restore point skipped for Harden All: {Message}", restorePoint.Message);
            }

            var applied = 0;
            string? firstError = null;
            foreach (var rule in targets)
            {
                var result = await SetRuleAsync(rule, enable: true, createRestorePoint: false, ct).ConfigureAwait(false);
                if (result.Success)
                {
                    applied++;
                }
                else
                {
                    firstError ??= result.Message;
                }
            }

            var suffix = firstError is null ? string.Empty : $" First failure: {firstError}";
            return new OperationResult(applied == targets.Count, $"Enabled {applied}/{targets.Count} privacy rules.{suffix}", restorePoint.RestorePointId);
        }
    }
}

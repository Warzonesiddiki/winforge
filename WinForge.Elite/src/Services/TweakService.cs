using Dapper;
using Newtonsoft.Json;
using System.Data;
using WinForge.Elite.Data;
using WinForge.Elite.Helpers;
using WinForge.Elite.Models;

namespace WinForge.Elite.Services
{
    /// <summary>Outcome of a single step inside a multi-operation apply/undo.</summary>
    public readonly record struct OperationStep(bool Success, string Message);

    /// <summary>
    /// Orchestrates the tweak apply/undo pipeline:
    ///   snapshot → restore point → execute operations → verify → audit → persist.
    /// On a mid-sequence failure the already-executed operations are rolled back
    /// (undo operations first, captured registry snapshot as fallback).
    /// </summary>
    public sealed class TweakService
    {
        private static readonly Serilog.ILogger Log = Logging.Logger.GetLogger<TweakService>();

        private readonly RegistryService _registry;
        private readonly PowerShellService _powerShell;
        private readonly RestorePointService _restorePoints;

        public TweakService(RegistryService registry, PowerShellService powerShell, RestorePointService restorePoints)
        {
            _registry = registry ?? throw new ArgumentNullException(nameof(registry));
            _powerShell = powerShell ?? throw new ArgumentNullException(nameof(powerShell));
            _restorePoints = restorePoints ?? throw new ArgumentNullException(nameof(restorePoints));
        }

        /// <summary>Deserializes the operations JSON stored on a catalog row.</summary>
        public static List<Operation> ParseOperations(string? operationsJson)
        {
            if (string.IsNullOrWhiteSpace(operationsJson))
            {
                return new List<Operation>();
            }

            return JsonConvert.DeserializeObject<List<Operation>>(operationsJson) ?? new List<Operation>();
        }

        public async Task<OperationResult> ApplyAsync(Tweak tweak, bool createRestorePoint = true, CancellationToken ct = default)
        {
            if (tweak is null)
            {
                throw new ArgumentNullException(nameof(tweak));
            }

            try
            {
                var operations = ParseOperations(tweak.Operations);
                if (operations.Count == 0)
                {
                    return new OperationResult(false, $"Tweak '{tweak.Name}' defines no operations — nothing to apply.");
                }

                // 1. Snapshot the current state of every affected registry key.
                List<KeySnapshot>? snapshot = null;
                string? snapshotPath = null;
                try
                {
                    snapshot = await _registry.CaptureSnapshotAsync(operations, ct).ConfigureAwait(false);
                    snapshotPath = SaveSnapshot(tweak.Id, snapshot);
                }
                catch (Exception ex)
                {
                    Log.Warning(ex, "Registry snapshot failed for tweak {TweakId}; continuing without a snapshot", tweak.Id);
                }

                // 2. Create a system restore point before mutating anything.
                int? restorePointId = null;
                if (createRestorePoint)
                {
                    var restorePoint = await _restorePoints.CreateAsync($"WinForge Elite: applying '{tweak.Name}'", ct).ConfigureAwait(false);
                    restorePointId = restorePoint.RestorePointId;
                    if (!restorePoint.Success)
                    {
                        Log.Warning("Restore point skipped for tweak {TweakId}: {Message}", tweak.Id, restorePoint.Message);
                    }
                }

                // 3. Execute operations sequentially with rollback on failure.
                for (var index = 0; index < operations.Count; index++)
                {
                    var step = await ExecuteOperationAsync(operations[index], ct).ConfigureAwait(false);
                    if (!step.Success)
                    {
                        Log.Error("Operation {Index} failed for tweak {TweakId}: {Message}", index + 1, tweak.Id, step.Message);
                        await RollbackAsync(ParseOperations(tweak.UndoOperations), snapshot, ct).ConfigureAwait(false);
                        return new OperationResult(
                            false,
                            $"Operation {index + 1} of {operations.Count} failed: {step.Message}",
                            restorePointId,
                            snapshotPath);
                    }
                }

                // 4. Persist the new state and write the audit trail.
                using var connection = DbConnectionFactory.CreateConnection();
                connection.Open();
                using var transaction = connection.BeginTransaction();
                var now = DateTime.UtcNow.ToString("o");
                connection.Execute(
                    "UPDATE Tweaks SET Applied = 1, UpdatedAt = @Now WHERE Id = @Id",
                    new { Now = now, Id = tweak.Id },
                    transaction);

                var undoPayload = JsonConvert.SerializeObject(new
                {
                    UndoOperations = tweak.UndoOperations,
                    SnapshotPath = snapshotPath
                });
                AuditService.Log(
                    connection,
                    transaction,
                    OperationType.Tweak,
                    $"Apply: {tweak.Name}",
                    $"Applied tweak '{tweak.Name}' ({operations.Count} operation(s)).",
                    undoPayload,
                    success: true,
                    restorePointId: restorePointId);
                transaction.Commit();

                return new OperationResult(true, $"Applied: {tweak.Name}", restorePointId, snapshotPath);
            }
            catch (Exception ex)
            {
                Log.Error(ex, "ApplyAsync failed for tweak {TweakId}", tweak.Id);
                return new OperationResult(false, ex.Message);
            }
        }

        public async Task<OperationResult> UndoAsync(Tweak tweak, CancellationToken ct = default)
        {
            if (tweak is null)
            {
                throw new ArgumentNullException(nameof(tweak));
            }

            try
            {
                var operations = ParseOperations(tweak.UndoOperations);
                if (operations.Count == 0)
                {
                    return new OperationResult(false, $"Tweak '{tweak.Name}' has no undo operations defined.");
                }

                for (var index = 0; index < operations.Count; index++)
                {
                    var step = await ExecuteOperationAsync(operations[index], ct).ConfigureAwait(false);
                    if (!step.Success)
                    {
                        Log.Error("Undo operation {Index} failed for tweak {TweakId}: {Message}", index + 1, tweak.Id, step.Message);
                        return new OperationResult(false, $"Undo operation {index + 1} of {operations.Count} failed: {step.Message}");
                    }
                }

                using var connection = DbConnectionFactory.CreateConnection();
                connection.Open();
                using var transaction = connection.BeginTransaction();
                var now = DateTime.UtcNow.ToString("o");
                connection.Execute(
                    "UPDATE Tweaks SET Applied = 0, UpdatedAt = @Now WHERE Id = @Id",
                    new { Now = now, Id = tweak.Id },
                    transaction);
                AuditService.Log(
                    connection,
                    transaction,
                    OperationType.Tweak,
                    $"Undo: {tweak.Name}",
                    $"Reverted tweak '{tweak.Name}' ({operations.Count} operation(s)).",
                    undoPayload: null,
                    success: true);
                transaction.Commit();

                return new OperationResult(true, $"Reverted: {tweak.Name}");
            }
            catch (Exception ex)
            {
                Log.Error(ex, "UndoAsync failed for tweak {TweakId}", tweak.Id);
                return new OperationResult(false, ex.Message);
            }
        }

        /// <summary>Executes a single catalog operation (registry, command, service, or scheduled task).</summary>
        public async Task<OperationStep> ExecuteOperationAsync(Operation operation, CancellationToken ct = default)
        {
            switch (operation.Type)
            {
                case "registry":
                    return await ExecuteRegistryAsync(operation, ct).ConfigureAwait(false);
                case "command":
                    return await ExecuteCommandAsync(operation, ct).ConfigureAwait(false);
                case "service":
                    return await ExecuteServiceAsync(operation, ct).ConfigureAwait(false);
                case "scheduledTask":
                    return await ExecuteScheduledTaskAsync(operation, ct).ConfigureAwait(false);
                default:
                    return new OperationStep(false, $"Unknown operation type '{operation.Type}'.");
            }
        }

        private async Task<OperationStep> ExecuteRegistryAsync(Operation operation, CancellationToken ct)
        {
            try
            {
                if (string.IsNullOrWhiteSpace(operation.Hive) || string.IsNullOrWhiteSpace(operation.Key))
                {
                    return new OperationStep(false, "Registry operation is missing hive or key.");
                }

                await _registry.WriteValueAsync(operation.Hive, operation.Key, operation.ValueName, operation.Kind ?? "String", operation.Data, ct).ConfigureAwait(false);
                if (!await _registry.VerifyAsync(operation, ct).ConfigureAwait(false))
                {
                    return new OperationStep(false, $"Registry verification failed for {operation.Hive}\\{operation.Key}\\{operation.ValueName ?? "(default)"}.");
                }

                return new OperationStep(true, "Registry value set.");
            }
            catch (Exception ex)
            {
                return new OperationStep(false, ex.Message);
            }
        }

        private async Task<OperationStep> ExecuteCommandAsync(Operation operation, CancellationToken ct)
        {
            if (string.IsNullOrWhiteSpace(operation.Command))
            {
                return new OperationStep(false, "Command operation has no command text.");
            }

            var result = await _powerShell.RunAsync(operation.Command, ct: ct).ConfigureAwait(false);
            if (result.Success)
            {
                return new OperationStep(true, "Command completed.");
            }

            var detail = result.Errors.Length > 0 ? result.Errors : $"exit code {result.ExitCode}";
            return new OperationStep(false, detail);
        }

        private async Task<OperationStep> ExecuteServiceAsync(Operation operation, CancellationToken ct)
        {
            if (string.IsNullOrWhiteSpace(operation.Name) || string.IsNullOrWhiteSpace(operation.StartMode))
            {
                return new OperationStep(false, "Service operation requires 'name' and 'startMode'.");
            }

            var serviceName = ValidateName(operation.Name);
            var startMode = operation.StartMode switch
            {
                "Disabled" => "disabled",
                "Manual" => "demand",
                "Automatic" => "auto",
                "Automatic (Delayed)" => "delayed-auto",
                var other => other
            };

            var result = await _powerShell.RunAsync($"sc.exe config \"{serviceName}\" start= {startMode}", ct: ct).ConfigureAwait(false);
            if (result.Success)
            {
                return new OperationStep(true, $"Service '{serviceName}' start mode set to {startMode}.");
            }

            var detail = result.Errors.Length > 0 ? result.Errors : $"exit code {result.ExitCode}";
            return new OperationStep(false, detail);
        }

        private async Task<OperationStep> ExecuteScheduledTaskAsync(Operation operation, CancellationToken ct)
        {
            if (string.IsNullOrWhiteSpace(operation.TaskPath) || string.IsNullOrWhiteSpace(operation.Action))
            {
                return new OperationStep(false, "ScheduledTask operation requires 'taskPath' and 'action'.");
            }

            var taskPath = ValidateTaskPath(operation.TaskPath);
            var verb = operation.Action switch
            {
                "Disable" => "/Disable",
                "Enable" => "/Enable",
                var other => other
            };

            var result = await _powerShell.RunAsync($"schtasks.exe /Change /TN \"{taskPath}\" {verb}", ct: ct).ConfigureAwait(false);
            if (result.Success)
            {
                return new OperationStep(true, $"Scheduled task '{taskPath}' {operation.Action}d.");
            }

            var detail = result.Errors.Length > 0 ? result.Errors : $"exit code {result.ExitCode}";
            return new OperationStep(false, detail);
        }

        private async Task RollbackAsync(List<Operation> undoOperations, List<KeySnapshot>? snapshot, CancellationToken ct)
        {
            try
            {
                if (undoOperations.Count > 0)
                {
                    foreach (var operation in undoOperations)
                    {
                        var step = await ExecuteOperationAsync(operation, ct).ConfigureAwait(false);
                        if (!step.Success)
                        {
                            Log.Error("Rollback operation failed: {Message}", step.Message);
                        }
                    }
                }
                else if (snapshot is { Count: > 0 })
                {
                    await _registry.RestoreSnapshotAsync(snapshot, ct).ConfigureAwait(false);
                    Log.Information("Rolled back {Count} registry key(s) from snapshot", snapshot.Count);
                }
            }
            catch (Exception ex)
            {
                Log.Error(ex, "Rollback failed");
            }
        }

        private static string? SaveSnapshot(string tweakId, List<KeySnapshot> snapshot)
        {
            var directory = PathHelper.GetBackupPath($"tweak_{tweakId}");
            Directory.CreateDirectory(directory);
            var file = Path.Combine(directory, "snapshot.json");
            File.WriteAllText(file, JsonConvert.SerializeObject(snapshot, Formatting.Indented));
            return file;
        }

        private static string ValidateName(string name)
        {
            if (string.IsNullOrWhiteSpace(name) || !System.Text.RegularExpressions.Regex.IsMatch(name, @"^[A-Za-z0-9_ .-]+$"))
            {
                throw new InvalidOperationException($"Invalid service name '{name}'.");
            }

            return name;
        }

        private static string ValidateTaskPath(string taskPath)
        {
            if (string.IsNullOrWhiteSpace(taskPath) || !System.Text.RegularExpressions.Regex.IsMatch(taskPath, @"^[A-Za-z0-9_\\ .-]+$"))
            {
                throw new InvalidOperationException($"Invalid scheduled task path '{taskPath}'.");
            }

            return taskPath;
        }
    }
}

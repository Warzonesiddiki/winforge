using Dapper;
using System.Text.RegularExpressions;
using WinForge.Elite.Data;
using WinForge.Elite.Models;

namespace WinForge.Elite.Services
{
    /// <summary>
    /// Removes and reinstalls Appx packages (debloat) through PowerShell.
    /// Batch removals share a single system restore point. Protected packages are
    /// always refused. Package names are validated against a strict whitelist before
    /// they reach the shell.
    /// </summary>
    public sealed class DebloatService
    {
        private static readonly Serilog.ILogger Log = Logging.Logger.GetLogger<DebloatService>();

        private static readonly Regex PackageNamePattern = new(@"^[A-Za-z0-9._-]+$", RegexOptions.Compiled);

        private readonly PowerShellService _powerShell;
        private readonly RestorePointService _restorePoints;

        public DebloatService(PowerShellService powerShell, RestorePointService restorePoints)
        {
            _powerShell = powerShell ?? throw new ArgumentNullException(nameof(powerShell));
            _restorePoints = restorePoints ?? throw new ArgumentNullException(nameof(restorePoints));
        }

        public Task<OperationResult> RemoveAsync(DebloatPackage package, CancellationToken ct = default)
        {
            return RemoveBatchAsync(new[] { package }, ct);
        }

        /// <summary>Removes a batch of packages under a single restore point.</summary>
        public async Task<OperationResult> RemoveBatchAsync(IReadOnlyList<DebloatPackage> packages, CancellationToken ct = default)
        {
            if (packages is null)
            {
                throw new ArgumentNullException(nameof(packages));
            }

            var targets = packages.Where(p => p.Status != PackageStatus.Protected).ToList();
            var protectedCount = packages.Count - targets.Count;
            if (targets.Count == 0)
            {
                return new OperationResult(false, "No removable packages selected (protected packages cannot be removed).");
            }

            var restorePoint = await _restorePoints.CreateAsync($"WinForge Elite: removing {targets.Count} bloatware package(s)", ct).ConfigureAwait(false);
            if (!restorePoint.Success)
            {
                Log.Warning("Restore point skipped for debloat batch: {Message}", restorePoint.Message);
            }

            var removed = 0;
            string? firstError = null;
            foreach (var package in targets)
            {
                var result = await RemoveCoreAsync(package, ct).ConfigureAwait(false);
                if (result.Success)
                {
                    removed++;
                }
                else
                {
                    firstError ??= result.Message;
                }
            }

            var suffix = protectedCount > 0 ? $" ({protectedCount} protected package(s) skipped)" : string.Empty;
            var errorSuffix = firstError is null ? string.Empty : $" First failure: {firstError}";
            return new OperationResult(
                removed == targets.Count,
                $"Removed {removed}/{targets.Count} package(s).{suffix}{errorSuffix}",
                restorePoint.RestorePointId);
        }

        public async Task<OperationResult> ReinstallAsync(DebloatPackage package, CancellationToken ct = default)
        {
            if (package is null)
            {
                throw new ArgumentNullException(nameof(package));
            }

            if (!package.CanReinstall)
            {
                return new OperationResult(false, $"'{package.DisplayName}' cannot be reinstalled.");
            }

            try
            {
                var name = SanitizePackageName(package.PackageName);
                var script = $"$pkg = Get-AppxPackage -AllUsers -Name '{name}' | Select-Object -First 1" + "\n" +
                             "if ($null -eq $pkg) { Write-Output 'NOT_FOUND' }" + "\n" +
                             "else { Add-AppxPackage -DisableDevelopmentMode -Register ($pkg.InstallLocation + '\\AppXManifest.xml') }";
                var result = await _powerShell.RunAsync(script, ct: ct).ConfigureAwait(false);
                var notFound = result.Output.Contains("NOT_FOUND", StringComparison.Ordinal);
                if (!result.Success && !notFound)
                {
                    var detail = result.Errors.Length > 0 ? result.Errors : $"exit code {result.ExitCode}";
                    RecordFailure(OperationType.Debloat, $"Reinstall: {package.DisplayName}", detail);
                    return new OperationResult(false, $"Failed to reinstall '{package.DisplayName}': {detail}");
                }

                using var connection = DbConnectionFactory.CreateConnection();
                connection.Open();
                var now = DateTime.UtcNow.ToString("o");
                connection.Execute(
                    "UPDATE DebloatPackages SET Status = 0, UpdatedAt = @Now WHERE PackageName = @Name",
                    new { Now = now, Name = package.PackageName });
                AuditService.Log(
                    connection,
                    null,
                    OperationType.Debloat,
                    $"Reinstall: {package.DisplayName}",
                    notFound
                        ? $"Package '{package.DisplayName}' is no longer provisioned on this system."
                        : $"Reinstalled Appx package '{package.DisplayName}'.");

                return new OperationResult(true, notFound
                    ? $"'{package.DisplayName}' is no longer provisioned on this system."
                    : $"Reinstalled: {package.DisplayName}");
            }
            catch (Exception ex)
            {
                Log.Error(ex, "ReinstallAsync failed for {Package}", package.PackageName);
                return new OperationResult(false, ex.Message);
            }
        }

        private async Task<OperationResult> RemoveCoreAsync(DebloatPackage package, CancellationToken ct)
        {
            try
            {
                var name = SanitizePackageName(package.PackageName);
                var script = $"$pkg = Get-AppxPackage -Name '{name}'" + "\n" +
                             "if ($null -eq $pkg) { Write-Output 'NOT_FOUND' }" + "\n" +
                             "else { $pkg | Remove-AppxPackage -AllUsers }";
                var result = await _powerShell.RunAsync(script, ct: ct).ConfigureAwait(false);
                var notFound = result.Output.Contains("NOT_FOUND", StringComparison.Ordinal);
                if (!result.Success && !notFound)
                {
                    var detail = result.Errors.Length > 0 ? result.Errors : $"exit code {result.ExitCode}";
                    RecordFailure(OperationType.Debloat, $"Remove: {package.DisplayName}", detail);
                    return new OperationResult(false, $"Failed to remove '{package.DisplayName}': {detail}");
                }

                using var connection = DbConnectionFactory.CreateConnection();
                connection.Open();
                var now = DateTime.UtcNow.ToString("o");
                connection.Execute(
                    "UPDATE DebloatPackages SET Status = 1, UpdatedAt = @Now WHERE PackageName = @Name",
                    new { Now = now, Name = package.PackageName });
                AuditService.Log(
                    connection,
                    null,
                    OperationType.Debloat,
                    $"Remove: {package.DisplayName}",
                    notFound
                        ? $"Package '{package.DisplayName}' was not installed (marked removed)."
                        : $"Removed Appx package '{package.DisplayName}' for all users.");

                return new OperationResult(true, notFound
                    ? $"'{package.DisplayName}' was not installed (marked removed)."
                    : $"Removed: {package.DisplayName}");
            }
            catch (Exception ex)
            {
                Log.Error(ex, "RemoveCoreAsync failed for {Package}", package.PackageName);
                return new OperationResult(false, ex.Message);
            }
        }

        private static void RecordFailure(OperationType type, string operationName, string detail)
        {
            try
            {
                using var connection = DbConnectionFactory.CreateConnection();
                connection.Open();
                AuditService.Log(connection, null, type, operationName, detail, success: false, errorMessage: detail);
            }
            catch (Exception ex)
            {
                Log.Warning(ex, "Failed to record audit failure entry for {Operation}", operationName);
            }
        }

        private static string SanitizePackageName(string packageName)
        {
            if (string.IsNullOrWhiteSpace(packageName) || !PackageNamePattern.IsMatch(packageName))
            {
                throw new InvalidOperationException($"Invalid Appx package name '{packageName}'.");
            }

            return packageName;
        }
    }
}

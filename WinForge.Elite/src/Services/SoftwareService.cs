using Dapper;
using System.Text.RegularExpressions;
using WinForge.Elite.Data;
using WinForge.Elite.Models;

namespace WinForge.Elite.Services
{
    /// <summary>
    /// Installs and uninstalls applications through the winget package manager.
    /// Installation state is recorded in the local database; success is determined
    /// by the winget process exit code.
    /// </summary>
    public sealed class SoftwareService
    {
        private static readonly Serilog.ILogger Log = Logging.Logger.GetLogger<SoftwareService>();

        private static readonly Regex WingetIdPattern = new(@"^[A-Za-z0-9.+_-]+$", RegexOptions.Compiled);

        private readonly PowerShellService _powerShell;

        public SoftwareService(PowerShellService powerShell)
        {
            _powerShell = powerShell ?? throw new ArgumentNullException(nameof(powerShell));
        }

        public async Task<OperationResult> InstallAsync(Application app, CancellationToken ct = default)
        {
            if (app is null)
            {
                throw new ArgumentNullException(nameof(app));
            }

            try
            {
                var id = SanitizeWingetId(app.Id);
                var script = $"winget install --id \"{id}\" --exact --silent --accept-package-agreements --accept-source-agreements";
                var result = await _powerShell.RunAsync(script, ct: ct).ConfigureAwait(false);
                if (!result.Success)
                {
                    var detail = result.Errors.Length > 0 ? result.Errors : $"exit code {result.ExitCode}";
                    return new OperationResult(false, $"winget install failed for '{app.Name}': {detail}");
                }

                using var connection = DbConnectionFactory.CreateConnection();
                connection.Open();
                var now = DateTime.UtcNow.ToString("o");
                connection.Execute(
                    "UPDATE Applications SET Installed = 1, UpdatedAt = @Now WHERE Id = @Id",
                    new { Now = now, Id = app.Id });
                AuditService.Log(
                    connection,
                    null,
                    OperationType.SoftwareInstall,
                    $"Install: {app.Name}",
                    $"Installed '{app.Name}' via winget (id {app.Id}).");

                return new OperationResult(true, $"Installed: {app.Name}");
            }
            catch (Exception ex)
            {
                Log.Error(ex, "InstallAsync failed for {AppId}", app.Id);
                return new OperationResult(false, ex.Message);
            }
        }

        public async Task<OperationResult> UninstallAsync(Application app, CancellationToken ct = default)
        {
            if (app is null)
            {
                throw new ArgumentNullException(nameof(app));
            }

            try
            {
                var id = SanitizeWingetId(app.Id);
                var script = $"winget uninstall --id \"{id}\" --exact --silent";
                var result = await _powerShell.RunAsync(script, ct: ct).ConfigureAwait(false);
                if (!result.Success)
                {
                    var detail = result.Errors.Length > 0 ? result.Errors : $"exit code {result.ExitCode}";
                    return new OperationResult(false, $"winget uninstall failed for '{app.Name}': {detail}");
                }

                using var connection = DbConnectionFactory.CreateConnection();
                connection.Open();
                var now = DateTime.UtcNow.ToString("o");
                connection.Execute(
                    "UPDATE Applications SET Installed = 0, UpdatedAt = @Now WHERE Id = @Id",
                    new { Now = now, Id = app.Id });
                AuditService.Log(
                    connection,
                    null,
                    OperationType.SoftwareUninstall,
                    $"Uninstall: {app.Name}",
                    $"Uninstalled '{app.Name}' via winget (id {app.Id}).");

                return new OperationResult(true, $"Uninstalled: {app.Name}");
            }
            catch (Exception ex)
            {
                Log.Error(ex, "UninstallAsync failed for {AppId}", app.Id);
                return new OperationResult(false, ex.Message);
            }
        }

        private static string SanitizeWingetId(string id)
        {
            if (string.IsNullOrWhiteSpace(id) || !WingetIdPattern.IsMatch(id))
            {
                throw new InvalidOperationException($"Invalid winget package id '{id}'.");
            }

            return id;
        }
    }
}

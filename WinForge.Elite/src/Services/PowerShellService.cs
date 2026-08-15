using System.Management.Automation;
using System.Management.Automation.Runspaces;

namespace WinForge.Elite.Services
{
    /// <summary>
    /// Executes PowerShell scripts asynchronously with output capture, exit-code
    /// reporting, timeout, and cancellation. The runspace uses the default session
    /// state with an unrestricted execution policy (equivalent to running
    /// powershell.exe locally), which the application needs for Appx/winget/sc.exe
    /// maintenance commands.
    ///
    /// Security note: every script built by this application uses command names and
    /// arguments that originate from the local catalog database and are validated
    /// with whitelist regular expressions (see SanitizePackageName/SanitizeWingetId).
    /// Never pass raw user input to this service.
    /// </summary>
    public sealed class PowerShellService
    {
        public sealed record ExecutionResult(
            bool Success,
            int ExitCode,
            string Output,
            string Errors,
            string Warnings,
            bool TimedOut,
            TimeSpan Duration);

        private static readonly Serilog.ILogger Log = Logging.Logger.GetLogger<PowerShellService>();

        private static readonly TimeSpan DefaultTimeout = TimeSpan.FromMinutes(5);

        public async Task<ExecutionResult> RunAsync(string script, TimeSpan? timeout = null, CancellationToken ct = default)
        {
            var started = DateTime.UtcNow;
            PowerShell? ps = null;
            try
            {
                var initialState = InitialSessionState.CreateDefault();
                initialState.ExecutionPolicy = ExecutionPolicy.Unrestricted;
                initialState.LanguageMode = PSLanguageMode.FullLanguage;
                ps = PowerShell.Create(initialState);

                ps.AddScript(script);
                // Append $LASTEXITCODE as the final pipeline object so the caller gets a real exit code.
                ps.AddScript("if ($null -eq $global:LASTEXITCODE) { $global:LASTEXITCODE = 0 }; $global:LASTEXITCODE");

                using var registration = ct.Register(() =>
                {
                    try
                    {
                        ps?.BeginStop(null, null);
                    }
                    catch (ObjectDisposedException)
                    {
                        // The runspace was already disposed; nothing to stop.
                    }
                });

                var invokeTask = Task.Run(() => ps.Invoke());
                var effectiveTimeout = timeout ?? DefaultTimeout;
                var completed = await Task.WhenAny(invokeTask, Task.Delay(effectiveTimeout, ct));
                var timedOut = completed != invokeTask;
                if (timedOut)
                {
                    Log.Warning("PowerShell command exceeded timeout of {Timeout} — stopping", effectiveTimeout);
                    ps.BeginStop(null, null);
                    await invokeTask;
                }

                var output = invokeTask.Result;
                var exitCode = 0;
                if (output.Count > 0 && output[output.Count - 1]?.BaseObject is int lastExitCode)
                {
                    exitCode = lastExitCode;
                }

                var text = string.Join(
                    Environment.NewLine,
                    output.Take(Math.Max(0, output.Count - 1)).Select(o => o?.ToString() ?? string.Empty));
                var errors = string.Join(Environment.NewLine, ps.Streams.Error.Select(e => e.ToString()));
                var warnings = string.Join(Environment.NewLine, ps.Streams.Warning.Select(w => w.Message));
                var success = !timedOut && exitCode == 0 && errors.Length == 0;

                if (!success)
                {
                    Log.Warning("PowerShell command failed (exit {ExitCode}, timed out: {TimedOut}). Errors: {Errors}",
                        exitCode, timedOut, errors);
                }

                return new ExecutionResult(success, exitCode, text, errors, warnings, timedOut, DateTime.UtcNow - started);
            }
            catch (Exception ex)
            {
                Log.Error(ex, "PowerShell execution threw an exception");
                return new ExecutionResult(false, -1, string.Empty, ex.Message, string.Empty, false, DateTime.UtcNow - started);
            }
            finally
            {
                ps?.Dispose();
            }
        }

        /// <summary>Escapes a value for safe use inside a single-quoted PowerShell string.</summary>
        public static string EscapeSingleQuoted(string value)
        {
            return value.Replace("'", "''");
        }
    }
}

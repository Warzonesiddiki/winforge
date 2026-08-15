using Serilog;
using Serilog.Events;
using System.IO;

namespace WinForge.Elite.Logging
{
    public static class Logger
    {
        private static bool _initialized = false;

        public static void Initialize()
        {
            if (_initialized) return;

            var logPath = Path.Combine(
                Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
                "WinForge",
                "Elite",
                "Logs",
                "winforge-.log"
            );

            // Ensure directory exists
            Directory.CreateDirectory(Path.GetDirectoryName(logPath)!);

            Log.Logger = new LoggerConfiguration()
                .MinimumLevel.Debug()
                .MinimumLevel.Override("Microsoft", LogEventLevel.Information)
                .Enrich.FromLogContext()
                .Enrich.WithMachineName()
                .Enrich.WithThreadId()
                .WriteTo.Console(
                    outputTemplate: "[{Timestamp:HH:mm:ss} {Level:u3}] {Message:lj}{NewLine}{Exception}"
                )
                .WriteTo.File(
                    path: logPath,
                    rollingInterval: RollingInterval.Day,
                    retainedFileCountLimit: 7,
                    outputTemplate: "{Timestamp:yyyy-MM-dd HH:mm:ss.fff zzz} [{Level:u3}] [{ThreadId}] {Message:lj}{NewLine}{Exception}",
                    fileSizeLimitBytes: 10_000_000 // 10MB
                )
                .CreateLogger();

            Log.Information("WinForge Elite logging initialized");
            Log.Information("OS Version: {OSVersion}", Environment.OSVersion.VersionString);
            Log.Information("Process ID: {ProcessId}", System.Diagnostics.Process.GetCurrentProcess().Id);

            _initialized = true;
        }

        public static Serilog.ILogger GetLogger<T>()
        {
            if (!_initialized) Initialize();
            return Log.ForType<T>();
        }

        public static Serilog.ILogger GetLogger(string name)
        {
            if (!_initialized) Initialize();
            return Log.ForContext("SourceContext", name);
        }
    }

    // Custom enrichers
    public static class LogEnricherExtensions
    {
        public static LoggerConfiguration WithMachineName(this LoggerConfiguration configuration)
        {
            return configuration.Enrich.WithProperty("MachineName", Environment.MachineName);
        }

        public static LoggerConfiguration WithThreadId(this LoggerConfiguration configuration)
        {
            return configuration.Enrich.WithProperty("ThreadId", System.Threading.Thread.CurrentThread.ManagedThreadId);
        }
    }
}

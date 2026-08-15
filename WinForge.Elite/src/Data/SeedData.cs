using Dapper;
using Newtonsoft.Json;
using System.Data;
using WinForge.Elite.Models;

namespace WinForge.Elite.Data
{
    /// <summary>
    /// Idempotent seed catalog for a fresh WinForge Elite database.
    /// Every statement uses INSERT OR IGNORE, so seeding can safely run on every
    /// application start without duplicating rows.
    ///
    /// Phase 1 subset of the full catalog defined in src/db/seed-data.ts. The
    /// catalog will be expanded to the complete 60+ tweak / 90+ package / 40+ rule
    /// set once the Tweak/Debloat/Privacy engines exist.
    ///
    /// Tweak operations are stored as JSON arrays with the following schema so the
    /// upcoming RegistryService can parse them directly:
    ///   { "type": "registry", "hive": "HKLM|HKCU", "key": "path\\without\\hive",
    ///     "valueName": "Name" | null (default value), "kind": "DWord|QWord|String|ExpandString|MultiString|Binary",
    ///     "data": "0" }
    /// </summary>
    public static class SeedData
    {
        private sealed record RegistryOp(string Type, string Hive, string Key, string? ValueName, string Kind, string Data);

        private sealed record TweakRow(
            string Id, string Name, string Description, string Category, int Risk, int DefaultEnabled,
            string Tags, string? WarningMessage, string? BreaksFeatures, string Operations, string UndoOperations, string UpdatedAt);

        private sealed record DebloatRow(
            string PackageName, string DisplayName, string Category, int Risk, int CanReinstall,
            string? StoreId, string? BreaksFeatures, int Status, int ProvisionedRemoved, string UpdatedAt);

        private sealed record PrivacyRow(
            string Id, string Name, string Description, string Category, int Risk,
            int DefaultEnabled, int Enabled, string UpdatedAt);

        private sealed record ApplicationRow(
            string Id, string Name, string Publisher, string Category, string Version,
            string Source, int Installed, string UpdatedAt);

        private sealed record PresetRow(
            string Id, string Name, string Description, int Type,
            string? IncludedTweakIds, string? IncludedPrivacyRuleIds, string? ExcludedPackageNames,
            int IsProtected, string UpdatedAt);

        private static string Op(string hive, string key, string? valueName, string kind, string data)
            => JsonConvert.SerializeObject(new[] { new RegistryOp("registry", hive, key, valueName, kind, data) });

        private static string Ops(params RegistryOp[] ops)
            => JsonConvert.SerializeObject(ops);

        public static void SeedAll(IDbConnection connection)
        {
            SeedTweaks(connection);
            SeedDebloatPackages(connection);
            SeedPrivacyRules(connection);
            SeedApplications(connection);
            SeedPresets(connection);
        }

        private static void SeedTweaks(IDbConnection connection)
        {
            const string sql = @"
                INSERT OR IGNORE INTO Tweaks
                    (Id, Name, Description, Category, Risk, DefaultEnabled, Applied, Tags, WarningMessage, BreaksFeatures, Operations, UndoOperations, UpdatedAt)
                VALUES
                    (@Id, @Name, @Description, @Category, @Risk, @DefaultEnabled, 0, @Tags, @WarningMessage, @BreaksFeatures, @Operations, @UndoOperations, @UpdatedAt)";

            var now = DateTime.UtcNow.ToString("o");
            var rows = new[]
            {
                new TweakRow(
                    "tel-disable-telemetry",
                    "Disable Telemetry",
                    "Sets telemetry data collection to the Security (minimum) level.",
                    "Telemetry", (int)RiskLevel.Low, 1,
                    JsonConvert.SerializeObject(new[] { "privacy", "telemetry", "recommended" }),
                    null, null,
                    Op("HKLM", @"SOFTWARE\Policies\Microsoft\Windows\DataCollection", "AllowTelemetry", "DWord", "0"),
                    Op("HKLM", @"SOFTWARE\Policies\Microsoft\Windows\DataCollection", "AllowTelemetry", "DWord", "1"),
                    now),
                new TweakRow(
                    "tel-ceip",
                    "Disable Customer Experience Improvement Program",
                    "Opts out of CEIP usage data collection.",
                    "Telemetry", (int)RiskLevel.Low, 1,
                    JsonConvert.SerializeObject(new[] { "privacy", "telemetry" }),
                    null, null,
                    Op("HKLM", @"SOFTWARE\Policies\Microsoft\SQMClient\Windows", "CEIPEnable", "DWord", "0"),
                    Op("HKLM", @"SOFTWARE\Policies\Microsoft\SQMClient\Windows", "CEIPEnable", "DWord", "1"),
                    now),
                new TweakRow(
                    "tel-advertising-id",
                    "Disable Advertising ID",
                    "Turns off the per-user advertising identifier used for targeted ads.",
                    "Telemetry", (int)RiskLevel.Low, 1,
                    JsonConvert.SerializeObject(new[] { "privacy" }),
                    null, null,
                    Op("HKCU", @"SOFTWARE\Microsoft\Windows\CurrentVersion\AdvertisingInfo", "Enabled", "DWord", "0"),
                    Op("HKCU", @"SOFTWARE\Microsoft\Windows\CurrentVersion\AdvertisingInfo", "Enabled", "DWord", "1"),
                    now),
                new TweakRow(
                    "perf-disable-hibernation",
                    "Disable Hibernation",
                    "Disables hibernation so hiberfil.sys is removed, freeing disk space equal to installed RAM.",
                    "Performance", (int)RiskLevel.Medium, 0,
                    JsonConvert.SerializeObject(new[] { "power", "storage" }),
                    null,
                    JsonConvert.SerializeObject(new[] { "Fast Startup", "Hibernate" }),
                    Op("HKLM", @"SYSTEM\CurrentControlSet\Control\Power", "HibernateEnabled", "DWord", "0"),
                    Op("HKLM", @"SYSTEM\CurrentControlSet\Control\Power", "HibernateEnabled", "DWord", "1"),
                    now),
                new TweakRow(
                    "perf-game-mode",
                    "Enable Game Mode",
                    "Prioritizes game processes and reduces background interruptions while gaming.",
                    "Performance", (int)RiskLevel.Low, 1,
                    JsonConvert.SerializeObject(new[] { "gaming", "recommended" }),
                    null, null,
                    Op("HKCU", @"SOFTWARE\Microsoft\GameBar", "AutoGameModeEnabled", "DWord", "1"),
                    Op("HKCU", @"SOFTWARE\Microsoft\GameBar", "AutoGameModeEnabled", "DWord", "0"),
                    now),
                new TweakRow(
                    "ui-dark-mode",
                    "Enable Dark Mode",
                    "Switches Windows apps and the system UI to the dark theme.",
                    "UI", (int)RiskLevel.Low, 0,
                    JsonConvert.SerializeObject(new[] { "ui", "theme" }),
                    null, null,
                    Ops(
                        new RegistryOp("registry", "HKCU", @"SOFTWARE\Microsoft\Windows\CurrentVersion\Themes\Personalize", "AppsUseLightTheme", "DWord", "0"),
                        new RegistryOp("registry", "HKCU", @"SOFTWARE\Microsoft\Windows\CurrentVersion\Themes\Personalize", "SystemUsesLightTheme", "DWord", "0")),
                    Ops(
                        new RegistryOp("registry", "HKCU", @"SOFTWARE\Microsoft\Windows\CurrentVersion\Themes\Personalize", "AppsUseLightTheme", "DWord", "1"),
                        new RegistryOp("registry", "HKCU", @"SOFTWARE\Microsoft\Windows\CurrentVersion\Themes\Personalize", "SystemUsesLightTheme", "DWord", "1")),
                    now),
                new TweakRow(
                    "exp-show-file-extensions",
                    "Show File Extensions",
                    "Shows file name extensions in File Explorer.",
                    "Explorer", (int)RiskLevel.Low, 1,
                    JsonConvert.SerializeObject(new[] { "explorer", "recommended" }),
                    null, null,
                    Op("HKCU", @"SOFTWARE\Microsoft\Windows\CurrentVersion\Explorer\Advanced", "HideFileExt", "DWord", "0"),
                    Op("HKCU", @"SOFTWARE\Microsoft\Windows\CurrentVersion\Explorer\Advanced", "HideFileExt", "DWord", "1"),
                    now),
                new TweakRow(
                    "pwr-disable-power-throttling",
                    "Disable Power Throttling",
                    "Prevents Windows from throttling background application CPU usage.",
                    "Power", (int)RiskLevel.Medium, 0,
                    JsonConvert.SerializeObject(new[] { "power", "gaming" }),
                    "May reduce battery life on laptops.",
                    JsonConvert.SerializeObject(new[] { "Battery life on laptops" }),
                    Op("HKLM", @"SYSTEM\CurrentControlSet\Control\Power\PowerThrottling", "PowerThrottlingOff", "DWord", "1"),
                    Op("HKLM", @"SYSTEM\CurrentControlSet\Control\Power\PowerThrottling", "PowerThrottlingOff", "DWord", "0"),
                    now),
            };

            foreach (var row in rows)
            {
                connection.Execute(sql, row);
            }
        }

        private static void SeedDebloatPackages(IDbConnection connection)
        {
            const string sql = @"
                INSERT OR IGNORE INTO DebloatPackages
                    (PackageName, DisplayName, Category, Risk, CanReinstall, StoreId, BreaksFeatures, Status, ProvisionedRemoved, UpdatedAt)
                VALUES
                    (@PackageName, @DisplayName, @Category, @Risk, @CanReinstall, @StoreId, @BreaksFeatures, @Status, @ProvisionedRemoved, @UpdatedAt)";

            var now = DateTime.UtcNow.ToString("o");

            DebloatRow Pkg(string name, string display, string category, int canReinstall = 1, string[]? breaks = null, int status = 0)
                => new DebloatRow(name, display, category, (int)RiskLevel.Low, canReinstall, null,
                    breaks is null ? null : JsonConvert.SerializeObject(breaks), status, 0, now);

            var rows = new[]
            {
                Pkg("Microsoft.BingNews", "Microsoft News", "Microsoft Bloat"),
                Pkg("Microsoft.BingWeather", "Microsoft Weather", "Microsoft Bloat"),
                Pkg("Microsoft.BingSports", "Microsoft Sports", "Microsoft Bloat"),
                Pkg("Microsoft.BingFinance", "Microsoft Finance", "Microsoft Bloat"),
                Pkg("Microsoft.GetHelp", "Get Help", "Microsoft Bloat"),
                Pkg("Microsoft.Getstarted", "Tips", "Microsoft Bloat"),
                Pkg("Microsoft.MicrosoftOfficeHub", "Microsoft Office Hub", "Microsoft Bloat", breaks: new[] { "My Office" }),
                Pkg("Microsoft.MicrosoftSolitaireCollection", "Microsoft Solitaire Collection", "Microsoft Bloat", breaks: new[] { "Solitaire games" }),
                Pkg("Microsoft.People", "Microsoft People", "Microsoft Bloat", breaks: new[] { "People bar", "Mail contact sync" }),
                Pkg("Microsoft.SkypeApp", "Skype", "Microsoft Bloat"),
                Pkg("Microsoft.Todos", "Microsoft To Do", "Microsoft Bloat"),
                Pkg("Microsoft.WindowsAlarms", "Windows Alarms & Clock", "Microsoft Bloat"),
                Pkg("Microsoft.WindowsCamera", "Windows Camera", "Microsoft Bloat"),
                Pkg("Microsoft.WindowsFeedbackHub", "Feedback Hub", "Microsoft Bloat"),
                Pkg("Microsoft.WindowsMaps", "Windows Maps", "Microsoft Bloat"),
                Pkg("Microsoft.WindowsSoundRecorder", "Voice Recorder", "Microsoft Bloat"),
                Pkg("Microsoft.YourPhone", "Phone Link", "Microsoft Bloat", breaks: new[] { "Phone Link", "Phone screen mirroring" }),
                Pkg("Microsoft.XboxGamingOverlay", "Xbox Game Bar", "Gaming", breaks: new[] { "Game Bar overlay", "Game DVR clips" }),
                Pkg("Microsoft.GamingApp", "Xbox App", "Gaming", breaks: new[] { "Xbox app", "Game Pass browsing" }),
                Pkg("Microsoft.Store", "Microsoft Store", "Protected", canReinstall: 0, status: (int)PackageStatus.Protected),
            };

            foreach (var row in rows)
            {
                connection.Execute(sql, row);
            }
        }

        private static void SeedPrivacyRules(IDbConnection connection)
        {
            const string sql = @"
                INSERT OR IGNORE INTO PrivacyRules
                    (Id, Name, Description, Category, Risk, DefaultEnabled, Enabled, UpdatedAt)
                VALUES
                    (@Id, @Name, @Description, @Category, @Risk, @DefaultEnabled, @Enabled, @UpdatedAt)";

            var now = DateTime.UtcNow.ToString("o");

            PrivacyRow Rule(string id, string name, string description, string category, RiskLevel risk, int defaultEnabled = 0)
                => new PrivacyRow(id, name, description, category, (int)risk, defaultEnabled, 0, now);

            var rows = new[]
            {
                Rule("priv-telemetry", "Disable Telemetry Data Collection", "Sets Windows diagnostic data to the minimum (Security) level.", "Data Collection", RiskLevel.Low, 1),
                Rule("priv-advertising-id", "Disable Advertising ID", "Turns off the per-user advertising identifier used for targeted ads.", "Advertising", RiskLevel.Low, 1),
                Rule("priv-tailored-experiences", "Disable Tailored Experiences", "Stops Windows from using diagnostic data to show personalized tips and ads.", "Diagnostics", RiskLevel.Low),
                Rule("priv-feedback-frequency", "Reduce Feedback Frequency", "Stops Windows from periodically requesting feedback.", "Diagnostics", RiskLevel.Low, 1),
                Rule("priv-location-tracking", "Disable Location Tracking", "Turns off the Windows location service for apps and the system.", "Apps", RiskLevel.Low),
                Rule("priv-wifi-sense", "Disable Wi-Fi Sense", "Prevents automatic connection to suggested open hotspots.", "Network", RiskLevel.Low, 1),
                Rule("priv-copilot", "Disable Windows Copilot", "Removes the Copilot icon and disables the Copilot AI component.", "AI", RiskLevel.High),
                Rule("priv-recall", "Disable Windows Recall", "Turns off AI snapshot capture on Copilot+ PCs (Windows 11 24H2+).", "AI", RiskLevel.High),
            };

            foreach (var row in rows)
            {
                connection.Execute(sql, row);
            }
        }

        private static void SeedApplications(IDbConnection connection)
        {
            const string sql = @"
                INSERT OR IGNORE INTO Applications
                    (Id, Name, Publisher, Category, Version, Source, Installed, UpdatedAt)
                VALUES
                    (@Id, @Name, @Publisher, @Category, @Version, @Source, @Installed, @UpdatedAt)";

            var now = DateTime.UtcNow.ToString("o");

            ApplicationRow App(string id, string name, string publisher, string category)
                => new ApplicationRow(id, name, publisher, category, "latest", "winget", 0, now);

            var rows = new[]
            {
                App("Google.Chrome", "Google Chrome", "Google LLC", "Browsers"),
                App("Mozilla.Firefox", "Mozilla Firefox", "Mozilla", "Browsers"),
                App("Microsoft.VisualStudioCode", "Visual Studio Code", "Microsoft", "Development"),
                App("Git.Git", "Git", "Software Freedom Conservancy", "Development"),
                App("VideoLAN.VLC", "VLC Media Player", "VideoLAN", "Media"),
                App("Discord.Discord", "Discord", "Discord Inc.", "Communication"),
                App("Notepad++.Notepad++", "Notepad++", "Don Ho", "Utilities"),
                App("7zip.7zip", "7-Zip", "Igor Pavlov", "Utilities"),
                App("Valve.Steam", "Steam", "Valve Corporation", "Gaming"),
            };

            foreach (var row in rows)
            {
                connection.Execute(sql, row);
            }
        }

        private static void SeedPresets(IDbConnection connection)
        {
            const string sql = @"
                INSERT OR IGNORE INTO Presets
                    (Id, Name, Description, Type, IncludedTweakIds, IncludedPrivacyRuleIds, ExcludedPackageNames, IsProtected, UpdatedAt)
                VALUES
                    (@Id, @Name, @Description, @Type, @IncludedTweakIds, @IncludedPrivacyRuleIds, @ExcludedPackageNames, @IsProtected, @UpdatedAt)";

            var now = DateTime.UtcNow.ToString("o");

            PresetRow Preset(string id, string name, string description, PresetType type,
                string[] tweakIds, string[] privacyRuleIds, string[] excludedPackages, int isProtected = 0)
                => new PresetRow(id, name, description, (int)type,
                    JsonConvert.SerializeObject(tweakIds),
                    JsonConvert.SerializeObject(privacyRuleIds),
                    JsonConvert.SerializeObject(excludedPackages),
                    isProtected, now);

            var rows = new[]
            {
                Preset("preset-standard", "Standard", "Safe, recommended optimizations for everyday use.",
                    PresetType.Standard,
                    new[] { "tel-disable-telemetry", "tel-ceip", "tel-advertising-id", "perf-game-mode", "exp-show-file-extensions" },
                    new[] { "priv-telemetry", "priv-advertising-id", "priv-feedback-frequency", "priv-wifi-sense" },
                    Array.Empty<string>()),
                Preset("preset-gaming", "Gaming", "Performance-focused profile for maximum responsiveness while gaming.",
                    PresetType.Gaming,
                    new[] { "perf-disable-hibernation", "perf-game-mode", "pwr-disable-power-throttling", "exp-show-file-extensions" },
                    Array.Empty<string>(),
                    Array.Empty<string>()),
                Preset("preset-privacy", "Privacy Hardened", "Aggressive lockdown: disables telemetry, AI features, and data collection.",
                    PresetType.Privacy,
                    new[] { "tel-disable-telemetry", "tel-ceip", "tel-advertising-id", "pwr-disable-power-throttling" },
                    new[] { "priv-telemetry", "priv-advertising-id", "priv-tailored-experiences", "priv-feedback-frequency", "priv-location-tracking", "priv-wifi-sense", "priv-copilot", "priv-recall" },
                    Array.Empty<string>()),
                Preset("preset-work", "Work / Corporate", "Conservative changes that avoid breaking enterprise tooling.",
                    PresetType.Work,
                    new[] { "perf-game-mode", "exp-show-file-extensions" },
                    new[] { "priv-telemetry", "priv-feedback-frequency" },
                    Array.Empty<string>(),
                    isProtected: 1),
            };

            foreach (var row in rows)
            {
                connection.Execute(sql, row);
            }
        }
    }
}

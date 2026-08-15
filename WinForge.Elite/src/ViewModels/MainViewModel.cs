using System.Collections.ObjectModel;
using System.Runtime.CompilerServices;
using System.Windows.Input;
using Dapper;
using WinForge.Elite.Data;
using WinForge.Elite.Helpers;
using WinForge.Elite.Models;

namespace WinForge.Elite.ViewModels
{
    /// <summary>Top-level navigation destinations. Module views are added per section in Phase 1 Step 3.</summary>
    public enum AppSection
    {
        Dashboard,
        Tweaks,
        Debloat,
        Privacy,
        Software,
        Presets
    }

    /// <summary>Sidebar entry: navigation target plus a live statistic line computed from the database.</summary>
    public sealed class ModuleSummary
    {
        public AppSection Section { get; init; }
        public string Title { get; init; } = string.Empty;
        public string CountLabel { get; init; } = string.Empty;
    }

    /// <summary>
    /// Main window view model: owns section navigation and the live catalog
    /// summary (module statistics + recent operation history) read from the
    /// local SQLite database.
    /// </summary>
    public sealed class MainViewModel : BaseViewModel
    {
        private static readonly Serilog.ILogger Log = Logging.Logger.GetLogger<MainViewModel>();

        private const int RecentActivityLimit = 8;

        private AppSection _currentSection;

        public MainViewModel()
        {
            AdminStatus = AdminHelper.GetAdminStatus();
            DatabasePath = PathHelper.DatabasePath;

            RefreshCommand = new RelayCommand(_ => _ = InitializeAsync());
            NavigateCommand = new RelayCommand<AppSection>(Navigate);
        }

        public ObservableCollection<ModuleSummary> Modules { get; } = new();

        public ObservableCollection<OperationHistory> RecentActivity { get; } = new();

        /// <summary>Elevation state of the current process ("Administrator" or "Standard User").</summary>
        public string AdminStatus { get; }

        /// <summary>Full path of the local SQLite database backing the app.</summary>
        public string DatabasePath { get; }

        public ICommand RefreshCommand { get; }

        public ICommand NavigateCommand { get; }

        public AppSection CurrentSection
        {
            get => _currentSection;
            private set => SetProperty(ref _currentSection, value);
        }

        public string SectionTitle => TitleFor(CurrentSection);

        /// <summary>False while a refresh is running so the UI can disable the refresh button.</summary>
        public bool CanRefresh => !IsBusy;

        /// <summary>
        /// Loads module statistics and recent activity from the database.
        /// Safe to call from the window Loaded event; exceptions are captured and
        /// surfaced via StatusMessage/ErrorMessage instead of crashing the UI.
        /// </summary>
        public async Task InitializeAsync()
        {
            await RunBusyAsync(LoadCatalogAsync, "Failed to load the catalog from the local database").ConfigureAwait(true);
        }

        public void Navigate(AppSection section)
        {
            if (CurrentSection == section)
            {
                return;
            }

            CurrentSection = section;
            OnPropertyChanged(nameof(SectionTitle));
            Log.Information("Navigated to {Section}", section);
        }

        protected override void OnPropertyChanged([CallerMemberName] string? propertyName = null)
        {
            base.OnPropertyChanged(propertyName);
            if (propertyName == nameof(IsBusy))
            {
                base.OnPropertyChanged(nameof(CanRefresh));
            }
        }

        private static string TitleFor(AppSection section)
        {
            return section switch
            {
                AppSection.Dashboard => "Dashboard",
                AppSection.Tweaks => "System Tweaks",
                AppSection.Debloat => "Debloat",
                AppSection.Privacy => "Privacy Hardening",
                AppSection.Software => "Software Installer",
                AppSection.Presets => "Presets",
                _ => section.ToString()
            };
        }

        private Task LoadCatalogAsync()
        {
            using var connection = DbConnectionFactory.CreateConnection();
            connection.Open();

            long Count(string sql) => connection.ExecuteScalar<long>(sql);

            long tweaksTotal = Count("SELECT COUNT(*) FROM Tweaks");
            long tweaksApplied = Count("SELECT COUNT(*) FROM Tweaks WHERE Applied = 1");
            long debloatTotal = Count("SELECT COUNT(*) FROM DebloatPackages");
            long debloatRemoved = Count("SELECT COUNT(*) FROM DebloatPackages WHERE Status = 1");
            long privacyTotal = Count("SELECT COUNT(*) FROM PrivacyRules");
            long privacyEnabled = Count("SELECT COUNT(*) FROM PrivacyRules WHERE Enabled = 1");
            long appsTotal = Count("SELECT COUNT(*) FROM Applications");
            long appsInstalled = Count("SELECT COUNT(*) FROM Applications WHERE Installed = 1");
            long presetsTotal = Count("SELECT COUNT(*) FROM Presets");
            long restorePoints = Count("SELECT COUNT(*) FROM RestorePoints WHERE IsValid = 1");

            Modules.Clear();
            Modules.Add(new ModuleSummary
            {
                Section = AppSection.Dashboard,
                Title = TitleFor(AppSection.Dashboard),
                CountLabel = $"{restorePoints} valid restore point(s)"
            });
            Modules.Add(new ModuleSummary
            {
                Section = AppSection.Tweaks,
                Title = TitleFor(AppSection.Tweaks),
                CountLabel = $"{tweaksTotal} tweaks · {tweaksApplied} applied"
            });
            Modules.Add(new ModuleSummary
            {
                Section = AppSection.Debloat,
                Title = TitleFor(AppSection.Debloat),
                CountLabel = $"{debloatTotal} packages · {debloatRemoved} removed"
            });
            Modules.Add(new ModuleSummary
            {
                Section = AppSection.Privacy,
                Title = TitleFor(AppSection.Privacy),
                CountLabel = $"{privacyTotal} rules · {privacyEnabled} enabled"
            });
            Modules.Add(new ModuleSummary
            {
                Section = AppSection.Software,
                Title = TitleFor(AppSection.Software),
                CountLabel = $"{appsTotal} apps · {appsInstalled} installed"
            });
            Modules.Add(new ModuleSummary
            {
                Section = AppSection.Presets,
                Title = TitleFor(AppSection.Presets),
                CountLabel = $"{presetsTotal} profiles"
            });

            var activity = connection.Query<OperationHistory>(
                "SELECT * FROM OperationHistory ORDER BY Id DESC LIMIT @Limit",
                new { Limit = RecentActivityLimit }).ToList();

            RecentActivity.Clear();
            foreach (var entry in activity)
            {
                RecentActivity.Add(entry);
            }

            SetStatus($"Catalog loaded: {tweaksTotal} tweaks, {debloatTotal} debloat packages, " +
                      $"{privacyTotal} privacy rules, {appsTotal} applications, {presetsTotal} presets.");
            return Task.CompletedTask;
        }
    }
}

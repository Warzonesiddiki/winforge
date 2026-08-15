using System.Collections.ObjectModel;
using System.Windows.Input;
using Dapper;
using WinForge.Elite.Data;
using WinForge.Elite.Helpers;
using WinForge.Elite.Services;

namespace WinForge.Elite.ViewModels
{
    /// <summary>Top-level navigation destinations.</summary>
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
    /// summary (module statistics) read from the local SQLite database. Each
    /// section hosts its own page view model created by the composition root.
    /// </summary>
    public sealed class MainViewModel : BaseViewModel
    {
        private static readonly Serilog.ILogger Log = Logging.Logger.GetLogger<MainViewModel>();

        private AppSection _currentSection;

        public MainViewModel()
        {
            AdminStatus = AdminHelper.GetAdminStatus();
            DatabasePath = PathHelper.DatabasePath;

            RefreshCommand = new RelayCommand(_ => _ = InitializeAsync());
            NavigateCommand = new RelayCommand<AppSection>(Navigate);
        }

        public ObservableCollection<ModuleSummary> Modules { get; } = new();

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

        /// <summary>The page view model currently shown in the main content area.</summary>
        public BaseViewModel? CurrentPage
        {
            get => _currentPage;
            private set => SetProperty(ref _currentPage, value);
        }

        private BaseViewModel? _currentPage;

        public string SectionTitle => TitleFor(CurrentSection);

        /// <summary>
        /// Loads module statistics from the database. Safe to call from the window
        /// Loaded event; exceptions are captured and surfaced via StatusMessage.
        /// </summary>
        public override async Task InitializeAsync()
        {
            await RunBusyAsync(LoadCatalogAsync, "Failed to load the catalog from the local database").ConfigureAwait(true);
        }

        public void Navigate(AppSection section)
        {
            if (CurrentSection == section && CurrentPage is not null)
            {
                return;
            }

            if (CurrentPage is IDisposable disposable)
            {
                disposable.Dispose();
            }

            CurrentSection = section;
            var page = CreatePage(section);
            CurrentPage = page;
            _ = InitializePageAsync(page);
            Log.Information("Navigated to {Section}", section);
        }

        private static BaseViewModel CreatePage(AppSection section)
        {
            return section switch
            {
                AppSection.Dashboard => AppServices.CreateDashboardViewModel(),
                AppSection.Tweaks => AppServices.CreateTweaksViewModel(),
                AppSection.Debloat => AppServices.CreateDebloatViewModel(),
                AppSection.Privacy => AppServices.CreatePrivacyViewModel(),
                AppSection.Software => AppServices.CreateSoftwareViewModel(),
                AppSection.Presets => AppServices.CreatePresetsViewModel(),
                _ => throw new ArgumentOutOfRangeException(nameof(section), section, "Unknown application section.")
            };
        }

        private async Task InitializePageAsync(BaseViewModel page)
        {
            try
            {
                await page.InitializeAsync();
            }
            catch (Exception ex)
            {
                Log.Error(ex, "Failed to initialize page {Page}", page.GetType().Name);
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

            SetStatus($"Catalog loaded: {tweaksTotal} tweaks, {debloatTotal} debloat packages, " +
                      $"{privacyTotal} privacy rules, {appsTotal} applications, {presetsTotal} presets.");
            return Task.CompletedTask;
        }
    }
}

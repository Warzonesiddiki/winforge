using System.Collections.ObjectModel;
using System.Windows.Input;
using System.Windows.Threading;
using Dapper;
using WinForge.Elite.Data;
using WinForge.Elite.Models;
using WinForge.Elite.Services;

namespace WinForge.Elite.ViewModels
{
    /// <summary>
    /// Dashboard page: live system telemetry (CPU/RAM/uptime/drive), the health
    /// score with category breakdown, and recent activity. Telemetry is sampled
    /// every two seconds from native Windows APIs; the health score is re-evaluated
    /// every thirty seconds.
    /// </summary>
    public sealed class DashboardViewModel : BaseViewModel, IDisposable
    {
        private static readonly TimeSpan TelemetryInterval = TimeSpan.FromSeconds(2);
        private const int HealthReevaluationIntervalTicks = 15; // 15 * 2s = 30s

        private readonly HealthService _health;
        private readonly SystemInfoService _systemInfo;
        private readonly DispatcherTimer _telemetryTimer;
        private int _tickCount;

        public DashboardViewModel(HealthService health, SystemInfoService systemInfo)
        {
            _health = health ?? throw new ArgumentNullException(nameof(health));
            _systemInfo = systemInfo ?? throw new ArgumentNullException(nameof(systemInfo));

            RefreshCommand = new RelayCommand(_ => _ = RefreshAsync());

            _telemetryTimer = new DispatcherTimer { Interval = TelemetryInterval };
            _telemetryTimer.Tick += OnTelemetryTick;
        }

        public ObservableCollection<OperationHistory> RecentActivity { get; } = new();

        public HealthSnapshot? Health { get; private set; }

        public SystemTelemetry? Telemetry { get; private set; }

        public ICommand RefreshCommand { get; }

        public override async Task InitializeAsync()
        {
            await RefreshAsync();
            _telemetryTimer.Start();
        }

        public async Task RefreshAsync()
        {
            await RunBusyAsync(RefreshCoreAsync, "Failed to refresh the dashboard").ConfigureAwait(true);
        }

        private async Task RefreshCoreAsync()
        {
            await EvaluateHealthAsync().ConfigureAwait(true);
            await LoadActivityAsync().ConfigureAwait(true);
            SetStatus($"Health score: {Health?.OverallScore ?? 0}/100 · {RecentActivity.Count} recent operation(s)");
        }

        private async Task EvaluateHealthAsync()
        {
            try
            {
                Health = await _health.EvaluateAsync().ConfigureAwait(true);
                OnPropertyChanged(nameof(Health));
            }
            catch (Exception ex)
            {
                SetError($"Health evaluation failed: {ex.Message}");
            }
        }

        private async Task LoadActivityAsync()
        {
            var rows = await Task.Run(() =>
            {
                using var connection = DbConnectionFactory.CreateConnection();
                connection.Open();
                return connection.Query<OperationHistory>(
                    "SELECT * FROM OperationHistory ORDER BY Id DESC LIMIT @Limit",
                    new { Limit = 8 }).ToList();
            }).ConfigureAwait(true);

            RecentActivity.Clear();
            foreach (var row in rows)
            {
                RecentActivity.Add(row);
            }
        }

        private async void OnTelemetryTick(object? sender, EventArgs e)
        {
            try
            {
                Telemetry = _systemInfo.Sample();
                OnPropertyChanged(nameof(Telemetry));

                _tickCount++;
                if (_tickCount % HealthReevaluationIntervalTicks == 0)
                {
                    await EvaluateHealthAsync().ConfigureAwait(true);
                }
            }
            catch (Exception ex)
            {
                SetError($"Telemetry refresh failed: {ex.Message}");
            }
        }

        public void Dispose()
        {
            _telemetryTimer.Stop();
            _telemetryTimer.Tick -= OnTelemetryTick;
        }
    }
}

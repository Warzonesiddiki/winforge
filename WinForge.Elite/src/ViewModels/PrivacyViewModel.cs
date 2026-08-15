using System.Collections.ObjectModel;
using System.Windows.Input;
using Dapper;
using WinForge.Elite.Data;
using WinForge.Elite.Models;
using WinForge.Elite.Services;

namespace WinForge.Elite.ViewModels
{
    /// <summary>Item wrapper for the privacy rules list.</summary>
    public sealed class PrivacyItem : BaseViewModel
    {
        private bool _isBusy;

        public PrivacyItem(PrivacyRule model)
        {
            Model = model ?? throw new ArgumentNullException(nameof(model));
        }

        public PrivacyRule Model { get; }

        public RiskLevel Risk => Model.Risk;

        public bool IsBusy
        {
            get => _isBusy;
            set
            {
                if (SetProperty(ref _isBusy, value))
                {
                    Refresh();
                }
            }
        }

        public string ToggleText => Model.Enabled ? "Disable" : "Enable";

        public bool CanToggle => !IsBusy;

        public void Refresh()
        {
            OnPropertyChanged(nameof(ToggleText));
            OnPropertyChanged(nameof(CanToggle));
        }
    }

    /// <summary>
    /// Privacy Hardening page: toggles individual privacy rules (real registry
    /// operations with undo) and offers one-click "Harden All" under a single
    /// restore point. The privacy score is the percentage of enabled rules.
    /// </summary>
    public sealed class PrivacyViewModel : BaseViewModel
    {
        private readonly PrivacyService _privacy;
        private readonly RelayCommand<PrivacyItem> _toggleCommand;
        private List<PrivacyItem> _all = new();

        public PrivacyViewModel(PrivacyService privacy)
        {
            _privacy = privacy ?? throw new ArgumentNullException(nameof(privacy));

            RefreshCommand = new RelayCommand(_ => _ = RefreshAsync());
            HardenAllCommand = new RelayCommand(_ => _ = HardenAllAsync(), _ => !IsBusy);
            _toggleCommand = new RelayCommand<PrivacyItem>(item => _ = ToggleAsync(item), item => item is { CanToggle: true });
        }

        public ObservableCollection<PrivacyItem> Items { get; } = new();

        public ICommand RefreshCommand { get; }

        public ICommand HardenAllCommand { get; }

        public ICommand ToggleCommand => _toggleCommand;

        public double PrivacyPercent { get; private set; }

        public int EnabledCount { get; private set; }

        public int TotalCount { get; private set; }

        public override async Task InitializeAsync()
        {
            await RefreshAsync().ConfigureAwait(true);
        }

        public async Task RefreshAsync()
        {
            await RunBusyAsync(RefreshCoreAsync, "Failed to load privacy rules").ConfigureAwait(true);
        }

        private async Task RefreshCoreAsync()
        {
            var rows = await Task.Run(() =>
            {
                using var connection = DbConnectionFactory.CreateConnection();
                connection.Open();
                return connection.Query<PrivacyRule>("SELECT * FROM PrivacyRules ORDER BY Category, Name").ToList();
            }).ConfigureAwait(true);

            _all = rows.Select(r => new PrivacyItem(r)).ToList();
            Items.Clear();
            foreach (var item in _all)
            {
                Items.Add(item);
            }

            RecomputeScore();
            SetStatus($"Loaded {rows.Count} privacy rules.");
        }

        private void RecomputeScore()
        {
            EnabledCount = _all.Count(i => i.Model.Enabled);
            TotalCount = _all.Count;
            PrivacyPercent = TotalCount > 0 ? 100.0 * EnabledCount / TotalCount : 0.0;
            OnPropertyChanged(nameof(PrivacyPercent));
            OnPropertyChanged(nameof(EnabledCount));
            OnPropertyChanged(nameof(TotalCount));
        }

        private async Task ToggleAsync(PrivacyItem item)
        {
            item.IsBusy = true;
            try
            {
                var enable = !item.Model.Enabled;
                var result = await _privacy.SetRuleAsync(item.Model, enable).ConfigureAwait(true);
                if (result.Success)
                {
                    item.Model.Enabled = enable;
                    RecomputeScore();
                    SetStatus(result.Message);
                }
                else
                {
                    SetError(result.Message);
                }
            }
            catch (Exception ex)
            {
                SetError($"Toggle failed: {ex.Message}");
            }
            finally
            {
                item.IsBusy = false;
                item.Refresh();
                _toggleCommand.RaiseCanExecuteChanged();
            }
        }

        private async Task HardenAllAsync()
        {
            await RunBusyAsync(HardenAllCoreAsync, "Failed to harden privacy settings").ConfigureAwait(true);
        }

        private async Task HardenAllCoreAsync()
        {
            var result = await _privacy.HardenAllAsync(_all.Select(i => i.Model).ToList()).ConfigureAwait(true);
            if (result.Success)
            {
                SetStatus(result.Message);
            }
            else
            {
                SetError(result.Message);
            }

            foreach (var item in _all)
            {
                item.Refresh();
            }

            RecomputeScore();
            _toggleCommand.RaiseCanExecuteChanged();
        }
    }
}

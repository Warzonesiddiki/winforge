using System.Collections.ObjectModel;
using System.Windows.Input;
using Dapper;
using WinForge.Elite.Data;
using WinForge.Elite.Models;
using WinForge.Elite.Services;

namespace WinForge.Elite.ViewModels
{
    /// <summary>Item wrapper for the presets list.</summary>
    public sealed class PresetItem : BaseViewModel
    {
        private bool _isBusy;
        private string? _lastResult;

        public PresetItem(Preset model)
        {
            Model = model ?? throw new ArgumentNullException(nameof(model));
        }

        public Preset Model { get; }

        public PresetType Type => Model.Type;

        public bool IsProtected => Model.IsProtected;

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

        public string? LastResult
        {
            get => _lastResult;
            set => SetProperty(ref _lastResult, value);
        }

        public string IncludedSummary => $"{Model.IncludedTweakIds.Count} tweaks · {Model.IncludedPrivacyRuleIds.Count} privacy rules";

        public bool CanApply => !IsBusy;

        public void Refresh()
        {
            OnPropertyChanged(nameof(CanApply));
        }
    }

    /// <summary>
    /// Presets page: one-click profiles that apply their included tweaks and privacy
    /// rules under a single restore point.
    /// </summary>
    public sealed class PresetsViewModel : BaseViewModel
    {
        private readonly PresetService _presets;
        private readonly RelayCommand<PresetItem> _applyCommand;
        private List<Tweak> _tweaks = new();
        private List<PrivacyRule> _privacyRules = new();

        public PresetsViewModel(PresetService presets)
        {
            _presets = presets ?? throw new ArgumentNullException(nameof(presets));

            RefreshCommand = new RelayCommand(_ => _ = RefreshAsync());
            _applyCommand = new RelayCommand<PresetItem>(item => _ = ApplyAsync(item), item => item is { CanApply: true });
        }

        public ObservableCollection<PresetItem> Items { get; } = new();

        public ICommand RefreshCommand { get; }

        public ICommand ApplyCommand => _applyCommand;

        public override async Task InitializeAsync()
        {
            await RefreshAsync().ConfigureAwait(true);
        }

        public async Task RefreshAsync()
        {
            await RunBusyAsync(RefreshCoreAsync, "Failed to load presets").ConfigureAwait(true);
        }

        private async Task RefreshCoreAsync()
        {
            var data = await Task.Run(() =>
            {
                using var connection = DbConnectionFactory.CreateConnection();
                connection.Open();
                var presets = connection.Query<Preset>("SELECT * FROM Presets ORDER BY Name").ToList();
                var tweaks = connection.Query<Tweak>("SELECT * FROM Tweaks").ToList();
                var rules = connection.Query<PrivacyRule>("SELECT * FROM PrivacyRules").ToList();
                return (presets, tweaks, rules);
            }).ConfigureAwait(true);

            _tweaks = data.tweaks;
            _privacyRules = data.rules;

            Items.Clear();
            foreach (var preset in data.presets)
            {
                Items.Add(new PresetItem(preset));
            }

            SetStatus($"Loaded {data.presets.Count} presets.");
        }

        private async Task ApplyAsync(PresetItem item)
        {
            item.IsBusy = true;
            try
            {
                var result = await _presets.ApplyAsync(item.Model, _tweaks, _privacyRules).ConfigureAwait(true);
                item.LastResult = result.Message;
                if (result.Success)
                {
                    SetStatus(result.Message);
                }
                else
                {
                    SetError(result.Message);
                }
            }
            catch (Exception ex)
            {
                item.LastResult = ex.Message;
                SetError($"Preset failed: {ex.Message}");
            }
            finally
            {
                item.IsBusy = false;
                item.Refresh();
                _applyCommand.RaiseCanExecuteChanged();
            }
        }
    }
}

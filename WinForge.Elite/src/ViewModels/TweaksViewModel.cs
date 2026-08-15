using System.Collections.ObjectModel;
using System.Windows.Input;
using Dapper;
using WinForge.Elite.Data;
using WinForge.Elite.Models;
using WinForge.Elite.Services;

namespace WinForge.Elite.ViewModels
{
    /// <summary>Item wrapper for the tweaks list with apply/undo capability state.</summary>
    public sealed class TweakListItem : BaseViewModel
    {
        private bool _isBusy;

        public TweakListItem(Tweak model)
        {
            Model = model ?? throw new ArgumentNullException(nameof(model));
        }

        public Tweak Model { get; }

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

        public bool CanApply => !IsBusy && !Model.Applied;

        public bool CanUndo => !IsBusy && Model.Applied;

        public void Refresh()
        {
            OnPropertyChanged(nameof(CanApply));
            OnPropertyChanged(nameof(CanUndo));
        }
    }

    /// <summary>
    /// System Tweaks page: loads the tweak catalog, filters by category and search
    /// text, and applies/reverts tweaks through the TweakService (restore point +
    /// snapshot + operations + audit + verification).
    /// </summary>
    public sealed class TweaksViewModel : BaseViewModel
    {
        private const string AllCategories = "All";

        private readonly TweakService _tweaks;
        private readonly RelayCommand<TweakListItem> _applyCommand;
        private readonly RelayCommand<TweakListItem> _undoCommand;
        private List<TweakListItem> _all = new();
        private string _filterText = string.Empty;
        private string _selectedCategory = AllCategories;

        public TweaksViewModel(TweakService tweaks)
        {
            _tweaks = tweaks ?? throw new ArgumentNullException(nameof(tweaks));

            RefreshCommand = new RelayCommand(_ => _ = RefreshAsync());
            _applyCommand = new RelayCommand<TweakListItem>(item => _ = ApplyAsync(item), item => item is { CanApply: true });
            _undoCommand = new RelayCommand<TweakListItem>(item => _ = UndoAsync(item), item => item is { CanUndo: true });
        }

        public ObservableCollection<TweakListItem> Items { get; } = new();

        public ObservableCollection<string> Categories { get; } = new();

        public ICommand RefreshCommand { get; }

        public ICommand ApplyCommand => _applyCommand;

        public ICommand UndoCommand => _undoCommand;

        public string FilterText
        {
            get => _filterText;
            set
            {
                if (SetProperty(ref _filterText, value))
                {
                    ApplyFilter();
                }
            }
        }

        public string SelectedCategory
        {
            get => _selectedCategory;
            set
            {
                if (SetProperty(ref _selectedCategory, value))
                {
                    ApplyFilter();
                }
            }
        }

        public override async Task InitializeAsync()
        {
            await RefreshAsync().ConfigureAwait(true);
        }

        public async Task RefreshAsync()
        {
            await RunBusyAsync(RefreshCoreAsync, "Failed to load tweaks").ConfigureAwait(true);
        }

        private async Task RefreshCoreAsync()
        {
            var rows = await Task.Run(() =>
            {
                using var connection = DbConnectionFactory.CreateConnection();
                connection.Open();
                return connection.Query<Tweak>("SELECT * FROM Tweaks ORDER BY Category, Name").ToList();
            }).ConfigureAwait(true);

            Categories.Clear();
            Categories.Add(AllCategories);
            foreach (var category in rows.Select(t => t.Category).Distinct().OrderBy(c => c))
            {
                Categories.Add(category);
            }

            _all = rows.Select(t => new TweakListItem(t)).ToList();
            ApplyFilter();
            SetStatus($"Loaded {rows.Count} tweaks.");
        }

        private void ApplyFilter()
        {
            Items.Clear();
            foreach (var item in _all)
            {
                if (SelectedCategory != AllCategories && item.Model.Category != SelectedCategory)
                {
                    continue;
                }

                if (FilterText.Length > 0 &&
                    !item.Model.Name.Contains(FilterText, StringComparison.OrdinalIgnoreCase) &&
                    !item.Model.Description.Contains(FilterText, StringComparison.OrdinalIgnoreCase))
                {
                    continue;
                }

                Items.Add(item);
            }
        }

        private async Task ApplyAsync(TweakListItem item)
        {
            item.IsBusy = true;
            try
            {
                var result = await _tweaks.ApplyAsync(item.Model).ConfigureAwait(true);
                if (result.Success)
                {
                    item.Model.Applied = true;
                    SetStatus(result.Message);
                }
                else
                {
                    SetError(result.Message);
                }
            }
            catch (Exception ex)
            {
                SetError($"Apply failed: {ex.Message}");
            }
            finally
            {
                item.IsBusy = false;
                item.Refresh();
                _applyCommand.RaiseCanExecuteChanged();
                _undoCommand.RaiseCanExecuteChanged();
            }
        }

        private async Task UndoAsync(TweakListItem item)
        {
            item.IsBusy = true;
            try
            {
                var result = await _tweaks.UndoAsync(item.Model).ConfigureAwait(true);
                if (result.Success)
                {
                    item.Model.Applied = false;
                    SetStatus(result.Message);
                }
                else
                {
                    SetError(result.Message);
                }
            }
            catch (Exception ex)
            {
                SetError($"Undo failed: {ex.Message}");
            }
            finally
            {
                item.IsBusy = false;
                item.Refresh();
                _applyCommand.RaiseCanExecuteChanged();
                _undoCommand.RaiseCanExecuteChanged();
            }
        }
    }
}

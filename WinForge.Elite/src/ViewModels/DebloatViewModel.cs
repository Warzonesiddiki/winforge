using System.Collections.ObjectModel;
using System.Windows.Input;
using Dapper;
using WinForge.Elite.Data;
using WinForge.Elite.Models;
using WinForge.Elite.Services;

namespace WinForge.Elite.ViewModels
{
    /// <summary>Item wrapper for the debloat list with selection and capability state.</summary>
    public sealed class DebloatItem : BaseViewModel
    {
        private bool _isSelected;
        private bool _isBusy;

        public DebloatItem(DebloatPackage package)
        {
            Package = package ?? throw new ArgumentNullException(nameof(package));
        }

        public DebloatPackage Package { get; }

        public RiskLevel Risk => Package.Risk;

        public bool IsSelected
        {
            get => _isSelected;
            set => SetProperty(ref _isSelected, value);
        }

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

        public string StatusText
        {
            get
            {
                return Package.Status switch
                {
                    PackageStatus.Installed => "Installed",
                    PackageStatus.Removed => "Removed",
                    _ => "Protected"
                };
            }
        }

        public bool CanSelect => !IsBusy && Package.Status != PackageStatus.Protected;

        public bool CanRemove => !IsBusy && Package.Status == PackageStatus.Installed;

        public bool CanReinstall => !IsBusy && Package.Status == PackageStatus.Removed && Package.CanReinstall;

        public bool ShowReinstall => Package.Status == PackageStatus.Removed && Package.CanReinstall;

        public void Refresh()
        {
            OnPropertyChanged(nameof(StatusText));
            OnPropertyChanged(nameof(CanSelect));
            OnPropertyChanged(nameof(CanRemove));
            OnPropertyChanged(nameof(CanReinstall));
            OnPropertyChanged(nameof(ShowReinstall));
        }
    }

    /// <summary>
    /// Debloat page: lists the Appx package catalog, supports per-category selection
    /// and batch removal (single restore point), and reinstallation of removed
    /// inbox packages.
    /// </summary>
    public sealed class DebloatViewModel : BaseViewModel
    {
        private const string AllCategories = "All";

        private readonly DebloatService _debloat;
        private List<DebloatItem> _all = new();
        private string _selectedCategory = AllCategories;

        public DebloatViewModel(DebloatService debloat)
        {
            _debloat = debloat ?? throw new ArgumentNullException(nameof(debloat));

            RefreshCommand = new RelayCommand(_ => _ = RefreshAsync());
            RemoveSelectedCommand = new RelayCommand(_ => _ = RemoveSelectedAsync(), _ => !IsBusy);
            SelectAllCommand = new RelayCommand<string>(category => SelectAllInCategory(category));
            ReinstallCommand = new RelayCommand<DebloatItem>(item => _ = ReinstallAsync(item), item => item is { CanReinstall: true });
        }

        public ObservableCollection<DebloatItem> Items { get; } = new();

        public ObservableCollection<string> Categories { get; } = new();

        public ICommand RefreshCommand { get; }

        public ICommand RemoveSelectedCommand { get; }

        public ICommand SelectAllCommand { get; }

        public ICommand ReinstallCommand { get; }

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
            await RunBusyAsync(RefreshCoreAsync, "Failed to load the debloat catalog").ConfigureAwait(true);
        }

        private async Task RefreshCoreAsync()
        {
            var rows = await Task.Run(() =>
            {
                using var connection = DbConnectionFactory.CreateConnection();
                connection.Open();
                return connection.Query<DebloatPackage>("SELECT * FROM DebloatPackages ORDER BY Category, DisplayName").ToList();
            }).ConfigureAwait(true);

            Categories.Clear();
            Categories.Add(AllCategories);
            foreach (var category in rows.Select(p => p.Category).Distinct().OrderBy(c => c))
            {
                Categories.Add(category);
            }

            _all = rows.Select(p => new DebloatItem(p)).ToList();
            ApplyFilter();
            SetStatus($"Loaded {rows.Count} packages.");
        }

        private void ApplyFilter()
        {
            Items.Clear();
            foreach (var item in _all)
            {
                if (SelectedCategory != AllCategories && item.Package.Category != SelectedCategory)
                {
                    continue;
                }

                Items.Add(item);
            }
        }

        private void SelectAllInCategory(string? category)
        {
            var targets = _all.Where(i =>
                i.CanSelect &&
                (string.IsNullOrWhiteSpace(category) || category == AllCategories || i.Package.Category == category));
            foreach (var item in targets)
            {
                item.IsSelected = true;
            }

            SetStatus($"Selected {targets.Count()} removable package(s).");
        }

        private async Task RemoveSelectedAsync()
        {
            var selected = _all.Where(i => i.IsSelected && i.CanRemove).ToList();
            if (selected.Count == 0)
            {
                SetStatus("Select at least one installed package to remove.");
                return;
            }

            foreach (var item in selected)
            {
                item.IsBusy = true;
                item.Refresh();
            }

            try
            {
                var result = await _debloat.RemoveBatchAsync(selected.Select(i => i.Package).ToList()).ConfigureAwait(true);
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
                SetError($"Batch removal failed: {ex.Message}");
            }
            finally
            {
                foreach (var item in selected)
                {
                    item.IsBusy = false;
                    item.Refresh();
                }
            }
        }

        private async Task ReinstallAsync(DebloatItem item)
        {
            item.IsBusy = true;
            item.Refresh();
            try
            {
                var result = await _debloat.ReinstallAsync(item.Package).ConfigureAwait(true);
                if (result.Success)
                {
                    item.Package.Status = PackageStatus.Installed;
                    SetStatus(result.Message);
                }
                else
                {
                    SetError(result.Message);
                }
            }
            catch (Exception ex)
            {
                SetError($"Reinstall failed: {ex.Message}");
            }
            finally
            {
                item.IsBusy = false;
                item.Refresh();
            }
        }
    }
}

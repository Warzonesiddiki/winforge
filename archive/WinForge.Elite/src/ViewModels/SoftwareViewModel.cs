using System.Collections.ObjectModel;
using System.Windows.Input;
using Dapper;
using WinForge.Elite.Data;
using WinForge.Elite.Models;
using WinForge.Elite.Services;

namespace WinForge.Elite.ViewModels
{
    /// <summary>Item wrapper for the software list with selection and install-state tracking.</summary>
    public sealed class SoftwareItem : BaseViewModel
    {
        private bool _isSelected;
        private bool _isBusy;
        private string _statusText = "Not installed";

        public SoftwareItem(Application model)
        {
            Model = model ?? throw new ArgumentNullException(nameof(model));
        }

        public Application Model { get; }

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
            get => _statusText;
            private set
            {
                if (SetProperty(ref _statusText, value))
                {
                    Refresh();
                }
            }
        }

        public bool CanInstall => !IsBusy && !Model.Installed && _statusText == "Not installed";

        public bool CanUninstall => !IsBusy && Model.Installed;

        public void SetStatus(string status)
        {
            StatusText = status;
        }

        public void Refresh()
        {
            OnPropertyChanged(nameof(CanInstall));
            OnPropertyChanged(nameof(CanUninstall));
        }
    }

    /// <summary>
    /// Software Installer page: browse the curated winget catalog, queue a batch
    /// installation (sequential, with per-app status), and uninstall apps.
    /// </summary>
    public sealed class SoftwareViewModel : BaseViewModel
    {
        private const string AllCategories = "All";

        private readonly SoftwareService _software;
        private List<SoftwareItem> _all = new();
        private string _selectedCategory = AllCategories;

        public SoftwareViewModel(SoftwareService software)
        {
            _software = software ?? throw new ArgumentNullException(nameof(software));

            RefreshCommand = new RelayCommand(_ => _ = RefreshAsync());
            InstallSelectedCommand = new RelayCommand(_ => _ = InstallSelectedAsync(), _ => !IsBusy);
            UninstallCommand = new RelayCommand<SoftwareItem>(item => _ = UninstallAsync(item), item => item is { CanUninstall: true });
        }

        public ObservableCollection<SoftwareItem> Items { get; } = new();

        public ObservableCollection<string> Categories { get; } = new();

        public ICommand RefreshCommand { get; }

        public ICommand InstallSelectedCommand { get; }

        public ICommand UninstallCommand { get; }

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
            await RunBusyAsync(RefreshCoreAsync, "Failed to load the software catalog").ConfigureAwait(true);
        }

        private async Task RefreshCoreAsync()
        {
            var rows = await Task.Run(() =>
            {
                using var connection = DbConnectionFactory.CreateConnection();
                connection.Open();
                return connection.Query<Application>("SELECT * FROM Applications ORDER BY Category, Name").ToList();
            }).ConfigureAwait(true);

            Categories.Clear();
            Categories.Add(AllCategories);
            foreach (var category in rows.Select(a => a.Category).Distinct().OrderBy(c => c))
            {
                Categories.Add(category);
            }

            _all = rows.Select(a => new SoftwareItem(a)).ToList();
            ApplyFilter();
            SetStatus($"Loaded {rows.Count} applications.");
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

                Items.Add(item);
            }
        }

        private async Task InstallSelectedAsync()
        {
            var selected = _all.Where(i => i.IsSelected && i.CanInstall).ToList();
            if (selected.Count == 0)
            {
                SetStatus("Select at least one application to install.");
                return;
            }

            var installed = 0;
            string? firstError = null;
            foreach (var item in selected)
            {
                item.IsBusy = true;
                item.SetStatus("Installing…");
                try
                {
                    var result = await _software.InstallAsync(item.Model).ConfigureAwait(true);
                    if (result.Success)
                    {
                        item.Model.Installed = true;
                        item.SetStatus("Installed");
                        installed++;
                    }
                    else
                    {
                        firstError ??= result.Message;
                        item.SetStatus("Failed");
                    }
                }
                catch (Exception ex)
                {
                    firstError ??= ex.Message;
                    item.SetStatus("Failed");
                }
                finally
                {
                    item.IsBusy = false;
                }
            }

            var errorSuffix = firstError is null ? string.Empty : $" First failure: {firstError}";
            SetStatus($"Installed {installed}/{selected.Count} application(s).{errorSuffix}");
        }

        private async Task UninstallAsync(SoftwareItem item)
        {
            item.IsBusy = true;
            item.SetStatus("Uninstalling…");
            try
            {
                var result = await _software.UninstallAsync(item.Model).ConfigureAwait(true);
                if (result.Success)
                {
                    item.Model.Installed = false;
                    item.SetStatus("Not installed");
                    SetStatus(result.Message);
                }
                else
                {
                    item.SetStatus("Installed");
                    SetError(result.Message);
                }
            }
            catch (Exception ex)
            {
                item.SetStatus("Installed");
                SetError($"Uninstall failed: {ex.Message}");
            }
            finally
            {
                item.IsBusy = false;
            }
        }
    }
}

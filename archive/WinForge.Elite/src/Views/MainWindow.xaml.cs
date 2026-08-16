using System.ComponentModel;
using System.Windows;
using System.Windows.Controls;
using WinForge.Elite.ViewModels;

namespace WinForge.Elite.Views
{
    /// <summary>
    /// Main navigation shell: sidebar of modules on the left, section content and
    /// recent activity on the right. DataContext is the <see cref="MainViewModel"/>.
    /// </summary>
    public partial class MainWindow : Window
    {
        private readonly MainViewModel _viewModel;

        public MainWindow()
            : this(new MainViewModel())
        {
        }

        public MainWindow(MainViewModel viewModel)
        {
            InitializeComponent();
            _viewModel = viewModel ?? throw new ArgumentNullException(nameof(viewModel));
            DataContext = viewModel;
        }

        private async void OnLoaded(object sender, RoutedEventArgs e)
        {
            try
            {
                await _viewModel.InitializeAsync();
                ModuleList.SelectedIndex = 0;
            }
            catch (Exception ex)
            {
                Logging.Logger.GetLogger<MainWindow>().Error(ex, "Failed to initialize main window");
                MessageBox.Show(
                    $"Failed to load the WinForge Elite catalog.\n\n{ex.Message}",
                    "WinForge Elite — Initialization Error",
                    MessageBoxButton.OK,
                    MessageBoxImage.Error);
            }
        }

        private void OnModuleSelected(object sender, SelectionChangedEventArgs e)
        {
            if (ModuleList.SelectedItem is ModuleSummary module)
            {
                _viewModel.Navigate(module.Section);
            }
        }

        private void OnClosing(object sender, CancelEventArgs e)
        {
            Logging.Logger.GetLogger<MainWindow>().Information("Main window closing");
        }
    }
}

using System.Windows;

namespace WinForge.Elite
{
    public partial class App : Application
    {
        protected override void OnStartup(StartupEventArgs e)
        {
            base.OnStartup(e);
            
            // Initialize logging
            Logging.Logger.Initialize();
            
            // Check admin privileges
            if (!Helpers.AdminHelper.IsRunningAsAdmin())
            {
                MessageBox.Show(
                    "WinForge Elite requires administrator privileges to function.\n\n" +
                    "Please restart the application as Administrator.",
                    "Administrator Required",
                    MessageBoxButton.OK,
                    MessageBoxImage.Warning
                );
                Shutdown(1);
                return;
            }
            
            // Initialize database
            Data.DbConnectionFactory.Initialize();
            
            // Show main window
            var mainWindow = new Views.MainWindow();
            mainWindow.Show();
        }
    }
}

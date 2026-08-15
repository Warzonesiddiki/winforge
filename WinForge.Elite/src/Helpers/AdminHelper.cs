using System.Security.Principal;

namespace WinForge.Elite.Helpers
{
    public static class AdminHelper
    {
        public static bool IsRunningAsAdmin()
        {
            try
            {
                using var identity = WindowsIdentity.GetCurrent();
                var principal = new WindowsPrincipal(identity);
                return principal.IsInRole(WindowsBuiltInRole.Administrator);
            }
            catch
            {
                return false;
            }
        }
        
        public static void RestartAsAdmin()
        {
            try
            {
                var exeName = System.Diagnostics.Process.GetCurrentProcess().MainModule!.FileName;
                var startInfo = new System.Diagnostics.ProcessStartInfo(exeName)
                {
                    UseShellExecute = true,
                    Verb = "runas",
                    WorkingDirectory = Environment.CurrentDirectory
                };
                
                System.Diagnostics.Process.Start(startInfo);
                System.Diagnostics.Process.GetCurrentProcess().Kill();
            }
            catch (System.ComponentModel.Win32Exception ex) when (ex.NativeErrorCode == 1223)
            {
                // User declined UAC prompt
                throw new InvalidOperationException("Administrator privileges are required to run WinForge Elite.");
            }
        }
        
        public static string GetAdminStatus()
        {
            return IsRunningAsAdmin() ? "Administrator" : "Standard User";
        }
    }
}

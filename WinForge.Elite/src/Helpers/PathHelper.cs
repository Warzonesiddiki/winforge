namespace WinForge.Elite.Helpers
{
    public static class PathHelper
    {
        private static readonly string _baseDirectory;
        private static readonly string _dataDirectory;
        private static readonly string _backupDirectory;
        private static readonly string _tempDirectory;
        
        static PathHelper()
        {
            _baseDirectory = Path.Combine(
                Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
                "WinForge",
                "Elite"
            );
            
            _dataDirectory = Path.Combine(_baseDirectory, "Data");
            _backupDirectory = Path.Combine(_baseDirectory, "Backups");
            _tempDirectory = Path.Combine(_baseDirectory, "Temp");
            
            // Ensure all directories exist
            Directory.CreateDirectory(_baseDirectory);
            Directory.CreateDirectory(_dataDirectory);
            Directory.CreateDirectory(_backupDirectory);
            Directory.CreateDirectory(_tempDirectory);
        }
        
        public static string BaseDirectory => _baseDirectory;
        public static string DataDirectory => _dataDirectory;
        public static string BackupDirectory => _backupDirectory;
        public static string TempDirectory => _tempDirectory;
        
        public static string DatabasePath => Path.Combine(_dataDirectory, "winforge.db");
        
        public static string GetBackupPath(string prefix)
        {
            var timestamp = DateTime.Now.ToString("yyyyMMdd_HHmmss");
            return Path.Combine(_backupDirectory, $"{prefix}_{timestamp}");
        }
        
        public static string GetTempFile(string extension = ".tmp")
        {
            var fileName = $"{Guid.NewGuid()}{extension}";
            return Path.Combine(_tempDirectory, fileName);
        }
        
        public static void CleanupTempFiles(int maxAgeDays = 1)
        {
            try
            {
                var cutoff = DateTime.Now.AddDays(-maxAgeDays);
                var files = Directory.GetFiles(_tempDirectory);
                
                foreach (var file in files)
                {
                    var creationTime = File.GetCreationTime(file);
                    if (creationTime < cutoff)
                    {
                        try
                        {
                            File.Delete(file);
                        }
                        catch (Exception ex)
                        {
                            Logging.Logger.GetLogger<PathHelper>()
                                .Warning(ex, "Failed to delete temp file: {FilePath}", file);
                        }
                    }
                }
            }
            catch (Exception ex)
            {
                Logging.Logger.GetLogger<PathHelper>()
                    .Error(ex, "Failed to cleanup temp files");
            }
        }
    }
}

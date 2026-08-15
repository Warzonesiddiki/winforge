using Microsoft.Data.Sqlite;
using Dapper;
using WinForge.Elite.Models;
using System.Data;

namespace WinForge.Elite.Data
{
    public class DbConnectionFactory
    {
        private static readonly string DbPath = Path.Combine(
            Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
            "WinForge",
            "Elite",
            "winforge.db"
        );
        
        private static readonly string ConnectionString = $"Data Source={DbPath}";
        
        static DbConnectionFactory()
        {
            InitializeDatabase();
        }
        
        public static IDbConnection CreateConnection()
        {
            return new SqliteConnection(ConnectionString);
        }
        
        private static void InitializeDatabase()
        {
            var dir = Path.GetDirectoryName(DbPath);
            if (!string.IsNullOrEmpty(dir) && !Directory.Exists(dir))
            {
                Directory.CreateDirectory(dir);
            }
            
            using var connection = CreateConnection();
            connection.Open();
            
            // Create Tweaks table
            connection.Execute(@"
                CREATE TABLE IF NOT EXISTS Tweaks (
                    Id TEXT PRIMARY KEY,
                    Name TEXT NOT NULL,
                    Description TEXT NOT NULL,
                    Category TEXT NOT NULL,
                    Risk INTEGER NOT NULL,
                    DefaultEnabled INTEGER NOT NULL,
                    Applied INTEGER NOT NULL DEFAULT 0,
                    Tags TEXT,
                    WarningMessage TEXT,
                    BreaksFeatures TEXT,
                    Operations TEXT,
                    UndoOperations TEXT,
                    UpdatedAt TEXT NOT NULL
                )
            ");
            
            // Create DebloatPackages table
            connection.Execute(@"
                CREATE TABLE IF NOT EXISTS DebloatPackages (
                    PackageName TEXT PRIMARY KEY,
                    DisplayName TEXT NOT NULL,
                    Category TEXT NOT NULL,
                    Risk INTEGER NOT NULL,
                    CanReinstall INTEGER NOT NULL,
                    StoreId TEXT,
                    BreaksFeatures TEXT,
                    Status INTEGER NOT NULL,
                    ProvisionedRemoved INTEGER NOT NULL DEFAULT 0,
                    UpdatedAt TEXT NOT NULL
                )
            ");
            
            // Create PrivacyRules table
            connection.Execute(@"
                CREATE TABLE IF NOT EXISTS PrivacyRules (
                    Id TEXT PRIMARY KEY,
                    Name TEXT NOT NULL,
                    Description TEXT NOT NULL,
                    Category TEXT NOT NULL,
                    Risk INTEGER NOT NULL,
                    DefaultEnabled INTEGER NOT NULL,
                    Enabled INTEGER NOT NULL DEFAULT 0,
                    UpdatedAt TEXT NOT NULL
                )
            ");
            
            // Create Applications table
            connection.Execute(@"
                CREATE TABLE IF NOT EXISTS Applications (
                    Id TEXT PRIMARY KEY,
                    Name TEXT NOT NULL,
                    Publisher TEXT NOT NULL,
                    Category TEXT NOT NULL,
                    Version TEXT NOT NULL,
                    Source TEXT NOT NULL,
                    Installed INTEGER NOT NULL DEFAULT 0,
                    UpdatedAt TEXT NOT NULL
                )
            ");
            
            // Create Presets table
            connection.Execute(@"
                CREATE TABLE IF NOT EXISTS Presets (
                    Id TEXT PRIMARY KEY,
                    Name TEXT NOT NULL,
                    Description TEXT NOT NULL,
                    Type INTEGER NOT NULL,
                    IncludedTweakIds TEXT,
                    IncludedPrivacyRuleIds TEXT,
                    ExcludedPackageNames TEXT,
                    IsProtected INTEGER NOT NULL DEFAULT 0,
                    UpdatedAt TEXT NOT NULL
                )
            ");
            
            // Create RestorePoints table
            connection.Execute(@"
                CREATE TABLE IF NOT EXISTS RestorePoints (
                    Id INTEGER PRIMARY KEY AUTOINCREMENT,
                    Name TEXT NOT NULL,
                    Description TEXT NOT NULL,
                    CreatedAt TEXT NOT NULL,
                    SnapshotPath TEXT NOT NULL,
                    IsValid INTEGER NOT NULL DEFAULT 1,
                    DiskSpaceUsed INTEGER NOT NULL DEFAULT 0
                )
            ");
            
            // Create OperationHistory table
            connection.Execute(@"
                CREATE TABLE IF NOT EXISTS OperationHistory (
                    Id INTEGER PRIMARY KEY AUTOINCREMENT,
                    OperationType INTEGER NOT NULL,
                    OperationName TEXT NOT NULL,
                    Details TEXT NOT NULL,
                    UndoPayload TEXT,
                    Success INTEGER NOT NULL DEFAULT 0,
                    ErrorMessage TEXT,
                    ExecutedAt TEXT NOT NULL,
                    RestorePointId INTEGER
                )
            ");
            
            // Create HealthHistory table
            connection.Execute(@"
                CREATE TABLE IF NOT EXISTS HealthHistory (
                    Id INTEGER PRIMARY KEY AUTOINCREMENT,
                    OverallScore INTEGER NOT NULL,
                    SecurityScore INTEGER NOT NULL,
                    PerformanceScore INTEGER NOT NULL,
                    CleanlinessScore INTEGER NOT NULL,
                    PrivacyScore INTEGER NOT NULL,
                    CriticalIssues INTEGER NOT NULL,
                    WarningIssues INTEGER NOT NULL,
                    InfoIssues INTEGER NOT NULL,
                    RecordedAt TEXT NOT NULL
                )
            ");
            
            // Create WindowsServices table
            connection.Execute(@"
                CREATE TABLE IF NOT EXISTS WindowsServices (
                    ServiceName TEXT PRIMARY KEY,
                    DisplayName TEXT NOT NULL,
                    Description TEXT NOT NULL,
                    Category TEXT NOT NULL,
                    Risk INTEGER NOT NULL,
                    DefaultStartup TEXT NOT NULL,
                    CurrentStartup TEXT NOT NULL,
                    IsRunning INTEGER NOT NULL DEFAULT 1,
                    IsCritical INTEGER NOT NULL DEFAULT 0,
                    Dependencies TEXT,
                    UpdatedAt TEXT NOT NULL
                )
            ");
            
            // Create ScheduledTasks table
            connection.Execute(@"
                CREATE TABLE IF NOT EXISTS ScheduledTasks (
                    TaskName TEXT PRIMARY KEY,
                    DisplayName TEXT NOT NULL,
                    Description TEXT NOT NULL,
                    Category TEXT NOT NULL,
                    Risk INTEGER NOT NULL,
                    Enabled INTEGER NOT NULL DEFAULT 1,
                    IsMicrosoft INTEGER NOT NULL DEFAULT 1,
                    Trigger TEXT,
                    UpdatedAt TEXT NOT NULL
                )
            ");
            
            // Create StartupItems table
            connection.Execute(@"
                CREATE TABLE IF NOT EXISTS StartupItems (
                    Id TEXT PRIMARY KEY,
                    Name TEXT NOT NULL,
                    Publisher TEXT NOT NULL,
                    Path TEXT NOT NULL,
                    Location TEXT NOT NULL,
                    Enabled INTEGER NOT NULL DEFAULT 1,
                    IsMicrosoft INTEGER NOT NULL DEFAULT 0,
                    UpdatedAt TEXT NOT NULL
                )
            ");
            
            // Create ContextMenuItems table
            connection.Execute(@"
                CREATE TABLE IF NOT EXISTS ContextMenuItems (
                    Id TEXT PRIMARY KEY,
                    Name TEXT NOT NULL,
                    Description TEXT NOT NULL,
                    RegistryPath TEXT NOT NULL,
                    Enabled INTEGER NOT NULL DEFAULT 1,
                    Risk INTEGER NOT NULL,
                    UpdatedAt TEXT NOT NULL
                )
            ");
            
            // Seed initial data
            SeedData.SeedAll(connection);
        }
    }
}

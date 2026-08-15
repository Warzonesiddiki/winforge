namespace WinForge.Elite.Models
{
    public enum RiskLevel
    {
        Low = 0,
        Medium = 1,
        High = 2,
        Expert = 3
    }
    
    public enum PackageStatus
    {
        Installed = 0,
        Removed = 1,
        Protected = 2
    }
    
    public enum OperationType
    {
        Tweak = 0,
        Debloat = 1,
        Privacy = 2,
        SoftwareInstall = 3,
        SoftwareUninstall = 4,
        Repair = 5,
        Update = 6,
        Service = 7,
        ScheduledTask = 8
    }
    
    public enum PresetType
    {
        Standard = 0,
        Gaming = 1,
        Privacy = 2,
        Work = 3
    }
    
    // Tweaks Entity
    public class Tweak
    {
        public string Id { get; set; } = string.Empty;
        public string Name { get; set; } = string.Empty;
        public string Description { get; set; } = string.Empty;
        public string Category { get; set; } = string.Empty;
        public RiskLevel Risk { get; set; } = RiskLevel.Low;
        public bool DefaultEnabled { get; set; } = false;
        public bool Applied { get; set; } = false;
        public List<string> Tags { get; set; } = new();
        public string? WarningMessage { get; set; }
        public List<string> BreaksFeatures { get; set; } = new();
        public List<string> Operations { get; set; } = new();
        public List<string> UndoOperations { get; set; } = new();
        public DateTime UpdatedAt { get; set; } = DateTime.Now;
    }
    
    // Debloat Package Entity
    public class DebloatPackage
    {
        public string PackageName { get; set; } = string.Empty;
        public string DisplayName { get; set; } = string.Empty;
        public string Category { get; set; } = string.Empty;
        public RiskLevel Risk { get; set; } = RiskLevel.Low;
        public bool CanReinstall { get; set; } = true;
        public string? StoreId { get; set; }
        public List<string> BreaksFeatures { get; set; } = new();
        public PackageStatus Status { get; set; } = PackageStatus.Installed;
        public bool ProvisionedRemoved { get; set; } = false;
        public DateTime UpdatedAt { get; set; } = DateTime.Now;
    }
    
    // Privacy Rule Entity
    public class PrivacyRule
    {
        public string Id { get; set; } = string.Empty;
        public string Name { get; set; } = string.Empty;
        public string Description { get; set; } = string.Empty;
        public string Category { get; set; } = string.Empty;
        public RiskLevel Risk { get; set; } = RiskLevel.Low;
        public bool DefaultEnabled { get; set; } = false;
        public bool Enabled { get; set; } = false;
        public DateTime UpdatedAt { get; set; } = DateTime.Now;
    }
    
    // Application Entity
    public class Application
    {
        public string Id { get; set; } = string.Empty;
        public string Name { get; set; } = string.Empty;
        public string Publisher { get; set; } = string.Empty;
        public string Category { get; set; } = string.Empty;
        public string Version { get; set; } = "latest";
        public string Source { get; set; } = "winget";
        public bool Installed { get; set; } = false;
        public DateTime UpdatedAt { get; set; } = DateTime.Now;
    }
    
    // Preset Entity
    public class Preset
    {
        public string Id { get; set; } = string.Empty;
        public string Name { get; set; } = string.Empty;
        public string Description { get; set; } = string.Empty;
        public PresetType Type { get; set; } = PresetType.Standard;
        public List<string> IncludedTweakIds { get; set; } = new();
        public List<string> IncludedPrivacyRuleIds { get; set; } = new();
        public List<string> ExcludedPackageNames { get; set; } = new();
        public bool IsProtected { get; set; } = false;
        public DateTime UpdatedAt { get; set; } = DateTime.Now;
    }
    
    // Restore Point Entity
    public class RestorePoint
    {
        public int Id { get; set; }
        public string Name { get; set; } = string.Empty;
        public string Description { get; set; } = string.Empty;
        public DateTime CreatedAt { get; set; } = DateTime.Now;
        public string SnapshotPath { get; set; } = string.Empty;
        public bool IsValid { get; set; } = true;
        public long DiskSpaceUsed { get; set; } = 0;
    }
    
    // Operation History Entity
    public class OperationHistory
    {
        public int Id { get; set; }
        public OperationType OperationType { get; set; }
        public string OperationName { get; set; } = string.Empty;
        public string Details { get; set; } = string.Empty;
        public string? UndoPayload { get; set; }
        public bool Success { get; set; } = false;
        public string? ErrorMessage { get; set; }
        public DateTime ExecutedAt { get; set; } = DateTime.Now;
        public int? RestorePointId { get; set; }
    }
    
    // Health History Entity
    public class HealthHistory
    {
        public int Id { get; set; }
        public int OverallScore { get; set; }
        public int SecurityScore { get; set; }
        public int PerformanceScore { get; set; }
        public int CleanlinessScore { get; set; }
        public int PrivacyScore { get; set; }
        public int CriticalIssues { get; set; }
        public int WarningIssues { get; set; }
        public int InfoIssues { get; set; }
        public DateTime RecordedAt { get; set; } = DateTime.Now;
    }
    
    // Windows Service Entity
    public class WindowsService
    {
        public string ServiceName { get; set; } = string.Empty;
        public string DisplayName { get; set; } = string.Empty;
        public string Description { get; set; } = string.Empty;
        public string Category { get; set; } = string.Empty;
        public RiskLevel Risk { get; set; } = RiskLevel.Low;
        public string DefaultStartup { get; set; } = "Automatic";
        public string CurrentStartup { get; set; } = "Automatic";
        public bool IsRunning { get; set; } = true;
        public bool IsCritical { get; set; } = false;
        public List<string> Dependencies { get; set; } = new();
        public DateTime UpdatedAt { get; set; } = DateTime.Now;
    }
    
    // Scheduled Task Entity
    public class ScheduledTask
    {
        public string TaskName { get; set; } = string.Empty;
        public string DisplayName { get; set; } = string.Empty;
        public string Description { get; set; } = string.Empty;
        public string Category { get; set; } = string.Empty;
        public RiskLevel Risk { get; set; } = RiskLevel.Low;
        public bool Enabled { get; set; } = true;
        public bool IsMicrosoft { get; set; } = true;
        public string? Trigger { get; set; }
        public DateTime UpdatedAt { get; set; } = DateTime.Now;
    }
    
    // Startup Item Entity
    public class StartupItem
    {
        public string Id { get; set; } = string.Empty;
        public string Name { get; set; } = string.Empty;
        public string Publisher { get; set; } = string.Empty;
        public string Path { get; set; } = string.Empty;
        public string Location { get; set; } = string.Empty;
        public bool Enabled { get; set; } = true;
        public bool IsMicrosoft { get; set; } = false;
        public DateTime UpdatedAt { get; set; } = DateTime.Now;
    }
    
    // Context Menu Item Entity
    public class ContextMenuItem
    {
        public string Id { get; set; } = string.Empty;
        public string Name { get; set; } = string.Empty;
        public string Description { get; set; } = string.Empty;
        public string RegistryPath { get; set; } = string.Empty;
        public bool Enabled { get; set; } = true;
        public RiskLevel Risk { get; set; } = RiskLevel.Low;
        public DateTime UpdatedAt { get; set; } = DateTime.Now;
    }
    
    // Community Pack Entity
    public class CommunityPack
    {
        public string Id { get; set; } = string.Empty;
        public string Name { get; set; } = string.Empty;
        public string Author { get; set; } = string.Empty;
        public string Description { get; set; } = string.Empty;
        public int DownloadCount { get; set; } = 0;
        public double Rating { get; set; } = 0.0;
        public List<string> IncludedTweakIds { get; set; } = new();
        public List<string> IncludedPrivacyRuleIds { get; set; } = new();
        public List<string> ExcludedPackageNames { get; set; } = new();
        public DateTime CreatedAt { get; set; } = DateTime.Now;
        public DateTime UpdatedAt { get; set; } = DateTime.Now;
    }
}

using WinForge.Elite.ViewModels;

namespace WinForge.Elite.Services
{
    /// <summary>
    /// Composition root: wires the service graph and the view model factories in one
    /// place so every page receives the same singleton service instances.
    /// Services are created in dependency order (registry before tweaks, etc.).
    /// </summary>
    public static class AppServices
    {
        public static RegistryService Registry { get; } = new RegistryService();

        public static PowerShellService PowerShell { get; } = new PowerShellService();

        public static RestorePointService RestorePoints { get; } = new RestorePointService();

        public static SystemInfoService SystemInfo { get; } = new SystemInfoService();

        public static TweakService Tweaks { get; } = new TweakService(Registry, PowerShell, RestorePoints);

        public static PrivacyService Privacy { get; } = new PrivacyService(Tweaks, RestorePoints);

        public static DebloatService Debloat { get; } = new DebloatService(PowerShell, RestorePoints);

        public static SoftwareService Software { get; } = new SoftwareService(PowerShell);

        public static PresetService Presets { get; } = new PresetService(Tweaks, Privacy, RestorePoints);

        public static HealthService Health { get; } = new HealthService(Registry);

        public static DashboardViewModel CreateDashboardViewModel() => new(Health, SystemInfo);

        public static TweaksViewModel CreateTweaksViewModel() => new(Tweaks);

        public static DebloatViewModel CreateDebloatViewModel() => new(Debloat);

        public static PrivacyViewModel CreatePrivacyViewModel() => new(Privacy);

        public static SoftwareViewModel CreateSoftwareViewModel() => new(Software);

        public static PresetsViewModel CreatePresetsViewModel() => new(Presets);
    }
}

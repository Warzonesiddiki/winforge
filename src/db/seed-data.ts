export type Risk = "low" | "medium" | "high" | "expert";

export interface TweakSeed {
  id: string;
  name: string;
  description: string;
  category: string;
  risk: Risk;
  defaultEnabled: boolean;
  tags: string[];
  warningMessage?: string;
  breaksFeatures?: string[];
  operations: string[];
  undoOperations: string[];
}

export const tweaksSeed: TweakSeed[] = [
  // Performance
  t("perf-game-mode", "Enable Game Mode", "Ensures Windows Game Mode is active for reduced background interruptions.", "Performance", "low", true, ["gaming", "recommended"], ["HKCU\\SOFTWARE\\Microsoft\\GameBar: AutoGameModeEnabled = 1"], ["HKCU\\SOFTWARE\\Microsoft\\GameBar: AutoGameModeEnabled = 0"]),
  t("perf-ultimate-power", "Ultimate Performance Power Plan", "Unlocks and activates the hidden Ultimate Performance power plan.", "Performance", "low", false, ["power", "recommended"], ["powercfg -duplicatescheme e9a42b02-d5df-448d-aa00-03f14749eb61"], ["powercfg -delete e9a42b02-d5df-448d-aa00-03f14749eb61"]),
  t("perf-pagefile", "Optimize Pagefile Management", "Sets a system-managed pagefile sized for optimal performance.", "Performance", "medium", false, ["memory"], ["HKLM\\SYSTEM\\CurrentControlSet\\Control\\Session Manager\\Memory Management: PagingFiles = auto"], ["HKLM\\SYSTEM\\CurrentControlSet\\Control\\Session Manager\\Memory Management: PagingFiles = default"]),
  t("perf-8dot3", "Disable NTFS 8.3 Name Generation", "Disables legacy short filename generation for faster NTFS operations on large directories.", "Performance", "medium", false, ["filesystem"], ["fsutil behavior set disable8dot3 1"], ["fsutil behavior set disable8dot3 0"]),
  t("perf-memcompression", "Tune Memory Compression", "Adjusts the memory compression store to reduce CPU overhead on low-core systems.", "Performance", "medium", false, ["memory"], ["Disable-MMAgent -MemoryCompression (native API equivalent)"], ["Enable-MMAgent -MemoryCompression (native API equivalent)"]),
  t("perf-prefetch", "Prefetch / Superfetch Tuning", "Tunes prefetch parameters for SSD-equipped systems.", "Performance", "low", false, ["storage"], ["HKLM\\...\\PrefetchParameters: EnablePrefetcher = 2"], ["HKLM\\...\\PrefetchParameters: EnablePrefetcher = 3"]),
  // Telemetry
  t("tel-disable-telemetry", "Disable Telemetry", "Sets telemetry data collection to Security (minimum) level.", "Telemetry", "low", true, ["privacy", "telemetry", "microsoft", "recommended"], ["HKLM\\SOFTWARE\\Policies\\Microsoft\\Windows\\DataCollection: AllowTelemetry = 0"], ["HKLM\\SOFTWARE\\Policies\\Microsoft\\Windows\\DataCollection: AllowTelemetry = 1"]),
  t("tel-ceip", "Disable Customer Experience Improvement Program", "Opts out of CEIP data collection.", "Telemetry", "low", true, ["privacy", "recommended"], ["HKLM\\SOFTWARE\\Policies\\Microsoft\\SQMClient\\Windows: CEIPEnable = 0"], ["HKLM\\SOFTWARE\\Policies\\Microsoft\\SQMClient\\Windows: CEIPEnable = 1"]),
  t("tel-feedback", "Disable Customer Feedback Requests", "Stops Windows from periodically requesting feedback.", "Telemetry", "low", true, ["privacy"], ["HKCU\\SOFTWARE\\Microsoft\\Siuf\\Rules: NumberOfSIUFInPeriod = 0"], ["HKCU\\SOFTWARE\\Microsoft\\Siuf\\Rules: NumberOfSIUFInPeriod = 1"]),
  t("tel-autologger", "Disable AutoLogger Diagtrack Listener", "Disables the AutoLogger-Diagtrack-Listener scheduled trace on boot.", "Telemetry", "medium", false, ["privacy"], ["Scheduled Task: Microsoft\\Windows\\Autochk\\Proxy -> Disabled"], ["Scheduled Task: Microsoft\\Windows\\Autochk\\Proxy -> Enabled"]),
  t("tel-datacollection-lock", "Lock Data Collection Policy", "Applies group-policy level registry locks preventing telemetry level from being raised.", "Telemetry", "expert", false, ["privacy", "expert"], ["HKLM\\SOFTWARE\\Policies\\Microsoft\\Windows\\DataCollection: MaxTelemetryAllowed = 0"], ["HKLM\\SOFTWARE\\Policies\\Microsoft\\Windows\\DataCollection: MaxTelemetryAllowed = 3"]),
  // UI/UX
  t("ui-search-highlights", "Remove Search Highlights", "Removes promotional highlights from Windows Search box.", "UI", "low", true, ["ui", "recommended"], ["HKCU\\...\\SearchSettings: IsDynamicSearchBoxEnabled = 0"], ["HKCU\\...\\SearchSettings: IsDynamicSearchBoxEnabled = 1"]),
  t("ui-disable-widgets", "Disable Widgets", "Removes the Widgets icon and background service from the taskbar.", "UI", "low", true, ["ui", "recommended"], ["HKLM\\SOFTWARE\\Policies\\Microsoft\\Dsh: AllowNewsAndInterests = 0"], ["HKLM\\SOFTWARE\\Policies\\Microsoft\\Dsh: AllowNewsAndInterests = 1"], undefined, ["Widgets panel", "News feed"]),
  t("ui-disable-copilot", "Disable Copilot", "Removes the Copilot icon and disables the Copilot component.", "UI", "low", true, ["ui", "ai", "recommended"], ["HKLM\\SOFTWARE\\Policies\\Microsoft\\Windows\\WindowsCopilot: TurnOffWindowsCopilot = 1"], ["HKLM\\SOFTWARE\\Policies\\Microsoft\\Windows\\WindowsCopilot: TurnOffWindowsCopilot = 0"], "Disables Windows Copilot AI assistant", ["Copilot AI", "Windows AI features"]),
  t("ui-disable-recommended", "Disable Start Menu Recommended Section", "Hides the recommended/recently used files section in the Start Menu.", "UI", "low", false, ["ui"], ["HKLM\\SOFTWARE\\Policies\\Microsoft\\Windows\\Explorer: HideRecommendedSection = 1"], ["HKLM\\SOFTWARE\\Policies\\Microsoft\\Windows\\Explorer: HideRecommendedSection = 0"]),
  t("ui-classic-context", "Classic Right-Click Context Menu", "Restores the full Windows 10-style context menu in Windows 11.", "UI", "low", false, ["ui"], ["HKCU\\SOFTWARE\\Classes\\CLSID\\{86ca1aa0-34aa-4e8b-a509-50c905bae2a2}\\InprocServer32: (Default) = \"\""], ["HKCU\\SOFTWARE\\Classes\\CLSID\\{86ca1aa0-34aa-4e8b-a509-50c905bae2a2} -> removed"]),
  t("ui-hide-taskview", "Hide Task View Button", "Removes the Task View button from the taskbar.", "UI", "low", false, ["ui"], ["HKCU\\...\\Advanced: ShowTaskViewButton = 0"], ["HKCU\\...\\Advanced: ShowTaskViewButton = 1"]),
  t("ui-disable-snapassist", "Disable Snap Assist Suggestions", "Stops Windows from suggesting other windows to snap alongside.", "UI", "low", false, ["ui", "optional"], ["HKCU\\...\\Explorer\\Advanced: SnapAssist = 0"], ["HKCU\\...\\Explorer\\Advanced: SnapAssist = 1"]),
  // Network
  t("net-disable-llmnr", "Disable LLMNR", "Disables Link-Local Multicast Name Resolution to reduce attack surface.", "Network", "medium", false, ["network", "security"], ["HKLM\\SOFTWARE\\Policies\\Microsoft\\Windows NT\\DNSClient: EnableMulticast = 0"], ["HKLM\\SOFTWARE\\Policies\\Microsoft\\Windows NT\\DNSClient: EnableMulticast = 1"]),
  t("net-disable-netbios", "Disable NetBIOS over TCP/IP", "Disables legacy NetBIOS name resolution on all adapters.", "Network", "medium", false, ["network", "security"], ["WMI Win32_NetworkAdapterConfiguration.SetTcpipNetbios(2)"], ["WMI Win32_NetworkAdapterConfiguration.SetTcpipNetbios(0)"]),
  t("net-doh", "Enable DNS over HTTPS", "Configures DNS-over-HTTPS for supported resolvers.", "Network", "low", false, ["network", "privacy"], ["HKLM\\SYSTEM\\CurrentControlSet\\Services\\Dnscache\\Parameters: EnableAutoDoh = 2"], ["HKLM\\SYSTEM\\CurrentControlSet\\Services\\Dnscache\\Parameters: EnableAutoDoh = 0"]),
  t("net-disable-wifisense", "Disable Wi-Fi Sense", "Disables automatic connection to suggested open hotspots.", "Network", "low", false, ["network", "privacy"], ["HKLM\\SOFTWARE\\Microsoft\\WcmSvc\\wifinetworkmanager\\config: AutoConnectAllowedOEM = 0"], ["HKLM\\SOFTWARE\\Microsoft\\WcmSvc\\wifinetworkmanager\\config: AutoConnectAllowedOEM = 1"]),
  t("net-qos", "Limit QoS Packet Scheduler Reservation", "Reduces the reserved bandwidth percentage from 20% to 0%.", "Network", "medium", false, ["network", "gaming"], ["HKLM\\SOFTWARE\\Policies\\Microsoft\\Windows\\Psched: NonBestEffortLimit = 0"], ["HKLM\\SOFTWARE\\Policies\\Microsoft\\Windows\\Psched: NonBestEffortLimit = 20"]),
  // Services
  t("svc-disable-sysmain", "Disable SysMain (Superfetch)", "Disables SysMain — recommended only for low-RAM or SSD-only systems.", "Services", "medium", false, ["services"], ["Service SysMain -> Disabled"], ["Service SysMain -> Automatic"]),
  t("svc-disable-xbox", "Disable Xbox Services", "Disables Xbox-related background services for non-gamers.", "Services", "low", false, ["services"], ["Service XblAuthManager, XblGameSave, XboxNetApiSvc -> Disabled"], ["Service XblAuthManager, XblGameSave, XboxNetApiSvc -> Manual"], "May affect Xbox Game Bar and achievements", ["Xbox Game Bar", "Xbox achievements", "Game DVR"]),
  t("svc-disable-diagtrack", "Disable DiagTrack Service", "Disables the Connected User Experiences and Telemetry service.", "Services", "medium", true, ["services", "telemetry", "recommended"], ["Service DiagTrack -> Disabled"], ["Service DiagTrack -> Automatic"]),
  t("svc-disable-wmp-net", "Disable WMP Network Sharing Service", "Disables Windows Media Player Network Sharing.", "Services", "low", false, ["services"], ["Service WMPNetworkSvc -> Disabled"], ["Service WMPNetworkSvc -> Manual"]),
  t("svc-disable-fax", "Disable Fax Service", "Disables the legacy Fax service on systems without fax hardware.", "Services", "low", false, ["services"], ["Service Fax -> Disabled"], ["Service Fax -> Manual"]),
  t("svc-disable-mapsbroker", "Disable Downloaded Maps Manager", "Disables MapsBroker for systems that don't use offline maps.", "Services", "low", false, ["services"], ["Service MapsBroker -> Disabled"], ["Service MapsBroker -> Automatic (Delayed)"]),
  // Gaming
  t("game-disable-gamebar", "Disable Game Bar", "Disables the Xbox Game Bar overlay.", "Gaming", "low", false, ["gaming"], ["HKCU\\SOFTWARE\\Microsoft\\GameBar: UseNexusForGameBarEnabled = 0"], ["HKCU\\SOFTWARE\\Microsoft\\GameBar: UseNexusForGameBarEnabled = 1"]),
  t("game-hags", "Hardware-Accelerated GPU Scheduling", "Enables HAGS for supported GPUs to reduce latency.", "Gaming", "medium", false, ["gaming", "performance"], ["HKLM\\SYSTEM\\CurrentControlSet\\Control\\GraphicsDrivers: HwSchMode = 2"], ["HKLM\\SYSTEM\\CurrentControlSet\\Control\\GraphicsDrivers: HwSchMode = 1"]),
  t("game-disable-fso", "Disable Fullscreen Optimizations (Global)", "Disables fullscreen optimizations which can add input latency in some games.", "Gaming", "medium", false, ["gaming"], ["HKCU\\System\\GameConfigStore: GameDVR_FSEBehaviorMode = 2"], ["HKCU\\System\\GameConfigStore: GameDVR_FSEBehaviorMode = 0"]),
  t("game-hpet", "Disable Dynamic Tick / HPET", "Disables the High Precision Event Timer for reduced micro-stutter on some CPUs.", "Gaming", "expert", false, ["gaming", "expert"], ["bcdedit /set disabledynamictick yes", "bcdedit /set useplatformclock false"], ["bcdedit /set disabledynamictick no", "bcdedit /deletevalue useplatformclock"], "CAUTION: May cause instability on some systems. Test thoroughly.", ["System timer accuracy", "Some audio applications"]),
  // Power
  t("pwr-hibernation", "Disable Hibernation", "Disables hibernation and removes hiberfil.sys to reclaim disk space.", "Power", "medium", false, ["power", "storage"], ["powercfg /hibernate off"], ["powercfg /hibernate on"], "Removes hibernate option from power menu", ["Hibernation", "Hybrid Sleep", "Fast Startup (if enabled)"]),
  t("pwr-fast-startup", "Disable Fast Startup", "Disables Fast Startup which can interfere with dual-boot and driver updates.", "Power", "low", false, ["power"], ["HKLM\\SYSTEM\\CurrentControlSet\\Control\\Session Manager\\Power: HiberbootEnabled = 0"], ["HKLM\\SYSTEM\\CurrentControlSet\\Control\\Session Manager\\Power: HiberbootEnabled = 1"]),
  t("pwr-usb-suspend", "Disable USB Selective Suspend", "Prevents USB devices from being suspended to save power, useful for peripherals.", "Power", "low", false, ["power", "gaming"], ["HKLM\\SYSTEM\\CurrentControlSet\\Control\\Power: USBSelectiveSuspend = 0"], ["HKLM\\SYSTEM\\CurrentControlSet\\Control\\Power: USBSelectiveSuspend = 1"]),
  t("pwr-processor-mgmt", "Maximize Processor Power Management", "Sets minimum processor state to reduce ramp-up latency under load.", "Power", "medium", false, ["power", "performance"], ["powercfg -setacvalueindex SCHEME_CURRENT SUB_PROCESSOR PROCTHROTTLEMIN 50"], ["powercfg -setacvalueindex SCHEME_CURRENT SUB_PROCESSOR PROCTHROTTLEMIN 5"]),
  // Explorer
  t("exp-disable-ads", "Disable Explorer Ads & Tips", "Removes suggestions and sync provider ads inside File Explorer.", "Explorer", "low", true, ["explorer", "recommended"], ["HKCU\\...\\Explorer\\Advanced: ShowSyncProviderNotifications = 0"], ["HKCU\\...\\Explorer\\Advanced: ShowSyncProviderNotifications = 1"]),
  t("exp-hide-recent", "Hide Recent Files in Quick Access", "Stops recently used files from appearing in Quick Access.", "Explorer", "low", false, ["explorer", "privacy"], ["HKCU\\...\\Explorer\\Advanced: ShowRecent = 0"], ["HKCU\\...\\Explorer\\Advanced: ShowRecent = 1"]),
  t("exp-classic-paint", "Restore Classic Paint/Notepad", "Reverts to the classic (non-modernized) Paint and Notepad experience where available.", "Explorer", "low", false, ["explorer", "optional"], ["Appx: revert Microsoft.Paint / Microsoft.WindowsNotepad to classic build"], ["Appx: keep modernized Paint / Notepad"]),
  t("exp-remove-onedrive", "Remove OneDrive from Explorer", "Hides the OneDrive entry from File Explorer's navigation pane.", "Explorer", "medium", false, ["explorer"], ["HKCR\\CLSID\\{018D5C66-4533-4307-9B53-224DE2ED1FE6}: System.IsPinnedToNameSpaceTree = 0"], ["HKCR\\CLSID\\{018D5C66-4533-4307-9B53-224DE2ED1FE6}: System.IsPinnedToNameSpaceTree = 1"]),
  t("exp-thumbnail-cache", "Limit Thumbnail Cache Size", "Caps the thumbnail cache database size to reclaim disk space over time.", "Explorer", "low", false, ["explorer", "storage"], ["HKLM\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Explorer: ThumbnailCacheMaxSizeMb = 200"], ["HKLM\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Explorer: ThumbnailCacheMaxSizeMb = default"]),
  // ── AtlasOS / ReviOS / CTT-inspired catalog ──────────────────────────────
  t("perf-disable-lastaccess", "Disable NTFS Last Access Updates", "Stops NTFS from stamping last-access timestamps on every read — reduces disk writes on SSDs.", "Performance", "medium", false, ["storage", "ssd", "performance"], ["fsutil behavior set disablelastaccess 1"], ["fsutil behavior set disablelastaccess 0"]),
  t("perf-large-system-cache", "Enable Large System Cache", "Caches recently used files aggressively in RAM for faster repeat access.", "Performance", "medium", false, ["performance", "memory"], ["HKLM\\SYSTEM\\CurrentControlSet\\Control\\Session Manager\\Memory Management: LargeSystemCache = 1"], ["HKLM\\SYSTEM\\CurrentControlSet\\Control\\Session Manager\\Memory Management: LargeSystemCache = 0"], undefined, ["Server-style workloads"]),
  t("perf-verbose-status", "Enable Verbose Boot Messages", "Shows detailed status messages during boot — useful for diagnosing slow startups.", "Performance", "low", false, ["boot", "diagnostics"], ["HKLM\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Policies\\System: VerboseStatus = 1"], ["HKLM\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Policies\\System: VerboseStatus = 0"]),
  t("perf-tdr-level", "Disable TDR (Timeout Detection & Recovery)", "Disables the GPU driver timeout watchdog — can fix stutters but may freeze the screen on driver hangs.", "Performance", "expert", false, ["gpu", "expert"], ["HKLM\\SYSTEM\\CurrentControlSet\\Control\\GraphicsDrivers: TdrLevel = 0"], ["HKLM\\SYSTEM\\CurrentControlSet\\Control\\GraphicsDrivers: TdrLevel = 3"], "CAUTION: A hung GPU driver will freeze the display instead of recovering.", ["GPU driver crash recovery"]),
  t("perf-mitigations-off", "Disable Spectre/Meltdown Mitigations", "Turns off CPU speculative-execution mitigations for maximum throughput — security trade-off.", "Performance", "expert", false, ["cpu", "expert"], ["HKLM\\SYSTEM\\CurrentControlSet\\Control\\Session Manager\\Memory Management: FeatureSettingsOverride = 3, FeatureSettingsOverrideMask = 3"], ["HKLM\\SYSTEM\\CurrentControlSet\\Control\\Session Manager\\Memory Management: FeatureSettingsOverride = 0, FeatureSettingsOverrideMask = 0"], "SECURITY RISK: Only for trusted machines. Reduces protection against known CPU exploits.", ["CPU exploit mitigations", "Virtualization-based security"]),
  t("game-mmcss-games", "MMCSS Gaming Priority Profile", "Boosts GPU scheduling priority for the Games multimedia class (AtlasOS-style gaming tweak).", "Gaming", "medium", false, ["gaming", "performance"], ["HKLM\\SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion\\Multimedia\\SystemProfile\\Tasks\\Games: GPU Priority = 8, Priority = 6, Scheduling Category = High"], ["HKLM\\SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion\\Multimedia\\SystemProfile\\Tasks\\Games: GPU Priority = 8, Priority = 2, Scheduling Category = Medium"]),
  t("game-disable-powerthrottling", "Disable Power Throttling", "Prevents Windows from throttling background app CPU usage (AtlasOS power tweak).", "Gaming", "medium", false, ["gaming", "power"], ["HKLM\\SYSTEM\\CurrentControlSet\\Control\\Power\\PowerThrottling: PowerThrottlingOff = 1"], ["HKLM\\SYSTEM\\CurrentControlSet\\Control\\Power\\PowerThrottling: PowerThrottlingOff = 0"], undefined, ["Battery life on laptops"]),
  t("net-nagle", "Disable Nagle's Algorithm", "Disables Nagle's algorithm and ACK delay on TCP interfaces — reduces latency in games.", "Network", "medium", false, ["network", "gaming", "latency"], ["HKLM\\SYSTEM\\CurrentControlSet\\Services\\Tcpip\\Parameters\\Interfaces: TcpAckFrequency = 1, TCPNoDelay = 1 (all interfaces)"], ["HKLM\\SYSTEM\\CurrentControlSet\\Services\\Tcpip\\Parameters\\Interfaces: TcpAckFrequency = 2, TCPNoDelay = 0 (all interfaces)"], "May slightly increase network overhead for small packets.", ["Network efficiency on slow connections"]),
  t("net-throttling", "Disable Network Throttling Index", "Raises the network throttling index to unlimited — removes bandwidth reservation for multimedia.", "Network", "low", false, ["network", "performance"], ["HKLM\\SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion\\Multimedia\\SystemProfile: NetworkThrottlingIndex = 0xffffffff"], ["HKLM\\SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion\\Multimedia\\SystemProfile: NetworkThrottlingIndex = 10"]),
  t("svc-disable-wsearch", "Disable Windows Search", "Disables the Windows Search service — frees RAM and disk I/O on systems that don't use search indexing.", "Services", "medium", false, ["services", "performance"], ["Service WSearch -> Disabled"], ["Service WSearch -> Automatic (Delayed)"], "Start menu and File Explorer search will stop working.", ["Start menu search", "File Explorer search", "Outlook search"]),
  t("svc-disable-retaildemo", "Disable Retail Demo Experience", "Disables the RetailDemo service used in store display units.", "Services", "low", false, ["services"], ["Service RetailDemo -> Disabled"], ["Service RetailDemo -> Manual"]),
  t("svc-disable-remoteregistry", "Disable Remote Registry", "Disables the Remote Registry service — a common attack surface on networked machines.", "Services", "medium", false, ["services", "security"], ["Service RemoteRegistry -> Disabled"], ["Service RemoteRegistry -> Manual"], undefined, ["Remote registry administration"]),
  t("svc-disable-dosvc", "Disable Delivery Optimization", "Disables peer-to-peer Windows Update delivery — updates download only from Microsoft.", "Services", "medium", false, ["services", "privacy"], ["Service DoSvc -> Disabled"], ["Service DoSvc -> Manual"], "Updates may download slower without peer caching.", ["Windows Update peer sharing", "LAN update sharing"]),
  t("svc-disable-printspooler", "Disable Print Spooler", "Disables the Print Spooler service — also eliminates the PrintNightmare attack surface.", "Services", "medium", false, ["services", "security"], ["Service Spooler -> Disabled"], ["Service Spooler -> Automatic"], "Printing and printer discovery will stop working.", ["Printing", "Printer discovery", "Fax"]),
  t("svc-disable-cdp", "Disable Connected Devices Platform", "Disables CDPSvc used for Phone Link, nearby sharing, and some casting features.", "Services", "medium", false, ["services", "privacy"], ["Service CDPSvc -> Disabled"], ["Service CDPSvc -> Manual"], undefined, ["Phone Link", "Nearby sharing", "Cast to device"]),
  t("svc-disable-sensors", "Disable Sensor Service", "Disables the Sensor Service — stops sensor data collection on desktops.", "Services", "medium", false, ["services", "privacy"], ["Service SensorService -> Disabled"], ["Service SensorService -> Manual"], undefined, ["Auto-rotation", "Ambient light sensors", "Sensor apps"]),
  t("svc-disable-insider", "Disable Windows Insider Service", "Disables the Windows Insider program service — reduces background polling.", "Services", "low", false, ["services"], ["Service Wisvc -> Disabled"], ["Service Wisvc -> Manual"]),
  t("ui-disable-bing-search", "Disable Bing Search in Start", "Disables Bing web suggestions inside the Start menu search box.", "UI", "low", true, ["privacy", "search", "recommended"], ["HKCU\\Software\\Policies\\Microsoft\\Windows\\Explorer: DisableSearchBoxSuggestions = 1"], ["HKCU\\Software\\Policies\\Microsoft\\Windows\\Explorer: DisableSearchBoxSuggestions = 0"]),
  t("ui-disable-consumer", "Disable Consumer Experiences & Suggestions", "Disables silent app installs, 'Get Office' prompts, and Windows Spotlight suggestions (CTT-style).", "UI", "low", true, ["privacy", "recommended"], ["HKLM\\SOFTWARE\\Policies\\Microsoft\\Windows\\CloudContent: DisableSoftLanding = 1, DisableWindowsConsumerFeatures = 1, DisableWindowsSpotlightFeatures = 1"], ["HKLM\\SOFTWARE\\Policies\\Microsoft\\Windows\\CloudContent: DisableSoftLanding = 0, DisableWindowsConsumerFeatures = 0, DisableWindowsSpotlightFeatures = 0"]),
  t("tel-disable-web-search", "Disable Web Search (Bing)", "Disables the Bing web-search integration in the local search index.", "Telemetry", "low", true, ["privacy", "search", "recommended"], ["HKCU\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Search: BingSearchEnabled = 0"], ["HKCU\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Search: BingSearchEnabled = 1"]),
  t("pwr-storage-sense", "Enable Storage Sense Auto Cleanup", "Automatically cleans temp files and recycle bin contents monthly.", "Power", "low", false, ["storage", "maintenance"], ["HKCU\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\StorageSense\\Parameters\\StoragePolicy: 01 = 1, 2048 = 30"], ["HKCU\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\StorageSense\\Parameters\\StoragePolicy: 01 = 0"]),
  t("exp-disable-gallery", "Disable Gallery in File Explorer", "Hides the new Gallery entry from the File Explorer navigation pane (Windows 11 23H2+).", "Explorer", "low", false, ["explorer", "ui"], ["HKCU\\Software\\Classes\\CLSID\\{e88865ea-0e1c-4e20-9aa6-edcd0212c87c}: System.IsPinnedToNameSpaceTree = 0"], ["HKCU\\Software\\Classes\\CLSID\\{e88865ea-0e1c-4e20-9aa6-edcd0212c87c}: System.IsPinnedToNameSpaceTree = 1"]),
];

function t(
  id: string,
  name: string,
  description: string,
  category: string,
  risk: Risk,
  defaultEnabled: boolean,
  tags: string[],
  operations: string[],
  undoOperations: string[],
  warningMessage?: string,
  breaksFeatures?: string[]
): TweakSeed {
  return {
    id,
    name,
    description,
    category,
    risk,
    defaultEnabled,
    tags,
    operations,
    undoOperations,
    warningMessage,
    breaksFeatures: breaksFeatures ?? [],
  };
}

export interface DebloatSeed {
  packageName: string;
  displayName: string;
  category: string;
  risk: Risk;
  canReinstall: boolean;
  storeId?: string;
}

const debloatRaw: [string, string, string, Risk][] = [
  // Microsoft Bloat
  ["Microsoft.BingNews", "News", "Microsoft Bloat", "low"],
  ["Microsoft.BingWeather", "Weather", "Microsoft Bloat", "low"],
  ["Microsoft.GetHelp", "Get Help", "Microsoft Bloat", "low"],
  ["Microsoft.Getstarted", "Tips", "Microsoft Bloat", "low"],
  ["Microsoft.Microsoft3DViewer", "3D Viewer", "Microsoft Bloat", "low"],
  ["Microsoft.MicrosoftOfficeHub", "Office Hub", "Microsoft Bloat", "low"],
  ["Microsoft.MicrosoftSolitaireCollection", "Solitaire Collection", "Microsoft Bloat", "low"],
  ["Microsoft.MixedReality.Portal", "Mixed Reality Portal", "Microsoft Bloat", "low"],
  ["Microsoft.People", "People", "Microsoft Bloat", "low"],
  ["Microsoft.WindowsAlarms", "Alarms & Clock", "Microsoft Bloat", "low"],
  ["Microsoft.WindowsCommunicationsApps", "Mail and Calendar", "Microsoft Bloat", "medium"],
  ["Microsoft.WindowsFeedbackHub", "Feedback Hub", "Microsoft Bloat", "low"],
  ["Microsoft.WindowsMaps", "Maps", "Microsoft Bloat", "low"],
  ["Microsoft.WindowsSoundRecorder", "Voice Recorder", "Microsoft Bloat", "low"],
  ["Microsoft.Print3D", "Print 3D", "Microsoft Bloat", "low"],
  ["Microsoft.Whiteboard", "Whiteboard", "Microsoft Bloat", "low"],
  ["Microsoft.YourPhone", "Phone Link", "Microsoft Bloat", "medium"],
  ["Microsoft.ZuneMusic", "Media Player (Groove Music)", "Microsoft Bloat", "medium"],
  ["Microsoft.ZuneVideo", "Movies & TV", "Microsoft Bloat", "medium"],
  ["Microsoft.OneConnect", "Mobile Plans", "Microsoft Bloat", "low"],
  ["Microsoft.Wallet", "Microsoft Pay", "Microsoft Bloat", "low"],
  ["Microsoft.PowerAutomateDesktop", "Power Automate Desktop", "Microsoft Bloat", "low"],
  ["Microsoft.Todos", "Microsoft To Do", "Microsoft Bloat", "low"],
  ["Microsoft.549981C3F5F10", "Cortana", "Microsoft Bloat", "medium"],
  ["MicrosoftTeams", "Microsoft Teams (Consumer)", "Microsoft Bloat", "low"],
  ["Microsoft.GamingApp", "Xbox App", "Gaming", "low"],
  // OEM Apps
  ["Dell.SupportAssist", "Dell SupportAssist", "OEM Apps", "low"],
  ["Dell.CustomerConnect", "Dell Customer Connect", "OEM Apps", "low"],
  ["HP.SupportAssistant", "HP Support Assistant", "OEM Apps", "low"],
  ["HP.JumpStart", "HP JumpStart", "OEM Apps", "low"],
  ["Lenovo.Vantage", "Lenovo Vantage", "OEM Apps", "low"],
  ["Lenovo.Utility", "Lenovo Utility", "OEM Apps", "low"],
  ["ASUS.GiftBox", "ASUS GiftBox", "OEM Apps", "low"],
  ["ASUS.LiveUpdate", "ASUS Live Update", "OEM Apps", "medium"],
  ["Acer.CareCenter", "Acer Care Center", "OEM Apps", "low"],
  ["McAfee.LiveSafe", "McAfee LiveSafe (Trial)", "OEM Apps", "low"],
  ["Norton.Security", "Norton Security (Trial)", "OEM Apps", "low"],
  ["Dropbox.Promotion", "Dropbox Promotional Tile", "OEM Apps", "low"],
  ["CyberLink.PowerDirector", "CyberLink PowerDirector (Trial)", "OEM Apps", "low"],
  // Advertising
  ["Microsoft.Advertising.Xaml", "Advertising Library", "Advertising", "low"],
  ["king.com.CandyCrushSaga", "Candy Crush Saga", "Advertising", "low"],
  ["king.com.CandyCrushSodaSaga", "Candy Crush Soda Saga", "Advertising", "low"],
  ["Microsoft.BubbleWitch3Saga", "Bubble Witch 3 Saga", "Advertising", "low"],
  ["Disney.MagicKingdoms", "Disney Magic Kingdoms", "Advertising", "low"],
  ["Gameloft.MarchofEmpires", "March of Empires", "Advertising", "low"],
  ["ZyngaInc.FarmVille2CountryEscape", "FarmVille 2: Country Escape", "Advertising", "low"],
  ["PicsArt.PicsArtPhotoStudio", "PicsArt Photo Studio", "Advertising", "low"],
  ["SpotifyAB.SpotifyMusic", "Spotify (Promoted Tile)", "Advertising", "low"],
  ["TikTok.TikTok", "TikTok", "Advertising", "low"],
  ["Facebook.Facebook", "Facebook", "Advertising", "low"],
  ["Instagram.Instagram", "Instagram", "Advertising", "low"],
  ["LinkedIn.LinkedIn", "LinkedIn", "Advertising", "low"],
  ["Netflix.Netflix", "Netflix (Promoted Tile)", "Advertising", "low"],
  // Gaming
  ["Microsoft.XboxApp", "Xbox Console Companion", "Gaming", "low"],
  ["Microsoft.XboxGameOverlay", "Xbox Game Overlay", "Gaming", "low"],
  ["Microsoft.XboxGamingOverlay", "Xbox Gaming Overlay", "Gaming", "low"],
  ["Microsoft.XboxIdentityProvider", "Xbox Identity Provider", "Gaming", "medium"],
  ["Microsoft.XboxSpeechToTextOverlay", "Xbox Speech To Text Overlay", "Gaming", "low"],
  ["Microsoft.Xbox.TCUI", "Xbox TCUI", "Gaming", "medium"],
  // Social
  ["Microsoft.SkypeApp", "Skype", "Social", "low"],
  ["WhatsApp.WhatsApp", "WhatsApp Desktop (Promoted)", "Social", "low"],
  ["Twitter.Twitter", "Twitter / X (Promoted Tile)", "Social", "low"],
  // Widgets / AI
  ["MicrosoftWindows.Client.WebExperience", "Widgets", "Widgets", "low"],
  ["Microsoft.Windows.Ai.Copilot", "Copilot", "AI/Copilot", "medium"],
  ["Microsoft.Windows.Recall", "Recall (AI Snapshot)", "AI/Copilot", "high"],
  ["Microsoft.Windows.AIFabricService", "AI Fabric Service", "AI/Copilot", "medium"],
  ["Microsoft.Windows.CopilotRuntime", "Copilot Runtime", "AI/Copilot", "medium"],
  // Protected (never removable) — shown greyed out in UI
  ["Microsoft.WindowsStore", "Microsoft Store", "Protected", "expert"],
  ["Microsoft.DesktopAppInstaller", "App Installer", "Protected", "expert"],
  ["Microsoft.UI.Xaml.2.8", "Windows UI Library", "Protected", "expert"],
  ["Microsoft.VCLibs.140.00", "Visual C++ Runtime Framework", "Protected", "expert"],
  ["Windows.CBSPreview", "Component-Based Servicing Preview", "Protected", "expert"],
  // AtlasOS / CTT extended catalog
  ["Clipchamp.Clipchamp", "Clipchamp", "Microsoft Bloat", "low"],
  ["Microsoft.DevHome", "Dev Home", "Microsoft Bloat", "low"],
  ["Microsoft.BingSearch", "Bing Search", "Microsoft Bloat", "low"],
  ["Microsoft.BingSports", "Bing Sports", "Microsoft Bloat", "low"],
  ["Microsoft.BingFinance", "Bing Finance", "Microsoft Bloat", "low"],
  ["Microsoft.MicrosoftStickyNotes", "Sticky Notes", "Microsoft Bloat", "low"],
  ["Microsoft.WindowsCalculator", "Calculator", "Microsoft Bloat", "low"],
  ["Microsoft.WindowsClock", "Clock", "Microsoft Bloat", "low"],
  ["Microsoft.WindowsCamera", "Camera", "Microsoft Bloat", "low"],
  ["Microsoft.Windows.Photos", "Photos", "Microsoft Bloat", "medium"],
  ["Microsoft.MSPaint", "Paint", "Microsoft Bloat", "low"],
  ["Microsoft.Office.OneNote", "OneNote", "Microsoft Bloat", "low"],
  ["MicrosoftCorporationII.QuickAssist", "Quick Assist", "Microsoft Bloat", "low"],
  ["MicrosoftWindows.CrossDevice", "Cross Device Experience (Phone Link)", "Microsoft Bloat", "medium"],
  ["Microsoft.Teams", "Microsoft Teams (New)", "Microsoft Bloat", "low"],
  ["Microsoft.Copilot", "Copilot (Store variant)", "AI/Copilot", "medium"],
  ["Microsoft.MicrosoftMahjong", "Microsoft Mahjong", "Advertising", "low"],
  ["Microsoft.MicrosoftJigsaw", "Microsoft Jigsaw", "Advertising", "low"],
  ["Microsoft.MicrosoftSudoku", "Microsoft Sudoku", "Advertising", "low"],
  ["Microsoft.MicrosoftMinesweeper", "Microsoft Minesweeper", "Advertising", "low"],
];

export const debloatSeed: DebloatSeed[] = debloatRaw.map(([packageName, displayName, category, risk]) => ({
  packageName,
  displayName,
  category,
  risk,
  canReinstall: category !== "Protected",
  storeId: category === "Protected" ? undefined : Math.random().toString(36).slice(2, 12),
}));

export interface PrivacyRuleSeed {
  id: string;
  name: string;
  description: string;
  category: string;
  risk: Risk;
  defaultEnabled: boolean;
}

export const privacySeed: PrivacyRuleSeed[] = [
  // Data Collection
  { id: "priv-telemetry-security", name: "Telemetry Level → Security", description: "Sets diagnostic data collection to the minimum Security level.", category: "Data Collection", risk: "low", defaultEnabled: true },
  { id: "priv-ceip", name: "Disable CEIP", description: "Opts out of the Customer Experience Improvement Program.", category: "Data Collection", risk: "low", defaultEnabled: true },
  { id: "priv-wer", name: "Disable Windows Error Reporting", description: "Prevents crash dumps and error reports from being sent to Microsoft.", category: "Data Collection", risk: "medium", defaultEnabled: false },
  { id: "priv-inking-typing", name: "Disable Inking & Typing Personalization", description: "Stops keystroke and handwriting data from being used for personalization.", category: "Data Collection", risk: "low", defaultEnabled: true },
  { id: "priv-speech", name: "Disable Online Speech Recognition", description: "Disables cloud-based speech recognition processing.", category: "Data Collection", risk: "low", defaultEnabled: false },
  { id: "priv-activity-history", name: "Disable Activity History / Timeline", description: "Stops Windows from collecting and uploading activity history.", category: "Data Collection", risk: "low", defaultEnabled: true },
  // App Permissions
  { id: "priv-perm-camera", name: "Restrict Camera Access", description: "Limits background app access to the camera.", category: "App Permissions", risk: "medium", defaultEnabled: false },
  { id: "priv-perm-mic", name: "Restrict Microphone Access", description: "Limits background app access to the microphone.", category: "App Permissions", risk: "medium", defaultEnabled: false },
  { id: "priv-perm-location", name: "Disable Location Tracking", description: "Turns off system-wide location services for apps.", category: "App Permissions", risk: "medium", defaultEnabled: true },
  { id: "priv-perm-contacts", name: "Restrict Contacts Access", description: "Prevents apps from accessing your contacts.", category: "App Permissions", risk: "low", defaultEnabled: true },
  { id: "priv-perm-calendar", name: "Restrict Calendar Access", description: "Prevents apps from accessing your calendar.", category: "App Permissions", risk: "low", defaultEnabled: false },
  { id: "priv-perm-callhistory", name: "Restrict Call History Access", description: "Prevents apps from accessing call history.", category: "App Permissions", risk: "low", defaultEnabled: true },
  { id: "priv-perm-email", name: "Restrict Email Access", description: "Prevents apps from reading account email.", category: "App Permissions", risk: "low", defaultEnabled: false },
  { id: "priv-perm-messages", name: "Restrict Messaging Access", description: "Prevents apps from reading/sending text messages.", category: "App Permissions", risk: "low", defaultEnabled: true },
  { id: "priv-perm-notifications", name: "Restrict Cross-Device Notification Access", description: "Prevents apps from reading notifications system-wide.", category: "App Permissions", risk: "low", defaultEnabled: false },
  { id: "priv-perm-account", name: "Restrict Account Info Access", description: "Prevents apps from reading name, picture, and account details.", category: "App Permissions", risk: "low", defaultEnabled: false },
  { id: "priv-perm-background", name: "Limit Background Apps", description: "Restricts apps from running and refreshing content in the background.", category: "App Permissions", risk: "low", defaultEnabled: true },
  // Advertising
  { id: "priv-ad-id", name: "Disable Advertising ID", description: "Disables the per-user advertising identifier used for personalized ads.", category: "Advertising", risk: "low", defaultEnabled: true },
  { id: "priv-suggested-content", name: "Disable Suggested Content in Settings", description: "Stops Microsoft suggestions from appearing inside Settings.", category: "Advertising", risk: "low", defaultEnabled: true },
  { id: "priv-tailored-experiences", name: "Disable Tailored Experiences", description: "Stops diagnostic data from being used to tailor tips and recommendations.", category: "Advertising", risk: "low", defaultEnabled: true },
  { id: "priv-start-suggestions", name: "Disable Start Menu Suggestions", description: "Removes suggested apps from appearing in the Start menu.", category: "Advertising", risk: "low", defaultEnabled: true },
  // Microsoft Account
  { id: "priv-local-account", name: "Enforce Local Account Awareness", description: "Flags and reports when a Microsoft Account is used instead of a local account.", category: "Microsoft Account", risk: "medium", defaultEnabled: false },
  { id: "priv-signin-sync", name: "Disable Sign-in Info Sync", description: "Prevents sign-in information from syncing across devices.", category: "Microsoft Account", risk: "low", defaultEnabled: false },
  { id: "priv-cloud-clipboard", name: "Disable Cloud Clipboard Sync", description: "Prevents clipboard content from syncing to the cloud.", category: "Microsoft Account", risk: "low", defaultEnabled: true },
  // Browser Privacy
  { id: "priv-edge-telemetry", name: "Disable Edge Telemetry", description: "Applies registry policy disabling Microsoft Edge diagnostic data collection.", category: "Browser Privacy", risk: "low", defaultEnabled: false },
  { id: "priv-smartscreen", name: "Review SmartScreen Settings", description: "Ensures SmartScreen for apps and files is configured per policy.", category: "Browser Privacy", risk: "low", defaultEnabled: true },
  { id: "priv-dnt", name: "Send Do Not Track Header", description: "Configures the browser to send the Do Not Track header.", category: "Browser Privacy", risk: "low", defaultEnabled: false },
  // Network Privacy
  { id: "priv-net-netbios", name: "Disable NetBIOS", description: "Disables legacy NetBIOS name resolution broadcasting.", category: "Network Privacy", risk: "medium", defaultEnabled: false },
  { id: "priv-net-llmnr", name: "Disable LLMNR", description: "Disables Link-Local Multicast Name Resolution.", category: "Network Privacy", risk: "medium", defaultEnabled: false },
  { id: "priv-net-mdns", name: "Disable mDNS", description: "Disables multicast DNS discovery broadcasts.", category: "Network Privacy", risk: "medium", defaultEnabled: false },
  { id: "priv-net-ncsi", name: "Disable NCSI Active Probing", description: "Stops Windows from actively probing Microsoft servers to test connectivity.", category: "Network Privacy", risk: "low", defaultEnabled: false },
  { id: "priv-net-wcn", name: "Disable Windows Connect Now", description: "Disables the WCN wireless configuration service.", category: "Network Privacy", risk: "low", defaultEnabled: false },
  // Extended privacy rules (AtlasOS / CTT-aligned)
  { id: "priv-wifi-sense", name: "Disable Wi-Fi Sense", description: "Prevents automatic connection to suggested open hotspots.", category: "Network Privacy", risk: "low", defaultEnabled: false },
  { id: "priv-device-metadata", name: "Disable Device Metadata Downloads", description: "Stops Windows from downloading device metadata from Microsoft servers.", category: "Data Collection", risk: "low", defaultEnabled: false },
  { id: "priv-bing-search", name: "Disable Bing Web Search", description: "Removes Bing web results from the local search index.", category: "Browser Privacy", risk: "low", defaultEnabled: true },
  { id: "priv-explorer-ads", name: "Disable Ads in File Explorer", description: "Removes synced provider suggestions and ad tiles from Explorer.", category: "Advertising", risk: "low", defaultEnabled: true },
  { id: "priv-copilot", name: "Disable Copilot AI", description: "Disables the Windows Copilot assistant and its data collection.", category: "Data Collection", risk: "medium", defaultEnabled: false },
  { id: "priv-recall", name: "Disable Recall (AI Snapshots)", description: "Disables AI snapshot capture of screen activity.", category: "Data Collection", risk: "high", defaultEnabled: false },
  { id: "priv-microsoft-store-ads", name: "Disable Store Personalized Ads", description: "Disables personalized ads inside the Microsoft Store.", category: "Advertising", risk: "low", defaultEnabled: true },
  { id: "priv-diagnostic-data-viewer", name: "Restrict Diagnostic Data Viewer", description: "Prevents non-admins from viewing diagnostic data.", category: "Data Collection", risk: "low", defaultEnabled: false },
];

export interface AppSeed {
  id: string;
  name: string;
  publisher: string;
  category: string;
  version: string;
  installed?: boolean;
}

export const appsSeed: AppSeed[] = [
  // Browsers
  { id: "Google.Chrome", name: "Google Chrome", publisher: "Google LLC", category: "Browsers", version: "131.0" },
  { id: "Mozilla.Firefox", name: "Mozilla Firefox", publisher: "Mozilla", category: "Browsers", version: "133.0" },
  { id: "Brave.Brave", name: "Brave Browser", publisher: "Brave Software", category: "Browsers", version: "1.74" },
  { id: "Microsoft.Edge", name: "Microsoft Edge", publisher: "Microsoft", category: "Browsers", version: "131.0", installed: true },
  { id: "Opera.Opera", name: "Opera", publisher: "Opera Software", category: "Browsers", version: "115.0" },
  { id: "VivaldiTechnologies.Vivaldi", name: "Vivaldi", publisher: "Vivaldi Technologies", category: "Browsers", version: "6.9" },
  // Dev Tools
  { id: "Microsoft.VisualStudioCode", name: "Visual Studio Code", publisher: "Microsoft", category: "Dev Tools", version: "1.96" },
  { id: "Git.Git", name: "Git", publisher: "Software Freedom Conservancy", category: "Dev Tools", version: "2.47" },
  { id: "Python.Python.3.12", name: "Python 3.12", publisher: "Python Software Foundation", category: "Dev Tools", version: "3.12.7" },
  { id: "OpenJS.NodeJS.LTS", name: "Node.js LTS", publisher: "OpenJS Foundation", category: "Dev Tools", version: "22.12" },
  { id: "Notepad++.Notepad++", name: "Notepad++", publisher: "Don Ho", category: "Dev Tools", version: "8.7" },
  { id: "WinMerge.WinMerge", name: "WinMerge", publisher: "WinMerge Team", category: "Dev Tools", version: "2.16.44" },
  { id: "Microsoft.WindowsTerminal", name: "Windows Terminal", publisher: "Microsoft", category: "Dev Tools", version: "1.21" },
  { id: "Microsoft.PowerShell", name: "PowerShell 7", publisher: "Microsoft", category: "Dev Tools", version: "7.4" },
  { id: "Docker.DockerDesktop", name: "Docker Desktop", publisher: "Docker Inc.", category: "Dev Tools", version: "4.37" },
  { id: "JetBrains.IntelliJIDEA.Community", name: "IntelliJ IDEA CE", publisher: "JetBrains", category: "Dev Tools", version: "2024.3" },
  { id: "Postman.Postman", name: "Postman", publisher: "Postman Inc.", category: "Dev Tools", version: "11.24" },
  { id: "GitHub.GitHubDesktop", name: "GitHub Desktop", publisher: "GitHub Inc.", category: "Dev Tools", version: "3.4" },
  { id: "Microsoft.DotNet.SDK.8", name: ".NET 8 SDK", publisher: "Microsoft", category: "Dev Tools", version: "8.0.404" },
  { id: "Neovim.Neovim", name: "Neovim", publisher: "Neovim Team", category: "Dev Tools", version: "0.10" },
  // Media
  { id: "VideoLAN.VLC", name: "VLC Media Player", publisher: "VideoLAN", category: "Media", version: "3.0.21" },
  { id: "mpv.net", name: "mpv.net", publisher: "mpv.net", category: "Media", version: "7.1" },
  { id: "HandBrake.HandBrake", name: "HandBrake", publisher: "HandBrake Team", category: "Media", version: "1.9" },
  { id: "GIMP.GIMP", name: "GIMP", publisher: "GIMP Team", category: "Media", version: "2.10.38" },
  { id: "Inkscape.Inkscape", name: "Inkscape", publisher: "Inkscape Project", category: "Media", version: "1.3.2" },
  { id: "ShareX.ShareX", name: "ShareX", publisher: "ShareX Team", category: "Media", version: "16.1" },
  { id: "Audacity.Audacity", name: "Audacity", publisher: "Audacity Team", category: "Media", version: "3.7" },
  { id: "Spotify.Spotify", name: "Spotify", publisher: "Spotify AB", category: "Media", version: "1.2.55" },
  { id: "foobar2000.foobar2000", name: "foobar2000", publisher: "Peter Pawlowski", category: "Media", version: "2.24" },
  { id: "OBSProject.OBSStudio", name: "OBS Studio", publisher: "OBS Project", category: "Media", version: "31.0" },
  { id: "Blender.Blender", name: "Blender", publisher: "Blender Foundation", category: "Media", version: "4.3" },
  { id: "Blackmagic.DaVinciResolve", name: "DaVinci Resolve", publisher: "Blackmagic Design", category: "Media", version: "19.1" },
  // Utilities
  { id: "7zip.7zip", name: "7-Zip", publisher: "Igor Pavlov", category: "Utilities", version: "24.09" },
  { id: "RARLab.WinRAR", name: "WinRAR", publisher: "win.rar GmbH", category: "Utilities", version: "7.10" },
  { id: "voidtools.Everything", name: "Everything", publisher: "voidtools", category: "Utilities", version: "1.4.1" },
  { id: "JAMSoftware.TreeSize", name: "TreeSize Free", publisher: "JAM Software", category: "Utilities", version: "4.7" },
  { id: "CrystalDewWorld.CrystalDiskInfo", name: "CrystalDiskInfo", publisher: "Crystal Dew World", category: "Utilities", version: "9.4" },
  { id: "REALiX.HWiNFO", name: "HWiNFO64", publisher: "REALiX", category: "Utilities", version: "8.02" },
  { id: "CPUID.CPU-Z", name: "CPU-Z", publisher: "CPUID", category: "Utilities", version: "2.14" },
  { id: "TechPowerUp.GPU-Z", name: "GPU-Z", publisher: "TechPowerUp", category: "Utilities", version: "2.61" },
  { id: "AntibodySoftware.WizTree", name: "WizTree", publisher: "Antibody Software", category: "Utilities", version: "4.19" },
  { id: "BulkRenameUtility.BulkRenameUtility", name: "Bulk Rename Utility", publisher: "TGRMN Software", category: "Utilities", version: "4.1" },
  { id: "Microsoft.PowerToys", name: "Microsoft PowerToys", publisher: "Microsoft", category: "Utilities", version: "0.87" },
  { id: "WinDirStat.WinDirStat", name: "WinDirStat", publisher: "WinDirStat Team", category: "Utilities", version: "2.2" },
  { id: "RudyOntheGo.Rufus", name: "Rufus", publisher: "Pete Batard", category: "Utilities", version: "4.6" },
  { id: "Balena.Etcher", name: "balenaEtcher", publisher: "Balena", category: "Utilities", version: "1.19" },
  // Comms
  { id: "Discord.Discord", name: "Discord", publisher: "Discord Inc.", category: "Comms", version: "1.0.9187" },
  { id: "SlackTechnologies.Slack", name: "Slack", publisher: "Slack Technologies", category: "Comms", version: "4.42" },
  { id: "Zoom.Zoom", name: "Zoom", publisher: "Zoom Video Communications", category: "Comms", version: "6.2" },
  { id: "Signal.Signal", name: "Signal", publisher: "Signal Foundation", category: "Comms", version: "7.35" },
  { id: "Telegram.TelegramDesktop", name: "Telegram Desktop", publisher: "Telegram FZ-LLC", category: "Comms", version: "5.10" },
  { id: "Microsoft.Teams", name: "Microsoft Teams", publisher: "Microsoft", category: "Comms", version: "24325" },
  // Security
  { id: "Bitwarden.Bitwarden", name: "Bitwarden", publisher: "Bitwarden Inc.", category: "Security", version: "2024.12" },
  { id: "Malwarebytes.Malwarebytes", name: "Malwarebytes", publisher: "Malwarebytes", category: "Security", version: "5.2" },
  { id: "WinSCP.WinSCP", name: "WinSCP", publisher: "Martin Prikryl", category: "Security", version: "6.3.6" },
  { id: "PuTTY.PuTTY", name: "PuTTY", publisher: "Simon Tatham", category: "Security", version: "0.83" },
  { id: "KeePassXCTeam.KeePassXC", name: "KeePassXC", publisher: "KeePassXC Team", category: "Security", version: "2.7.9" },
  { id: "NordSecurity.NordVPN", name: "NordVPN", publisher: "Nord Security", category: "Security", version: "7.30" },
  // Gaming
  { id: "Valve.Steam", name: "Steam", publisher: "Valve Corporation", category: "Gaming", version: "3.6" },
  { id: "EpicGames.EpicGamesLauncher", name: "Epic Games Launcher", publisher: "Epic Games", category: "Gaming", version: "17.4" },
  { id: "GOG.Galaxy", name: "GOG Galaxy", publisher: "GOG.com", category: "Gaming", version: "2.0.83" },
  { id: "Ubisoft.Connect", name: "Ubisoft Connect", publisher: "Ubisoft", category: "Gaming", version: "165" },
  { id: "ElectronicArts.EADesktop", name: "EA App", publisher: "Electronic Arts", category: "Gaming", version: "13.622" },
  { id: "Playnite.Playnite", name: "Playnite", publisher: "Josef Nemec", category: "Gaming", version: "10.35" },
];

export interface PresetSeed {
  id: string;
  name: string;
  description: string;
  tweakIds: string[];
  debloatPackages: string[];
  privacyRuleIds: string[];
}

export const presetsSeed: PresetSeed[] = [
  {
    id: "standard",
    name: "Standard",
    description: "Safe optimizations suitable for all users — low risk, high impact.",
    tweakIds: tweaksSeed.filter((x) => x.defaultEnabled).map((x) => x.id),
    debloatPackages: debloatSeed.filter((p) => p.category !== "Protected" && p.risk === "low").slice(0, 20).map((p) => p.packageName),
    privacyRuleIds: privacySeed.filter((p) => p.defaultEnabled).map((p) => p.id),
  },
  {
    id: "gaming",
    name: "Gaming",
    description: "Maximizes performance and reduces latency for gaming rigs.",
    tweakIds: tweaksSeed.filter((x) => ["Performance", "Gaming", "Power"].includes(x.category) && x.risk !== "expert").map((x) => x.id),
    debloatPackages: debloatSeed.filter((p) => ["Advertising", "Microsoft Bloat"].includes(p.category) && p.risk !== "expert").slice(0, 15).map((p) => p.packageName),
    privacyRuleIds: [],
  },
  {
    id: "privacy",
    name: "Privacy Hardened",
    description: "Aggressively locks down telemetry, tracking, and data collection.",
    tweakIds: tweaksSeed.filter((x) => (x.category === "Telemetry" || x.tags.includes("privacy")) && x.risk !== "expert").map((x) => x.id),
    debloatPackages: debloatSeed.filter((p) => ["Advertising", "AI/Copilot", "Widgets"].includes(p.category) && p.risk !== "expert").map((p) => p.packageName),
    privacyRuleIds: privacySeed.filter((p) => p.risk !== "expert" && p.risk !== "high").map((p) => p.id),
  },
  {
    id: "work",
    name: "Work / Corporate",
    description: "Balanced profile for managed corporate devices — conservative changes only.",
    tweakIds: tweaksSeed.filter((x) => x.risk === "low" && ["Telemetry", "Explorer", "UI"].includes(x.category)).map((x) => x.id),
    debloatPackages: debloatSeed.filter((p) => ["Microsoft Bloat", "Advertising", "Social"].includes(p.category) && p.risk === "low").map((p) => p.packageName),
    privacyRuleIds: privacySeed.filter((p) => p.category === "Data Collection" || p.category === "Advertising").map((p) => p.id),
  },
];

export const updatesSeed = [
  { id: "kb5034123", title: "2024-12 Cumulative Update for Windows 11", kb: "KB5034123", sizeMb: 620, severity: "Critical", releaseDate: "2024-12-10", installed: false, hidden: false },
  { id: "kb5034441", title: "2024-12 Security Update for .NET Framework", kb: "KB5034441", sizeMb: 45, severity: "Important", releaseDate: "2024-12-10", installed: false, hidden: false },
  { id: "kb5034765", title: "Intel Graphics Driver Update", kb: "KB5034765", sizeMb: 310, severity: "Optional", releaseDate: "2024-12-05", installed: false, hidden: false },
  { id: "kb5033920", title: "2024-11 Cumulative Update for Windows 11", kb: "KB5033920", sizeMb: 580, severity: "Critical", releaseDate: "2024-11-12", installed: true, hidden: false },
  { id: "kb5033921", title: "2024-11 Servicing Stack Update", kb: "KB5033921", sizeMb: 12, severity: "Important", releaseDate: "2024-11-12", installed: true, hidden: false },
  { id: "kb5032190", title: "2024-10 Cumulative Update for Windows 11", kb: "KB5032190", sizeMb: 545, severity: "Critical", releaseDate: "2024-10-08", installed: true, hidden: false },
  { id: "kb5031354", title: "Realtek Audio Driver Update", kb: "KB5031354", sizeMb: 85, severity: "Optional", releaseDate: "2024-10-01", installed: false, hidden: true },
  { id: "kb5030310", title: "Feature Update to Windows 11, version 24H2", kb: "KB5030310", sizeMb: 4200, severity: "Feature", releaseDate: "2024-09-20", installed: false, hidden: false },
  { id: "kb5029351", title: "2024-09 Security Update for Adobe Flash Removal", kb: "KB5029351", sizeMb: 8, severity: "Important", releaseDate: "2024-09-10", installed: true, hidden: false },
  { id: "kb5028185", title: "Precision Touchpad Driver Update", kb: "KB5028185", sizeMb: 22, severity: "Optional", releaseDate: "2024-08-14", installed: false, hidden: false },
];

export interface ServiceSeed {
  id: string;
  displayName: string;
  description: string;
  category: string;
  startType: string;
  status: "Running" | "Stopped";
  risk: Risk;
  protected: boolean;
  recommended: "keep" | "disable" | "manual";
}

// service id, display name, description, category, startType, status, risk, protected, recommended
const serviceRaw: [string, string, string, string, string, "Running" | "Stopped", Risk, boolean, "keep" | "disable" | "manual"][] = [
  // ── Protected (never touch) ──
  ["WinDefend", "Microsoft Defender Antivirus", "Real-time protection against malware and threats.", "Security", "Automatic", "Running", "low", true, "keep"],
  ["SecurityHealthService", "Windows Security Service", "Security center health monitoring.", "Security", "Automatic", "Running", "low", true, "keep"],
  ["wscsvc", "Security Center", "Monitors security health settings.", "Security", "Automatic", "Running", "low", true, "keep"],
  ["MpsSvc", "Windows Defender Firewall", "Host-based firewall protection.", "Security", "Automatic", "Running", "low", true, "keep"],
  ["WdNisSvc", "Defender Network Inspection", "Network inspection for Defender.", "Security", "Automatic", "Running", "low", true, "keep"],
  ["Sense", "Microsoft Defender Advanced Threat Protection", "Endpoint detection and response.", "Security", "Automatic", "Running", "low", true, "keep"],
  ["EventLog", "Windows Event Log", "Core event logging infrastructure.", "System", "Automatic", "Running", "low", true, "keep"],
  ["RpcSs", "Remote Procedure Call", "Core RPC endpoint mapper.", "System", "Automatic", "Running", "low", true, "keep"],
  ["DcomLaunch", "DCOM Server Process Launcher", "COM/DCOM activation.", "System", "Automatic", "Running", "low", true, "keep"],
  // ── Telemetry / data collection ──
  ["DiagTrack", "Connected User Experiences and Telemetry", "Collects and transmits telemetry data.", "Telemetry", "Automatic", "Running", "medium", false, "disable"],
  ["dmwappushservice", "Device Management WAP Push", "Syncs device policies and settings.", "Telemetry", "Automatic", "Running", "medium", false, "disable"],
  ["WerSvc", "Windows Error Reporting", "Sends crash reports to Microsoft.", "Telemetry", "Manual", "Stopped", "low", false, "disable"],
  ["PcaSvc", "Program Compatibility Assistant", "Detects compatibility issues and reports them.", "Telemetry", "Automatic", "Running", "low", false, "disable"],
  ["MessagingService", "Messaging Service", "SMS and messaging sync — unused on desktops.", "Telemetry", "Manual", "Stopped", "low", false, "disable"],
  ["WpcMonSvc", "Parental Controls", "Family safety monitoring.", "Telemetry", "Manual", "Stopped", "low", false, "disable"],
  ["PhoneSvc", "Phone Service", "Phone Link / device sync backend.", "Telemetry", "Manual", "Stopped", "low", false, "disable"],
  // ── Performance / search ──
  ["WSearch", "Windows Search", "Content indexing for search — frees RAM/disk I/O when disabled.", "Performance", "Automatic", "Running", "medium", false, "disable"],
  ["SysMain", "SysMain (Superfetch)", "Prefetches frequently used apps — recommend disable on low-RAM/SSD systems.", "Performance", "Automatic", "Running", "medium", false, "disable"],
  ["FontCache", "Windows Font Cache", "Optimizes font performance.", "System", "Automatic", "Running", "low", false, "keep"],
  ["Themes", "Themes", "Manages visual themes.", "System", "Automatic", "Running", "low", false, "keep"],
  ["TimeBrokerSvc", "Time Broker", "Coordinates background app activity.", "System", "Manual", "Running", "low", false, "keep"],
  // ── Gaming / Xbox ──
  ["XboxGipSvc", "Xbox Accessory Management", "Manages Xbox accessories.", "Gaming", "Manual", "Stopped", "low", false, "disable"],
  ["XblAuthManager", "Xbox Live Auth Manager", "Xbox Live authentication.", "Gaming", "Manual", "Stopped", "low", false, "disable"],
  ["XblGameSave", "Xbox Live Game Save", "Cloud game saves for Xbox titles.", "Gaming", "Manual", "Stopped", "low", false, "disable"],
  ["XboxNetApiSvc", "Xbox Live Networking", "Xbox Live networking services.", "Gaming", "Manual", "Stopped", "low", false, "disable"],
  // ── Unnecessary / legacy ──
  ["Fax", "Fax", "Legacy fax service.", "Legacy", "Manual", "Stopped", "low", false, "disable"],
  ["WMPNetworkSvc", "Windows Media Player Network Sharing", "Shares media libraries over the network.", "Legacy", "Manual", "Stopped", "low", false, "disable"],
  ["MapsBroker", "Downloaded Maps Manager", "Offline map downloads.", "Legacy", "Automatic", "Stopped", "low", false, "disable"],
  ["RetailDemo", "Retail Demo Service", "Store demo mode — irrelevant on consumer PCs.", "Legacy", "Manual", "Stopped", "low", false, "disable"],
  ["StiSvc", "Windows Image Acquisition", "Scanner/camera acquisition.", "Legacy", "Manual", "Stopped", "low", false, "disable"],
  ["RemoteRegistry", "Remote Registry", "Allows remote registry editing — attack surface.", "Security", "Manual", "Stopped", "medium", false, "disable"],
  ["DoSvc", "Delivery Optimization", "Peer-to-peer update sharing.", "Network", "Automatic", "Running", "medium", false, "disable"],
  ["CDPSvc", "Connected Devices Platform", "Phone Link, nearby sharing, casting.", "Network", "Automatic", "Running", "medium", false, "disable"],
  ["SensorService", "Sensor Service", "Sensor data collection — unused on most desktops.", "Privacy", "Manual", "Stopped", "medium", false, "disable"],
  ["Wisvc", "Windows Insider Service", "Insider program enrollment.", "Telemetry", "Manual", "Stopped", "low", false, "disable"],
  ["Spooler", "Print Spooler", "Print management — disable to also eliminate PrintNightmare attack surface.", "System", "Automatic", "Running", "medium", false, "disable"],
  // ── Keep running (essential) ──
  ["Dnscache", "DNS Client", "Local DNS caching.", "Network", "Automatic", "Running", "low", false, "keep"],
  ["Dhcp", "DHCP Client", "IP configuration from DHCP.", "Network", "Automatic", "Running", "low", false, "keep"],
  ["NlaSvc", "Network Location Awareness", "Network location detection.", "Network", "Automatic", "Running", "low", false, "keep"],
  ["AudioSrv", "Windows Audio", "Audio playback.", "System", "Automatic", "Running", "low", false, "keep"],
  ["WlanSvc", "WLAN AutoConfig", "Wi-Fi connectivity.", "Network", "Automatic", "Running", "low", false, "keep"],
  ["wuauserv", "Windows Update", "Installs updates — keep enabled.", "System", "Manual", "Running", "low", false, "keep"],
  ["BITS", "Background Intelligent Transfer", "Transfers files in the background.", "System", "Manual", "Running", "low", false, "keep"],
  ["Schedule", "Task Scheduler", "Runs scheduled tasks — required by Windows.", "System", "Automatic", "Running", "low", false, "keep"],
];

export const servicesSeed: ServiceSeed[] = serviceRaw.map(
  ([id, displayName, description, category, startType, status, risk, isProtected, recommended]) => ({
    id,
    displayName,
    description,
    category,
    startType,
    status,
    risk,
    protected: isProtected,
    recommended,
  })
);

export interface TaskSeed {
  id: string;
  name: string;
  path: string;
  description: string;
  enabled: boolean;
  risk: Risk;
  category: string;
}

export const tasksSeed: TaskSeed[] = [
  { id: "task-appraiser", name: "Microsoft Compatibility Appraiser", path: "\\Microsoft\\Windows\\Application Experience\\Microsoft Compatibility Appraiser", description: "Telemetry collection of app and hardware inventory.", enabled: true, risk: "medium", category: "Telemetry" },
  { id: "task-programdataupdater", name: "ProgramDataUpdater", path: "\\Microsoft\\Windows\\Application Experience\\ProgramDataUpdater", description: "Uploads compatibility data to Microsoft.", enabled: true, risk: "medium", category: "Telemetry" },
  { id: "task-consolidator", name: "Consolidator", path: "\\Microsoft\\Windows\\Customer Experience Improvement Program\\Consolidator", description: "CEIP data consolidation.", enabled: true, risk: "medium", category: "Telemetry" },
  { id: "task-usbceip", name: "UsbCeip", path: "\\Microsoft\\Windows\\Customer Experience Improvement Program\\UsbCeip", description: "USB device telemetry collection.", enabled: true, risk: "medium", category: "Telemetry" },
  { id: "task-kernelceip", name: "KernelCeipTask", path: "\\Microsoft\\Windows\\Customer Experience Improvement Program\\KernelCeipTask", description: "Kernel telemetry reporting.", enabled: true, risk: "medium", category: "Telemetry" },
  { id: "task-diskdiag", name: "DiskDiagnosticDataCollector", path: "\\Microsoft\\Windows\\DiskDiagnostic\\Microsoft-Windows-DiskDiagnosticDataCollector", description: "Sends disk health telemetry.", enabled: true, risk: "low", category: "Telemetry" },
  { id: "task-queuereporting", name: "QueueReporting", path: "\\Microsoft\\Windows\\Windows Error Reporting\\QueueReporting", description: "Queues and sends error reports.", enabled: true, risk: "medium", category: "Telemetry" },
  { id: "task-dmclient", name: "DmClient", path: "\\Microsoft\\Windows\\Feedback\\Siuf\\DmClient", description: "Feedback request handling.", enabled: true, risk: "low", category: "Telemetry" },
  { id: "task-location", name: "Notifications (Location)", path: "\\Microsoft\\Windows\\Location\\Notifications", description: "Location-based notifications.", enabled: true, risk: "low", category: "Privacy" },
  { id: "task-officetelemetry", name: "OfficeTelemetryAgentLogOn", path: "\\Microsoft\\Office\\OfficeTelemetryAgentLogOn", description: "Office telemetry upload at logon.", enabled: true, risk: "low", category: "Telemetry" },
  { id: "task-startupapptask", name: "StartupAppTask", path: "\\Microsoft\\Windows\\Application Experience\\StartupAppTask", description: "Startup app evaluation.", enabled: true, risk: "low", category: "System" },
  { id: "task-winsat", name: "WinSAT", path: "\\Microsoft\\Windows\\Maintenance\\WinSAT", description: "Windows System Assessment Tool.", enabled: true, risk: "low", category: "System" },
  { id: "task-scheduleddefrag", name: "ScheduledDefrag", path: "\\Microsoft\\Windows\\Defrag\\ScheduledDefrag", description: "Automatic drive optimization.", enabled: true, risk: "low", category: "Maintenance" },
  { id: "task-update-scheduled", name: "Scheduled Start (Windows Update)", path: "\\Microsoft\\Windows\\WindowsUpdate\\Scheduled Start", description: "Automatic update installation.", enabled: true, risk: "low", category: "Maintenance" },
  { id: "task-autochk", name: "Proxy (Autochk)", path: "\\Microsoft\\Windows\\Autochk\\Proxy", description: "Disk check scheduling on boot.", enabled: true, risk: "low", category: "System" },
];

// Health history: plausible improvement trend ending at today's modeled score.
// Scores climb as optimizations are applied over the last 24 hours.
export const healthHistorySeed = (() => {
  const scores = [42, 43, 45, 46, 48, 49, 51, 52, 53, 55, 56, 57, 58, 59, 60, 61, 61, 62, 62, 63, 63, 64, 64, 64];
  const privacyScores = [30, 31, 33, 34, 36, 37, 38, 39, 40, 41, 42, 43, 43, 44, 44, 45, 45, 46, 46, 47, 47, 47, 47, 47];
  const now = Date.now();
  return scores.map((score, i) => {
    const ageHours = scores.length - 1 - i;
    return {
      timestamp: new Date(now - ageHours * 3600_000),
      score,
      privacyScore: privacyScores[i],
      bloatCount: 92 - Math.round((i / scores.length) * 5),
      appliedTweaks: 5 + Math.round((i / scores.length) * 4),
      pendingUpdates: 5,
    };
  });
})();

export const startupItemsSeed = [
  { id: "su-onedrive", name: "Microsoft OneDrive", publisher: "Microsoft Corporation", command: "\"C:\\Program Files\\Microsoft OneDrive\\OneDrive.exe\" /background", impact: "medium", enabled: true },
  { id: "su-teams", name: "Microsoft Teams", publisher: "Microsoft Corporation", command: "\"C:\\Users\\...\\Teams.exe\" --processStart Teams.exe", impact: "high", enabled: true },
  { id: "su-spotify", name: "Spotify", publisher: "Spotify AB", command: "\"C:\\Users\\...\\Spotify.exe\" /uri spotify:autostart", impact: "low", enabled: true },
  { id: "su-discord", name: "Discord", publisher: "Discord Inc.", command: "\"C:\\Users\\...\\Discord\\Update.exe\" --processStart Discord.exe", impact: "medium", enabled: true },
  { id: "su-steam", name: "Steam Client Bootstrapper", publisher: "Valve Corporation", command: "\"C:\\Program Files (x86)\\Steam\\Steam.exe\" -silent", impact: "medium", enabled: true },
  { id: "su-nvidia", name: "NVIDIA GeForce Experience", publisher: "NVIDIA Corporation", command: "\"C:\\Program Files\\NVIDIA Corporation\\NVIDIA GeForce Experience\\NVIDIA GeForce Experience.exe\"", impact: "medium", enabled: true },
  { id: "su-adobe", name: "Adobe Creative Cloud", publisher: "Adobe Inc.", command: "\"C:\\Program Files\\Adobe\\Adobe Creative Cloud\\ACC\\Creative Cloud.exe\"", impact: "high", enabled: false },
  { id: "su-dropbox", name: "Dropbox", publisher: "Dropbox Inc.", command: "\"C:\\Program Files (x86)\\Dropbox\\Client\\Dropbox.exe\" /systemstartup", impact: "low", enabled: false },
  { id: "su-cortana", name: "Cortana", publisher: "Microsoft Corporation", command: "Cortana.exe -RunFromStartup", impact: "medium", enabled: false },
  { id: "su-realtek", name: "Realtek HD Audio Manager", publisher: "Realtek Semiconductor", command: "RtkNGUI64.exe -s", impact: "low", enabled: true },
];

export interface ContextMenuItemSeed {
  id: string;
  title: string;
  description: string;
  registryKey: string;
  targetExtension: string;
  enabled: boolean;
  risk: Risk;
  category: string;
}

export const contextMenuItemsSeed: ContextMenuItemSeed[] = [
  { id: "cm-clipchamp", title: "Edit with Clipchamp", description: "Removes Clipchamp video editor from right-click context menu.", registryKey: "HKCR\\*\\shell\\Clipchamp.EditWithClipchamp", targetExtension: "Media / *", enabled: true, risk: "low", category: "Microsoft Junk" },
  { id: "cm-paint3d", title: "Edit with Paint 3D", description: "Removes Edit with Paint 3D from image file context menu.", registryKey: "HKCR\\SystemFileAssociations\\image\\Shell\\3D Edit", targetExtension: "Images", enabled: true, risk: "low", category: "Microsoft Junk" },
  { id: "cm-print3d", title: "3D Print with 3D Builder", description: "Removes 3D Print from context menu.", registryKey: "HKCR\\SystemFileAssociations\\.3mf\\Shell\\TPrint", targetExtension: "3D Files", enabled: true, risk: "low", category: "Microsoft Junk" },
  { id: "cm-skype-share", title: "Share with Skype", description: "Removes Share with Skype from file and directory context menus.", registryKey: "HKCR\\*\\shellex\\ContextMenuHandlers\\SharingPrivate", targetExtension: "*", enabled: true, risk: "low", category: "Social / Sharing" },
  { id: "cm-cast-device", title: "Cast to Device", description: "Removes Play to / Cast to Device menu item.", registryKey: "HKCR\\Directory\\shellex\\ContextMenuHandlers\\PlayTo", targetExtension: "Media / Dirs", enabled: true, risk: "low", category: "Network" },
  { id: "cm-give-access", title: "Give access to", description: "Removes legacy network sharing sub-menu from context menu.", registryKey: "HKCR\\*\\shellex\\ContextMenuHandlers\\Sharing", targetExtension: "*", enabled: true, risk: "low", category: "Windows Shell" },
  { id: "cm-include-library", title: "Include in library", description: "Removes Include in Library from folder context menus.", registryKey: "HKCR\\Folder\\ShellEx\\ContextMenuHandlers\\Library Location", targetExtension: "Folders", enabled: true, risk: "low", category: "Windows Shell" },
  { id: "cm-restore-versions", title: "Restore previous versions", description: "Removes Restore previous versions tab/entry.", registryKey: "HKCR\\AllFilesystemObjects\\shellex\\ContextMenuHandlers\\PreviousVersions", targetExtension: "All Objects", enabled: true, risk: "medium", category: "Backup / History" },
  { id: "cm-troubleshoot-compat", title: "Troubleshoot compatibility", description: "Removes Troubleshoot compatibility from executable files.", registryKey: "HKCR\\exefile\\shellex\\ContextMenuHandlers\\Compatibility", targetExtension: ".exe / .msi", enabled: true, risk: "low", category: "Windows Shell" },
  { id: "cm-pin-start", title: "Pin to Start", description: "Removes Pin to Start clutter from file context menus.", registryKey: "HKCR\\*\\shellex\\ContextMenuHandlers\\{a2a9545d-a0c2-42b4-9708-a0b2badd77c8}", targetExtension: "*", enabled: true, risk: "low", category: "Windows Shell" },
  { id: "cm-wmp-playlist", title: "Add to Windows Media Player list", description: "Removes WMP playlist options from audio/video files.", registryKey: "HKCR\\SystemFileAssociations\\audio\\shell\\Enqueue", targetExtension: "Audio", enabled: true, risk: "low", category: "Legacy Media" },
  { id: "cm-sendto-skype", title: "Send to Skype recipient", description: "Removes Skype from Send To submenu.", registryKey: "HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Explorer\\SendTo\\Skype", targetExtension: "Send To", enabled: true, risk: "low", category: "Send To Menu" },
];

export interface CommunityPackSeed {
  id: string;
  name: string;
  author: string;
  description: string;
  version: string;
  category: string;
  icon: string;
  tweakIds: string[];
  debloatPackages: string[];
  privacyRuleIds: string[];
  installed: boolean;
}

export const communityPacksSeed: CommunityPackSeed[] = [
  {
    id: "pack-atlasos-gaming",
    name: "AtlasOS Latency & Esports Suite",
    author: "AtlasOS Community",
    description: "Maximum responsiveness, D3D scheduling priority boost, power throttling disabled, and MMCSS tuned for competitive gaming.",
    version: "0.4.1",
    category: "Gaming & Latency",
    icon: "⚡",
    tweakIds: ["perf-game-mode", "game-mmcss-games", "game-disable-powerthrottling", "net-nagle", "net-throttling", "perf-large-system-cache", "perf-disable-lastaccess"],
    debloatPackages: ["Microsoft.XboxApp", "Microsoft.XboxGameOverlay", "Microsoft.XboxGamingOverlay", "Clipchamp.Clipchamp", "Microsoft.BingNews"],
    privacyRuleIds: ["priv-telemetry-security", "priv-activity-history"],
    installed: false,
  },
  {
    id: "pack-revios-hardened",
    name: "ReviOS Privacy Lockdown",
    author: "Revision Team",
    description: "Military-grade telemetry, telemetry task autologger disabled, AI Copilot removed, and diagnostic background reporting completely silenced.",
    version: "24.11",
    category: "Privacy & Hardening",
    icon: "🛡️",
    tweakIds: ["tel-disable-telemetry", "tel-ceip", "tel-feedback", "tel-autologger", "ui-disable-copilot", "tel-disable-web-search", "ui-disable-consumer"],
    debloatPackages: ["Microsoft.Windows.Ai.Copilot", "Microsoft.Windows.Recall", "Microsoft.Copilot", "MicrosoftWindows.Client.WebExperience", "Microsoft.BingSearch"],
    privacyRuleIds: ["priv-telemetry-security", "priv-ceip", "priv-wer", "priv-inking-typing", "priv-ad-id", "priv-suggested-content", "priv-tailored-experiences", "priv-copilot", "priv-recall"],
    installed: false,
  },
  {
    id: "pack-ctt-minimalist",
    name: "Chris Titus Tech Recommended Defaults",
    author: "Chris Titus (WinUtil)",
    description: "The gold-standard curated configuration: essential bloat removed, consumer experiences disabled, update peer sharing stopped, but fully safe for everyday use.",
    version: "2024.12",
    category: "Balanced & Everyday",
    icon: "🐧",
    tweakIds: ["ui-disable-bing-search", "ui-disable-consumer", "ui-search-highlights", "exp-disable-ads", "svc-disable-diagtrack", "svc-disable-dosvc", "pwr-fast-startup"],
    debloatPackages: ["Microsoft.BingNews", "Microsoft.BingWeather", "Microsoft.GetHelp", "Microsoft.Getstarted", "Microsoft.People", "Microsoft.WindowsFeedbackHub", "Microsoft.YourPhone", "Clipchamp.Clipchamp", "Microsoft.DevHome"],
    privacyRuleIds: ["priv-telemetry-security", "priv-ceip", "priv-inking-typing", "priv-ad-id", "priv-suggested-content", "priv-cloud-clipboard"],
    installed: false,
  },
  {
    id: "pack-dev-power",
    name: "Software Engineer / Power User Pack",
    author: "WinForge Lab",
    description: "Optimized for heavy dev workloads: fast NTFS indexing, pagefile memory optimization, verbose diagnostics, and developer bloatware cleanup.",
    version: "1.2.0",
    category: "Development",
    icon: "💻",
    tweakIds: ["perf-8dot3", "perf-pagefile", "perf-memcompression", "perf-verbose-status", "ui-classic-context", "exp-disable-ads"],
    debloatPackages: ["Microsoft.MicrosoftSolitaireCollection", "king.com.CandyCrushSaga", "Disney.MagicKingdoms", "TikTok.TikTok", "Facebook.Facebook"],
    privacyRuleIds: ["priv-telemetry-security", "priv-inking-typing", "priv-ad-id"],
    installed: false,
  },
];

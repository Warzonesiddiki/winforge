//go:build windows

package bloatware

import "winforge/internal/registry"

// uninstallRoots are the registry locations that list installed applications,
// covering both 32- and 64-bit installs for the machine and the current user.
var uninstallRoots = []struct {
	hive registry.Hive
	path string
}{
	{registry.HKEY_LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`},
	{registry.HKEY_LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`},
	{registry.HKEY_CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Uninstall`},
}

// Installed returns the display names of all installed applications, gathered
// from the registry uninstall keys. Enumeration failures are best-effort:
// an unreadable hive/root is skipped.
func Installed() []string {
	var names []string
	for _, root := range uninstallRoots {
		subs, err := registry.EnumSubkeys(root.hive, root.path)
		if err != nil {
			continue
		}
		for _, sub := range subs {
			name, err := registry.String(root.hive, root.path+`\`+sub, "DisplayName")
			if err != nil || name == "" {
				continue
			}
			names = append(names, name)
		}
	}
	return names
}

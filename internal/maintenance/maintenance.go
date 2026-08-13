// Package maintenance implements WinForge's one-click system fixes and network
// configuration. Everything here uses native executables (dism.exe, netsh,
// ipconfig) or the service layer — never PowerShell.
package maintenance

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"
)

// ErrUnsupported is returned on non-Windows platforms.
var ErrUnsupported = errors.New("maintenance operations are only supported on Windows")

// LogFunc receives streaming command output (may be nil).
type LogFunc func(line string)

// ResetWindowsUpdate stops the update services, renames the
// SoftwareDistribution cache, and restarts the services.
func ResetWindowsUpdate(log LogFunc) error { return resetWindowsUpdate(log) }

// RepairImage runs DISM /Online /Cleanup-Image /RestoreHealth.
func RepairImage(log LogFunc) error { return repairImage(log) }

// FlushDNS flushes the DNS resolver cache.
func FlushDNS() error { return flushDNS() }

// NetworkReset resets Winsock and the TCP/IP stack, then refreshes DHCP.
func NetworkReset(log LogFunc) error { return networkReset(log) }

// EnableFeature enables a Windows optional feature via DISM.
func EnableFeature(name string, log LogFunc) error {
	if err := ValidateFeatureName(name); err != nil {
		return err
	}
	return enableFeature(name, log)
}

// DisableFeature disables a Windows optional feature via DISM.
func DisableFeature(name string, log LogFunc) error {
	if err := ValidateFeatureName(name); err != nil {
		return err
	}
	return disableFeature(name, log)
}

// ValidateFeatureName rejects values outside DISM's feature-identity syntax.
func ValidateFeatureName(name string) error {
	if name == "" || len(name) > 256 || strings.TrimSpace(name) != name {
		return fmt.Errorf("invalid Windows feature name %q", name)
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			continue
		}
		return fmt.Errorf("invalid Windows feature name %q", name)
	}
	return nil
}

// RemoveProvisionedAppx removes provisioned Appx packages matching an identity
// name or version-specific package name via DISM.
func RemoveProvisionedAppx(packageName string) error { return removeProvisionedAppx(packageName) }

// RemoveAppxPackageByFullName removes current-user Appx packages matching an
// identity name, family name, or version-specific full name using the
// Windows.Management.Deployment.PackageManager WinRT interface. This is
// the stdlib-only, CGO-free equivalent of PowerShell's Remove-AppxPackage,
// using raw ole32/combase P/Invoke (RoActivateInstance). Per-user Appx
// packages have no clean CLI alternative; Remove-AppxPackage is
// PowerShell-only → banned.
func RemoveAppxPackageByFullName(packageName string) error {
	return removeAppxPackageByFullName(packageName)
}

type provisionedPackage struct {
	displayName string
	packageName string
}

// parseProvisionedPackages reads the stable and version-specific identities
// emitted by `dism /Get-ProvisionedAppxPackages /English`. A successful empty
// result is distinguished from truncated, localized, or otherwise unexpected
// output so a parse failure is never cached as "no packages installed".
func parseProvisionedPackages(output string) ([]provisionedPackage, error) {
	var packages []provisionedPackage
	var current provisionedPackage
	var malformedRecords int
	flush := func() {
		if current.displayName != "" && current.packageName != "" {
			packages = append(packages, current)
		} else if current.displayName != "" || current.packageName != "" {
			malformedRecords++
		}
		current = provisionedPackage{}
	}
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "DisplayName":
			flush()
			current.displayName = strings.TrimSpace(value)
		case "PackageName":
			current.packageName = strings.TrimSpace(value)
		}
	}
	flush()

	if malformedRecords != 0 {
		return packages, fmt.Errorf("parse provisioned Appx packages: %d incomplete package record(s)", malformedRecords)
	}
	if !strings.Contains(output, "The operation completed successfully.") {
		return packages, errors.New("parse provisioned Appx packages: DISM success marker is missing")
	}
	return packages, nil
}

// SetDns sets static DNS servers on a named adapter.
func SetDns(adapter, primary, secondary string) error {
	if err := ValidateDnsSettings(adapter, primary, secondary); err != nil {
		return err
	}
	return setDns(adapter, primary, secondary)
}

// ValidateDnsSettings validates a named-adapter DNS request without changing
// network configuration.
func ValidateDnsSettings(adapter, primary, secondary string) error {
	if err := validateAdapterName(adapter); err != nil {
		return err
	}
	return ValidateDnsServers(primary, secondary)
}

func validateAdapterName(adapter string) error {
	if strings.TrimSpace(adapter) == "" {
		return errors.New("network adapter is required")
	}
	if strings.ContainsAny(adapter, "\x00\r\n\"") {
		return errors.New("network adapter contains invalid characters")
	}
	return nil
}

// SetDnsOnAll sets static DNS servers on every active (non-loopback) adapter.
func SetDnsOnAll(primary, secondary string) error {
	if err := ValidateDnsServers(primary, secondary); err != nil {
		return err
	}
	return setDnsOnAll(primary, secondary)
}

// ValidateDnsServers validates a primary and optional secondary DNS address
// without changing any adapters.
func ValidateDnsServers(primary, secondary string) error {
	if err := validateDnsServers(primary, secondary); err != nil {
		return err
	}
	return nil
}

func validateDnsServers(primary, secondary string) error {
	if err := validateDnsServer("primary", primary, true); err != nil {
		return err
	}
	return validateDnsServer("secondary", secondary, false)
}

func validateDnsServer(label, value string, required bool) error {
	if value == "" && !required {
		return nil
	}
	addr, err := netip.ParseAddr(value)
	if err != nil {
		return fmt.Errorf("%s DNS server %q is not a valid IP address: %w", label, value, err)
	}
	if !addr.Is4() {
		return fmt.Errorf("%s DNS server %q is not an IPv4 address", label, value)
	}
	if addr.IsUnspecified() || addr.IsMulticast() {
		return fmt.Errorf("%s DNS server %q is not a usable unicast address", label, value)
	}
	return nil
}

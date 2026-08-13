// Package maintenance implements WinForge's one-click system fixes and network
// configuration. Everything here uses native executables (dism.exe, netsh,
// ipconfig) or the service layer — never PowerShell.
package maintenance

import "errors"

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
func EnableFeature(name string, log LogFunc) error { return enableFeature(name, log) }

// DisableFeature disables a Windows optional feature via DISM.
func DisableFeature(name string, log LogFunc) error { return disableFeature(name, log) }

// RemoveProvisionedAppx removes a provisioned Appx package via DISM.
func RemoveProvisionedAppx(packageName string) error { return removeProvisionedAppx(packageName) }

// SetDns sets static DNS servers on a named adapter.
func SetDns(adapter, primary, secondary string) error { return setDns(adapter, primary, secondary) }

// SetDnsOnAll sets static DNS servers on every active (non-loopback) adapter.
func SetDnsOnAll(primary, secondary string) error { return setDnsOnAll(primary, secondary) }

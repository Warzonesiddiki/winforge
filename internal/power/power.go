// Package power manages Windows power plans (schemes) and their settings via
// powercfg.exe — the stdlib-only, PowerShell-free equivalent of the Power
// Options control panel.
package power

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrUnsupported is returned on non-Windows platforms.
var ErrUnsupported = errors.New("power plan management is only supported on Windows")

// Built-in scheme GUIDs, identical on every Windows install.
const (
	Balanced        = "381b4222-f694-41f0-9685-ff5bb260df2e"
	HighPerformance = "8c5e7fda-e8bf-4a96-9a85-a6e23a8c635c"
	PowerSaver      = "a1841308-3541-4fab-bc81-f71556f20b4a"
	// Ultimate is the hidden "Ultimate Performance" scheme (not present on
	// laptops and some desktops; it must be duplicated before activation).
	Ultimate = "e9a42b02-d5df-448d-aa00-03f14749eb61"

	// UltimateClone is the stable GUID WinForge gives its copy of the
	// "Ultimate Performance" scheme so the copy is idempotent (a fresh
	// /duplicatescheme would mint a random GUID on every run).
	UltimateClone = "dd847358-6d09-4a60-9d3e-0f4a0f0d0d0d"
)

// Processor power-management subgroup and setting GUIDs (used with
// /setacvalueindex).
const (
	SubProcessor      = "54533251-82be-4824-96c1-47b60b740d00"
	SettingMinProc    = "893dee8e-2bef-41e0-89c6-b55d0929964c"
	SettingMaxProc    = "bc5038f7-23e0-4960-96da-33abaf5935ec"
	SubUsb            = "2a737441-1930-4402-8d77-b2bebba308a3"
	SettingUsbSuspend = "48e6b7a6-50f5-4782-a5d4-53bb8f07e226"
)

// Plan describes a power scheme as reported by powercfg /list.
type Plan struct {
	GUID string `json:"guid"`
	Name string `json:"name"`
}

// aliases maps human-readable names (used in tweaks.json and the CLI) to the
// canonical scheme GUIDs.
var aliases = map[string]string{
	"balanced":         Balanced,
	"high-performance": HighPerformance,
	"highperformance":  HighPerformance,
	"power-saver":      PowerSaver,
	"powersaver":       PowerSaver,
	"ultimate":         UltimateClone,
}

// Resolve maps a scheme alias ("balanced", "ultimate", …) or raw GUID to the
// canonical GUID that WinForge treats as the scheme's target state.
func Resolve(aliasOrGUID string) (string, error) {
	s := strings.TrimSpace(aliasOrGUID)
	if g, ok := aliases[strings.ToLower(s)]; ok {
		return g, nil
	}
	if !isGUID(s) {
		return "", fmt.Errorf("unknown power scheme %q (use a built-in alias or a GUID)", aliasOrGUID)
	}
	return strings.ToLower(s), nil
}

// guidParts splits a canonical 8-4-4-4-12 GUID string into the binary fields
// of the Win32 GUID structure (Data1..Data4). It is portable so the parsing
// logic is unit-testable off Windows; only the syscalls that consume the
// result are Windows-specific.
func guidParts(s string) (data1 uint32, data2, data3 uint16, data4 [8]byte, err error) {
	if !isGUID(s) {
		return 0, 0, 0, data4, fmt.Errorf("invalid GUID %q", s)
	}
	p1, err := strconv.ParseUint(s[0:8], 16, 32)
	if err != nil {
		return 0, 0, 0, data4, fmt.Errorf("invalid GUID %q: %w", s, err)
	}
	p2, err := strconv.ParseUint(s[9:13], 16, 16)
	if err != nil {
		return 0, 0, 0, data4, fmt.Errorf("invalid GUID %q: %w", s, err)
	}
	p3, err := strconv.ParseUint(s[14:18], 16, 16)
	if err != nil {
		return 0, 0, 0, data4, fmt.Errorf("invalid GUID %q: %w", s, err)
	}
	tail := s[19:23] + s[24:36]
	for i := 0; i < 8; i++ {
		b, parseErr := strconv.ParseUint(tail[i*2:i*2+2], 16, 8)
		if parseErr != nil {
			return 0, 0, 0, data4, fmt.Errorf("invalid GUID %q: %w", s, parseErr)
		}
		data4[i] = byte(b)
	}
	return uint32(p1), uint16(p2), uint16(p3), data4, nil
}

// isGUID reports whether s is a canonical 8-4-4-4-12 hexadecimal GUID.
func isGUID(s string) bool {
	hex := "0123456789abcdefABCDEF"
	if len(s) != 36 {
		return false
	}
	for i, r := range s {
		hyphen := i == 8 || i == 13 || i == 18 || i == 23
		if hyphen {
			if r != '-' {
				return false
			}
			continue
		}
		if !strings.ContainsRune(hex, r) {
			return false
		}
	}
	return true
}

// Active returns the GUID of the currently active power scheme.
func Active() (string, error) { return active() }

// SetActive makes a GUID or built-in alias the active power scheme, creating a
// copy of the "Ultimate Performance" scheme first when requested.
func SetActive(scheme string) error {
	guid, err := Resolve(scheme)
	if err != nil {
		return err
	}
	return setActive(guid)
}

// List enumerates all power schemes, newest first.
func List() ([]Plan, error) { return list() }

// SetHibernate enables or disables hibernation by committing or decommitting
// the hibernation file, via CallNtPowerInformation(SystemReserveHiberFile) —
// the documented native equivalent of `powercfg /hibernate on|off`. Requires
// elevation (the call returns STATUS_ACCESS_DENIED otherwise).
func SetHibernate(enable bool) error { return setHibernate(enable) }

// HibernateEnabled reports whether hibernation is currently available, via
// IsPwrHibernateAllowed: TRUE when the machine supports S4 and Hiberfil.sys
// is present, FALSE otherwise.
func HibernateEnabled() (bool, error) { return hibernateEnabled() }

// GetProcessorState returns the AC minimum and maximum processor state
// (percent) of the active power scheme, read natively via
// PowerGetActiveScheme + PowerReadACValueIndex.
func GetProcessorState() (minPct, maxPct uint32, err error) { return getProcessorState() }

// SetProcessorState sets the minimum and maximum processor state (percent) of
// the current scheme and applies the change immediately.
func SetProcessorState(minPct, maxPct uint32) error {
	if minPct > 100 || maxPct > 100 {
		return fmt.Errorf("processor state percentages must be between 0 and 100 (got %d and %d)", minPct, maxPct)
	}
	if minPct > maxPct {
		return fmt.Errorf("minimum processor state %d exceeds maximum %d", minPct, maxPct)
	}
	return setProcessorState(minPct, maxPct)
}

// SetAcValueIndex sets one AC power setting (identified by subgroup and
// setting GUIDs) on the given scheme and applies it to the active scheme.
func SetAcValueIndex(scheme, subgroup, setting string, value uint32) error {
	if !strings.EqualFold(scheme, "SCHEME_CURRENT") && !isGUID(scheme) {
		return fmt.Errorf("invalid power scheme GUID %q", scheme)
	}
	if !isGUID(subgroup) {
		return fmt.Errorf("invalid power subgroup GUID %q", subgroup)
	}
	if !isGUID(setting) {
		return fmt.Errorf("invalid power setting GUID %q", setting)
	}
	return setAcValueIndex(scheme, subgroup, setting, value)
}

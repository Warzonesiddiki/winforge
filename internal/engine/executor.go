// Package engine implements tweak.Executor against real Windows APIs
// (registry, services), plus protected-service and command execution guards.
package engine

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"unicode"
	"unicode/utf8"

	"winforge/internal/maintenance"
	"winforge/internal/platform"
	"winforge/internal/power"
	"winforge/internal/registry"
	"winforge/internal/scheduler"
	"winforge/internal/service"
)

// Executor is the concrete, Windows-backed implementation of tweak.Executor.
type Executor struct {
	protected map[string]struct{}
	elevated  bool
}

// NewExecutor creates an Executor. protectedServices are service names that
// may not be mutated.
func NewExecutor(protectedServices []string) *Executor {
	return NewExecutorWithElevation(protectedServices, platform.IsElevated())
}

// NewExecutorWithElevation creates an Executor using the caller's captured
// security mode so configuration loading and command dispatch share one stable
// trust decision.
func NewExecutorWithElevation(protectedServices []string, elevated bool) *Executor {
	e := &Executor{
		protected: make(map[string]struct{}, len(protectedServices)),
		elevated:  elevated,
	}
	for _, s := range protectedServices {
		e.protected[strings.ToLower(s)] = struct{}{}
	}
	return e
}

// RegistryGetDword reads a REG_DWORD value.
func (e *Executor) RegistryGetDword(hive, path, name string) (uint32, bool, error) {
	v, err := registry.Dword(registry.Hive(hive), path, name)
	if err != nil {
		if err == registry.ErrNotFound {
			return 0, false, nil
		}
		return 0, false, err
	}
	return v, true, nil
}

// RegistryGetString reads a REG_SZ value.
func (e *Executor) RegistryGetString(hive, path, name string) (string, bool, error) {
	v, err := registry.String(registry.Hive(hive), path, name)
	if err != nil {
		if err == registry.ErrNotFound {
			return "", false, nil
		}
		return "", false, err
	}
	return v, true, nil
}

// RegistryGetExpandString reads an unexpanded REG_EXPAND_SZ value.
func (e *Executor) RegistryGetExpandString(hive, path, name string) (string, bool, error) {
	v, err := registry.ExpandString(registry.Hive(hive), path, name)
	if err != nil {
		if err == registry.ErrNotFound {
			return "", false, nil
		}
		return "", false, err
	}
	return v, true, nil
}

// RegistryGetQword reads a REG_QWORD value.
func (e *Executor) RegistryGetQword(hive, path, name string) (uint64, bool, error) {
	v, err := registry.Qword(registry.Hive(hive), path, name)
	if err != nil {
		if err == registry.ErrNotFound {
			return 0, false, nil
		}
		return 0, false, err
	}
	return v, true, nil
}

// RegistrySetDword writes a REG_DWORD value.
func (e *Executor) RegistrySetDword(hive, path, name string, value uint32) error {
	return registry.SetDword(registry.Hive(hive), path, name, value)
}

// RegistrySetString writes a REG_SZ value.
func (e *Executor) RegistrySetString(hive, path, name string, value string) error {
	return registry.SetString(registry.Hive(hive), path, name, value)
}

// RegistrySetExpandString writes an unexpanded REG_EXPAND_SZ value.
func (e *Executor) RegistrySetExpandString(hive, path, name string, value string) error {
	return registry.SetExpandString(registry.Hive(hive), path, name, value)
}

// RegistrySetQword writes a REG_QWORD value.
func (e *Executor) RegistrySetQword(hive, path, name string, value uint64) error {
	return registry.SetQword(registry.Hive(hive), path, name, value)
}

// RegistryDeleteValue deletes a value.
func (e *Executor) RegistryDeleteValue(hive, path, name string) error {
	return registry.DeleteValue(registry.Hive(hive), path, name)
}

// maxServiceNameLen is the SCM's limit on a service name (CreateService and
// OpenService both document a 256-character maximum).
const maxServiceNameLen = 256

// validateServiceName rejects names the SCM would not accept, and any name
// that is not already in canonical form.
//
// The protected-service check is a lookup in a normalised set, so it can only
// be as strong as the normalisation agrees with the SCM's own comparison. The
// SCM compares service names case-insensitively, which the lookup mirrors, but
// a name carrying surrounding whitespace hashes differently from the same name
// without it: "WinDefend " would miss the protected entry for "WinDefend" and
// fall through to a real service call. Operation names reach here straight from
// a tweaks.json (including third-party plugin catalogues), whose validation
// requires a non-blank name but stores it untrimmed.
//
// Such a name is rejected rather than silently trimmed: a padded or otherwise
// malformed identity is never something the caller meaningfully asked for, and
// quietly rewriting it into a different service's name is exactly the confusion
// this guard exists to prevent.
func validateServiceName(name string) error {
	if name == "" {
		return errors.New("service name is required")
	}
	if strings.TrimSpace(name) != name {
		return fmt.Errorf("service name %q has leading or trailing whitespace", name)
	}
	if utf8.RuneCountInString(name) > maxServiceNameLen {
		return fmt.Errorf("service name exceeds the %d-character limit", maxServiceNameLen)
	}
	// Forward-slash and backslash are documented as invalid service name
	// characters; a NUL would truncate the string once it crosses into the
	// UTF-16 conversion for the Win32 call.
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("service name %q contains a path separator", name)
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return fmt.Errorf("service name %q contains a control character", name)
		}
	}
	return nil
}

func (e *Executor) ensureServiceMutable(name string) error {
	if err := validateServiceName(name); err != nil {
		return err
	}
	if _, blocked := e.protected[strings.ToLower(name)]; blocked {
		return fmt.Errorf("service %q is protected and cannot be modified", name)
	}
	return nil
}

// ServiceSetStartMode changes a service start type, refusing protected services.
func (e *Executor) ServiceSetStartMode(name string, mode string) error {
	if err := e.ensureServiceMutable(name); err != nil {
		return err
	}
	m, err := service.ParseStartMode(mode)
	if err != nil {
		return err
	}
	return service.SetStartMode(name, m)
}

// ServiceGetStartMode returns the current start mode as a string. Reading is
// allowed for protected services, but the name is still validated so a
// malformed identity cannot reach the SCM.
func (e *Executor) ServiceGetStartMode(name string) (string, error) {
	if err := validateServiceName(name); err != nil {
		return "", err
	}
	m, err := service.GetStartMode(name)
	if err != nil {
		return "", err
	}
	return m.String(), nil
}

// ServiceStart starts a service, refusing protected services.
func (e *Executor) ServiceStart(name string) error {
	if err := e.ensureServiceMutable(name); err != nil {
		return err
	}
	return service.Start(name)
}

// ServiceStop stops a service, refusing protected services.
func (e *Executor) ServiceStop(name string) error {
	if err := e.ensureServiceMutable(name); err != nil {
		return err
	}
	return service.Stop(name)
}

// TaskDisable disables a scheduled task via schtasks.exe.
func (e *Executor) TaskDisable(path string) error { return scheduler.Disable(path) }

// TaskEnable enables a scheduled task via schtasks.exe.
func (e *Executor) TaskEnable(path string) error { return scheduler.Enable(path) }

// TaskDelete removes a scheduled task via schtasks.exe.
func (e *Executor) TaskDelete(path string) error { return scheduler.Delete(path) }

// AppxRemove removes current-user and provisioned registrations matching a
// stable package identity name, family name, or version-specific full name.
func (e *Executor) AppxRemove(name string) error {
	var errs []error
	if err := maintenance.RemoveAppxPackageByFullName(name); err != nil {
		errs = append(errs, fmt.Errorf("per-user: %w", err))
	}
	if err := maintenance.RemoveProvisionedAppx(name); err != nil {
		errs = append(errs, fmt.Errorf("provisioned: %w", err))
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("appx removal failed for %q: %w", name, err)
	}
	return nil
}

// RunCommand executes a native command line. Used by explicit command tweaks
// and the winget fallback. PowerShell is deliberately never invoked here.
func (e *Executor) RunCommand(cmdline string, args []string) error {
	command, trusted := trustedCommand(cmdline)
	if e.elevated && !trusted {
		return fmt.Errorf("command %q is not an allowlisted Windows system executable", cmdline)
	}
	if !trusted {
		command = cmdline
	}
	cmd := exec.Command(command, args...)
	return cmd.Run()
}

// PowerGetActive returns the GUID of the active power scheme.
func (e *Executor) PowerGetActive() (string, error) { return power.Active() }

// PowerSetActive activates a power scheme by GUID.
func (e *Executor) PowerSetActive(guid string) error { return power.SetActive(guid) }

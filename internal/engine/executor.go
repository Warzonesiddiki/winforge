// Package engine implements tweak.Executor against real Windows APIs
// (registry, services), plus protected-service and command execution guards.
package engine

import (
	"fmt"
	"os/exec"
	"strings"

	"winforge/internal/registry"
	"winforge/internal/service"
)

// Executor is the concrete, Windows-backed implementation of tweak.Executor.
type Executor struct {
	protected map[string]struct{}
}

// NewExecutor creates an Executor. protectedServices are service names that
// may not have their start mode modified.
func NewExecutor(protectedServices []string) *Executor {
	e := &Executor{protected: make(map[string]struct{}, len(protectedServices))}
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

// RegistrySetDword writes a REG_DWORD value.
func (e *Executor) RegistrySetDword(hive, path, name string, value uint32) error {
	return registry.SetDword(registry.Hive(hive), path, name, value)
}

// RegistrySetString writes a REG_SZ value.
func (e *Executor) RegistrySetString(hive, path, name string, value string) error {
	return registry.SetString(registry.Hive(hive), path, name, value)
}

// RegistryDeleteValue deletes a value.
func (e *Executor) RegistryDeleteValue(hive, path, name string) error {
	return registry.DeleteValue(registry.Hive(hive), path, name)
}

// ServiceSetStartMode changes a service start type, refusing protected services.
func (e *Executor) ServiceSetStartMode(name string, mode string) error {
	if _, blocked := e.protected[strings.ToLower(name)]; blocked {
		return fmt.Errorf("service %q is protected and cannot be modified", name)
	}
	m, err := service.ParseStartMode(mode)
	if err != nil {
		return err
	}
	return service.SetStartMode(name, m)
}

// ServiceGetStartMode returns the current start mode as a string.
func (e *Executor) ServiceGetStartMode(name string) (string, error) {
	m, err := service.GetStartMode(name)
	if err != nil {
		return "", err
	}
	return m.String(), nil
}

// ServiceStart starts a service.
func (e *Executor) ServiceStart(name string) error { return service.Start(name) }

// ServiceStop stops a service.
func (e *Executor) ServiceStop(name string) error { return service.Stop(name) }

// TaskDisable is not yet implemented (Task Scheduler COM interop is a later phase).
func (e *Executor) TaskDisable(path string) error {
	return fmt.Errorf("task scheduler control not implemented (task: %s)", path)
}

// TaskEnable is not yet implemented.
func (e *Executor) TaskEnable(path string) error {
	return fmt.Errorf("task scheduler control not implemented (task: %s)", path)
}

// TaskDelete is not yet implemented.
func (e *Executor) TaskDelete(path string) error {
	return fmt.Errorf("task scheduler control not implemented (task: %s)", path)
}

// AppxRemove is not yet implemented (Appx PackageManager interop is a later phase).
func (e *Executor) AppxRemove(name string) error {
	return fmt.Errorf("appx removal not implemented (package: %s)", name)
}

// RunCommand executes a native command line. Used by explicit command tweaks
// and the winget fallback. PowerShell is deliberately never invoked here.
func (e *Executor) RunCommand(cmdline string, args []string) error {
	cmd := exec.Command(cmdline, args...)
	return cmd.Run()
}

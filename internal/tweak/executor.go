// Package tweak orchestrates the application, dry-run simulation, and undo of
// system tweaks. It is platform-agnostic: all mutation happens through an
// Executor, so the orchestration and health logic are fully unit-testable.
package tweak

// Executor is the platform seam through which the orchestrator reads and
// mutates system state. A Windows implementation wires this to the registry,
// service, scheduler, and appx packages.
type Executor interface {
	RegistryGetDword(hive, path, name string) (uint32, bool, error)
	RegistryGetString(hive, path, name string) (string, bool, error)
	RegistryGetExpandString(hive, path, name string) (string, bool, error)
	RegistryGetQword(hive, path, name string) (uint64, bool, error)
	RegistrySetDword(hive, path, name string, value uint32) error
	RegistrySetString(hive, path, name string, value string) error
	RegistrySetExpandString(hive, path, name string, value string) error
	RegistrySetQword(hive, path, name string, value uint64) error
	RegistryDeleteValue(hive, path, name string) error
	RegistryKeyExists(hive, path string) (bool, error)
	RegistryDeleteKeyTree(hive, path string) error

	ServiceSetStartMode(name string, mode string) error
	ServiceGetStartMode(name string) (string, error)
	ServiceStart(name string) error
	ServiceStop(name string) error

	TaskDisable(path string) error
	TaskEnable(path string) error
	TaskDelete(path string) error

	AppxRemove(name string) error

	RunCommand(cmdline string, args []string) error

	PowerGetActive() (string, error)
	PowerSetActive(guid string) error
	PowerHibernateEnabled() (bool, error)
	PowerSetHibernate(enable bool) error
	PowerGetProcessorState() (minPct, maxPct uint32, err error)
	PowerSetProcessorState(minPct, maxPct uint32) error

	// NetbiosGetOptions reads the current NetbiosOptions value of every
	// NetBT interface, keyed by interface subkey name. NetbiosSetOptions
	// writes one NetbiosOptions value to every NetBT interface.
	NetbiosGetOptions() (map[string]uint32, error)
	NetbiosSetOptions(value uint32) error
}

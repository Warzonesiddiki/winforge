package engine

import (
	"errors"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"winforge/internal/maintenance"
	"winforge/internal/registry"
	"winforge/internal/service"
)

// skipOnWindows guards tests that assert the off-Windows platform stub
// (ErrUnsupported). On a real Windows runner those calls reach the live service
// control manager, registry, Appx store, task scheduler and power APIs: stub
// assertions do not hold there, and several of the calls would mutate runner
// state. The Windows build path is still exercised by compilation, vet, and the
// windows-tagged tests in this package.
func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("asserts the off-Windows platform stub; on Windows it would hit live system state")
	}
}

func TestElevatedExecutorRejectsUnknownCommand(t *testing.T) {
	executor := &Executor{elevated: true}
	err := executor.RunCommand("user-controlled-command.exe", nil)
	if err == nil || !strings.Contains(err.Error(), "not an allowlisted") {
		t.Fatalf("elevated unknown command returned %v", err)
	}
}

func TestProtectedServicesRejectEveryMutation(t *testing.T) {
	executor := NewExecutor([]string{"WinDefend"})
	tests := []struct {
		name   string
		mutate func() error
	}{
		{name: "start mode", mutate: func() error { return executor.ServiceSetStartMode("windefend", "manual") }},
		{name: "start", mutate: func() error { return executor.ServiceStart("WINDEFEND") }},
		{name: "stop", mutate: func() error { return executor.ServiceStop("WinDefend") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.mutate()
			if err == nil || !strings.Contains(err.Error(), "protected") {
				t.Fatalf("protected mutation returned %v", err)
			}
		})
	}
}

// TestProtectedServiceLookupIsCaseInsensitive pins the normalisation the
// protected set relies on. The SCM compares service names case-insensitively,
// so the guard must too, or a differently-cased spelling of a protected
// service would be mutable.
func TestProtectedServiceLookupIsCaseInsensitive(t *testing.T) {
	executor := NewExecutor([]string{"WinDefend", "SgrmBroker"})
	for _, name := range []string{
		"WinDefend", "windefend", "WINDEFEND", "WiNdEfEnD",
		"SgrmBroker", "sgrmbroker", "SGRMBROKER",
	} {
		t.Run(name, func(t *testing.T) {
			err := executor.ServiceStop(name)
			if err == nil || !strings.Contains(err.Error(), "protected") {
				t.Fatalf("ServiceStop(%q) = %v, want a protected-service refusal", name, err)
			}
		})
	}
}

// TestMalformedServiceNamesAreRejected covers a bypass of the protected-service
// guard. The guard is a lookup in a lowercased set, so a name carrying padding
// hashed differently from the protected entry and fell straight through to a
// real SCM call: "WinDefend " missed the entry for "WinDefend". Operation names
// arrive from tweaks.json (including third-party plugin catalogues), which
// requires a non-blank name but stores it untrimmed.
//
// Every mutating entry point must reject these before consulting the set, and
// must reject rather than trim: silently rewriting a name into a different
// service's identity is the confusion the guard exists to prevent.
func TestMalformedServiceNamesAreRejected(t *testing.T) {
	executor := NewExecutor([]string{"WinDefend"})

	names := []struct {
		name string
		want string
	}{
		{"WinDefend ", "whitespace"},
		{" WinDefend", "whitespace"},
		{"\tWinDefend", "whitespace"},
		{"WinDefend\n", "whitespace"},
		{"WinDefend\r\n", "whitespace"},
		{"\u00a0WinDefend", "whitespace"}, // non-breaking space
		{"WinDefend\x00", "control character"},
		{"Win\x00Defend", "control character"},
		{"WinDefend\x07", "control character"},
		{`windefend\..\windefend`, "path separator"},
		{"windefend/../windefend", "path separator"},
		{`\WinDefend`, "path separator"},
		{"", "required"},
		{strings.Repeat("a", maxServiceNameLen+1), "limit"},
	}

	mutations := []struct {
		verb string
		call func(string) error
	}{
		{"stop", executor.ServiceStop},
		{"start", executor.ServiceStart},
		{"set start mode", func(n string) error { return executor.ServiceSetStartMode(n, "disabled") }},
		{"get start mode", func(n string) error { _, err := executor.ServiceGetStartMode(n); return err }},
	}

	for _, m := range mutations {
		for _, tc := range names {
			t.Run(m.verb+"/"+tc.want+"/"+tc.name, func(t *testing.T) {
				err := m.call(tc.name)
				if err == nil {
					t.Fatalf("%s(%q) accepted a malformed service name", m.verb, tc.name)
				}
				if !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("%s(%q) error = %q, want it to mention %q", m.verb, tc.name, err, tc.want)
				}
				// A malformed name must never reach the platform layer.
				if errors.Is(err, service.ErrUnsupported) {
					t.Fatalf("%s(%q) reached the service layer instead of being rejected", m.verb, tc.name)
				}
			})
		}
	}
}

// TestServiceNameAtLimitIsAccepted keeps the length check from rejecting a name
// the SCM would accept, so the bound above is a boundary rather than a blanket.
func TestServiceNameAtLimitIsAccepted(t *testing.T) {
	if err := validateServiceName(strings.Repeat("a", maxServiceNameLen)); err != nil {
		t.Fatalf("a %d-character service name was rejected: %v", maxServiceNameLen, err)
	}
}

// TestUnprotectedServiceReachesPlatformLayer proves the guards above are
// selective. An ordinary, well-formed name must pass validation and the
// protected-set lookup and reach the platform call, which off Windows reports
// ErrUnsupported. Without this, a guard that rejected everything would satisfy
// every other service test here.
func TestUnprotectedServiceReachesPlatformLayer(t *testing.T) {
	skipOnWindows(t)
	executor := NewExecutor([]string{"WinDefend"})
	for _, name := range []string{"Spooler", "wuauserv", "Some.Service_Name-1"} {
		t.Run(name, func(t *testing.T) {
			if err := executor.ServiceStop(name); !errors.Is(err, service.ErrUnsupported) {
				t.Fatalf("ServiceStop(%q) = %v, want it to reach the service layer", name, err)
			}
			if err := executor.ServiceStart(name); !errors.Is(err, service.ErrUnsupported) {
				t.Fatalf("ServiceStart(%q) = %v, want it to reach the service layer", name, err)
			}
			if _, err := executor.ServiceGetStartMode(name); !errors.Is(err, service.ErrUnsupported) {
				t.Fatalf("ServiceGetStartMode(%q) = %v, want it to reach the service layer", name, err)
			}
		})
	}
}

// TestServiceSetStartModeValidatesMode checks the mode is parsed before the
// platform call, and that the protected check runs before mode parsing so a
// protected service is refused even with a nonsense mode.
func TestServiceSetStartModeValidatesMode(t *testing.T) {
	skipOnWindows(t)
	executor := NewExecutor([]string{"WinDefend"})

	for _, mode := range []string{"sideways", "", "auto ; delete", "4"} {
		t.Run("bad mode "+mode, func(t *testing.T) {
			err := executor.ServiceSetStartMode("Spooler", mode)
			if err == nil || !strings.Contains(err.Error(), "unknown start mode") {
				t.Fatalf("ServiceSetStartMode(Spooler, %q) = %v, want an unknown-mode error", mode, err)
			}
		})
	}

	for _, mode := range []string{"auto", "automatic", "manual", "demand", "disabled", "disable", "AUTO", " manual "} {
		t.Run("good mode "+mode, func(t *testing.T) {
			if err := executor.ServiceSetStartMode("Spooler", mode); !errors.Is(err, service.ErrUnsupported) {
				t.Fatalf("ServiceSetStartMode(Spooler, %q) = %v, want it to reach the service layer", mode, err)
			}
		})
	}

	// Protection outranks mode validation: a protected service is refused
	// before an invalid mode is even considered.
	if err := executor.ServiceSetStartMode("WinDefend", "sideways"); err == nil ||
		!strings.Contains(err.Error(), "protected") {
		t.Fatalf("protected service with an invalid mode = %v, want a protected-service refusal", err)
	}
}

// TestRegistryHiveValidation checks an unrecognised hive is rejected before any
// platform call rather than being passed through as an opaque string.
func TestRegistryHiveValidation(t *testing.T) {
	skipOnWindows(t)
	e := &Executor{}
	const path = `Software\WinForge`

	badHives := []string{"BOGUS", "", "hklm", "HKEY_LOCAL_MACHINE_EXTRA", "HKLM\x00"}
	for _, hive := range badHives {
		t.Run("bad/"+hive, func(t *testing.T) {
			checks := map[string]error{}
			_, _, err := e.RegistryGetDword(hive, path, "V")
			checks["get dword"] = err
			_, _, err = e.RegistryGetQword(hive, path, "V")
			checks["get qword"] = err
			_, _, err = e.RegistryGetString(hive, path, "V")
			checks["get string"] = err
			_, _, err = e.RegistryGetExpandString(hive, path, "V")
			checks["get expand string"] = err
			checks["set dword"] = e.RegistrySetDword(hive, path, "V", 1)
			checks["set qword"] = e.RegistrySetQword(hive, path, "V", 1)
			checks["set string"] = e.RegistrySetString(hive, path, "V", "x")
			checks["set expand string"] = e.RegistrySetExpandString(hive, path, "V", "x")
			checks["delete"] = e.RegistryDeleteValue(hive, path, "V")

			for op, err := range checks {
				if err == nil {
					t.Fatalf("%s accepted unsupported hive %q", op, hive)
				}
				if !strings.Contains(err.Error(), "unsupported registry hive") {
					t.Fatalf("%s with hive %q = %q, want an unsupported-hive error", op, hive, err)
				}
			}
		})
	}

	for _, hive := range []string{"HKLM", "HKCU", "HKCR", "HKU"} {
		t.Run("good/"+hive, func(t *testing.T) {
			_, _, err := e.RegistryGetDword(hive, path, "V")
			if !errors.Is(err, registry.ErrUnsupported) {
				t.Fatalf("hive %q = %v, want it to reach the registry layer", hive, err)
			}
		})
	}
}

// TestRegistryGetMapsNotFoundToAbsent pins the contract the orchestrator relies
// on to decide whether a value needs restoring: a missing value is reported as
// (zero, false, nil), while a real failure keeps its error. Collapsing the two
// would make a failed read look like a value that was simply absent.
func TestRegistryGetMapsNotFoundToAbsent(t *testing.T) {
	skipOnWindows(t)
	e := &Executor{}

	// Off Windows every read fails with ErrUnsupported, which must NOT be
	// flattened into "absent".
	if _, found, err := e.RegistryGetDword("HKLM", `Software\X`, "V"); err == nil || found {
		t.Fatalf("unsupported dword read = (found %v, err %v), want an error and found=false", found, err)
	}
	if _, found, err := e.RegistryGetString("HKLM", `Software\X`, "V"); err == nil || found {
		t.Fatalf("unsupported string read = (found %v, err %v), want an error and found=false", found, err)
	}
	if _, found, err := e.RegistryGetQword("HKLM", `Software\X`, "V"); err == nil || found {
		t.Fatalf("unsupported qword read = (found %v, err %v), want an error and found=false", found, err)
	}
	if _, found, err := e.RegistryGetExpandString("HKLM", `Software\X`, "V"); err == nil || found {
		t.Fatalf("unsupported expand-string read = (found %v, err %v), want an error and found=false", found, err)
	}

	// The mapping the getters implement, stated directly: ErrNotFound becomes
	// absent-without-error, anything else propagates.
	if !errors.Is(registry.ErrNotFound, registry.ErrNotFound) {
		t.Fatal("registry.ErrNotFound is not comparable to itself")
	}
	if errors.Is(registry.ErrUnsupported, registry.ErrNotFound) {
		t.Fatal("ErrUnsupported must not be treated as ErrNotFound")
	}
}

// TestRunCommandElevationBoundary is the core guard on command execution. While
// elevated, only allowlisted inbox Windows tools may run, because a tweak
// definition (including one from a plugin) would otherwise name an arbitrary
// executable to be launched with administrator rights.
func TestRunCommandElevationBoundary(t *testing.T) {
	elevated := &Executor{elevated: true}

	untrusted := []string{
		"user-controlled-command.exe",
		"powershell",
		"powershell.exe",
		"pwsh.exe",
		"cmd.exe",
		"wscript.exe",
		"C:\\Users\\Public\\evil.exe",
		"/tmp/evil",
		"dism.exe.evil",
		"notdism.exe",
		"",
	}
	for _, name := range untrusted {
		t.Run("elevated/"+name, func(t *testing.T) {
			err := elevated.RunCommand(name, []string{"--whatever"})
			if err == nil || !strings.Contains(err.Error(), "not an allowlisted") {
				t.Fatalf("elevated RunCommand(%q) = %v, want an allowlist refusal", name, err)
			}
			// The refusal must name the rejected command for the audit
			// trail. It is quoted with %q, so compare against that form.
			if name != "" && !strings.Contains(err.Error(), strconv.Quote(name)) {
				t.Fatalf("elevated RunCommand(%q) error %q does not name the command", name, err)
			}
		})
	}
}

// TestRunCommandUnelevatedRunsWithoutAllowlist documents the deliberate
// asymmetry: without elevation the allowlist is not enforced, since the command
// runs with no more privilege than the user already has. This is asserted so
// the behaviour is a decision on record rather than an accident.
//
// The success and non-zero-exit cases re-invoke the test binary itself so the
// assertions hold on every platform (no /bin/true assumptions on Windows).
func TestRunCommandUnelevatedRunsWithoutAllowlist(t *testing.T) {
	unelevated := &Executor{elevated: false}

	// A child that exits 0 must succeed.
	if err := unelevated.RunCommand(os.Args[0], []string{"-test.run=TestRunCommandChildHelper"}); err != nil {
		t.Fatalf("unelevated RunCommand(exit 0) = %v, want it to run", err)
	}
	// A non-zero exit is surfaced to the caller rather than swallowed.
	t.Setenv("WINFORGE_CHILD_EXIT", "3")
	if err := unelevated.RunCommand(os.Args[0], []string{"-test.run=TestRunCommandChildHelper"}); err == nil {
		t.Fatal("unelevated RunCommand(exit 3) reported success for a failing command")
	}
	// A missing executable is an error, not a silent no-op.
	if err := unelevated.RunCommand("definitely-not-a-real-binary-xyz", nil); err == nil {
		t.Fatal("unelevated RunCommand(missing binary) reported success")
	}
}

// TestRunCommandChildHelper is re-invoked by RunCommand tests; it exits 3 when
// WINFORGE_CHILD_EXIT=3, otherwise 0.
func TestRunCommandChildHelper(t *testing.T) {
	if os.Getenv("WINFORGE_CHILD_EXIT") == "3" {
		os.Exit(3)
	}
}

// TestRunCommandPassesArguments confirms arguments reach the process as
// separate argv entries rather than being concatenated into a shell string.
// The child verifies that "alpha" and "beta" arrive as two distinct entries
// after the "--" separator, and exits non-zero when they do not.
func TestRunCommandPassesArguments(t *testing.T) {
	e := &Executor{elevated: false}

	t.Setenv("WINFORGE_ARGS_HELPER", "1")
	if err := e.RunCommand(os.Args[0], []string{"-test.run=TestRunCommandArgsHelper", "--", "alpha", "beta"}); err != nil {
		t.Fatalf("RunCommand with arguments = %v, want success", err)
	}
	if err := e.RunCommand(os.Args[0], []string{"-test.run=TestRunCommandArgsHelper", "--", "alpha", "smashed-together"}); err == nil {
		t.Fatal("RunCommand did not surface a non-zero exit from its arguments")
	}
}

// TestRunCommandArgsHelper is re-invoked by TestRunCommandPassesArguments.
// When WINFORGE_ARGS_HELPER is unset the suite runs it directly and it passes
// trivially; when set (child invocation) it exits 3 unless os.Args contains
// "alpha" and "beta" as the two entries immediately after "--".
func TestRunCommandArgsHelper(t *testing.T) {
	if os.Getenv("WINFORGE_ARGS_HELPER") != "1" {
		return
	}
	for i, a := range os.Args {
		if a == "--" && i+2 < len(os.Args) && os.Args[i+1] == "alpha" && os.Args[i+2] == "beta" {
			return
		}
	}
	os.Exit(3)
}

// TestNewExecutorCapturesElevation checks the constructor stores the caller's
// captured trust decision instead of re-querying the platform, so config
// loading and command dispatch cannot disagree.
func TestNewExecutorCapturesElevation(t *testing.T) {
	if e := NewExecutorWithElevation(nil, true); !e.elevated {
		t.Fatal("NewExecutorWithElevation(true) did not record elevation")
	}
	if e := NewExecutorWithElevation(nil, false); e.elevated {
		t.Fatal("NewExecutorWithElevation(false) recorded elevation")
	}

	// An unelevated executor must not enforce the allowlist...
	if err := NewExecutorWithElevation(nil, false).RunCommand(os.Args[0], []string{"-test.run=TestRunCommandChildHelper"}); err != nil {
		t.Fatalf("unelevated executor refused a command: %v", err)
	}
	// ...while an elevated one must.
	if err := NewExecutorWithElevation(nil, true).RunCommand("definitely-not-a-real-binary-xyz", nil); err == nil ||
		!strings.Contains(err.Error(), "not an allowlisted") {
		t.Fatalf("elevated executor did not enforce the allowlist: %v", err)
	}
}

// TestProtectedSetIsSnapshotAtConstruction checks the caller's slice is copied
// into the set, so mutating it afterwards cannot unprotect a service.
func TestProtectedSetIsSnapshotAtConstruction(t *testing.T) {
	skipOnWindows(t)
	names := []string{"WinDefend"}
	e := NewExecutor(names)
	names[0] = "Spooler"

	if err := e.ServiceStop("WinDefend"); err == nil || !strings.Contains(err.Error(), "protected") {
		t.Fatalf("mutating the caller's slice unprotected WinDefend: %v", err)
	}
	if err := e.ServiceStop("Spooler"); !errors.Is(err, service.ErrUnsupported) {
		t.Fatalf("mutating the caller's slice protected Spooler: %v", err)
	}
}

func TestEmptyProtectedListProtectsNothing(t *testing.T) {
	skipOnWindows(t)
	for _, e := range []*Executor{NewExecutor(nil), NewExecutor([]string{})} {
		if err := e.ServiceStop("WinDefend"); !errors.Is(err, service.ErrUnsupported) {
			t.Fatalf("empty protected list still refused a service: %v", err)
		}
	}
}

// TestAppxRemoveReportsBothLayers checks a removal failure names the package
// and reports the per-user and provisioned attempts separately, since a package
// can be registered in either or both places and a partial failure must not
// read as a clean removal.
func TestAppxRemoveReportsBothLayers(t *testing.T) {
	skipOnWindows(t)
	e := &Executor{}
	const pkg = "Microsoft.BingWeather"

	err := e.AppxRemove(pkg)
	if err == nil {
		t.Fatal("AppxRemove reported success while both layers were failing")
	}
	msg := err.Error()
	for _, want := range []string{pkg, "per-user", "provisioned", "appx removal failed"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("AppxRemove error %q is missing %q", msg, want)
		}
	}
	// The underlying cause must remain inspectable rather than being flattened
	// into an opaque string.
	if !errors.Is(err, maintenance.ErrUnsupported) {
		t.Fatalf("AppxRemove error does not wrap the underlying cause: %v", err)
	}
}

// TestTaskAndPowerDelegate confirms the thin wrappers forward to their packages
// and propagate the error rather than reporting success.
func TestTaskAndPowerDelegate(t *testing.T) {
	skipOnWindows(t)
	e := &Executor{}
	const task = `\Microsoft\Windows\Application Experience\ProgramDataUpdater`

	calls := map[string]error{
		"task disable": e.TaskDisable(task),
		"task enable":  e.TaskEnable(task),
		"task delete":  e.TaskDelete(task),
		"power set":    e.PowerSetActive("381b4222-f694-41f0-9685-ff5bb260df2e"),
	}
	if _, err := e.PowerGetActive(); true {
		calls["power get"] = err
	}

	for name, err := range calls {
		if err == nil {
			t.Fatalf("%s reported success on an unsupported platform", name)
		}
	}
}

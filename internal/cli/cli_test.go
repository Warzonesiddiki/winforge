package cli

import (
	"bytes"
	"errors"
	"flag"
	"net/http"
	"strings"
	"testing"

	"winforge/internal/maintenance"
	"winforge/internal/tweak"
	"winforge/internal/updater"
)

func TestStandaloneAuditLoggerIsDisabledWhileElevated(t *testing.T) {
	t.Setenv("WINFORGE_DATA_DIR", t.TempDir())
	if logger := standaloneAuditLogger(true); logger != nil {
		t.Fatal("elevated standalone maintenance constructed a user-profile audit logger")
	}
	if logger := standaloneAuditLogger(false); logger == nil {
		t.Fatal("standard-user standalone maintenance did not construct an audit logger")
	}
}

// captureOutput redirects package output for the duration of one call and
// returns everything written to it.
func captureOutput(t *testing.T, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	prev := out
	out = &buf
	t.Cleanup(func() { out = prev })
	fn()
	return buf.String()
}

// isolate points the CLI at a scratch data directory so a test can never touch
// the developer's real WinForge state, and suppresses restore-point attempts.
func isolate(t *testing.T) {
	t.Helper()
	t.Setenv("WINFORGE_DATA_DIR", t.TempDir())
	t.Setenv("WINFORGE_NO_RESTORE_POINT", "1")
}

// stubListen replaces the serve loop so a test exercises argument validation
// without binding a socket, and records the address that would have been used.
// Without this a test that stops rejecting a bad host does not fail — it hangs
// in ListenAndServe until the whole package times out.
func stubListen(t *testing.T) *string {
	t.Helper()
	var addr string
	prev := listenAndServe
	listenAndServe = func(srv *http.Server) error {
		addr = srv.Addr
		return errServeStubbed
	}
	t.Cleanup(func() { listenAndServe = prev })
	return &addr
}

var errServeStubbed = errors.New("serve stub: reached ListenAndServe")

// TestRunRejectsUnknownCommand covers the default branch of the argv switch: an
// unrecognised verb must be a reported error, not a silent no-op.
func TestRunRejectsUnknownCommand(t *testing.T) {
	args := []string{
		"frobnicate",
		"--not-a-command",
		"-x",
		"APPLY",  // the switch is deliberately case-sensitive
		"apply ", // a trailing space is not the apply command
		"",
	}
	for _, arg := range args {
		t.Run("arg="+arg, func(t *testing.T) {
			var err error
			captureOutput(t, func() { err = Run([]string{arg}) })
			if err == nil {
				t.Fatalf("Run(%q) accepted an unknown command", arg)
			}
			if !strings.Contains(err.Error(), "unknown command") {
				t.Fatalf("Run(%q) error = %v, want an unknown-command error", arg, err)
			}
		})
	}
}

// TestRunHelpAndVersion pins the two verbs that must always succeed without
// touching machine state.
func TestRunHelpAndVersion(t *testing.T) {
	for _, arg := range []string{"help", "--help", "-h"} {
		t.Run(arg, func(t *testing.T) {
			var err error
			output := captureOutput(t, func() { err = Run([]string{arg}) })
			if err != nil {
				t.Fatalf("Run(%q) = %v, want nil", arg, err)
			}
			if !strings.Contains(output, "Usage:") {
				t.Fatalf("Run(%q) printed no usage text: %q", arg, output)
			}
		})
	}

	for _, arg := range []string{"version", "--version", "-v"} {
		t.Run(arg, func(t *testing.T) {
			var err error
			output := captureOutput(t, func() { err = Run([]string{arg}) })
			if err != nil {
				t.Fatalf("Run(%q) = %v, want nil", arg, err)
			}
			if strings.TrimSpace(output) == "" {
				t.Fatalf("Run(%q) printed no version", arg)
			}
		})
	}
}

// TestRequiredFlagsAreEnforced covers the required-argument guards. Each of
// these returns before any system call, so they are safe on any platform.
func TestRequiredFlagsAreEnforced(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"apply without id", []string{"apply"}, "--id is required"},
		{"undo without id", []string{"undo"}, "--id is required"},
		{"install without id", []string{"install"}, "--id is required"},
		{"set-dns without primary", []string{"set-dns"}, "--primary is required"},
		{"enable-feature without name", []string{"enable-feature"}, "--name is required"},
		{"disable-feature without name", []string{"disable-feature"}, "--name is required"},
		{"build-iso without source", []string{"build-iso", "--output", "out.iso"}, "--source and --output are required"},
		{"build-iso without output", []string{"build-iso", "--source", "src"}, "--source and --output are required"},
		{"list-editions without source", []string{"build-iso", "--list-editions"}, "--source is required"},
		{"search without query", []string{"search"}, "search requires a query"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolate(t)
			var err error
			captureOutput(t, func() { err = Run(tt.args) })
			if err == nil {
				t.Fatalf("Run(%v) accepted a missing required argument", tt.args)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Run(%v) error = %q, want it to contain %q", tt.args, err, tt.want)
			}
		})
	}
}

// TestUndefinedFlagIsReportedNotFatal is the regression guard for the switch
// from flag.ExitOnError to flag.ContinueOnError. Under ExitOnError this test
// could not exist: a single bad flag terminated the whole test binary with
// exit status 2 from inside the flag package.
func TestUndefinedFlagIsReportedNotFatal(t *testing.T) {
	commands := [][]string{
		{"serve", "--bogus"},
		{"apply", "--bogus"},
		{"undo", "--bogus"},
		{"install", "--bogus"},
		{"restore-point", "--bogus"},
		{"build-iso", "--bogus"},
		{"updates", "--bogus"},
		{"set-dns", "--bogus"},
		{"enable-feature", "--bogus"},
		{"disable-feature", "--bogus"},
	}

	for _, args := range commands {
		t.Run(args[0], func(t *testing.T) {
			isolate(t)
			var err error
			captureOutput(t, func() { err = Run(args) })
			if err == nil {
				t.Fatalf("Run(%v) accepted an undefined flag", args)
			}
			if !strings.Contains(err.Error(), "not defined") {
				t.Fatalf("Run(%v) error = %q, want an undefined-flag error", args, err)
			}
		})
	}
}

// TestHelpFlagReturnsErrHelp documents the contract main relies on to exit 0
// for "winforge <command> -h" instead of reporting a spurious failure.
func TestHelpFlagReturnsErrHelp(t *testing.T) {
	for _, cmd := range []string{"serve", "apply", "undo", "install", "build-iso", "updates", "set-dns", "enable-feature"} {
		t.Run(cmd, func(t *testing.T) {
			isolate(t)
			var err error
			output := captureOutput(t, func() { err = Run([]string{cmd, "-h"}) })
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("Run(%q -h) error = %v, want flag.ErrHelp", cmd, err)
			}
			if !strings.Contains(output, "Usage of "+strings.TrimSuffix(cmd, "x")) && !strings.Contains(output, "Usage of ") {
				t.Fatalf("Run(%q -h) printed no usage: %q", cmd, output)
			}
		})
	}
}

// TestServeRejectsNonLoopbackHost guards the security-relevant parse path: the
// dashboard performs elevated operations for anyone who can reach it, so it
// must refuse to bind anything but loopback.
func TestServeRejectsNonLoopbackHost(t *testing.T) {
	hosts := []string{
		"0.0.0.0",
		"::",
		"192.168.1.10",
		"8.8.8.8",
		"example.com",
		"",
		"127.0.0.1 ",
		"127.0.0.1:9999",
		"0177.0.0.1", // octal spelling must not slip through
	}
	for _, host := range hosts {
		t.Run("host="+host, func(t *testing.T) {
			isolate(t)
			stubListen(t)
			var err error
			captureOutput(t, func() { err = Run([]string{"serve", "--host", host, "--no-browser"}) })
			if err == nil {
				t.Fatalf("serve accepted non-loopback host %q", host)
			}
			if errors.Is(err, errServeStubbed) {
				t.Fatalf("serve tried to bind non-loopback host %q", host)
			}
			if !strings.Contains(err.Error(), "loopback") {
				t.Fatalf("serve --host %q error = %q, want a loopback error", host, err)
			}
		})
	}
}

// TestServeRejectsInvalidPort covers the boundaries of the valid port range.
func TestServeRejectsInvalidPort(t *testing.T) {
	ports := []string{"0", "-1", "65536", "99999", "abc", "", "8696.5", " 8696", "0x21F8", "+8696 "}
	for _, port := range ports {
		t.Run("port="+port, func(t *testing.T) {
			isolate(t)
			stubListen(t)
			var err error
			captureOutput(t, func() {
				err = Run([]string{"serve", "--host", "127.0.0.1", "--port", port, "--no-browser"})
			})
			if err == nil {
				t.Fatalf("serve accepted invalid port %q", port)
			}
			if errors.Is(err, errServeStubbed) {
				t.Fatalf("serve tried to bind invalid port %q", port)
			}
			if !strings.Contains(err.Error(), "port") {
				t.Fatalf("serve --port %q error = %q, want a port error", port, err)
			}
		})
	}
}

// TestServeAcceptsLoopbackAliases proves the host check accepts the forms it is
// meant to, so the guard above is not simply rejecting everything, and pins the
// address actually handed to the server. "localhost" must be resolved to the
// literal 127.0.0.1 rather than trusting a hosts-file mapping for what is a
// security boundary.
func TestServeAcceptsLoopbackAliases(t *testing.T) {
	tests := []struct{ host, wantAddr string }{
		{"127.0.0.1", "127.0.0.1:8696"},
		{"localhost", "127.0.0.1:8696"},
		{"LocalHost", "127.0.0.1:8696"},
		{"127.0.0.2", "127.0.0.2:8696"},
		{"::1", "[::1]:8696"},
	}
	for _, tt := range tests {
		t.Run("host="+tt.host, func(t *testing.T) {
			isolate(t)
			addr := stubListen(t)
			var err error
			captureOutput(t, func() {
				err = Run([]string{"serve", "--host", tt.host, "--no-browser"})
			})
			if !errors.Is(err, errServeStubbed) {
				t.Fatalf("serve --host %q = %v, want it to reach the listener", tt.host, err)
			}
			if *addr != tt.wantAddr {
				t.Fatalf("serve --host %q bound %q, want %q", tt.host, *addr, tt.wantAddr)
			}
		})
	}
}

// TestInstallValidatesPackageID ensures an option-like or malformed package id
// is rejected in the CLI before it can reach a winget command line.
func TestInstallValidatesPackageID(t *testing.T) {
	ids := []string{
		"--source=evil",
		"-x",
		"has space",
		"Trailing...",
		"Truncated…",
		strings.Repeat("a", 200) + ".Pkg",
		"Vendor.Pkg\\..\\evil",
		"Vendor.Pkg|calc",
	}
	for _, id := range ids {
		t.Run("id="+id, func(t *testing.T) {
			isolate(t)
			var err error
			captureOutput(t, func() { err = Run([]string{"install", "--id", id}) })
			if err == nil {
				t.Fatalf("install accepted malformed package id %q", id)
			}
			if !strings.Contains(err.Error(), "invalid WinGet package id") {
				t.Fatalf("install --id %q error = %q, want a package-id validation error", id, err)
			}
		})
	}
}

// TestSetDnsValidatesServers checks DNS arguments are validated before any
// restore point is created or any adapter is touched.
func TestSetDnsValidatesServers(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"primary out of range", []string{"set-dns", "--primary", "999.1.1.1"}},
		{"primary not an ip", []string{"set-dns", "--primary", "not-an-ip"}},
		{"secondary out of range", []string{"set-dns", "--primary", "1.1.1.1", "--secondary", "999.1.1.1"}},
		{"primary with port", []string{"set-dns", "--primary", "1.1.1.1:53"}},
		{"primary with spaces", []string{"set-dns", "--primary", " 1.1.1.1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolate(t)
			var err error
			captureOutput(t, func() { err = Run(tt.args) })
			if err == nil {
				t.Fatalf("set-dns accepted invalid arguments %v", tt.args)
			}
			if !strings.Contains(err.Error(), "DNS server") {
				t.Fatalf("set-dns %v error = %q, want a DNS validation error", tt.args, err)
			}
		})
	}
}

// featureNamesRejected are values that must never reach a DISM command line.
var featureNamesRejected = []string{
	"bad name",
	"name;rm -rf /",
	"name&whoami",
	"name|calc",
	"name$(id)",
	" leading",
	strings.Repeat("a", 300),
}

// TestFeatureValidationIsDefenceInDepth pins the second validation layer.
// featureCmd checks the name itself, but maintenance.EnableFeature re-checks it
// so a caller that bypasses the CLI cannot reach DISM with a crafted name.
// Both layers are asserted so that removing either one is caught.
func TestFeatureValidationIsDefenceInDepth(t *testing.T) {
	for _, name := range featureNamesRejected {
		t.Run(name, func(t *testing.T) {
			if err := maintenance.EnableFeature(name, nil); err == nil ||
				!strings.Contains(err.Error(), "invalid Windows feature name") {
				t.Fatalf("maintenance.EnableFeature(%q) = %v, want a validation error", name, err)
			}
			if err := maintenance.DisableFeature(name, nil); err == nil ||
				!strings.Contains(err.Error(), "invalid Windows feature name") {
				t.Fatalf("maintenance.DisableFeature(%q) = %v, want a validation error", name, err)
			}
		})
	}
}

// TestFeatureCommandValidatesName keeps shell-significant characters out of the
// DISM feature name.
func TestFeatureCommandValidatesName(t *testing.T) {
	for _, verb := range []string{"enable-feature", "disable-feature"} {
		for _, name := range featureNamesRejected {
			t.Run(verb+"/"+name, func(t *testing.T) {
				isolate(t)
				var err error
				captureOutput(t, func() { err = Run([]string{verb, "--name", name}) })
				if err == nil {
					t.Fatalf("%s accepted invalid feature name %q", verb, name)
				}
				if !strings.Contains(err.Error(), "invalid Windows feature name") {
					t.Fatalf("%s --name %q error = %q, want a feature-name validation error", verb, name, err)
				}
			})
		}
	}
}

// TestMaintenanceCmdRejectsUnknownName covers the defensive default branch of
// the internal maintenance dispatcher.
func TestMaintenanceCmdRejectsUnknownName(t *testing.T) {
	isolate(t)
	var err error
	captureOutput(t, func() { err = maintenanceCmd("not-a-real-command", nil) })
	if err == nil {
		t.Fatal("maintenanceCmd accepted an unknown maintenance command")
	}
	if !strings.Contains(err.Error(), "unknown maintenance command") {
		t.Fatalf("error = %q, want an unknown-maintenance-command error", err)
	}
}

func TestStringListFlagCollectsRepeatedValues(t *testing.T) {
	var editions stringListFlag
	if got := editions.String(); got != "" {
		t.Fatalf("empty stringListFlag.String() = %q, want empty", got)
	}
	for _, v := range []string{"Pro", "Home", "Enterprise"} {
		if err := editions.Set(v); err != nil {
			t.Fatalf("Set(%q): %v", v, err)
		}
	}
	if got, want := len(editions), 3; got != want {
		t.Fatalf("collected %d editions, want %d", got, want)
	}
	if got, want := editions.String(), "Pro,Home,Enterprise"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

// TestBuildIsoCollectsRepeatedEditions exercises the repeatable --edition flag
// through the real flag set rather than the Value type alone.
func TestBuildIsoCollectsRepeatedEditions(t *testing.T) {
	fs := newFlagSet("build-iso")
	var editions stringListFlag
	fs.Var(&editions, "edition", "edition name to keep")
	if err := fs.Parse([]string{"--edition", "Pro", "--edition", "Home"}); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got, want := editions.String(), "Pro,Home"; got != want {
		t.Fatalf("editions = %q, want %q", got, want)
	}
}

func TestUpdateStatePrecedence(t *testing.T) {
	tests := []struct {
		name string
		u    updater.Update
		want string
	}{
		{"installed wins", updater.Update{IsInstalled: true, IsHidden: true, IsDownloaded: true}, "installed"},
		{"hidden before downloaded", updater.Update{IsHidden: true, IsDownloaded: true}, "hidden"},
		{"downloaded", updater.Update{IsDownloaded: true}, "downloaded"},
		{"available", updater.Update{}, "available"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := updateState(tt.u); got != tt.want {
				t.Fatalf("updateState = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestPrintResultReportsFailures checks the result summary names the operations
// that failed, since that output is how a CLI user learns a tweak only applied
// partially.
func TestPrintResultReportsFailures(t *testing.T) {
	res := tweak.Result{
		TweakID:   "disable-telemetry",
		Succeeded: 1,
		Failed:    1,
		Changed:   1,
		Effects: []tweak.Effect{
			{OperationType: "registry", Target: `HKLM\Software\Good`, Applied: true, Changed: true},
			{OperationType: "service", Target: "DiagTrack", Err: errors.New("access denied")},
		},
	}

	output := captureOutput(t, func() { printResult(res) })
	if !strings.Contains(output, "applied disable-telemetry") {
		t.Fatalf("summary line missing: %q", output)
	}
	if !strings.Contains(output, "1 changed, 1 succeeded, 1 failed") {
		t.Fatalf("counts missing: %q", output)
	}
	if !strings.Contains(output, "DiagTrack") || !strings.Contains(output, "access denied") {
		t.Fatalf("failing operation not reported: %q", output)
	}
	if strings.Contains(output, `HKLM\Software\Good`) {
		t.Fatalf("succeeding operation was listed as a failure: %q", output)
	}
}

func TestPrintResultMarksDryRun(t *testing.T) {
	res := tweak.Result{TweakID: "disable-telemetry", DryRun: true}
	output := captureOutput(t, func() { printResult(res) })
	if !strings.HasPrefix(output, "dry-run ") {
		t.Fatalf("dry-run result was not labelled: %q", output)
	}
}

func TestLogToStdoutWritesLine(t *testing.T) {
	output := captureOutput(t, func() { logToStdout("progress line") })
	if output != "progress line\n" {
		t.Fatalf("logToStdout wrote %q, want %q", output, "progress line\n")
	}
}

// TestNewFlagSetWritesUsageToInjectedWriter confirms flag output follows the
// package writer, so a parse failure cannot bypass a caller's capture and
// cannot terminate the process.
func TestNewFlagSetWritesUsageToInjectedWriter(t *testing.T) {
	var err error
	output := captureOutput(t, func() {
		fs := newFlagSet("example")
		fs.String("known", "", "a known flag")
		err = fs.Parse([]string{"--unknown"})
	})
	if err == nil {
		t.Fatal("Parse accepted an unknown flag")
	}
	if !strings.Contains(output, "unknown") {
		t.Fatalf("flag set did not write usage to the injected writer: %q", output)
	}
}

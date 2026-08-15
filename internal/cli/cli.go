// Package cli implements the command-line interface. When run with no
// arguments, winforge starts the local dashboard server (the default action
// for double-clicking the exe).
package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"winforge/internal/app"
	"winforge/internal/appmanager"
	"winforge/internal/audit"
	"winforge/internal/httpapi"
	"winforge/internal/isobuilder"
	"winforge/internal/maintenance"
	"winforge/internal/platform"
	"winforge/internal/restorepoint"
	"winforge/internal/tweak"
	"winforge/internal/updater"
)

// out is the destination for command output. It is a package-level variable
// rather than a hard-coded os.Stdout so tests can capture what a command
// prints; production never reassigns it.
var out io.Writer = os.Stdout

// newFlagSet builds a flag set that reports parse failures to the caller
// instead of terminating the process.
//
// flag.ExitOnError calls os.Exit(2) from inside Parse. That bypasses main's
// error path (which exits 1 after printing "winforge: <err>"), produces an
// inconsistent exit code for what is simply bad input, and makes the argument
// parser — the one component that always handles untrusted argv — impossible
// to test. Usage output is routed to the same writer so a failed parse cannot
// interleave onto a different stream than the error itself.
func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(out)
	return fs
}

// listenAndServe runs the dashboard server. It is a variable so tests can
// exercise serve's argument validation without binding a socket; production
// never reassigns it.
var listenAndServe = func(srv *http.Server) error { return srv.ListenAndServe() }

// Run executes the CLI with the given arguments (excluding the program name).
func Run(args []string) error {
	if len(args) == 0 {
		return serve([]string{})
	}

	switch args[0] {
	case "serve", "ui":
		return serve(args[1:])
	case "apply":
		return applyCmd(args[1:])
	case "undo":
		return undoCmd(args[1:])
	case "list":
		return listCmd()
	case "scan", "health":
		return scanCmd()
	case "history":
		return historyCmd()
	case "plugins":
		return pluginsCmd()
	case "install":
		return installCmd(args[1:])
	case "search":
		return searchCmd(args[1:])
	case "restore-point":
		return restorePointCmd(args[1:])
	case "restore-points":
		return listRestorePointsCmd()
	case "run-maintenance":
		return runMaintenanceCmd()
	case "schedule":
		return scheduleCmd(true)
	case "unschedule":
		return scheduleCmd(false)
	case "build-iso":
		return buildIsoCmd(args[1:])
	case "updates":
		return updatesCmd(args[1:])
	case "install-updates":
		return installUpdatesCmd(args[1:])
	case "reset-windows-update":
		return maintenanceCmd("reset-windows-update", args[1:])
	case "repair-image":
		return maintenanceCmd("repair-image", args[1:])
	case "flush-dns":
		return maintenanceCmd("flush-dns", args[1:])
	case "network-reset":
		return maintenanceCmd("network-reset", args[1:])
	case "set-dns":
		return setDnsCmd(args[1:])
	case "enable-feature":
		return featureCmd(args[1:], true)
	case "disable-feature":
		return featureCmd(args[1:], false)
	case "version", "--version", "-v":
		fmt.Fprintln(out, app.Version)
		return nil
	case "help", "--help", "-h":
		usage()
		return nil
	default:
		usage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func usage() {
	fmt.Fprint(out, `WinForge — self-contained Windows tuning & maintenance toolkit

Usage:
  winforge                 start the dashboard server (default)
  winforge serve [--host 127.0.0.1] [--port 8696] [--no-browser]
  winforge list            list all tweaks and their applied state
  winforge scan            show the system health score
  winforge apply  --id <id> [--dry-run]
  winforge undo   --id <id>
  winforge history         show the operation history
  winforge plugins         list installed plugins
  winforge install --id <winget-id>
  winforge search  <query>
  winforge restore-point [--description "…"]
  winforge restore-points    list existing restore points
  winforge run-maintenance   verify tweak states + upgrade apps
  winforge schedule          register the weekly maintenance task
  winforge unschedule        remove the weekly maintenance task
  winforge build-iso --source <dir> --output <iso> [--label <label>] [--edition <name>]...
  winforge build-iso --source <dir> --list-editions
  winforge updates [--installed]       search Windows Update
  winforge install-updates             download + install available updates
  winforge reset-windows-update | repair-image | flush-dns | network-reset
  winforge set-dns --primary <ip> [--secondary <ip>] [--adapter <name>]
  winforge enable-feature  --name <feature>
  winforge disable-feature --name <feature>
  winforge version
`)
}

// newApp builds the application from the default data directory, respecting an
// optional WINFORGE_DATA_DIR override.
func newApp() (*app.App, error) {
	dir := os.Getenv("WINFORGE_DATA_DIR")
	return app.New(dir)
}

func standaloneAuditLogger(elevated bool) *audit.Logger {
	if elevated {
		return nil
	}
	dataDir := os.Getenv("WINFORGE_DATA_DIR")
	if dataDir == "" {
		dataDir = app.DefaultDataDir()
	}
	return audit.NewLogger(filepath.Join(dataDir, "logs"))
}

func ensureRestorePoint(description string) {
	// Standalone maintenance commands do not otherwise need the full config.
	// Build only the safety dependencies so a malformed optional override cannot
	// turn a best-effort restore point into a blocker for an unrelated repair.
	a := &app.App{
		Logger:           standaloneAuditLogger(platform.IsElevated()),
		AutoRestorePoint: os.Getenv("WINFORGE_NO_RESTORE_POINT") == "",
	}
	a.EnsureRestorePoint(description)
}

func serve(args []string) error {
	fs := newFlagSet("serve")
	host := fs.String("host", "127.0.0.1", "bind address")
	port := fs.String("port", "8696", "bind port")
	noBrowser := fs.Bool("no-browser", false, "do not open the browser")
	if err := fs.Parse(args); err != nil {
		return err
	}

	hostValue := *host
	if strings.EqualFold(hostValue, "localhost") {
		// Do not rely on a hosts-file mapping for the loopback-only security
		// boundary; bind the literal address after accepting the friendly alias.
		hostValue = "127.0.0.1"
	}
	ip := net.ParseIP(hostValue)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("dashboard host must be a loopback address")
	}
	portNumber, err := strconv.Atoi(*port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return fmt.Errorf("dashboard port must be an integer between 1 and 65535")
	}

	a, err := newApp()
	if err != nil {
		return err
	}

	addr := net.JoinHostPort(hostValue, strconv.Itoa(portNumber))
	srv := &http.Server{
		Addr:              addr,
		Handler:           httpapi.New(a),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	url := "http://" + addr + "/"
	fmt.Fprintf(out, "WinForge dashboard: %s\n", url)
	fmt.Fprintf(out, "Elevated: %v\n", a.Elevated())
	fmt.Fprintf(out, "(Press Ctrl+C to stop.)\n")

	if !*noBrowser {
		openBrowser(url)
	}
	return listenAndServe(srv)
}

func applyCmd(args []string) error {
	fs := newFlagSet("apply")
	id := fs.String("id", "", "tweak id to apply")
	dryRun := fs.Bool("dry-run", false, "simulate without applying")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *id == "" {
		return fmt.Errorf("--id is required")
	}
	a, err := newApp()
	if err != nil {
		return err
	}
	res, err := a.Apply(*id, *dryRun)
	if err != nil {
		return err
	}
	printResult(res)
	return res.Failure()
}

func undoCmd(args []string) error {
	fs := newFlagSet("undo")
	id := fs.String("id", "", "tweak id to undo")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *id == "" {
		return fmt.Errorf("--id is required")
	}
	a, err := newApp()
	if err != nil {
		return err
	}
	res, err := a.Undo(*id)
	if err != nil {
		return err
	}
	printResult(res)
	return res.Failure()
}

func printResult(res tweak.Result) {
	mode := "applied"
	if res.DryRun {
		mode = "dry-run"
	}
	fmt.Fprintf(out, "%s %s: %d changed, %d succeeded, %d failed\n",
		mode, res.TweakID, res.Changed, res.Succeeded, res.Failed)
	for _, e := range res.Effects {
		if e.Err != nil {
			fmt.Fprintf(out, "  ! %s %s: %v\n", e.OperationType, e.Target, e.Err)
		}
	}
}

func listCmd() error {
	a, err := newApp()
	if err != nil {
		return err
	}
	applied := a.AppliedMap()
	for _, t := range a.Tweaks {
		state := "not applied"
		if applied[t.ID] {
			state = "applied"
		}
		fmt.Fprintf(out, "[%s] %-8s %-28s %s\n", state, t.Risk, t.ID, t.Name)
	}
	return nil
}

func scanCmd() error {
	a, err := newApp()
	if err != nil {
		return err
	}
	h := a.Health(a.BloatwareCount())
	fmt.Fprintf(out, "Health score: %d/100\n", h.Score)
	fmt.Fprintf(out, "  tweaks: %d/%d applied\n", h.AppliedTweaks, h.TotalTweaks)
	fmt.Fprintf(out, "  unapplied low/medium/high: %d/%d/%d\n", h.UnappliedLow, h.UnappliedMedium, h.UnappliedHigh)
	fmt.Fprintf(out, "  bloatware: %d\n", h.BloatwareCount)
	for _, b := range a.Bloatware() {
		fmt.Fprintf(out, "    - %s\n", b)
	}
	return nil
}

func pluginsCmd() error {
	a, err := newApp()
	if err != nil {
		return err
	}
	if len(a.Plugins) == 0 {
		fmt.Fprintf(out, "no plugins installed (add a manifest.json to a folder under %s)\n",
			filepath.Join(a.DataDir, "plugins"))
		return nil
	}
	for _, p := range a.Plugins {
		fmt.Fprintf(out, "%-24s v%-10s %3d tweaks  %s\n", p.ID, p.Version, len(p.Tweaks), p.Name)
	}
	return nil
}

func historyCmd() error {
	a, err := newApp()
	if err != nil {
		return err
	}
	entries, readErr := a.History()
	for _, e := range entries {
		status := "ok"
		if !e.Success {
			status = "failed"
		}
		fmt.Fprintf(out, "%s  %-8s %-24s %s\n", e.Timestamp.Format("2006-01-02 15:04:05"), status, e.OperationType, e.Target)
	}
	return readErr
}

func installCmd(args []string) error {
	fs := newFlagSet("install")
	id := fs.String("id", "", "winget package id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *id == "" {
		return fmt.Errorf("--id is required")
	}
	if err := appmanager.ValidatePackageID(*id); err != nil {
		return err
	}
	a, err := newApp()
	if err != nil {
		return err
	}
	res, err := a.InstallPackage(context.Background(), *id, func(p appmanager.Progress) {
		if p.Line != "" {
			fmt.Fprintln(out, p.Line)
		}
	})
	if err != nil {
		return err
	}
	if !res.Success {
		return fmt.Errorf("install failed (winget exited non-zero)")
	}
	return nil
}

func searchCmd(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("search requires a query")
	}
	a, err := newApp()
	if err != nil {
		return err
	}
	ids, err := a.SearchPackages(context.Background(), strings.Join(args, " "))
	if err != nil {
		return err
	}
	for _, id := range ids {
		fmt.Fprintln(out, id)
	}
	return nil
}

func restorePointCmd(args []string) error {
	fs := newFlagSet("restore-point")
	desc := fs.String("description", "WinForge restore point", "restore point description")
	if err := fs.Parse(args); err != nil {
		return err
	}
	a, err := newApp()
	if err != nil {
		return err
	}
	info, err := a.CreateRestorePoint(*desc)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "restore point created: sequence=%d\n", info.SequenceNumber)
	return nil
}

func listRestorePointsCmd() error {
	points, err := restorepoint.List()
	if err != nil {
		return err
	}
	if len(points) == 0 {
		fmt.Fprintln(out, "no restore points found")
		return nil
	}
	for _, p := range points {
		fmt.Fprintf(out, "%-6d %s  %s\n", p.SequenceNumber, p.CreatedAt.Format("2006-01-02 15:04:05"), p.Description)
	}
	return nil
}

func runMaintenanceCmd() error {
	a, err := newApp()
	if err != nil {
		return err
	}
	sum := a.RunMaintenance(context.Background(), logToStdout)
	for _, e := range sum.TweakErrors {
		fmt.Fprintln(out, "tweak error:", e)
	}
	if sum.AppError != "" {
		fmt.Fprintln(out, "app update error:", sum.AppError)
	}
	if sum.AuditError != "" {
		fmt.Fprintln(out, "audit error:", sum.AuditError)
	}
	fmt.Fprintf(out, "maintenance complete: %d tweaks applied", len(sum.TweaksApplied))
	switch {
	case sum.AppsUpgraded:
		fmt.Fprint(out, ", apps updated")
	case sum.AppsSkipped:
		fmt.Fprint(out, ", app updates skipped (winget missing)")
	}
	fmt.Fprintln(out)
	if len(sum.TweakErrors) > 0 || sum.AppError != "" || sum.AuditError != "" {
		return fmt.Errorf("maintenance completed with errors")
	}
	return nil
}

func scheduleCmd(register bool) error {
	a, err := newApp()
	if err != nil {
		return err
	}
	if register {
		if err := a.ScheduleMaintenance(); err != nil {
			return err
		}
		fmt.Fprintf(out, "scheduled weekly maintenance task %q\n", app.MaintenanceTaskName)
		return nil
	}
	if err := a.UnscheduleMaintenance(); err != nil {
		return err
	}
	fmt.Fprintf(out, "removed scheduled maintenance task %q\n", app.MaintenanceTaskName)
	return nil
}

// stringListFlag collects repeated flag values (e.g. --edition a --edition b).
type stringListFlag []string

func (s *stringListFlag) String() string { return strings.Join(*s, ",") }

func (s *stringListFlag) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func buildIsoCmd(args []string) error {
	fs := newFlagSet("build-iso")
	source := fs.String("source", "", "extracted Windows installation source directory")
	output := fs.String("output", "", "output .iso path")
	label := fs.String("label", "", "ISO volume label")
	listOnly := fs.Bool("list-editions", false, "list editions in the source image and exit")
	var editions stringListFlag
	fs.Var(&editions, "edition", "edition name to keep (repeatable; default keeps all)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *listOnly {
		if *source == "" {
			return fmt.Errorf("--source is required with --list-editions")
		}
		eds, err := isobuilder.ListEditions(*source)
		if err != nil {
			return err
		}
		if len(eds) == 0 {
			fmt.Fprintln(out, "no editions found")
			return nil
		}
		for _, e := range eds {
			fmt.Fprintf(out, "%d: %s\n", e.Index, e.Name)
		}
		return nil
	}

	if *source == "" || *output == "" {
		return fmt.Errorf("--source and --output are required")
	}
	opts := isobuilder.Options{
		SourceDir: *source,
		OutputISO: *output,
		Label:     *label,
		Editions:  editions,
		Log:       logToStdout,
	}
	res, err := isobuilder.Build(opts)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "ISO built: %s\n", res.ISO)
	return nil
}

func updatesCmd(args []string) error {
	fs := newFlagSet("updates")
	installed := fs.Bool("installed", false, "list installed updates instead of available")
	if err := fs.Parse(args); err != nil {
		return err
	}

	updates, err := updater.Search(*installed)
	if err != nil {
		return err
	}
	if len(updates) == 0 {
		fmt.Fprintln(out, "no updates found")
		return nil
	}
	for _, u := range updates {
		fmt.Fprintf(out, "[%s] %s\n", updateState(u), u.Title)
	}
	return nil
}

func installUpdatesCmd(args []string) error {
	_ = args
	ensureRestorePoint("WinForge: install Windows updates")
	fmt.Fprintln(out, "Downloading and installing updates (this can take a while)…")
	res, err := updater.InstallAll()
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "result: %s", res.ResultCode)
	if res.RebootRequired {
		fmt.Fprint(out, " (reboot required)")
	}
	fmt.Fprintln(out)
	if res.ResultCode != updater.ResultSucceeded && res.ResultCode != updater.ResultSucceededWithErrors {
		return fmt.Errorf("install result: %s", res.ResultCode)
	}
	return nil
}

func updateState(u updater.Update) string {
	switch {
	case u.IsInstalled:
		return "installed"
	case u.IsHidden:
		return "hidden"
	case u.IsDownloaded:
		return "downloaded"
	default:
		return "available"
	}
}

// logToStdout prints maintenance progress lines to stdout.
func logToStdout(line string) { fmt.Fprintln(out, line) }

func maintenanceCmd(name string, args []string) error {
	_ = args // maintenance commands take no flags today
	if name != "flush-dns" {
		ensureRestorePoint("WinForge: " + name)
	}
	fmt.Fprintf(out, "Running %s…\n", name)
	var err error
	switch name {
	case "reset-windows-update":
		err = maintenance.ResetWindowsUpdate(logToStdout)
	case "repair-image":
		err = maintenance.RepairImage(logToStdout)
	case "flush-dns":
		err = maintenance.FlushDNS()
	case "network-reset":
		err = maintenance.NetworkReset(logToStdout)
	default:
		err = fmt.Errorf("unknown maintenance command %q", name)
	}
	if err != nil {
		return err
	}
	fmt.Fprintln(out, "done")
	return nil
}

func setDnsCmd(args []string) error {
	fs := newFlagSet("set-dns")
	primary := fs.String("primary", "", "primary DNS server")
	secondary := fs.String("secondary", "", "secondary DNS server (optional)")
	adapter := fs.String("adapter", "", "adapter name (default: all active adapters)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *primary == "" {
		return fmt.Errorf("--primary is required")
	}
	var validationErr error
	if *adapter != "" {
		validationErr = maintenance.ValidateDnsSettings(*adapter, *primary, *secondary)
	} else {
		validationErr = maintenance.ValidateDnsServers(*primary, *secondary)
	}
	if validationErr != nil {
		return validationErr
	}
	ensureRestorePoint("WinForge: change DNS settings")

	var err error
	if *adapter != "" {
		err = maintenance.SetDns(*adapter, *primary, *secondary)
	} else {
		err = maintenance.SetDnsOnAll(*primary, *secondary)
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "DNS set to %s", *primary)
	if *secondary != "" {
		fmt.Fprintf(out, " / %s", *secondary)
	}
	fmt.Fprintln(out)
	return nil
}

func featureCmd(args []string, enable bool) error {
	verb := "disable"
	if enable {
		verb = "enable"
	}
	fs := newFlagSet(verb + "-feature")
	name := fs.String("name", "", "Windows feature name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *name == "" {
		return fmt.Errorf("--name is required")
	}
	if err := maintenance.ValidateFeatureName(*name); err != nil {
		return err
	}
	ensureRestorePoint("WinForge: " + verb + " Windows feature " + *name)
	if enable {
		return maintenance.EnableFeature(*name, logToStdout)
	}
	return maintenance.DisableFeature(*name, logToStdout)
}

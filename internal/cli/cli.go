// Package cli implements the command-line interface. When run with no
// arguments, winforge starts the local dashboard server (the default action
// for double-clicking the exe).
package cli

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"winforge/internal/app"
	"winforge/internal/appmanager"
	"winforge/internal/httpapi"
	"winforge/internal/maintenance"
	"winforge/internal/platform"
	"winforge/internal/restorepoint"
	"winforge/internal/tweak"
)

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
		fmt.Println(app.Version)
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
	fmt.Print(`WinForge — self-contained Windows tuning & maintenance toolkit

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

func serve(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	host := fs.String("host", "127.0.0.1", "bind address")
	port := fs.String("port", "8696", "bind port")
	noBrowser := fs.Bool("no-browser", false, "do not open the browser")
	_ = fs.Parse(args)

	a, err := newApp()
	if err != nil {
		return err
	}

	addr := net.JoinHostPort(*host, *port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           httpapi.New(a),
		ReadHeaderTimeout: 10 * time.Second,
	}

	url := "http://" + addr + "/"
	fmt.Printf("WinForge dashboard: %s\n", url)
	fmt.Printf("Elevated: %v\n", platform.IsElevated())
	fmt.Printf("(Press Ctrl+C to stop.)\n")

	if !*noBrowser {
		openBrowser(url)
	}
	return srv.ListenAndServe()
}

func applyCmd(args []string) error {
	fs := flag.NewFlagSet("apply", flag.ExitOnError)
	id := fs.String("id", "", "tweak id to apply")
	dryRun := fs.Bool("dry-run", false, "simulate without applying")
	_ = fs.Parse(args)
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
	return nil
}

func undoCmd(args []string) error {
	fs := flag.NewFlagSet("undo", flag.ExitOnError)
	id := fs.String("id", "", "tweak id to undo")
	_ = fs.Parse(args)
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
	return nil
}

func printResult(res tweak.Result) {
	mode := "applied"
	if res.DryRun {
		mode = "dry-run"
	}
	fmt.Printf("%s %s: %d changed, %d succeeded, %d failed\n",
		mode, res.TweakID, res.Changed, res.Succeeded, res.Failed)
	for _, e := range res.Effects {
		if e.Err != nil {
			fmt.Printf("  ! %s %s: %v\n", e.OperationType, e.Target, e.Err)
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
		fmt.Printf("[%s] %-8s %-28s %s\n", state, t.Risk, t.ID, t.Name)
	}
	return nil
}

func scanCmd() error {
	a, err := newApp()
	if err != nil {
		return err
	}
	h := a.Health(0)
	fmt.Printf("Health score: %d/100\n", h.Score)
	fmt.Printf("  tweaks: %d/%d applied\n", h.AppliedTweaks, h.TotalTweaks)
	fmt.Printf("  unapplied low/medium/high: %d/%d/%d\n", h.UnappliedLow, h.UnappliedMedium, h.UnappliedHigh)
	fmt.Printf("  bloatware: %d\n", h.BloatwareCount)
	return nil
}

func pluginsCmd() error {
	a, err := newApp()
	if err != nil {
		return err
	}
	if len(a.Plugins) == 0 {
		fmt.Printf("no plugins installed (add a manifest.json to a folder under %s)\n",
			filepath.Join(a.DataDir, "plugins"))
		return nil
	}
	for _, p := range a.Plugins {
		fmt.Printf("%-24s v%-10s %3d tweaks  %s\n", p.ID, p.Version, len(p.Tweaks), p.Name)
	}
	return nil
}

func historyCmd() error {
	a, err := newApp()
	if err != nil {
		return err
	}
	entries, err := a.History()
	if err != nil {
		return err
	}
	for _, e := range entries {
		status := "ok"
		if !e.Success {
			status = "failed"
		}
		fmt.Printf("%s  %-8s %-24s %s\n", e.Timestamp.Format("2006-01-02 15:04:05"), status, e.OperationType, e.Target)
	}
	return nil
}

func installCmd(args []string) error {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	id := fs.String("id", "", "winget package id")
	_ = fs.Parse(args)
	if *id == "" {
		return fmt.Errorf("--id is required")
	}
	a, err := newApp()
	if err != nil {
		return err
	}
	res, err := a.Packages.Install(context.Background(), *id, func(p appmanager.Progress) {
		if p.Line != "" {
			fmt.Println(p.Line)
		}
	})
	if err != nil {
		return err
	}
	if !res.Success {
		return fmt.Errorf("install failed (winget exited non-zero)")
	}
	fmt.Println(strings.Join(res.Lines, "\n"))
	return nil
}

func searchCmd(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("search requires a query")
	}
	a, err := newApp()
	if err != nil {
		return err
	}
	ids, err := a.Packages.Search(context.Background(), args[1])
	if err != nil {
		return err
	}
	for _, id := range ids {
		fmt.Println(id)
	}
	return nil
}

func restorePointCmd(args []string) error {
	fs := flag.NewFlagSet("restore-point", flag.ExitOnError)
	desc := fs.String("description", "WinForge restore point", "restore point description")
	_ = fs.Parse(args)
	info, err := restorepoint.Create(*desc)
	if err != nil {
		return err
	}
	fmt.Printf("restore point created: sequence=%d\n", info.SequenceNumber)
	return nil
}

// logToStdout prints maintenance progress lines to stdout.
func logToStdout(line string) { fmt.Println(line) }

func maintenanceCmd(name string, args []string) error {
	_ = args // maintenance commands take no flags today
	fmt.Printf("Running %s…\n", name)
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
	fmt.Println("done")
	return nil
}

func setDnsCmd(args []string) error {
	fs := flag.NewFlagSet("set-dns", flag.ExitOnError)
	primary := fs.String("primary", "", "primary DNS server")
	secondary := fs.String("secondary", "", "secondary DNS server (optional)")
	adapter := fs.String("adapter", "", "adapter name (default: all active adapters)")
	_ = fs.Parse(args)
	if *primary == "" {
		return fmt.Errorf("--primary is required")
	}

	var err error
	if *adapter != "" {
		err = maintenance.SetDns(*adapter, *primary, *secondary)
	} else {
		err = maintenance.SetDnsOnAll(*primary, *secondary)
	}
	if err != nil {
		return err
	}
	fmt.Printf("DNS set to %s", *primary)
	if *secondary != "" {
		fmt.Printf(" / %s", *secondary)
	}
	fmt.Println()
	return nil
}

func featureCmd(args []string, enable bool) error {
	verb := "disable"
	if enable {
		verb = "enable"
	}
	fs := flag.NewFlagSet(verb+"-feature", flag.ExitOnError)
	name := fs.String("name", "", "Windows feature name")
	_ = fs.Parse(args)
	if *name == "" {
		return fmt.Errorf("--name is required")
	}
	if enable {
		return maintenance.EnableFeature(*name, logToStdout)
	}
	return maintenance.DisableFeature(*name, logToStdout)
}

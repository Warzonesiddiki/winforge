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
	"strings"
	"time"

	"winforge/internal/app"
	"winforge/internal/appmanager"
	"winforge/internal/httpapi"
	"winforge/internal/platform"
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
	case "install":
		return installCmd(args[1:])
	case "search":
		return searchCmd(args[1:])
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
  winforge install --id <winget-id>
  winforge search  <query>
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

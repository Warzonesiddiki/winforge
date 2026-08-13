// Package appmanager wraps winget.exe for install/uninstall/search, streaming
// stdout line-by-line so the UI can render progress.
package appmanager

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"
)

// ErrWingetMissing is returned when winget.exe is not on PATH.
var ErrWingetMissing = errors.New("winget.exe not found; install the App Installer from the Microsoft Store")

// Progress carries one line of winget output plus a terminal flag.
type Progress struct {
	Line string
	Done bool
}

// Result summarizes a completed winget operation.
type Result struct {
	Success bool
	Lines   []string
	Error   error
}

// Manager executes winget operations.
type Manager struct{}

// New creates a Manager.
func New() *Manager { return &Manager{} }

// report streams lines to progress, returning the collected lines and whether
// the underlying command exited successfully.
func (m *Manager) report(ctx context.Context, args []string, progress func(Progress)) ([]string, bool, error) {
	if _, err := exec.LookPath("winget"); err != nil {
		return nil, false, ErrWingetMissing
	}

	cmd := exec.CommandContext(ctx, "winget", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, false, err
	}
	cmd.Stderr = cmd.Stdout // winget writes progress to stderr on some versions

	if err := cmd.Start(); err != nil {
		return nil, false, err
	}

	var lines []string
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		lines = append(lines, line)
		if progress != nil {
			progress(Progress{Line: line})
		}
	}

	waitErr := cmd.Wait()
	if progress != nil {
		progress(Progress{Done: true})
	}
	if err := sc.Err(); err != nil && err != io.EOF {
		return lines, false, err
	}
	// winget returns non-zero for "already installed" etc.; treat context
	// cancellation as a real error, everything else as a reported result.
	if ctx.Err() != nil {
		return lines, false, ctx.Err()
	}
	return lines, waitErr == nil, waitErr
}

// Install installs a package by exact id, streaming progress.
func (m *Manager) Install(ctx context.Context, packageID string, progress func(Progress)) (*Result, error) {
	lines, ok, err := m.report(ctx, []string{
		"install", "--id", packageID, "--silent",
		"--accept-source-agreements", "--accept-package-agreements", "-e",
	}, progress)
	return &Result{Success: ok, Lines: lines, Error: err}, err
}

// Uninstall removes a package by exact id.
func (m *Manager) Uninstall(ctx context.Context, packageID string, progress func(Progress)) (*Result, error) {
	lines, ok, err := m.report(ctx, []string{
		"uninstall", "--id", packageID, "--silent",
		"--accept-source-agreements", "-e",
	}, progress)
	return &Result{Success: ok, Lines: lines, Error: err}, err
}

// Search queries the winget catalog, returning matching package ids.
func (m *Manager) Search(ctx context.Context, query string) ([]string, error) {
	lines, _, err := m.report(ctx, []string{"search", query, "--accept-source-agreements"}, nil)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		id := fields[0]
		if id != "Name" && id != "---" && strings.Contains(id, ".") {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

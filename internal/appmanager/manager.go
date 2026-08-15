// Package appmanager wraps winget.exe for install/uninstall/search, streaming
// stdout line-by-line so the UI can render progress.
package appmanager

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxReportLines     = 10000
	maxReportBytes     = 16 << 20
	maxReportLineBytes = 1 << 20
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

func collectReportOutput(output io.Reader, progress func(Progress), lineLimit, byteLimit int) ([]string, error) {
	var lines []string
	var outputErr error
	outputBytes := 0
	sc := bufio.NewScanner(output)
	sc.Buffer(make([]byte, 64*1024), maxReportLineBytes)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		lineBytes := len(line) + 1
		if outputErr == nil && (len(lines) >= lineLimit || outputBytes > byteLimit-lineBytes) {
			outputErr = fmt.Errorf("winget output exceeds %d lines or %d bytes", lineLimit, byteLimit)
		}
		if outputErr != nil {
			// Keep draining the pipe so the child can terminate, but do not retain
			// or forward unbounded output from a misbehaving executable.
			continue
		}
		outputBytes += lineBytes
		lines = append(lines, line)
		if progress != nil {
			progress(Progress{Line: line})
		}
	}

	scanErr := sc.Err()
	var drainErr error
	if scanErr != nil {
		_, drainErr = io.Copy(io.Discard, output)
	}
	return lines, errors.Join(outputErr, scanErr, drainErr)
}

// report streams lines to progress, returning the collected lines and whether
// the underlying command exited successfully.
func (m *Manager) report(ctx context.Context, args []string, progress func(Progress)) ([]string, bool, error) {
	winget, err := exec.LookPath("winget")
	if err != nil {
		return nil, false, ErrWingetMissing
	}
	// Execute the exact path that was discovered. Calling CommandContext with
	// the bare name would perform a second PATH lookup and could select a
	// different executable if the process environment or working directory
	// changed between the two operations.
	winget, err = filepath.Abs(winget)
	if err != nil {
		return nil, false, fmt.Errorf("resolve winget.exe path: %w", err)
	}

	cmd := exec.CommandContext(ctx, winget, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, false, err
	}
	cmd.Stderr = cmd.Stdout // winget writes progress to stderr on some versions

	if err := cmd.Start(); err != nil {
		return nil, false, err
	}

	lines, outputErr := collectReportOutput(stdout, progress, maxReportLines, maxReportBytes)
	waitErr := cmd.Wait()
	if progress != nil {
		progress(Progress{Done: true})
	}
	if ctx.Err() != nil {
		return lines, false, errors.Join(ctx.Err(), outputErr, waitErr)
	}
	err = errors.Join(outputErr, waitErr)
	return lines, err == nil, err
}

// Install installs a package by exact id, streaming progress.
func (m *Manager) Install(ctx context.Context, packageID string, progress func(Progress)) (*Result, error) {
	if err := ValidatePackageID(packageID); err != nil {
		return &Result{Error: err}, err
	}
	lines, ok, err := m.report(ctx, []string{
		"install", "--id", packageID, "--silent",
		"--accept-source-agreements", "--accept-package-agreements", "-e",
	}, progress)
	return &Result{Success: ok, Lines: lines, Error: err}, err
}

// Upgrade upgrades a single package by exact id, streaming progress.
func (m *Manager) Upgrade(ctx context.Context, packageID string, progress func(Progress)) (*Result, error) {
	if err := ValidatePackageID(packageID); err != nil {
		return &Result{Error: err}, err
	}
	lines, ok, err := m.report(ctx, []string{
		"upgrade", "--id", packageID, "--silent",
		"--accept-source-agreements", "--accept-package-agreements", "-e",
	}, progress)
	return &Result{Success: ok, Lines: lines, Error: err}, err
}

// UpgradeAll upgrades every outdated package, streaming progress. Used by the
// scheduled maintenance pass.
func (m *Manager) UpgradeAll(ctx context.Context, progress func(Progress)) (*Result, error) {
	lines, ok, err := m.report(ctx, []string{
		"upgrade", "--all", "--silent",
		"--accept-source-agreements", "--accept-package-agreements",
	}, progress)
	return &Result{Success: ok, Lines: lines, Error: err}, err
}

// Uninstall removes a package by exact id.
func (m *Manager) Uninstall(ctx context.Context, packageID string, progress func(Progress)) (*Result, error) {
	if err := ValidatePackageID(packageID); err != nil {
		return &Result{Error: err}, err
	}
	lines, ok, err := m.report(ctx, []string{
		"uninstall", "--id", packageID, "--silent",
		"--accept-source-agreements", "-e",
	}, progress)
	return &Result{Success: ok, Lines: lines, Error: err}, err
}

// Search queries the winget catalog, returning matching package ids.
func (m *Manager) Search(ctx context.Context, query string) ([]string, error) {
	if err := validateSearchQuery(query); err != nil {
		return nil, err
	}
	lines, _, err := m.report(ctx, []string{"search", "--query", query, "--accept-source-agreements"}, nil)
	if err != nil {
		return nil, err
	}
	return parseSearchIDs(lines), nil
}

// parseSearchIDs reads winget's fixed-width search table. Package display names
// can contain spaces, so taking the first whitespace-delimited field returns a
// fragment of the name rather than the package ID. Winget aligns the ID column
// with its "Id" header and the next column with the following header.
func parseSearchIDs(lines []string) []string {
	idStart, idEnd, headerLine := -1, -1, -1
	for lineIndex, line := range lines {
		if start, end, ok := searchIDColumns(line); ok {
			idStart, idEnd, headerLine = start, end, lineIndex
			break
		}
	}
	if headerLine < 0 {
		return nil
	}

	var ids []string
	seen := make(map[string]struct{})
	for _, line := range lines[headerLine+1:] {
		field, ok := displayColumnSlice(line, idStart, idEnd)
		if !ok {
			continue
		}
		id := strings.TrimSpace(field)
		if !isPackageID(id) {
			continue
		}
		key := strings.ToLower(id)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

// searchIDColumns locates the ID field by terminal display columns, not rune
// offsets. WinGet pads its table according to rendered width, where (for
// example) a Han character occupies two columns despite being one rune.
func searchIDColumns(line string) (idStart, idEnd int, ok bool) {
	runes := []rune(line)
	column := 0
	for start := 0; start < len(runes); {
		for start < len(runes) && unicode.IsSpace(runes[start]) {
			column += runeDisplayWidth(runes[start])
			start++
		}
		end := start
		wordStart := column
		for end < len(runes) && !unicode.IsSpace(runes[end]) {
			column += runeDisplayWidth(runes[end])
			end++
		}
		if end > start && strings.EqualFold(string(runes[start:end]), "id") {
			nextColumn := column
			next := end
			for next < len(runes) && unicode.IsSpace(runes[next]) {
				nextColumn += runeDisplayWidth(runes[next])
				next++
			}
			if next > end && next < len(runes) {
				return wordStart, nextColumn, true
			}
			return 0, 0, false
		}
		if end == start {
			break
		}
		start = end
	}
	return 0, 0, false
}

// displayColumnSlice returns the text occupying [start, end) terminal columns.
// A boundary that bisects a wide rune indicates a malformed/misaligned row and
// is rejected instead of accidentally returning part of a neighboring field.
func displayColumnSlice(line string, start, end int) (string, bool) {
	if start < 0 || end <= start {
		return "", false
	}
	var field strings.Builder
	column := 0
	writing := false
	for _, r := range line {
		width := runeDisplayWidth(r)
		next := column + width
		if width == 0 {
			if writing && column <= end {
				field.WriteRune(r)
			}
			continue
		}
		if next <= start {
			column = next
			continue
		}
		if column >= end {
			break
		}
		if column < start || next > end {
			return "", false
		}
		writing = true
		field.WriteRune(r)
		column = next
	}
	return field.String(), writing
}

// runeDisplayWidth implements the stable subset of wcwidth needed for WinGet
// tables. Combining/format characters occupy no cells; East Asian wide and
// full-width characters occupy two. Remaining printable runes occupy one.
func runeDisplayWidth(r rune) int {
	if unicode.IsControl(r) || unicode.Is(unicode.Mn, r) ||
		unicode.Is(unicode.Me, r) || unicode.Is(unicode.Cf, r) ||
		(r >= 0x1F3FB && r <= 0x1F3FF) { // emoji skin-tone modifiers
		return 0
	}
	if r >= 0x1100 && (r <= 0x115F ||
		r == 0x2329 || r == 0x232A ||
		(r >= 0x2E80 && r <= 0xA4CF && r != 0x303F) ||
		(r >= 0xAC00 && r <= 0xD7A3) ||
		(r >= 0xF900 && r <= 0xFAFF) ||
		(r >= 0xFE10 && r <= 0xFE19) ||
		(r >= 0xFE30 && r <= 0xFE6F) ||
		(r >= 0xFF00 && r <= 0xFF60) ||
		(r >= 0xFFE0 && r <= 0xFFE6) ||
		(r >= 0x1B000 && r <= 0x1B2FF) ||
		(r >= 0x1F300 && r <= 0x1F64F) ||
		(r >= 0x1F900 && r <= 0x1FAFF) ||
		(r >= 0x20000 && r <= 0x3FFFD)) {
		return 2
	}
	return 1
}

// ValidatePackageID rejects values that are not exact WinGet package identities.
func ValidatePackageID(id string) error {
	if strings.TrimSpace(id) != id || !isPackageID(id) {
		return fmt.Errorf("invalid WinGet package id %q", id)
	}
	return nil
}

func validateSearchQuery(query string) error {
	if query == "" || len(query) > 512 || strings.TrimSpace(query) != query || strings.HasPrefix(query, "-") {
		return errors.New("invalid WinGet search query")
	}
	for _, r := range query {
		if unicode.IsControl(r) {
			return errors.New("invalid WinGet search query")
		}
	}
	return nil
}

func isPackageID(id string) bool {
	if id == "" || utf8.RuneCountInString(id) > 128 || strings.HasPrefix(id, "-") ||
		strings.ContainsRune(id, '…') || strings.HasSuffix(id, "...") {
		// WinGet marks fields that do not fit in the table with an ellipsis.
		// Such a value is not an exact package identity and must never be fed
		// back to install/uninstall with --exact.
		return false
	}

	if !strings.Contains(id, ".") {
		// Microsoft Store product IDs are uppercase alphanumeric tokens rather
		// than publisher-qualified IDs (for example 9NBLGGH4NNS1).
		if len(id) < 8 {
			return false
		}
		for _, r := range id {
			if !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') {
				return false
			}
		}
		return true
	}

	// WinGet's manifest schema permits two to eight period-separated segments,
	// each containing one to 32 non-whitespace characters other than the
	// Windows filename delimiters below.
	segments := strings.Split(id, ".")
	if len(segments) < 2 || len(segments) > 8 {
		return false
	}
	for _, segment := range segments {
		if count := utf8.RuneCountInString(segment); count == 0 || count > 32 {
			return false
		}
		for _, r := range segment {
			if unicode.IsSpace(r) || unicode.IsControl(r) || strings.ContainsRune(`\/:*?"<>|`, r) {
				return false
			}
		}
	}
	return true
}

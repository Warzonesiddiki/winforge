// Package isobuilder builds a bootable Windows installation ISO from an
// extracted (or mounted) Windows installation source using only native tools:
// dism.exe for edition export and oscdimg.exe (Windows ADK Deployment Tools)
// for the final ISO. No PowerShell, no third-party modules.
//
// The orchestration and parsing logic in this file is platform-agnostic and
// unit-tested; the dism/oscdimg execution lives in isobuilder_windows.go.
package isobuilder

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// ErrUnsupported is returned on non-Windows platforms.
var ErrUnsupported = errors.New("ISO building is only supported on Windows")

// ErrOscdimgMissing is returned when oscdimg.exe (Windows ADK Deployment
// Tools) is not installed or not on PATH.
var ErrOscdimgMissing = errors.New("oscdimg.exe not found; install the Windows ADK Deployment Tools")

// LogFunc receives streaming command output (may be nil).
type LogFunc func(line string)

// Edition is one Windows edition inside an install.wim/install.esd.
type Edition struct {
	Index int    `json:"index"`
	Name  string `json:"name"`
}

// Options configures an ISO build.
type Options struct {
	// SourceDir is the root of an extracted Windows installation source. It
	// must contain sources\install.wim or sources\install.esd.
	SourceDir string `json:"source"`
	// OutputISO is the destination .iso path. ".iso" is appended if missing.
	OutputISO string `json:"output"`
	// Label is the ISO volume label (sanitized: uppercase, spaces removed).
	Label string `json:"label"`
	// Editions lists the edition names to keep. Empty keeps all editions.
	Editions []string `json:"editions,omitempty"`
	// Log receives progress lines (may be nil).
	Log LogFunc `json:"-"`
}

// Result reports the outcome of a build.
type Result struct {
	ISO       string    `json:"iso"`
	SourceDir string    `json:"sourceDir"`
	Exported  []Edition `json:"exportedEditions,omitempty"`
}

// ListEditions enumerates the Windows editions inside sourceDir's image file.
func ListEditions(sourceDir string) ([]Edition, error) { return listEditions(sourceDir) }

// Build creates a (optionally edition-slimmed) bootable ISO.
func Build(opts Options) (Result, error) { return build(opts) }

// imageFile locates install.wim (preferred) or install.esd under sourceDir.
func imageFile(sourceDir string) (string, error) {
	for _, name := range []string{"install.wim", "install.esd"} {
		p := filepath.Join(sourceDir, "sources", name)
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("no sources\\install.wim or sources\\install.esd under %q", sourceDir)
}

// ValidateOptions checks o and normalizes OutputISO (appends ".iso") and Label
// (sanitized, defaulted) in place.
func ValidateOptions(o *Options) error {
	if o == nil {
		return errors.New("nil options")
	}
	if strings.TrimSpace(o.SourceDir) == "" {
		return errors.New("source directory is required")
	}
	fi, err := os.Stat(o.SourceDir)
	if err != nil {
		return fmt.Errorf("source directory %q: %w", o.SourceDir, err)
	}
	if !fi.IsDir() {
		return fmt.Errorf("source %q is not a directory", o.SourceDir)
	}
	if _, err := imageFile(o.SourceDir); err != nil {
		return err
	}
	if strings.TrimSpace(o.OutputISO) == "" {
		return errors.New("output iso path is required")
	}
	if !strings.HasSuffix(strings.ToLower(o.OutputISO), ".iso") {
		o.OutputISO += ".iso"
	}
	o.Label = SanitizeLabel(o.Label)
	for i := range o.Editions {
		o.Editions[i] = strings.TrimSpace(o.Editions[i])
	}
	return nil
}

// SanitizeLabel normalizes a volume label for oscdimg: uppercase, alphanumeric
// only (other runes become '_'), truncated to 32 chars. Empty input becomes
// "WINFORGE".
func SanitizeLabel(s string) string {
	if strings.TrimSpace(s) == "" {
		s = "WINFORGE"
	}
	s = strings.ToUpper(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := b.String()
	if out == "" {
		out = "WINFORGE"
	}
	if len(out) > 32 {
		out = out[:32]
	}
	return out
}

// reIndexLine matches any line that ends in ": <digits>" (e.g. "Index : 1" in
// English, "索引: 1" in Chinese). This makes DISM output parsing
// language-agnostic: the edition name is read from the next ":"-bearing line.
var reIndexLine = regexp.MustCompile(`^\s*.*:\s*(\d+)\s*$`)

// parseWimInfo parses "dism /Get-WimInfo" output into a list of editions.
func parseWimInfo(out string) []Edition {
	var editions []Edition
	lines := strings.Split(out, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		m := reIndexLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		idx, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		name := ""
		for j := i + 1; j < len(lines) && j <= i+3; j++ {
			nl := strings.TrimSpace(lines[j])
			if k := strings.Index(nl, ":"); k >= 0 {
				name = strings.TrimSpace(nl[k+1:])
				break
			}
		}
		editions = append(editions, Edition{Index: idx, Name: name})
	}
	return editions
}

// selectIndexes resolves wanted edition names to image indexes. An empty wanted
// list selects every edition.
func selectIndexes(all []Edition, wanted []string) ([]int, error) {
	if len(all) == 0 {
		return nil, errors.New("no editions found in the image")
	}
	if len(wanted) == 0 {
		out := make([]int, len(all))
		for i, e := range all {
			out[i] = e.Index
		}
		return out, nil
	}

	byName := make(map[string][]int, len(all))
	for _, e := range all {
		n := strings.ToLower(strings.TrimSpace(e.Name))
		byName[n] = append(byName[n], e.Index)
	}

	var out []int
	seen := make(map[int]bool, len(all))
	for _, w := range wanted {
		key := strings.ToLower(strings.TrimSpace(w))
		idx, ok := byName[key]
		if !ok {
			return nil, fmt.Errorf("edition %q not found in image (available: %s)", w, editionNames(all))
		}
		for _, i := range idx {
			if !seen[i] {
				seen[i] = true
				out = append(out, i)
			}
		}
	}
	return out, nil
}

func editionNames(all []Edition) string {
	names := make([]string, len(all))
	for i, e := range all {
		names[i] = e.Name
	}
	return strings.Join(names, ", ")
}

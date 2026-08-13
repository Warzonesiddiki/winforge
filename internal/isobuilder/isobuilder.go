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
	"io"
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
		if fi, err := os.Lstat(p); err == nil && fi.Mode().IsRegular() {
			return p, nil
		}
	}
	return "", fmt.Errorf("no sources\\install.wim or sources\\install.esd under %q", sourceDir)
}

// copyTree copies regular files from src into dst, skipping the original image
// file because the caller replaces it with the slimmed WIM. Symlinks, reparse
// points, devices, and other non-regular entries are rejected rather than
// followed or silently transformed.
func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		imageRel := strings.ToLower(rel)
		switch imageRel {
		case filepath.Join("sources", "install.wim"), filepath.Join("sources", "install.esd"):
			return nil
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("source contains unsupported non-regular file %q", path)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			return errors.Join(err, in.Close())
		}
		_, copyErr := io.Copy(out, in)
		inCloseErr := in.Close()
		outCloseErr := out.Close()
		return errors.Join(copyErr, inCloseErr, outCloseErr)
	})
}

func validateSourceTree(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("source contains unsupported symbolic link %q", path)
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("source contains unsupported non-regular file %q", path)
		}
		return nil
	})
}

// resolvePath resolves symlinks/reparse points in the existing portion of path
// and then reattaches any not-yet-created suffix. This makes containment checks
// meaningful even when the output file or its immediate parent does not exist.
func resolvePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	candidate := abs
	var suffix []string
	for {
		resolved, err := filepath.EvalSymlinks(candidate)
		if err == nil {
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return filepath.Clean(resolved), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(candidate)
		if parent == candidate {
			return "", err
		}
		suffix = append(suffix, filepath.Base(candidate))
		candidate = parent
	}
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
	sourceResolved, err := resolvePath(o.SourceDir)
	if err != nil {
		return fmt.Errorf("resolve source directory: %w", err)
	}
	if err := validateSourceTree(sourceResolved); err != nil {
		return fmt.Errorf("validate source tree: %w", err)
	}
	outputResolved, err := resolvePath(o.OutputISO)
	if err != nil {
		return fmt.Errorf("resolve output ISO path: %w", err)
	}
	relOutput, err := filepath.Rel(sourceResolved, outputResolved)
	if err != nil {
		return fmt.Errorf("compare source and output paths: %w", err)
	}
	if relOutput == "." || (relOutput != ".." && !strings.HasPrefix(relOutput, ".."+string(filepath.Separator)) && !filepath.IsAbs(relOutput)) {
		return errors.New("output ISO must be outside the source directory")
	}
	// Pass canonical absolute paths to DISM and oscdimg. Besides making the
	// validated paths the paths actually used, this prevents relative names
	// beginning with '-' or '/' from being interpreted as command options.
	o.SourceDir = sourceResolved
	o.OutputISO = outputResolved
	o.Label = SanitizeLabel(o.Label)
	for i := range o.Editions {
		o.Editions[i] = strings.TrimSpace(o.Editions[i])
		if o.Editions[i] == "" {
			return fmt.Errorf("edition %d must not be empty", i+1)
		}
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

// reIndexLine matches the English field emitted by DISM /English. Matching the
// field name is important: image metadata also contains numeric colon-delimited
// fields (for example "ServicePack Build : 1") that are not edition indexes.
var (
	reIndexLine = regexp.MustCompile(`(?i)^\s*Index\s*:\s*(\d+)\s*$`)
	reNameLine  = regexp.MustCompile(`(?i)^\s*Name\s*:\s*(.*?)\s*$`)
)

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
		if err != nil || idx <= 0 {
			continue
		}
		name := ""
		for j := i + 1; j < len(lines); j++ {
			if reIndexLine.MatchString(lines[j]) {
				break
			}
			if nameMatch := reNameLine.FindStringSubmatch(lines[j]); nameMatch != nil {
				name = strings.TrimSpace(nameMatch[1])
				break
			}
		}
		if name != "" {
			editions = append(editions, Edition{Index: idx, Name: name})
		}
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

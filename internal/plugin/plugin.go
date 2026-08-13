// Package plugin discovers and loads WinForge plugins: directories under a
// plugins root that carry a manifest.json and an optional tweaks.json.
//
// Plugins are the extension point for shipping extra tweaks (and, later, app
// catalogs) without recompiling the binary. They are scanned at startup from
// %LOCALAPPDATA%\WinForge\plugins and merged into the configuration, with the
// built-in defaults taking precedence on id collisions.
package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"winforge/internal/config"
)

const (
	maxPluginFileBytes        = 8 << 20
	maxPluginRootEntries      = 10000
	maxPluginDirectories      = 1024
	maxDiscoveredPlugins      = 256
	maxDiscoveredPluginTweaks = 10000
)

func readPluginFile(path string) ([]byte, error) {
	expected, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if expected.Mode()&os.ModeSymlink != 0 || !expected.Mode().IsRegular() {
		return nil, fmt.Errorf("plugin file %q must be a regular file", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	opened, err := f.Stat()
	if err != nil {
		return nil, errors.Join(err, f.Close())
	}
	if !opened.Mode().IsRegular() || !os.SameFile(expected, opened) {
		return nil, errors.Join(fmt.Errorf("plugin file %q changed while it was opened", path), f.Close())
	}
	b, readErr := io.ReadAll(io.LimitReader(f, maxPluginFileBytes+1))
	closeErr := f.Close()
	if readErr != nil || closeErr != nil {
		return nil, errors.Join(readErr, closeErr)
	}
	if len(b) > maxPluginFileBytes {
		return nil, fmt.Errorf("plugin file %q exceeds %d-byte limit", path, maxPluginFileBytes)
	}
	return b, nil
}

func readPluginDirectories(root string) ([]os.DirEntry, error) {
	dir, err := os.Open(root)
	if err != nil {
		return nil, err
	}
	var directories []os.DirEntry
	seen := 0
	var scanErr error
	for {
		batch, readErr := dir.ReadDir(256)
		for _, entry := range batch {
			seen++
			if seen > maxPluginRootEntries {
				scanErr = fmt.Errorf("plugin root exceeds %d-entry scan limit", maxPluginRootEntries)
				break
			}
			if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			if len(directories) >= maxPluginDirectories {
				scanErr = fmt.Errorf("plugin root exceeds %d-directory limit", maxPluginDirectories)
				break
			}
			directories = append(directories, entry)
		}
		if scanErr != nil || errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			scanErr = readErr
			break
		}
	}
	closeErr := dir.Close()
	sort.Slice(directories, func(i, j int) bool { return directories[i].Name() < directories[j].Name() })
	return directories, errors.Join(scanErr, closeErr)
}

// Plugin is one installed plugin. ID is the plugin directory's name.
type Plugin struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Version     string         `json:"version"`
	Description string         `json:"description,omitempty"`
	Author      string         `json:"author,omitempty"`
	Dir         string         `json:"dir"`
	Tweaks      []config.Tweak `json:"-"`
}

// Manifest is the shape of a plugin's manifest.json. All fields are optional
// except, by convention, name.
type Manifest struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Author      string `json:"author"`
}

// Discover scans root for plugin directories and loads each valid plugin.
//
// A subdirectory is treated as a plugin when it contains a manifest.json.
// Plugins whose manifest is unreadable/invalid, or whose tweaks.json fails to
// parse or validate, are skipped (best-effort). The returned slice is ordered
// by directory name. A missing root yields an empty result, not an error.
func Discover(root string) ([]Plugin, error) {
	entries, err := readPluginDirectories(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read plugin directory: %w", err)
	}

	var plugins []Plugin
	totalTweaks := 0
	for _, e := range entries {
		name := e.Name()
		dir := filepath.Join(root, name)
		if _, err := os.Stat(filepath.Join(dir, "manifest.json")); err != nil {
			continue // not a plugin directory
		}
		p, err := load(dir, name)
		if err != nil {
			continue // best-effort: skip broken plugins
		}
		if len(plugins) >= maxDiscoveredPlugins {
			return nil, fmt.Errorf("plugin root exceeds %d-valid-plugin limit", maxDiscoveredPlugins)
		}
		if len(p.Tweaks) > maxDiscoveredPluginTweaks-totalTweaks {
			return nil, fmt.Errorf("plugins exceed %d-tweak aggregate limit", maxDiscoveredPluginTweaks)
		}
		totalTweaks += len(p.Tweaks)
		plugins = append(plugins, p)
	}
	return plugins, nil
}

// load reads and validates a single plugin directory.
func load(dir, id string) (Plugin, error) {
	b, err := readPluginFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return Plugin{}, err
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return Plugin{}, fmt.Errorf("parse manifest.json: %w", err)
	}

	p := Plugin{
		ID:          id,
		Name:        m.Name,
		Version:     m.Version,
		Description: m.Description,
		Author:      m.Author,
		Dir:         dir,
	}
	if p.Name == "" {
		p.Name = id
	}

	// tweaks.json is optional.
	tb, err := readPluginFile(filepath.Join(dir, "tweaks.json"))
	switch {
	case err == nil:
		var tc config.TweakConfig
		if err := config.DecodeJSON(tb, &tc); err != nil {
			return Plugin{}, fmt.Errorf("parse tweaks.json: %w", err)
		}
		if err := tc.Validate(); err != nil {
			return Plugin{}, fmt.Errorf("validate tweaks.json: %w", err)
		}
		p.Tweaks = tc.Tweaks
	case os.IsNotExist(err):
		// no tweaks; plugin still valid
	default:
		return Plugin{}, err
	}
	return p, nil
}

// MergeTweaks appends extra tweaks to base, skipping any id already present
// in base and any duplicate id within extra. The built-in/base tweaks win on
// collisions. A new slice is returned; base is not mutated.
func MergeTweaks(base, extra []config.Tweak) []config.Tweak {
	seen := make(map[string]struct{}, len(base)+len(extra))
	out := make([]config.Tweak, 0, len(base)+len(extra))
	for _, t := range base {
		seen[t.ID] = struct{}{}
		out = append(out, t)
	}
	for _, t := range extra {
		if t.ID == "" {
			continue
		}
		if _, dup := seen[t.ID]; dup {
			continue
		}
		seen[t.ID] = struct{}{}
		out = append(out, t)
	}
	return out
}

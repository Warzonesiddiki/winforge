package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"winforge"
)

// Loader resolves configuration from the user's override directory first and
// falls back to the embedded defaults.
type Loader struct {
	// OverrideDir is an optional directory (e.g. %LOCALAPPDATA%\WinForge\config)
	// whose files shadow the embedded defaults. Empty means "use embedded only".
	OverrideDir string

	// embedded supplies default config files. Injectable for tests.
	embedded fs.FS
}

// NewLoader builds a Loader that reads embedded defaults and, if overrideDir is
// non-empty, user overrides from that directory.
func NewLoader(overrideDir string) *Loader {
	return &Loader{OverrideDir: overrideDir, embedded: winforge.Assets}
}

// readFile reads from the override dir if the file exists, else the embedded FS.
func (l *Loader) readFile(rel string) ([]byte, error) {
	if l.OverrideDir != "" {
		p := filepath.Join(l.OverrideDir, filepath.FromSlash(rel))
		if b, err := os.ReadFile(p); err == nil {
			return b, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	b, err := fs.ReadFile(l.embedded, filepath.ToSlash(rel))
	if err != nil {
		return nil, fmt.Errorf("config %q: %w", rel, err)
	}
	return b, nil
}

// LoadTweaks loads and validates tweaks.json.
func (l *Loader) LoadTweaks() (*TweakConfig, error) {
	b, err := l.readFile("config/tweaks.json")
	if err != nil {
		return nil, err
	}
	var c TweakConfig
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("parse tweaks.json: %w", err)
	}
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("validate tweaks.json: %w", err)
	}
	return &c, nil
}

// LoadApps loads applications.json.
func (l *Loader) LoadApps() (*AppsConfig, error) {
	b, err := l.readFile("config/applications.json")
	if err != nil {
		return nil, err
	}
	var c AppsConfig
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("parse applications.json: %w", err)
	}
	return &c, nil
}

// LoadDns loads dns.json.
func (l *Loader) LoadDns() (*DnsConfig, error) {
	b, err := l.readFile("config/dns.json")
	if err != nil {
		return nil, err
	}
	var c DnsConfig
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("parse dns.json: %w", err)
	}
	return &c, nil
}

// LoadProtectedServices loads protectedServices.json, returning an empty list if
// the file is absent.
func (l *Loader) LoadProtectedServices() ([]string, error) {
	b, err := l.readFile("config/protectedServices.json")
	if err != nil {
		if strings.Contains(err.Error(), "protectedServices.json") {
			return nil, nil // optional file
		}
		return nil, err
	}
	var c ProtectedServices
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("parse protectedServices.json: %w", err)
	}
	return c.Services, nil
}

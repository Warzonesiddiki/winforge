package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"winforge"
)

// Loader resolves configuration from the user's override directory first and
// falls back to the embedded defaults.
const maxConfigFileBytes = 8 << 20

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

func readBounded(r io.Reader, name string) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, maxConfigFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxConfigFileBytes {
		return nil, fmt.Errorf("configuration %q exceeds %d-byte limit", name, maxConfigFileBytes)
	}
	return b, nil
}

func readBoundedOSFile(path string) ([]byte, error) {
	expected, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if expected.Mode()&os.ModeSymlink != 0 || !expected.Mode().IsRegular() {
		return nil, fmt.Errorf("configuration file %q must be a regular file", path)
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
		return nil, errors.Join(fmt.Errorf("configuration file %q changed while it was opened", path), f.Close())
	}
	b, readErr := readBounded(f, path)
	return b, errors.Join(readErr, f.Close())
}

func readBoundedFSFile(fsys fs.FS, name string) ([]byte, error) {
	f, err := fsys.Open(name)
	if err != nil {
		return nil, err
	}
	b, readErr := readBounded(f, name)
	return b, errors.Join(readErr, f.Close())
}

// readFile reads from the override dir if the file exists, else the embedded
// FS. Embedded files live under config/, while OverrideDir already denotes the
// user's config directory; do not accidentally look under config/config/.
func (l *Loader) readFile(rel string) ([]byte, error) {
	if l.OverrideDir != "" {
		overrideRel := filepath.ToSlash(rel)
		overrideRel = strings.TrimPrefix(overrideRel, "config/")
		p := filepath.Join(l.OverrideDir, filepath.FromSlash(overrideRel))
		if b, err := readBoundedOSFile(p); err == nil {
			return b, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	b, err := readBoundedFSFile(l.embedded, filepath.ToSlash(rel))
	if err != nil {
		return nil, fmt.Errorf("config %q: %w", rel, err)
	}
	return b, nil
}

// DecodeJSON decodes one configuration value and rejects unknown fields or
// trailing values. Strict decoding turns misspelled safety-critical fields into
// startup errors instead of silently applying zero values.
func DecodeJSON(b []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("configuration contains multiple JSON values")
		}
		return fmt.Errorf("decode trailing configuration data: %w", err)
	}
	return nil
}

// LoadTweaks loads and validates tweaks.json.
func (l *Loader) LoadTweaks() (*TweakConfig, error) {
	b, err := l.readFile("config/tweaks.json")
	if err != nil {
		return nil, err
	}
	var c TweakConfig
	if err := DecodeJSON(b, &c); err != nil {
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
	if err := DecodeJSON(b, &c); err != nil {
		return nil, fmt.Errorf("parse applications.json: %w", err)
	}
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("validate applications.json: %w", err)
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
	if err := DecodeJSON(b, &c); err != nil {
		return nil, fmt.Errorf("parse dns.json: %w", err)
	}
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("validate dns.json: %w", err)
	}
	return &c, nil
}

// LoadProtectedServices loads protectedServices.json, returning an empty list if
// the file is absent.
func (l *Loader) LoadProtectedServices() ([]string, error) {
	b, err := l.readFile("config/protectedServices.json")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, os.ErrNotExist) {
			return nil, nil // optional file
		}
		return nil, err
	}
	var c ProtectedServices
	if err := DecodeJSON(b, &c); err != nil {
		return nil, fmt.Errorf("parse protectedServices.json: %w", err)
	}
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("validate protectedServices.json: %w", err)
	}
	return c.Services, nil
}

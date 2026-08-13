// Package config defines WinForge's on-disk configuration models and loading.
//
// All configuration is JSON. Defaults are embedded in the binary (see the root
// package's embed.FS); users may override them by dropping equivalent files
// into %LOCALAPPDATA%\WinForge\config (or ./config next to the exe).
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Risk classifies how dangerous a tweak is. It feeds the dashboard health score.
type Risk string

const (
	RiskLow    Risk = "low"
	RiskMedium Risk = "medium"
	RiskHigh   Risk = "high"
)

// Tweak is a single reversible (or one-way) system modification, expressed as
// an ordered list of operations plus an optional explicit revert list.
type Tweak struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Category    string      `json:"category"`
	Description string      `json:"description"`
	Risk        Risk        `json:"risk"`
	Reversible  bool        `json:"reversible"`
	Operations  []Operation `json:"operations"`
	Revert      []Operation `json:"revert,omitempty"`
}

// Operation is one atomic action within a tweak. Value is kept raw so a single
// field can hold a DWORD (number), a string, or a command line depending on Type.
type Operation struct {
	Type  string          `json:"type"`
	Hive  string          `json:"hive,omitempty"`
	Path  string          `json:"path,omitempty"`
	Name  string          `json:"name,omitempty"`
	Value json.RawMessage `json:"value,omitempty"`
	Args  []string        `json:"args,omitempty"`
}

// Operation type constants understood by the engine.
const (
	OpRegistrySetDword   = "registry_set_dword"
	OpRegistrySetString  = "registry_set_string"
	OpRegistryDelete     = "registry_delete"
	OpServiceStartMode   = "service_start_mode"
	OpServiceStart       = "service_start"
	OpServiceStop        = "service_stop"
	OpTaskDisable        = "task_disable"
	OpTaskEnable         = "task_enable"
	OpTaskDelete         = "task_delete"
	OpAppxRemove         = "appx_remove"
	OpCommand            = "command"
)

// Value helpers decode the raw JSON value into the type an operation expects.

// DwordValue returns the operation's value as a uint32.
func (o Operation) DwordValue() (uint32, error) {
	var n int64
	if err := json.Unmarshal(o.Value, &n); err != nil {
		return 0, fmt.Errorf("operation %q expects an integer value: %w", o.Type, err)
	}
	if n < 0 {
		return 0, fmt.Errorf("operation %q expects a non-negative integer", o.Type)
	}
	return uint32(n), nil
}

// StringValue returns the operation's value as a string.
func (o Operation) StringValue() (string, error) {
	var s string
	if err := json.Unmarshal(o.Value, &s); err != nil {
		return "", fmt.Errorf("operation %q expects a string value: %w", o.Type, err)
	}
	return s, nil
}

// App is one installable application surfaced in the package manager.
type App struct {
	ID          string   `json:"id"`          // winget package id
	Name        string   `json:"name"`
	Category    string   `json:"category"`
	Description string   `json:"description"`
	Tags        []string `json:"tags,omitempty"`
}

// DnsEntry describes a DNS server preset for a named adapter profile.
type DnsEntry struct {
	Profile  string   `json:"profile"`
	Primary  string   `json:"primary"`
	Secondary string  `json:"secondary,omitempty"`
}

// TweakConfig is the root shape of tweaks.json.
type TweakConfig struct {
	Tweaks []Tweak `json:"tweaks"`
}

// AppsConfig is the root shape of applications.json.
type AppsConfig struct {
	Applications []App `json:"applications"`
}

// DnsConfig is the root shape of dns.json.
type DnsConfig struct {
	Presets []DnsEntry `json:"presets"`
}

// ProtectedServices is the root shape of protectedServices.json.
type ProtectedServices struct {
	Services []string `json:"services"`
}

// Validation errors.
var (
	ErrDuplicateID = errors.New("duplicate tweak id")
)

// Validate returns an error if the config is internally inconsistent.
func (c *TweakConfig) Validate() error {
	seen := make(map[string]struct{}, len(c.Tweaks))
	for _, t := range c.Tweaks {
		if t.ID == "" {
			return errors.New("tweak with empty id")
		}
		if _, dup := seen[t.ID]; dup {
			return fmt.Errorf("%w: %s", ErrDuplicateID, t.ID)
		}
		seen[t.ID] = struct{}{}
		if len(t.Operations) == 0 {
			return fmt.Errorf("tweak %q has no operations", t.ID)
		}
		if err := t.ValidateRisk(); err != nil {
			return err
		}
	}
	return nil
}

// ValidateRisk normalizes/validates the risk level.
func (t *Tweak) ValidateRisk() error {
	switch strings.ToLower(string(t.Risk)) {
	case "low":
		t.Risk = RiskLow
	case "medium":
		t.Risk = RiskMedium
	case "high":
		t.Risk = RiskHigh
	case "":
		t.Risk = RiskLow
	default:
		return fmt.Errorf("tweak %q has invalid risk %q", t.ID, t.Risk)
	}
	return nil
}

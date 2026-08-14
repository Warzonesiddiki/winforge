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
	"math"
	"strings"
	"unicode"

	"winforge/internal/appmanager"
	"winforge/internal/maintenance"
	"winforge/internal/power"
	"winforge/internal/service"
)

// Risk classifies how dangerous a tweak is. It feeds the dashboard health score.
type Risk string

const (
	RiskLow    Risk = "low"
	RiskMedium Risk = "medium"
	RiskHigh   Risk = "high"
)

// Tweak is a single reversible (or one-way) system modification, expressed as
// an ordered list of operations plus an explicit revert list when reversible.
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
	OpRegistrySetDword  = "registry_set_dword"
	OpRegistrySetString = "registry_set_string"
	OpRegistrySetQword  = "registry_set_qword"
	OpRegistryDelete    = "registry_delete"
	OpServiceStartMode  = "service_start_mode"
	OpServiceStart      = "service_start"
	OpServiceStop       = "service_stop"
	OpTaskDisable       = "task_disable"
	OpTaskEnable        = "task_enable"
	OpTaskDelete        = "task_delete"
	OpAppxRemove        = "appx_remove"
	OpCommand           = "command"
	OpPowerScheme       = "power_scheme"
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
	if n > math.MaxUint32 {
		return 0, fmt.Errorf("operation %q value %d exceeds the DWORD range", o.Type, n)
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

// QwordValue returns the operation's value as a uint64.
func (o Operation) QwordValue() (uint64, error) {
	var n uint64
	if err := json.Unmarshal(o.Value, &n); err != nil {
		return 0, fmt.Errorf("operation %q expects a non-negative integer value: %w", o.Type, err)
	}
	return n, nil
}

// App is one installable application surfaced in the package manager.
type App struct {
	ID          string   `json:"id"` // winget package id
	Name        string   `json:"name"`
	Category    string   `json:"category"`
	Description string   `json:"description"`
	Tags        []string `json:"tags,omitempty"`
}

// DnsEntry describes a DNS server preset for a named adapter profile.
type DnsEntry struct {
	Profile   string `json:"profile"`
	Primary   string `json:"primary"`
	Secondary string `json:"secondary,omitempty"`
}

// TweakConfig is the root shape of tweaks.json.
type TweakConfig struct {
	Tweaks []Tweak `json:"tweaks"`
}

// AppsConfig is the root shape of applications.json.
type AppsConfig struct {
	Applications []App `json:"applications"`
}

// Validate rejects malformed or duplicate catalog entries before they reach a
// UI or package-manager command.
func (c *AppsConfig) Validate() error {
	if err := checkCount("applications", len(c.Applications), maxApplications); err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(c.Applications))
	for i := range c.Applications {
		entry := &c.Applications[i]
		if err := appmanager.ValidatePackageID(entry.ID); err != nil {
			return fmt.Errorf("application %d: %w", i+1, err)
		}
		key := strings.ToLower(entry.ID)
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf("duplicate application id %q", entry.ID)
		}
		seen[key] = struct{}{}
		if strings.TrimSpace(entry.Name) == "" {
			return fmt.Errorf("application %q has no name", entry.ID)
		}
		if err := entry.checkLimits(); err != nil {
			return fmt.Errorf("application %q: %w", entry.ID, err)
		}
	}
	return nil
}

// checkLimits bounds one application's display fields. The ID is already bounded
// by appmanager.ValidatePackageID, which caps it at 128 runes.
func (a *App) checkLimits() error {
	if err := checkLen("name", a.Name, maxNameLen); err != nil {
		return err
	}
	if err := checkLen("category", a.Category, maxCategoryLen); err != nil {
		return err
	}
	if err := checkLen("description", a.Description, maxDescriptionLen); err != nil {
		return err
	}
	if err := checkCount("tags", len(a.Tags), maxTagsPerApp); err != nil {
		return err
	}
	for i, tag := range a.Tags {
		if err := checkLen(fmt.Sprintf("tag %d", i+1), tag, maxTagLen); err != nil {
			return err
		}
	}
	return nil
}

// DnsConfig is the root shape of dns.json.
type DnsConfig struct {
	Presets []DnsEntry `json:"presets"`
}

// Validate rejects unusable or ambiguously duplicated DNS presets before they
// are shown in the dashboard.
func (c *DnsConfig) Validate() error {
	if err := checkCount("DNS presets", len(c.Presets), maxDnsPresets); err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(c.Presets))
	for i := range c.Presets {
		preset := &c.Presets[i]
		profile := strings.TrimSpace(preset.Profile)
		if profile == "" {
			return fmt.Errorf("DNS preset %d has no profile", i+1)
		}
		if err := checkLen(fmt.Sprintf("DNS preset %d profile", i+1), preset.Profile, maxDnsProfileLen); err != nil {
			return err
		}
		if err := checkLen(fmt.Sprintf("DNS profile %q primary", profile), preset.Primary, maxDnsServerLen); err != nil {
			return err
		}
		if err := checkLen(fmt.Sprintf("DNS profile %q secondary", profile), preset.Secondary, maxDnsServerLen); err != nil {
			return err
		}
		if profile != preset.Profile {
			return fmt.Errorf("DNS profile %q has leading or trailing whitespace", preset.Profile)
		}
		key := strings.ToLower(profile)
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf("duplicate DNS profile %q", preset.Profile)
		}
		seen[key] = struct{}{}
		if err := maintenance.ValidateDnsServers(preset.Primary, preset.Secondary); err != nil {
			return fmt.Errorf("DNS profile %q: %w", preset.Profile, err)
		}
	}
	return nil
}

// ProtectedServices is the root shape of protectedServices.json.
type ProtectedServices struct {
	Services []string `json:"services"`
}

// Validate rejects empty, padded, or duplicate service names so a malformed
// protection entry cannot silently fail to match the executor's service name.
func (c *ProtectedServices) Validate() error {
	if err := checkCount("protected services", len(c.Services), maxProtectedServices); err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(c.Services))
	for i, name := range c.Services {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			return fmt.Errorf("protected service %d has no name", i+1)
		}
		if err := checkLen(fmt.Sprintf("protected service %d", i+1), name, maxServiceNameLen); err != nil {
			return err
		}
		if trimmed != name {
			return fmt.Errorf("protected service %q has leading or trailing whitespace", name)
		}
		key := strings.ToLower(name)
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf("duplicate protected service %q", name)
		}
		seen[key] = struct{}{}
	}
	return nil
}

// Validation errors.
var (
	ErrDuplicateID = errors.New("duplicate tweak id")
)

// Validate returns an error if the config is internally inconsistent.
func (c *TweakConfig) Validate() error {
	if err := checkCount("tweaks", len(c.Tweaks), maxTweaks); err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(c.Tweaks))
	for i := range c.Tweaks {
		t := &c.Tweaks[i]
		t.ID = strings.TrimSpace(t.ID)
		if t.ID == "" {
			return errors.New("tweak with empty id")
		}
		// Bound the ID before it is used in any later error message or map key.
		if err := checkLen(fmt.Sprintf("tweak %d id", i+1), t.ID, maxIDLen); err != nil {
			return err
		}
		if _, dup := seen[t.ID]; dup {
			return fmt.Errorf("%w: %s", ErrDuplicateID, t.ID)
		}
		seen[t.ID] = struct{}{}
		if strings.TrimSpace(t.Name) == "" {
			t.Name = displayName(t.ID)
		}
		if err := t.checkLimits(); err != nil {
			return fmt.Errorf("tweak %q: %w", t.ID, err)
		}
		if len(t.Operations) == 0 {
			return fmt.Errorf("tweak %q has no operations", t.ID)
		}
		if t.Reversible && len(t.Revert) == 0 {
			return fmt.Errorf("reversible tweak %q has no explicit revert operations", t.ID)
		}
		if err := t.ValidateRisk(); err != nil {
			return err
		}
		for j := range t.Operations {
			if err := validateOperation(t.Operations[j]); err != nil {
				return fmt.Errorf("tweak %q operation %d: %w", t.ID, j+1, err)
			}
		}
		for j := range t.Revert {
			if err := validateOperation(t.Revert[j]); err != nil {
				return fmt.Errorf("tweak %q revert operation %d: %w", t.ID, j+1, err)
			}
		}
	}
	return nil
}

// checkLimits bounds a tweak's display fields and its operation lists. The
// name is a display string and may be truncated by a UI; the ID is an identity
// and is bounded by the caller before it is used as a map key.
func (t *Tweak) checkLimits() error {
	if err := checkLen("name", t.Name, maxNameLen); err != nil {
		return err
	}
	if err := checkLen("category", t.Category, maxCategoryLen); err != nil {
		return err
	}
	if err := checkLen("description", t.Description, maxDescriptionLen); err != nil {
		return err
	}
	if err := checkCount("operations", len(t.Operations), maxOperationsPerTweak); err != nil {
		return err
	}
	return checkCount("revert operations", len(t.Revert), maxOperationsPerTweak)
}

// checkOperationLimits bounds every field of one operation. It runs before the
// type-specific checks so an over-long value is rejected before it is decoded
// or handed to a downstream validator.
func checkOperationLimits(o Operation) error {
	if len(o.Value) > maxRawValueBytes {
		return fmt.Errorf("value exceeds the %d-byte limit (%d bytes)", maxRawValueBytes, len(o.Value))
	}
	if err := checkLen("path", o.Path, maxRegistryPathLen); err != nil {
		return err
	}
	if err := checkCount("args", len(o.Args), maxArgsPerOperation); err != nil {
		return err
	}
	for i, arg := range o.Args {
		if err := checkLen(fmt.Sprintf("arg %d", i+1), arg, maxArgLen); err != nil {
			return err
		}
	}
	return nil
}

func validateOperation(o Operation) error {
	if err := checkOperationLimits(o); err != nil {
		return err
	}

	requireRegistryTarget := func() error {
		switch o.Hive {
		case "HKLM", "HKCU", "HKCR", "HKU":
		default:
			return fmt.Errorf("invalid registry hive %q", o.Hive)
		}
		if strings.TrimSpace(o.Path) == "" {
			return errors.New("registry path is required")
		}
		// A registry value name longer than Windows accepts can only fail at
		// execution time, so reject it while the error is still diagnosable.
		return checkLen("registry value name", o.Name, maxRegistryValueNameLen)
	}

	switch o.Type {
	case OpRegistrySetDword:
		if err := requireRegistryTarget(); err != nil {
			return err
		}
		_, err := o.DwordValue()
		return err
	case OpRegistrySetQword:
		if err := requireRegistryTarget(); err != nil {
			return err
		}
		_, err := o.QwordValue()
		return err
	case OpRegistrySetString:
		if err := requireRegistryTarget(); err != nil {
			return err
		}
		value, err := o.StringValue()
		if err != nil {
			return err
		}
		return checkLen("string value", value, maxStringValueLen)
	case OpRegistryDelete:
		return requireRegistryTarget()
	case OpServiceStartMode:
		if strings.TrimSpace(o.Name) == "" {
			return errors.New("service name is required")
		}
		if err := checkLen("service name", o.Name, maxServiceNameLen); err != nil {
			return err
		}
		mode, err := o.StringValue()
		if err != nil {
			return err
		}
		_, err = service.ParseStartMode(mode)
		return err
	case OpServiceStart, OpServiceStop:
		if strings.TrimSpace(o.Name) == "" {
			return errors.New("service name is required")
		}
		return checkLen("service name", o.Name, maxServiceNameLen)
	case OpTaskDisable, OpTaskEnable, OpTaskDelete:
		if strings.TrimSpace(o.Path) == "" {
			return errors.New("scheduled task path is required")
		}
		return checkLen("scheduled task path", o.Path, maxTaskPathLen)
	case OpAppxRemove:
		if strings.TrimSpace(o.Name) == "" {
			return errors.New("Appx package name is required")
		}
		// An Appx package name is an identity used to select what gets removed,
		// so it is rejected when over-long rather than truncated: a truncated
		// identity could match a different package than the one intended.
		return checkLen("Appx package name", o.Name, maxAppxNameLen)
	case OpPowerScheme:
		value, err := o.StringValue()
		if err != nil {
			return err
		}
		if _, err := power.Resolve(value); err != nil {
			return err
		}
		return nil
	case OpCommand:
		command, err := o.StringValue()
		if err != nil {
			return err
		}
		if command == "" {
			return errors.New("command executable is required")
		}
		if err := checkLen("command", command, maxCommandLen); err != nil {
			return err
		}
		if strings.ContainsAny(command, "\x00<>|&") {
			return errors.New("command must name an executable, not use shell syntax")
		}
		if strings.ContainsAny(command, " \t\r\n") && !strings.ContainsAny(command, `/\`) {
			return errors.New("command arguments must be supplied in the args array")
		}
		for _, arg := range o.Args {
			if strings.ContainsAny(arg, "\x00\"") {
				return errors.New("command args must not contain NULs or shell-style quotes")
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown operation type %q", o.Type)
	}
}

func displayName(id string) string {
	name := id
	for _, prefix := range []string{"atlas-", "winutil-wpf", "debloat-", "winforge-"} {
		name = strings.TrimPrefix(name, prefix)
	}
	words := strings.FieldsFunc(name, func(r rune) bool { return r == '-' || r == '_' })
	for i, word := range words {
		runes := []rune(word)
		if len(runes) == 0 {
			continue
		}
		runes[0] = unicode.ToUpper(runes[0])
		words[i] = string(runes)
	}
	if len(words) == 0 {
		return id
	}
	return strings.Join(words, " ")
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

package config

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"testing"
	"testing/fstest"
	"unicode/utf8"

	"winforge"
)

// TestEmbeddedCatalogLoadsUnderFieldLimits is the guard requested for the
// per-field bounds: it parses and validates every catalog the loader reads from
// the real embedded FS. If a bound is set tighter than the shipped catalog, the
// product would fail to start, so this must fail loudly rather than subtly.
func TestEmbeddedCatalogLoadsUnderFieldLimits(t *testing.T) {
	loader := NewLoader("")

	tweaks, err := loader.LoadTweaks()
	if err != nil {
		t.Fatalf("embedded tweaks.json rejected by the field limits: %v", err)
	}
	if len(tweaks.Tweaks) == 0 {
		t.Fatal("embedded tweaks.json parsed to zero tweaks; the test would not prove anything")
	}

	apps, err := loader.LoadApps()
	if err != nil {
		t.Fatalf("embedded applications.json rejected by the field limits: %v", err)
	}
	if len(apps.Applications) == 0 {
		t.Fatal("embedded applications.json parsed to zero applications")
	}

	dns, err := loader.LoadDns()
	if err != nil {
		t.Fatalf("embedded dns.json rejected by the field limits: %v", err)
	}
	if len(dns.Presets) == 0 {
		t.Fatal("embedded dns.json parsed to zero presets")
	}

	services, err := loader.LoadProtectedServices()
	if err != nil {
		t.Fatalf("embedded protectedServices.json rejected by the field limits: %v", err)
	}
	if len(services) == 0 {
		t.Fatal("embedded protectedServices.json parsed to zero services")
	}
}

// TestEveryEmbeddedConfigFileIsParseable walks the whole embedded config
// directory, so a newly added catalog file cannot ship as invalid JSON simply
// because no loader method references it yet. playbooks.json and debloat.json
// have no Go loader today and would otherwise go entirely unchecked.
func TestEveryEmbeddedConfigFileIsParseable(t *testing.T) {
	entries, err := fs.ReadDir(winforge.Assets, "config")
	if err != nil {
		t.Fatalf("read embedded config dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("embedded config directory is empty")
	}

	seen := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		seen++
		name := path.Join("config", entry.Name())
		t.Run(entry.Name(), func(t *testing.T) {
			b, err := fs.ReadFile(winforge.Assets, name)
			if err != nil {
				t.Fatalf("read %s: %v", name, err)
			}
			if len(b) > maxConfigFileBytes {
				t.Fatalf("%s is %d bytes, over the %d-byte loader cap", name, len(b), maxConfigFileBytes)
			}
			var doc any
			if err := json.Unmarshal(b, &doc); err != nil {
				t.Fatalf("%s is not valid JSON: %v", name, err)
			}
			if !utf8.Valid(b) {
				t.Fatalf("%s is not valid UTF-8; rune-based limits would be meaningless", name)
			}
		})
	}
	if seen < 4 {
		t.Fatalf("only %d embedded JSON catalogs found; expected the full catalog set", seen)
	}
}

// TestEmbeddedCatalogHasHeadroomUnderLimits asserts the shipped catalog is not
// merely under each bound but comfortably under it. A catalog sitting at 99% of
// a limit would mean the next legitimate entry breaks the build, which is the
// failure mode the handover asked to avoid.
func TestEmbeddedCatalogHasHeadroomUnderLimits(t *testing.T) {
	const minHeadroomFactor = 2 // every bound must be at least 2x the largest shipped value

	tweaks, err := loadTweaksFromEmbedded(t)
	if err != nil {
		t.Fatalf("load embedded tweaks: %v", err)
	}

	type measure struct {
		field string
		got   int
		limit int
	}
	var measures []measure
	record := func(field string, got, limit int) {
		measures = append(measures, measure{field, got, limit})
	}

	maxOf := func(vals ...int) int {
		best := 0
		for _, v := range vals {
			if v > best {
				best = v
			}
		}
		return best
	}

	var id, name, category, desc, ops, revert int
	for i := range tweaks.Tweaks {
		tw := &tweaks.Tweaks[i]
		id = maxOf(id, utf8.RuneCountInString(tw.ID))
		name = maxOf(name, utf8.RuneCountInString(tw.Name))
		category = maxOf(category, utf8.RuneCountInString(tw.Category))
		desc = maxOf(desc, utf8.RuneCountInString(tw.Description))
		ops = maxOf(ops, len(tw.Operations))
		revert = maxOf(revert, len(tw.Revert))
	}
	record("tweak.id", id, maxIDLen)
	record("tweak.name", name, maxNameLen)
	record("tweak.category", category, maxCategoryLen)
	record("tweak.description", desc, maxDescriptionLen)
	record("tweak.operations", ops, maxOperationsPerTweak)
	record("tweak.revert", revert, maxOperationsPerTweak)
	record("tweaks", len(tweaks.Tweaks), maxTweaks)

	for _, m := range measures {
		if m.got == 0 {
			continue
		}
		if m.got*minHeadroomFactor > m.limit {
			t.Errorf("%s: shipped max %d leaves less than %dx headroom under limit %d",
				m.field, m.got, minHeadroomFactor, m.limit)
		}
	}
}

func loadTweaksFromEmbedded(t *testing.T) (*TweakConfig, error) {
	t.Helper()
	return NewLoader("").LoadTweaks()
}

// oversize builds a string of n ASCII runes.
func oversize(n int) string { return strings.Repeat("a", n) }

func TestTweakFieldLimitsAreEnforced(t *testing.T) {
	validOp := Operation{Type: OpCommand, Value: json.RawMessage(`"example.exe"`)}

	tests := []struct {
		name  string
		tweak Tweak
	}{
		{
			name:  "id over limit",
			tweak: Tweak{ID: oversize(maxIDLen + 1), Operations: []Operation{validOp}},
		},
		{
			name:  "name over limit",
			tweak: Tweak{ID: "t", Name: oversize(maxNameLen + 1), Operations: []Operation{validOp}},
		},
		{
			name:  "category over limit",
			tweak: Tweak{ID: "t", Category: oversize(maxCategoryLen + 1), Operations: []Operation{validOp}},
		},
		{
			name:  "description over limit",
			tweak: Tweak{ID: "t", Description: oversize(maxDescriptionLen + 1), Operations: []Operation{validOp}},
		},
		{
			name:  "too many operations",
			tweak: Tweak{ID: "t", Operations: repeatOp(validOp, maxOperationsPerTweak+1)},
		},
		{
			name: "too many revert operations",
			tweak: Tweak{
				ID: "t", Reversible: true,
				Operations: []Operation{validOp},
				Revert:     repeatOp(validOp, maxOperationsPerTweak+1),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &TweakConfig{Tweaks: []Tweak{tt.tweak}}
			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate accepted an over-long field")
			}
		})
	}
}

func repeatOp(op Operation, n int) []Operation {
	ops := make([]Operation, n)
	for i := range ops {
		ops[i] = op
	}
	return ops
}

func TestTweakListLimitIsEnforced(t *testing.T) {
	tweaks := make([]Tweak, maxTweaks+1)
	for i := range tweaks {
		tweaks[i] = Tweak{
			ID:         fmt.Sprintf("tweak-%d", i),
			Operations: []Operation{{Type: OpCommand, Value: json.RawMessage(`"example.exe"`)}},
		}
	}
	if err := (&TweakConfig{Tweaks: tweaks}).Validate(); err == nil {
		t.Fatal("Validate accepted more tweaks than the list limit")
	}
}

func TestOperationFieldLimitsAreEnforced(t *testing.T) {
	tests := []struct {
		name string
		op   Operation
	}{
		{
			name: "registry path over limit",
			op:   Operation{Type: OpRegistryDelete, Hive: "HKLM", Path: oversize(maxRegistryPathLen + 1)},
		},
		{
			name: "registry value name over limit",
			op: Operation{
				Type: OpRegistrySetDword, Hive: "HKLM", Path: "A",
				Name: oversize(maxRegistryValueNameLen + 1), Value: json.RawMessage(`1`),
			},
		},
		{
			name: "registry string value over limit",
			op: Operation{
				Type: OpRegistrySetString, Hive: "HKLM", Path: "A", Name: "B",
				Value: json.RawMessage(`"` + oversize(maxStringValueLen+1) + `"`),
			},
		},
		{
			name: "service name over limit",
			op:   Operation{Type: OpServiceStart, Name: oversize(maxServiceNameLen + 1)},
		},
		{
			name: "task path over limit",
			op:   Operation{Type: OpTaskDisable, Path: oversize(maxTaskPathLen + 1)},
		},
		{
			name: "appx name over limit",
			op:   Operation{Type: OpAppxRemove, Name: oversize(maxAppxNameLen + 1)},
		},
		{
			name: "command over limit",
			op: Operation{
				Type:  OpCommand,
				Value: json.RawMessage(`"` + oversize(maxCommandLen+1) + `"`),
			},
		},
		{
			name: "single arg over limit",
			op: Operation{
				Type:  OpCommand,
				Value: json.RawMessage(`"example.exe"`),
				Args:  []string{oversize(maxArgLen + 1)},
			},
		},
		{
			name: "too many args",
			op: Operation{
				Type:  OpCommand,
				Value: json.RawMessage(`"example.exe"`),
				Args:  make([]string, maxArgsPerOperation+1),
			},
		},
		{
			name: "raw value over limit",
			op: Operation{
				Type:  OpRegistrySetString,
				Hive:  "HKLM",
				Path:  "A",
				Name:  "B",
				Value: json.RawMessage(`"` + oversize(maxRawValueBytes) + `"`),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &TweakConfig{Tweaks: []Tweak{{ID: "t", Operations: []Operation{tt.op}}}}
			if err := cfg.Validate(); err == nil {
				t.Fatalf("Validate accepted an over-long operation field: %+v", tt.op)
			}
		})
	}
}

// TestOperationFieldsAtLimitAreAccepted pins the boundary from the other side:
// a value exactly at the bound must still load, so the limits reject only what
// they are documented to reject.
func TestOperationFieldsAtLimitAreAccepted(t *testing.T) {
	tests := []struct {
		name string
		op   Operation
	}{
		{
			name: "registry path at limit",
			op:   Operation{Type: OpRegistryDelete, Hive: "HKLM", Path: oversize(maxRegistryPathLen)},
		},
		{
			name: "service name at limit",
			op:   Operation{Type: OpServiceStart, Name: oversize(maxServiceNameLen)},
		},
		{
			name: "task path at limit",
			op:   Operation{Type: OpTaskDisable, Path: oversize(maxTaskPathLen)},
		},
		{
			name: "appx name at limit",
			op:   Operation{Type: OpAppxRemove, Name: oversize(maxAppxNameLen)},
		},
		{
			name: "arg at limit",
			op: Operation{
				Type:  OpCommand,
				Value: json.RawMessage(`"example.exe"`),
				Args:  []string{oversize(maxArgLen)},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &TweakConfig{Tweaks: []Tweak{{ID: "t", Operations: []Operation{tt.op}}}}
			if err := cfg.Validate(); err != nil {
				t.Fatalf("Validate rejected a field exactly at its limit: %v", err)
			}
		})
	}
}

// TestLimitsCountRunesNotBytes ensures a multi-byte field is measured in
// characters. Measuring bytes would reject a legitimate non-ASCII catalog at
// roughly a third of the documented limit.
func TestLimitsCountRunesNotBytes(t *testing.T) {
	// Three bytes per rune, exactly at the rune limit.
	name := strings.Repeat("あ", maxNameLen)
	if len(name) <= maxNameLen {
		t.Fatalf("test precondition failed: %d bytes is not over the byte count", len(name))
	}
	cfg := &TweakConfig{Tweaks: []Tweak{{
		ID:         "unicode",
		Name:       name,
		Operations: []Operation{{Type: OpCommand, Value: json.RawMessage(`"example.exe"`)}},
	}}}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate measured bytes instead of runes: %v", err)
	}
}

func TestAppFieldLimitsAreEnforced(t *testing.T) {
	tests := []struct {
		name string
		app  App
	}{
		{"name over limit", App{ID: "Vendor.App", Name: oversize(maxNameLen + 1)}},
		{"category over limit", App{ID: "Vendor.App", Name: "n", Category: oversize(maxCategoryLen + 1)}},
		{"description over limit", App{ID: "Vendor.App", Name: "n", Description: oversize(maxDescriptionLen + 1)}},
		{"tag over limit", App{ID: "Vendor.App", Name: "n", Tags: []string{oversize(maxTagLen + 1)}}},
		{"too many tags", App{ID: "Vendor.App", Name: "n", Tags: make([]string, maxTagsPerApp+1)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &AppsConfig{Applications: []App{tt.app}}
			if err := cfg.Validate(); err == nil {
				t.Fatalf("Validate accepted an over-long application field: %+v", tt.app)
			}
		})
	}
}

func TestAppListLimitIsEnforced(t *testing.T) {
	apps := make([]App, maxApplications+1)
	for i := range apps {
		apps[i] = App{ID: fmt.Sprintf("Vendor.App%d", i), Name: "n"}
	}
	if err := (&AppsConfig{Applications: apps}).Validate(); err == nil {
		t.Fatal("Validate accepted more applications than the list limit")
	}
}

func TestDnsFieldLimitsAreEnforced(t *testing.T) {
	t.Run("profile over limit", func(t *testing.T) {
		cfg := &DnsConfig{Presets: []DnsEntry{{
			Profile: oversize(maxDnsProfileLen + 1),
			Primary: "1.1.1.1",
		}}}
		if err := cfg.Validate(); err == nil {
			t.Fatal("Validate accepted an over-long DNS profile")
		}
	})

	t.Run("too many presets", func(t *testing.T) {
		presets := make([]DnsEntry, maxDnsPresets+1)
		for i := range presets {
			presets[i] = DnsEntry{Profile: fmt.Sprintf("p%d", i), Primary: "1.1.1.1"}
		}
		if err := (&DnsConfig{Presets: presets}).Validate(); err == nil {
			t.Fatal("Validate accepted more DNS presets than the list limit")
		}
	})
}

func TestProtectedServiceLimitsAreEnforced(t *testing.T) {
	t.Run("name over limit", func(t *testing.T) {
		cfg := &ProtectedServices{Services: []string{oversize(maxServiceNameLen + 1)}}
		if err := cfg.Validate(); err == nil {
			t.Fatal("Validate accepted an over-long protected service name")
		}
	})

	t.Run("too many services", func(t *testing.T) {
		services := make([]string, maxProtectedServices+1)
		for i := range services {
			services[i] = fmt.Sprintf("svc%d", i)
		}
		if err := (&ProtectedServices{Services: services}).Validate(); err == nil {
			t.Fatal("Validate accepted more protected services than the list limit")
		}
	})
}

// TestLoaderRejectsOversizedFieldEndToEnd proves the bounds apply through the
// real loader path, not only when Validate is called directly.
func TestLoaderRejectsOversizedFieldEndToEnd(t *testing.T) {
	raw := fmt.Sprintf(
		`{"tweaks":[{"id":"t","name":%q,"operations":[{"type":"command","value":"example.exe"}]}]}`,
		oversize(maxNameLen+1),
	)
	fsys := fstest.MapFS{"config/tweaks.json": &fstest.MapFile{Data: []byte(raw)}}
	if _, err := (&Loader{embedded: fsys}).LoadTweaks(); err == nil {
		t.Fatal("LoadTweaks accepted a tweak whose name exceeds the field limit")
	}
}

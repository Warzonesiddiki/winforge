package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func loadTweaks(t *testing.T, raw string) *TweakConfig {
	t.Helper()
	fsys := fstest.MapFS{"config/tweaks.json": &fstest.MapFile{Data: []byte(raw)}}
	l := &Loader{embedded: fsys}
	c, err := l.LoadTweaks()
	if err != nil {
		t.Fatalf("LoadTweaks: %v", err)
	}
	return c
}

func TestLoadTweaksValid(t *testing.T) {
	raw := `{"tweaks":[
		{"id":"t1","name":"One","category":"privacy","description":"d","risk":"low",
		 "reversible":true,
		 "operations":[{"type":"registry_set_dword","hive":"HKLM","path":"A","name":"B","value":1}],
		 "revert":[{"type":"registry_delete","hive":"HKLM","path":"A","name":"B"}]}
	]}`
	c := loadTweaks(t, raw)
	if len(c.Tweaks) != 1 {
		t.Fatalf("want 1 tweak, got %d", len(c.Tweaks))
	}
	if c.Tweaks[0].Risk != RiskLow {
		t.Errorf("want risk low, got %q", c.Tweaks[0].Risk)
	}
}

func TestEmbeddedConfigReadIsBounded(t *testing.T) {
	fsys := fstest.MapFS{
		"config/tweaks.json": &fstest.MapFile{Data: make([]byte, maxConfigFileBytes+1)},
	}
	if _, err := (&Loader{embedded: fsys}).LoadTweaks(); err == nil {
		t.Fatal("LoadTweaks accepted an oversized embedded catalog")
	}
}

func TestOverrideConfigReadIsBounded(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tweaks.json")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(path, maxConfigFileBytes+1); err != nil {
		t.Fatal(err)
	}
	if _, err := NewLoader(dir).LoadTweaks(); err == nil {
		t.Fatal("LoadTweaks accepted an oversized override")
	}
}

func TestOverrideConfigRejectsSymbolicLink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(t.TempDir(), "tweaks.json")
	if err := os.WriteFile(target, []byte(`{"tweaks":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "tweaks.json")); err != nil {
		t.Skipf("symbolic links unavailable: %v", err)
	}
	if _, err := NewLoader(dir).LoadTweaks(); err == nil {
		t.Fatal("LoadTweaks accepted a symbolic-link override")
	}
}

func TestLoadTweaksRejectsUnknownFields(t *testing.T) {
	raw := `{"tweaks":[
		{"id":"typo","operations":[{"type":"registry_set_dword","hive":"HKLM","path":"A","nmae":"B","value":1}]}
	]}`
	fsys := fstest.MapFS{"config/tweaks.json": &fstest.MapFile{Data: []byte(raw)}}
	l := &Loader{embedded: fsys}
	if _, err := l.LoadTweaks(); err == nil {
		t.Fatal("expected unknown-field decoding error")
	}
}

func TestDecodeJSONRejectsTrailingValue(t *testing.T) {
	var cfg TweakConfig
	if err := DecodeJSON([]byte(`{"tweaks":[]} {"tweaks":[]}`), &cfg); err == nil {
		t.Fatal("DecodeJSON accepted multiple configuration values")
	}
}

func TestLoadTweaksRejectsReversibleTweakWithoutExplicitRevert(t *testing.T) {
	raw := `{"tweaks":[
		{"id":"unsafe-undo","reversible":true,
		 "operations":[{"type":"registry_set_dword","hive":"HKLM","path":"A","name":"B","value":1}]}
	]}`
	fsys := fstest.MapFS{"config/tweaks.json": &fstest.MapFile{Data: []byte(raw)}}
	l := &Loader{embedded: fsys}
	if _, err := l.LoadTweaks(); err == nil {
		t.Fatal("expected missing-revert validation error")
	}
}

func TestLoadTweaksDuplicateID(t *testing.T) {
	raw := `{"tweaks":[
		{"id":"dup","name":"A","operations":[{"type":"command","value":"x"}]},
		{"id":"dup","name":"B","operations":[{"type":"command","value":"y"}]}
	]}`
	fsys := fstest.MapFS{"config/tweaks.json": &fstest.MapFile{Data: []byte(raw)}}
	l := &Loader{embedded: fsys}
	if _, err := l.LoadTweaks(); err == nil {
		t.Fatal("expected duplicate-id error")
	}
}

func TestOverrideDirectoryShadowsEmbeddedConfig(t *testing.T) {
	embedded := fstest.MapFS{"config/tweaks.json": &fstest.MapFile{Data: []byte(`{"tweaks":[{"id":"embedded","operations":[{"type":"command","value":"embedded.exe"}]}]}`)}}
	dir := t.TempDir()
	override := `{"tweaks":[{"id":"override","operations":[{"type":"command","value":"override.exe"}]}]}`
	if err := os.WriteFile(filepath.Join(dir, "tweaks.json"), []byte(override), 0o600); err != nil {
		t.Fatal(err)
	}

	loader := &Loader{OverrideDir: dir, embedded: embedded}
	cfg, err := loader.LoadTweaks()
	if err != nil {
		t.Fatalf("LoadTweaks: %v", err)
	}
	if len(cfg.Tweaks) != 1 || cfg.Tweaks[0].ID != "override" {
		t.Fatalf("override was ignored: %+v", cfg.Tweaks)
	}
}

func TestLoadProtectedServicesRejectsMalformedEntries(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{name: "blank", raw: `{"services":[" "]}`},
		{name: "padded", raw: `{"services":[" RpcSs"]}`},
		{name: "case-insensitive duplicate", raw: `{"services":["RpcSs","rpcss"]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := fstest.MapFS{"config/protectedServices.json": &fstest.MapFile{Data: []byte(tt.raw)}}
			loader := &Loader{embedded: fsys}
			if _, err := loader.LoadProtectedServices(); err == nil {
				t.Fatal("LoadProtectedServices accepted malformed protection entry")
			}
		})
	}
}

func TestLoadDnsRejectsMalformedAndDuplicatePresets(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{name: "blank profile", raw: `{"presets":[{"profile":" ","primary":"1.1.1.1"}]}`},
		{name: "malformed address", raw: `{"presets":[{"profile":"Bad","primary":"999.1.1.1"}]}`},
		{name: "IPv6 address", raw: `{"presets":[{"profile":"IPv6","primary":"2606:4700:4700::1111"}]}`},
		{name: "case-insensitive duplicate", raw: `{"presets":[{"profile":"Cloudflare","primary":"1.1.1.1"},{"profile":"cloudflare","primary":"1.0.0.1"}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := fstest.MapFS{"config/dns.json": &fstest.MapFile{Data: []byte(tt.raw)}}
			loader := &Loader{embedded: fsys}
			if _, err := loader.LoadDns(); err == nil {
				t.Fatal("LoadDns accepted malformed DNS catalog")
			}
		})
	}
}

func TestLoadAppsRejectsMalformedAndDuplicatePackageIDs(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{
			name: "option-like id",
			raw:  `{"applications":[{"id":"--source.Evil","name":"Evil"}]}`,
		},
		{
			name: "case-insensitive duplicate",
			raw:  `{"applications":[{"id":"Vendor.App","name":"One"},{"id":"vendor.app","name":"Two"}]}`,
		},
		{
			name: "missing name",
			raw:  `{"applications":[{"id":"Vendor.App","name":"  "}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := fstest.MapFS{"config/applications.json": &fstest.MapFile{Data: []byte(tt.raw)}}
			loader := &Loader{embedded: fsys}
			if _, err := loader.LoadApps(); err == nil {
				t.Fatal("LoadApps accepted malformed application catalog")
			}
		})
	}
}

func TestOperationDwordValue(t *testing.T) {
	op := Operation{Value: json.RawMessage(`42`)}
	v, err := op.DwordValue()
	if err != nil {
		t.Fatal(err)
	}
	if v != 42 {
		t.Errorf("want 42, got %d", v)
	}
}

func TestOperationStringValue(t *testing.T) {
	op := Operation{Value: json.RawMessage(`"hello"`)}
	v, err := op.StringValue()
	if err != nil {
		t.Fatal(err)
	}
	if v != "hello" {
		t.Errorf("want hello, got %q", v)
	}
}

func TestOperationQwordValueSupportsFullUnsignedRange(t *testing.T) {
	op := Operation{Type: OpRegistrySetQword, Value: json.RawMessage(`18446744073709551615`)}
	v, err := op.QwordValue()
	if err != nil {
		t.Fatal(err)
	}
	if v != ^uint64(0) {
		t.Errorf("want max uint64, got %d", v)
	}
}

func TestValidateReportsOperationContext(t *testing.T) {
	cfg := &TweakConfig{Tweaks: []Tweak{{
		ID:   "broken",
		Name: "Broken",
		Operations: []Operation{{
			Type: OpRegistrySetDword, Hive: "INVALID", Path: "A", Value: json.RawMessage(`1`),
		}},
	}}}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
	want := `tweak "broken" operation 1: invalid registry hive "INVALID"`
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err, want)
	}
}

func TestValidateRejectsMalformedOperations(t *testing.T) {
	tests := []struct {
		name string
		op   Operation
	}{
		{"unknown type", Operation{Type: "unknown"}},
		{"missing registry path", Operation{Type: OpRegistryDelete, Hive: "HKLM"}},
		{"bad service mode", Operation{Type: OpServiceStartMode, Name: "svc", Value: json.RawMessage(`"sometimes"`)}},
		{"bad power scheme", Operation{Type: OpPowerScheme, Value: json.RawMessage(`"not-a-scheme"`)}},
		{"shell command", Operation{Type: OpCommand, Value: json.RawMessage(`"cmd.exe & whoami"`)}},
		{"missing task path", Operation{Type: OpTaskDisable}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &TweakConfig{Tweaks: []Tweak{{ID: "test", Operations: []Operation{tt.op}}}}
			if err := cfg.Validate(); err == nil {
				t.Fatalf("operation was accepted: %+v", tt.op)
			}
		})
	}
}

func TestValidateSuppliesFallbackName(t *testing.T) {
	cfg := &TweakConfig{Tweaks: []Tweak{{
		ID:         "winforge-example-tweak",
		Operations: []Operation{{Type: OpCommand, Value: json.RawMessage(`"example.exe"`)}},
	}}}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := cfg.Tweaks[0].Name; got != "Example Tweak" {
		t.Fatalf("fallback name = %q, want %q", got, "Example Tweak")
	}
}

func TestValidateSuppliesUnicodeFallbackName(t *testing.T) {
	cfg := &TweakConfig{Tweaks: []Tweak{{
		ID:         "élan-vital",
		Operations: []Operation{{Type: OpCommand, Value: json.RawMessage(`"example.exe"`)}},
	}}}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := cfg.Tweaks[0].Name; got != "Élan Vital" {
		t.Fatalf("fallback name = %q, want %q", got, "Élan Vital")
	}
}

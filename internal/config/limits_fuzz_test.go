package config

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

// TestLimitsFuzzStringAndPath exercises the 16 KiB string/path bounds with
// many sizes around the boundary. It is the sandbox-verifiable counterpart to
// a fuzzing pass: every size from 16k-10 to 16k+10 is tried for both the
// registry string value and the registry path, plus multi-byte rune cases.
func TestLimitsFuzzStringAndPath(t *testing.T) {
	// Test string value 16k boundary
	for delta := -10; delta <= 10; delta++ {
		n := maxStringValueLen + delta
		s := strings.Repeat("a", n)
		op := Operation{
			Type:  OpRegistrySetString,
			Hive:  "HKCU",
			Path:  `Software\WinForge`,
			Name:  "Value",
			Value: json.RawMessage(fmt.Sprintf("%q", s)),
		}
		cfg := &TweakConfig{Tweaks: []Tweak{{ID: "t", Operations: []Operation{op}}}}
		err := cfg.Validate()
		if n <= maxStringValueLen && err != nil {
			t.Errorf("string len %d (<= %d) should be accepted but got %v", n, maxStringValueLen, err)
		}
		if n > maxStringValueLen && err == nil {
			t.Errorf("string len %d (> %d) should be rejected", n, maxStringValueLen)
		}
		// Rune vs byte: multi-byte string of same rune count must also respect rune limit
		if n == maxStringValueLen {
			mb := strings.Repeat("あ", n) // 3 bytes per rune, n runes
			if utf8.RuneCountInString(mb) != n {
				t.Fatalf("rune count mismatch")
			}
			opMB := Operation{
				Type:  OpRegistrySetString,
				Hive:  "HKCU",
				Path:  `Software\WinForge`,
				Name:  "Value",
				Value: json.RawMessage(fmt.Sprintf("%q", mb)),
			}
			cfgMB := &TweakConfig{Tweaks: []Tweak{{ID: "t", Operations: []Operation{opMB}}}}
			if err := cfgMB.Validate(); err != nil {
				t.Errorf("multi-byte string of %d runes should be accepted: %v", n, err)
			}
			overMB := strings.Repeat("あ", n+1)
			opOver := Operation{
				Type:  OpRegistrySetString,
				Hive:  "HKCU",
				Path:  `Software\WinForge`,
				Name:  "Value",
				Value: json.RawMessage(fmt.Sprintf("%q", overMB)),
			}
			cfgOver := &TweakConfig{Tweaks: []Tweak{{ID: "t", Operations: []Operation{opOver}}}}
			if err := cfgOver.Validate(); err == nil {
				t.Errorf("multi-byte string of %d runes should be rejected", n+1)
			}
		}
	}

	// Registry path 512 boundary (not 16k but similar fuzz)
	for delta := -5; delta <= 5; delta++ {
		n := maxRegistryPathLen + delta
		p := strings.Repeat("a", n)
		op := Operation{Type: OpRegistryDelete, Hive: "HKLM", Path: p}
		cfg := &TweakConfig{Tweaks: []Tweak{{ID: "t", Operations: []Operation{op}}}}
		err := cfg.Validate()
		if n <= maxRegistryPathLen && err != nil {
			t.Errorf("path len %d (<=%d) should be accepted but got %v", n, maxRegistryPathLen, err)
		}
		if n > maxRegistryPathLen && err == nil {
			t.Errorf("path len %d (> %d) should be rejected", n, maxRegistryPathLen)
		}
	}

	// Args 1024 boundary
	for delta := -5; delta <= 5; delta++ {
		n := maxArgLen + delta
		arg := strings.Repeat("a", n)
		op := Operation{Type: OpCommand, Value: json.RawMessage(`"example.exe"`), Args: []string{arg}}
		cfg := &TweakConfig{Tweaks: []Tweak{{ID: "t", Operations: []Operation{op}}}}
		err := cfg.Validate()
		if n <= maxArgLen && err != nil {
			t.Errorf("arg len %d (<=%d) should be accepted but got %v", n, maxArgLen, err)
		}
		if n > maxArgLen && err == nil {
			t.Errorf("arg len %d (> %d) should be rejected", n, maxArgLen)
		}
	}
}

// TestLimitsMaxValuesAreDocumented ensures the 16k constants are not silently
// changed. The changelog narrative in CATALOG_PARITY.md references these values.
func TestLimitsMaxValuesAreDocumented(t *testing.T) {
	if maxStringValueLen != 16384 {
		t.Errorf("maxStringValueLen = %d, want 16384 (documented 16k)", maxStringValueLen)
	}
	if maxRegistryValueNameLen != 16383 {
		t.Errorf("maxRegistryValueNameLen = %d, want 16383 (Windows limit)", maxRegistryValueNameLen)
	}
}

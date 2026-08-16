package power

import (
	"strings"
	"testing"
)

func TestResolveRejectsMalformedGUIDs(t *testing.T) {
	malformed := []string{
		"381b4222f694-41f0-9685-ff5bb260df2e",  // missing required hyphen
		"381b422-2f694-41f0-9685-ff5bb260df2e", // hyphen in the wrong position
		"381b4222-f694-41f0-9685-ff5bb260df2g", // non-hexadecimal digit
	}
	for _, value := range malformed {
		if got, err := Resolve(value); err == nil {
			t.Errorf("Resolve(%q) = %q, nil; want error", value, got)
		}
	}
}

func TestResolveNormalizesAliasesAndGUIDCase(t *testing.T) {
	if got, err := Resolve(" Ultimate "); err != nil || got != UltimateClone {
		t.Fatalf("Resolve(Ultimate) = %q, %v; want %q, nil", got, err, UltimateClone)
	}
	upper := strings.ToUpper(Balanced)
	if got, err := Resolve(upper); err != nil || got != Balanced {
		t.Fatalf("Resolve(%q) = %q, %v; want %q, nil", upper, got, err, Balanced)
	}
}

func TestSetProcessorStateValidatesPercentages(t *testing.T) {
	for _, tc := range []struct {
		min uint32
		max uint32
	}{
		{min: 101, max: 101},
		{min: 20, max: 101},
		{min: 80, max: 20},
	} {
		if err := SetProcessorState(tc.min, tc.max); err == nil {
			t.Errorf("SetProcessorState(%d, %d) error = nil, want validation error", tc.min, tc.max)
		}
	}
}

func TestGuidPartsRoundTrip(t *testing.T) {
	d1, d2, d3, d4, err := guidParts(SubProcessor) // 54533251-82be-4824-96c1-47b60b740d00
	if err != nil {
		t.Fatalf("guidParts(%q) error = %v", SubProcessor, err)
	}
	if d1 != 0x54533251 || d2 != 0x82be || d3 != 0x4824 {
		t.Fatalf("guidParts data1-3 = %08x %04x %04x", d1, d2, d3)
	}
	want4 := [8]byte{0x96, 0xc1, 0x47, 0xb6, 0x0b, 0x74, 0x0d, 0x00}
	if d4 != want4 {
		t.Fatalf("guidParts data4 = %x, want %x", d4, want4)
	}
	if _, _, _, _, err := guidParts("not-a-guid"); err == nil {
		t.Fatal("guidParts accepted a malformed GUID")
	}
}

func TestSetAcValueIndexValidatesIdentifiers(t *testing.T) {
	if err := SetAcValueIndex("not-a-scheme", SubProcessor, SettingMinProc, 50); err == nil {
		t.Fatal("SetAcValueIndex() accepted invalid scheme")
	}
	if err := SetAcValueIndex("SCHEME_CURRENT", "not-a-subgroup", SettingMinProc, 50); err == nil {
		t.Fatal("SetAcValueIndex() accepted invalid subgroup")
	}
	if err := SetAcValueIndex("SCHEME_CURRENT", SubProcessor, "not-a-setting", 50); err == nil {
		t.Fatal("SetAcValueIndex() accepted invalid setting")
	}
}

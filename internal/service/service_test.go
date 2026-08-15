package service

import (
	"errors"
	"testing"
)

func TestSetStartModeRejectsInvalidValueBeforePlatformDispatch(t *testing.T) {
	err := SetStartMode("example", StartMode(99))
	if err == nil {
		t.Fatal("SetStartMode accepted an invalid start mode")
	}
	if errors.Is(err, ErrUnsupported) {
		t.Fatalf("invalid mode reached platform dispatch: %v", err)
	}
}

func TestParseStartModeRoundTrip(t *testing.T) {
	for _, mode := range []StartMode{StartAuto, StartManual, StartDisabled} {
		parsed, err := ParseStartMode(mode.String())
		if err != nil {
			t.Fatalf("ParseStartMode(%q): %v", mode, err)
		}
		if parsed != mode {
			t.Fatalf("ParseStartMode(%q) = %d, want %d", mode, parsed, mode)
		}
	}
}

func TestParseStartModeNormalizesConfigSpelling(t *testing.T) {
	mode, err := ParseStartMode("  AUTOMATIC  ")
	if err != nil {
		t.Fatalf("ParseStartMode: %v", err)
	}
	if mode != StartAuto {
		t.Fatalf("ParseStartMode = %d, want %d", mode, StartAuto)
	}
}

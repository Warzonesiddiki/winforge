package isobuilder

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateUnattendXMLBasic(t *testing.T) {
	cfg := UnattendConfig{Language: "en-US", Locale: "en-US", Label: "WINFORGE"}
	xmlStr, err := GenerateUnattendXML(cfg)
	if err != nil {
		t.Fatalf("GenerateUnattendXML: %v", err)
	}
	if !strings.Contains(xmlStr, "<unattend") {
		t.Error("missing <unattend> root")
	}
	if !strings.Contains(xmlStr, "en-US") {
		t.Error("missing language")
	}
	if err := ValidateUnattendXML(xmlStr); err != nil {
		t.Fatalf("ValidateUnattendXML failed: %v", err)
	}
	// Must be parseable by encoding/xml (covered by Validate)
}

func TestGenerateUnattendXMLBypass(t *testing.T) {
	cfg := UnattendConfig{Language: "de-DE", Locale: "de-DE", Label: "TEST", BypassRequirements: true}
	xmlStr, err := GenerateUnattendXML(cfg)
	if err != nil {
		t.Fatalf("GenerateUnattendXML: %v", err)
	}
	if !strings.Contains(xmlStr, "BypassTPMCheck") {
		t.Error("expected BypassTPMCheck in bypass XML")
	}
	if !strings.Contains(xmlStr, "BypassSecureBootCheck") {
		t.Error("expected BypassSecureBootCheck")
	}
}

func TestGenerateUnattendXMLWithEditions(t *testing.T) {
	cfg := UnattendConfig{Language: "en-US", Label: "MYLABEL", Editions: []string{"Pro", "Enterprise"}}
	xmlStr, err := GenerateUnattendXML(cfg)
	if err != nil {
		t.Fatalf("GenerateUnattendXML: %v", err)
	}
	if !strings.Contains(xmlStr, "Pro") || !strings.Contains(xmlStr, "Enterprise") {
		t.Error("editions not embedded")
	}
}

func TestValidateUnattendXMLRejectsBad(t *testing.T) {
	if err := ValidateUnattendXML(""); err == nil {
		t.Error("expected error for empty xml")
	}
	if err := ValidateUnattendXML("<notunattend></notunattend>"); err == nil {
		t.Error("expected error for missing unattend root")
	}
	if err := ValidateUnattendXML("<unattend><bad>"); err == nil {
		t.Error("expected error for malformed xml")
	}
}

func TestValidateUnattendConfigDefaults(t *testing.T) {
	cfg := UnattendConfig{}
	if err := ValidateUnattendConfig(&cfg); err != nil {
		t.Fatalf("ValidateUnattendConfig: %v", err)
	}
	if cfg.Language != "en-US" {
		t.Errorf("Language = %q, want en-US", cfg.Language)
	}
	if cfg.Label != "WINFORGE" {
		t.Errorf("Label = %q, want WINFORGE", cfg.Label)
	}
}

func TestValidateUnattendConfigRejectsBadLanguage(t *testing.T) {
	cfg := UnattendConfig{Language: "not-a-locale!"}
	if err := ValidateUnattendConfig(&cfg); err == nil {
		t.Error("expected error for invalid language")
	}
}

func TestWriteUnattendFile(t *testing.T) {
	dir := t.TempDir()
	cfg := UnattendConfig{Language: "fr-FR", Locale: "fr-FR", Label: "TEST"}
	path, err := WriteUnattendFile(dir, cfg)
	if err != nil {
		t.Fatalf("WriteUnattendFile: %v", err)
	}
	if filepath.Base(path) != "Autounattend.xml" {
		t.Errorf("path = %q, want Autounattend.xml", path)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if err := ValidateUnattendXML(string(raw)); err != nil {
		t.Fatalf("written xml invalid: %v", err)
	}
	// Ensure python xml validation would also pass (simple check)
	if !strings.Contains(string(raw), `<?xml`) {
		t.Error("missing xml declaration")
	}
}

func TestWriteUnattendFileRejectsMissingDir(t *testing.T) {
	if _, err := WriteUnattendFile("/nonexistent/dir/xyz", UnattendConfig{}); err == nil {
		t.Error("expected error for missing dir")
	}
}

func TestGenerateWimConfig(t *testing.T) {
	dir := makeSource(t, "install.wim")
	opts := Options{SourceDir: dir, OutputISO: filepath.Join(t.TempDir(), "out.iso"), Label: "WINFORGE"}
	xmlStr, err := GenerateWimConfig(opts, UnattendConfig{Language: "en-US"})
	if err != nil {
		t.Fatalf("GenerateWimConfig: %v", err)
	}
	if err := ValidateUnattendXML(xmlStr); err != nil {
		t.Fatalf("ValidateUnattendXML: %v", err)
	}
}

func TestGenerateWimConfigInvalidOptions(t *testing.T) {
	opts := Options{SourceDir: "/nonexistent", OutputISO: "out.iso"}
	if _, err := GenerateWimConfig(opts, UnattendConfig{}); err == nil {
		t.Error("expected error for invalid opts")
	}
}

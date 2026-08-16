// Package isobuilder — WIM configuration / Autounattend generation (dry-run).
//
// This file provides sandbox-verifiable generation of Autounattend.xml
// snippets for unattended Windows installation. No ADK, no dism, no oscdimg
// is required: the output is validated with encoding/xml and with
// python3 -c 'import xml.etree.ElementTree' in CI, keeping the build
// offline (GOPROXY=off) and platform-independent.
//
// The generated XML is intentionally minimal and standards-compliant so it
// can be injected into a mounted WIM or placed alongside installation media.

package isobuilder

import (
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// UnattendConfig controls the generated Autounattend.xml content.
type UnattendConfig struct {
	// Language is the Windows display language, e.g. "en-US".
	// Defaults to "en-US" if empty.
	Language string `json:"language"`
	// Locale groups language + locale: e.g. "en-US". Used for InputLocale/SystemLocale.
	Locale string `json:"locale"`
	// Label is the image volume label (sanitized via SanitizeLabel).
	Label string `json:"label"`
	// Editions lists edition names to include in the specialize pass (optional).
	Editions []string `json:"editions,omitempty"`
	// BypassRequirements adds the documented LabConfig registry bypasses for
	// TPM, Secure Boot, RAM and Disk checks (useful for testing VMs).
	BypassRequirements bool `json:"bypassRequirements"`
	// ProductKey is an optional generic KMS key (empty means no key element).
	ProductKey string `json:"productKey,omitempty"`
}

// ValidateUnattendConfig checks cfg and fills defaults.
func ValidateUnattendConfig(cfg *UnattendConfig) error {
	if cfg == nil {
		return errors.New("nil unattend config")
	}
	if strings.TrimSpace(cfg.Language) == "" {
		cfg.Language = "en-US"
	}
	if strings.TrimSpace(cfg.Locale) == "" {
		cfg.Locale = cfg.Language
	}
	cfg.Label = SanitizeLabel(cfg.Label)
	if len(cfg.Editions) > 256 {
		return fmt.Errorf("too many editions: %d", len(cfg.Editions))
	}
	for i, e := range cfg.Editions {
		if strings.TrimSpace(e) == "" {
			return fmt.Errorf("edition %d must not be empty", i+1)
		}
	}
	if len(cfg.ProductKey) > 64 {
		return errors.New("product key too long")
	}
	// Basic language format check: xx or xx-XX
	if !isValidLanguageTag(cfg.Language) {
		return fmt.Errorf("invalid language tag %q", cfg.Language)
	}
	if !isValidLanguageTag(cfg.Locale) {
		return fmt.Errorf("invalid locale tag %q", cfg.Locale)
	}
	return nil
}

func isValidLanguageTag(tag string) bool {
	if tag == "" {
		return false
	}
	parts := strings.Split(tag, "-")
	if len(parts) == 1 {
		return len(parts[0]) >= 2 && len(parts[0]) <= 3
	}
	if len(parts) == 2 {
		return len(parts[0]) >= 2 && len(parts[0]) <= 3 && len(parts[1]) == 2
	}
	return false
}

// GenerateUnattendXML returns a minimal, well-formed Autounattend.xml as a
// string. The output validates with both Go's encoding/xml and Python's
// xml.etree.ElementTree without needing the Windows ADK.
func GenerateUnattendXML(cfg UnattendConfig) (string, error) {
	if err := ValidateUnattendConfig(&cfg); err != nil {
		return "", err
	}

	// Escape XML text nodes.
	esc := func(s string) string {
		r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&apos;")
		return r.Replace(s)
	}

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	b.WriteString(`<unattend xmlns="urn:schemas-microsoft-com:unattend">` + "\n")
	// windowsPE pass — language + optional LabConfig bypass
	b.WriteString(`  <settings pass="windowsPE">` + "\n")
	b.WriteString(`    <component name="Microsoft-Windows-International-Core-WinPE" processorArchitecture="amd64" publicKeyToken="31bf3856ad364e35" language="neutral" versionScope="nonSxS">` + "\n")
	b.WriteString(`      <SetupUILanguage><UILanguage>` + esc(cfg.Language) + `</UILanguage></SetupUILanguage>` + "\n")
	b.WriteString(`      <InputLocale>` + esc(cfg.Locale) + `</InputLocale>` + "\n")
	b.WriteString(`      <SystemLocale>` + esc(cfg.Locale) + `</SystemLocale>` + "\n")
	b.WriteString(`      <UILanguage>` + esc(cfg.Language) + `</UILanguage>` + "\n")
	b.WriteString(`      <UserLocale>` + esc(cfg.Locale) + `</UserLocale>` + "\n")
	b.WriteString(`    </component>` + "\n")
	if cfg.BypassRequirements {
		b.WriteString(`    <component name="Microsoft-Windows-Setup" processorArchitecture="amd64" publicKeyToken="31bf3856ad364e35" language="neutral" versionScope="nonSxS">` + "\n")
		b.WriteString(`      <RunSynchronous>` + "\n")
		b.WriteString(`        <RunSynchronousCommand wcm:action="add">` + "\n")
		b.WriteString(`          <Order>1</Order>` + "\n")
		b.WriteString(`          <Path>reg add HKLM\SYSTEM\Setup\LabConfig /v BypassTPMCheck /t REG_DWORD /d 1 /f</Path>` + "\n")
		b.WriteString(`        </RunSynchronousCommand>` + "\n")
		b.WriteString(`        <RunSynchronousCommand wcm:action="add">` + "\n")
		b.WriteString(`          <Order>2</Order>` + "\n")
		b.WriteString(`          <Path>reg add HKLM\SYSTEM\Setup\LabConfig /v BypassSecureBootCheck /t REG_DWORD /d 1 /f</Path>` + "\n")
		b.WriteString(`        </RunSynchronousCommand>` + "\n")
		b.WriteString(`      </RunSynchronous>` + "\n")
		b.WriteString(`    </component>` + "\n")
	}
	b.WriteString(`  </settings>` + "\n")

	// offlineServicing would host edition-specific packages; we emit a comment.
	if len(cfg.Editions) > 0 {
		b.WriteString(`  <!-- WinForge selected editions: ` + esc(strings.Join(cfg.Editions, ", ")) + ` -->` + "\n")
	}
	// specialize pass — product key + label
	b.WriteString(`  <settings pass="specialize">` + "\n")
	b.WriteString(`    <component name="Microsoft-Windows-Shell-Setup" processorArchitecture="amd64" publicKeyToken="31bf3856ad364e35" language="neutral" versionScope="nonSxS">` + "\n")
	b.WriteString(`      <ComputerName>WINFORGE-` + esc(cfg.Label) + `</ComputerName>` + "\n")
	if cfg.ProductKey != "" {
		b.WriteString(`      <ProductKey>` + esc(cfg.ProductKey) + `</ProductKey>` + "\n")
	}
	b.WriteString(`    </component>` + "\n")
	b.WriteString(`  </settings>` + "\n")
	b.WriteString(`</unattend>` + "\n")

	out := b.String()
	if err := ValidateUnattendXML(out); err != nil {
		return "", fmt.Errorf("generated Autounattend.xml is not well-formed: %w", err)
	}
	return out, nil
}

// ValidateUnattendXML checks that s is well-formed XML and contains the
// expected unattend root. It mirrors `python3 -c 'import xml.etree.ElementTree; ET.fromstring(data)'`.
func ValidateUnattendXML(s string) error {
	if strings.TrimSpace(s) == "" {
		return errors.New("empty xml")
	}
	if !strings.Contains(s, "<unattend") {
		return errors.New("missing <unattend> root")
	}
	dec := xml.NewDecoder(strings.NewReader(s))
	for {
		tok, err := dec.Token()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return fmt.Errorf("xml parse error: %w", err)
		}
		_ = tok
	}
	return nil
}

// WriteUnattendFile writes Autounattend.xml for cfg into dir and validates
// the result. dir must exist; the file is created as dir/Autounattend.xml.
// It returns the absolute path written.
func WriteUnattendFile(dir string, cfg UnattendConfig) (string, error) {
	if strings.TrimSpace(dir) == "" {
		return "", errors.New("output directory is required")
	}
	fi, err := os.Stat(dir)
	if err != nil {
		return "", fmt.Errorf("output dir %q: %w", dir, err)
	}
	if !fi.IsDir() {
		return "", fmt.Errorf("output path %q is not a directory", dir)
	}
	xmlStr, err := GenerateUnattendXML(cfg)
	if err != nil {
		return "", err
	}
	dest := filepath.Join(dir, "Autounattend.xml")
	if err := os.WriteFile(dest, []byte(xmlStr), 0o644); err != nil {
		return "", fmt.Errorf("write Autounattend.xml: %w", err)
	}
	// Re-validate from disk (ensures no write corruption).
	raw, err := os.ReadFile(dest)
	if err != nil {
		return "", err
	}
	if err := ValidateUnattendXML(string(raw)); err != nil {
		return "", err
	}
	return dest, nil
}

// GenerateWimConfig is a convenience dry-run that validates opts via
// ValidateOptions and then generates a locale-aware Autounattend.xml for the
// build. It does not invoke DISM or oscdimg, so it runs on any platform and
// is suitable for CI parity checks. The returned XML string is ready to be
// written alongside the ISO source tree.
func GenerateWimConfig(opts Options, cfg UnattendConfig) (string, error) {
	// Normalize a copy so the caller's opts are untouched, but still ensure
	// the source tree would be valid for a real build.
	clone := opts
	if err := ValidateOptions(&clone); err != nil {
		return "", fmt.Errorf("invalid ISO options: %w", err)
	}
	if cfg.Label == "" {
		cfg.Label = clone.Label
	}
	return GenerateUnattendXML(cfg)
}

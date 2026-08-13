package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateOpenedAuditFileRejectsIdentityChangedAfterInspection(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.jsonl")
	second := filepath.Join(dir, "second.jsonl")
	if err := os.WriteFile(first, []byte("first\n"), 0o600); err != nil {
		t.Fatalf("write first file: %v", err)
	}
	if err := os.WriteFile(second, []byte("second\n"), 0o600); err != nil {
		t.Fatalf("write second file: %v", err)
	}
	expected, err := os.Lstat(first)
	if err != nil {
		t.Fatalf("inspect first file: %v", err)
	}
	opened, err := os.Open(second)
	if err != nil {
		t.Fatalf("open second file: %v", err)
	}
	defer opened.Close()

	err = validateOpenedAuditFile(second, opened, expected)
	if err == nil || !strings.Contains(err.Error(), "after it was inspected") {
		t.Fatalf("validateOpenedAuditFile() error = %v, want identity-change error", err)
	}
}

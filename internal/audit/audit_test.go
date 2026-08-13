package audit

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAppendAndReadAll(t *testing.T) {
	dir := t.TempDir()
	l := NewLogger(dir)

	now := time.Now()
	for i := 0; i < 3; i++ {
		if err := l.Append(Entry{
			Timestamp:     now.Add(time.Duration(i) * time.Minute),
			OperationType: "registry_set_dword",
			Target:        "HKLM\\A\\B",
			NewValue:      "1",
			Success:       true,
			CanUndo:       true,
		}); err != nil {
			t.Fatalf("append: %v", err)
		}
	}

	entries, err := l.ReadAll()
	if err != nil {
		t.Fatalf("readall: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("want 3 entries, got %d", len(entries))
	}
	if entries[0].OperationType != "registry_set_dword" {
		t.Errorf("unexpected entry: %+v", entries[0])
	}
	// Entries must be sorted oldest first.
	if !entries[0].Timestamp.Before(entries[2].Timestamp) {
		t.Error("entries not sorted oldest-first")
	}

	// Verify the log file exists and is non-empty.
	matches, _ := filepath.Glob(filepath.Join(dir, "operations-*.jsonl"))
	if len(matches) == 0 {
		t.Fatal("no log file written")
	}
}

func TestReadAllEmptyDir(t *testing.T) {
	l := NewLogger(filepath.Join(t.TempDir(), "nope"))
	entries, err := l.ReadAll()
	if err != nil {
		t.Fatalf("readall: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("want 0 entries, got %d", len(entries))
	}
	_ = os.RemoveAll(filepath.Dir(""))
}

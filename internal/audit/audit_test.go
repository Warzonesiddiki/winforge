package audit

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAuditLogNameValidation(t *testing.T) {
	tests := map[string]bool{
		"operations-2026-08-14.jsonl":       true,
		"operations-2026-08-14~0001.jsonl":  true,
		"operations-2026-08-14~9999.jsonl":  true,
		"operations-2026-02-30.jsonl":       false,
		"operations-2026-08-14~0000.jsonl":  false,
		"operations-2026-08-14~10000.jsonl": false,
		"operations-latest.jsonl":           false,
		"operations-2026-08-14.jsonl.bak":   false,
	}
	for name, want := range tests {
		if got := isAuditLogName(name); got != want {
			t.Errorf("isAuditLogName(%q) = %v, want %v", name, got, want)
		}
	}
}

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
}

func TestAppendRejectsSymlinkWithoutModifyingTarget(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.jsonl")
	if err := os.WriteFile(target, []byte("unchanged\n"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	stamp := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	if err := os.Symlink(target, filepath.Join(dir, "operations-2026-08-14.jsonl")); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}

	if err := NewLogger(dir).Append(Entry{Timestamp: stamp, ID: "must-not-be-written"}); err == nil {
		t.Fatal("Append accepted a symbolic-link audit path")
	}
	contents, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(contents) != "unchanged\n" {
		t.Fatalf("symlink target was modified: %q", contents)
	}
}

func TestReadAllRejectsOversizedFileAndKeepsOtherValidEntries(t *testing.T) {
	dir := t.TempDir()
	valid, err := json.Marshal(Entry{ID: "valid", Timestamp: time.Unix(1, 0), Success: true})
	if err != nil {
		t.Fatalf("marshal valid entry: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "operations-2026-08-13.jsonl"), append(valid, '\n'), 0o600); err != nil {
		t.Fatalf("write valid audit log: %v", err)
	}
	oversized := filepath.Join(dir, "operations-2026-08-14.jsonl")
	if err := os.WriteFile(oversized, nil, 0o600); err != nil {
		t.Fatalf("create oversized audit log: %v", err)
	}
	if err := os.Truncate(oversized, maxAuditFileBytes+1); err != nil {
		t.Fatalf("truncate oversized audit log: %v", err)
	}

	entries, err := NewLogger(dir).ReadAll()
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("ReadAll() error = %v, want size-limit error", err)
	}
	if len(entries) != 1 || entries[0].ID != "valid" {
		t.Fatalf("ReadAll() entries = %#v, want valid entry from bounded file", entries)
	}
}

func TestPruneLockedRemovesOldestSegmentsBeyondReadBound(t *testing.T) {
	dir := t.TempDir()
	logger := NewLogger(dir)
	var paths []string
	for i := 1; i <= 5; i++ {
		path := filepath.Join(dir, fmt.Sprintf("operations-2026-01-%02d.jsonl", i))
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Truncate(path, maxAuditFileBytes); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	if err := logger.pruneLocked(); err != nil {
		t.Fatalf("pruneLocked: %v", err)
	}
	if _, err := os.Stat(paths[0]); !os.IsNotExist(err) {
		t.Fatalf("oldest segment was not pruned: %v", err)
	}
	for _, path := range paths[1:] {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("retained segment %q: %v", path, err)
		}
	}
}

func TestAppendRotatesBeforeDailyLogExceedsReadLimit(t *testing.T) {
	dir := t.TempDir()
	stamp := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	path := filepath.Join(dir, "operations-2026-08-14.jsonl")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(path, maxAuditFileBytes-1); err != nil {
		t.Fatal(err)
	}

	if err := NewLogger(dir).Append(Entry{Timestamp: stamp, ID: "rotated"}); err != nil {
		t.Fatalf("Append did not rotate a full daily log: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != maxAuditFileBytes-1 {
		t.Fatalf("rotation changed full segment size to %d", info.Size())
	}
	entries, err := NewLogger(dir).ReadAll()
	if err == nil || !strings.Contains(err.Error(), "malformed audit record") {
		// The sparse fixture is deliberately not valid JSON, but the valid
		// rotated entry must still be retained alongside that reported damage.
		t.Fatalf("ReadAll() error = %v, want malformed base-segment error", err)
	}
	if len(entries) != 1 || entries[0].ID != "rotated" {
		t.Fatalf("ReadAll() entries = %#v, want rotated entry", entries)
	}
}

func writePaddedAuditFile(t *testing.T, dir, date string, entry Entry, size int) {
	t.Helper()
	encoded, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal audit entry: %v", err)
	}
	contents := append(encoded, '\n')
	if len(contents) > size {
		t.Fatalf("encoded audit entry needs %d bytes, exceeds requested %d", len(contents), size)
	}
	contents = append(contents, strings.Repeat(" ", size-len(contents))...)
	if err := os.WriteFile(filepath.Join(dir, "operations-"+date+".jsonl"), contents, 0o600); err != nil {
		t.Fatalf("write audit log: %v", err)
	}
}

func TestReadAllAggregateLimitKeepsNewestFiles(t *testing.T) {
	dir := t.TempDir()
	writePaddedAuditFile(t, dir, "2026-08-12", Entry{ID: "old", Timestamp: time.Unix(1, 0)}, 256)
	writePaddedAuditFile(t, dir, "2026-08-13", Entry{ID: "middle", Timestamp: time.Unix(2, 0)}, 256)
	writePaddedAuditFile(t, dir, "2026-08-14", Entry{ID: "new", Timestamp: time.Unix(3, 0)}, 256)

	entries, err := NewLogger(dir).readAll(readLimits{fileBytes: 256, totalBytes: 512, entries: 10})
	if err == nil || !strings.Contains(err.Error(), "older files omitted") {
		t.Fatalf("readAll() error = %v, want aggregate-limit error", err)
	}
	if len(entries) != 2 || entries[0].ID != "middle" || entries[1].ID != "new" {
		t.Fatalf("readAll() entries = %#v, want the two newest files", entries)
	}
}

func TestReadAllEntryLimitKeepsNewestEntries(t *testing.T) {
	dir := t.TempDir()
	var contents []byte
	for i := 1; i <= 5; i++ {
		encoded, err := json.Marshal(Entry{ID: fmt.Sprintf("entry-%d", i), Timestamp: time.Unix(int64(i), 0)})
		if err != nil {
			t.Fatalf("marshal entry %d: %v", i, err)
		}
		contents = append(contents, encoded...)
		contents = append(contents, '\n')
	}
	if err := os.WriteFile(filepath.Join(dir, "operations-2026-08-14.jsonl"), contents, 0o600); err != nil {
		t.Fatalf("write audit log: %v", err)
	}

	entries, err := NewLogger(dir).readAll(readLimits{fileBytes: 1 << 20, totalBytes: 1 << 20, entries: 3})
	if !errors.Is(err, errAuditEntryLimit) {
		t.Fatalf("readAll() error = %v, want entry-limit error", err)
	}
	if len(entries) != 3 || entries[0].ID != "entry-3" || entries[1].ID != "entry-4" || entries[2].ID != "entry-5" {
		t.Fatalf("readAll() entries = %#v, want newest three in chronological order", entries)
	}
}

func TestReadAllRejectsAuditSymlinkAndMatchingDirectory(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.jsonl")
	if err := os.WriteFile(target, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write symlink target: %v", err)
	}
	link := filepath.Join(dir, "operations-2026-08-14.jsonl")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "operations-2026-08-13.jsonl"), 0o700); err != nil {
		t.Fatalf("mkdir matching audit name: %v", err)
	}

	entries, err := NewLogger(dir).ReadAll()
	if err == nil || !strings.Contains(err.Error(), "symbolic link") || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("ReadAll() error = %v, want symlink and non-regular-file errors", err)
	}
	if len(entries) != 0 {
		t.Fatalf("ReadAll() entries = %#v, want none", entries)
	}
}

func TestReadAllReportsCorruptionAndKeepsValidEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "operations-2026-08-14.jsonl")
	first, err := json.Marshal(Entry{ID: "first", Timestamp: time.Unix(1, 0), Success: true})
	if err != nil {
		t.Fatalf("marshal first entry: %v", err)
	}
	second, err := json.Marshal(Entry{ID: "second", Timestamp: time.Unix(2, 0), Success: true})
	if err != nil {
		t.Fatalf("marshal second entry: %v", err)
	}
	contents := string(first) + "\nnot-json\n" + string(second) + "\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write corrupt audit log: %v", err)
	}

	entries, err := NewLogger(dir).ReadAll()
	if err == nil {
		t.Fatal("ReadAll() error = nil, want malformed-record error")
	}
	if !strings.Contains(err.Error(), "operations-2026-08-14.jsonl line 2") {
		t.Fatalf("ReadAll() error = %q, want file and line", err)
	}
	if len(entries) != 2 || entries[0].ID != "first" || entries[1].ID != "second" {
		t.Fatalf("ReadAll() entries = %#v, want both valid entries", entries)
	}
}

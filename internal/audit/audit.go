// Package audit provides the append-only operation log that powers WinForge's
// History view and undo functionality.
//
// Entries are written as JSON Lines to %LOCALAPPDATA%\WinForge\logs\
// operations-{yyyy-MM-dd}.jsonl. JSONL (one object per line) is used so that
// appends are O(1) and crash-safe, unlike a single JSON array.
package audit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Entry is one recorded operation, mirroring the product's audit schema.
type Entry struct {
	ID                    string    `json:"id"`
	Timestamp             time.Time `json:"timestamp"`
	OperationType         string    `json:"operationType"`
	Target                string    `json:"target"`
	PreviousValue         string    `json:"previousValue,omitempty"`
	PreviousValueCaptured bool      `json:"previousValueCaptured,omitempty"`
	PreviousValueExists   bool      `json:"previousValueExists,omitempty"`
	PreviousValueType     string    `json:"previousValueType,omitempty"`
	NewValue              string    `json:"newValue,omitempty"`
	Success               bool      `json:"success"`
	ErrorMessage          string    `json:"errorMessage,omitempty"`
	CanUndo               bool      `json:"canUndo"`
	UndoScript            string    `json:"undoScript,omitempty"`
	UndoOf                string    `json:"undoOf,omitempty"`

	// Structured fields (registry operations) enabling precise per-row undo.
	TweakID      string `json:"tweakId,omitempty"`
	RegistryHive string `json:"registryHive,omitempty"`
	RegistryPath string `json:"registryPath,omitempty"`
	RegistryName string `json:"registryName,omitempty"`
}

const (
	maxAuditFileBytes        = 16 << 20
	maxAuditReadBytes        = 64 << 20
	maxAuditEntries          = 100000
	maxAuditDirectoryEntries = 100000
	maxAuditFiles            = 10000
)

var errAuditEntryLimit = errors.New("audit history entry limit reached")

type readLimits struct {
	fileBytes  int64
	totalBytes int64
	entries    int
}

var defaultReadLimits = readLimits{
	fileBytes:  maxAuditFileBytes,
	totalBytes: maxAuditReadBytes,
	entries:    maxAuditEntries,
}

// Logger writes and reads audit entries under a single directory.
type Logger struct {
	dir string
	mu  sync.Mutex
}

// NewLogger creates a Logger rooted at dir, creating it if needed.
func NewLogger(dir string) *Logger { return &Logger{dir: dir} }

// logPath returns the first daily log file path for a timestamp.
func (l *Logger) logPath(t time.Time) string {
	return l.logSegmentPath(t, 0)
}

func (l *Logger) logSegmentPath(t time.Time, segment int) string {
	date := t.Format("2006-01-02")
	if segment == 0 {
		return filepath.Join(l.dir, fmt.Sprintf("operations-%s.jsonl", date))
	}
	// '~' sorts after the base filename's '.', preserving segment order when
	// ReadAll traverses filenames from newest to oldest.
	return filepath.Join(l.dir, fmt.Sprintf("operations-%s~%04d.jsonl", date, segment))
}

func validateOpenedAuditFile(path string, f *os.File, expected os.FileInfo) error {
	openedInfo, err := f.Stat()
	if err != nil {
		return err
	}
	pathInfo, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if pathInfo.Mode()&os.ModeSymlink != 0 || !pathInfo.Mode().IsRegular() {
		return errors.New("audit log must be a regular file, not a symbolic link or special file")
	}
	if !openedInfo.Mode().IsRegular() || !os.SameFile(openedInfo, pathInfo) {
		return errors.New("audit log changed while it was being opened")
	}
	if expected != nil && !os.SameFile(expected, openedInfo) {
		return errors.New("audit log changed after it was inspected")
	}
	return nil
}

// Append writes a single entry to the current day's log file.
func (l *Logger) Append(e Entry) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if e.ID == "" {
		e.ID = NewID()
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}

	var encoded bytes.Buffer
	enc := json.NewEncoder(&encoded)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(e); err != nil {
		return err
	}
	if encoded.Len() > maxAuditFileBytes {
		return fmt.Errorf("encoded audit entry size %d exceeds %d-byte file limit", encoded.Len(), maxAuditFileBytes)
	}

	if err := os.MkdirAll(l.dir, 0o700); err != nil {
		return err
	}
	for segment := 0; segment < 10000; segment++ {
		path := l.logSegmentPath(e.Timestamp, segment)
		var (
			expected os.FileInfo
			f        *os.File
			err      error
		)
		for {
			info, statErr := os.Lstat(path)
			switch {
			case statErr == nil:
				if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
					return errors.New("audit log must be a regular file, not a symbolic link or special file")
				}
				expected = info
				f, err = os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o600)
			case os.IsNotExist(statErr):
				// O_EXCL ensures a path introduced after Lstat is never followed.
				// If another process creates it first, inspect that file normally.
				expected = nil
				f, err = os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY|os.O_APPEND, 0o600)
				if os.IsExist(err) {
					continue
				}
			default:
				return statErr
			}
			break
		}
		if err != nil {
			return err
		}
		if err := validateOpenedAuditFile(path, f, expected); err != nil {
			return errors.Join(err, f.Close())
		}
		openedInfo, err := f.Stat()
		if err != nil {
			return errors.Join(err, f.Close())
		}
		if openedInfo.Size() > maxAuditFileBytes-int64(encoded.Len()) {
			if err := f.Close(); err != nil {
				return err
			}
			continue
		}

		written, writeErr := f.Write(encoded.Bytes())
		if writeErr == nil && written != encoded.Len() {
			writeErr = io.ErrShortWrite
		}
		syncErr := f.Sync()
		closeErr := f.Close()
		if persistErr := errors.Join(writeErr, syncErr, closeErr); persistErr != nil {
			return persistErr
		}
		return l.pruneLocked()
	}
	return errors.New("audit log exhausted its daily segment limit")
}

// pruneLocked keeps the complete retained history within the same aggregate
// bound used by ReadAll. Oldest whole segments are removed first, so normal
// operation cannot eventually make every undo fail merely because logs grew.
// Caller must hold l.mu.
func (l *Logger) pruneLocked() error {
	entries, err := readAuditDirectory(l.dir)
	if err != nil {
		return fmt.Errorf("scan audit history for retention: %w", err)
	}
	type retainedFile struct {
		path string
		size int64
	}
	files := make([]retainedFile, 0, len(entries))
	var total int64
	for _, entry := range entries {
		path := filepath.Join(l.dir, entry.Name())
		info, err := os.Lstat(path)
		if err != nil {
			return fmt.Errorf("inspect audit history for retention: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("audit log %q must be a regular file", path)
		}
		if info.Size() < 0 || info.Size() > maxAuditFileBytes {
			return fmt.Errorf("audit log %q has invalid size %d", path, info.Size())
		}
		files = append(files, retainedFile{path: path, size: info.Size()})
		total += info.Size()
	}
	for _, file := range files {
		if total <= maxAuditReadBytes {
			break
		}
		if err := os.Remove(file.path); err != nil {
			return fmt.Errorf("remove expired audit log %q: %w", file.path, err)
		}
		total -= file.size
	}
	return nil
}

func isAuditLogName(name string) bool {
	if !strings.HasPrefix(name, "operations-") || !strings.HasSuffix(name, ".jsonl") {
		return false
	}
	stem := strings.TrimSuffix(strings.TrimPrefix(name, "operations-"), ".jsonl")
	date := stem
	if len(stem) == len("2006-01-02~0001") {
		if stem[10] != '~' || stem[11:] == "0000" {
			return false
		}
		for _, digit := range stem[11:] {
			if digit < '0' || digit > '9' {
				return false
			}
		}
		date = stem[:10]
	}
	if len(date) != len("2006-01-02") {
		return false
	}
	_, err := time.Parse("2006-01-02", date)
	return err == nil
}

func readAuditDirectory(path string) ([]os.DirEntry, error) {
	dir, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	var matches []os.DirEntry
	var scanErr error
	seen := 0
	for {
		batch, readErr := dir.ReadDir(256)
		for _, entry := range batch {
			seen++
			if seen > maxAuditDirectoryEntries {
				scanErr = fmt.Errorf("audit directory exceeds %d-entry scan limit", maxAuditDirectoryEntries)
				break
			}
			if isAuditLogName(entry.Name()) {
				if len(matches) >= maxAuditFiles {
					scanErr = fmt.Errorf("audit directory exceeds %d-log-file limit", maxAuditFiles)
					break
				}
				matches = append(matches, entry)
			}
		}
		if scanErr != nil || errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			scanErr = readErr
			break
		}
	}
	closeErr := dir.Close()
	sort.Slice(matches, func(i, j int) bool { return matches[i].Name() < matches[j].Name() })
	return matches, errors.Join(scanErr, closeErr)
}

// ReadAll returns audit entries sorted oldest first. Reads are bounded so a
// corrupt or unexpectedly large log set cannot exhaust the dashboard process;
// when a bound is reached, the newest valid entries are returned with an error.
func (l *Logger) ReadAll() ([]Entry, error) {
	return l.readAll(defaultReadLimits)
}

func (l *Logger) readAll(limits readLimits) ([]Entry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if limits.fileBytes <= 0 || limits.totalBytes <= 0 || limits.entries <= 0 {
		return nil, errors.New("invalid audit read limits")
	}

	dirs, scanErr := readAuditDirectory(l.dir)
	if scanErr != nil && os.IsNotExist(scanErr) {
		return nil, nil
	}
	if scanErr != nil && len(dirs) == 0 {
		return nil, scanErr
	}

	var out []Entry
	var readErrs []error
	if scanErr != nil {
		readErrs = append(readErrs, scanErr)
	}
	var bytesRead int64
	// os.ReadDir sorts by filename. Daily audit filenames therefore run oldest
	// to newest; walk them backwards so a bounded partial result keeps recent
	// history rather than stale entries.
	for i := len(dirs) - 1; i >= 0; i-- {
		d := dirs[i]
		if !isAuditLogName(d.Name()) {
			continue
		}
		if d.Type()&os.ModeSymlink != 0 {
			readErrs = append(readErrs, fmt.Errorf("%s: audit log must not be a symbolic link", d.Name()))
			continue
		}
		if d.IsDir() {
			readErrs = append(readErrs, fmt.Errorf("%s: audit log is not a regular file", d.Name()))
			continue
		}
		info, err := d.Info()
		if err != nil {
			readErrs = append(readErrs, fmt.Errorf("inspect %s: %w", d.Name(), err))
			continue
		}
		if !info.Mode().IsRegular() {
			readErrs = append(readErrs, fmt.Errorf("%s: audit log is not a regular file", d.Name()))
			continue
		}
		if info.Size() > limits.fileBytes {
			readErrs = append(readErrs, fmt.Errorf("%s: audit log size %d exceeds %d-byte limit", d.Name(), info.Size(), limits.fileBytes))
			continue
		}
		if bytesRead+info.Size() > limits.totalBytes {
			readErrs = append(readErrs, fmt.Errorf("audit history exceeds %d-byte read limit; older files omitted", limits.totalBytes))
			break
		}
		remaining := limits.entries - len(out)
		if remaining <= 0 {
			readErrs = append(readErrs, errAuditEntryLimit)
			break
		}
		fileReadLimit := limits.fileBytes
		if left := limits.totalBytes - bytesRead; left < fileReadLimit {
			fileReadLimit = left
		}
		entries, consumed, err := l.readFile(filepath.Join(l.dir, d.Name()), info, remaining, fileReadLimit)
		bytesRead += consumed
		out = append(out, entries...)
		if bytesRead > limits.totalBytes {
			readErrs = append(readErrs, fmt.Errorf("audit history exceeds %d-byte read limit; older files omitted", limits.totalBytes))
			break
		}
		if err != nil {
			readErrs = append(readErrs, err)
			if errors.Is(err, errAuditEntryLimit) {
				break
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Timestamp.Before(out[j].Timestamp) })
	return out, errors.Join(readErrs...)
}

func (l *Logger) readFile(path string, expected os.FileInfo, entryLimit int, byteLimit int64) ([]Entry, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	if err := validateOpenedAuditFile(path, f, expected); err != nil {
		return nil, 0, errors.Join(err, f.Close())
	}
	b, readErr := io.ReadAll(io.LimitReader(f, byteLimit+1))
	closeErr := f.Close()
	if readErr != nil || closeErr != nil {
		return nil, int64(len(b)), errors.Join(readErr, closeErr)
	}
	if int64(len(b)) > byteLimit {
		return nil, int64(len(b)), fmt.Errorf("%s: audit log exceeds %d-byte read limit", filepath.Base(path), byteLimit)
	}

	lines := strings.Split(string(b), "\n")
	out := make([]Entry, 0, min(len(lines), entryLimit))
	var parseErrs []error
	// Parse newest lines first so an entry bound retains the most useful part of
	// a daily file. ReadAll sorts successful records chronologically afterward.
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if len(out) >= entryLimit {
			parseErrs = append(parseErrs, fmt.Errorf("%s: %w", filepath.Base(path), errAuditEntryLimit))
			break
		}
		var e Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			parseErrs = append(parseErrs, fmt.Errorf("%s line %d: malformed audit record: %w", filepath.Base(path), i+1, err))
			continue
		}
		out = append(out, e)
	}
	// Restore append order after scanning backward to retain the newest subset.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, int64(len(b)), errors.Join(parseErrs...)
}

var idSequence atomic.Uint64

// NewID returns a compact unique identifier for an operation. The PID and a
// process-local sequence prevent collisions when several entries are created
// within one clock tick.
func NewID() string {
	return fmt.Sprintf("op-%d-%d-%d", time.Now().UnixNano(), os.Getpid(), idSequence.Add(1))
}

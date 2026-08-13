// Package audit provides the append-only operation log that powers WinForge's
// History view and undo functionality.
//
// Entries are written as JSON Lines to %LOCALAPPDATA%\WinForge\logs\
// operations-{yyyy-MM-dd}.jsonl. JSONL (one object per line) is used so that
// appends are O(1) and crash-safe, unlike a single JSON array.
package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Entry is one recorded operation, mirroring the product's audit schema.
type Entry struct {
	ID            string    `json:"id"`
	Timestamp     time.Time `json:"timestamp"`
	OperationType string    `json:"operationType"`
	Target        string    `json:"target"`
	PreviousValue string    `json:"previousValue,omitempty"`
	NewValue      string    `json:"newValue,omitempty"`
	Success       bool      `json:"success"`
	ErrorMessage  string    `json:"errorMessage,omitempty"`
	CanUndo       bool      `json:"canUndo"`
	UndoScript    string    `json:"undoScript,omitempty"`

	// Structured fields (registry operations) enabling precise per-row undo.
	TweakID      string `json:"tweakId,omitempty"`
	RegistryHive string `json:"registryHive,omitempty"`
	RegistryPath string `json:"registryPath,omitempty"`
	RegistryName string `json:"registryName,omitempty"`
}

// Logger writes and reads audit entries under a single directory.
type Logger struct {
	dir string
	mu  sync.Mutex
}

// NewLogger creates a Logger rooted at dir, creating it if needed.
func NewLogger(dir string) *Logger { return &Logger{dir: dir} }

// logPath returns the daily log file path for a timestamp.
func (l *Logger) logPath(t time.Time) string {
	return filepath.Join(l.dir, fmt.Sprintf("operations-%s.jsonl", t.Format("2006-01-02")))
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

	if err := os.MkdirAll(l.dir, 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(l.logPath(e.Timestamp), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	return enc.Encode(e)
}

// ReadAll returns every entry across all log files, sorted oldest first.
func (l *Logger) ReadAll() ([]Entry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	dirs, err := os.ReadDir(l.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var out []Entry
	for _, d := range dirs {
		if d.IsDir() || !strings.HasPrefix(d.Name(), "operations-") || !strings.HasSuffix(d.Name(), ".jsonl") {
			continue
		}
		entries, err := l.readFile(filepath.Join(l.dir, d.Name()))
		if err != nil {
			return nil, err
		}
		out = append(out, entries...)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Timestamp.Before(out[j].Timestamp) })
	return out, nil
}

func (l *Logger) readFile(path string) ([]Entry, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []Entry
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var e Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			// Skip corrupt lines rather than failing the whole read.
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

// NewID returns a compact unique identifier for an operation. The PID is
// included so concurrent WinForge processes cannot collide.
func NewID() string {
	return fmt.Sprintf("op-%d-%d", time.Now().UnixNano(), os.Getpid())
}

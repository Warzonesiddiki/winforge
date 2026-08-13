// Package procout runs child processes while retaining a bounded amount of
// their combined standard output and standard error.
package procout

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"sync"
)

// LimitError reports that a child wrote more output than the caller allowed.
type LimitError struct {
	Limit int
}

func (e *LimitError) Error() string {
	return fmt.Sprintf("process output exceeds %d bytes", e.Limit)
}

type boundedBuffer struct {
	mu       sync.Mutex
	buf      bytes.Buffer
	limit    int
	overflow bool
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	originalLen := len(p)
	remaining := b.limit - b.buf.Len()
	if remaining < len(p) {
		b.overflow = true
		if remaining <= 0 {
			return originalLen, nil
		}
		p = p[:remaining]
	}
	_, _ = b.buf.Write(p) // bytes.Buffer.Write never returns an error.
	return originalLen, nil
}

func (b *boundedBuffer) result() ([]byte, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]byte(nil), b.buf.Bytes()...), b.overflow
}

// CombinedOutput runs cmd and returns at most limit bytes of its combined
// standard output and standard error. Output beyond the limit is discarded
// while the pipes continue to be drained, preventing a noisy child from
// deadlocking or growing the parent process without bound.
func CombinedOutput(cmd *exec.Cmd, limit int) ([]byte, error) {
	if cmd == nil {
		return nil, errors.New("nil command")
	}
	if limit < 0 {
		return nil, fmt.Errorf("invalid process output limit %d", limit)
	}
	if cmd.Stdout != nil || cmd.Stderr != nil {
		return nil, errors.New("command output is already configured")
	}

	output := &boundedBuffer{limit: limit}
	cmd.Stdout = output
	cmd.Stderr = output
	runErr := cmd.Run()
	data, overflow := output.result()
	if overflow {
		return data, errors.Join(runErr, &LimitError{Limit: limit})
	}
	return data, runErr
}

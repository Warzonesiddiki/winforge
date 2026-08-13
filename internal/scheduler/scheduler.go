// Package scheduler controls the Windows Task Scheduler through the native
// schtasks.exe command (no PowerShell, no COM).
package scheduler

import "errors"

// ErrUnsupported is returned on non-Windows platforms.
var ErrUnsupported = errors.New("task scheduler control is only supported on Windows")

// Enable enables a scheduled task by its path (e.g. "\Microsoft\Windows\Foo\Bar").
func Enable(path string) error { return enable(path) }

// Disable disables a scheduled task.
func Disable(path string) error { return disable(path) }

// Delete removes a scheduled task.
func Delete(path string) error { return deleteTask(path) }

// Register creates a weekly maintenance task that runs "<exePath> run-maintenance".
func Register(name, exePath string) error { return register(name, exePath) }

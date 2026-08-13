// Package updater searches for and installs Windows updates through the
// Windows Update Agent COM API (Microsoft.Update.Session), using raw ole32/
// oleaut32 P/Invoke — no PowerShell, no third-party modules.
//
// The platform-agnostic types and helpers in this file are unit-tested; the
// COM interop lives in updater_windows.go.
package updater

import "errors"

// ErrUnsupported is returned on non-Windows platforms.
var ErrUnsupported = errors.New("Windows Update control is only supported on Windows")

// Update is one update reported by Windows Update.
type Update struct {
	Title        string `json:"title"`
	IsInstalled  bool   `json:"isInstalled"`
	IsDownloaded bool   `json:"isDownloaded"`
	IsHidden     bool   `json:"isHidden"`
}

// ResultCode is the OperationResultCode of an update search/install operation.
type ResultCode int32

const (
	ResultNotStarted ResultCode = iota
	ResultInProgress
	ResultSucceeded
	ResultSucceededWithErrors
	ResultFailed
	ResultAborted
)

func (c ResultCode) String() string {
	switch c {
	case ResultNotStarted:
		return "not-started"
	case ResultInProgress:
		return "in-progress"
	case ResultSucceeded:
		return "succeeded"
	case ResultSucceededWithErrors:
		return "succeeded-with-errors"
	case ResultFailed:
		return "failed"
	case ResultAborted:
		return "aborted"
	default:
		return "unknown"
	}
}

// InstallResult reports the outcome of an install pass.
type InstallResult struct {
	ResultCode     ResultCode `json:"resultCode"`
	RebootRequired bool       `json:"rebootRequired"`
}

// Search returns updates matching installedOnly. When installedOnly is false it
// returns available (not-installed, not-hidden) updates.
func Search(installedOnly bool) ([]Update, error) { return search(installedOnly) }

// InstallAll downloads and installs every available update.
func InstallAll() (InstallResult, error) { return installAll() }

// searchCriteria builds the WQL-ish criteria string for the searcher.
func searchCriteria(installedOnly bool) string {
	if installedOnly {
		return "IsInstalled=1"
	}
	return "IsInstalled=0 and IsHidden=0"
}

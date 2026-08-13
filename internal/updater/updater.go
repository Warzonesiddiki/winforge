// Package updater searches for and installs Windows updates through the
// Windows Update Agent COM API (Microsoft.Update.Session), using raw ole32/
// oleaut32 P/Invoke — no PowerShell, no third-party modules.
//
// The platform-agnostic types and helpers in this file are unit-tested; the
// COM interop lives in updater_windows.go.
package updater

import (
	"errors"
	"unicode/utf8"
)

// ErrUnsupported is returned on non-Windows platforms.
var ErrUnsupported = errors.New("Windows Update control is only supported on Windows")

const (
	// Aggregate enumeration bounds. Individual BSTR and count checks alone
	// still allow a hostile or corrupt Windows Update Agent to force WinForge
	// to retain gigabytes: 100,000 updates each carrying a 1 MiB title, plus a
	// per-update error string for every one of them. These budgets cap total
	// retained memory instead of only per-item size.
	//
	// maxUpdateCount is far above any realistic pending-update backlog (a
	// neglected machine reports a few hundred) while still bounding the number
	// of COM round trips a single search or install performs.
	maxUpdateCount = 4096
	// maxBSTRBytes bounds one BSTR conversion. Update titles are short display
	// strings; 64 KiB is generous and rejects absurd buffers early.
	maxBSTRBytes = 64 << 10
	// maxTitleBytes bounds a single retained update title.
	maxTitleBytes = 4096
	// maxTitleBudgetBytes bounds all titles retained by one search.
	maxTitleBudgetBytes = 4 << 20
	// maxResultErrors and maxResultErrorBudgetBytes bound the per-update error
	// detail accumulated during a download or install so a failing batch cannot
	// be turned into an unbounded error string.
	maxResultErrors           = 64
	maxResultErrorBudgetBytes = 256 << 10
)

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

// truncateUTF8 bounds text to limit bytes without splitting a rune. Update
// titles originate from the Windows Update Agent and are only ever displayed,
// so trimming an oversized one is preferable to retaining it.
func truncateUTF8(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	const ellipsis = "…"
	if limit < len(ellipsis) {
		// Too small to hold even the marker; return nothing rather than slicing
		// with a negative index.
		return ""
	}
	text = text[:limit-len(ellipsis)]
	for len(text) > 0 && !utf8.ValidString(text) {
		text = text[:len(text)-1]
	}
	return text + ellipsis
}

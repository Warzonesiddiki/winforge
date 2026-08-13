package updater

import (
	"testing"
	"unicode/utf8"
)

func TestSearchCriteria(t *testing.T) {
	if got := searchCriteria(true); got != "IsInstalled=1" {
		t.Errorf("searchCriteria(true) = %q, want %q", got, "IsInstalled=1")
	}
	if got := searchCriteria(false); got != "IsInstalled=0 and IsHidden=0" {
		t.Errorf("searchCriteria(false) = %q, want %q", got, "IsInstalled=0 and IsHidden=0")
	}
}

func TestResultCodeString(t *testing.T) {
	cases := map[ResultCode]string{
		ResultNotStarted:          "not-started",
		ResultInProgress:          "in-progress",
		ResultSucceeded:           "succeeded",
		ResultSucceededWithErrors: "succeeded-with-errors",
		ResultFailed:              "failed",
		ResultAborted:             "aborted",
		ResultCode(99):            "unknown",
	}
	for code, want := range cases {
		if got := code.String(); got != want {
			t.Errorf("ResultCode(%d).String() = %q, want %q", code, got, want)
		}
	}
}

// TestTruncateUTF8 covers the aggregate title-bound helper. Update titles come
// from the Windows Update Agent, so the helper must never split a multi-byte
// rune and must always respect the byte limit.
func TestTruncateUTF8(t *testing.T) {
	cases := []struct {
		name  string
		in    string
		limit int
		want  string
	}{
		{"under limit is unchanged", "short", 64, "short"},
		{"exactly at limit is unchanged", "abcd", 4, "abcd"},
		{"ascii is truncated with ellipsis", "abcdefghij", 6, "abc…"},
		{"multibyte runes are not split", "héllo wörld", 8, "héll…"},
		{"emoji are not split", "a😀😀😀", 8, "a😀…"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := truncateUTF8(c.in, c.limit)
			if got != c.want {
				t.Errorf("truncateUTF8(%q, %d) = %q, want %q", c.in, c.limit, got, c.want)
			}
			if len(got) > c.limit {
				t.Errorf("truncateUTF8(%q, %d) returned %d bytes, over the limit", c.in, c.limit, len(got))
			}
			if !utf8.ValidString(got) {
				t.Errorf("truncateUTF8(%q, %d) = %q, which is not valid UTF-8", c.in, c.limit, got)
			}
		})
	}
}

// TestTruncateUTF8RespectsLimitForAllPrefixes guards the byte bound against
// off-by-one errors at every cut point of a multi-byte string.
func TestTruncateUTF8RespectsLimitForAllPrefixes(t *testing.T) {
	const input = "ünïcödé ströng with 😀 emoji and ASCII"
	for limit := 4; limit <= len(input)+4; limit++ {
		got := truncateUTF8(input, limit)
		if len(got) > limit {
			t.Fatalf("truncateUTF8(input, %d) returned %d bytes", limit, len(got))
		}
		if !utf8.ValidString(got) {
			t.Fatalf("truncateUTF8(input, %d) = %q, which is not valid UTF-8", limit, got)
		}
	}
}

// TestEnumerationBoundsAreRealistic pins the aggregate enumeration budgets so a
// future change cannot quietly restore effectively unbounded retention. The
// previous limits (100,000 updates each allowed a 1 MiB title) permitted a
// corrupt Windows Update Agent to exhaust memory.
func TestEnumerationBoundsAreRealistic(t *testing.T) {
	if maxUpdateCount > 16384 {
		t.Errorf("maxUpdateCount = %d, which is too permissive for a real update backlog", maxUpdateCount)
	}
	if maxTitleBytes > maxTitleBudgetBytes {
		t.Errorf("per-title bound %d exceeds the aggregate title budget %d", maxTitleBytes, maxTitleBudgetBytes)
	}
	if maxBSTRBytes > 1<<20 {
		t.Errorf("maxBSTRBytes = %d, which still allows megabyte-scale BSTRs", maxBSTRBytes)
	}
	// The worst case a single search or install can retain must stay bounded.
	if worst := maxTitleBudgetBytes + maxResultErrorBudgetBytes; worst > 16<<20 {
		t.Errorf("worst-case retained enumeration memory is %d bytes", worst)
	}
}

// TestTruncateUTF8TinyLimits ensures a limit smaller than the ellipsis marker
// returns empty rather than panicking on a negative slice index.
func TestTruncateUTF8TinyLimits(t *testing.T) {
	for limit := 0; limit < 3; limit++ {
		got := truncateUTF8("some long title", limit)
		if len(got) > limit {
			t.Errorf("truncateUTF8(_, %d) = %q, over the limit", limit, got)
		}
	}
}

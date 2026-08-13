package updater

import "testing"

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

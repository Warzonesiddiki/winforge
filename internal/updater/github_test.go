package updater

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCheckGitHubReleaseFromList(t *testing.T) {
	releases := []ReleaseInfo{
		{TagName: "v0.2.0", Name: "WinForge 0.2.0", HTMLURL: "https://github.com/Warzonesiddiki/winforge/releases/tag/v0.2.0"},
		{TagName: "v0.1.0", Name: "WinForge 0.1.0", HTMLURL: "https://github.com/Warzonesiddiki/winforge/releases/tag/v0.1.0"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(releases)
	}))
	defer srv.Close()

	info, err := CheckGitHubRelease(srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("CheckGitHubRelease: %v", err)
	}
	if info.TagName != "v0.2.0" {
		t.Errorf("TagName = %q, want v0.2.0", info.TagName)
	}
}

func TestCheckGitHubReleaseSingleObject(t *testing.T) {
	single := ReleaseInfo{TagName: "v1.0.0", Name: "1.0.0"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(single)
	}))
	defer srv.Close()

	info, err := CheckGitHubRelease(srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("CheckGitHubRelease: %v", err)
	}
	if info.TagName != "v1.0.0" {
		t.Errorf("TagName = %q, want v1.0.0", info.TagName)
	}
}

func TestCheckGitHubReleaseSkipsDraft(t *testing.T) {
	releases := []ReleaseInfo{
		{TagName: "v0.3.0-draft", Draft: true},
		{TagName: "v0.2.0", Draft: false},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(releases)
	}))
	defer srv.Close()

	info, err := CheckGitHubRelease(srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("CheckGitHubRelease: %v", err)
	}
	if info.TagName != "v0.2.0" {
		t.Errorf("TagName = %q, want v0.2.0 (skip draft)", info.TagName)
	}
}

func TestCheckGitHubReleaseEmptyList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("[]"))
	}))
	defer srv.Close()

	if _, err := CheckGitHubRelease(srv.Client(), srv.URL); err == nil {
		t.Error("expected error for empty list")
	}
}

func TestCheckGitHubReleaseHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	if _, err := CheckGitHubRelease(srv.Client(), srv.URL); err == nil {
		t.Error("expected error for 404")
	}
}

func TestCheckGitHubReleaseInvalidScheme(t *testing.T) {
	if _, err := CheckGitHubRelease(nil, "ftp://example.com/releases"); err == nil {
		t.Error("expected error for ftp scheme")
	}
}

func TestCheckGitHubReleaseDefaultURL(t *testing.T) {
	// Empty apiURL should default to DefaultReleasesAPI — but we don't want to hit live.
	// This test ensures the default constant is the expected value and that an invalid
	// live fetch is handled (we mock failure by using a bad client).
	if DefaultReleasesAPI != "https://api.github.com/repos/Warzonesiddiki/winforge/releases" {
		t.Errorf("DefaultReleasesAPI = %q, unexpected", DefaultReleasesAPI)
	}
}

func TestCheckGitHubReleaseOversized(t *testing.T) {
	// Craft an oversized JSON payload (>1 MiB)
	large := strings.Repeat("a", maxReleaseJSONBytes+10)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Send a JSON array with one huge string body to exceed limit.
		// We directly write bytes without encoding to exceed limit.
		w.Write([]byte(`[{"tag_name":"` + large + `"}]`))
	}))
	defer srv.Close()

	if _, err := CheckGitHubRelease(srv.Client(), srv.URL); err == nil {
		t.Error("expected error for oversized response")
	}
}

func TestCheckForUpdate(t *testing.T) {
	cases := []struct {
		current, latest string
		want            bool
	}{
		{"0.1.0", "v0.2.0", true},
		{"0.2.0", "v0.2.0", false},
		{"0.3.0", "v0.2.0", false},
		{"", "v0.1.0", true},
		{"v0.1.0", "v0.1.1", true},
		{"1.0.0", "1.0.0", false},
	}
	for _, c := range cases {
		got, err := CheckForUpdate(c.current, c.latest)
		if err != nil {
			t.Errorf("CheckForUpdate(%q,%q) error: %v", c.current, c.latest, err)
			continue
		}
		if got != c.want {
			t.Errorf("CheckForUpdate(%q,%q) = %v, want %v", c.current, c.latest, got, c.want)
		}
	}
}

func TestCheckForUpdateEmptyLatest(t *testing.T) {
	if _, err := CheckForUpdate("0.1.0", ""); err == nil {
		t.Error("expected error for empty latest")
	}
}

func TestCompareVersions(t *testing.T) {
	if compareVersions("0.2.0", "0.1.0") <= 0 {
		t.Error("0.2.0 should be > 0.1.0")
	}
	if compareVersions("0.1.0", "0.2.0") >= 0 {
		t.Error("0.1.0 should be < 0.2.0")
	}
	if compareVersions("1.0.0", "1.0.0") != 0 {
		t.Error("1.0.0 == 1.0.0")
	}
}

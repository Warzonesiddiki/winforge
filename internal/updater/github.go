// Package updater — GitHub Releases check (platform-independent, mocked in tests).
//
// This file provides a sandbox-verifiable update check against the GitHub
// Releases API (https://api.github.com/repos/Warzonesiddiki/winforge/releases)
// without requiring Windows, a live network fetch in CI, or a module proxy.
// Tests use net/http/httptest with GOPROXY=off and never hit the live API.
//
// The production call site passes the real API URL; tests pass the httptest
// server's URL. All JSON decoding is bounded and version strings are validated.

package updater

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// DefaultReleasesAPI is the live GitHub Releases endpoint for WinForge.
	DefaultReleasesAPI = "https://api.github.com/repos/Warzonesiddiki/winforge/releases"
	// maxReleaseJSONBytes caps a single releases API response. The real
	// payload for 30 releases is ~50 KiB; 1 MiB is generous and prevents a
	// rogue API response from retaining unbounded memory.
	maxReleaseJSONBytes = 1 << 20
	// maxReleaseTagLen mirrors maxIDLen from config/limits.go: a tag is
	// short, human-readable text.
	maxReleaseTagLen = 256
)

// ReleaseInfo describes one GitHub release (subset of the API fields we need).
type ReleaseInfo struct {
	TagName     string         `json:"tag_name"`
	Name        string         `json:"name"`
	HTMLURL     string         `json:"html_url"`
	PublishedAt time.Time      `json:"published_at"`
	Draft       bool           `json:"draft"`
	Prerelease  bool           `json:"prerelease"`
	Body        string         `json:"body"`
	Assets      []ReleaseAsset `json:"assets,omitempty"`
}

// ReleaseAsset is one downloadable asset attached to a release.
type ReleaseAsset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
	Size        int64  `json:"size"`
}

// CheckGitHubRelease fetches the releases list from apiURL (or
// DefaultReleasesAPI if empty) and returns the newest non-draft release.
// apiURL may be a httptest server URL in tests. client may be nil to use
// http.DefaultClient. The JSON payload is bounded to 1 MiB and titles/tags
// are validated via truncateUTF8 and length checks.
func CheckGitHubRelease(client *http.Client, apiURL string) (*ReleaseInfo, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if strings.TrimSpace(apiURL) == "" {
		apiURL = DefaultReleasesAPI
	}
	// Basic URL sanity: must be http(s) and under 2 KiB.
	if len(apiURL) > 2048 {
		return nil, errors.New("api URL too long")
	}
	if !strings.HasPrefix(apiURL, "http://") && !strings.HasPrefix(apiURL, "https://") {
		return nil, fmt.Errorf("invalid api URL scheme in %q", apiURL)
	}

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "WinForge-updater/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("github api %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	limited := io.LimitReader(resp.Body, maxReleaseJSONBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read releases: %w", err)
	}
	if len(raw) > maxReleaseJSONBytes {
		return nil, fmt.Errorf("releases response exceeds %d bytes", maxReleaseJSONBytes)
	}

	// The releases endpoint returns a JSON array; the latest endpoint returns
	// a single object. Support both shapes.
	var releases []*ReleaseInfo
	trimmed := strings.TrimSpace(string(raw))
	if strings.HasPrefix(trimmed, "[") {
		if err := json.Unmarshal(raw, &releases); err != nil {
			return nil, fmt.Errorf("decode releases: %w", err)
		}
	} else {
		var single ReleaseInfo
		if err := json.Unmarshal(raw, &single); err != nil {
			return nil, fmt.Errorf("decode release: %w", err)
		}
		releases = []*ReleaseInfo{&single}
	}

	if len(releases) == 0 {
		return nil, errors.New("no releases found")
	}

	// Pick newest non-draft. GitHub returns most-recent first, so first
	// non-draft is the answer; fall back to first entry if all are drafts.
	var chosen *ReleaseInfo
	for _, r := range releases {
		if r == nil {
			continue
		}
		if !r.Draft {
			chosen = r
			break
		}
	}
	if chosen == nil {
		chosen = releases[0]
	}

	if err := validateReleaseInfo(chosen); err != nil {
		return nil, err
	}
	return chosen, nil
}

// CheckForUpdate reports whether latestTag is newer than currentVersion.
// Both must be semver-like (v1.2.3 or 1.2.3). Comparison is lexicographic
// segment-wise so it works offline without a semver library. An empty
// currentVersion always reports update available.
func CheckForUpdate(currentVersion, latestTag string) (bool, error) {
	currentVersion = strings.TrimSpace(currentVersion)
	latestTag = strings.TrimSpace(latestTag)
	if latestTag == "" {
		return false, errors.New("latest tag is empty")
	}
	if utf8.RuneCountInString(currentVersion) > maxReleaseTagLen || utf8.RuneCountInString(latestTag) > maxReleaseTagLen {
		return false, errors.New("version string too long")
	}
	if currentVersion == "" {
		return true, nil
	}
	cv := normalizeVersion(currentVersion)
	lv := normalizeVersion(latestTag)
	if cv == lv {
		return false, nil
	}
	return compareVersions(lv, cv) > 0, nil
}

func normalizeVersion(v string) string {
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	v = strings.TrimSpace(v)
	// Strip build metadata (+...) and pre-release suffix for comparison,
	// but keep the numeric core.
	if idx := strings.Index(v, "+"); idx >= 0 {
		v = v[:idx]
	}
	return v
}

func compareVersions(a, b string) int {
	// Split on '.' and compare numerically where possible.
	splitA := strings.Split(a, ".")
	splitB := strings.Split(b, ".")
	n := len(splitA)
	if len(splitB) > n {
		n = len(splitB)
	}
	for i := 0; i < n; i++ {
		var av, bv string
		if i < len(splitA) {
			av = splitA[i]
		}
		if i < len(splitB) {
			bv = splitB[i]
		}
		// Strip non-digit suffix for numeric compare (e.g. "3-rc1" -> "3")
		avNum := leadingDigits(av)
		bvNum := leadingDigits(bv)
		ai := parseInt(avNum)
		bi := parseInt(bvNum)
		if ai != bi {
			if ai > bi {
				return 1
			}
			return -1
		}
		// If numeric parts equal, compare full segment lexicographically
		if av != bv {
			if av > bv {
				return 1
			}
			return -1
		}
	}
	return 0
}

func leadingDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else {
			break
		}
	}
	return b.String()
}

func parseInt(s string) int {
	if s == "" {
		return 0
	}
	n := 0
	for _, r := range s {
		n = n*10 + int(r-'0')
		if n > 1_000_000 {
			break
		}
	}
	return n
}

func validateReleaseInfo(r *ReleaseInfo) error {
	if r == nil {
		return errors.New("nil release")
	}
	if strings.TrimSpace(r.TagName) == "" {
		return errors.New("release tag_name is empty")
	}
	if utf8.RuneCountInString(r.TagName) > maxReleaseTagLen {
		return fmt.Errorf("tag_name exceeds %d characters", maxReleaseTagLen)
	}
	if utf8.RuneCountInString(r.Name) > 4096 {
		return errors.New("release name too long")
	}
	if len(r.Body) > 256<<10 {
		return errors.New("release body too long")
	}
	// Bound assets
	if len(r.Assets) > 256 {
		return fmt.Errorf("too many assets: %d", len(r.Assets))
	}
	for i, a := range r.Assets {
		if utf8.RuneCountInString(a.Name) > 256 {
			return fmt.Errorf("asset %d name too long", i)
		}
		if len(a.DownloadURL) > 2048 {
			return fmt.Errorf("asset %d download URL too long", i)
		}
	}
	return nil
}

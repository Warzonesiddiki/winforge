// Package bloatware detects preinstalled bloatware among the applications
// installed on the machine. The detection rules are platform-agnostic and
// unit-tested; the installed-application enumeration is build-tagged (it reads
// the registry uninstall keys on Windows).
package bloatware

import (
	"sort"
	"strings"
)

// exact are normalized (lowercased, whitespace-collapsed) display names of
// well-known bloatware, matched in full.
var exact = []string{
	"3d builder",
	"alarms & clock",
	"bing news",
	"bing weather",
	"camera",
	"candy crush friends",
	"candy crush saga",
	"candy crush soda saga",
	"cortana",
	"disney magic kingdoms",
	"facebook",
	"farmville 2: country escape",
	"feedback hub",
	"get help",
	"get started",
	"groove music",
	"hidden city: hidden object adventure",
	"instagram",
	"mail and calendar",
	"maps",
	"march of empires",
	"messaging",
	"microsoft mahjong",
	"microsoft minesweeper",
	"microsoft news",
	"microsoft solitaire collection",
	"microsoft sudoku",
	"mixed reality portal",
	"movies & tv",
	"netflix",
	"onenote",
	"paint 3d",
	"pandora",
	"people",
	"phone link",
	"phototastic collage",
	"royal revolt 2",
	"skype",
	"solitaire",
	"spotify music",
	"sticky notes",
	"sway",
	"tiktok",
	"tips",
	"twitter",
	"voice recorder",
	"weather",
	"word mobile",
	"xbox console companion",
	"xbox game bar",
	"xbox live",
	"your phone",

	// OEM / third-party preloads commonly considered bloatware.
	"booking.com",
	"dell supportassist",
	"hp games",
	"hp support assistant",
	"mcafee livesafe",
	"mcafee security scan plus",
	"mcafee webadvisor",
	"norton security scan",
	"priceline.com",
	"wildtangent games",
}

// signatures are normalized substrings whose presence marks an application as
// bloatware. They capture whole families (e.g. "Candy Crush Friends Saga")
// without requiring an exact name match.
var signatures = []string{
	"3d builder",
	"bing news",
	"bing weather",
	"booking.com",
	"bubble witch",
	"candy crush",
	"cortana",
	"dell supportassist",
	"disney",
	"dropbox promotion",
	"facebook",
	"farmville",
	"feedback hub",
	"get help",
	"get started",
	"groove music",
	"hp support assistant",
	"instagram",
	"mail and calendar",
	"march of empires",
	"mcafee",
	"messaging",
	"microsoft mahjong",
	"microsoft minesweeper",
	"microsoft news",
	"microsoft solitaire",
	"microsoft sudoku",
	"mixed reality",
	"movies & tv",
	"netflix",
	"norton",
	"office hub",
	"oneconnect",
	"onenote",
	"paint 3d",
	"pandora",
	"phone link",
	"priceline.com",
	"royal revolt",
	"skype",
	"solitaire",
	"spotify",
	"sticky notes",
	"supportassist",
	"tiktok",
	"tips",
	"twitter",
	"voice recorder",
	"wildtangent",
	"xbox",
	"your phone",
}

// Normalize lowercases s and collapses internal whitespace so that
// "Candy   Crush" and "candy crush" compare equal.
func Normalize(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

// IsBloatware reports whether the given display name is recognized bloatware.
func IsBloatware(name string) bool {
	n := Normalize(name)
	if n == "" {
		return false
	}
	for _, e := range exact {
		if n == e {
			return true
		}
	}
	for _, sig := range signatures {
		if strings.Contains(n, sig) {
			return true
		}
	}
	return false
}

// Detect returns the subset of names recognized as bloatware, deduplicated and
// sorted.
func Detect(names []string) []string {
	seen := make(map[string]bool, len(names))
	var out []string
	for _, name := range names {
		if !IsBloatware(name) || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

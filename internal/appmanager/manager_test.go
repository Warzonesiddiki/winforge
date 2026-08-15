package appmanager

import (
	"reflect"
	"strings"
	"testing"
)

func TestCollectReportOutputEnforcesLineAndByteLimits(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		lineLimit int
		byteLimit int
		want      []string
	}{
		{name: "lines", input: "one\ntwo\nthree\n", lineLimit: 2, byteLimit: 100, want: []string{"one", "two"}},
		{name: "bytes", input: "1234\n5678\n", lineLimit: 10, byteLimit: 5, want: []string{"1234"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var progress []string
			lines, err := collectReportOutput(strings.NewReader(tt.input), func(p Progress) {
				progress = append(progress, p.Line)
			}, tt.lineLimit, tt.byteLimit)
			if err == nil {
				t.Fatal("collectReportOutput unexpectedly succeeded")
			}
			if !reflect.DeepEqual(lines, tt.want) {
				t.Fatalf("lines = %#v, want %#v", lines, tt.want)
			}
			if !reflect.DeepEqual(progress, tt.want) {
				t.Fatalf("progress = %#v, want %#v", progress, tt.want)
			}
		})
	}
}

func TestCollectReportOutputDrainsAfterOversizedLine(t *testing.T) {
	input := strings.NewReader(strings.Repeat("x", maxReportLineBytes+1) + "\ntrailer\n")
	lines, err := collectReportOutput(input, nil, maxReportLines, maxReportBytes)
	if err == nil {
		t.Fatal("collectReportOutput unexpectedly accepted an oversized line")
	}
	if len(lines) != 0 {
		t.Fatalf("collectReportOutput retained %d lines, want 0", len(lines))
	}
	if input.Len() != 0 {
		t.Fatalf("collectReportOutput left %d bytes unread after scanner failure", input.Len())
	}
}

func TestParseSearchIDs(t *testing.T) {
	const (
		idColumn      = 30
		versionColumn = 62
	)
	displayWidth := func(s string) int {
		width := 0
		for _, r := range s {
			width += runeDisplayWidth(r)
		}
		return width
	}
	row := func(name, id, version, source string) string {
		return name + strings.Repeat(" ", idColumn-displayWidth(name)) +
			id + strings.Repeat(" ", versionColumn-idColumn-displayWidth(id)) +
			version + "  " + source
	}
	lines := []string{
		"A source agreement message that is not part of the table",
		row("Name", "Id", "Version", "Source"),
		strings.Repeat("-", 90),
		row("Visual Studio Code", "Microsoft.VisualStudioCode", "1.99.0", "winget"),
		row("Unicode 工具", "Vendor.UnicodeTool", "2.0", "winget"),
		row("日本語アプリ", "Vendor.JapaneseTool", "3.0", "winget"),
		row("Cafe\u0301 utility", "Vendor.CombiningTool", "4.0", "winget"),
		row("Store application", "9NBLGGH4NNS1", "Unknown", "msstore"),
		row("Truncated ID", "Publisher.VeryLongPack…", "1.0", "winget"),
		row("Visual Studio Code duplicate", "Microsoft.VisualStudioCode", "1.99.0", "winget"),
	}
	want := []string{
		"Microsoft.VisualStudioCode",
		"Vendor.UnicodeTool",
		"Vendor.JapaneseTool",
		"Vendor.CombiningTool",
		"9NBLGGH4NNS1",
	}
	if got := parseSearchIDs(lines); !reflect.DeepEqual(got, want) {
		t.Fatalf("parseSearchIDs() = %#v, want %#v", got, want)
	}
}

func TestParseSearchIDsRequiresTableHeader(t *testing.T) {
	if got := parseSearchIDs([]string{"No package found matching input criteria."}); got != nil {
		t.Fatalf("parseSearchIDs() = %#v, want nil", got)
	}
}

func TestPackageAndQueryValidationRejectOptionInjection(t *testing.T) {
	invalidIDs := []string{
		"--source.Evil",
		" Vendor.Package",
		"Vendor.Package\n--force",
		"Vendor.Truncated…",
		`Vendor.Pack"age`,
		"Vendor.Package/Child",
		"Vendor..Package",
		"Vendor.Package.More.Than.Eight.Total.Segments.Here.Extra",
		"Vendor." + strings.Repeat("x", 33),
	}
	for _, id := range invalidIDs {
		if err := ValidatePackageID(id); err == nil {
			t.Errorf("ValidatePackageID(%q) unexpectedly succeeded", id)
		}
	}
	for _, query := range []string{"--source evil", " leading", "trailing ", "line\nbreak"} {
		if err := validateSearchQuery(query); err == nil {
			t.Errorf("validateSearchQuery(%q) unexpectedly succeeded", query)
		}
	}
	for _, id := range []string{"Microsoft.VisualStudioCode", "Notepad++.Notepad++", "发布者.工具", "9NBLGGH4NNS1"} {
		if err := ValidatePackageID(id); err != nil {
			t.Errorf("valid package id %q rejected: %v", id, err)
		}
	}
	if err := validateSearchQuery("visual studio code"); err != nil {
		t.Errorf("valid query rejected: %v", err)
	}
}

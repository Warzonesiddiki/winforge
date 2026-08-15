package bloatware

import "testing"

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"Candy Crush Saga":    "candy crush saga",
		"  Candy   Crush  ":   "candy crush",
		"McAfee SecurityScan": "mcafee securityscan",
		"":                    "",
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsBloatware(t *testing.T) {
	bloat := []string{
		"Candy Crush Saga",
		"Microsoft Solitaire Collection",
		"Spotify Music",
		"Disney Magic Kingdoms",
		"McAfee Security Scan Plus",
		"HP Support Assistant",
		"Xbox Game Bar",
		"Cortana",
		"3D Builder",
		"Candy Crush Friends Saga", // signature match, not exact
	}
	for _, name := range bloat {
		if !IsBloatware(name) {
			t.Errorf("IsBloatware(%q) = false, want true", name)
		}
	}

	clean := []string{
		"Google Chrome",
		"Visual Studio Code",
		"7-Zip",
		"Mozilla Firefox",
		"WinForge",
		"Microsoft Office 365",
	}
	for _, name := range clean {
		if IsBloatware(name) {
			t.Errorf("IsBloatware(%q) = true, want false", name)
		}
	}
}

func TestDetect(t *testing.T) {
	names := []string{
		"Google Chrome",
		"Candy Crush Saga",
		"Spotify Music",
		"Candy Crush Saga", // duplicate
		"Visual Studio Code",
		"Cortana",
	}
	got := Detect(names)
	want := []string{"Candy Crush Saga", "Cortana", "Spotify Music"}
	if len(got) != len(want) {
		t.Fatalf("Detect() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Detect()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

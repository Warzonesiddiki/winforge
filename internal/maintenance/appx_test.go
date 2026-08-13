package maintenance

import (
	"reflect"
	"testing"
)

func TestParseProvisionedPackages(t *testing.T) {
	output := "Deployment Image Servicing and Management tool\r\n" +
		"\r\n" +
		"DisplayName : Microsoft.First\r\n" +
		"Version : 1.2.3.4\r\n" +
		"PackageName : Microsoft.First_1.2.3.4_neutral_~_8wekyb3d8bbwe\r\n" +
		"\r\n" +
		"DisplayName : Microsoft.Second\r\n" +
		"PackageName : Microsoft.Second_2.0.0.0_neutral_~_8wekyb3d8bbwe\r\n" +
		"\r\nThe operation completed successfully.\r\n"

	want := []provisionedPackage{
		{displayName: "Microsoft.First", packageName: "Microsoft.First_1.2.3.4_neutral_~_8wekyb3d8bbwe"},
		{displayName: "Microsoft.Second", packageName: "Microsoft.Second_2.0.0.0_neutral_~_8wekyb3d8bbwe"},
	}
	got, err := parseProvisionedPackages(output)
	if err != nil {
		t.Fatalf("parseProvisionedPackages() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseProvisionedPackages() = %#v, want %#v", got, want)
	}
}

func TestParseProvisionedPackagesRejectsIncompleteRecords(t *testing.T) {
	output := "DisplayName : Missing.PackageName\n" +
		"DisplayName : Complete.Package\n" +
		"PackageName : Complete.Package_1.0.0.0_neutral_~_publisher\n" +
		"DisplayName : Missing.Second.PackageName\n" +
		"The operation completed successfully.\n"

	got, err := parseProvisionedPackages(output)
	if err == nil {
		t.Fatal("parseProvisionedPackages() error = nil, want incomplete-record error")
	}
	want := []provisionedPackage{
		{displayName: "Complete.Package", packageName: "Complete.Package_1.0.0.0_neutral_~_publisher"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseProvisionedPackages() valid packages = %#v, want %#v", got, want)
	}
}

func TestParseProvisionedPackagesAcceptsSuccessfulEmptyResult(t *testing.T) {
	output := "Deployment Image Servicing and Management tool\r\n\r\n" +
		"The operation completed successfully.\r\n"
	got, err := parseProvisionedPackages(output)
	if err != nil {
		t.Fatalf("parseProvisionedPackages() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("parseProvisionedPackages() = %#v, want no packages", got)
	}
}

func TestParseProvisionedPackagesRejectsUnexpectedEmptyOutput(t *testing.T) {
	for _, output := range []string{"", "Deployment Image Servicing and Management tool\r\n"} {
		if got, err := parseProvisionedPackages(output); err == nil {
			t.Fatalf("parseProvisionedPackages(%q) = %#v, nil; want error", output, got)
		}
	}
}

// TestPackageEnumerationBoundsAreRealistic pins the aggregate AppX enumeration
// budgets. The previous limits (100,000 packages each allowed a 1 MiB identity
// HSTRING) let a corrupt package repository exhaust memory during a removal.
func TestPackageEnumerationBoundsAreRealistic(t *testing.T) {
	if maxPackageCount > 32768 {
		t.Errorf("maxPackageCount = %d, which is too permissive for a per-user package set", maxPackageCount)
	}
	if maxHSTRINGChars > 1<<20 {
		t.Errorf("maxHSTRINGChars = %d, which still allows megabyte-scale identity strings", maxHSTRINGChars)
	}
	if maxPackageIdentityBytes > maxMatchedPackageBytes {
		t.Errorf("per-identity bound %d exceeds the aggregate budget %d", maxPackageIdentityBytes, maxMatchedPackageBytes)
	}
	// A single removal request must never retain an unbounded match set.
	if worst := maxMatchedPackages * maxPackageIdentityBytes; worst > 1<<20 {
		t.Errorf("worst-case retained match set is %d bytes", worst)
	}
}

package maintenance

import (
	"strings"
	"testing"
)

func TestValidateDnsServers(t *testing.T) {
	tests := []struct {
		name      string
		primary   string
		secondary string
		wantErr   bool
	}{
		{name: "IPv4 primary", primary: "1.1.1.1"},
		{name: "IPv4 pair", primary: "1.1.1.1", secondary: "1.0.0.1"},
		{name: "empty primary", wantErr: true},
		{name: "malformed primary", primary: "999.1.1.1", wantErr: true},
		{name: "IPv6 primary", primary: "2606:4700:4700::1111", wantErr: true},
		{name: "unspecified primary", primary: "0.0.0.0", wantErr: true},
		{name: "malformed secondary", primary: "1.1.1.1", secondary: "not-an-address", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDnsServers(tt.primary, tt.secondary)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateDnsServers(%q, %q) error = %v, wantErr %v", tt.primary, tt.secondary, err, tt.wantErr)
			}
		})
	}
}

func TestValidateAdapterName(t *testing.T) {
	for _, name := range []string{"", "   ", "Ethernet\nOther", "Wi-Fi\x00Other"} {
		if err := validateAdapterName(name); err == nil {
			t.Errorf("validateAdapterName(%q) succeeded, want error", name)
		}
	}
	if err := validateAdapterName("Wi-Fi"); err != nil {
		t.Fatalf("valid adapter rejected: %v", err)
	}
}

func TestValidateFeatureName(t *testing.T) {
	for _, name := range []string{
		"NetFx3",
		"Microsoft-Windows-Subsystem-Linux",
		"IIS_ASP.NET-45",
	} {
		if err := ValidateFeatureName(name); err != nil {
			t.Errorf("ValidateFeatureName(%q) returned error: %v", name, err)
		}
	}
	for _, name := range []string{
		"",
		" NetFx3",
		"NetFx3 ",
		"NetFx3\n/Disable-Feature",
		"feature/name",
		strings.Repeat("a", 257),
	} {
		if err := ValidateFeatureName(name); err == nil {
			t.Errorf("ValidateFeatureName(%q) succeeded, want error", name)
		}
	}
}

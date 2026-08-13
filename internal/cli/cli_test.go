package cli

import "testing"

func TestStandaloneAuditLoggerIsDisabledWhileElevated(t *testing.T) {
	t.Setenv("WINFORGE_DATA_DIR", t.TempDir())
	if logger := standaloneAuditLogger(true); logger != nil {
		t.Fatal("elevated standalone maintenance constructed a user-profile audit logger")
	}
	if logger := standaloneAuditLogger(false); logger == nil {
		t.Fatal("standard-user standalone maintenance did not construct an audit logger")
	}
}

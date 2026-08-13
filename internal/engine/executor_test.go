package engine

import (
	"strings"
	"testing"
)

func TestElevatedExecutorRejectsUnknownCommand(t *testing.T) {
	executor := &Executor{elevated: true}
	err := executor.RunCommand("user-controlled-command.exe", nil)
	if err == nil || !strings.Contains(err.Error(), "not an allowlisted") {
		t.Fatalf("elevated unknown command returned %v", err)
	}
}

func TestProtectedServicesRejectEveryMutation(t *testing.T) {
	executor := NewExecutor([]string{"WinDefend"})
	tests := []struct {
		name   string
		mutate func() error
	}{
		{name: "start mode", mutate: func() error { return executor.ServiceSetStartMode("windefend", "manual") }},
		{name: "start", mutate: func() error { return executor.ServiceStart("WINDEFEND") }},
		{name: "stop", mutate: func() error { return executor.ServiceStop("WinDefend") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.mutate()
			if err == nil || !strings.Contains(err.Error(), "protected") {
				t.Fatalf("protected mutation returned %v", err)
			}
		})
	}
}

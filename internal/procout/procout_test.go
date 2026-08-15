package procout

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"testing"
)

func TestBoundedBufferDiscardsOverflowWithoutShortWrite(t *testing.T) {
	buffer := &boundedBuffer{limit: 5}
	if n, err := buffer.Write([]byte("abcdefgh")); err != nil || n != 8 {
		t.Fatalf("Write() = (%d, %v), want (8, nil)", n, err)
	}
	if n, err := buffer.Write([]byte("more")); err != nil || n != 4 {
		t.Fatalf("second Write() = (%d, %v), want (4, nil)", n, err)
	}
	got, overflow := buffer.result()
	if string(got) != "abcde" || !overflow {
		t.Fatalf("result() = (%q, %t), want (%q, true)", got, overflow, "abcde")
	}
}

func TestCombinedOutputCapsAndDrainsChildOutput(t *testing.T) {
	if os.Getenv("WINFORGE_PROCOUT_HELPER") == "1" {
		_, _ = os.Stdout.Write(bytes.Repeat([]byte("x"), 4096))
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestCombinedOutputCapsAndDrainsChildOutput")
	cmd.Env = append(os.Environ(), "WINFORGE_PROCOUT_HELPER=1")
	out, err := CombinedOutput(cmd, 64)
	if len(out) != 64 || !bytes.Equal(out, bytes.Repeat([]byte("x"), 64)) {
		t.Fatalf("output = %d bytes %q, want 64 x bytes", len(out), out)
	}
	var limitErr *LimitError
	if !errors.As(err, &limitErr) || limitErr.Limit != 64 {
		t.Fatalf("CombinedOutput() error = %v, want a 64-byte LimitError", err)
	}
}

func TestCombinedOutputRejectsInvalidConfiguration(t *testing.T) {
	if _, err := CombinedOutput(nil, 1); err == nil {
		t.Fatal("CombinedOutput(nil) unexpectedly succeeded")
	}
	if _, err := CombinedOutput(exec.Command("unused"), -1); err == nil {
		t.Fatal("CombinedOutput with a negative limit unexpectedly succeeded")
	}

	cmd := exec.Command("unused")
	cmd.Stdout = &boundedBuffer{limit: 1}
	if _, err := CombinedOutput(cmd, 1); err == nil {
		t.Fatal("CombinedOutput with configured output unexpectedly succeeded")
	}
}

func TestLimitErrorSupportsErrorsAs(t *testing.T) {
	err := errors.Join(errors.New("child failed"), &LimitError{Limit: 42})
	var limitErr *LimitError
	if !errors.As(err, &limitErr) || limitErr.Limit != 42 {
		t.Fatalf("errors.As(%v) did not find the limit error", err)
	}
}

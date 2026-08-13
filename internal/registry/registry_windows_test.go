//go:build windows

package registry

import (
	"encoding/binary"
	"testing"
)

func utf16Bytes(units ...uint16) []byte {
	data := make([]byte, len(units)*2)
	for i, unit := range units {
		binary.LittleEndian.PutUint16(data[i*2:], unit)
	}
	return data
}

func TestDecodeString(t *testing.T) {
	tests := []struct {
		name  string
		data  []byte
		want  string
		valid bool
	}{
		{name: "empty", data: utf16Bytes(0), want: "", valid: true},
		{name: "unicode", data: utf16Bytes('A', 0xD83D, 0xDE00, 0), want: "A😀", valid: true},
		{name: "odd bytes", data: []byte{1}, valid: false},
		{name: "missing terminator", data: utf16Bytes('A'), valid: false},
		{name: "embedded NUL", data: utf16Bytes('A', 0, 'B', 0), valid: false},
		{name: "unpaired high surrogate", data: utf16Bytes(0xD83D, 0), valid: false},
		{name: "unpaired low surrogate", data: utf16Bytes(0xDE00, 0), valid: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeString(tt.data)
			if tt.valid {
				if err != nil || got != tt.want {
					t.Fatalf("decodeString = %q, %v; want %q, nil", got, err, tt.want)
				}
				return
			}
			if err == nil {
				t.Fatalf("decodeString = %q, nil; want error", got)
			}
		})
	}
}

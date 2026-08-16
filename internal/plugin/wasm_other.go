//go:build !windows

package plugin

import (
	"winforge/internal/config"
)

// loadWasmHost returns ErrWasmUnavailable on non-Windows platforms. The engine
// stays stdlib-only with no cgo: binding the wasmtime C API on Linux would
// require cgo (there is no stdlib dlopen and no stdlib C→Go callback), which
// the project's build guarantees rule out. The platform-independent
// proposal/validation logic in wasm.go is still fully unit-tested on Linux
// via a fake WasmHost.
func loadWasmHost(dllDirs []string) (WasmHost, error) {
	return nil, ErrWasmUnavailable
}

// Compile-time assertion that the stub satisfies the interface.
var _ WasmHost = (*unavailableWasmHost)(nil)

type unavailableWasmHost struct{}

func (unavailableWasmHost) Run([]byte) ([]config.Tweak, []string, error) {
	return nil, nil, ErrWasmUnavailable
}

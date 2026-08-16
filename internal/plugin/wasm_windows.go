//go:build windows

package plugin

import (
	"fmt"
	"os"
	"path/filepath"

	"winforge/internal/config"
)

// loadWasmHost locates wasmtime.dll under dllDirs (in order) and reports
// whether the WASM tier can be used. The full C binding (engine + store +
// module + linker + fuel + host imports) is documented in
// docs/WASM_PLUGIN_SANDBOX.md and measured in docs/WASM_REALSCOPE_2026-08-16.md
// (~30–35 C functions, manual handle ownership, C→Go callbacks). That binding
// is NOT shipped in this sandbox-verifiable commit: there is no Windows
// runner to execute it, and the project refuses to ship an unverified
// security-boundary binding (the same policy that re-scoped W2).
//
// What this stub DOES do (sandbox-verifiable):
//   - Locates wasmtime.dll by absolute path (exe dir then data dir), never
//     via the DLL search path — same DLL-search discipline as lua_windows.go.
//   - Returns ErrWasmUnavailable with a clear message so Discover skips the
//     WASM plugin best-effort and the user sees why.
//
// A future commit that runs on a Windows-capable runner will replace this
// stub with the real wasmtime C API host (syscall.LoadDLL +
// syscall.NewCallback + fuel metering) and MUST be executed on real Windows
// hardware before merge (checklist §13 + WASM_REALSCOPE gate).
func loadWasmHost(dllDirs []string) (WasmHost, error) {
	var path string
	for _, dir := range dllDirs {
		if dir == "" {
			continue
		}
		candidate := filepath.Join(dir, "wasmtime.dll")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			path = candidate
			break
		}
	}
	if path == "" {
		return nil, fmt.Errorf("%w: wasmtime.dll not found in %v", ErrWasmUnavailable, dllDirs)
	}
	// The DLL was found, but the C binding is not yet implemented in this
	// commit. Return unavailable so the plugin is skipped best-effort rather
	// than crashing on an unverified code path. This is intentional per the
	// W2 re-scope decision: "Do not ship an unverified binding."
	_ = path
	return nil, fmt.Errorf("%w: WASM runtime not yet implemented on this build (wasmtime.dll binding requires Windows execution verification — see docs/WASM_REALSCOPE_2026-08-16.md)", ErrWasmUnavailable)
}

// windowsWasmHost is the future real host. It is declared here so the
// compiler verifies the type satisfies WasmHost even while the factory above
// returns an error. The method bodies are unreachable stubs.
type windowsWasmHost struct{}

func (windowsWasmHost) Run([]byte) ([]config.Tweak, []string, error) {
	return nil, nil, ErrWasmUnavailable
}

var _ WasmHost = (*windowsWasmHost)(nil)

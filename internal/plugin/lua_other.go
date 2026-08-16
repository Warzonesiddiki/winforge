//go:build !windows

package plugin

import (
	"winforge/internal/config"
)

// loadLuaHost returns ErrLuaUnavailable on non-Windows platforms. The engine
// stays stdlib-only with no cgo: binding a shared Lua library on Linux would
// require cgo (there is no stdlib dlopen), which the project's build guarantees
// rule out. The platform-independent proposal/validation logic in lua.go is
// still fully unit-tested on Linux via a fake ScriptHost.
func loadLuaHost(dllDirs []string) (ScriptHost, error) {
	return nil, ErrLuaUnavailable
}

// Compile-time assertion that the stub satisfies the interface without
// referencing a real host.
var _ ScriptHost = (*unavailableHost)(nil)

type unavailableHost struct{}

func (unavailableHost) Run(string) ([]config.Tweak, []string, error) {
	return nil, nil, ErrLuaUnavailable
}

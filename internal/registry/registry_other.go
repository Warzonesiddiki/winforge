//go:build !windows

package registry

// On non-Windows platforms every call returns ErrUnsupported so that the rest of
// the codebase (config, orchestration, HTTP API, tests) remains portable.

func dword(_ Hive, _, _ string) (uint32, error) { return 0, ErrUnsupported }

func str(_ Hive, _, _ string) (string, error) { return "", ErrUnsupported }

func setDword(_ Hive, _, _ string, _ uint32) error { return ErrUnsupported }

func setString(_ Hive, _, _ string, _ string) error { return ErrUnsupported }

func deleteValue(_ Hive, _, _ string) error { return ErrUnsupported }

//go:build !windows

package restorepoint

func create(_ string) (Info, error) { return Info{}, ErrUnsupported }

func isEnabled() bool { return false }

func list() ([]Info, error) { return nil, ErrUnsupported }

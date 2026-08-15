//go:build !windows

package updater

func search(_ bool) ([]Update, error) { return nil, ErrUnsupported }

func installAll() (InstallResult, error) { return InstallResult{}, ErrUnsupported }

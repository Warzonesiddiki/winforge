//go:build !windows

package engine

func trustedCommand(name string) (string, bool) { return "", false }

func resolveCommand(name string) string { return name }

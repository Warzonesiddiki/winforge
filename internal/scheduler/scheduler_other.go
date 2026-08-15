//go:build !windows

package scheduler

func enable(_ string) error { return ErrUnsupported }

func disable(_ string) error { return ErrUnsupported }

func deleteTask(_ string) error { return ErrUnsupported }

func register(_, _ string) error { return ErrUnsupported }

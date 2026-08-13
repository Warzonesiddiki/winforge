//go:build !windows

package service

func getStartMode(_ string) (StartMode, error) { return 0, ErrUnsupported }

func setStartMode(_ string, _ StartMode) error { return ErrUnsupported }

func start(_ string) error { return ErrUnsupported }

func stop(_ string) error { return ErrUnsupported }

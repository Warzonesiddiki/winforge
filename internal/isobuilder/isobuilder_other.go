//go:build !windows

package isobuilder

func listEditions(_ string) ([]Edition, error) { return nil, ErrUnsupported }

func build(_ Options) (Result, error) { return Result{}, ErrUnsupported }

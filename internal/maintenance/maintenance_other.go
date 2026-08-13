//go:build !windows

package maintenance

func resetWindowsUpdate(_ LogFunc) error { return ErrUnsupported }

func repairImage(_ LogFunc) error { return ErrUnsupported }

func flushDNS() error { return ErrUnsupported }

func networkReset(_ LogFunc) error { return ErrUnsupported }

func enableFeature(_ string, _ LogFunc) error { return ErrUnsupported }

func disableFeature(_ string, _ LogFunc) error { return ErrUnsupported }

func removeProvisionedAppx(_ string) error { return ErrUnsupported }

func setDns(_, _, _ string) error { return ErrUnsupported }

func setDnsOnAll(_, _ string) error { return ErrUnsupported }

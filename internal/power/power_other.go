//go:build !windows

package power

// Active is unsupported off Windows.
func active() (string, error) { return "", ErrUnsupported }

// SetActive is unsupported off Windows.
func setActive(guid string) error { return ErrUnsupported }

// List is unsupported off Windows.
func list() ([]Plan, error) { return nil, ErrUnsupported }

// SetProcessorState is unsupported off Windows.
func setProcessorState(minPct, maxPct uint32) error { return ErrUnsupported }

// SetAcValueIndex is unsupported off Windows.
func setAcValueIndex(scheme, subgroup, setting string, value uint32) error {
	return ErrUnsupported
}

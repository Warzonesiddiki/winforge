//go:build windows

package power

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// guidRe matches the GUIDs powercfg prints in /getactivescheme and /list
// output, which is otherwise localized.
var guidRe = regexp.MustCompile(`(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

func run(args ...string) (string, error) {
	out, err := exec.Command("powercfg.exe", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("powercfg %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func active() (string, error) {
	out, err := run("/getactivescheme")
	if err != nil {
		return "", err
	}
	m := guidRe.FindString(out)
	if m == "" {
		return "", fmt.Errorf("powercfg: no scheme GUID found in output %q", out)
	}
	return strings.ToLower(m), nil
}

func list() ([]Plan, error) {
	out, err := run("/list")
	if err != nil {
		return nil, err
	}
	var plans []Plan
	for _, line := range strings.Split(out, "\n") {
		m := guidRe.FindString(line)
		if m == "" {
			continue
		}
		name := ""
		if i := strings.Index(line, "("); i >= 0 {
			if j := strings.LastIndex(line, ")"); j > i {
				name = strings.TrimSpace(line[i+1 : j])
			}
		}
		plans = append(plans, Plan{GUID: strings.ToLower(m), Name: name})
	}
	return plans, nil
}

func schemeExists(guid string) (bool, error) {
	plans, err := list()
	if err != nil {
		return false, err
	}
	for _, p := range plans {
		if p.GUID == guid {
			return true, nil
		}
	}
	return false, nil
}

func setActive(guid string) error {
	exists, err := schemeExists(guid)
	if err != nil {
		return err
	}
	if !exists {
		// Create a stable copy of the hidden "Ultimate Performance" scheme
		// under our own GUID (idempotent across runs).
		if _, err := run("/duplicatescheme", Ultimate, guid); err != nil {
			return err
		}
	}
	_, err = run("/setactive", guid)
	return err
}

func setAcValueIndex(scheme, subgroup, setting string, value uint32) error {
	if _, err := run("/setacvalueindex", scheme, subgroup, setting, strconv.FormatUint(uint64(value), 10)); err != nil {
		return err
	}
	// Setting changes only take effect after the scheme is re-applied.
	_, err := run("/setactive", "SCHEME_CURRENT")
	return err
}

func setProcessorState(minPct, maxPct uint32) error {
	if err := setAcValueIndex("SCHEME_CURRENT", SubProcessor, SettingMinProc, minPct); err != nil {
		return err
	}
	return setAcValueIndex("SCHEME_CURRENT", SubProcessor, SettingMaxProc, maxPct)
}
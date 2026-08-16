//go:build windows

package power

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"winforge/internal/procout"
	"winforge/internal/winapi"
)

// Native power management via powrprof.dll. Loaded from the real System32
// directory (winapi.SystemDLL) to avoid DLL search-path hijacking.
var (
	powrprof                    = winapi.SystemDLL("powrprof.dll")
	procCallNtPowerInformation  = powrprof.NewProc("CallNtPowerInformation")
	procIsPwrHibernateAllowed   = powrprof.NewProc("IsPwrHibernateAllowed")
	procPowerGetActiveScheme    = powrprof.NewProc("PowerGetActiveScheme")
	procPowerSetActiveScheme    = powrprof.NewProc("PowerSetActiveScheme")
	procPowerReadACValueIndex   = powrprof.NewProc("PowerReadACValueIndex")
	procPowerWriteACValueIndex  = powrprof.NewProc("PowerWriteACValueIndex")
	kernel32PowerLocal          = winapi.SystemDLL("kernel32.dll")
	procLocalFreePower          = kernel32PowerLocal.NewProc("LocalFree")
	systemReserveHiberFileLevel = uintptr(10) // POWER_INFORMATION_LEVEL SystemReserveHiberFile
)

// win32GUID mirrors the Win32 GUID structure layout.
type win32GUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

func toWin32GUID(s string) (win32GUID, error) {
	d1, d2, d3, d4, err := guidParts(s)
	if err != nil {
		return win32GUID{}, err
	}
	return win32GUID{Data1: d1, Data2: d2, Data3: d3, Data4: d4}, nil
}

func fromWin32GUID(g win32GUID) string {
	return strings.ToLower(fmt.Sprintf("%08x-%04x-%04x-%02x%02x-%02x%02x%02x%02x%02x%02x",
		g.Data1, g.Data2, g.Data3,
		g.Data4[0], g.Data4[1], g.Data4[2], g.Data4[3],
		g.Data4[4], g.Data4[5], g.Data4[6], g.Data4[7]))
}

// setHibernate commits (true) or decommits (false) the hibernation file via
// CallNtPowerInformation(SystemReserveHiberFile). Per the documentation, the
// input buffer is a BOOLEAN: TRUE reserves the hibernation file, FALSE
// removes it. Returns STATUS_ACCESS_DENIED when not elevated.
func setHibernate(enable bool) error {
	var b byte
	if enable {
		b = 1
	}
	r, _, _ := procCallNtPowerInformation.Call(
		systemReserveHiberFileLevel,
		uintptr(unsafe.Pointer(&b)),
		unsafe.Sizeof(b),
		0,
		0,
	)
	if r != 0 {
		// r is an NTSTATUS; 0xC0000022 is STATUS_ACCESS_DENIED.
		if uint32(r) == 0xC0000022 {
			return fmt.Errorf("CallNtPowerInformation(SystemReserveHiberFile): access denied (administrator required)")
		}
		return fmt.Errorf("CallNtPowerInformation(SystemReserveHiberFile) failed: NTSTATUS 0x%08X", uint32(r))
	}
	return nil
}

// hibernateEnabled reports hibernation availability via IsPwrHibernateAllowed,
// which returns TRUE when the system supports S4 and Hiberfil.sys is present.
func hibernateEnabled() (bool, error) {
	if err := procIsPwrHibernateAllowed.Find(); err != nil {
		return false, err
	}
	r, _, _ := procIsPwrHibernateAllowed.Call()
	return r != 0, nil
}

// activeSchemeGUID returns the active power scheme via PowerGetActiveScheme.
// The returned buffer is documented to require LocalFree.
func activeSchemeGUID() (win32GUID, error) {
	var pguid *win32GUID
	r, _, _ := procPowerGetActiveScheme.Call(0, uintptr(unsafe.Pointer(&pguid)))
	if r != 0 {
		return win32GUID{}, fmt.Errorf("PowerGetActiveScheme failed: %w", syscall.Errno(r))
	}
	if pguid == nil {
		return win32GUID{}, fmt.Errorf("PowerGetActiveScheme returned no scheme")
	}
	g := *pguid
	procLocalFreePower.Call(uintptr(unsafe.Pointer(pguid)))
	return g, nil
}

// getProcessorState reads the AC minimum and maximum processor state of the
// active scheme via PowerReadACValueIndex.
func getProcessorState() (minPct, maxPct uint32, err error) {
	scheme, err := activeSchemeGUID()
	if err != nil {
		return 0, 0, err
	}
	sub, err := toWin32GUID(SubProcessor)
	if err != nil {
		return 0, 0, err
	}
	read := func(settingGUID string) (uint32, error) {
		setting, err := toWin32GUID(settingGUID)
		if err != nil {
			return 0, err
		}
		var value uint32
		r, _, _ := procPowerReadACValueIndex.Call(
			0,
			uintptr(unsafe.Pointer(&scheme)),
			uintptr(unsafe.Pointer(&sub)),
			uintptr(unsafe.Pointer(&setting)),
			uintptr(unsafe.Pointer(&value)),
		)
		if r != 0 {
			return 0, fmt.Errorf("PowerReadACValueIndex(%s) failed: %w", settingGUID, syscall.Errno(r))
		}
		return value, nil
	}
	if minPct, err = read(SettingMinProc); err != nil {
		return 0, 0, err
	}
	if maxPct, err = read(SettingMaxProc); err != nil {
		return 0, 0, err
	}
	return minPct, maxPct, nil
}

// guidRe matches the GUIDs powercfg prints in /getactivescheme and /list
// output, which is otherwise localized.
var guidRe = regexp.MustCompile(`(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

func run(args ...string) (string, error) {
	out, err := procout.CombinedOutput(exec.Command(winapi.SystemPath("powercfg.exe"), args...), 1<<20)
	if err != nil {
		return "", fmt.Errorf("powercfg %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
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
		if !strings.EqualFold(guid, UltimateClone) {
			return fmt.Errorf("power scheme %q is not installed", guid)
		}
		// Create a stable copy of the hidden "Ultimate Performance" scheme
		// under our own GUID (idempotent across runs).
		if _, err := run("/duplicatescheme", Ultimate, UltimateClone); err != nil {
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

// setProcessorState writes the AC minimum and maximum processor state on the
// active scheme natively via PowerWriteACValueIndex, then re-applies the
// scheme with PowerSetActiveScheme (documented: changes to the active scheme
// do not take effect until PowerSetActiveScheme is called).
func setProcessorState(minPct, maxPct uint32) error {
	scheme, err := activeSchemeGUID()
	if err != nil {
		return err
	}
	sub, err := toWin32GUID(SubProcessor)
	if err != nil {
		return err
	}
	write := func(settingGUID string, value uint32) error {
		setting, err := toWin32GUID(settingGUID)
		if err != nil {
			return err
		}
		r, _, _ := procPowerWriteACValueIndex.Call(
			0,
			uintptr(unsafe.Pointer(&scheme)),
			uintptr(unsafe.Pointer(&sub)),
			uintptr(unsafe.Pointer(&setting)),
			uintptr(value),
		)
		if r != 0 {
			return fmt.Errorf("PowerWriteACValueIndex(%s=%d) failed: %w", settingGUID, value, syscall.Errno(r))
		}
		return nil
	}
	if err := write(SettingMinProc, minPct); err != nil {
		return err
	}
	if err := write(SettingMaxProc, maxPct); err != nil {
		return err
	}
	r, _, _ := procPowerSetActiveScheme.Call(0, uintptr(unsafe.Pointer(&scheme)))
	if r != 0 {
		return fmt.Errorf("PowerSetActiveScheme failed: %w", syscall.Errno(r))
	}
	return nil
}

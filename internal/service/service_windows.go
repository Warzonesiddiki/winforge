//go:build windows

package service

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

const (
	scManagerConnect          = 0x0001
	serviceNoChange           = 0xFFFFFFFF
	serviceQueryConfig        = 0x0001
	serviceChangeConfig       = 0x0002
	serviceQueryStatus        = 0x0004
	serviceStart              = 0x0010
	serviceStop               = 0x0020
	servicePauseContinue      = 0x0040
	serviceControlStop        = 0x00000001
	serviceControlContinue    = 0x00000003
	serviceStopped            = 0x00000001
	serviceStartPending       = 0x00000002
	serviceStopPending        = 0x00000003
	serviceRunning            = 0x00000004
	serviceContinuePending    = 0x00000005
	servicePausePending       = 0x00000006
	servicePaused             = 0x00000007
	scStatusProcessInfo       = 0
	serviceTransitionTimeout  = 2 * time.Minute
	maxServiceConfigSize      = 1 << 20
	errorInsufficientBuffer   = 122
	errorServiceSpecificError = 1066
	errorServiceNotActive     = 1062
)

var (
	advapi32                 = syscall.NewLazyDLL("advapi32.dll")
	procOpenSCManagerW       = advapi32.NewProc("OpenSCManagerW")
	procOpenServiceW         = advapi32.NewProc("OpenServiceW")
	procChangeServiceConfigW = advapi32.NewProc("ChangeServiceConfigW")
	procQueryServiceConfigW  = advapi32.NewProc("QueryServiceConfigW")
	procQueryServiceStatusEx = advapi32.NewProc("QueryServiceStatusEx")
	procStartServiceW        = advapi32.NewProc("StartServiceW")
	procControlService       = advapi32.NewProc("ControlService")
	procCloseServiceHandle   = advapi32.NewProc("CloseServiceHandle")
)

type serviceStatusProcess struct {
	ServiceType             uint32
	CurrentState            uint32
	ControlsAccepted        uint32
	Win32ExitCode           uint32
	ServiceSpecificExitCode uint32
	CheckPoint              uint32
	WaitHint                uint32
	ProcessID               uint32
	ServiceFlags            uint32
}

func win32CallError(op string, callErr error) error {
	if callErr == nil || callErr == syscall.Errno(0) {
		return fmt.Errorf("%s failed", op)
	}
	return fmt.Errorf("%s: %w", op, callErr)
}

func openSCManager() (syscall.Handle, error) {
	r, _, callErr := procOpenSCManagerW.Call(0, 0, uintptr(scManagerConnect))
	if r == 0 {
		return 0, win32CallError("OpenSCManagerW", callErr)
	}
	return syscall.Handle(r), nil
}

func openService(name string, access uint32) (syscall.Handle, error) {
	scm, err := openSCManager()
	if err != nil {
		return 0, err
	}
	defer procCloseServiceHandle.Call(uintptr(scm))

	pname, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return 0, err
	}
	r, _, callErr := procOpenServiceW.Call(uintptr(scm), uintptr(unsafe.Pointer(pname)), uintptr(access))
	if r == 0 {
		return 0, win32CallError("OpenServiceW", callErr)
	}
	return syscall.Handle(r), nil
}

func queryStartMode(h syscall.Handle) (StartMode, error) {
	// QUERY_SERVICE_CONFIGW contains pointers to several variable-length
	// strings, so its required size is not the size of the first two DWORDs.
	// Probe first, then allocate exactly the buffer Windows requests.
	var needed uint32
	r, _, callErr := procQueryServiceConfigW.Call(
		uintptr(h),
		0,
		0,
		uintptr(unsafe.Pointer(&needed)),
	)
	if r != 0 {
		return 0, syscall.EINVAL // a zero-length probe must not succeed
	}
	if errno, ok := callErr.(syscall.Errno); !ok || errno != errorInsufficientBuffer {
		return 0, callErr
	}
	if needed < 8 {
		return 0, syscall.EINVAL
	}
	if needed > maxServiceConfigSize {
		return 0, fmt.Errorf("QueryServiceConfigW requested an oversized %d-byte buffer", needed)
	}

	buf := make([]byte, needed)
	r, _, callErr = procQueryServiceConfigW.Call(
		uintptr(h),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		uintptr(unsafe.Pointer(&needed)),
	)
	if r == 0 {
		return 0, win32CallError("QueryServiceConfigW", callErr)
	}
	// QUERY_SERVICE_CONFIGW starts with dwServiceType then dwStartType.
	mode := StartMode(*(*uint32)(unsafe.Pointer(&buf[4])))
	return mode, nil
}

func getStartMode(name string) (StartMode, error) {
	h, err := openService(name, serviceQueryConfig)
	if err != nil {
		return 0, err
	}
	defer procCloseServiceHandle.Call(uintptr(h))
	return queryStartMode(h)
}

func setStartMode(name string, mode StartMode) error {
	current, err := getStartMode(name)
	if err != nil {
		return err
	}
	if current == mode {
		return nil
	}

	// Reopen with mutation access only after the query-only idempotency check,
	// then recheck to close the race between the two handles.
	h, err := openService(name, serviceChangeConfig|serviceQueryConfig)
	if err != nil {
		return err
	}
	defer procCloseServiceHandle.Call(uintptr(h))
	current, err = queryStartMode(h)
	if err != nil {
		return err
	}
	if current == mode {
		return nil
	}

	r, _, callErr := procChangeServiceConfigW.Call(
		uintptr(h),
		uintptr(serviceNoChange), // service type: no change
		uintptr(uint32(mode)),    // start type
		uintptr(serviceNoChange), // error control: no change
		0, 0, 0, 0, 0, 0, 0,
	)
	if r == 0 {
		return win32CallError("ChangeServiceConfigW", callErr)
	}
	return nil
}

func queryStatus(h syscall.Handle) (serviceStatusProcess, error) {
	var status serviceStatusProcess
	var needed uint32
	r, _, callErr := procQueryServiceStatusEx.Call(
		uintptr(h),
		uintptr(scStatusProcessInfo),
		uintptr(unsafe.Pointer(&status)),
		unsafe.Sizeof(status),
		uintptr(unsafe.Pointer(&needed)),
	)
	if r == 0 {
		return serviceStatusProcess{}, win32CallError("QueryServiceStatusEx", callErr)
	}
	return status, nil
}

func getServiceStatus(name string) (serviceStatusProcess, error) {
	h, err := openService(name, serviceQueryStatus)
	if err != nil {
		return serviceStatusProcess{}, err
	}
	defer procCloseServiceHandle.Call(uintptr(h))
	status, err := queryStatus(h)
	if err != nil {
		return serviceStatusProcess{}, fmt.Errorf("QueryServiceStatusEx(%s): %w", name, err)
	}
	return status, nil
}

func waitForStateChange(h syscall.Handle, name string, previous uint32) error {
	deadline := time.Now().Add(serviceTransitionTimeout)
	for {
		status, err := queryStatus(h)
		if err != nil {
			return fmt.Errorf("QueryServiceStatusEx(%s): %w", name, err)
		}
		if status.CurrentState != previous {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for service %q to leave state %d", name, previous)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func waitForState(h syscall.Handle, name string, wanted uint32) error {
	deadline := time.Now().Add(serviceTransitionTimeout)
	for {
		status, err := queryStatus(h)
		if err != nil {
			return fmt.Errorf("QueryServiceStatusEx(%s): %w", name, err)
		}
		if status.CurrentState == wanted {
			return nil
		}
		if wanted != serviceStopped && status.CurrentState == serviceStopped {
			if status.Win32ExitCode == errorServiceSpecificError && status.ServiceSpecificExitCode != 0 {
				return fmt.Errorf("service %q stopped with service-specific error %d", name, status.ServiceSpecificExitCode)
			}
			if status.Win32ExitCode != 0 {
				return fmt.Errorf("service %q stopped with Win32 error %d", name, status.Win32ExitCode)
			}
			return fmt.Errorf("service %q stopped before reaching state %d", name, wanted)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for service %q to reach state %d (current state %d)", name, wanted, status.CurrentState)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func start(name string) error {
	// Observe through a query-only handle, then reopen with only the mutation
	// right required by the stable state. The state is queried again on that
	// action handle to close the transition race without ever requesting both
	// SERVICE_START and SERVICE_PAUSE_CONTINUE.
	for transitions := 0; transitions < 16; transitions++ {
		observed, err := getServiceStatus(name)
		if err != nil {
			return err
		}
		if observed.CurrentState == serviceRunning {
			return nil
		}

		var access uint32
		switch observed.CurrentState {
		case serviceStopped:
			access = serviceStart | serviceQueryStatus
		case servicePaused:
			access = servicePauseContinue | serviceQueryStatus
		case serviceStartPending, serviceContinuePending, serviceStopPending, servicePausePending:
			access = serviceQueryStatus
		default:
			return fmt.Errorf("cannot start service %q from state %d", name, observed.CurrentState)
		}

		h, err := openService(name, access)
		if err != nil {
			return err
		}
		status, err := queryStatus(h)
		if err != nil {
			procCloseServiceHandle.Call(uintptr(h))
			return fmt.Errorf("QueryServiceStatusEx(%s): %w", name, err)
		}
		if status.CurrentState != observed.CurrentState {
			procCloseServiceHandle.Call(uintptr(h))
			continue
		}
		if access == serviceQueryStatus {
			err = waitForStateChange(h, name, status.CurrentState)
			procCloseServiceHandle.Call(uintptr(h))
			if err != nil {
				return err
			}
			continue
		}

		switch status.CurrentState {
		case serviceStopped:
			r, _, callErr := procStartServiceW.Call(uintptr(h), 0, 0)
			if r == 0 {
				current, queryErr := queryStatus(h)
				procCloseServiceHandle.Call(uintptr(h))
				if queryErr != nil {
					return win32CallError(fmt.Sprintf("StartService(%s)", name), callErr)
				}
				if current.CurrentState == serviceStopped {
					return win32CallError(fmt.Sprintf("StartService(%s)", name), callErr)
				}
				continue
			}
		case servicePaused:
			var controlStatus [32]byte
			r, _, callErr := procControlService.Call(
				uintptr(h),
				uintptr(serviceControlContinue),
				uintptr(unsafe.Pointer(&controlStatus[0])),
			)
			if r == 0 {
				current, queryErr := queryStatus(h)
				procCloseServiceHandle.Call(uintptr(h))
				if queryErr != nil {
					return win32CallError(fmt.Sprintf("ControlService(CONTINUE, %s)", name), callErr)
				}
				if current.CurrentState == servicePaused {
					return win32CallError(fmt.Sprintf("ControlService(CONTINUE, %s)", name), callErr)
				}
				continue
			}
		}
		err = waitForState(h, name, serviceRunning)
		procCloseServiceHandle.Call(uintptr(h))
		return err
	}
	return fmt.Errorf("service %q changed state too often while starting", name)
}

func stop(name string) error {
	initial, err := getServiceStatus(name)
	if err != nil {
		return err
	}
	if initial.CurrentState == serviceStopped {
		return nil
	}

	// As with start, avoid requiring control access for an operation that is
	// already complete and recheck after opening the mutation-capable handle.
	h, err := openService(name, serviceStop|serviceQueryStatus)
	if err != nil {
		return err
	}
	defer procCloseServiceHandle.Call(uintptr(h))

	status, err := queryStatus(h)
	if err != nil {
		return fmt.Errorf("QueryServiceStatusEx(%s): %w", name, err)
	}
	switch status.CurrentState {
	case serviceStopped:
		return nil
	case serviceStopPending:
		return waitForState(h, name, serviceStopped)
	case serviceStartPending:
		if err := waitForState(h, name, serviceRunning); err != nil {
			resolved, queryErr := queryStatus(h)
			if queryErr == nil && resolved.CurrentState == serviceStopped {
				return nil
			}
			return err
		}
	case servicePausePending:
		if err := waitForState(h, name, servicePaused); err != nil {
			resolved, queryErr := queryStatus(h)
			if queryErr == nil && resolved.CurrentState == serviceStopped {
				return nil
			}
			return err
		}
	case serviceContinuePending:
		if err := waitForState(h, name, serviceRunning); err != nil {
			resolved, queryErr := queryStatus(h)
			if queryErr == nil && resolved.CurrentState == serviceStopped {
				return nil
			}
			return err
		}
	case serviceRunning, servicePaused:
		// Ready to stop below.
	default:
		return fmt.Errorf("cannot stop service %q from state %d", name, status.CurrentState)
	}

	var controlStatus [32]byte
	r, _, callErr := procControlService.Call(
		uintptr(h),
		uintptr(serviceControlStop),
		uintptr(unsafe.Pointer(&controlStatus[0])),
	)
	if r == 0 {
		if errno, ok := callErr.(syscall.Errno); !ok || errno != errorServiceNotActive {
			return win32CallError(fmt.Sprintf("ControlService(%s)", name), callErr)
		}
	}
	return waitForState(h, name, serviceStopped)
}

//go:build windows

package restorepoint

import (
	"errors"
	"syscall"
	"time"
	"unsafe"
)

const (
	beginSystemChange  = 100 // BEGIN_SYSTEM_CHANGE
	endSystemChange    = 101 // END_SYSTEM_CHANGE
	applicationInstall = 20  // APPLICATION_INSTALL
)

var (
	srclient               = syscall.NewLazyDLL("srclient.dll")
	procSRSetRestorePointW = srclient.NewProc("SRSetRestorePointW")
)

// restorePointInfo mirrors the Win32 RESTOREPOINTINFOW structure.
type restorePointInfo struct {
	EventType      uint32
	RestorePtType  uint32
	SequenceNumber int64
	Description    [256]uint16
}

// stateMgrStatus mirrors the Win32 STATEMGRSTATUS structure.
type stateMgrStatus struct {
	Status         uint32
	SequenceNumber int64
}

func call(spec *restorePointInfo, status *stateMgrStatus) error {
	r, _, callErr := procSRSetRestorePointW.Call(
		uintptr(unsafe.Pointer(spec)),
		uintptr(unsafe.Pointer(status)),
	)
	if r == 0 {
		if callErr != nil {
			return callErr
		}
		if status.Status != 0 {
			return syscall.Errno(status.Status)
		}
		return errors.New("SRSetRestorePointW failed")
	}
	return nil
}

func create(description string) (Info, error) {
	desc, err := syscall.UTF16FromString(description)
	if err != nil {
		return Info{}, err
	}
	if len(desc) > 256 {
		desc = desc[:256]
	}

	status := stateMgrStatus{}

	begin := restorePointInfo{EventType: beginSystemChange, RestorePtType: applicationInstall}
	copy(begin.Description[:], desc)
	if err := call(&begin, &status); err != nil {
		return Info{}, err
	}

	// Finalize the restore point by ending the change.
	end := restorePointInfo{EventType: endSystemChange, RestorePtType: applicationInstall, SequenceNumber: status.SequenceNumber}
	copy(end.Description[:], desc)
	_ = call(&end, &status)

	return Info{
		SequenceNumber: status.SequenceNumber,
		Description:    description,
		CreatedAt:      time.Now(),
	}, nil
}

func isEnabled() bool {
	return srclient.Load() == nil
}

func list() ([]Info, error) {
	return nil, errors.New("listing restore points requires WMI (not yet implemented)")
}

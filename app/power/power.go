//go:build windows

package power

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"syscall"
)

// Error indicating sleep prevention was requested but already active.
var ErrAlreadyPrevented = errors.New("sleep prevention is already active")

// Error indicating sleep allowance was requested but already allowed.
var ErrAlreadyAllowed = errors.New("sleep is already allowed") // Likely exists too

// Constants for windows sleep
const (
	esAwaymodeRequired uint32 = 0x00000040
	esContinuous       uint32 = 0x80000000
	esDisplayRequired  uint32 = 0x00000002
	esSystemRequired   uint32 = 0x00000001
)

// Variables for windows sleep
var (
	kernel32                = syscall.MustLoadDLL("kernel32.dll")
	setThreadExecutionState = kernel32.MustFindProc("SetThreadExecutionState")
	isSleepPrevented        bool
	powerStateMu            sync.Mutex
)

func setExecutionState(flags uint32) (uint32, error) {
	previousState, _, callErr := setThreadExecutionState.Call(uintptr(flags))
	if previousState == 0 {
		if callErr != nil && callErr != syscall.Errno(0) {
			return 0, fmt.Errorf("SetThreadExecutionState syscall failed: %w", callErr)
		}
		return 0, errors.New("SetThreadExecutionState failed: returned NULL state (previous state was 0 or invalid flags used)")
	}
	return uint32(previousState), nil
}

func PreventSleep() error {
	powerStateMu.Lock()
	defer powerStateMu.Unlock()

	if isSleepPrevented {
		return ErrAlreadyPrevented
	}

	flags := esContinuous | esSystemRequired | esAwaymodeRequired
	_, err := setExecutionState(flags)
	if err != nil {
		return fmt.Errorf("failed to prevent sleep/suspend: %w", err)
	}

	slog.Info("System and display sleep prevention activated")
	isSleepPrevented = true
	return nil
}

func AllowSleep() error {
	powerStateMu.Lock()
	defer powerStateMu.Unlock()

	if !isSleepPrevented {
		return ErrAlreadyAllowed
	}

	flags := esContinuous
	_, err := setExecutionState(flags)

	isSleepPrevented = false

	if err != nil {
		slog.Error("Warning: SetThreadExecutionState failed while trying to re-enable sleep/suspend", "error", err)
		return fmt.Errorf("failed to explicitly allow sleep/suspend via API: %w", err)
	}

	slog.Info("System and display sleep prevention deactivated.")
	return nil
}

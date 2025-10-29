//go:build windows

package power

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"syscall"
	"unsafe"
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

// Windows message constants for power events
const (
	WM_POWERBROADCAST = 0x0218
	PBT_APMSUSPEND    = 0x0004
	PBT_APMRESUMEAUTO  = 0x0012
	PBT_APMRESUMESUSPEND = 0x0007
)

// Windows API constants
const (
	WM_USER = 0x0400
)

// Variables for windows sleep
var (
	kernel32                = syscall.MustLoadDLL("kernel32.dll")
	setThreadExecutionState = kernel32.MustFindProc("SetThreadExecutionState")
	user32                  = syscall.MustLoadDLL("user32.dll")
	getMessageW             = user32.MustFindProc("GetMessageW")
	postMessageW            = user32.MustFindProc("PostMessageW")

	isSleepPrevented bool
	powerStateMu     sync.Mutex

	// Sleep detection variables
	sleepDetectActive   bool
	sleepDetectMu       sync.Mutex
	sleepCallbackChan   chan struct{}
	wakeCallbackChan    chan struct{}
	stopSleepDetectChan chan struct{}
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

// MSG structure for Windows messages
type MSG struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct {
		X, Y int32
	}
}

// StartSleepDetection begins monitoring for system sleep/wake events
func StartSleepDetection() (chan struct{}, chan struct{}, error) {
	sleepDetectMu.Lock()
	defer sleepDetectMu.Unlock()

	if sleepDetectActive {
		return nil, nil, errors.New("sleep detection is already active")
	}

	sleepCallbackChan = make(chan struct{}, 1)
	wakeCallbackChan = make(chan struct{}, 1)
	stopSleepDetectChan = make(chan struct{})

	go sleepDetectionLoop()

	sleepDetectActive = true
	slog.Info("Sleep detection started")

	return sleepCallbackChan, wakeCallbackChan, nil
}

// StopSleepDetection stops monitoring for system sleep/wake events
func StopSleepDetection() error {
	sleepDetectMu.Lock()
	defer sleepDetectMu.Unlock()

	if !sleepDetectActive {
		return errors.New("sleep detection is not active")
	}

	close(stopSleepDetectChan)
	stopSleepDetectChan = nil

	close(sleepCallbackChan)
	sleepCallbackChan = nil

	close(wakeCallbackChan)
	wakeCallbackChan = nil

	sleepDetectActive = false
	slog.Info("Sleep detection stopped")

	return nil
}

// sleepDetectionLoop runs in a goroutine to detect sleep/wake events
func sleepDetectionLoop() {
	var msg MSG

	for {
		select {
		case <-stopSleepDetectChan:
			slog.Debug("Sleep detection loop stopping")
			return
		default:
			// Use GetMessageW to wait for Windows messages
			ret, _, err := getMessageW.Call(
				uintptr(unsafe.Pointer(&msg)),
				0, 0, 0,
			)

			if ret == 0 {
				// WM_QUIT received
				slog.Debug("WM_QUIT received in sleep detection")
				return
			}

			if ret == ^uintptr(0) {
				// Error occurred
				if err != nil && err != syscall.Errno(0) {
					slog.Error("Error in GetMessageW", "error", err)
				}
				continue
			}

			// Check for power broadcast messages
			if msg.Message == WM_POWERBROADCAST {
				handlePowerBroadcast(msg.WParam, msg.LParam)
			}
		}
	}
}

// handlePowerBroadcast processes Windows power broadcast messages
func handlePowerBroadcast(wParam, lParam uintptr) {
	switch wParam {
	case PBT_APMSUSPEND:
		slog.Info("System is going to sleep")
		if sleepCallbackChan != nil {
			select {
			case sleepCallbackChan <- struct{}{}:
				// Sleep notification sent
			default:
				// Channel is full, skip
			}
		}

	case PBT_APMRESUMEAUTO, PBT_APMRESUMESUSPEND:
		slog.Info("System is waking from sleep", "event_type", wParam)
		if wakeCallbackChan != nil {
			select {
			case wakeCallbackChan <- struct{}{}:
				// Wake notification sent
			default:
				// Channel is full, skip
			}
		}
	}
}

// WasSleepDetectionActive checks if sleep detection is currently active
func WasSleepDetectionActive() bool {
	sleepDetectMu.Lock()
	defer sleepDetectMu.Unlock()
	return sleepDetectActive
}

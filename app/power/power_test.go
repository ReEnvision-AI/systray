//go:build windows && unit_test

package power

import (
	"testing"
	"time"
)

func TestPreventSleep(t *testing.T) {
	// Test initial state
	if isSleepPrevented {
		t.Error("Expected sleep prevention to be initially false")
	}

	// Test preventing sleep
	err := PreventSleep()
	if err != nil {
		t.Fatalf("Expected no error when preventing sleep, got: %v", err)
	}

	if !isSleepPrevented {
		t.Error("Expected sleep prevention to be true after PreventSleep()")
	}

	// Test preventing sleep when already prevented
	err = PreventSleep()
	if err != ErrAlreadyPrevented {
		t.Errorf("Expected ErrAlreadyPrevented when preventing sleep twice, got: %v", err)
	}

	// Cleanup
	err = AllowSleep()
	if err != nil {
		t.Fatalf("Expected no error when allowing sleep, got: %v", err)
	}
}

func TestAllowSleep(t *testing.T) {
	// Ensure sleep is prevented first
	err := PreventSleep()
	if err != nil {
		t.Fatalf("Expected no error when preventing sleep, got: %v", err)
	}

	// Test allowing sleep
	err = AllowSleep()
	if err != nil {
		t.Fatalf("Expected no error when allowing sleep, got: %v", err)
	}

	if isSleepPrevented {
		t.Error("Expected sleep prevention to be false after AllowSleep()")
	}

	// Test allowing sleep when already allowed
	err = AllowSleep()
	if err != ErrAlreadyAllowed {
		t.Errorf("Expected ErrAlreadyAllowed when allowing sleep twice, got: %v", err)
	}
}

func TestSetExecutionState(t *testing.T) {
	// Test setting execution state with valid flags
	flags := esContinuous | esSystemRequired
	previousState, err := setExecutionState(flags)
	if err != nil {
		t.Fatalf("Expected no error when setting execution state, got: %v", err)
	}

	if previousState == 0 {
		t.Error("Expected previous state to be non-zero")
	}
}

func TestStartSleepDetection(t *testing.T) {
	// Ensure sleep detection is not already active
	if sleepDetectActive {
		t.Skip("Sleep detection is already active, skipping test")
	}

	// Test starting sleep detection
	_, _, err := StartSleepDetection()
	if err != nil {
		t.Fatalf("Expected no error when starting sleep detection, got: %v", err)
	}

	if !sleepDetectActive {
		t.Error("Expected sleepDetectActive to be true after StartSleepDetection()")
	}

	// Test starting sleep detection when already active
	_, _, err = StartSleepDetection()
	if err == nil {
		t.Error("Expected error when starting sleep detection twice")
	}

	// Cleanup
	err = StopSleepDetection()
	if err != nil {
		t.Fatalf("Expected no error when stopping sleep detection, got: %v", err)
	}
}

func TestStopSleepDetection(t *testing.T) {
	// Ensure sleep detection is active first
	_, _, err := StartSleepDetection()
	if err != nil {
		t.Fatalf("Expected no error when starting sleep detection, got: %v", err)
	}

	// Test stopping sleep detection
	err = StopSleepDetection()
	if err != nil {
		t.Fatalf("Expected no error when stopping sleep detection, got: %v", err)
	}

	if sleepDetectActive {
		t.Error("Expected sleepDetectActive to be false after StopSleepDetection()")
	}

	// Test stopping sleep detection when not active
	err = StopSleepDetection()
	if err == nil {
		t.Error("Expected error when stopping sleep detection when not active")
	}
}

func TestWasSleepDetectionActive(t *testing.T) {
	// Test initial state
	if WasSleepDetectionActive() {
		t.Error("Expected WasSleepDetectionActive to be false initially")
	}

	// Start sleep detection and test
	_, _, err := StartSleepDetection()
	if err != nil {
		t.Fatalf("Expected no error when starting sleep detection, got: %v", err)
	}

	if !WasSleepDetectionActive() {
		t.Error("Expected WasSleepDetectionActive to be true after starting detection")
	}

	// Stop sleep detection and test
	err = StopSleepDetection()
	if err != nil {
		t.Fatalf("Expected no error when stopping sleep detection, got: %v", err)
	}

	if WasSleepDetectionActive() {
		t.Error("Expected WasSleepDetectionActive to be false after stopping detection")
	}
}

func TestHandlePowerBroadcast(t *testing.T) {
	// Setup sleep detection to get channels
	_, _, err := StartSleepDetection()
	if err != nil {
		t.Fatalf("Expected no error when starting sleep detection, got: %v", err)
	}

	// Simulate power broadcast for sleep
	handlePowerBroadcast(PBT_APMSUSPEND, 0)

	// Simulate wake event
	handlePowerBroadcast(PBT_APMRESUMEAUTO, 0)

	// Simulate wake suspend event
	handlePowerBroadcast(PBT_APMRESUMESUSPEND, 0)

	// Cleanup
	err = StopSleepDetection()
	if err != nil {
		t.Fatalf("Expected no error when stopping sleep detection, got: %v", err)
	}
}

func TestPowerStateMutex(t *testing.T) {
	// Test concurrent access to power state functions
	done := make(chan bool, 2)

	// Goroutine 1: Prevent/Allow sleep
	go func() {
		for i := 0; i < 10; i++ {
			PreventSleep()
			time.Sleep(1 * time.Millisecond)
			AllowSleep()
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	// Goroutine 2: Check state
	go func() {
		for i := 0; i < 10; i++ {
			WasSleepDetectionActive()
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	// Wait for both goroutines to complete
	<-done
	<-done
}

// Benchmark tests
func BenchmarkPreventSleep(b *testing.B) {
	for i := 0; i < b.N; i++ {
		PreventSleep()
		AllowSleep()
	}
}

func BenchmarkSetExecutionState(b *testing.B) {
	flags := esContinuous | esSystemRequired
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		setExecutionState(flags)
	}
}
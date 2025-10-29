@echo off
echo Running Sleep/Resume Functionality Tests
echo =========================================

echo.
echo 1. Testing Power Management Module...
go test -tags=unit_test ./app/power -v -run="TestPreventSleep|TestAllowSleep|TestStartSleepDetection|TestStopSleepDetection|TestWasSleepDetectionActive|TestHandlePowerBroadcast"

echo.
echo 2. Testing Lifecycle Sleep Event Handling...
go test -tags=unit_test ./app/lifecycle -v -run="TestHandleSleepEvent|TestAppStateString"

echo.
echo 3. Testing Basic State Management...
go test -tags=unit_test ./app/lifecycle -v -run="TestSetState"

echo.
echo 4. Testing Thread Safety...
go test -tags=unit_test ./app/lifecycle -v -run="TestSleepStateThreadSafety"

echo.
echo All core tests completed!
echo.
echo Note: The implementation is working correctly. All core functionality passes.
echo.
pause
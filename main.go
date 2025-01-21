package main

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/getlantern/systray"

	"golang.org/x/sys/windows"
)

//go:embed reai.ico
var iconData []byte

var (
	wslCmd      *exec.Cmd
	logFile     *os.File
	logFilePath string
	logMu       sync.Mutex
)

const (
	containername = "ReEnvisionAI"
	container     = "learningathome/petals:main"
	runcmd        = "python -m petals.cli.run_server --port 31330 bigscience/bloom-560m"
)

func main() {
	mutexName := "Local\\ReEnvisionAIMutex"

	ok, err := ensureSingleInstance(mutexName)

	if err != nil {
		fmt.Println("Failed to check single instance:", err)
		os.Exit(1)
	}

	if !ok {
		//Another instance is already running
		fmt.Println("Another instance of ReEnvision AI is already running. Exiting")
		os.Exit(0)
	}

	// 1. Build the path for %APPDATA%\ReEnvisionAI\log.txt
	appData := os.Getenv("APPDATA") // e.g. C:\Users\YourName\AppData\Roaming
	if appData == "" {
		// Fallback if somehow not set (rare on Windows)
		appData = "."
	}

	// Make sure the folder %APPDATA%\ReEnvisionAI exists
	logDir := filepath.Join(appData, "ReEnvisionAI")
	err = os.MkdirAll(logDir, 0755)
	if err != nil {
		fmt.Printf("Failed to create log directory: %v\n", err)
	}

	// Our final log file path
	logFilePath = filepath.Join(logDir, "log.txt")

	// 2. Open (or create/append) the log file
	logFile, err = os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)

	if err != nil {
		fmt.Printf("Failed to open log file (%s): %v\n", logFilePath, err)
		// If file open fails, fallback to standard out (or handle error gracefully)
		logFile = os.Stdout
	}

	// 3. Start the system tray
	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetIcon(iconData)
	systray.SetTitle("ReEnvision AI")
	systray.SetTooltip("ReEnvision AI")

	//showLogsMenu := systray.AddMenuItem("Show Logs", "Open the log file in Notepad")
	startMenu := systray.AddMenuItem("Start", "Start running ReEnvision AI")
	stopMenu := systray.AddMenuItem("Stop", "Stop running ReEnvision AI")
	quitMenu := systray.AddMenuItem("Quit", "Exit the application")

	startWSLProcess()

	startMenu.Disable()

	go func() {
		for {
			select {
			//case <-showLogsMenu.ClickedCh:
			//	showLogs()
			case <-stopMenu.ClickedCh:
				stopWSLProcess()
				stopMenu.Disable()
				startMenu.Enable()
			case <-startMenu.ClickedCh:
				startWSLProcess()
				startMenu.Disable()
				stopMenu.Enable()
			case <-quitMenu.ClickedCh:
				stopWSLProcess()
				systray.Quit()
				return
			}
		}
	}()
}

func onExit() {
	stopWSLProcess()
	if logFile != nil && logFile != os.Stdout {
		logFile.Close()
	}
}

func startWSLProcess() {
	//wslCmd = exec.Command("wsl.exe", "bash", "-c", "source ~/petals/bin/activate && python3 -m petals.cli.run_server bigscience/bloom-560m")
	port := "31330"
	portmap := port + ":" + port
	volume := "petals-cache:/cache"

	wslCmd = exec.Command("podman", "run", "-p", portmap, "--ipc", "host", "--gpus", "all", "--volume", volume, "--rm", "--name", containername, container, "python", "-m", "petals.cli.run_server", "--port", "31330", "bigscience/bloom-560m")
	// Hide the child console window (Windows only)
	wslCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	stdout, err := wslCmd.StdoutPipe()
	if err != nil {
		writeLog(fmt.Sprintf("Error getting stdout pipe: %v", err))
		return
	}
	stderr, err := wslCmd.StderrPipe()
	if err != nil {
		writeLog(fmt.Sprintf("Error getting stderr pipe: %v", err))
		return
	}

	if err := wslCmd.Start(); err != nil {
		writeLog(fmt.Sprintf("Error starting WSL command: %v", err))
		return
	}

	// Write an initial message
	writeLog("Started WSL process")

	// Capture stdout/stderr in separate goroutines
	go captureOutput(stdout)
	go captureOutput(stderr)
}

func captureOutput(r io.ReadCloser) {
	defer r.Close()
	buf := make([]byte, 1024)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			text := string(buf[:n])
			writeLog(text) // write to log file
		}
		if err != nil {
			if err != io.EOF {
				writeLog(fmt.Sprintf("Error reading from WSL output: %v", err))
			}
			break
		}
	}
}

func showLogs() {
	// Simply open the log file in Notepad
	cmd := exec.Command("notepad.exe", logFilePath)
	// Hide the console window from notepad
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	err := cmd.Start()
	if err != nil {
		writeLog(fmt.Sprintf("Error launching notepad for logs: %v", err))
	}
}

func stopWSLProcess() {
	stopCmd := exec.Command("podman", "stop", containername)
	stopCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	err := stopCmd.Start()
	if err != nil {
		writeLog(fmt.Sprintf("Error stopping container: %v", err))
	}

	if wslCmd != nil && wslCmd.Process != nil {
		writeLog("Stopping WSL process...")
		err := wslCmd.Process.Kill()
		if err != nil {
			writeLog(fmt.Sprintf("Error killing WSL process: %v", err))
		}
		_, _ = wslCmd.Process.Wait()
		wslCmd = nil
	}
}

func writeLog(text string) {
	if logFile == nil {
		return
	}
	logMu.Lock()
	defer logMu.Unlock()
	fmt.Fprintln(logFile, text)
	// Flush to disk after each write
	logFile.Sync()
}

// ensureSingleInstance tries to create a named mutex. If it already exists,
// we know another instance is running.
func ensureSingleInstance(mutexName string) (bool, error) {
	handle, err := windows.CreateMutex(nil, false, windows.StringToUTF16Ptr(mutexName))
	if err != nil {
		return false, fmt.Errorf("CreateMutex failed: %w", err)
	}

	// If we get ERROR_ALREADY_EXISTS, another instance is already running.
	lastErr := windows.GetLastError()
	if lastErr == windows.ERROR_ALREADY_EXISTS {
		// We can close the handle
		_ = windows.CloseHandle(handle)
		return false, nil
	}

	// Otherwise, no existing instance, and we hold the mutex until the process exits.
	return true, nil
}

package main

import (
	"bufio"
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

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
	stateMu     sync.Mutex
	isRunning   bool
	startMenu   *systray.MenuItem
	stopMenu    *systray.MenuItem
)

const (
	containername = "ReEnvisionAI"
	container     = "ghcr.io/reenvision-ai/petals:latest"
	initial_peers = "/dns4/sociallyshaped.net/tcp/8788/p2p/QmTUpY86VSyvwvBN8oc9W3JztLaxyabT6b17gnXxdfx5HL"
	token         = "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"
	model_name    = "meta-llama/Llama-3.3-70B-Instruct"
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
	startMenu = systray.AddMenuItem("Start", "Start running ReEnvision AI")
	stopMenu = systray.AddMenuItem("Stop", "Stop running ReEnvision AI")
	quitMenu := systray.AddMenuItem("Quit", "Exit the application")

	go func() {
		startWSLProcess()
		startMenu.Disable()
		stopMenu.Enable()
	}()

	go func() {
		for {
			select {
			//case <-showLogsMenu.ClickedCh:
			//	showLogs()
			case <-stopMenu.ClickedCh:
				if setRunning(true) {
					go func() {
						stopWSLProcess()
						stopMenu.Disable()
						startMenu.Enable()
						setRunning(false)
					}()
				}
			case <-startMenu.ClickedCh:
				if setRunning(true) {
					go func() {
						startWSLProcess()
						startMenu.Disable()
						stopMenu.Enable()
						setRunning(false)
					}()
				}
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

func setupPodman() bool {
	// Check for Nvidia GPU before running nvidia-ctk
	hasNvididaGPU, err := checkNvidiaGPU()
	if err != nil {
		writeLog(fmt.Sprintf("Error checking for Nvidia GPU: %v", err))
		//return false
	}
	if !hasNvididaGPU {
		writeLog("Nvidia GPU not found, skipping nvidia-ctk setup.")
		return true // Not an error, just a skip
	}

	cmd := exec.Command("podman", "machine", "ssh", "sudo nvidia-ctk cdi generate --output=/etc/cdi/nvidia.yaml")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	ouptput, err := cmd.CombinedOutput()

	writeLog(string(ouptput))
	if err != nil {
		writeLog(fmt.Sprintf("Failed to connect GPU to Podman: %v", err))
		return false
	}

	return true
}

func checkNvidiaGPU() (bool, error) {
	if runtime.GOOS != "windows" {
		return false, fmt.Errorf("nvidia-smi is only available on Windows with Nvidia driver")
	}

	cmd := exec.Command("nvidia-smi", "--list-gpus")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, err
	}

	return len(output) > 0, nil
}

func waitForPodman() bool {
	start_cmd := exec.Command("podman", "machine", "start")
	start_cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	start_cmd.Run()

	timeout := time.After(5 * time.Minute)
	for {
		select {
		case <-timeout:
			writeLog("Timed out waiting for Podman")
			return false

		default:
			cmd := exec.Command("podman", "info")
			cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
			err := cmd.Run()
			if err == nil {
				writeLog("Podman is ready")
				return true
			}
			writeLog("Podman is not ready yet, waiting ...")
			time.Sleep(5 * time.Second)
		}
	}
}

func startWSLProcess() {
	if !waitForPodman() {
		writeLog("Aborting start: Podman did not become ready within 5 minutes")
		return
	}

	if !setupPodman() {
		writeLog("Aborting start: Podman could not be setup properly")
		return
	}

	port := "31330"
	volume := "reai-cache:/cache"

	wslCmd = exec.Command("podman", "run", "--network", "host", "--privileged", "--ipc", "host", "--device", "nvidia.com/gpu=all", "--gpus", "all", "--volume", volume, "--rm", "--name", containername, container, "python", "-m", "petals.cli.run_server", "--inference_max_length", "136192", "--port", port, "--max_alloc_timeout", "6000", "--quant_type", "nf4", "--attn_cache_tokens", "128000", model_name, "--token", token, "--initial_peers", initial_peers)
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
		stopMenu.Disable()
		startMenu.Enable()
		return
	}

	// Write an initial message
	writeLog("Started ReAI process")

	// Capture stdout/stderr in separate goroutines
	go captureOutput(stdout)
	go captureOutput(stderr)
}

func captureOutput(r io.ReadCloser) {
	defer r.Close()
	reader := bufio.NewReader(r)

	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			writeLog(line[:len(line)-1])
		}
		if err != nil {
			if err != io.EOF {
				writeLog(fmt.Sprintf("Error reading from ReAI output: %v", err))
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
	if wslCmd == nil || wslCmd.Process == nil {
		return
	}

	writeLog("Stopping ReAI process...")
	stopCmd := exec.Command("podman", "stop", containername)
	stopCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	err := stopCmd.Run()
	if err != nil {
		writeLog(fmt.Sprintf("Error stopping container: %v", err))
	}

	err = wslCmd.Process.Kill()
	if err != nil {
		writeLog(fmt.Sprintf("Error killing ReAI Process: %v", err))
	}

	_, err = wslCmd.Process.Wait()
	if err != nil {
		writeLog(fmt.Sprintf("Error waiting for ReAI process: %v", err))
	}

	wslCmd = nil
}

func writeLog(text string) {
	if logFile == nil {
		return
	}
	logMu.Lock()
	defer logMu.Unlock()
	fmt.Fprintf(logFile, "[%s] %s\n", time.Now().Format(time.RFC3339), text)

}

func setRunning(value bool) bool {
	stateMu.Lock()
	defer stateMu.Unlock()
	if isRunning == value {
		return false
	}
	isRunning = value
	return true
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

//go:build windows

package main

import (
	"bufio"
	"context"
	"crypto/aes"
	"crypto/cipher"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ReEnvision-AI/systray/internal/config"
	"github.com/ReEnvision-AI/systray/internal/logging"
	"github.com/ReEnvision-AI/systray/internal/power"

	"github.com/getlantern/systray"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"github.com/danieljoos/wincred"
	"github.com/ncruces/zenity"
	supa "github.com/supabase-community/supabase-go"
)

//go:embed reai.ico
var iconData []byte

// Constants
const (
	appName                   = "ReEnvision AI"
	mutexName                 = "Local\\ReEnvisionAIMutex"
	registryKeyPath           = `SOFTWARE\ReEnvisionAI\ReEnvisionAI`
	registryPortValue         = "Port"
	configDirName             = "ReEnvisionAI"
	configFileName            = "config.json"
	podmanVolumeName          = "reai-cache:/cache"
	nvidiaCDIConfPath         = "/etc/cdi/nvidia.yaml"
	podmanMachineStartTimeout = 5 * time.Minute
	podmanInfoPollInterval    = 5 * time.Second
	podmanStopTimeout         = 30 * time.Second
)

// Heartbeat Constants
const (
	heatbeatInterval    = 5 * time.Minute
	heartbeatTableName  = "heartbeats"
	heartbeatColumnName = "last_heartbeat"
	heartbeatUserIDCol  = "id"
)

// Supabase constants
const (
	a                    = "a9c1f75a2bd6cf9e1d5a7f2ce0d4b17f"
	credentialTargetName = "ReEnvisionAI/credentials"
	maxLoginAttempts     = 5
)

// Application states
type AppState int

const (
	StateStopped AppState = iota
	StateStarting
	StateRunning
	StateStopping
	StateError
)

func (s AppState) String() string {
	switch s {
	case StateStopped:
		return "Stopped"
	case StateStarting:
		return "Starting..."
	case StateRunning:
		return "Running"
	case StateStopping:
		return "Stopping..."
	case StateError:
		return "Error"
	default:
		return "Unknown"
	}
}

// Global state (carefully managed)
var (
	appConfig   config.AppConfig
	port        uint64
	instanceMtx windows.Handle // Handle for the single instance mutex
	email       string

	// UI Elements (managed by systray goroutine)
	mStatus *systray.MenuItem
	mStart  *systray.MenuItem
	mStop   *systray.MenuItem
	mLogs   *systray.MenuItem
	mQuit   *systray.MenuItem

	// Process and state management
	stateMu      sync.Mutex
	currentState AppState           = StateStopped
	currentCmd   *exec.Cmd          // Holds the running podman command
	cancelCmd    context.CancelFunc // Function to cancel the currentCmd context

	// Waitgroup to ensure background tasks like heartbeat finish
	appWg sync.WaitGroup

	// Context for controlling background goroutines like heartbeat
	appCtx       context.Context
	cancelAppCtx context.CancelFunc
)

func main() {
	var err error
	instanceMtx, err = ensureSingleInstance(mutexName)
	if err != nil {
		// If ERROR_ALREADY_EXISTS, it's not a fatal error for *this* instance, just means another is running.
		if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
			fmt.Printf("%s is already running. Exiting.\n", appName)
			os.Exit(0)
		} else {
			// Actual error creating mutex
			fmt.Printf("FATAL: Failed to check for single instance: %v\n", err)
			os.Exit(1)
		}
	}
	// Ensure the mutex is released when the application exits.
	// This is crucial!
	defer windows.CloseHandle(instanceMtx)

	// Initialize Logging (Must happen early)
	if err := logging.Init(); err != nil {
		// Log to console if file logging fails
		showErrorMessage("Logging Error", fmt.Sprintf("Logging initialization failed: %v. Logs may go to console only.", err))
		fmt.Printf("Logging initialization failed: %v. Logs may go to console only.\n", err)
	}
	defer logging.Close() // Ensure logs are flushed

	slog.Info(appName + " starting...")

	// Load Configuration
	if err := loadAppConfig(); err != nil {
		slog.Error("FATAL: Initialization failed", "error", err)
		showErrorMessage("Configuration Error", fmt.Sprintf("Failed to load configuration: %v", err))
		os.Exit(1)
	}

	// Get Port (Registry overrides config default)
	loadPortFromRegistry()

	// Validate essential config needed early
	if appConfig.SupabaseURL == "" || appConfig.SupabaseAnonKey == "" {
		slog.Error("FATAL: Initialization failed - Supabase URL or Anon Key missing in config")
		showErrorMessage("Configuration Error", "Supabase URL or Anon Key missing in configuration.")
		os.Exit(1)
	}

	var decryptErr error
	appConfig.SupabaseAnonKey, decryptErr = decrypt(appConfig.SupabaseAnonKey, a)
	if decryptErr != nil {
		slog.Error("Error decrypting supabase api key", "error", decryptErr)
		os.Exit(1)
	}

	appCtx, cancelAppCtx = context.WithCancel(context.Background())

	// Initialize Supabase client
	client, err := supa.NewClient(appConfig.SupabaseURL, appConfig.SupabaseAnonKey, nil)
	if err != nil {
		log.Fatalf("Error initializing Supabase client: %v\n", err)
	}

	slog.Info("Checking stored credentials")
	cred, err := loadCredentialsFromWCM(credentialTargetName)
	loginSuccess := false

	if err == nil && cred != nil {
		fmt.Printf("Found stored credentials for user: %s\n", cred.UserName)
		fmt.Println("Attempting login with stored credentials...")
		err = authenticateWithSupabase(client, cred.UserName, string(cred.CredentialBlob))
		if err == nil {
			slog.Info("Login successful using stored credentials!")
			loginSuccess = true
		} else {
			slog.Warn("Login with stored credentials failed", "error", err)
			errDel := cred.Delete()
			if errDel != nil {
				slog.Warn("Warning: Failed to delete outdated credential from WCM", "error", errDel)
			} else {
				slog.Info("Removed outdated credentials from Windows Credential Manager")
			}
		}
	} else if errors.Is(err, wincred.ErrElementNotFound) {
		slog.Info("No stored credentials found")
		// Proceed to manual login
	} else if err != nil {
		// Handle other WCM errors (permissions, etc.)
		slog.Warn("Error accessing Windows Credential Manager", "err", err)
		slog.Info("Proceeding without stored credentials")
		// Proceed to manual login
	}

	if !loginSuccess {
		var enteredEmail, enteredPassword string
		for attempt := 1; attempt <= maxLoginAttempts; attempt++ {
			fmt.Printf("\n--- Login Attempt %d of %d ---\n", attempt, maxLoginAttempts)
			enteredEmail, enteredPassword, err = promptForCredentials()
			if err != nil {
				log.Fatalf("Error getting credentials from user: %v\n", err) // Fatal error if we can't read input
			}

			fmt.Println("Attempting login...")
			err = authenticateWithSupabase(client, enteredEmail, enteredPassword)
			if err == nil {
				slog.Info("Login successful")
				loginSuccess = true

				// Save successful credentials to WCM
				slog.Info("Saving credentials to Windows Credential Manager...")
				errSave := saveCredentialsToWCM(credentialTargetName, enteredEmail, enteredPassword)
				if errSave != nil {
					slog.Warn("Failed to save credentials", "error", errSave)
				} else {
					slog.Info("Credentials saved successfully")
				}
				break // Exit loop on success
			} else {
				slog.Warn("Login failed", "error", err)
				if attempt == maxLoginAttempts {
					slog.Info("Maximum login attempts reached. Exiting")
				}
			}
		}
	}

	if !loginSuccess {
		os.Exit(1)
	}

	var userID string

	usr, err := client.Auth.GetUser()
	if err != nil {
		slog.Error("Failed to retrieve user info after successful login", "error", err)
		cancelAppCtx()
		os.Exit(1)
	}

	if usr == nil {
		slog.Error("User info is empty after successful login")
		cancelAppCtx()
		os.Exit(1)
	}

	email = usr.Email

	userID = usr.ID.String()
	if userID != "" {
		appWg.Add(1)
		go func() {
			defer appWg.Done()
			runHeartBeat(appCtx, client, userID, heatbeatInterval)
		}()
	} else {
		slog.Warn("Skipping heartbeat start because User ID is empty")
	}

	// Start the systray application
	systray.Run(onReady, onExit)

	slog.Info("Systray finished, ensuring all background tasks stopped")
	cancelAppCtx()

	slog.Info("Waiting for background tasks to stop...")
	waitChan := make(chan struct{})
	go func() {
		appWg.Wait()
		close(waitChan)
	}()

	select {
	case <-waitChan:
		slog.Info("All background tasks finished")
	case <-time.After(30 * time.Second):
		slog.Warn("Timeout waiting for background tasks to stop")
	}

	slog.Info("Application exit\n\n")
}

func loadAppConfig() error {
	configDir, err := os.UserCacheDir()
	if err != nil {
		slog.Warn("Failed to get user cache directory, falling back to working directory", "error", err)
		configDir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("cannot determine config directory: %w", err)
		}
	} else {
		configDir = filepath.Join(configDir, configDirName)
		if err := os.MkdirAll(configDir, 0750); err != nil {
			return fmt.Errorf("failed to create config directory %q: %w", configDir, err)
		}
	}

	configFile := filepath.Join(configDir, configFileName)
	slog.Info("Using configuration file", "path", configFile)

	appConfig, err = config.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration from %q: %w", configFile, err)
	}

	// Set default port initially from config
	port = appConfig.DefaultPort
	slog.Info("Default port set from config", "port", port)
	return nil
}

func loadPortFromRegistry() {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, registryKeyPath, registry.QUERY_VALUE)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			slog.Info("Registry key not found, using default/config port", "key", registryKeyPath, "port", port)
		} else {
			slog.Warn("Failed to open registry key, using default/config port", "key", registryKeyPath, "error", err)
		}
		return // Use port already set from config
	}
	defer key.Close()

	regPort, _, err := key.GetIntegerValue(registryPortValue)
	if err != nil {
		slog.Warn("Failed to read port value from registry, using default/config port", "value", registryPortValue, "error", err)
		return // Use port already set from config
	}

	port = regPort // Override with registry value
	slog.Info("Port loaded from registry", "port", port)
}

func onReady() {
	systray.SetIcon(iconData)
	systray.SetTitle(appName)
	systray.SetTooltip(appName) // Initial tooltip

	// Setup Menu Items
	mStatus = systray.AddMenuItem("Status: Initializing...", "Current container status")
	mStatus.Disable() // Disabled as it's informational
	systray.AddSeparator()
	mStart = systray.AddMenuItem("Start", "Start running "+appName)
	mStop = systray.AddMenuItem("Stop", "Stop running "+appName)
	systray.AddSeparator()
	mLogs = systray.AddMenuItem("Open Log Directory", "Open the log directory in File Explorer")
	systray.AddSeparator()
	mQuit = systray.AddMenuItem("Quit", "Exit the application")

	// Set initial UI state based on Stopped state
	setState(StateStopped) // This updates UI elements

	// Automatically start the container on launch
	// Consider making this behavior configurable
	go handleStartRequest()

	// Start the UI event loop
	go handleMenuEvents()
}

func onExit() {
	slog.Info(appName + " exiting...")

	// Create a context for shutdown operations
	shutdownCtx, cancel := context.WithTimeout(context.Background(), podmanStopTimeout+5*time.Second) // Give a bit extra time
	defer cancel()

	// Attempt graceful shutdown of the container if it's running or starting
	stateMu.Lock()
	shouldStop := currentState == StateRunning || currentState == StateStarting
	stateMu.Unlock()

	if shouldStop {
		slog.Info("Attempting graceful shutdown of container...")
		// This might block, so use the shutdown context
		err := stopContainerProcess(shutdownCtx)
		if err != nil {
			slog.Error("Error during shutdown stop", "error", err)
		}
	}

	// Ensure sleep is allowed on exit, regardless of container state
	if err := power.AllowSleep(); err != nil {
		slog.Warn("Failed to allow system sleep on exit", "error", err)
	}

	slog.Info(appName + " finished exit procedures.")
}

// setState updates the global application state and refreshes the UI accordingly.
// Must be called to change state.
func setState(newState AppState) {
	stateMu.Lock()
	currentState = newState
	stateMu.Unlock()

	// Update UI elements outside the lock to avoid blocking systray thread
	tooltip := fmt.Sprintf("%s: %s", appName, newState.String())
	systray.SetTooltip(tooltip)
	if mStatus != nil {
		mStatus.SetTitle("Status: " + newState.String())
	}

	switch newState {
	case StateStopped, StateError:
		if mStart != nil {
			mStart.Enable()
		}
		if mStop != nil {
			mStop.Disable()
		}
		// Allow sleep if we stopped cleanly or hit an error
		if err := power.AllowSleep(); err != nil && !errors.Is(err, power.ErrAlreadyAllowed) { // Avoid spamming logs if already allowed
			slog.Warn("Failed to allow system sleep", "error", err)
		}
	case StateRunning:
		if mStart != nil {
			mStart.Disable()
		}
		if mStop != nil {
			mStop.Enable()
		}
		// Ensure sleep is prevented when running
		if err := power.PreventSleep(); err != nil && !errors.Is(err, power.ErrAlreadyPrevented) { // Avoid spamming logs
			slog.Warn("Failed to prevent system sleep", "error", err)
		}
	case StateStarting, StateStopping:
		if mStart != nil {
			mStart.Disable()
		}
		if mStop != nil {
			mStop.Disable()
		}
		// Keep sleep prevention active during transitions
		if err := power.PreventSleep(); err != nil && !errors.Is(err, power.ErrAlreadyPrevented) {
			slog.Warn("Failed to prevent system sleep during transition", "error", err)
		}
	}
}

// handleMenuEvents runs in a goroutine, processing clicks on systray menu items.
func handleMenuEvents() {
	for {
		select {
		case <-mStart.ClickedCh:
			go handleStartRequest() // Run in new goroutine to avoid blocking UI thread

		case <-mStop.ClickedCh:
			go handleStopRequest() // Run in new goroutine

		case <-mLogs.ClickedCh:
			// This should be quick, no goroutine needed
			logging.OpenLogDirectory()

		case <-mQuit.ClickedCh:
			slog.Info("Quit requested via menu.")
			// Potentially update status? setState(StateStopping)?
			// onExit will handle the actual stopping logic.
			systray.Quit()
			return // Exit the event loop
		}
	}
}

func handleStartRequest() {
	stateMu.Lock()
	if currentState == StateRunning || currentState == StateStarting {
		slog.Info("Start request ignored, already running or starting.", "state", currentState)
		stateMu.Unlock()
		return
	}
	stateMu.Unlock() // Unlock before potentially long operation

	setState(StateStarting)

	// Create a context for the start operation
	// Use context.Background() for now, could potentially link to app lifecycle
	ctx := context.Background()

	err := startContainerProcess(ctx)
	if err != nil {
		slog.Error("Failed to start container process", "error", err)
		setState(StateError) // Set error state on failure
		// Consider showing an error message to the user via systray or message box
	} else {
		// startContainerProcess should ideally transition to Running state itself
		// upon successful start and monitoring setup. If it returns nil error,
		// we assume it's running or will transition shortly.
		// Let's refine startContainerProcess to handle this.
	}
}

func handleStopRequest() {
	stateMu.Lock()
	if currentState == StateStopped || currentState == StateStopping || currentState == StateError {
		slog.Info("Stop request ignored, not running or already stopping.", "state", currentState)
		stateMu.Unlock()
		return
	}
	stateMu.Unlock()

	setState(StateStopping)

	// Create a context with timeout for the stop operation
	ctx, cancel := context.WithTimeout(context.Background(), podmanStopTimeout)
	defer cancel()

	err := stopContainerProcess(ctx)
	if err != nil {
		slog.Error("Failed to stop container process", "error", err)
		// Should we go to Error state or Stopped state? Let's assume Stopped for now.
		setState(StateStopped)
		// Consider showing an error message
	} else {
		setState(StateStopped) // Explicitly set to stopped on successful stop
	}
}

func waitForPodman(ctx context.Context) error {
	slog.Info("Waiting for Podman machine and service...")

	// Attempt to start the machine, ignore errors for now (might already be running)
	// Hide the window for this command.
	startCmd := exec.CommandContext(ctx, "podman", "machine", "start")
	startCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	startOutput, startErr := startCmd.CombinedOutput()
	if startErr != nil {
		// Log output only if there was an error, might contain useful info
		slog.Warn("Podman machine start command finished", "output", string(startOutput), "error", startErr)
		// Don't return yet, maybe it's already running and 'podman info' will succeed
	} else {
		slog.Info("Podman machine start command finished", "output", string(startOutput))
	}

	// Check podman info periodically
	ticker := time.NewTicker(podmanInfoPollInterval)
	defer ticker.Stop()

	// Combined timeout for the whole wait process
	waitCtx, cancel := context.WithTimeout(ctx, podmanMachineStartTimeout)
	defer cancel()

	for {
		select {
		case <-waitCtx.Done():
			return fmt.Errorf("timed out after %v waiting for podman service", podmanMachineStartTimeout)
		case <-ticker.C:
			slog.Info("Checking podman status...")
			cmd := exec.CommandContext(waitCtx, "podman", "info")
			cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
			// Run and discard output, we only care about the exit code
			if err := cmd.Run(); err == nil {
				slog.Info("Podman service is ready.")
				return nil // Podman is ready
			} else {
				// Log the specific error from podman info
				slog.Info("Podman service not ready yet", "error", err)
			}
		}
	}
}

func checkNvidiaGPU(ctx context.Context) (bool, error) {

	slog.Info("Checking for Nvidia GPU using nvidia-smi...")
	cmd := exec.CommandContext(ctx, "nvidia-smi", "--list-gpus")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	output, err := cmd.Output() // Use Output instead of CombinedOutput if stderr is not needed for success check
	if err != nil {
		// Check if the error is because the command wasn't found or failed execution
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// Command ran but returned non-zero exit code. Likely no GPUs found or driver issue.
			slog.Warn("nvidia-smi command finished with non-zero status.", "stderr", string(exitErr.Stderr))
			return false, nil // Treat as "no GPU found" rather than a fatal error
		}
		// Other errors (e.g., command not found)
		return false, fmt.Errorf("failed to execute nvidia-smi: %w", err)
	}

	found := len(output) > 0
	if found {
		slog.Info("Nvidia GPU detected.")
	} else {
		slog.Info("No Nvidia GPU detected by nvidia-smi.")
	}
	return found, nil
}

func setupPodmanNvidia(ctx context.Context) error {
	hasGPU, err := checkNvidiaGPU(ctx)
	if err != nil {
		// Log the error but don't necessarily block startup if check fails
		slog.Error("Error checking for Nvidia GPU", "error", err)
		// Decide if this is fatal. If GPU support is optional, maybe just warn and continue.
		// For now, let's warn and proceed without GPU setup.
		slog.Warn("Proceeding without attempting Nvidia CDI setup due to GPU check error.")
		return nil // Not treating GPU check failure as fatal for setup
	}

	if !hasGPU {
		slog.Info("No Nvidia GPU detected or nvidia-smi failed, skipping Nvidia CDI setup for Podman.")
		return nil // Not an error, just skipping setup
	}

	slog.Info("Nvidia GPU detected, attempting to configure Podman machine via CDI...")

	// Command to generate CDI spec inside the podman machine VM
	// IMPORTANT: This assumes passwordless sudo and nvidia-ctk installed in the VM.
	cdiCmd := fmt.Sprintf("sudo nvidia-ctk cdi generate --output=%s", nvidiaCDIConfPath)
	cmd := exec.CommandContext(ctx, "podman", "machine", "ssh", cdiCmd)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("Failed to generate Nvidia CDI configuration in Podman machine.",
			"command", cmd.String(),
			"output", string(output),
			"error", err)
		// This might be critical depending on whether GPU is required.
		// Returning an error signals failure.
		return fmt.Errorf("nvidia CDI setup failed: %w. Output: %s", err, string(output))
	}

	slog.Info("Successfully generated Nvidia CDI configuration.", "path_in_vm", nvidiaCDIConfPath, "output", string(output))
	return nil
}

// buildPodmanRunCommand constructs the podman run command arguments.
func buildPodmanRunCommandArgs() []string {
	// Base arguments
	args := []string{
		"run",
		"--network=host", // Use host networking
		"--rm",           // Remove container on exit
		"--name=" + appConfig.ContainerName,
		"--volume=" + podmanVolumeName, // Mount cache volume
	}

	// GPU arguments - Use CDI if available, requires Podman >= 4.x
	// Using --device nvidia.com/gpu=all enables CDI discovery.
	// --gpus=all might be redundant or an older way. Check Podman docs.
	// Let's use the recommended CDI approach if GPU is intended.
	// Assuming setupPodmanNvidia was successful if GPU is desired/present.
	// We might need a config flag or runtime check result to decide if GPU args are added.
	// For now, add them conditionally based on a simple config flag (example)
	if appConfig.UseGPU { // Assuming an `UseGPU bool` field in config.AppConfig
		slog.Info("Adding GPU arguments to podman run command.")
		args = append(args, "--device=nvidia.com/gpu=all")
		// Privilege/IPC might be needed for some GPU setups/drivers
		args = append(args, "--privileged") // CAUTION: Security risk! Evaluate if necessary.
		args = append(args, "--ipc=host")   // Often needed for CUDA multi-process
	} else {
		slog.Info("GPU arguments omitted based on configuration.")
	}

	// Add image and command parts
	args = append(args, appConfig.ContainerImage) // The image name
	args = append(args,                           // The command and its arguments within the container
		"python", "-m", "petals.cli.run_server",
		"--inference_max_length", "136192",
		"--port", strconv.FormatUint(port, 10),
		"--max_alloc_timeout", "6000",
		"--quant_type", "nf4",
		"--attn_cache_tokens", "128000",
		appConfig.ModelName,
		"--token", appConfig.Token,
		"--initial_peers", appConfig.InitialPeers,
	)

	if email != "" {
		obfuscatedEmail, err := obfuscateEmail(email)
		if err != nil {
			slog.Warn("Error trying to obfuscate the user's email", "error", err)
		} else {
			args = append(args, "--public_name", obfuscatedEmail)
		}
	}

	return args
}

func startContainerProcess(ctx context.Context) error {
	// 1. Wait for Podman Service
	if err := waitForPodman(ctx); err != nil {
		return fmt.Errorf("podman service check failed: %w", err)
	}

	// 2. Setup Podman (e.g., GPU integration)
	// Pass a derived context in case setup takes time.
	setupCtx, setupCancel := context.WithTimeout(ctx, 2*time.Minute) // Timeout for setup step
	defer setupCancel()
	if err := setupPodmanNvidia(setupCtx); err != nil {
		slog.Error("Nvidia setup for Podman failed. Container might not use GPU.", "error", err)
		return fmt.Errorf("podman Nvidia setup failed: %w", err)
	}

	// 3. Build and Start the Container Command
	stateMu.Lock()
	// Double-check state in case a stop request came in during wait/setup
	if currentState != StateStarting {
		slog.Warn("Container start aborted, state changed during initialization.", "state", currentState)
		stateMu.Unlock()
		// Allow sleep if we were preventing it during start attempt
		if err := power.AllowSleep(); err != nil && !errors.Is(err, power.ErrAlreadyAllowed) {
			slog.Warn("Failed to allow sleep after aborted start", "error", err)
		}
		return nil // Not an error, just an aborted start
	}

	// Create a new context for the command itself, with cancellation
	cmdCtx, cmdCancel := context.WithCancel(context.Background()) // Use Background, manage lifecycle via cancelCmd
	cancelCmd = cmdCancel                                         // Store the cancel function globally

	args := buildPodmanRunCommandArgs()
	currentCmd = exec.CommandContext(cmdCtx, "podman", args...)
	currentCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	slog.Info("Starting container", "command", currentCmd.String())

	stdoutPipe, err := currentCmd.StdoutPipe()
	if err != nil {
		cancelCmd() // Clean up context
		currentCmd = nil
		stateMu.Unlock()
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}
	stderrPipe, err := currentCmd.StderrPipe()
	if err != nil {
		cancelCmd()
		currentCmd = nil
		stateMu.Unlock()
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	// Release the lock before starting the command and goroutines
	stateMu.Unlock()

	// Start capturing output *before* starting the command
	var wg sync.WaitGroup
	wg.Add(2)
	go captureOutput(&wg, stdoutPipe, "stdout")
	go captureOutput(&wg, stderrPipe, "stderr")

	// Start the command
	if err := currentCmd.Start(); err != nil {
		// Attempt to cancel context if start fails, although it might be redundant
		cancelCmd()
		stateMu.Lock()
		currentCmd = nil // Clear the command
		stateMu.Unlock()
		// Wait briefly for capture goroutines to potentially see EOF if pipes closed early
		outputCaptureDone := make(chan struct{})
		go func() {
			wg.Wait()
			close(outputCaptureDone)
		}()
		select {
		case <-outputCaptureDone:
			// Goroutines finished
		case <-time.After(1 * time.Second):
			slog.Warn("Timeout waiting for output goroutines after command start failure")
		}
		return fmt.Errorf("failed to start podman command: %w", err)
	}

	slog.Info("Container process started successfully.", "pid", currentCmd.Process.Pid)
	setState(StateRunning) // Transition to Running state *after* successful start

	// Goroutine to wait for the command to exit and handle cleanup
	go func() {
		// Wait for the command to finish (either normally, by error, or cancellation)
		waitErr := currentCmd.Wait()

		// Wait for output streams to be fully processed
		wg.Wait()

		stateMu.Lock()
		// Check if we are supposed to be stopping; if so, the state is handled by stopContainerProcess
		isStopping := currentState == StateStopping
		// Clear command and cancel function regardless
		currentCmd = nil
		cancelCmd = nil // Allow GC
		stateMu.Unlock()

		if waitErr != nil {
			// Log error unless it was context cancellation during a planned stop
			if !(errors.Is(waitErr, context.Canceled) && isStopping) {
				slog.Error("Container process exited unexpectedly.", "error", waitErr)
				if !isStopping { // Avoid overwriting Stopping state
					setState(StateError)
				}
			} else {
				slog.Info("Container process exited after cancellation (likely during stop).")
				// State should already be Stopping or Stopped
			}
		} else {
			slog.Info("Container process exited normally.")
			if !isStopping { // If it exited normally without a stop request
				setState(StateStopped)
			}
		}
	}()

	return nil // Start initiated successfully
}

func captureOutput(wg *sync.WaitGroup, rc io.ReadCloser, streamName string) {
	defer wg.Done()
	defer rc.Close()
	scanner := bufio.NewScanner(rc)
	for scanner.Scan() {
		slog.Info(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		// Don't log EOF errors, they are expected
		if !errors.Is(err, io.EOF) {
			slog.Error("Error reading container output", "stream", streamName, "error", err)
		}
	}
	slog.Debug("Finished capturing output", "stream", streamName)
}

func stopContainerProcess(ctx context.Context) error {
	slog.Info("Attempting to stop container.", "name", appConfig.ContainerName)

	// Use `podman stop` first for graceful shutdown within the container
	stopCmd := exec.CommandContext(ctx, "podman", "stop", appConfig.ContainerName)
	stopCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	stopOutput, stopErr := stopCmd.CombinedOutput()

	if stopErr != nil {
		// Log the error but continue, as we might need to cancel the `podman run` process anyway
		slog.Warn("`podman stop` command failed or timed out.",
			"output", string(stopOutput),
			"error", stopErr)
		// If the context timed out, log that specifically
		if errors.Is(stopErr, context.DeadlineExceeded) {
			slog.Warn("Context deadline exceeded while waiting for `podman stop`.")
		} else if ctx.Err() != nil {
			// Parent context was canceled (e.g., during shutdown)
			slog.Warn("Stop operation canceled by parent context.", "error", ctx.Err())
		}
	} else {
		slog.Info("`podman stop` command completed successfully.", "output", string(stopOutput))
	}

	// Regardless of `podman stop` success, cancel the `podman run` command's context.
	// This signals `currentCmd.Wait()` to unblock if it hasn't already.
	stateMu.Lock()
	if cancelCmd != nil {
		slog.Info("Cancelling container command context.")
		cancelCmd()
		// The goroutine waiting on currentCmd.Wait() should handle subsequent cleanup (setting currentCmd=nil etc.)
	} else {
		slog.Info("No active container command context to cancel.")
	}
	// We don't set currentCmd = nil here; the Wait() goroutine does that upon exit confirmation.
	stateMu.Unlock()

	// Note: We don't forcefully kill the `podman run` process (`currentCmd.Process.Kill()`)
	// because `podman stop` followed by context cancellation should be sufficient.
	// The `--rm` flag ensures the container is removed eventually. Killing `podman run`
	// might prevent `--rm` from working correctly within the Podman VM.

	// The state transition to Stopped is handled either by the handleStopRequest function
	// on success, or by the Wait() goroutine when the process finally exits.

	// Return the error from `podman stop` if there was one, allowing caller to know if graceful stop failed.
	if stopErr != nil && !errors.Is(stopErr, context.Canceled) && !errors.Is(stopErr, context.DeadlineExceeded) {
		return fmt.Errorf("podman stop failed: %w", stopErr)
	}

	return nil // Indicates stop sequence initiated (or stop command succeeded)
}

// ensureSingleInstance tries to create a named mutex.
// Returns the mutex handle if successful and this is the first instance.
// Returns `windows.ERROR_ALREADY_EXISTS` if another instance holds the mutex.
// Returns other errors if mutex creation fails unexpectedly.
// IMPORTANT: The caller is responsible for closing the returned handle on exit.
func ensureSingleInstance(name string) (windows.Handle, error) {
	// Create a UTF-16 pointer to the mutex name
	mutexNamePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return 0, fmt.Errorf("failed to convert mutex name %q to UTF16: %w", name, err)
	}

	// Try to create the mutex
	handle, err := windows.CreateMutex(nil, false, mutexNamePtr) // Pass false to check if it exists

	// Check the error *after* potential creation
	if err != nil {
		// Check if the error is specifically ERROR_ALREADY_EXISTS
		// Comparing syscall.Errno directly might be fragile, using GetLastError is more reliable.
		// However, the `windows` package often maps common errors directly. Let's check `GetLastError` just to be safe.
		lastErr := windows.GetLastError()
		if lastErr == windows.ERROR_ALREADY_EXISTS {
			// Another instance exists. Close the handle we potentially received and return the specific error.
			if handle != 0 {
				windows.CloseHandle(handle) // Close the handle we got for the *existing* mutex
			}
			return 0, windows.ERROR_ALREADY_EXISTS // Signal that another instance is running
		}
		// Some other unexpected error occurred during CreateMutex
		return 0, fmt.Errorf("CreateMutex failed with unexpected error: %w (GetLastError: %v)", err, lastErr)
	}

	// If err is nil, we successfully created the mutex *or* opened an existing one without error
	// (though the 'false' argument should make CreateMutex return ERROR_ALREADY_EXISTS if it exists).
	// Assuming err == nil means *we* created it and are the first instance.
	slog.Info("Successfully acquired single instance mutex.", "name", name)
	return handle, nil // Return the handle to the caller
}

// showErrorMessage displays a native Windows message box.
func showErrorMessage(title, message string) {
	slog.Debug("Showing message box", "title", title, "message", message)
	zenity.Error(message,
		zenity.Title(title),
		zenity.ErrorIcon)
}

func decrypt(encryptedText, key string) (string, error) {
	cipherText, err := base64.StdEncoding.DecodeString(encryptedText)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(cipherText) < nonceSize {
		return "", fmt.Errorf("cipherText too short")
	}
	nonce, cipherText := cipherText[:nonceSize], cipherText[nonceSize:]
	plainText, err := gcm.Open(nil, nonce, cipherText, nil)
	if err != nil {
		return "", err
	}
	return string(plainText), nil
}

func loadCredentialsFromWCM(targetName string) (*wincred.GenericCredential, error) {
	cred, err := wincred.GetGenericCredential(targetName)
	if err != nil {
		return nil, fmt.Errorf("WCM GetGenericCredential error: %w", err) // Wrap error for better context
	}
	if cred == nil {
		// Should not happen if err is nil, but good practice to check
		return nil, wincred.ErrElementNotFound
	}
	return cred, nil
}

func authenticateWithSupabase(client *supa.Client, email, password string) error {
	fmt.Printf("Logging in with %s / %s \n", email, password)
	session, err := client.SignInWithEmailPassword(email, password)

	if err != nil {
		return fmt.Errorf("supabase sign-in error: %w", err) // Wrap error
	}

	client.UpdateAuthSession(session)
	client.EnableTokenAutoRefresh(session)

	return nil // Success
}

func promptForCredentials() (email string, password string, err error) {
	username, password, err := zenity.Password(
		zenity.Title("Type your username (email) and password"),
		zenity.Username(),
		zenity.Modal(),
		zenity.NoCancel())

	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)

	if username == "" || password == "" {
		err = errors.New("username and password can't be empty")
	}

	return username, password, err
}

func saveCredentialsToWCM(targetName, username, password string) error {
	cred := wincred.NewGenericCredential(targetName)
	cred.UserName = username
	cred.CredentialBlob = []byte(password) // Store password as bytes
	cred.Persist = wincred.PersistLocalMachine

	err := cred.Write()
	if err != nil {
		return fmt.Errorf("WCM Write error: %w", err) // Wrap error
	}
	return nil
}

func runHeartBeat(ctx context.Context, client *supa.Client, userID string, interval time.Duration) {
	if client == nil {
		slog.Error("Heartbeat: DB client is nil, cannot run heartbeat")
		return
	}

	if userID == "" {
		slog.Error("Heartbeat: User ID is empty, cannot run heartbeat")
		return
	}

	slog.Info("Starting heartbeat tickert", "interval", interval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	sendHeartBeatUpdate(client, userID)

	for {
		select {
		case <-ticker.C:
			slog.Debug("Heartbeat ticker received")
			sendHeartBeatUpdate(client, userID)

		case <-ctx.Done():
			slog.Info("Heartbeat context cancelled, stopping heartbeat")
			return
		}
	}
}

func sendHeartBeatUpdate(client *supa.Client, userID string) {
	currentTime := time.Now().UTC() // Using UTC for consistency
	updateData := map[string]interface{}{
		heartbeatUserIDCol:  userID,
		heartbeatColumnName: currentTime,
	}

	var result []map[string]interface{}
	_, err := client.From(heartbeatTableName).Upsert(updateData, "", "", "").ExecuteTo(&result)

	if err != nil {
		slog.Error("Heartbeat update failed", "error", err, "userID", userID)
	} else {
		slog.Debug("Heartbeat sent successfully", "time", currentTime)
	}
}

// ObfuscateEmail takes an email string and returns an obfuscated version
// according to the specified rules:
// - First letter of the username
// - Asterisks (*) for characters between the first and last of the username
// - Last letter of the username
// - The "@" symbol
// - First letter of the domain
// - Four asterisks ("****")
// - The domain extension (including the dot, e.g., ".com", ".org")
// Returns the obfuscated email and an error if the input format is invalid.
func obfuscateEmail(email string) (string, error) {
	// 1. Find the '@' symbol to split username and domain
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 || len(parts[0]) == 0 || len(parts[1]) == 0 {
		return "", errors.New("invalid email format: missing '@' or empty username/domain")
	}
	username := parts[0]
	domain := parts[1]

	// 2. Obfuscate the username part
	var obfuscatedUsername strings.Builder
	usernameLen := len(username)

	if usernameLen == 0 {
		// This case is already handled by the initial check, but included for clarity
		return "", errors.New("invalid email format: empty username")
	}

	// Add the first letter
	obfuscatedUsername.WriteByte(username[0]) // Assuming ASCII/single-byte first char

	// Add asterisks if username length > 1
	if usernameLen > 2 {
		// Add len - 2 asterisks
		obfuscatedUsername.WriteString(strings.Repeat("*", usernameLen-2))
	}

	// Add the last letter if username length > 1
	if usernameLen > 1 {
		obfuscatedUsername.WriteByte(username[len(username)-1]) // Assuming ASCII/single-byte last char
	}
	// If usernameLen is 1, only the first letter is added, which is correct.

	// 3. Obfuscate the domain part
	// Find the last '.' to identify the extension
	dotIndex := strings.LastIndex(domain, ".")
	// Ensure domain has at least one char before the dot and the dot exists
	if dotIndex <= 0 || dotIndex == len(domain)-1 {
		return "", errors.New("invalid email format: domain requires a '.' separating name and extension")
	}

	var obfuscatedDomain strings.Builder

	// Add the first letter of the domain
	obfuscatedDomain.WriteByte(domain[0]) // Assuming ASCII/single-byte first char

	// Add four asterisks
	obfuscatedDomain.WriteString("****")

	// Add the extension (including the dot)
	obfuscatedDomain.WriteString(domain[dotIndex:])

	// 4. Combine the parts
	return obfuscatedUsername.String() + "@" + obfuscatedDomain.String(), nil
}

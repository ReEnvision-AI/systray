//go:build windows

// Package logging provides simple file logging capabilities.
package logging

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath" // Added for initialization
	"runtime"
	"sync"
	"time"

	"github.com/gonutz/w32/v2"         // For showing message boxes if needed early
	"gopkg.in/natefinch/lumberjack.v2" // for log rotation
)

var (
	// LogFile is the handle to the log file. It should be initialized before use.
	// It's exported so main (or another setup function) can assign the opened file handle to it.
	LogFile *os.File

	// logMu protects concurrent writes to LogFile. It's kept unexported
	// as WriteLog handles the locking internally.
	logMu sync.Mutex

	logDir string

	// logFilePath stores the path to the log file after initialization. Kept unexported.
	logFilePath string

	logOutput *lumberjack.Logger
)

// Init initializes the logging system.
// It creates the necessary directory and opens/truncates the log file.
// It should be called once at application startup.
func Init() error {
	logMu.Lock()
	defer logMu.Unlock()

	// Prevent re-initialization if already done
	if LogFile != nil && LogFile != os.Stdout {
		return nil // Already initialized
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get user config directory: %w", err)
	}

	logDir = filepath.Join(configDir, "ReEnvisionAI")
	err = os.MkdirAll(logDir, 0755)
	if err != nil {
		// Log directly to stderr if directory creation fails early
		fmt.Fprintf(os.Stderr, "[%s] Failed to create log directory %s: %v\n", time.Now().Format(time.RFC3339), logDir, err)
		// Fallback to stdout, but return the error
		LogFile = os.Stdout
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	logFilePath = filepath.Join(logDir, "reai.log")

	logOutput = &lumberjack.Logger{
		Filename:   logFilePath,
		MaxSize:    10, //MBs
		MaxBackups: 3,
		MaxAge:     28,
		Compress:   false,
	}

	log.SetOutput(logOutput)
	log.SetFlags(log.LstdFlags)

	log.Printf("[%s] Logging initialized to file: %s\n", time.Now().Format(time.RFC3339), logFilePath)

	return nil
}

func Close() error {
	err := logOutput.Close()

	return err
}

// OpenLogDirectory opens the directory containing the log files in Windows Explorer.
func OpenLogDirectory() {
	if logDir == "" {
		log.Println("Log directory not initialized.")
		// Maybe try to determine it again?
		return
	}
	if runtime.GOOS == "windows" {
		cmd := exec.Command("explorer", logDir)
		err := cmd.Start() // Use Start, not Run, to avoid blocking
		if err != nil {
			log.Printf("Failed to open log directory '%s': %v", logDir, err)
			// Show message box as fallback?
			w32.MessageBox(0, fmt.Sprintf("Could not open log directory automatically.\n\nPlease navigate to:\n%s", logDir), "Error", w32.MB_OK|w32.MB_ICONERROR)
		}
	} else {
		log.Println("OpenLogDirectory is only implemented for Windows.")
	}
}

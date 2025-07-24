package lifecycle

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	AppName          = "ReEnvisionAI"
	AppDir           = "/opt/reai"
	AppDataDir       = "/opt/reai"
	UpdateStageDir   = "/tmp"
	AppLogFile       = "/tmp/reai_app.log"
	UpgradeLogFile   = "/tmp/reai_update.log"
	Installer        = "ReEnvisionAISetup.exe"
	LogRotationCount = 5
)

func init() {
	if runtime.GOOS == "windows" {
		AppName += ".exe"
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			slog.Error("LOCALAPPDATA environment variable not set")
			// Handle error appropriately, maybe fall back to a default
			return
		}
		AppDataDir = filepath.Join(localAppData, "ReEnvision AI")
		UpdateStageDir = filepath.Join(AppDataDir, "updates")
		AppLogFile = filepath.Join(AppDataDir, "app.log")
		UpgradeLogFile = filepath.Join(AppDataDir, "upgrade.log")

		exe, err := os.Executable()
		if err != nil {
			slog.Warn("error discovering executable directory", "error", err)
			AppDir = filepath.Join(localAppData, "Programs", "ReEnvision AI")
		} else {
			AppDir = filepath.Dir(exe)
		}
		slog.Debug("Application paths initialized",
			"AppName", AppName,
			"AppDir", AppDir,
			"AppDataDir", AppDataDir,
			"UpdateStageDir", UpdateStageDir,
			"AppLogFile", AppLogFile,
			"UpgradeLogFile", UpgradeLogFile,
		)

		// Make sure we have PATH set correctly for any spawned children
		paths := strings.Split(os.Getenv("PATH"), ";")
		found := false
		for _, path := range paths {
			d, err := filepath.Abs(path)
			if err != nil {
				continue
			}
			if strings.EqualFold(AppDir, d) {
				found = true
				break
			}
		}
		if !found {
			newPath := strings.Join(append(paths, AppDir), ";")
			slog.Debug("Updating PATH", "newPath", newPath)
			if err := os.Setenv("PATH", newPath); err != nil {
				slog.Error("failed to update PATH", "error", err)
			}
		}

		// Make sure our logging dir exists
		if _, err := os.Stat(AppDataDir); errors.Is(err, os.ErrNotExist) {
			slog.Info("Creating application data directory", "path", AppDataDir)
			if err := os.MkdirAll(AppDataDir, 0o755); err != nil {
				slog.Error("failed to create application data directory", "path", AppDataDir, "error", err)
			}
		}
	}
}

package lifecycle

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	AppName    = "ReEnvisionAI"
	AppDir     = "/opt/reai"
	AppDataDir = "/opt/reai"
	// TODO - should there be a distinct log dir?
	UpdateStageDir   = "/tmp"
	AppLogFile       = "/tmp/reai_app.log"
	UpgradeLogFile   = "/tmp/reai_update.log"
	Installer        = "ReEnvisionAISetup.exe"
	LogRotationCount = 5
)

func init() {
	if runtime.GOOS == "windows" {
		AppName += ".exe"
		// Logs, configs, downloads go to LOCALAPPDATA
		localAppData := os.Getenv("LOCALAPPDATA")
		fmt.Println("localAppData", localAppData)
		AppDataDir = filepath.Join(localAppData, "ReEnvision AI")
		fmt.Println("AppDataDir", AppDataDir)
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

		// Make sure we have PATH set correctly for any spawned children
		paths := strings.Split(os.Getenv("PATH"), ";")
		// Start with whatever we find in the PATH/LD_LIBRARY_PATH
		found := false
		for _, path := range paths {
			d, err := filepath.Abs(path)
			if err != nil {
				continue
			}
			if strings.EqualFold(AppDir, d) {
				found = true
			}
		}
		if !found {
			paths = append(paths, AppDir)

			pathVal := strings.Join(paths, ";")
			slog.Debug("setting PATH=" + pathVal)
			err := os.Setenv("PATH", pathVal)
			if err != nil {
				slog.Error(fmt.Sprintf("failed to update PATH: %s", err))
			}
		}

		// Make sure our logging dir exists
		_, err = os.Stat(AppDataDir)
		if errors.Is(err, os.ErrNotExist) {
			if err := os.MkdirAll(AppDataDir, 0o755); err != nil {
				slog.Error(fmt.Sprintf("create reai dir %s: %v", AppDataDir, err))
			}
		}
	}
}

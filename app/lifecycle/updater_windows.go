package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
)

func DoUpgrade(cancel context.CancelFunc, done chan int) error {
	files, err := filepath.Glob(filepath.Join(UpdateStageDir, "*", "*.exe"))
	if err != nil {
		return fmt.Errorf("failed to lookup downloads: %s", err)
	}
	if len(files) == 0 {
		return errors.New("no update downloads found")
	} else if len(files) > 1 {
		// Shouldn't happen
		slog.Warn(fmt.Sprintf("multiple downloads found, using first one %v", files))
	}
	installerExe := files[0]
	slog.Info("starting upgrade with " + installerExe)
	slog.Info("upgrade log file " + UpgradeLogFile)

	// make the upgrade show progress, but non interactive
	installArgs := []string{
		"/CLOSEAPPLICATIONS",                    // Quit the tray app if it's still running
		"/LOG=" + filepath.Base(UpgradeLogFile), // Only relative seems reliable, so set pwd
		"/FORCECLOSEAPPLICATIONS",               // Force close the tray app - might be needed
		"/SP",                                   // Skip the "This will install... Do you wish to continue" prompt
		"/NOCANCEL",                             // Disable the ability to cancel upgrade mid-flight to avoid partially installed upgrades
		"/SILENT",
	}

	cancel()
	if done != nil {
		<-done
	} else {
		// Shouldn't happen
		slog.Warn("done chan was nil, not actually waiting")
	}

	slog.Debug(fmt.Sprintf("starting installer: %s %v", installerExe, installArgs))
	os.Chdir(filepath.Dir(UpgradeLogFile)) //nolint:errcheck
	cmd := exec.Command(installerExe, installArgs...)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("unable to start ReEnvision AI app %w", err)
	}

	if cmd.Process != nil {
		err = cmd.Process.Release()
		if err != nil {
			slog.Error(fmt.Sprintf("failed to release server process: %s", err))
		}
	} else {
		return errors.New("installer process did not start")
	}

	slog.Info("Installer started in background, exiting")

	os.Exit(0)
	// Not reached
	return nil
}

package lifecycle

import (
	"log/slog"
	"os/exec"
	"syscall"
)

func ShowLogs() {
	cmdPath := "c:\\Windows\\system32\\cmd.exe"
	slog.Debug("Opening log directory", "path", AppDataDir)
	cmd := exec.Command(cmdPath, "/c", "explorer", AppDataDir)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: false, CreationFlags: 0x08000000}
	if err := cmd.Start(); err != nil {
		slog.Error("Failed to open log directory", "path", AppDataDir, "error", err)
	}
}
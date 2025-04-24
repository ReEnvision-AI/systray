package lifecycle

import (
	"fmt"
	"log/slog"
	"os/exec"
	"syscall"
)

func ShowLogs() {
	cmd_path := "c:\\Windows\\system32\\cmd.exe"
	fmt.Println("cmd_path", cmd_path)
	slog.Debug(fmt.Sprintf("viewing logs with start %s", AppDataDir))
	fmt.Println("AppDataDir", AppDataDir)
	cmd := exec.Command(cmd_path, "/c", "explorer", AppDataDir)
	fmt.Println("cmd", cmd)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: false, CreationFlags: 0x08000000}
	err := cmd.Start()
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to open log dir: %s", err))
		fmt.Println("Failed to open log dir", err)
	}
}

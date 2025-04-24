package lifecycle

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func InitLogging() {
	level := slog.LevelInfo

	var logFile *os.File
	var err error

	rotateLogs(AppLogFile)
	logFile, err = os.OpenFile(AppLogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0755)
	fmt.Println("logFile", logFile)
	if err != nil {
		slog.Error(fmt.Sprintf("failed to create log %v", err))
		return
	}
	//defer logFile.Close()
	handler := slog.NewTextHandler(logFile, &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
		ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
			if attr.Key == slog.SourceKey {
				source := attr.Value.Any().(*slog.Source)
				source.File = filepath.Base(source.File)
			}
			return attr
		},
	})

	slog.SetDefault(slog.New(handler))

	slog.Info("ReEnvision AI logging starting")

}

func rotateLogs(logFile string) {
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		return
	}
	index := strings.LastIndex(logFile, ".")
	pre := logFile[:index]
	post := "." + logFile[index+1:]
	for i := LogRotationCount; i > 0; i-- {
		older := pre + "-" + strconv.Itoa(i) + post
		newer := pre + "-" + strconv.Itoa(i-1) + post
		if i == 1 {
			newer = pre + post
		}
		if _, err := os.Stat(newer); err == nil {
			if _, err := os.Stat(older); err == nil {
				err := os.Remove(older)
				if err != nil {
					slog.Warn("Failed to remove older log", "older", older, "error", err)
					continue
				}
			}
			err := os.Rename(newer, older)
			if err != nil {
				slog.Warn("Failed to rotate log", "older", older, "newer", newer, "error", err)
			}
		}
	}
}

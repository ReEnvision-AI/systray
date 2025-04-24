package tray

import (
	"github.com/ReEnvision-AI/systray/app/tray/commontray"
	"github.com/ReEnvision-AI/systray/app/tray/wintray"
)

func InitPlatformTray(icon, updateIcon []byte) (commontray.ReaiTray, error) {
	return wintray.InitTray(icon, updateIcon)
}
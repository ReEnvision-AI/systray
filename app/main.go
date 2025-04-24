package main

import (
	"github.com/ReEnvision-AI/systray/app/lifecycle"
)

// Compile with the following to get rid of the cmd popup on windows
// go build -ldflags="-H windowsgui"

func main() {
	lifecycle.Run()
}

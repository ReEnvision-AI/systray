package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/ReEnvision-AI/systray/app/power"
	"github.com/ReEnvision-AI/systray/app/store"
	"github.com/ReEnvision-AI/systray/app/tray"
	"github.com/ReEnvision-AI/systray/app/tray/commontray"
)

type AppState int

const (
	StateStopped AppState = iota
	StateStarting
	StateRunning
	StateStopping
	StateThankyou
	StateError
)

var (
	currentState AppState = StateStopped
	stateMu      sync.Mutex
	t            commontray.ReaiTray
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
		return "Please restart ReEnvision AI"
	case StateThankyou:
		return "Thank you!"
	default:
		return "Unknown"
	}
}

func Run() {
	InitLogging()
	slog.Info("ReEnvision AI app starting")

	updaterCtx, updaterCancel := context.WithCancel(context.Background())
	var updaterDone chan int

	var err error
	t, err = tray.NewTray()
	if err != nil {
		log.Fatalf("Failed to start: %s", err)
	}

	callbacks := t.GetCallbacks()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		slog.Debug("starting callback loop")
		for {
			select {
			case <-callbacks.Quit:
				slog.Debug("quit called")
				handleQuit()
			case <-signals:
				slog.Debug("shutting down due to signal")
				handleQuit()
			case <-callbacks.Update:
				err := DoUpgrade(updaterCancel, updaterDone)
				if err != nil {
					slog.Warn(fmt.Sprintf("upgrade attempt failed: %s", err))
				}
			case <-callbacks.ShowLogs:
				ShowLogs()
			case <-callbacks.StartContainer:
				// Start the container
				slog.Info("Starting container")
				handleStartRequest()
			case <-callbacks.StopContainer:
				// Stop the container
				slog.Info("Stopping container")
				handleStopRequest()
			case <-callbacks.DoFirstUse:
				err := GetStarted()
				if err != nil {
					slog.Warn(fmt.Sprintf("Failed to launch getting started shell: %s", err))
				}
			}
		}
	}()

	// Are we first use?
	if !store.GetFirstTimeRun() {
		slog.Debug("First time run")
		err = t.DisplayFirstUseNotification()
		if err != nil {
			slog.Debug(fmt.Sprintf("XXX failed to display first use notification %v", err))
		}
		store.SetFirstTimeRun(true)
	} else {
		slog.Debug("Not first time, skipping first run notification")
	}

	StartBackgroundUpdaterChecker(updaterCtx, t.UpdateAvailable)

	handleStartRequest()

	t.Run()

	updaterCancel()
	slog.Info("Waiting for app to shutdown..")
	if updaterDone != nil {
		<-updaterDone
	}

	slog.Info("ReEnvision AI app exiting")
}

func SetState(newState AppState) {
	stateMu.Lock()
	currentState = newState
	stateMu.Unlock()
	t.ChangeStatusText(newState.String())

	switch newState {
	case StateStopping, StateStopped, StateError:
		t.SetStopped()
		if err := power.AllowSleep(); err != nil && !errors.Is(err, power.ErrAlreadyAllowed) {
			slog.Warn("Failed to allow system sleep", "error", err)
		}

	case StateStarting, StateRunning:
		t.SetStarted()
		if err := power.PreventSleep(); err != nil && !errors.Is(err, power.ErrAlreadyPrevented) {
			slog.Warn("Failed to prevent system sleep", "error", err)
		}
	}
}

func handleStartRequest() {
	SetState(StateStarting)

	ctx := context.Background()

	err := StartContainer(ctx)
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to start container: %s", err))
		SetState(StateError)
		return
	}
}

func handleStopRequest() {
	SetState(StateStopping)
	ctx, cancel := context.WithTimeout(context.Background(), podmanStopTimeout)
	defer cancel()

	err := StopContainer(ctx)
	if err != nil {
		slog.Error("Failed to stop container process", "error", err)
		// Should we go to Error state or Stopped state? Let's assume Stopped for now.
		SetState(StateStopped)
		// Consider showing an error message
	} else {
		SetState(StateStopped) // Explicitly set to stopped on successful stop
	}
}

func handleQuit() {
	slog.Info("Quitting..")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), podmanStopTimeout+5*time.Second) // Give a bit extra time
	defer cancel()

	stateMu.Lock()
	shouldStop := currentState == StateRunning || currentState == StateStarting
	stateMu.Unlock()

	if shouldStop {
		slog.Info("Attempting graceful shutdown of container...")
		// This might block, so use the shutdown context
		err := StopContainer(shutdownCtx)
		if err != nil {
			slog.Error("Error during shutdown stop", "error", err)
		}
	}

	t.Quit()

	slog.Info("Finished exit procedures.")
}

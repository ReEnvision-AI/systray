package store

import (
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
)

type Store struct {
	ID           string `json:"id"`
	FirstTimeRun bool   `json:"first-time-run"`
}

var (
	lock  sync.Mutex
	store Store
)

func GetID() string {
	lock.Lock()
	defer lock.Unlock()
	if store.ID == "" {
		initStore()
	}
	return store.ID
}

func GetFirstTimeRun() bool {
	lock.Lock()
	defer lock.Unlock()
	if store.ID == "" {
		initStore()
	}
	return store.FirstTimeRun
}

func SetFirstTimeRun(val bool) {
	lock.Lock()
	defer lock.Unlock()
	if store.FirstTimeRun == val {
		return
	}
	store.FirstTimeRun = val
	writeStore(getStorePath())
}

func initStore() {
	storePath := getStorePath()
	storeFile, err := os.Open(storePath)
	if err == nil {
		defer storeFile.Close()
		if err = json.NewDecoder(storeFile).Decode(&store); err == nil {
			slog.Debug("loaded existing store", "path", storePath, "id", store.ID)
			return // Successfully loaded and decoded
		}
		// Decoding failed, file is likely corrupt
		slog.Warn("failed to decode store file, creating a new one", "path", storePath, "error", err)
	} else if !errors.Is(err, os.ErrNotExist) {
		// File could not be opened for a reason other than not existing
		slog.Warn("unexpected error opening store, creating a new one", "path", storePath, "error", err)
	}

	// If we get here, we need to create a new store
	slog.Debug("initializing new store")
	store.ID = uuid.NewString()
	writeStore(storePath)
}

func writeStore(storeFilename string) {
	reaiDir := filepath.Dir(storeFilename)
	_, err := os.Stat(reaiDir)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(reaiDir, 0o755); err != nil {
			slog.Error("failed to create dir", "path", reaiDir, "error", err)
			return
		}
	}

	payload, err := json.Marshal(store)
	if err != nil {
		slog.Error("failed to marshal store", "error", err)
		return
	}
	fp, err := os.OpenFile(storeFilename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		slog.Error("failed to write store", "path", storeFilename, "error", err)
		return
	}
	defer fp.Close()
	if n, err := fp.Write(payload); err != nil || n != len(payload) {
		slog.Error("failed to write store payload", "path", storeFilename, "bytes_written", n, "payload_length", len(payload), "error", err)
		return
	}

	slog.Debug("Store contents", "contents", string(payload))
	slog.Info("wrote store", "path", storeFilename)
}

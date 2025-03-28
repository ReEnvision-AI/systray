//go:build windows

package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/danieljoos/wincred" // For Windows Credential Manager access
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// AppConfig struct holds values loaded from config.json and Windows Credential Manager.
type AppConfig struct {
	ContainerName  string `json:"container_name"`
	ContainerImage string `json:"container_image"`
	InitialPeers   string `json:"initial_peers"`
	ModelName      string `json:"model_name"`
	DefaultPort    uint64 `json:"default_port"`
	UseGPU         bool   `json:"use_gpu"`
	Token          string // Loaded separately from Credential Manager
}

// LoadConfig reads the configuration file from the given path, parses it,
// validates required fields, and fetches the API token from Windows Credential Manager.
func LoadConfig(filePath string) (AppConfig, error) {
	var cfg AppConfig

	// --- Load from JSON file ---
	data, err := os.ReadFile(filePath)
	if err != nil {
		return cfg, fmt.Errorf("failed to read config file '%s': %w", filePath, err)
	}

	err = json.Unmarshal(data, &cfg)
	if err != nil {
		return cfg, fmt.Errorf("failed to parse config file '%s': %w", filePath, err)
	}

	// --- Validate required fields from JSON ---
	if cfg.ContainerName == "" || cfg.ContainerImage == "" || cfg.ModelName == "" {
		return cfg, fmt.Errorf("config file '%s' is missing required fields (container_name, container_image, model_name)", filePath)
	}

	if cfg.DefaultPort == 0 {
		slog.Warn("DefaultPort is zero in config, using fallback 31330", "filePath", filePath)
		cfg.DefaultPort = 31330 // Provide a default fallback
	}

	// --- Load Token from Windows Credential Manager ---
	targetName := "ReEnvisionAI/hf_token" // The target name used in Credential Manager

	cred, err := wincred.GetGenericCredential(targetName)
	if err != nil {
		// Check if the error specifically means the credential wasn't found
		if errors.Is(err, wincred.ErrElementNotFound) {
			// Return a specific error indicating the credential is missing
			return cfg, fmt.Errorf("credential '%s' not found in Windows Credential Manager. Please ensure it has been added: %w", targetName, err)
		}
		// Return other potential errors (e.g., access permissions)
		return cfg, fmt.Errorf("error retrieving credential '%s': %w", targetName, err)
	}

	// Decode the token from UTF-16LE (as stored by Windows) to UTF-8
	apiTokenBytesUTF16LE := cred.CredentialBlob
	utf16leDecoder := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewDecoder()

	apiTokenBytesUTF8, _, err := transform.Bytes(utf16leDecoder, apiTokenBytesUTF16LE)
	if err != nil {
		// Handle potential decoding errors
		return cfg, fmt.Errorf("error decoding token from UTF-16LE to UTF-8: %w", err)
	}

	cfg.Token = string(apiTokenBytesUTF8)
	slog.Debug(fmt.Sprintf("DEBUG: Successfully loaded and decoded token (starts with: %s***)", cfg.Token[:min(len(cfg.Token), 4)]))

	return cfg, nil
}

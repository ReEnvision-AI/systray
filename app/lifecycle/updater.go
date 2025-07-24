package lifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/ReEnvision-AI/systray/version"
)

var (
	UpdateCheckURLBase  = "https://sociallyshaped.net/api/update"
	UpdateDownloaded    = false
	UpdateCheckInterval = 24 * time.Hour
)

type UpdateResponse struct {
	UpdateURL     string `json:"url"`
	UpdateVersion string `json:"version"`
}

func IsNewReleaseAvailable(ctx context.Context) (bool, UpdateResponse) {
	var updateResp UpdateResponse

	requestURL, err := url.Parse(UpdateCheckURLBase)
	if err != nil {
		return false, updateResp
	}

	query := requestURL.Query()
	query.Add("os", runtime.GOOS)
	query.Add("arch", runtime.GOARCH)
	query.Add("version", version.Version)
	query.Add("ts", strconv.FormatInt(time.Now().Unix(), 10))

	//nonce, err := auth.NewNonce(rand.Reader, 16)
	//if err != nil {
	//	return false, updateResp
	//}

	//query.Add("nonce", nonce)
	requestURL.RawQuery = query.Encode()

	//data := []byte(fmt.Sprintf("%s,%s", http.MethodGet, requestURL.RequestURI()))
	//signature, err := auth.Sign(ctx, data)
	//if err != nil {
	//	return false, updateResp
	//}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		slog.Warn("failed to check for update", "error", err)
		return false, updateResp
	}
	//req.Header.Set("Authorization", signature)
	req.Header.Set("User-Agent", fmt.Sprintf("reai/%s (%s %s) Go/%s", version.Version, runtime.GOARCH, runtime.GOOS, runtime.Version()))

	slog.Debug("checking for available update", "requestURL", requestURL)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Warn("failed to check for update", "error", err)
		return false, updateResp
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		slog.Debug("check update response 204 (current version is up to date)")
		return false, updateResp
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Warn("failed to read body response", "error", err)
	}

	if resp.StatusCode != http.StatusOK {
		slog.Info("check update error", "status_code", resp.StatusCode, "body", string(body))
		return false, updateResp
	}
	err = json.Unmarshal(body, &updateResp)
	if err != nil {
		slog.Warn("malformed response checking for update", "error", err)
		return false, updateResp
	}

	if _, err := url.ParseRequestURI(updateResp.UpdateURL); err != nil {
		slog.Warn("malformed response checking for update", "error", fmt.Sprintf("update URL is not a valid URL: %s", err))
		return false, updateResp
	}

	// Extract the version string from the URL in the github release artifact path
	updateResp.UpdateVersion = path.Base(path.Dir(updateResp.UpdateURL))

	slog.Info("New update available at " + updateResp.UpdateURL)
	return true, updateResp
}

func DownloadNewRelease(ctx context.Context, updateResp UpdateResponse) error {
	// Do a head first to check etag info
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, updateResp.UpdateURL, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("error checking update: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status attempting to download update %d", resp.StatusCode)
	}
	resp.Body.Close()
	etag := strings.Trim(resp.Header.Get("etag"), "\"")
	if etag == "" {
		slog.Debug("no etag detected, falling back to filename based dedup")
		etag = "_"
	}
	filename := Installer
	_, params, err := mime.ParseMediaType(resp.Header.Get("content-disposition"))
	if err == nil {
		filename = params["filename"]
	}

	stageFilename := filepath.Join(UpdateStageDir, etag, filename)

	// Check to see if we already have it downloaded
	_, err = os.Stat(stageFilename)
	if err == nil {
		slog.Info("update already downloaded")
		return nil
	}

	cleanupOldDownloads()

	req.Method = http.MethodGet
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("error checking update: %w", err)
	}
	defer resp.Body.Close()
	etag = strings.Trim(resp.Header.Get("etag"), "\"")
	if etag == "" {
		slog.Debug("no etag detected, falling back to filename based dedup") // TODO probably can get rid of this redundant log
		etag = "_"
	}

	stageFilename = filepath.Join(UpdateStageDir, etag, filename)

	_, err = os.Stat(filepath.Dir(stageFilename))
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(stageFilename), 0o755); err != nil {
			return fmt.Errorf("create ReEnvision AI dir %s: %v", filepath.Dir(stageFilename), err)
		}
	}

	fp, err := os.OpenFile(stageFilename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("failed to create update file %s: %w", stageFilename, err)
	}
	defer fp.Close()

	// Stream the download directly to the file
	_, err = io.Copy(fp, resp.Body)
	if err != nil {
		// Clean up partially downloaded file on error
		os.Remove(stageFilename)
		return fmt.Errorf("failed to write update to %s: %w", stageFilename, err)
	}
	slog.Info("new update downloaded " + stageFilename)

	UpdateDownloaded = true
	return nil
}

func cleanupOldDownloads() {
	files, err := os.ReadDir(UpdateStageDir)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		// Expected behavior on first run
		return
	} else if err != nil {
		slog.Warn("failed to list stage dir", "error", err)
		return
	}
	for _, file := range files {
		fullname := filepath.Join(UpdateStageDir, file.Name())
		slog.Debug("cleaning up old download: " + fullname)
		err = os.RemoveAll(fullname)
		if err != nil {
			slog.Warn("failed to cleanup stale update download", "error", err)
		}
	}
}

func StartBackgroundUpdaterChecker(ctx context.Context, cb func(string) error) {
	go func() {
		// Don't blast an update message immediately after startup
		time.Sleep(30 * time.Second)

		for {
			available, resp := IsNewReleaseAvailable(ctx)
			if available {
				err := DownloadNewRelease(ctx, resp)
				if err != nil {
					slog.Error("failed to download new release", "error", err)
				}
				err = cb(resp.UpdateVersion)
				if err != nil {
					slog.Warn("failed to register update available with tray", "error", err)
				}
			}
			select {
			case <-ctx.Done():
				slog.Debug("stopping background update checker")
				return
			default:
				time.Sleep(UpdateCheckInterval)
			}
		}
	}()
}

package browser

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// DownloadURITemplate is the download URL pattern.
const DownloadURITemplate = "https://storage.googleapis.com/chrome-for-testing-public/%s/%s/%s.zip"

// DownloadProgress is a callback for progress reporting.
type DownloadProgress func(downloaded, total int64)

// Download fetches a Chromium zip to a temp file.
// Returns the path to the downloaded zip.
// Shows progress via the callback (called with 0,0 at start).
func Download(version string, plat Platform, progress DownloadProgress) (zipPath string, err error) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "3s-chromium-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	cleanupTemp := true
	defer func() {
		if cleanupTemp {
			_ = os.RemoveAll(tmpDir)
		}
	}()

	// Build download URL
	url := DownloadURL(version, plat)

	// HTTP client with timeout for large download
	client := &http.Client{Timeout: 5 * time.Minute}

	// HTTP GET
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("http get: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	total := resp.ContentLength

	// Signal start
	if progress != nil {
		progress(0, total)
	}

	// Create temp file
	zipPath = filepath.Join(tmpDir, "chromium.zip")
	out, err := os.Create(zipPath)
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer func() {
		if cerr := out.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("close temp file: %w", cerr)
		}
	}()

	// Download with progress
	buf := make([]byte, 32*1024) // 32KB buffer
	var downloaded int64
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			_, writeErr := out.Write(buf[:n])
			if writeErr != nil {
				return "", fmt.Errorf("write to temp file: %w", writeErr)
			}
			downloaded += int64(n)
			if progress != nil {
				progress(downloaded, total)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return "", fmt.Errorf("read response body: %w", readErr)
		}
	}

	cleanupTemp = false
	return zipPath, nil
}

// DownloadURL returns the full download URL for the given version and platform.
func DownloadURL(version string, plat Platform) string {
	return fmt.Sprintf(DownloadURITemplate, version, plat.Name, plat.ZipName)
}

package browser

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Extract unzips the downloaded Chromium zip into destDir.
// Returns the path to the chromium binary inside destDir.
func Extract(zipPath, destDir string, plat Platform) (binaryPath string, err error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf("open zip: %w", err)
	}
	defer func() { _ = reader.Close() }()

	// Zip entries are under "chrome-{platform}/", e.g., "chrome-linux64/"
	prefix := plat.ZipName + "/"

	for _, f := range reader.File {
		// Skip entries not under the platform directory
		if !strings.HasPrefix(f.Name, prefix) {
			continue
		}

		// Build destination path (keep original zip structure)
		destPath := filepath.Join(destDir, f.Name)

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(destPath, 0755); err != nil {
				return "", fmt.Errorf("create directory %s: %w", destPath, err)
			}
			continue
		}

		// Create parent directories
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return "", fmt.Errorf("create parent dir for %s: %w", destPath, err)
		}

		// Write file
		out, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return "", fmt.Errorf("create file %s: %w", destPath, err)
		}

		rc, err := f.Open()
		if err != nil {
			_ = out.Close()
			return "", fmt.Errorf("open zip entry %s: %w", f.Name, err)
		}

		_, err = io.Copy(out, rc)
		_ = out.Close()
		_ = rc.Close()
		if err != nil {
			return "", fmt.Errorf("write file %s: %w", destPath, err)
		}
	}

	// Verify binary exists
	binaryPath = filepath.Join(destDir, plat.Binary)
	if _, err := os.Stat(binaryPath); err != nil {
		return "", fmt.Errorf("binary not found after extraction at %s: %w", binaryPath, err)
	}

	// Make binary executable on unix-like systems
	if plat.Name != "win64" {
		if err := os.Chmod(binaryPath, 0755); err != nil {
			return "", fmt.Errorf("chmod binary: %w", err)
		}
	}

	return binaryPath, nil
}

// CacheDir returns the cache root for Chromium downloads: ~/.cache/3s/chromium/
func CacheDir() (string, error) {
	userCacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("get user cache dir: %w", err)
	}
	return filepath.Join(userCacheDir, "3s", "chromium"), nil
}

// VersionDir returns the per-version directory: ~/.cache/3s/chromium/<version>/
func VersionDir(version string) (string, error) {
	root, err := CacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, version), nil
}

// BinaryPath returns the expected binary location for a cached version.
func BinaryPath(version string, plat Platform) (string, error) {
	dir, err := VersionDir(version)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, plat.Binary), nil
}

// CachedBinary checks if a version is already downloaded and the binary exists.
// Returns the path and true if found, empty string and false otherwise.
func CachedBinary(version string, plat Platform) (string, bool, error) {
	dir, err := VersionDir(version)
	if err != nil {
		return "", false, err
	}

	binaryPath := filepath.Join(dir, plat.Binary)
	info, err := os.Stat(binaryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("stat binary: %w", err)
	}

	// Check it's not a directory and is executable (on unix-like)
	if info.IsDir() {
		return "", false, nil
	}

	return binaryPath, true, nil
}

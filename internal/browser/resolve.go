package browser

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fusoya59/3s/internal/config"
	"github.com/fusoya59/3s/internal/output"
)

// progressCb returns a DownloadProgress callback that prints a dot every ~10%.
func progressCb() DownloadProgress {
	var lastPct int

	return func(downloaded, total int64) {
		if total <= 0 {
			return
		}
		pct := int(downloaded * 100 / total)
		// Print a dot every 10% increment
		if pct/10 > lastPct/10 {
			output.Stderrf(".")
			lastPct = pct
		}
	}
}

// resolveCheckOnly checks existing sources without downloading.
// Returns the binary path, whether config update is needed, and whether found.
func resolveCheckOnly(cfgPath string, plat Platform) (binaryPath string, needsConfigUpdate bool, found bool) {
	// 1. Check config path
	if cfgPath != "" {
		cfg, loadErr := config.Load(cfgPath)
		if loadErr == nil && cfg.BrowserBinPath != "" {
			if info, statErr := os.Stat(cfg.BrowserBinPath); statErr == nil && !info.IsDir() {
				return cfg.BrowserBinPath, false, true
			}
		}
	}

	// Get current version for cache check
	version := FallbackVersion
	resolvedVersion, vErr := ResolveVersion()
	if vErr == nil {
		version = resolvedVersion
	}

	// 2. Check cache
	if binPath, found, cacheErr := CachedBinary(version, plat); cacheErr == nil && found {
		return binPath, true, true
	}

	// 3. Check system
	if sysPath, found := CheckSystem(); found {
		return sysPath, true, true
	}

	return "", false, false
}

// ResolveExisting checks for an existing Chromium binary without downloading.
// Order: config path → cached → system.
// Returns error only if platform is unsupported.
// If not found, returns empty path with no error — caller decides fallback.
func ResolveExisting(cfgPath string) (binaryPath string, err error) {
	plat, err := DetectPlatform()
	if err != nil {
		return "", fmt.Errorf("detect platform: %w", err)
	}

	binPath, _, found := resolveCheckOnly(cfgPath, plat)
	if !found {
		return "", nil
	}
	return binPath, nil
}

// Resolve finds or downloads a Chromium binary.
// Order: config path → cached → system → download.
// If download happens, returns the new path and sets needsConfigUpdate=true.
func Resolve(cfgPath, cacheRoot string) (binaryPath string, needsConfigUpdate bool, err error) {
	// Detect platform
	plat, err := DetectPlatform()
	if err != nil {
		return "", false, fmt.Errorf("detect platform: %w", err)
	}

	// Resolve version
	version := FallbackVersion
	resolvedVersion, vErr := ResolveVersion()
	if vErr == nil {
		version = resolvedVersion
	}

	// Check existing sources first
	if binPath, needsUpdate, found := resolveCheckOnly(cfgPath, plat); found {
		return binPath, needsUpdate, nil
	}

	// 4. Download
	output.Stderrf("Downloading Chromium %s (%s)", version, plat.Display)
	output.Stderr("") // newline after the above

	// Download with progress
	zipPath, err := Download(version, plat, progressCb())
	if err != nil {
		return "", false, fmt.Errorf("download chromium: %w", err)
	}
	output.Stderr("") // newline after progress dots

	// Clean up temp zip on any subsequent error or return
	zipTempDir := filepath.Dir(zipPath)
	defer func() { _ = os.RemoveAll(zipTempDir) }()

	// Create version directory
	versionDir, err := VersionDir(version)
	if err != nil {
		return "", false, fmt.Errorf("version dir: %w", err)
	}

	// Clean any previous partial extraction
	if _, statErr := os.Stat(versionDir); statErr == nil {
		if err := os.RemoveAll(versionDir); err != nil {
			return "", false, fmt.Errorf("clean previous version dir: %w", err)
		}
	}

	if err := os.MkdirAll(versionDir, 0755); err != nil {
		return "", false, fmt.Errorf("create version dir %s: %w (check permissions on %s)", versionDir, err, filepath.Dir(versionDir))
	}

	// Extract
	output.Stderrf("Extracting...")
	binPath, err := Extract(zipPath, versionDir, plat)
	if err != nil {
		return "", false, fmt.Errorf("extract chromium: %w", err)
	}
	output.Stderrf(" done\n")

	// Verify binary
	if _, err := os.Stat(binPath); err != nil {
		return "", false, fmt.Errorf("verify binary after extraction: %w", err)
	}

	// Write version file
	cacheRoot2, _ := CacheDir()
	if cacheRoot2 != "" {
		if err := os.MkdirAll(cacheRoot2, 0755); err == nil {
			_ = WriteCachedVersion(cacheRoot2, version)
		}
	}

	// Small delay to let OS settle file permissions
	time.Sleep(100 * time.Millisecond)

	return binPath, true, nil
}

// ResolveWithConfig is a convenience wrapper that loads config, resolves,
// and returns results. Caller uses the returned path.
func ResolveWithConfig(resolvedCfgPath string) (binaryPath string, needsConfigUpdate bool, err error) {
	return Resolve(resolvedCfgPath, "")
}

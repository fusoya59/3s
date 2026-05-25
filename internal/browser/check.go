package browser

import (
	"os"
	"os/exec"
)

// BinaryNames is the list of Chromium binary names to search on PATH.
var BinaryNames = []string{
	"google-chrome-stable",
	"google-chrome",
	"chromium-browser",
	"chromium",
	"chrome",
}

// CommonPaths is the list of common absolute Chromium paths.
var CommonPaths = []string{
	"/usr/bin/google-chrome-stable",
	"/usr/bin/google-chrome",
	"/usr/bin/chromium-browser",
	"/usr/bin/chromium",
	"/snap/bin/chromium",
	// macOS
	"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
	"/Applications/Chromium.app/Contents/MacOS/Chromium",
	// Windows (empty string skipped on non-Windows)
	`C:\Program Files\Google\Chrome\Application\chrome.exe`,
	`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
	`C:\Program Files\Chromium\Application\chrome.exe`,
}

// CheckSystem searches PATH and common paths for an existing Chromium.
// Returns the path and true if found.
func CheckSystem() (string, bool) {
	// Try PATH first
	for _, name := range BinaryNames {
		if p, err := exec.LookPath(name); err == nil {
			return p, true
		}
	}

	// Try common absolute paths
	for _, p := range CommonPaths {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			return p, true
		}
	}

	return "", false
}

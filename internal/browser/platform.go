package browser

import (
	"fmt"
	"runtime"
)

// Platform represents a Chrome for Testing platform identifier.
type Platform struct {
	Name    string // e.g., "linux64", "mac-arm64", "win64"
	Binary  string // relative path to binary within zip, e.g., "chrome-linux64/chrome"
	ZipName string // e.g., "chrome-linux64", "chrome-mac-arm64", "chrome-win64"
	Display string // human-readable, e.g., "Linux x64", "macOS ARM64", "Windows x64"
}

// DetectPlatform returns the Platform for the current OS/arch.
// Returns error for unsupported platforms (linux/arm64, darwin/386, etc.).
func DetectPlatform() (Platform, error) {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	switch {
	case osName == "linux" && arch == "amd64":
		return Platform{
			Name:    "linux64",
			Binary:  "chrome-linux64/chrome",
			ZipName: "chrome-linux64",
			Display: "Linux x64",
		}, nil
	case osName == "darwin" && arch == "amd64":
		return Platform{
			Name:    "mac-x64",
			Binary:  "chrome-mac-x64/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing",
			ZipName: "chrome-mac-x64",
			Display: "macOS x64",
		}, nil
	case osName == "darwin" && arch == "arm64":
		return Platform{
			Name:    "mac-arm64",
			Binary:  "chrome-mac-arm64/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing",
			ZipName: "chrome-mac-arm64",
			Display: "macOS ARM64",
		}, nil
	case osName == "windows" && arch == "amd64":
		return Platform{
			Name:    "win64",
			Binary:  "chrome-win64/chrome.exe",
			ZipName: "chrome-win64",
			Display: "Windows x64",
		}, nil
	default:
		return Platform{}, fmt.Errorf("unsupported platform: %s/%s", osName, arch)
	}
}

// AllPlatforms returns supported platforms for testing.
func AllPlatforms() []Platform {
	return []Platform{
		{
			Name:    "linux64",
			Binary:  "chrome-linux64/chrome",
			ZipName: "chrome-linux64",
			Display: "Linux x64",
		},
		{
			Name:    "mac-x64",
			Binary:  "chrome-mac-x64/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing",
			ZipName: "chrome-mac-x64",
			Display: "macOS x64",
		},
		{
			Name:    "mac-arm64",
			Binary:  "chrome-mac-arm64/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing",
			ZipName: "chrome-mac-arm64",
			Display: "macOS ARM64",
		},
		{
			Name:    "win64",
			Binary:  "chrome-win64/chrome.exe",
			ZipName: "chrome-win64",
			Display: "Windows x64",
		},
	}
}

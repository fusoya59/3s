package browser

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// DefaultMilestone is the Chromium milestone we target.
const DefaultMilestone = "150"

// FallbackVersion used when the API is unreachable.
var FallbackVersion = "150.0.7858.0"

// LatestVersionsURL is the Chrome for Testing API endpoint.
const LatestVersionsURL = "https://googlechromelabs.github.io/chrome-for-testing/latest-versions-per-milestone.json"

// knownGoodVersionsResponse is the JSON shape from the API endpoint.
type knownGoodVersionsResponse struct {
	Milestones map[string]struct {
		Version  string `json:"version"`
		Revision string `json:"revision"`
	} `json:"milestones"`
}

// ResolveVersion fetches the latest patch version for the pinned milestone.
// Falls back to FallbackVersion if API unreachable.
func ResolveVersion() (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(LatestVersionsURL)
	if err != nil {
		return FallbackVersion, nil
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return FallbackVersion, nil
	}

	var data knownGoodVersionsResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return FallbackVersion, nil
	}

	entry, ok := data.Milestones[DefaultMilestone]
	if !ok || entry.Version == "" {
		return FallbackVersion, nil
	}

	return entry.Version, nil
}

// VersionFile returns the path to the version.txt file in the cache directory.
func VersionFile(cacheDir string) string {
	return cacheDir + "/version.txt"
}

// ReadCachedVersion reads the version string from the cache's version.txt.
func ReadCachedVersion(cacheDir string) (string, error) {
	data, err := os.ReadFile(VersionFile(cacheDir))
	if err != nil {
		return "", fmt.Errorf("read version file: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// WriteCachedVersion writes the version string to the cache's version.txt.
func WriteCachedVersion(cacheDir, version string) error {
	return os.WriteFile(VersionFile(cacheDir), []byte(version+"\n"), 0644)
}

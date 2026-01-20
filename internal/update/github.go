package update

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

const (
	// Public repo: https://github.com/pushchain/push-validator-cli
	githubOwner      = "pushchain"
	githubRepo       = "push-validator-cli"
	latestReleaseURL = "https://api.github.com/repos/pushchain/push-validator-cli/releases/latest"
	releasesURL      = "https://api.github.com/repos/pushchain/push-validator-cli/releases"
	releaseByTagURL  = "https://api.github.com/repos/pushchain/push-validator-cli/releases/tags/%s"

	httpTimeout = 30 * time.Second
)

// FetchLatestRelease gets the latest release from GitHub
func FetchLatestRelease() (*Release, error) {
	client := &http.Client{Timeout: httpTimeout}

	req, err := http.NewRequest("GET", latestReleaseURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "push-validator-cli")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("no releases found")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API error: %s", resp.Status)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release: %w", err)
	}

	return &release, nil
}

// FetchReleaseByTag gets a specific release by tag
func FetchReleaseByTag(tag string) (*Release, error) {
	// Ensure tag has 'v' prefix
	if !strings.HasPrefix(tag, "v") {
		tag = "v" + tag
	}

	client := &http.Client{Timeout: httpTimeout}
	url := fmt.Sprintf(releaseByTagURL, tag)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "push-validator-cli")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("release %s not found", tag)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API error: %s", resp.Status)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release: %w", err)
	}

	return &release, nil
}

// GetAssetForPlatform finds the correct binary for current OS/arch
func GetAssetForPlatform(release *Release) (*Asset, error) {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	// Expected format: push-validator_1.0.0_linux_amd64.tar.gz
	pattern := "push-validator_"
	suffix := fmt.Sprintf("_%s_%s.tar.gz", osName, arch)

	for i := range release.Assets {
		asset := &release.Assets[i]
		if strings.HasPrefix(asset.Name, pattern) && strings.HasSuffix(asset.Name, suffix) {
			return asset, nil
		}
	}

	return nil, fmt.Errorf("no binary found for %s/%s in release %s", osName, arch, release.TagName)
}

// GetChecksumAsset finds the checksums.txt asset
func GetChecksumAsset(release *Release) (*Asset, error) {
	for i := range release.Assets {
		asset := &release.Assets[i]
		if asset.Name == "checksums.txt" {
			return asset, nil
		}
	}
	return nil, fmt.Errorf("checksums.txt not found in release")
}

// IsNewerVersion returns true if latest is newer than current
func IsNewerVersion(current, latest string) bool {
	// Ensure both have 'v' prefix for semver comparison
	if !strings.HasPrefix(current, "v") {
		current = "v" + current
	}
	if !strings.HasPrefix(latest, "v") {
		latest = "v" + latest
	}

	// Handle "dev" or "unknown" versions
	if !semver.IsValid(current) {
		return true // Always update from dev builds
	}
	if !semver.IsValid(latest) {
		return false
	}

	return semver.Compare(latest, current) > 0
}

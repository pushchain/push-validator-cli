package update

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const (
	cacheFileName = ".update-check"
	cacheDuration = 10 * time.Minute
)

// CacheEntry stores the last update check result
type CacheEntry struct {
	CheckedAt       time.Time `json:"checked_at"`
	LatestVersion   string    `json:"latest_version"`
	UpdateAvailable bool      `json:"update_available"`
}

// GetCachePath returns the path to the cache file
func GetCachePath(homeDir string) string {
	return filepath.Join(homeDir, cacheFileName)
}

// LoadCache loads the cached update check result
func LoadCache(homeDir string) (*CacheEntry, error) {
	path := GetCachePath(homeDir)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, err
	}

	return &entry, nil
}

// SaveCache saves the update check result
func SaveCache(homeDir string, entry *CacheEntry) error {
	path := GetCachePath(homeDir)
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// IsCacheValid returns true if cache is fresh (< 10m old)
func IsCacheValid(entry *CacheEntry) bool {
	return time.Since(entry.CheckedAt) < cacheDuration
}

// ForceCheck performs a fresh update check, ignoring cache.
// Used by status and dashboard commands for immediate notification.
// Updates the cache after checking.
func ForceCheck(homeDir, currentVersion string) (*CheckResult, error) {
	updater, err := New(currentVersion)
	if err != nil {
		return nil, err
	}

	result, err := updater.Check()
	if err != nil {
		return nil, err
	}

	// Update cache with fresh result
	_ = SaveCache(homeDir, &CacheEntry{
		CheckedAt:       time.Now(),
		LatestVersion:   result.LatestVersion,
		UpdateAvailable: result.UpdateAvailable,
	})

	return result, nil
}

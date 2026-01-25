package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/pushchain/push-validator-cli/internal/update"
)

// mockUpdateChecker implements updateChecker for testing.
type mockUpdateChecker struct {
	result *update.CheckResult
	err    error
}

func (m *mockUpdateChecker) Check() (*update.CheckResult, error) {
	return m.result, m.err
}

func TestCheckForUpdateWith_CacheValid_UpdateAvailable(t *testing.T) {
	loadCache := func(homeDir string) (*update.CacheEntry, error) {
		return &update.CacheEntry{
			CheckedAt:       time.Now().Add(-5 * time.Minute), // Within 10-minute cache TTL
			LatestVersion:   "2.0.0",
			UpdateAvailable: true,
		}, nil
	}
	saveCache := func(homeDir string, entry *update.CacheEntry) error { return nil }
	newUpdater := func(version string) (updateChecker, error) {
		t.Fatal("should not create updater when cache is valid")
		return nil, nil
	}

	result := checkForUpdateWith("/tmp/test", "v1.0.0", loadCache, saveCache, newUpdater)
	if result == nil {
		t.Fatal("expected non-nil result for cached update")
	}
	if result.LatestVersion != "2.0.0" {
		t.Errorf("LatestVersion = %q, want %q", result.LatestVersion, "2.0.0")
	}
	if !result.UpdateAvailable {
		t.Error("expected UpdateAvailable=true")
	}
}

func TestCheckForUpdateWith_CacheValid_NoUpdate(t *testing.T) {
	loadCache := func(homeDir string) (*update.CacheEntry, error) {
		return &update.CacheEntry{
			CheckedAt:       time.Now().Add(-5 * time.Minute), // Within 10-minute cache TTL
			LatestVersion:   "1.0.0",
			UpdateAvailable: false,
		}, nil
	}
	saveCache := func(homeDir string, entry *update.CacheEntry) error { return nil }
	newUpdater := func(version string) (updateChecker, error) {
		t.Fatal("should not create updater when cache is valid")
		return nil, nil
	}

	result := checkForUpdateWith("/tmp/test", "v1.0.0", loadCache, saveCache, newUpdater)
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
}

func TestCheckForUpdateWith_CacheError_UpdaterCreationFails(t *testing.T) {
	loadCache := func(homeDir string) (*update.CacheEntry, error) {
		return nil, fmt.Errorf("cache file not found")
	}
	saveCache := func(homeDir string, entry *update.CacheEntry) error { return nil }
	newUpdater := func(version string) (updateChecker, error) {
		return nil, fmt.Errorf("failed to create updater")
	}

	result := checkForUpdateWith("/tmp/test", "v1.0.0", loadCache, saveCache, newUpdater)
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
}

func TestCheckForUpdateWith_CacheError_CheckFails(t *testing.T) {
	loadCache := func(homeDir string) (*update.CacheEntry, error) {
		return nil, fmt.Errorf("cache error")
	}
	saveCache := func(homeDir string, entry *update.CacheEntry) error { return nil }
	newUpdater := func(version string) (updateChecker, error) {
		return &mockUpdateChecker{err: fmt.Errorf("network error")}, nil
	}

	result := checkForUpdateWith("/tmp/test", "v1.0.0", loadCache, saveCache, newUpdater)
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
}

func TestCheckForUpdateWith_CacheError_UpdateAvailable(t *testing.T) {
	var savedEntry *update.CacheEntry
	loadCache := func(homeDir string) (*update.CacheEntry, error) {
		return nil, fmt.Errorf("no cache")
	}
	saveCache := func(homeDir string, entry *update.CacheEntry) error {
		savedEntry = entry
		return nil
	}
	newUpdater := func(version string) (updateChecker, error) {
		return &mockUpdateChecker{
			result: &update.CheckResult{
				CurrentVersion:  "1.0.0",
				LatestVersion:   "2.0.0",
				UpdateAvailable: true,
			},
		}, nil
	}

	result := checkForUpdateWith("/tmp/test", "v1.0.0", loadCache, saveCache, newUpdater)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.LatestVersion != "2.0.0" {
		t.Errorf("LatestVersion = %q, want %q", result.LatestVersion, "2.0.0")
	}
	if savedEntry == nil {
		t.Fatal("expected cache to be saved")
	}
	if savedEntry.LatestVersion != "2.0.0" {
		t.Errorf("cache LatestVersion = %q, want %q", savedEntry.LatestVersion, "2.0.0")
	}
}

func TestCheckForUpdateWith_CacheError_NoUpdateAvailable(t *testing.T) {
	loadCache := func(homeDir string) (*update.CacheEntry, error) {
		return nil, fmt.Errorf("no cache")
	}
	saveCache := func(homeDir string, entry *update.CacheEntry) error { return nil }
	newUpdater := func(version string) (updateChecker, error) {
		return &mockUpdateChecker{
			result: &update.CheckResult{
				CurrentVersion:  "1.0.0",
				LatestVersion:   "1.0.0",
				UpdateAvailable: false,
			},
		}, nil
	}

	result := checkForUpdateWith("/tmp/test", "v1.0.0", loadCache, saveCache, newUpdater)
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
}

func TestCheckForUpdateWith_CacheValid_SameVersion(t *testing.T) {
	// Cache says update available but we're now on the latest version
	loadCache := func(homeDir string) (*update.CacheEntry, error) {
		return &update.CacheEntry{
			CheckedAt:       time.Now().Add(-5 * time.Minute), // Within 10-minute cache TTL
			LatestVersion:   "1.0.0",
			UpdateAvailable: true,
		}, nil
	}
	saveCache := func(homeDir string, entry *update.CacheEntry) error { return nil }
	newUpdater := func(version string) (updateChecker, error) {
		t.Fatal("should not create updater when cache is valid")
		return nil, nil
	}

	// IsNewerVersion("v1.0.0", "1.0.0") should be false
	result := checkForUpdateWith("/tmp/test", "v1.0.0", loadCache, saveCache, newUpdater)
	if result != nil {
		t.Errorf("expected nil (same version), got %+v", result)
	}
}

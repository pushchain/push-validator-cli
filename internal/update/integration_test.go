// +build integration

package update

import (
	"testing"
)

// Integration tests for network-dependent functions
// Run with: go test -tags=integration ./internal/update/...
//
// These tests require internet connectivity and access to GitHub API.
// They are skipped in normal test runs.

func TestFetchLatestRelease_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	release, err := FetchLatestRelease()
	if err != nil {
		t.Fatalf("FetchLatestRelease() error = %v", err)
	}

	if release == nil {
		t.Fatal("FetchLatestRelease() returned nil release")
	}

	if release.TagName == "" {
		t.Error("Release TagName is empty")
	}

	if len(release.Assets) == 0 {
		t.Error("Release has no assets")
	}

	t.Logf("Latest release: %s", release.TagName)
}

func TestFetchReleaseByTag_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test requires a known release tag to exist
	// Update this tag based on actual releases
	testTag := "v0.1.0"

	release, err := FetchReleaseByTag(testTag)
	if err != nil {
		t.Logf("Note: Release %s may not exist yet: %v", testTag, err)
		t.Skipf("Skipping test for non-existent release: %v", err)
	}

	if release == nil {
		t.Fatal("FetchReleaseByTag() returned nil release")
	}

	if release.TagName != testTag && release.TagName != "v"+testTag {
		t.Errorf("Release TagName = %q, want %q or %q", release.TagName, testTag, "v"+testTag)
	}

	t.Logf("Found release: %s", release.TagName)
}

func TestCheck_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	u, err := NewUpdater("0.0.1")
	if err != nil {
		t.Fatalf("NewUpdater() error = %v", err)
	}

	result, err := u.Check()
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if result == nil {
		t.Fatal("Check() returned nil result")
	}

	if result.CurrentVersion != "0.0.1" {
		t.Errorf("CurrentVersion = %q, want %q", result.CurrentVersion, "0.0.1")
	}

	if result.LatestVersion == "" {
		t.Error("LatestVersion is empty")
	}

	t.Logf("Current: %s, Latest: %s, UpdateAvailable: %v",
		result.CurrentVersion, result.LatestVersion, result.UpdateAvailable)
}

func TestFullUpdateWorkflow_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test demonstrates the full update workflow
	// but does NOT actually install to avoid modifying the system

	t.Log("Step 1: Create updater")
	u, err := NewUpdater("0.0.1")
	if err != nil {
		t.Fatalf("NewUpdater() error = %v", err)
	}

	t.Log("Step 2: Check for updates")
	result, err := u.Check()
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if !result.UpdateAvailable {
		t.Skip("No update available, skipping download test")
	}

	t.Log("Step 3: Get platform-specific asset")
	asset, err := GetAssetForPlatform(result.Release)
	if err != nil {
		t.Fatalf("GetAssetForPlatform() error = %v", err)
	}

	t.Logf("Asset: %s (%d bytes)", asset.Name, asset.Size)

	t.Log("Step 4: Download binary (with progress)")
	progressCalled := false
	data, err := u.Download(asset, func(downloaded, total int64) {
		progressCalled = true
		if downloaded%100000 == 0 || downloaded == total {
			t.Logf("Progress: %d/%d bytes (%.1f%%)",
				downloaded, total, float64(downloaded)/float64(total)*100)
		}
	})
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}

	if !progressCalled {
		t.Error("Progress callback was never called")
	}

	if len(data) == 0 {
		t.Error("Downloaded data is empty")
	}

	t.Log("Step 5: Verify checksum")
	err = u.VerifyChecksum(data, result.Release, asset.Name)
	if err != nil {
		t.Fatalf("VerifyChecksum() error = %v", err)
	}

	t.Log("Step 6: Extract binary")
	binaryData, err := u.ExtractBinary(data)
	if err != nil {
		t.Fatalf("ExtractBinary() error = %v", err)
	}

	if len(binaryData) == 0 {
		t.Error("Extracted binary data is empty")
	}

	t.Logf("Successfully extracted binary (%d bytes)", len(binaryData))

	// Note: We do NOT call Install() here to avoid modifying the actual binary
	t.Log("Step 7: Install (SKIPPED in test)")
	t.Log("Full workflow completed successfully!")
}

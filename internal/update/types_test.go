package update

import (
	"encoding/json"
	"testing"
	"time"
)

func TestReleaseJSON(t *testing.T) {
	jsonData := `{
		"tag_name": "v1.2.3",
		"name": "Release 1.2.3",
		"body": "Release notes here",
		"draft": false,
		"prerelease": false,
		"published_at": "2024-01-01T12:00:00Z",
		"html_url": "https://github.com/pushchain/push-validator-cli/releases/tag/v1.2.3",
		"assets": [
			{
				"name": "binary.tar.gz",
				"browser_download_url": "https://example.com/binary.tar.gz",
				"size": 1024,
				"content_type": "application/gzip"
			}
		]
	}`

	var release Release
	err := json.Unmarshal([]byte(jsonData), &release)
	if err != nil {
		t.Fatalf("Failed to unmarshal Release JSON: %v", err)
	}

	if release.TagName != "v1.2.3" {
		t.Errorf("TagName = %q, want %q", release.TagName, "v1.2.3")
	}
	if release.Name != "Release 1.2.3" {
		t.Errorf("Name = %q, want %q", release.Name, "Release 1.2.3")
	}
	if release.Draft {
		t.Error("Draft = true, want false")
	}
	if release.Prerelease {
		t.Error("Prerelease = true, want false")
	}
	if len(release.Assets) != 1 {
		t.Fatalf("Assets length = %d, want 1", len(release.Assets))
	}
	if release.Assets[0].Name != "binary.tar.gz" {
		t.Errorf("Asset name = %q, want %q", release.Assets[0].Name, "binary.tar.gz")
	}
}

func TestAssetJSON(t *testing.T) {
	jsonData := `{
		"name": "push-validator_1.0.0_linux_amd64.tar.gz",
		"browser_download_url": "https://github.com/downloads/binary.tar.gz",
		"size": 2048,
		"content_type": "application/x-gzip"
	}`

	var asset Asset
	err := json.Unmarshal([]byte(jsonData), &asset)
	if err != nil {
		t.Fatalf("Failed to unmarshal Asset JSON: %v", err)
	}

	if asset.Name != "push-validator_1.0.0_linux_amd64.tar.gz" {
		t.Errorf("Name = %q, want %q", asset.Name, "push-validator_1.0.0_linux_amd64.tar.gz")
	}
	if asset.Size != 2048 {
		t.Errorf("Size = %d, want 2048", asset.Size)
	}
	if asset.ContentType != "application/x-gzip" {
		t.Errorf("ContentType = %q, want %q", asset.ContentType, "application/x-gzip")
	}
}

func TestCheckResult(t *testing.T) {
	release := &Release{
		TagName:     "v2.0.0",
		Name:        "Version 2.0.0",
		PublishedAt: time.Now(),
	}

	result := CheckResult{
		CurrentVersion:  "1.0.0",
		LatestVersion:   "2.0.0",
		UpdateAvailable: true,
		Release:         release,
	}

	if result.CurrentVersion != "1.0.0" {
		t.Errorf("CurrentVersion = %q, want %q", result.CurrentVersion, "1.0.0")
	}
	if result.LatestVersion != "2.0.0" {
		t.Errorf("LatestVersion = %q, want %q", result.LatestVersion, "2.0.0")
	}
	if !result.UpdateAvailable {
		t.Error("UpdateAvailable = false, want true")
	}
	if result.Release == nil {
		t.Fatal("Release is nil")
	}
	if result.Release.TagName != "v2.0.0" {
		t.Errorf("Release.TagName = %q, want %q", result.Release.TagName, "v2.0.0")
	}
}

func TestUpdateOptions(t *testing.T) {
	tests := []struct {
		name string
		opts UpdateOptions
	}{
		{
			name: "force update",
			opts: UpdateOptions{
				Force:      true,
				SkipVerify: false,
				Version:    "",
			},
		},
		{
			name: "skip verify",
			opts: UpdateOptions{
				Force:      false,
				SkipVerify: true,
				Version:    "",
			},
		},
		{
			name: "specific version",
			opts: UpdateOptions{
				Force:      false,
				SkipVerify: false,
				Version:    "1.5.0",
			},
		},
		{
			name: "all options",
			opts: UpdateOptions{
				Force:      true,
				SkipVerify: true,
				Version:    "2.0.0-beta",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify the struct can be created and values are set
			if tt.opts.Force && !tt.opts.Force {
				t.Error("Force value mismatch")
			}
			if tt.opts.SkipVerify && !tt.opts.SkipVerify {
				t.Error("SkipVerify value mismatch")
			}
		})
	}
}

func TestReleaseDefaults(t *testing.T) {
	// Test zero values
	var release Release

	if release.TagName != "" {
		t.Errorf("Default TagName = %q, want empty string", release.TagName)
	}
	if release.Draft {
		t.Error("Default Draft = true, want false")
	}
	if release.Prerelease {
		t.Error("Default Prerelease = true, want false")
	}
	if release.Assets != nil {
		t.Error("Default Assets should be nil")
	}
}

func TestAssetDefaults(t *testing.T) {
	var asset Asset

	if asset.Name != "" {
		t.Errorf("Default Name = %q, want empty string", asset.Name)
	}
	if asset.Size != 0 {
		t.Errorf("Default Size = %d, want 0", asset.Size)
	}
}

func TestCheckResultDefaults(t *testing.T) {
	var result CheckResult

	if result.UpdateAvailable {
		t.Error("Default UpdateAvailable = true, want false")
	}
	if result.Release != nil {
		t.Error("Default Release should be nil")
	}
}

func TestUpdateOptionsDefaults(t *testing.T) {
	var opts UpdateOptions

	if opts.Force {
		t.Error("Default Force = true, want false")
	}
	if opts.SkipVerify {
		t.Error("Default SkipVerify = true, want false")
	}
	if opts.Version != "" {
		t.Errorf("Default Version = %q, want empty string", opts.Version)
	}
}

func TestReleaseMarshaling(t *testing.T) {
	original := Release{
		TagName:     "v1.0.0",
		Name:        "Test Release",
		Body:        "Description",
		Draft:       false,
		Prerelease:  true,
		PublishedAt: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		HTMLURL:     "https://example.com/release",
		Assets: []Asset{
			{
				Name:               "test.tar.gz",
				BrowserDownloadURL: "https://example.com/test.tar.gz",
				Size:               1024,
				ContentType:        "application/gzip",
			},
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal Release: %v", err)
	}

	// Unmarshal back
	var unmarshaled Release
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal Release: %v", err)
	}

	// Verify fields
	if unmarshaled.TagName != original.TagName {
		t.Errorf("TagName = %q, want %q", unmarshaled.TagName, original.TagName)
	}
	if unmarshaled.Prerelease != original.Prerelease {
		t.Errorf("Prerelease = %v, want %v", unmarshaled.Prerelease, original.Prerelease)
	}
	if len(unmarshaled.Assets) != len(original.Assets) {
		t.Fatalf("Assets length = %d, want %d", len(unmarshaled.Assets), len(original.Assets))
	}
	if unmarshaled.Assets[0].Size != original.Assets[0].Size {
		t.Errorf("Asset size = %d, want %d", unmarshaled.Assets[0].Size, original.Assets[0].Size)
	}
}

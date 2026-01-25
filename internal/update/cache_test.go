package update

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGetCachePath(t *testing.T) {
	tests := []struct {
		name    string
		homeDir string
		want    string
	}{
		{
			name:    "unix path",
			homeDir: "/home/user",
			want:    "/home/user/.update-check",
		},
		{
			name:    "windows path",
			homeDir: "C:\\Users\\user",
			want:    filepath.Join("C:\\Users\\user", ".update-check"),
		},
		{
			name:    "relative path",
			homeDir: ".",
			want:    filepath.Join(".", ".update-check"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetCachePath(tt.homeDir)
			if got != tt.want {
				t.Errorf("GetCachePath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSaveAndLoadCache(t *testing.T) {
	homeDir := t.TempDir()

	// Create test cache entry
	original := &CacheEntry{
		CheckedAt:       time.Now().Truncate(time.Second), // Truncate for JSON precision
		LatestVersion:   "1.2.3",
		UpdateAvailable: true,
	}

	// Save cache
	err := SaveCache(homeDir, original)
	if err != nil {
		t.Fatalf("SaveCache() error = %v", err)
	}

	// Verify file exists
	cachePath := GetCachePath(homeDir)
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		t.Fatal("Cache file was not created")
	}

	// Load cache
	loaded, err := LoadCache(homeDir)
	if err != nil {
		t.Fatalf("LoadCache() error = %v", err)
	}

	// Compare values
	if !loaded.CheckedAt.Equal(original.CheckedAt) {
		t.Errorf("CheckedAt = %v, want %v", loaded.CheckedAt, original.CheckedAt)
	}
	if loaded.LatestVersion != original.LatestVersion {
		t.Errorf("LatestVersion = %v, want %v", loaded.LatestVersion, original.LatestVersion)
	}
	if loaded.UpdateAvailable != original.UpdateAvailable {
		t.Errorf("UpdateAvailable = %v, want %v", loaded.UpdateAvailable, original.UpdateAvailable)
	}
}

func TestLoadCache_NotExists(t *testing.T) {
	homeDir := t.TempDir()

	// Try to load cache from non-existent file
	_, err := LoadCache(homeDir)
	if err == nil {
		t.Fatal("LoadCache() expected error, got nil")
	}
	if !os.IsNotExist(err) {
		t.Errorf("LoadCache() error type = %T, want os.PathError", err)
	}
}

func TestLoadCache_InvalidJSON(t *testing.T) {
	homeDir := t.TempDir()
	cachePath := GetCachePath(homeDir)

	// Write invalid JSON
	err := os.WriteFile(cachePath, []byte("invalid json {"), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Try to load
	_, err = LoadCache(homeDir)
	if err == nil {
		t.Fatal("LoadCache() expected error for invalid JSON, got nil")
	}
}

func TestIsCacheValid(t *testing.T) {
	// Note: cacheDuration is 10 minutes
	tests := []struct {
		name      string
		checkedAt time.Time
		want      bool
	}{
		{
			name:      "fresh cache - just now",
			checkedAt: time.Now(),
			want:      true,
		},
		{
			name:      "fresh cache - 5 minutes ago",
			checkedAt: time.Now().Add(-5 * time.Minute),
			want:      true,
		},
		{
			name:      "fresh cache - 9 minutes ago",
			checkedAt: time.Now().Add(-9 * time.Minute),
			want:      true,
		},
		{
			name:      "stale cache - 11 minutes ago",
			checkedAt: time.Now().Add(-11 * time.Minute),
			want:      false,
		},
		{
			name:      "stale cache - 1 hour ago",
			checkedAt: time.Now().Add(-1 * time.Hour),
			want:      false,
		},
		{
			name:      "stale cache - 24 hours ago",
			checkedAt: time.Now().Add(-24 * time.Hour),
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := &CacheEntry{
				CheckedAt:       tt.checkedAt,
				LatestVersion:   "1.0.0",
				UpdateAvailable: false,
			}

			got := IsCacheValid(entry)
			if got != tt.want {
				t.Errorf("IsCacheValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSaveCache_Permissions(t *testing.T) {
	homeDir := t.TempDir()

	entry := &CacheEntry{
		CheckedAt:       time.Now(),
		LatestVersion:   "1.0.0",
		UpdateAvailable: false,
	}

	err := SaveCache(homeDir, entry)
	if err != nil {
		t.Fatalf("SaveCache() error = %v", err)
	}

	// Check file permissions
	cachePath := GetCachePath(homeDir)
	info, err := os.Stat(cachePath)
	if err != nil {
		t.Fatalf("Failed to stat cache file: %v", err)
	}

	mode := info.Mode()
	if mode.Perm() != 0644 {
		t.Errorf("Cache file permissions = %o, want 0644", mode.Perm())
	}
}

func TestSaveCache_OverwriteExisting(t *testing.T) {
	homeDir := t.TempDir()

	// Save first entry
	first := &CacheEntry{
		CheckedAt:       time.Now().Add(-1 * time.Hour),
		LatestVersion:   "1.0.0",
		UpdateAvailable: false,
	}
	err := SaveCache(homeDir, first)
	if err != nil {
		t.Fatalf("SaveCache() first error = %v", err)
	}

	// Save second entry (should overwrite)
	second := &CacheEntry{
		CheckedAt:       time.Now(),
		LatestVersion:   "2.0.0",
		UpdateAvailable: true,
	}
	err = SaveCache(homeDir, second)
	if err != nil {
		t.Fatalf("SaveCache() second error = %v", err)
	}

	// Load and verify second entry is present
	loaded, err := LoadCache(homeDir)
	if err != nil {
		t.Fatalf("LoadCache() error = %v", err)
	}

	if loaded.LatestVersion != "2.0.0" {
		t.Errorf("LatestVersion = %v, want 2.0.0", loaded.LatestVersion)
	}
}

func TestSaveCache_NonexistentDirectory(t *testing.T) {
	err := SaveCache("/nonexistent/path/to/dir", &CacheEntry{
		CheckedAt:       time.Now(),
		LatestVersion:   "1.0.0",
		UpdateAvailable: false,
	})
	if err == nil {
		t.Fatal("expected error writing to non-existent directory")
	}
}

func TestSaveCache_ReadonlyDirectory(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0o555)
	defer os.Chmod(dir, 0o755)

	err := SaveCache(dir, &CacheEntry{
		CheckedAt:       time.Now(),
		LatestVersion:   "1.0.0",
		UpdateAvailable: false,
	})
	if err == nil {
		t.Fatal("expected error writing to readonly directory")
	}
}

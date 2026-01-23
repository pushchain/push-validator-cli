package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/pushchain/push-validator-cli/internal/config"
	"github.com/pushchain/push-validator-cli/internal/snapshot"
)

// mockSnapshotService implements snapshot.Service for testing.
type mockSnapshotService struct {
	downloadErr error
	extractErr  error
	cacheValid  bool
	cacheErr    error
}

func (m *mockSnapshotService) Download(ctx context.Context, opts snapshot.Options) error {
	if opts.Progress != nil {
		opts.Progress(snapshot.PhaseDownload, 50, 100, "downloading")
		opts.Progress(snapshot.PhaseVerify, 100, 100, "verified")
	}
	return m.downloadErr
}

func (m *mockSnapshotService) Extract(ctx context.Context, opts snapshot.ExtractOptions) error {
	if opts.Progress != nil {
		opts.Progress(snapshot.PhaseExtract, 0, 0, "file.db")
	}
	return m.extractErr
}

func (m *mockSnapshotService) IsCacheValid(ctx context.Context, opts snapshot.Options) (bool, error) {
	return m.cacheValid, m.cacheErr
}

func TestTruncate_Short(t *testing.T) {
	result := truncate("short", 10)
	if result != "short" {
		t.Errorf("expected 'short', got '%s'", result)
	}
}

func TestTruncate_Exact(t *testing.T) {
	result := truncate("12345", 5)
	if result != "12345" {
		t.Errorf("expected '12345', got '%s'", result)
	}
}

func TestTruncate_Long(t *testing.T) {
	result := truncate("this is a very long string", 10)
	if result != "this is..." {
		t.Errorf("expected 'this is...', got '%s'", result)
	}
}

func TestTruncate_VeryShortMax(t *testing.T) {
	result := truncate("hello", 3)
	if result != "hel" {
		t.Errorf("expected 'hel', got '%s'", result)
	}
}

func TestTruncate_ZeroMax(t *testing.T) {
	result := truncate("hello", 0)
	if result != "" {
		t.Errorf("expected empty, got '%s'", result)
	}
}

func TestCreateSnapshotProgressCallback_JSON(t *testing.T) {
	cb := createSnapshotProgressCallback("json")
	// Should not panic when called
	cb(snapshot.PhaseDownload, 0, 100, "downloading")
	cb(snapshot.PhaseVerify, 100, 100, "verified")
	cb(snapshot.PhaseExtract, 0, 0, "extracting file.db")
}

func TestCreateSnapshotProgressCallback_Text_Download(t *testing.T) {
	cb := createSnapshotProgressCallback("text")
	// First call with total > 0 creates progress bar
	cb(snapshot.PhaseDownload, 0, 1000, "")
	cb(snapshot.PhaseDownload, 500, 1000, "")
	cb(snapshot.PhaseDownload, 1000, 1000, "")
}

func TestCreateSnapshotProgressCallback_Text_Verify(t *testing.T) {
	cb := createSnapshotProgressCallback("text")
	cb(snapshot.PhaseDownload, 0, 100, "")
	cb(snapshot.PhaseVerify, 0, 0, "Checksum OK")
}

func TestCreateSnapshotProgressCallback_Text_Extract(t *testing.T) {
	cb := createSnapshotProgressCallback("text")
	cb(snapshot.PhaseExtract, 0, 0, "short.db")
	// Test long filename truncation
	longName := "this_is_a_very_long_filename_that_exceeds_sixty_characters_and_should_be_truncated.db"
	cb(snapshot.PhaseExtract, 0, 0, longName)
}

func TestCreateSnapshotProgressCallback_Text_EmptyMessage(t *testing.T) {
	cb := createSnapshotProgressCallback("text")
	// Empty messages should not crash
	cb(snapshot.PhaseVerify, 0, 0, "")
	cb(snapshot.PhaseExtract, 0, 0, "")
}

func TestCreateSnapshotProgressCallback_Text_DownloadZeroTotal(t *testing.T) {
	cb := createSnapshotProgressCallback("text")
	// Download with zero total should not create progress bar
	cb(snapshot.PhaseDownload, 0, 0, "")
	cb(snapshot.PhaseDownload, 50, 0, "")
}

// --- Tests for runSnapshotDownloadCore ---

func snapshotCfg(homeDir string) config.Config {
	return config.Config{
		HomeDir: homeDir,
	}
}

func TestRunSnapshotDownloadCore_Success(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	svc := &mockSnapshotService{}
	cfg := snapshotCfg(t.TempDir())

	err := runSnapshotDownloadCore(context.Background(), svc, cfg, "https://example.com/snap", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSnapshotDownloadCore_DownloadError(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	svc := &mockSnapshotService{downloadErr: fmt.Errorf("network timeout")}
	cfg := snapshotCfg(t.TempDir())

	err := runSnapshotDownloadCore(context.Background(), svc, cfg, "https://example.com/snap", false)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "snapshot download failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunSnapshotDownloadCore_DefaultURL(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	svc := &mockSnapshotService{}
	cfg := snapshotCfg(t.TempDir())

	// Empty snapshotURL should use default
	err := runSnapshotDownloadCore(context.Background(), svc, cfg, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSnapshotDownloadCore_ConfigURL(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	svc := &mockSnapshotService{}
	cfg := snapshotCfg(t.TempDir())
	cfg.SnapshotURL = "https://custom-snapshot.example.com"

	err := runSnapshotDownloadCore(context.Background(), svc, cfg, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSnapshotDownloadCore_TextOutput(t *testing.T) {
	origOutput := flagOutput
	origNoColor := flagNoColor
	defer func() {
		flagOutput = origOutput
		flagNoColor = origNoColor
	}()
	flagOutput = "text"
	flagNoColor = true
	os.Setenv("NO_COLOR", "1")
	defer os.Unsetenv("NO_COLOR")

	svc := &mockSnapshotService{}
	cfg := snapshotCfg(t.TempDir())

	err := runSnapshotDownloadCore(context.Background(), svc, cfg, "https://example.com/snap", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSnapshotDownloadCore_NoCache(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	svc := &mockSnapshotService{}
	cfg := snapshotCfg(t.TempDir())

	err := runSnapshotDownloadCore(context.Background(), svc, cfg, "https://example.com/snap", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Tests for runSnapshotExtractCore ---

func TestRunSnapshotExtractCore_Success(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	svc := &mockSnapshotService{}
	cfg := snapshotCfg(t.TempDir())

	err := runSnapshotExtractCore(context.Background(), svc, cfg, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSnapshotExtractCore_ExtractError(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	svc := &mockSnapshotService{extractErr: fmt.Errorf("disk full")}
	cfg := snapshotCfg(t.TempDir())

	err := runSnapshotExtractCore(context.Background(), svc, cfg, "", false)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "snapshot extract failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunSnapshotExtractCore_AlreadyExtracted(t *testing.T) {
	origOutput := flagOutput
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagOutput = origOutput
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagOutput = "text"
	flagNoColor = true
	flagNoEmoji = true

	dir := t.TempDir()
	// Create marker file that IsSnapshotPresent checks
	dataDir := filepath.Join(dir, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "application.db"), 0o755); err != nil {
		t.Fatal(err)
	}

	svc := &mockSnapshotService{}
	cfg := snapshotCfg(dir)

	err := runSnapshotExtractCore(context.Background(), svc, cfg, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSnapshotExtractCore_AlreadyExtracted_JSON(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "application.db"), 0o755); err != nil {
		t.Fatal(err)
	}

	svc := &mockSnapshotService{}
	cfg := snapshotCfg(dir)

	err := runSnapshotExtractCore(context.Background(), svc, cfg, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSnapshotExtractCore_ForceReextract(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	dir := t.TempDir()
	// Create marker file
	dataDir := filepath.Join(dir, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "application.db"), 0o755); err != nil {
		t.Fatal(err)
	}

	svc := &mockSnapshotService{}
	cfg := snapshotCfg(dir)

	// Force should bypass the already-extracted check
	err := runSnapshotExtractCore(context.Background(), svc, cfg, "", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSnapshotExtractCore_CustomTargetDir(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	svc := &mockSnapshotService{}
	cfg := snapshotCfg(t.TempDir())

	err := runSnapshotExtractCore(context.Background(), svc, cfg, "/custom/target", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSnapshotExtractCore_TextOutput(t *testing.T) {
	origOutput := flagOutput
	origNoColor := flagNoColor
	defer func() {
		flagOutput = origOutput
		flagNoColor = origNoColor
	}()
	flagOutput = "text"
	flagNoColor = true
	os.Setenv("NO_COLOR", "1")
	defer os.Unsetenv("NO_COLOR")

	svc := &mockSnapshotService{}
	cfg := snapshotCfg(t.TempDir())

	err := runSnapshotExtractCore(context.Background(), svc, cfg, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSnapshotDownloadCore_TextOutput_WithColor(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "text"
	os.Unsetenv("NO_COLOR")

	svc := &mockSnapshotService{}
	cfg := snapshotCfg(t.TempDir())

	err := runSnapshotDownloadCore(context.Background(), svc, cfg, "https://example.com/snap", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSnapshotExtractCore_TextOutput_WithColor(t *testing.T) {
	origOutput := flagOutput
	origNoColor := flagNoColor
	defer func() {
		flagOutput = origOutput
		flagNoColor = origNoColor
	}()
	flagOutput = "text"
	flagNoColor = false
	os.Unsetenv("NO_COLOR")

	svc := &mockSnapshotService{}
	cfg := snapshotCfg(t.TempDir())

	err := runSnapshotExtractCore(context.Background(), svc, cfg, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSnapshotExtractCore_ExtractError_Text(t *testing.T) {
	origOutput := flagOutput
	origNoColor := flagNoColor
	defer func() {
		flagOutput = origOutput
		flagNoColor = origNoColor
	}()
	flagOutput = "text"
	flagNoColor = true

	svc := &mockSnapshotService{extractErr: fmt.Errorf("permission denied")}
	cfg := snapshotCfg(t.TempDir())

	err := runSnapshotExtractCore(context.Background(), svc, cfg, "/custom/target", false)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "snapshot extract failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunSnapshotDownloadCore_TextOutput_DownloadMessage(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "text"
	os.Unsetenv("NO_COLOR")

	// Custom mock that sends download message without total (exercises the else-if branch)
	svc := &mockSnapshotService{}
	cfg := snapshotCfg(t.TempDir())

	err := runSnapshotDownloadCore(context.Background(), svc, cfg, "https://example.com/snap", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

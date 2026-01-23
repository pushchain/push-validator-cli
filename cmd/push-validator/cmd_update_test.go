package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/pushchain/push-validator-cli/internal/update"
)

func TestCheckNodeRunningInDir_NoPidFiles(t *testing.T) {
	dir := t.TempDir()
	if checkNodeRunningInDir(dir) {
		t.Error("expected false when no PID files exist")
	}
}

func TestCheckNodeRunningInDir_PchaindPid(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "pchaind.pid")
	if err := os.WriteFile(pidFile, []byte("12345"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !checkNodeRunningInDir(dir) {
		t.Error("expected true when pchaind.pid exists")
	}
}

func TestCheckNodeRunningInDir_CosmovisorPid(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "cosmovisor.pid")
	if err := os.WriteFile(pidFile, []byte("67890"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !checkNodeRunningInDir(dir) {
		t.Error("expected true when cosmovisor.pid exists")
	}
}

func TestCheckNodeRunningInDir_BothPids(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pchaind.pid"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cosmovisor.pid"), []byte("2"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !checkNodeRunningInDir(dir) {
		t.Error("expected true when both PID files exist")
	}
}

func TestCheckNodeRunningInDir_NonExistentDir(t *testing.T) {
	if checkNodeRunningInDir("/nonexistent/path/12345") {
		t.Error("expected false for non-existent directory")
	}
}

// mockCLIUpdater implements CLIUpdater for testing.
type mockCLIUpdater struct {
	latestRelease *update.Release
	latestErr     error
	tagRelease    *update.Release
	tagErr        error
	downloadData  []byte
	downloadErr   error
	checksumErr   error
	extractData   []byte
	extractErr    error
	installErr    error
	rollbackErr   error
}

func (m *mockCLIUpdater) FetchLatestRelease() (*update.Release, error) {
	return m.latestRelease, m.latestErr
}
func (m *mockCLIUpdater) FetchReleaseByTag(tag string) (*update.Release, error) {
	return m.tagRelease, m.tagErr
}
func (m *mockCLIUpdater) Download(asset *update.Asset, progress update.ProgressFunc) ([]byte, error) {
	if progress != nil {
		progress(100, 100)
	}
	return m.downloadData, m.downloadErr
}
func (m *mockCLIUpdater) VerifyChecksum(data []byte, release *update.Release, assetName string) error {
	return m.checksumErr
}
func (m *mockCLIUpdater) ExtractBinary(archiveData []byte) ([]byte, error) {
	return m.extractData, m.extractErr
}
func (m *mockCLIUpdater) Install(binaryData []byte) error {
	return m.installErr
}
func (m *mockCLIUpdater) Rollback() error {
	return m.rollbackErr
}

func testRelease(tag string) *update.Release {
	ver := tag[1:] // strip "v" prefix
	assetName := fmt.Sprintf("push-validator_%s_%s_%s.tar.gz", ver, runtime.GOOS, runtime.GOARCH)
	return &update.Release{
		TagName: tag,
		Body:    "## Changes\n- Fixed bug\n- Added feature",
		HTMLURL: "https://github.com/pushchain/push-validator-cli/releases/tag/" + tag,
		Assets: []update.Asset{
			{Name: assetName, Size: 1024, BrowserDownloadURL: "https://example.com/asset.tar.gz"},
		},
	}
}

func TestRunUpdateCore_AlreadyUpToDate(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "text"

	cfg := testCfg()
	m := &mockCLIUpdater{latestRelease: testRelease("v1.0.0")}

	err := runUpdateCore(m, cfg, updateCoreOpts{
		currentVersion: "v1.0.0",
	}, testPrinter(), &nonInteractivePrompter{}, io.Discard, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunUpdateCore_FetchError(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "text"

	cfg := testCfg()
	m := &mockCLIUpdater{latestErr: fmt.Errorf("network error")}

	err := runUpdateCore(m, cfg, updateCoreOpts{
		currentVersion: "v1.0.0",
	}, testPrinter(), &nonInteractivePrompter{}, io.Discard, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "failed to fetch release") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunUpdateCore_FetchByTag(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "text"

	cfg := testCfg()
	m := &mockCLIUpdater{tagRelease: testRelease("v2.0.0")}

	err := runUpdateCore(m, cfg, updateCoreOpts{
		currentVersion: "v1.0.0",
		version:        "v2.0.0",
		force:          true,
		skipVerify:     true,
		binaryPath:     "/tmp/fake",
	}, testPrinter(), &nonInteractivePrompter{}, io.Discard, func(path string) (string, error) {
		return "2.0.0", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunUpdateCore_FetchByTagError(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "text"

	cfg := testCfg()
	m := &mockCLIUpdater{tagErr: fmt.Errorf("tag not found")}

	err := runUpdateCore(m, cfg, updateCoreOpts{
		currentVersion: "v1.0.0",
		version:        "v99.99.99",
	}, testPrinter(), &nonInteractivePrompter{}, io.Discard, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRunUpdateCore_CheckOnly(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "text"

	cfg := testCfg()
	m := &mockCLIUpdater{latestRelease: testRelease("v2.0.0")}

	err := runUpdateCore(m, cfg, updateCoreOpts{
		currentVersion: "v1.0.0",
		checkOnly:      true,
		force:          true, // force to skip version check
	}, testPrinter(), &nonInteractivePrompter{}, io.Discard, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunUpdateCore_DownloadError(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "text"

	cfg := testCfg()
	m := &mockCLIUpdater{
		latestRelease: testRelease("v2.0.0"),
		downloadErr:   fmt.Errorf("connection reset"),
	}

	err := runUpdateCore(m, cfg, updateCoreOpts{
		currentVersion: "v1.0.0",
		force:          true,
		skipVerify:     true,
	}, testPrinter(), &nonInteractivePrompter{}, io.Discard, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "download failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunUpdateCore_ChecksumError(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "text"

	cfg := testCfg()
	m := &mockCLIUpdater{
		latestRelease: testRelease("v2.0.0"),
		downloadData:  []byte("fake-archive"),
		checksumErr:   fmt.Errorf("checksum mismatch"),
	}

	err := runUpdateCore(m, cfg, updateCoreOpts{
		currentVersion: "v1.0.0",
		force:          true,
	}, testPrinter(), &nonInteractivePrompter{}, io.Discard, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "checksum verification failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunUpdateCore_ExtractError(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "text"

	cfg := testCfg()
	m := &mockCLIUpdater{
		latestRelease: testRelease("v2.0.0"),
		downloadData:  []byte("fake-archive"),
		extractErr:    fmt.Errorf("corrupt archive"),
	}

	err := runUpdateCore(m, cfg, updateCoreOpts{
		currentVersion: "v1.0.0",
		force:          true,
		skipVerify:     true,
	}, testPrinter(), &nonInteractivePrompter{}, io.Discard, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "extraction failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunUpdateCore_InstallError(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "text"

	cfg := testCfg()
	m := &mockCLIUpdater{
		latestRelease: testRelease("v2.0.0"),
		downloadData:  []byte("fake-archive"),
		extractData:   []byte("fake-binary"),
		installErr:    fmt.Errorf("permission denied"),
	}

	err := runUpdateCore(m, cfg, updateCoreOpts{
		currentVersion: "v1.0.0",
		force:          true,
		skipVerify:     true,
	}, testPrinter(), &nonInteractivePrompter{}, io.Discard, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "installation failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunUpdateCore_VerifyFails_Rollback(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "text"

	cfg := testCfg()
	m := &mockCLIUpdater{
		latestRelease: testRelease("v2.0.0"),
		downloadData:  []byte("fake-archive"),
		extractData:   []byte("fake-binary"),
	}

	err := runUpdateCore(m, cfg, updateCoreOpts{
		currentVersion: "v1.0.0",
		force:          true,
		skipVerify:     true,
		binaryPath:     "/tmp/fake-binary",
	}, testPrinter(), &nonInteractivePrompter{}, io.Discard, func(path string) (string, error) {
		return "", fmt.Errorf("binary crashed")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "rolled back") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunUpdateCore_VerifyFails_RollbackFails(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "text"

	cfg := testCfg()
	m := &mockCLIUpdater{
		latestRelease: testRelease("v2.0.0"),
		downloadData:  []byte("fake-archive"),
		extractData:   []byte("fake-binary"),
		rollbackErr:   fmt.Errorf("rollback permission denied"),
	}

	err := runUpdateCore(m, cfg, updateCoreOpts{
		currentVersion: "v1.0.0",
		force:          true,
		skipVerify:     true,
		binaryPath:     "/tmp/fake",
	}, testPrinter(), &nonInteractivePrompter{}, io.Discard, func(path string) (string, error) {
		return "", fmt.Errorf("binary crashed")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "rollback failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunUpdateCore_FullSuccess_WithVerify(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "text"

	cfg := testCfg()
	m := &mockCLIUpdater{
		latestRelease: testRelease("v2.0.0"),
		downloadData:  []byte("fake-archive"),
		extractData:   []byte("fake-binary"),
	}

	err := runUpdateCore(m, cfg, updateCoreOpts{
		currentVersion: "v1.0.0",
		force:          true,
		binaryPath:     "/tmp/fake",
	}, testPrinter(), &nonInteractivePrompter{}, io.Discard, func(path string) (string, error) {
		return "2.0.0", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunUpdateCore_FullSuccess_NodeRunning(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "text"

	// Create a home dir with a PID file to simulate running node
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pchaind.pid"), []byte("123"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := testCfg()
	cfg.HomeDir = dir
	m := &mockCLIUpdater{
		latestRelease: testRelease("v2.0.0"),
		downloadData:  []byte("fake-archive"),
		extractData:   []byte("fake-binary"),
	}

	err := runUpdateCore(m, cfg, updateCoreOpts{
		currentVersion: "v1.0.0",
		force:          true,
		skipVerify:     true,
	}, testPrinter(), &nonInteractivePrompter{}, io.Discard, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunUpdateCore_SkipVerify(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "text"

	cfg := testCfg()
	m := &mockCLIUpdater{
		latestRelease: testRelease("v2.0.0"),
		downloadData:  []byte("fake-archive"),
		extractData:   []byte("fake-binary"),
	}

	err := runUpdateCore(m, cfg, updateCoreOpts{
		currentVersion: "v1.0.0",
		force:          true,
		skipVerify:     true,
	}, testPrinter(), &nonInteractivePrompter{}, io.Discard, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunUpdateCore_PromptYes(t *testing.T) {
	origOutput := flagOutput
	origYes := flagYes
	defer func() { flagOutput = origOutput; flagYes = origYes }()
	flagOutput = "text"
	flagYes = false

	cfg := testCfg()
	m := &mockCLIUpdater{
		latestRelease: testRelease("v2.0.0"),
		downloadData:  []byte("fake-archive"),
		extractData:   []byte("fake-binary"),
	}

	prompter := &mockPrompter{interactive: true, responses: []string{"y"}}
	err := runUpdateCore(m, cfg, updateCoreOpts{
		currentVersion: "v1.0.0",
		skipVerify:     true,
	}, testPrinter(), prompter, io.Discard, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunUpdateCore_PromptEmpty(t *testing.T) {
	origOutput := flagOutput
	origYes := flagYes
	defer func() { flagOutput = origOutput; flagYes = origYes }()
	flagOutput = "text"
	flagYes = false

	cfg := testCfg()
	m := &mockCLIUpdater{
		latestRelease: testRelease("v2.0.0"),
		downloadData:  []byte("fake-archive"),
		extractData:   []byte("fake-binary"),
	}

	// Empty response (just Enter) should proceed
	prompter := &mockPrompter{interactive: true, responses: []string{""}}
	err := runUpdateCore(m, cfg, updateCoreOpts{
		currentVersion: "v1.0.0",
		skipVerify:     true,
	}, testPrinter(), prompter, io.Discard, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunUpdateCore_PromptNo(t *testing.T) {
	origOutput := flagOutput
	origYes := flagYes
	defer func() { flagOutput = origOutput; flagYes = origYes }()
	flagOutput = "text"
	flagYes = false

	cfg := testCfg()
	m := &mockCLIUpdater{latestRelease: testRelease("v2.0.0")}

	prompter := &mockPrompter{interactive: true, responses: []string{"n"}}
	err := runUpdateCore(m, cfg, updateCoreOpts{
		currentVersion: "v1.0.0",
	}, testPrinter(), prompter, io.Discard, nil)
	if err != nil {
		t.Fatal("expected nil (cancelled), got error:", err)
	}
}

func TestRunUpdateCore_PromptError(t *testing.T) {
	origOutput := flagOutput
	origYes := flagYes
	defer func() { flagOutput = origOutput; flagYes = origYes }()
	flagOutput = "text"
	flagYes = false

	cfg := testCfg()
	m := &mockCLIUpdater{latestRelease: testRelease("v2.0.0")}

	// Empty responses slice causes ReadLine to return error
	prompter := &mockPrompter{interactive: true, responses: []string{}}
	err := runUpdateCore(m, cfg, updateCoreOpts{
		currentVersion: "v1.0.0",
	}, testPrinter(), prompter, io.Discard, nil)
	if err != nil {
		t.Fatal("expected nil (cancelled on prompt error), got error:", err)
	}
}

func TestRunUpdateCore_EmptyChangelog(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "text"

	cfg := testCfg()
	rel := testRelease("v2.0.0")
	rel.Body = "" // empty changelog
	m := &mockCLIUpdater{latestRelease: rel}

	err := runUpdateCore(m, cfg, updateCoreOpts{
		currentVersion: "v1.0.0",
		checkOnly:      true,
		force:          true,
	}, testPrinter(), &nonInteractivePrompter{}, io.Discard, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunUpdateCore_AssetNotFound(t *testing.T) {
	origOutput := flagOutput
	origYes := flagYes
	defer func() { flagOutput = origOutput; flagYes = origYes }()
	flagOutput = "text"
	flagYes = true

	cfg := testCfg()
	// Release with no matching asset for current platform
	rel := &update.Release{
		TagName: "v2.0.0",
		Assets: []update.Asset{
			{Name: "push-validator_2.0.0_windows_amd64.tar.gz", Size: 1024},
		},
	}
	m := &mockCLIUpdater{latestRelease: rel}

	err := runUpdateCore(m, cfg, updateCoreOpts{
		currentVersion: "v1.0.0",
		force:          true,
	}, testPrinter(), &nonInteractivePrompter{}, io.Discard, nil)
	if err == nil {
		t.Fatal("expected error for missing platform asset")
	}
}

func TestRunUpdateCore_LongChangelog(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "text"

	cfg := testCfg()
	// Create release with long changelog (>10 lines)
	rel := testRelease("v2.0.0")
	rel.Body = "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\nline11\nline12"
	m := &mockCLIUpdater{latestRelease: rel}

	err := runUpdateCore(m, cfg, updateCoreOpts{
		currentVersion: "v1.0.0",
		checkOnly:      true,
		force:          true,
	}, testPrinter(), &nonInteractivePrompter{}, io.Discard, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

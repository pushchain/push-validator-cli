package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/pushchain/push-validator-cli/internal/chain"
)

// mockChainFetcher implements ChainReleaseFetcher for tests.
type mockChainFetcher struct {
	latest    *chain.Release
	latestErr error
	byTag     *chain.Release
	byTagErr  error
}

func (m *mockChainFetcher) FetchLatest() (*chain.Release, error)            { return m.latest, m.latestErr }
func (m *mockChainFetcher) FetchByTag(tag string) (*chain.Release, error) { return m.byTag, m.byTagErr }

// mockChainInstaller implements ChainInstaller for tests.
type mockChainInstaller struct {
	downloadData   []byte
	downloadErr    error
	checksumResult bool
	checksumErr    error
	installPath    string
	installErr     error
}

func (m *mockChainInstaller) Download(asset *chain.Asset, progress chain.ProgressFunc) ([]byte, error) {
	if progress != nil {
		progress(100, 100)
	}
	return m.downloadData, m.downloadErr
}
func (m *mockChainInstaller) VerifyChecksum(data []byte, release *chain.Release, assetName string) (bool, error) {
	return m.checksumResult, m.checksumErr
}
func (m *mockChainInstaller) ExtractAndInstall(data []byte) (string, error) {
	return m.installPath, m.installErr
}

func testChainRelease(tag string) *chain.Release {
	ver := tag[1:] // strip "v" prefix
	assetName := fmt.Sprintf("push-chain_%s_%s_%s.tar.gz", ver, runtime.GOOS, runtime.GOARCH)
	return &chain.Release{
		TagName: tag,
		Assets: []chain.Asset{
			{Name: assetName, Size: 2048, BrowserDownloadURL: "https://example.com/pchaind.tar.gz"},
		},
	}
}

func TestRunChainInstallCore_FetchLatestError(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "text"

	cfg := testCfg()
	fetcher := &mockChainFetcher{latestErr: fmt.Errorf("network error")}
	installer := &mockChainInstaller{}

	err := runChainInstallCore(cfg, fetcher, installer, chainInstallOpts{}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "failed to fetch release") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunChainInstallCore_FetchByTagError(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "text"

	cfg := testCfg()
	fetcher := &mockChainFetcher{byTagErr: fmt.Errorf("tag not found")}
	installer := &mockChainInstaller{}

	err := runChainInstallCore(cfg, fetcher, installer, chainInstallOpts{version: "v99.0.0"}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRunChainInstallCore_AlreadyInstalled(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "text"

	cfg := testCfg()
	// Create the cosmovisor bin path
	binDir := filepath.Join(cfg.HomeDir, "cosmovisor", "genesis", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "pchaind"), []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}

	fetcher := &mockChainFetcher{latest: testChainRelease("v1.0.0")}
	installer := &mockChainInstaller{}

	// verifyBinary returns the same version as the release
	verifyBinary := func(path string) (string, error) {
		return "1.0.0", nil
	}

	err := runChainInstallCore(cfg, fetcher, installer, chainInstallOpts{}, verifyBinary)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunChainInstallCore_ForceReinstall(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "text"

	cfg := testCfg()
	fetcher := &mockChainFetcher{latest: testChainRelease("v1.0.0")}
	installer := &mockChainInstaller{
		downloadData: []byte("archive-data"),
		installPath:  "/tmp/pchaind",
	}

	err := runChainInstallCore(cfg, fetcher, installer, chainInstallOpts{
		force:      true,
		skipVerify: true,
	}, func(path string) (string, error) {
		return "1.0.0", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunChainInstallCore_DownloadError(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "text"

	cfg := testCfg()
	fetcher := &mockChainFetcher{latest: testChainRelease("v2.0.0")}
	installer := &mockChainInstaller{downloadErr: fmt.Errorf("timeout")}

	err := runChainInstallCore(cfg, fetcher, installer, chainInstallOpts{
		force:      true,
		skipVerify: true,
	}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "download failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunChainInstallCore_ChecksumError(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "text"

	cfg := testCfg()
	fetcher := &mockChainFetcher{latest: testChainRelease("v2.0.0")}
	installer := &mockChainInstaller{
		downloadData: []byte("data"),
		checksumErr:  fmt.Errorf("mismatch"),
	}

	err := runChainInstallCore(cfg, fetcher, installer, chainInstallOpts{force: true}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "checksum verification failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunChainInstallCore_ChecksumNotAvailable(t *testing.T) {
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

	cfg := testCfg()
	fetcher := &mockChainFetcher{latest: testChainRelease("v2.0.0")}
	installer := &mockChainInstaller{
		downloadData:   []byte("data"),
		checksumResult: false, // not available
		installPath:    "/tmp/pchaind",
	}

	err := runChainInstallCore(cfg, fetcher, installer, chainInstallOpts{force: true}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunChainInstallCore_ExtractError(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "text"

	cfg := testCfg()
	fetcher := &mockChainFetcher{latest: testChainRelease("v2.0.0")}
	installer := &mockChainInstaller{
		downloadData: []byte("data"),
		installErr:   fmt.Errorf("corrupt archive"),
	}

	err := runChainInstallCore(cfg, fetcher, installer, chainInstallOpts{
		force:      true,
		skipVerify: true,
	}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "installation failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunChainInstallCore_FullSuccess_WithVersion(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "text"

	cfg := testCfg()
	fetcher := &mockChainFetcher{byTag: testChainRelease("v2.0.0")}
	installer := &mockChainInstaller{
		downloadData:   []byte("data"),
		checksumResult: true,
		installPath:    "/tmp/pchaind",
	}

	err := runChainInstallCore(cfg, fetcher, installer, chainInstallOpts{
		version: "v2.0.0",
		force:   true,
	}, func(path string) (string, error) {
		return "2.0.0", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunChainInstallCore_VerifyFails_StillSucceeds(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "text"

	cfg := testCfg()
	fetcher := &mockChainFetcher{latest: testChainRelease("v2.0.0")}
	installer := &mockChainInstaller{
		downloadData: []byte("data"),
		installPath:  "/tmp/pchaind",
	}

	// verifyBinary fails but install still succeeds (just shows tag instead of version)
	err := runChainInstallCore(cfg, fetcher, installer, chainInstallOpts{
		force:      true,
		skipVerify: true,
	}, func(path string) (string, error) {
		return "", fmt.Errorf("binary not executable")
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunChainInstallCore_JSON_Output(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	cfg := testCfg()
	fetcher := &mockChainFetcher{latest: testChainRelease("v2.0.0")}
	installer := &mockChainInstaller{
		downloadData: []byte("data"),
		installPath:  "/tmp/pchaind",
	}

	err := runChainInstallCore(cfg, fetcher, installer, chainInstallOpts{
		force:      true,
		skipVerify: true,
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

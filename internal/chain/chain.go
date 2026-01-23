package chain

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	// GitHub repository for push-chain-node
	githubOwner      = "pushchain"
	githubRepo       = "push-chain-node"
	latestReleaseURL = "https://api.github.com/repos/pushchain/push-chain-node/releases/latest"
	releaseByTagURL  = "https://api.github.com/repos/pushchain/push-chain-node/releases/tags/%s"

	httpTimeout = 30 * time.Second
)

// httpClient can be overridden for testing
var httpClient = &http.Client{Timeout: httpTimeout}

// Release represents a GitHub release
type Release struct {
	TagName    string  `json:"tag_name"`
	Name       string  `json:"name"`
	Body       string  `json:"body"`
	HTMLURL    string  `json:"html_url"`
	Assets     []Asset `json:"assets"`
	Prerelease bool    `json:"prerelease"`
}

// Asset represents a release asset
type Asset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// ProgressFunc is called during download with bytes downloaded and total size
type ProgressFunc func(downloaded, total int64)

// Installer handles downloading and installing pchaind
type Installer struct {
	HomeDir string // e.g., ~/.pchain
}

// NewInstaller creates a new chain installer
func NewInstaller(homeDir string) *Installer {
	return &Installer{HomeDir: homeDir}
}

// FetchLatestRelease gets the latest release from GitHub
func FetchLatestRelease() (*Release, error) {
	req, err := http.NewRequest("GET", latestReleaseURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "push-validator-cli")

	resp, err := httpClient.Do(req)
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

	url := fmt.Sprintf(releaseByTagURL, tag)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "push-validator-cli")

	resp, err := httpClient.Do(req)
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

	// Expected format: push-chain_0.0.2_darwin_arm64.tar.gz
	suffix := fmt.Sprintf("_%s_%s.tar.gz", osName, arch)

	for i := range release.Assets {
		asset := &release.Assets[i]
		if strings.HasPrefix(asset.Name, "push-chain_") && strings.HasSuffix(asset.Name, suffix) {
			return asset, nil
		}
	}

	return nil, fmt.Errorf("no binary found for %s/%s in release %s", osName, arch, release.TagName)
}

// GetChecksumAsset finds the checksum asset for a specific file
func GetChecksumAsset(release *Release, assetName string) (*Asset, error) {
	checksumName := assetName + ".sha256"
	for i := range release.Assets {
		asset := &release.Assets[i]
		if asset.Name == checksumName {
			return asset, nil
		}
	}
	return nil, fmt.Errorf("checksum file not found for %s", assetName)
}

// Download fetches the binary archive with progress
func (inst *Installer) Download(asset *Asset, progress ProgressFunc) ([]byte, error) {
	resp, err := http.Get(asset.BrowserDownloadURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed: %s", resp.Status)
	}

	var reader io.Reader = resp.Body
	if progress != nil {
		reader = &progressReader{
			reader:   resp.Body,
			total:    resp.ContentLength,
			progress: progress,
		}
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read download: %w", err)
	}

	return data, nil
}

// progressReader wraps a reader to report progress
type progressReader struct {
	reader     io.Reader
	total      int64
	downloaded int64
	progress   ProgressFunc
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.downloaded += int64(n)
	if pr.progress != nil {
		pr.progress(pr.downloaded, pr.total)
	}
	return n, err
}

// VerifyChecksum validates the downloaded archive.
// Returns (verified bool, err error):
//   - (true, nil): checksum verified successfully
//   - (false, nil): checksum file not found, verification skipped
//   - (false, err): checksum mismatch or download error
func (inst *Installer) VerifyChecksum(data []byte, release *Release, assetName string) (bool, error) {
	checksumAsset, err := GetChecksumAsset(release, assetName)
	if err != nil {
		// Checksum file not found in release - skip verification gracefully
		return false, nil
	}

	// Download checksum file
	resp, err := http.Get(checksumAsset.BrowserDownloadURL)
	if err != nil {
		return false, fmt.Errorf("failed to download checksum: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		// Checksum file URL returned 404 - skip verification gracefully
		return false, nil
	}

	// Parse checksum file (format: "sha256  filename" or just "sha256")
	var expectedHash string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 1 {
			expectedHash = parts[0]
			break
		}
	}

	if expectedHash == "" {
		return false, fmt.Errorf("could not parse checksum file")
	}

	// Calculate actual hash
	hash := sha256.Sum256(data)
	actualHash := hex.EncodeToString(hash[:])

	if actualHash != expectedHash {
		return false, fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	return true, nil
}

// ExtractAndInstall extracts the binary and installs to cosmovisor directory
func (inst *Installer) ExtractAndInstall(archiveData []byte) (string, error) {
	gzReader, err := gzip.NewReader(bytes.NewReader(archiveData))
	if err != nil {
		return "", fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer func() { _ = gzReader.Close() }()

	tarReader := tar.NewReader(gzReader)

	// Cosmovisor directory structure
	cosmovisorBin := filepath.Join(inst.HomeDir, "cosmovisor", "genesis", "bin")
	if err := os.MkdirAll(cosmovisorBin, 0o755); err != nil {
		return "", fmt.Errorf("failed to create cosmovisor directory: %w", err)
	}

	// Also create upgrades directory
	upgradesDir := filepath.Join(inst.HomeDir, "cosmovisor", "upgrades")
	if err := os.MkdirAll(upgradesDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create upgrades directory: %w", err)
	}

	var pchaindPath string
	var wasmLibPath string

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("failed to read tar: %w", err)
		}

		if header.Typeflag != tar.TypeReg {
			continue
		}

		baseName := filepath.Base(header.Name)

		// Extract pchaind binary
		if baseName == "pchaind" {
			destPath := filepath.Join(cosmovisorBin, "pchaind")
			if err := extractFile(tarReader, destPath, 0o755); err != nil {
				return "", fmt.Errorf("failed to extract pchaind: %w", err)
			}
			pchaindPath = destPath
		}

		// Extract libwasmvm.dylib if present (required on macOS)
		if baseName == "libwasmvm.dylib" {
			destPath := filepath.Join(cosmovisorBin, "libwasmvm.dylib")
			if err := extractFile(tarReader, destPath, 0o644); err != nil {
				return "", fmt.Errorf("failed to extract libwasmvm: %w", err)
			}
			wasmLibPath = destPath
		}
	}

	if pchaindPath == "" {
		return "", fmt.Errorf("pchaind binary not found in archive")
	}

	_ = wasmLibPath // Used but not returned

	return pchaindPath, nil
}

// extractFile extracts a single file from tar reader
func extractFile(reader io.Reader, destPath string, mode os.FileMode) error {
	// Remove existing file
	os.Remove(destPath)

	out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, reader)
	return err
}

// GetInstalledVersion returns the version of currently installed pchaind
func (inst *Installer) GetInstalledVersion() string {
	binPath := filepath.Join(inst.HomeDir, "cosmovisor", "genesis", "bin", "pchaind")
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		return ""
	}

	// Try to run pchaind version
	// This would require exec, but for simplicity we just check existence
	return "installed"
}

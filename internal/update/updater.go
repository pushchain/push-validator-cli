package update

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Updater handles the update process
type Updater struct {
	CurrentVersion string
	BinaryPath     string // Path to current executable
}

// NewUpdater creates an updater for the current binary
func NewUpdater(currentVersion string) (*Updater, error) {
	execPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}

	// Resolve symlinks to get actual binary path
	realPath, err := filepath.EvalSymlinks(execPath)
	if err != nil {
		realPath = execPath
	}

	return &Updater{
		CurrentVersion: currentVersion,
		BinaryPath:     realPath,
	}, nil
}

// Check compares current version with latest release
func (u *Updater) Check() (*CheckResult, error) {
	release, err := FetchLatestRelease()
	if err != nil {
		return nil, err
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	currentVersion := strings.TrimPrefix(u.CurrentVersion, "v")

	return &CheckResult{
		CurrentVersion:  currentVersion,
		LatestVersion:   latestVersion,
		UpdateAvailable: IsNewerVersion(u.CurrentVersion, release.TagName),
		Release:         release,
	}, nil
}

// ProgressFunc is called during download with bytes downloaded and total size
type ProgressFunc func(downloaded, total int64)

// Download fetches the binary archive
func (u *Updater) Download(asset *Asset, progress ProgressFunc) ([]byte, error) {
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

// VerifyChecksum validates the downloaded archive against checksums.txt
func (u *Updater) VerifyChecksum(data []byte, release *Release, assetName string) error {
	checksumAsset, err := GetChecksumAsset(release)
	if err != nil {
		return err
	}

	// Download checksums.txt
	resp, err := http.Get(checksumAsset.BrowserDownloadURL)
	if err != nil {
		return fmt.Errorf("failed to download checksums: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Parse checksums.txt (format: "sha256  filename")
	expectedHash := ""
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == assetName {
			expectedHash = parts[0]
			break
		}
	}

	if expectedHash == "" {
		return fmt.Errorf("checksum not found for %s", assetName)
	}

	// Calculate actual hash
	hash := sha256.Sum256(data)
	actualHash := hex.EncodeToString(hash[:])

	if actualHash != expectedHash {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	return nil
}

// ExtractBinary extracts the binary from the tar.gz archive
func (u *Updater) ExtractBinary(archiveData []byte) ([]byte, error) {
	gzReader, err := gzip.NewReader(bytes.NewReader(archiveData))
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer func() { _ = gzReader.Close() }()

	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar: %w", err)
		}

		// Look for the binary (push-validator)
		if header.Typeflag == tar.TypeReg &&
			(header.Name == "push-validator" || strings.HasSuffix(header.Name, "/push-validator")) {
			data, err := io.ReadAll(tarReader)
			if err != nil {
				return nil, fmt.Errorf("failed to read binary: %w", err)
			}
			return data, nil
		}
	}

	return nil, fmt.Errorf("binary not found in archive")
}

// Install performs atomic binary replacement
func (u *Updater) Install(binaryData []byte) error {
	// Get current binary permissions
	info, err := os.Stat(u.BinaryPath)
	if err != nil {
		return fmt.Errorf("failed to stat current binary: %w", err)
	}
	mode := info.Mode()

	// Create backup
	backupPath := u.BinaryPath + ".backup"
	if err := copyFile(u.BinaryPath, backupPath); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	// Write to temp file in same directory (for atomic rename)
	dir := filepath.Dir(u.BinaryPath)
	tempFile, err := os.CreateTemp(dir, "push-validator-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()

	// Write binary data
	if _, err := tempFile.Write(binaryData); err != nil {
		tempFile.Close()
		os.Remove(tempPath)
		return fmt.Errorf("failed to write new binary: %w", err)
	}
	tempFile.Close()

	// Set permissions
	if err := os.Chmod(tempPath, mode); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempPath, u.BinaryPath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to install binary: %w", err)
	}

	return nil
}

// Rollback restores the backup
func (u *Updater) Rollback() error {
	backupPath := u.BinaryPath + ".backup"
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("no backup found")
	}
	return os.Rename(backupPath, u.BinaryPath)
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = source.Close() }()

	dest, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = dest.Close() }()

	_, err = io.Copy(dest, source)
	return err
}

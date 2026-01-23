package snapshot

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// DefaultSnapshotURL is the default base URL for snapshot downloads.
const DefaultSnapshotURL = "https://snapshots.donut.push.org"

// ProgressPhase indicates which phase of the snapshot process is active.
type ProgressPhase string

const (
	PhaseCache    ProgressPhase = "cache"    // Checking/using cache
	PhaseDownload ProgressPhase = "download"
	PhaseVerify   ProgressPhase = "verify"
	PhaseExtract  ProgressPhase = "extract"
)

// ProgressFunc is called during download/extraction with progress updates.
// phase: current operation (download, verify, extract)
// current: bytes/items processed
// total: total bytes/items (-1 if unknown)
// message: optional status message
type ProgressFunc func(phase ProgressPhase, current, total int64, message string)

// Options configures the snapshot download and extraction.
type Options struct {
	SnapshotURL string       // Base URL for snapshots (default: DefaultSnapshotURL)
	HomeDir     string       // Node home directory (e.g., ~/.pchain)
	Progress    ProgressFunc // Optional progress callback
	NoCache     bool         // Force fresh download, skip cache check
}

// ExtractOptions configures the snapshot extraction.
type ExtractOptions struct {
	HomeDir   string       // Node home directory (e.g., ~/.pchain)
	TargetDir string       // Target directory for extraction (e.g., ~/.pchain/data)
	Progress  ProgressFunc // Optional progress callback
}

// Service downloads and extracts blockchain snapshots.
type Service interface {
	// Download fetches and caches a snapshot tarball (does not extract).
	Download(ctx context.Context, opts Options) error
	// Extract extracts the cached snapshot to the target directory.
	Extract(ctx context.Context, opts ExtractOptions) error
	// IsCacheValid checks if the cached snapshot matches the remote checksum.
	IsCacheValid(ctx context.Context, opts Options) (bool, error)
}

// HTTPDoer interface for HTTP requests (allows mocking in tests).
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type svc struct {
	http HTTPDoer
}

// New creates a new snapshot service with default HTTP client.
func New() Service {
	return &svc{
		http: &http.Client{
			Timeout: 0, // No timeout for large downloads
			Transport: &http.Transport{
				ResponseHeaderTimeout: 30 * time.Second,
				IdleConnTimeout:       90 * time.Second,
			},
		},
	}
}

// NewWith creates a snapshot service with custom HTTP client (for testing).
func NewWith(h HTTPDoer) Service {
	if h == nil {
		return New()
	}
	return &svc{http: h}
}

// CacheDir is the directory name for caching downloaded snapshot tarballs.
const CacheDir = "snapshot-cache"

// CachedTarball is the filename for the cached snapshot tarball.
const CachedTarball = "latest.tar.lz4"

// CachedChecksum is the filename for the cached checksum.
const CachedChecksum = "latest.tar.lz4.sha256"

// Retry constants for download resilience.
const (
	maxRetries     = 3
	initialBackoff = 2 * time.Second
	maxBackoff     = 30 * time.Second
)

// checkDiskSpace verifies that the filesystem containing path has at least
// requiredBytes of free space available. Returns nil if sufficient, error otherwise.
func checkDiskSpace(path string, requiredBytes int64) error {
	if requiredBytes <= 0 {
		return nil
	}

	// Ensure the path exists for statfs (use parent if path doesn't exist yet)
	checkPath := path
	for {
		if _, err := os.Stat(checkPath); err == nil {
			break
		}
		parent := filepath.Dir(checkPath)
		if parent == checkPath {
			break
		}
		checkPath = parent
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs(checkPath, &stat); err != nil {
		return fmt.Errorf("unable to check disk space: %w", err)
	}

	// Available space = blocks available to unprivileged users * block size
	available := int64(stat.Bavail) * int64(stat.Bsize)

	if available < requiredBytes {
		return fmt.Errorf("insufficient disk space: need %s, have %s available",
			formatBytesHuman(requiredBytes), formatBytesHuman(available))
	}

	return nil
}

// formatBytesHuman formats bytes into human-readable format (e.g., "6.5 GB").
func formatBytesHuman(b int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// IsSnapshotPresent checks if blockchain data already exists in the data directory.
// Returns true if the directory has significant blockchain state, indicating
// that a snapshot extraction is not needed.
func IsSnapshotPresent(homeDir string) bool {
	dataDir := filepath.Join(homeDir, "data")
	return hasBlockchainData(dataDir)
}

// hasBlockchainData checks if a directory contains blockchain state files.
func hasBlockchainData(dir string) bool {
	// Check if data directory has blockchain state (application.db, blockstore.db, or state.db)
	// These are the main database directories created after snapshot extraction
	markers := []string{"application.db", "blockstore.db", "state.db"}
	for _, marker := range markers {
		path := filepath.Join(dir, marker)
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		// If it's a directory, check if it has content (>1MB total indicates real data)
		if info.IsDir() {
			size, _ := dirSize(path)
			if size > 1024*1024 { // >1MB
				return true
			}
		} else if info.Size() > 1024*1024 { // >1MB file
			return true
		}
	}
	return false
}

// dirSize calculates the total size of all files in a directory.
func dirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// getCacheDir returns the path to the snapshot cache directory.
func getCacheDir(homeDir string) string {
	return filepath.Join(homeDir, CacheDir)
}

// getCachedTarballPath returns the path to the cached tarball file.
func getCachedTarballPath(homeDir string) string {
	return filepath.Join(getCacheDir(homeDir), CachedTarball)
}

// getCachedChecksumPath returns the path to the cached checksum file.
func getCachedChecksumPath(homeDir string) string {
	return filepath.Join(getCacheDir(homeDir), CachedChecksum)
}

// readCachedChecksum reads the stored checksum from cache.
func readCachedChecksum(homeDir string) (string, error) {
	checksumPath := getCachedChecksumPath(homeDir)
	data, err := os.ReadFile(checksumPath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// writeCachedChecksum writes the checksum to cache.
func writeCachedChecksum(homeDir, checksum string) error {
	checksumPath := getCachedChecksumPath(homeDir)
	return os.WriteFile(checksumPath, []byte(checksum), 0o644)
}

// isCacheValid checks if the cached tarball is valid by comparing checksums.
// Returns true if cache exists and matches the remote checksum.
func isCacheValid(homeDir, remoteChecksum string) bool {
	tarballPath := getCachedTarballPath(homeDir)

	// Check if tarball exists
	if _, err := os.Stat(tarballPath); os.IsNotExist(err) {
		return false
	}

	// Read stored checksum
	cachedChecksum, err := readCachedChecksum(homeDir)
	if err != nil {
		return false
	}

	// Compare checksums
	return cachedChecksum == remoteChecksum
}

// copyDir recursively copies a directory from src to dst.
// Used as fallback when os.Rename fails (cross-device move).
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		// Copy file
		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		dstFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		return err
	})
}

// Download fetches and caches a snapshot tarball (does not extract).
//
// Caching behavior:
// - Downloads are cached to HomeDir/snapshot-cache/latest.tar.lz4
// - Before downloading, compares remote checksum with cached checksum
// - If checksums match, skips download (cache is valid)
// - If checksums differ (new snapshot available), downloads and replaces cache
// - Use NoCache option to force fresh download
func (s *svc) Download(ctx context.Context, opts Options) error {
	if opts.HomeDir == "" {
		return fmt.Errorf("HomeDir required")
	}
	if opts.SnapshotURL == "" {
		opts.SnapshotURL = DefaultSnapshotURL
	}

	progress := opts.Progress
	if progress == nil {
		progress = func(ProgressPhase, int64, int64, string) {} // no-op
	}

	cacheDir := getCacheDir(opts.HomeDir)
	cachedTarball := getCachedTarballPath(opts.HomeDir)
	snapshotURL := opts.SnapshotURL + "/latest.tar.lz4"
	checksumURL := opts.SnapshotURL + "/latest.tar.lz4.sha256"

	// Step 1: Fetch remote checksum first (always needed to check for updates)
	progress(PhaseCache, 0, -1, "Fetching remote checksum...")
	remoteChecksum, err := s.fetchChecksum(ctx, checksumURL)
	if err != nil {
		return fmt.Errorf("fetch remote checksum: %w", err)
	}

	// Step 2: Check cache validity (unless NoCache is set)
	if !opts.NoCache && isCacheValid(opts.HomeDir, remoteChecksum) {
		progress(PhaseCache, 1, 1, "Snapshot cached (checksum matches remote)")
		return nil
	}

	// Step 3: Download to cache
	// Ensure cache directory exists
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}

	// Step 3a: Disk space pre-check (HEAD request to get Content-Length)
	progress(PhaseDownload, 0, -1, "Checking disk space...")
	headReq, err := http.NewRequestWithContext(ctx, http.MethodHead, snapshotURL, nil)
	if err == nil {
		if headResp, headErr := s.http.Do(headReq); headErr == nil {
			headResp.Body.Close()
			if headResp.ContentLength > 0 {
				if err := checkDiskSpace(cacheDir, headResp.ContentLength); err != nil {
					return fmt.Errorf("download disk space check: %w", err)
				}
			}
		}
	}

	// Check if partial exists and whether it's for the current snapshot
	partialPath := cachedTarball + ".partial"
	partialChecksumPath := partialPath + ".sha256"

	if _, err := os.Stat(partialPath); err == nil {
		// Partial file exists - check if it matches the current remote checksum
		if savedChecksum, readErr := os.ReadFile(partialChecksumPath); readErr == nil {
			if strings.TrimSpace(string(savedChecksum)) != remoteChecksum {
				// Newer snapshot available, discard stale partial
				progress(PhaseDownload, 0, -1, "Newer snapshot available, discarding stale partial download...")
				os.Remove(partialPath)
				os.Remove(partialChecksumPath)
			} else {
				progress(PhaseDownload, 0, -1, "Resuming interrupted download...")
			}
		} else {
			// No checksum marker (legacy partial) - resume anyway,
			// worst case: final checksum fails and we start fresh
			progress(PhaseDownload, 0, -1, "Resuming interrupted download...")
		}
	} else if _, err := os.Stat(cachedTarball); err == nil {
		progress(PhaseDownload, 0, -1, "New snapshot available, updating cache...")
	} else {
		progress(PhaseDownload, 0, -1, "Downloading snapshot to cache...")
	}

	// Save checksum marker alongside partial for stale detection on future resume
	os.WriteFile(partialChecksumPath, []byte(remoteChecksum), 0o644)

	// Download with retry and resume support
	if err := s.downloadWithRetry(ctx, snapshotURL, cachedTarball, func(current, total int64) {
		progress(PhaseDownload, current, total, "")
	}, progress); err != nil {
		return fmt.Errorf("download snapshot: %w", err)
	}

	// Clean up partial checksum marker (download complete, file renamed)
	os.Remove(partialChecksumPath)

	// Verify downloaded file
	progress(PhaseVerify, 0, 1, "Verifying checksum...")
	if err := verifyFile(cachedTarball, remoteChecksum); err != nil {
		// Remove corrupted download and partial file
		os.Remove(cachedTarball)
		os.Remove(partialPath)
		os.Remove(partialChecksumPath)
		return fmt.Errorf("checksum verification failed: %w", err)
	}
	progress(PhaseVerify, 1, 1, "Checksum verified")

	// Save checksum to cache
	if err := writeCachedChecksum(opts.HomeDir, remoteChecksum); err != nil {
		// Non-fatal, just log
		progress(PhaseCache, 0, -1, "Warning: could not save checksum to cache")
	}

	return nil
}

// IsCacheValid checks if the cached snapshot matches the remote checksum.
func (s *svc) IsCacheValid(ctx context.Context, opts Options) (bool, error) {
	if opts.HomeDir == "" {
		return false, fmt.Errorf("HomeDir required")
	}
	if opts.SnapshotURL == "" {
		opts.SnapshotURL = DefaultSnapshotURL
	}

	checksumURL := opts.SnapshotURL + "/latest.tar.lz4.sha256"
	remoteChecksum, err := s.fetchChecksum(ctx, checksumURL)
	if err != nil {
		return false, fmt.Errorf("fetch remote checksum: %w", err)
	}

	return isCacheValid(opts.HomeDir, remoteChecksum), nil
}

// Extract extracts the cached snapshot directly to the target directory.
// The target directory should be the node's data directory (e.g., ~/.pchain/data).
// This preserves priv_validator_state.json if it exists.
func (s *svc) Extract(ctx context.Context, opts ExtractOptions) error {
	if opts.HomeDir == "" {
		return fmt.Errorf("HomeDir required")
	}
	if opts.TargetDir == "" {
		opts.TargetDir = filepath.Join(opts.HomeDir, "data")
	}

	progress := opts.Progress
	if progress == nil {
		progress = func(ProgressPhase, int64, int64, string) {} // no-op
	}

	cachedTarball := getCachedTarballPath(opts.HomeDir)

	// Check if cache exists
	if _, err := os.Stat(cachedTarball); os.IsNotExist(err) {
		return fmt.Errorf("no cached snapshot found, run 'snapshot download' first")
	}

	// Pre-extract integrity verification
	progress(PhaseVerify, 0, 1, "Verifying snapshot integrity before extraction...")
	cachedChecksum, checksumErr := readCachedChecksum(opts.HomeDir)
	if checksumErr != nil {
		progress(PhaseVerify, 0, -1, "Warning: no checksum file found, skipping integrity check")
	} else {
		if err := verifyFile(cachedTarball, cachedChecksum); err != nil {
			// Remove corrupted cache
			os.Remove(cachedTarball)
			os.Remove(getCachedChecksumPath(opts.HomeDir))
			return fmt.Errorf("cached snapshot is corrupted (checksum mismatch), please re-download with 'snapshot download --no-cache': %w", err)
		}
		progress(PhaseVerify, 1, 1, "Integrity verified")
	}

	// Disk space pre-check for extraction
	progress(PhaseExtract, 0, -1, "Checking disk space...")
	if tarballInfo, err := os.Stat(cachedTarball); err == nil {
		// lz4 typical compression ratio ~3-4x for blockchain data
		estimatedSize := tarballInfo.Size() * 4
		if err := checkDiskSpace(opts.TargetDir, estimatedSize); err != nil {
			return fmt.Errorf("extraction disk space check: %w", err)
		}
	}

	// Backup priv_validator_state.json if it exists
	privValStatePath := filepath.Join(opts.TargetDir, "priv_validator_state.json")
	var privValStateBackup []byte
	if data, err := os.ReadFile(privValStatePath); err == nil {
		privValStateBackup = data
	}

	progress(PhaseExtract, 0, -1, "Extracting snapshot...")

	// Create temp directory for extraction (tarball contains data/ prefix)
	extractDir, err := os.MkdirTemp("", "snapshot-extract-*")
	if err != nil {
		return fmt.Errorf("create extract dir: %w", err)
	}
	defer os.RemoveAll(extractDir)

	if err := extractTarLz4(cachedTarball, extractDir, func(current, total int64, name string) {
		progress(PhaseExtract, current, total, name)
	}); err != nil {
		return fmt.Errorf("extract snapshot: %w", err)
	}

	// Move extracted data/ to target directory
	progress(PhaseExtract, 0, -1, "Moving to data directory...")
	extractedDataDir := filepath.Join(extractDir, "data")
	if _, err := os.Stat(extractedDataDir); os.IsNotExist(err) {
		return fmt.Errorf("extracted snapshot missing data/ directory")
	}

	// Prepare target directory (clear existing contents except priv_validator_state.json)
	if err := prepareDataDir(opts.TargetDir); err != nil {
		return fmt.Errorf("prepare target dir: %w", err)
	}

	// Copy extracted data to target
	if err := copyDir(extractedDataDir, opts.TargetDir); err != nil {
		return fmt.Errorf("copy to target: %w", err)
	}

	// Restore priv_validator_state.json if it was backed up
	if privValStateBackup != nil {
		if err := os.WriteFile(privValStatePath, privValStateBackup, 0o600); err != nil {
			progress(PhaseExtract, 0, -1, "Warning: could not restore priv_validator_state.json")
		}
	}

	progress(PhaseExtract, 1, 1, "Extraction complete")
	return nil
}

// downloadFile downloads a file from URL to destPath with resume support.
// Uses a .partial suffix file during download; renames to destPath on success.
// If a .partial file exists from a previous interrupted download, attempts to
// resume using HTTP Range headers.
func (s *svc) downloadFile(ctx context.Context, url, destPath string, progress func(current, total int64)) error {
	partialPath := destPath + ".partial"
	var startOffset int64

	// Check for existing partial download
	if info, err := os.Stat(partialPath); err == nil && info.Size() > 0 {
		startOffset = info.Size()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	if startOffset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", startOffset))
	}

	resp, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var out *os.File
	var totalSize int64

	switch resp.StatusCode {
	case http.StatusPartialContent: // 206 - server supports resume
		totalSize = startOffset + resp.ContentLength
		out, err = os.OpenFile(partialPath, os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return fmt.Errorf("open partial file for append: %w", err)
		}

	case http.StatusOK: // 200 - full download (server may not support Range, or no partial)
		startOffset = 0
		totalSize = resp.ContentLength
		out, err = os.Create(partialPath)
		if err != nil {
			return fmt.Errorf("create download file: %w", err)
		}

	case http.StatusRequestedRangeNotSatisfiable: // 416 - range invalid
		// Partial file may be corrupt or already complete, start fresh
		os.Remove(partialPath)
		resp.Body.Close()
		// Re-request without Range header
		req2, err2 := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err2 != nil {
			return err2
		}
		resp2, err2 := s.http.Do(req2)
		if err2 != nil {
			return err2
		}
		defer resp2.Body.Close()
		if resp2.StatusCode != http.StatusOK {
			return fmt.Errorf("HTTP %d: %s", resp2.StatusCode, resp2.Status)
		}
		resp = resp2
		startOffset = 0
		totalSize = resp.ContentLength
		out, err = os.Create(partialPath)
		if err != nil {
			return fmt.Errorf("create download file: %w", err)
		}

	default:
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	defer out.Close()

	var reader io.Reader = resp.Body
	if progress != nil {
		reader = &progressReader{
			reader:   resp.Body,
			total:    totalSize,
			current:  startOffset,
			progress: progress,
		}
	}

	if _, err = io.Copy(out, reader); err != nil {
		// Keep partial file for resume on next attempt
		return err
	}

	// Close file before rename
	out.Close()

	// Rename .partial to final path
	if err := os.Rename(partialPath, destPath); err != nil {
		return fmt.Errorf("finalize download: %w", err)
	}

	return nil
}

// downloadWithRetry wraps downloadFile with exponential backoff retry logic.
// On each retry, the resume logic in downloadFile picks up from where it left off.
func (s *svc) downloadWithRetry(ctx context.Context, url, destPath string, progress func(current, total int64), phaseProgress ProgressFunc) error {
	var lastErr error
	backoff := initialBackoff

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			phaseProgress(PhaseDownload, 0, -1, fmt.Sprintf("Retry %d/%d (waiting %v)...", attempt, maxRetries, backoff))

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}

			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}

		lastErr = s.downloadFile(ctx, url, destPath, progress)
		if lastErr == nil {
			return nil
		}

		// Don't retry on context cancellation
		if ctx.Err() != nil {
			return lastErr
		}

		phaseProgress(PhaseDownload, 0, -1, fmt.Sprintf("Download interrupted: %v", lastErr))
	}

	return fmt.Errorf("download failed after %d attempts: %w", maxRetries+1, lastErr)
}

// fetchChecksum downloads and parses the checksum file.
func (s *svc) fetchChecksum(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := s.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return parseChecksumFile(resp.Body)
}

// prepareDataDir clears the data directory while preserving critical files.
func prepareDataDir(dataDir string) error {
	// Files to preserve
	preserve := map[string]bool{
		"priv_validator_state.json": true,
	}

	entries, err := os.ReadDir(dataDir)
	if os.IsNotExist(err) {
		return os.MkdirAll(dataDir, 0o755)
	}
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if preserve[entry.Name()] {
			continue
		}
		path := filepath.Join(dataDir, entry.Name())
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("remove %s: %w", entry.Name(), err)
		}
	}

	return nil
}

// progressReader wraps a reader to report download progress.
type progressReader struct {
	reader   io.Reader
	total    int64
	current  int64
	progress func(current, total int64)
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.current += int64(n)
	if pr.progress != nil {
		pr.progress(pr.current, pr.total)
	}
	return n, err
}

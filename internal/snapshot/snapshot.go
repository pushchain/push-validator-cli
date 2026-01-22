package snapshot

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// DefaultSnapshotURL is the default base URL for snapshot downloads.
const DefaultSnapshotURL = "https://snapshots.donut.push.org"

// ProgressPhase indicates which phase of the snapshot process is active.
type ProgressPhase string

const (
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
}

// Service downloads and extracts blockchain snapshots.
type Service interface {
	// Download fetches, verifies, and extracts a snapshot to the node's data directory.
	Download(ctx context.Context, opts Options) error
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

// Download fetches, verifies, and extracts a snapshot.
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

	dataDir := filepath.Join(opts.HomeDir, "data")
	snapshotURL := opts.SnapshotURL + "/latest.tar.lz4"
	checksumURL := opts.SnapshotURL + "/latest.tar.lz4.sha256"

	// Create temp directory for download
	tempDir, err := os.MkdirTemp("", "snapshot-download-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	tempFile := filepath.Join(tempDir, "snapshot.tar.lz4")

	// Step 1: Download snapshot with progress
	progress(PhaseDownload, 0, -1, "Starting download...")
	if err := s.downloadFile(ctx, snapshotURL, tempFile, func(current, total int64) {
		progress(PhaseDownload, current, total, "")
	}); err != nil {
		return fmt.Errorf("download snapshot: %w", err)
	}

	// Step 2: Verify checksum
	progress(PhaseVerify, 0, 1, "Verifying checksum...")
	expectedHash, err := s.fetchChecksum(ctx, checksumURL)
	if err != nil {
		return fmt.Errorf("fetch checksum: %w", err)
	}
	if err := verifyFile(tempFile, expectedHash); err != nil {
		return fmt.Errorf("checksum verification failed: %w", err)
	}
	progress(PhaseVerify, 1, 1, "Checksum verified")

	// Step 3: Clear existing data directory (preserve priv_validator_state.json)
	progress(PhaseExtract, 0, -1, "Preparing data directory...")
	if err := prepareDataDir(dataDir); err != nil {
		return fmt.Errorf("prepare data dir: %w", err)
	}

	// Step 4: Extract snapshot
	progress(PhaseExtract, 0, -1, "Extracting snapshot...")
	if err := extractTarLz4(tempFile, opts.HomeDir, func(current, total int64, name string) {
		progress(PhaseExtract, current, total, name)
	}); err != nil {
		return fmt.Errorf("extract snapshot: %w", err)
	}

	progress(PhaseExtract, 1, 1, "Extraction complete")
	return nil
}

// downloadFile downloads a file from URL to destPath with progress callback.
func (s *svc) downloadFile(ctx context.Context, url, destPath string, progress func(current, total int64)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	var reader io.Reader = resp.Body
	if progress != nil {
		reader = &progressReader{
			reader:   resp.Body,
			total:    resp.ContentLength,
			progress: progress,
		}
	}

	_, err = io.Copy(out, reader)
	return err
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

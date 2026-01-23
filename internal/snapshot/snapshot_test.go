package snapshot

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pierrec/lz4/v4"
)

// mockHTTPDoer implements HTTPDoer for testing
type mockHTTPDoer struct {
	responses map[string]*http.Response
	err       error
}

func (m *mockHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	if resp, ok := m.responses[req.URL.String()]; ok {
		return resp, nil
	}
	return &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       io.NopCloser(strings.NewReader("")),
	}, nil
}

// customHTTPDoer allows custom Do function for more complex testing scenarios
type customHTTPDoer struct {
	doFunc func(*http.Request) (*http.Response, error)
}

func (c *customHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	return c.doFunc(req)
}

func makeResponse(statusCode int, body string, headers map[string]string) *http.Response {
	resp := &http.Response{
		StatusCode:    statusCode,
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
		Header:        make(http.Header),
	}
	for k, v := range headers {
		resp.Header.Set(k, v)
	}
	return resp
}

func TestNewWith(t *testing.T) {
	t.Run("WithCustomDoer", func(t *testing.T) {
		mock := &mockHTTPDoer{}
		service := NewWith(mock)
		if service == nil {
			t.Fatal("NewWith returned nil")
		}
		// Service should be created successfully
		// We'll test actual functionality in other tests
	})

	t.Run("WithNilDoer", func(t *testing.T) {
		service := NewWith(nil)
		if service == nil {
			t.Fatal("NewWith(nil) returned nil")
		}
		// Should fall back to default HTTP client
		// We'll test actual functionality in other tests
	})
}

func TestIsSnapshotPresent(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(homeDir string)
		expected bool
	}{
		{
			name: "NoDataDir",
			setup: func(homeDir string) {
				// Don't create data directory
			},
			expected: false,
		},
		{
			name: "EmptyDataDir",
			setup: func(homeDir string) {
				os.MkdirAll(filepath.Join(homeDir, "data"), 0o755)
			},
			expected: false,
		},
		{
			name: "HasApplicationDB",
			setup: func(homeDir string) {
				dataDir := filepath.Join(homeDir, "data")
				os.MkdirAll(dataDir, 0o755)
				dbDir := filepath.Join(dataDir, "application.db")
				os.MkdirAll(dbDir, 0o755)
				// Create a file >1MB
				f, _ := os.Create(filepath.Join(dbDir, "data.db"))
				f.Write(make([]byte, 2*1024*1024))
				f.Close()
			},
			expected: true,
		},
		{
			name: "HasBlockstoreDB",
			setup: func(homeDir string) {
				dataDir := filepath.Join(homeDir, "data")
				os.MkdirAll(dataDir, 0o755)
				dbDir := filepath.Join(dataDir, "blockstore.db")
				os.MkdirAll(dbDir, 0o755)
				// Create a file >1MB
				f, _ := os.Create(filepath.Join(dbDir, "data.db"))
				f.Write(make([]byte, 2*1024*1024))
				f.Close()
			},
			expected: true,
		},
		{
			name: "HasStateDB",
			setup: func(homeDir string) {
				dataDir := filepath.Join(homeDir, "data")
				os.MkdirAll(dataDir, 0o755)
				dbDir := filepath.Join(dataDir, "state.db")
				os.MkdirAll(dbDir, 0o755)
				// Create a file >1MB
				f, _ := os.Create(filepath.Join(dbDir, "data.db"))
				f.Write(make([]byte, 2*1024*1024))
				f.Close()
			},
			expected: true,
		},
		{
			name: "SmallFiles",
			setup: func(homeDir string) {
				dataDir := filepath.Join(homeDir, "data")
				os.MkdirAll(dataDir, 0o755)
				dbDir := filepath.Join(dataDir, "application.db")
				os.MkdirAll(dbDir, 0o755)
				// Create a file <1MB
				f, _ := os.Create(filepath.Join(dbDir, "data.db"))
				f.Write(make([]byte, 100))
				f.Close()
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			homeDir := t.TempDir()
			tt.setup(homeDir)
			result := IsSnapshotPresent(homeDir)
			if result != tt.expected {
				t.Errorf("IsSnapshotPresent() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDownload(t *testing.T) {
	t.Run("Success_NoCacheExists", func(t *testing.T) {
		homeDir := t.TempDir()
		tarballContent := "mock tarball data"
		// SHA256 of "mock tarball data"
		expectedChecksum := "a489343b0d32489b237895996aa623a6662cc8c17a5e01287b9218d1f2ac4408"
		checksumContent := expectedChecksum + "  latest.tar.lz4"

		mock := &mockHTTPDoer{
			responses: map[string]*http.Response{
				"https://snapshots.donut.push.org/latest.tar.lz4.sha256": makeResponse(
					http.StatusOK,
					checksumContent,
					nil,
				),
				"https://snapshots.donut.push.org/latest.tar.lz4": makeResponse(
					http.StatusOK,
					tarballContent,
					map[string]string{
						"Content-Length": "17",
					},
				),
			},
		}

		svc := NewWith(mock)
		opts := Options{
			HomeDir:     homeDir,
			SnapshotURL: "https://snapshots.donut.push.org",
		}

		err := svc.Download(context.Background(), opts)
		if err != nil {
			t.Fatalf("Download() error = %v", err)
		}

		// Verify tarball was written
		tarballPath := getCachedTarballPath(homeDir)
		data, err := os.ReadFile(tarballPath)
		if err != nil {
			t.Fatalf("failed to read tarball: %v", err)
		}
		if string(data) != tarballContent {
			t.Errorf("tarball content = %q, want %q", string(data), tarballContent)
		}

		// Verify checksum was written
		checksumPath := getCachedChecksumPath(homeDir)
		checksumData, err := os.ReadFile(checksumPath)
		if err != nil {
			t.Fatalf("failed to read checksum: %v", err)
		}
		if strings.TrimSpace(string(checksumData)) != expectedChecksum {
			t.Errorf("checksum = %q, want %q", string(checksumData), expectedChecksum)
		}
	})

	t.Run("Success_ValidCacheExists", func(t *testing.T) {
		homeDir := t.TempDir()
		checksumContent := "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789  latest.tar.lz4"
		tarballContent := "mock tarball data"

		// Pre-populate cache
		cacheDir := getCacheDir(homeDir)
		os.MkdirAll(cacheDir, 0o755)
		os.WriteFile(getCachedTarballPath(homeDir), []byte(tarballContent), 0o644)
		os.WriteFile(getCachedChecksumPath(homeDir), []byte("abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"), 0o644)

		mock := &mockHTTPDoer{
			responses: map[string]*http.Response{
				"https://snapshots.donut.push.org/latest.tar.lz4.sha256": makeResponse(
					http.StatusOK,
					checksumContent,
					nil,
				),
			},
		}

		svc := NewWith(mock)
		opts := Options{
			HomeDir:     homeDir,
			SnapshotURL: "https://snapshots.donut.push.org",
		}

		err := svc.Download(context.Background(), opts)
		if err != nil {
			t.Fatalf("Download() error = %v", err)
		}

		// Verify tarball was NOT re-downloaded (cache was used)
		data, _ := os.ReadFile(getCachedTarballPath(homeDir))
		if string(data) != tarballContent {
			t.Error("cache should have been used")
		}
	})

	t.Run("Success_InvalidCacheExists", func(t *testing.T) {
		homeDir := t.TempDir()
		tarballContent := "new tarball data"
		// SHA256 of "new tarball data"
		expectedChecksum := "601bd3f71eacaad36ada3594e7969041e8cd55dbf41f0b10e2ea95e3bd1cf3f6"
		checksumContent := expectedChecksum + "  latest.tar.lz4"

		// Pre-populate cache with old checksum
		cacheDir := getCacheDir(homeDir)
		os.MkdirAll(cacheDir, 0o755)
		os.WriteFile(getCachedTarballPath(homeDir), []byte("old tarball"), 0o644)
		os.WriteFile(getCachedChecksumPath(homeDir), []byte("fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321"), 0o644)

		mock := &mockHTTPDoer{
			responses: map[string]*http.Response{
				"https://snapshots.donut.push.org/latest.tar.lz4.sha256": makeResponse(
					http.StatusOK,
					checksumContent,
					nil,
				),
				"https://snapshots.donut.push.org/latest.tar.lz4": makeResponse(
					http.StatusOK,
					tarballContent,
					map[string]string{
						"Content-Length": "16",
					},
				),
			},
		}

		svc := NewWith(mock)
		opts := Options{
			HomeDir:     homeDir,
			SnapshotURL: "https://snapshots.donut.push.org",
		}

		err := svc.Download(context.Background(), opts)
		if err != nil {
			t.Fatalf("Download() error = %v", err)
		}

		// Verify tarball was updated
		data, _ := os.ReadFile(getCachedTarballPath(homeDir))
		if string(data) != tarballContent {
			t.Errorf("tarball should have been updated, got %q", string(data))
		}
	})

	t.Run("Error_NoHomeDir", func(t *testing.T) {
		svc := NewWith(&mockHTTPDoer{})
		err := svc.Download(context.Background(), Options{})
		if err == nil || !strings.Contains(err.Error(), "HomeDir required") {
			t.Errorf("expected HomeDir required error, got %v", err)
		}
	})

	t.Run("Error_ChecksumFetchFailed", func(t *testing.T) {
		homeDir := t.TempDir()
		mock := &mockHTTPDoer{
			responses: map[string]*http.Response{
				"https://snapshots.donut.push.org/latest.tar.lz4.sha256": makeResponse(
					http.StatusInternalServerError,
					"",
					nil,
				),
			},
		}

		svc := NewWith(mock)
		opts := Options{
			HomeDir:     homeDir,
			SnapshotURL: "https://snapshots.donut.push.org",
		}

		err := svc.Download(context.Background(), opts)
		if err == nil {
			t.Error("expected error when checksum fetch fails")
		}
	})

	t.Run("NoCache_Option", func(t *testing.T) {
		homeDir := t.TempDir()
		tarballContent := "new tarball data"
		// SHA256 of "new tarball data"
		expectedChecksum := "601bd3f71eacaad36ada3594e7969041e8cd55dbf41f0b10e2ea95e3bd1cf3f6"
		checksumContent := expectedChecksum + "  latest.tar.lz4"

		// Pre-populate cache
		cacheDir := getCacheDir(homeDir)
		os.MkdirAll(cacheDir, 0o755)
		os.WriteFile(getCachedTarballPath(homeDir), []byte("old tarball"), 0o644)
		os.WriteFile(getCachedChecksumPath(homeDir), []byte(expectedChecksum), 0o644)

		mock := &mockHTTPDoer{
			responses: map[string]*http.Response{
				"https://snapshots.donut.push.org/latest.tar.lz4.sha256": makeResponse(
					http.StatusOK,
					checksumContent,
					nil,
				),
				"https://snapshots.donut.push.org/latest.tar.lz4": makeResponse(
					http.StatusOK,
					tarballContent,
					map[string]string{
						"Content-Length": "16",
					},
				),
			},
		}

		svc := NewWith(mock)
		opts := Options{
			HomeDir:     homeDir,
			SnapshotURL: "https://snapshots.donut.push.org",
			NoCache:     true,
		}

		err := svc.Download(context.Background(), opts)
		if err != nil {
			t.Fatalf("Download() error = %v", err)
		}

		// Verify tarball was re-downloaded despite matching checksum
		data, _ := os.ReadFile(getCachedTarballPath(homeDir))
		if string(data) != tarballContent {
			t.Errorf("tarball should have been re-downloaded with NoCache, got %q", string(data))
		}
	})
}

func TestIsCacheValid(t *testing.T) {
	t.Run("ValidCache", func(t *testing.T) {
		homeDir := t.TempDir()
		checksumContent := "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789  latest.tar.lz4"

		// Pre-populate cache
		cacheDir := getCacheDir(homeDir)
		os.MkdirAll(cacheDir, 0o755)
		os.WriteFile(getCachedTarballPath(homeDir), []byte("tarball data"), 0o644)
		os.WriteFile(getCachedChecksumPath(homeDir), []byte("abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"), 0o644)

		mock := &mockHTTPDoer{
			responses: map[string]*http.Response{
				"https://snapshots.donut.push.org/latest.tar.lz4.sha256": makeResponse(
					http.StatusOK,
					checksumContent,
					nil,
				),
			},
		}

		svc := NewWith(mock)
		opts := Options{
			HomeDir:     homeDir,
			SnapshotURL: "https://snapshots.donut.push.org",
		}

		valid, err := svc.IsCacheValid(context.Background(), opts)
		if err != nil {
			t.Fatalf("IsCacheValid() error = %v", err)
		}
		if !valid {
			t.Error("expected cache to be valid")
		}
	})

	t.Run("InvalidCache", func(t *testing.T) {
		homeDir := t.TempDir()
		checksumContent := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef  latest.tar.lz4"

		// Pre-populate cache with old checksum
		cacheDir := getCacheDir(homeDir)
		os.MkdirAll(cacheDir, 0o755)
		os.WriteFile(getCachedTarballPath(homeDir), []byte("tarball data"), 0o644)
		os.WriteFile(getCachedChecksumPath(homeDir), []byte("fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321"), 0o644)

		mock := &mockHTTPDoer{
			responses: map[string]*http.Response{
				"https://snapshots.donut.push.org/latest.tar.lz4.sha256": makeResponse(
					http.StatusOK,
					checksumContent,
					nil,
				),
			},
		}

		svc := NewWith(mock)
		opts := Options{
			HomeDir:     homeDir,
			SnapshotURL: "https://snapshots.donut.push.org",
		}

		valid, err := svc.IsCacheValid(context.Background(), opts)
		if err != nil {
			t.Fatalf("IsCacheValid() error = %v", err)
		}
		if valid {
			t.Error("expected cache to be invalid")
		}
	})

	t.Run("NoCache", func(t *testing.T) {
		homeDir := t.TempDir()
		checksumContent := "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789  latest.tar.lz4"

		mock := &mockHTTPDoer{
			responses: map[string]*http.Response{
				"https://snapshots.donut.push.org/latest.tar.lz4.sha256": makeResponse(
					http.StatusOK,
					checksumContent,
					nil,
				),
			},
		}

		svc := NewWith(mock)
		opts := Options{
			HomeDir:     homeDir,
			SnapshotURL: "https://snapshots.donut.push.org",
		}

		valid, err := svc.IsCacheValid(context.Background(), opts)
		if err != nil {
			t.Fatalf("IsCacheValid() error = %v", err)
		}
		if valid {
			t.Error("expected cache to be invalid when it doesn't exist")
		}
	})

	t.Run("Error_NoHomeDir", func(t *testing.T) {
		svc := NewWith(&mockHTTPDoer{})
		_, err := svc.IsCacheValid(context.Background(), Options{})
		if err == nil || !strings.Contains(err.Error(), "HomeDir required") {
			t.Errorf("expected HomeDir required error, got %v", err)
		}
	})
}

func TestFormatBytesHuman(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{100, "100 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{1024 * 1024 * 1536 / 1024, "1.5 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
		{1024*1024*1024*3 + 512*1024*1024, "3.5 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatBytesHuman(tt.bytes)
			if result != tt.expected {
				t.Errorf("formatBytesHuman(%d) = %q, want %q", tt.bytes, result, tt.expected)
			}
		})
	}
}

func TestGetCacheDir(t *testing.T) {
	homeDir := "/home/test"
	expected := "/home/test/snapshot-cache"
	result := getCacheDir(homeDir)
	if result != expected {
		t.Errorf("getCacheDir(%q) = %q, want %q", homeDir, result, expected)
	}
}

func TestGetCachedTarballPath(t *testing.T) {
	homeDir := "/home/test"
	expected := "/home/test/snapshot-cache/latest.tar.lz4"
	result := getCachedTarballPath(homeDir)
	if result != expected {
		t.Errorf("getCachedTarballPath(%q) = %q, want %q", homeDir, result, expected)
	}
}

func TestGetCachedChecksumPath(t *testing.T) {
	homeDir := "/home/test"
	expected := "/home/test/snapshot-cache/latest.tar.lz4.sha256"
	result := getCachedChecksumPath(homeDir)
	if result != expected {
		t.Errorf("getCachedChecksumPath(%q) = %q, want %q", homeDir, result, expected)
	}
}

func TestProgressReader(t *testing.T) {
	data := []byte("hello world")
	reader := bytes.NewReader(data)

	var progressCalls []struct {
		current, total int64
	}

	pr := &progressReader{
		reader: reader,
		total:  int64(len(data)),
		progress: func(current, total int64) {
			progressCalls = append(progressCalls, struct{ current, total int64 }{current, total})
		},
	}

	buf := make([]byte, 5)
	n, err := pr.Read(buf)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if n != 5 {
		t.Errorf("Read() = %d bytes, want 5", n)
	}
	if len(progressCalls) != 1 {
		t.Errorf("expected 1 progress call, got %d", len(progressCalls))
	}
	if progressCalls[0].current != 5 {
		t.Errorf("progress current = %d, want 5", progressCalls[0].current)
	}
	if progressCalls[0].total != 11 {
		t.Errorf("progress total = %d, want 11", progressCalls[0].total)
	}
}

func TestDirSize(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some test files
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("hello"), 0o644)
	os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("world!"), 0o644)

	subDir := filepath.Join(tmpDir, "subdir")
	os.MkdirAll(subDir, 0o755)
	os.WriteFile(filepath.Join(subDir, "file3.txt"), []byte("test"), 0o644)

	size, err := dirSize(tmpDir)
	if err != nil {
		t.Fatalf("dirSize() error = %v", err)
	}

	expectedSize := int64(5 + 6 + 4) // "hello" + "world!" + "test"
	if size != expectedSize {
		t.Errorf("dirSize() = %d, want %d", size, expectedSize)
	}
}

func TestHasBlockchainData(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(dir string)
		expected bool
	}{
		{
			name: "NoMarkerFiles",
			setup: func(dir string) {
				os.MkdirAll(dir, 0o755)
			},
			expected: false,
		},
		{
			name: "ApplicationDBWithLargeFile",
			setup: func(dir string) {
				dbDir := filepath.Join(dir, "application.db")
				os.MkdirAll(dbDir, 0o755)
				f, _ := os.Create(filepath.Join(dbDir, "data.db"))
				f.Write(make([]byte, 2*1024*1024))
				f.Close()
			},
			expected: true,
		},
		{
			name: "ApplicationDBWithSmallFile",
			setup: func(dir string) {
				dbDir := filepath.Join(dir, "application.db")
				os.MkdirAll(dbDir, 0o755)
				f, _ := os.Create(filepath.Join(dbDir, "data.db"))
				f.Write(make([]byte, 100))
				f.Close()
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tt.setup(tmpDir)
			result := hasBlockchainData(tmpDir)
			if result != tt.expected {
				t.Errorf("hasBlockchainData() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExtract(t *testing.T) {
	t.Run("Success_ExtractCachedSnapshot", func(t *testing.T) {
		homeDir := t.TempDir()
		targetDir := filepath.Join(homeDir, "data")

		// Create a test tarball
		tarballPath := getCachedTarballPath(homeDir)
		os.MkdirAll(filepath.Dir(tarballPath), 0o755)

		// Use the helper from extractor_test to create a real tar.lz4
		files := map[string]string{
			"data/":          "",
			"data/file1.txt": "content1",
			"data/file2.txt": "content2",
		}
		createTestTarLz4ForExtract(t, tarballPath, files)

		// Compute checksum
		tarballData, _ := os.ReadFile(tarballPath)
		checksum := computeSHA256(tarballData)
		os.WriteFile(getCachedChecksumPath(homeDir), []byte(checksum), 0o644)

		svc := NewWith(&mockHTTPDoer{})
		opts := ExtractOptions{
			HomeDir: homeDir,
		}

		err := svc.Extract(context.Background(), opts)
		if err != nil {
			t.Fatalf("Extract() error = %v", err)
		}

		// Verify files were extracted
		content, err := os.ReadFile(filepath.Join(targetDir, "file1.txt"))
		if err != nil {
			t.Fatalf("failed to read extracted file: %v", err)
		}
		if string(content) != "content1" {
			t.Errorf("extracted content = %q, want %q", string(content), "content1")
		}
	})

	t.Run("Error_NoHomeDir", func(t *testing.T) {
		svc := NewWith(&mockHTTPDoer{})
		err := svc.Extract(context.Background(), ExtractOptions{})
		if err == nil || !strings.Contains(err.Error(), "HomeDir required") {
			t.Errorf("expected HomeDir required error, got %v", err)
		}
	})

	t.Run("Error_NoCachedSnapshot", func(t *testing.T) {
		homeDir := t.TempDir()
		svc := NewWith(&mockHTTPDoer{})
		opts := ExtractOptions{
			HomeDir: homeDir,
		}

		err := svc.Extract(context.Background(), opts)
		if err == nil || !strings.Contains(err.Error(), "no cached snapshot found") {
			t.Errorf("expected no cached snapshot error, got %v", err)
		}
	})

	t.Run("Success_PreservesPrivValidatorState", func(t *testing.T) {
		homeDir := t.TempDir()
		targetDir := filepath.Join(homeDir, "data")
		os.MkdirAll(targetDir, 0o755)

		// Create priv_validator_state.json
		privValState := `{"height":"100","round":0}`
		privValPath := filepath.Join(targetDir, "priv_validator_state.json")
		os.WriteFile(privValPath, []byte(privValState), 0o600)

		// Create a test tarball
		tarballPath := getCachedTarballPath(homeDir)
		os.MkdirAll(filepath.Dir(tarballPath), 0o755)
		files := map[string]string{
			"data/":          "",
			"data/file1.txt": "content1",
		}
		createTestTarLz4ForExtract(t, tarballPath, files)

		// Compute checksum
		tarballData, _ := os.ReadFile(tarballPath)
		checksum := computeSHA256(tarballData)
		os.WriteFile(getCachedChecksumPath(homeDir), []byte(checksum), 0o644)

		svc := NewWith(&mockHTTPDoer{})
		err := svc.Extract(context.Background(), ExtractOptions{
			HomeDir: homeDir,
		})
		if err != nil {
			t.Fatalf("Extract() error = %v", err)
		}

		// Verify priv_validator_state.json was preserved
		restoredState, err := os.ReadFile(privValPath)
		if err != nil {
			t.Fatalf("failed to read priv_validator_state.json: %v", err)
		}
		if string(restoredState) != privValState {
			t.Errorf("priv_validator_state.json = %q, want %q", string(restoredState), privValState)
		}
	})
}

func TestCopyDir(t *testing.T) {
	t.Run("Success_CopyDirectory", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := t.TempDir()

		// Create source files
		os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("content1"), 0o644)
		subDir := filepath.Join(srcDir, "subdir")
		os.MkdirAll(subDir, 0o755)
		os.WriteFile(filepath.Join(subDir, "file2.txt"), []byte("content2"), 0o600)

		err := copyDir(srcDir, dstDir)
		if err != nil {
			t.Fatalf("copyDir() error = %v", err)
		}

		// Verify files were copied
		content1, _ := os.ReadFile(filepath.Join(dstDir, "file1.txt"))
		if string(content1) != "content1" {
			t.Errorf("file1 content = %q, want %q", string(content1), "content1")
		}

		content2, _ := os.ReadFile(filepath.Join(dstDir, "subdir", "file2.txt"))
		if string(content2) != "content2" {
			t.Errorf("file2 content = %q, want %q", string(content2), "content2")
		}

		// Verify permissions were preserved
		info, _ := os.Stat(filepath.Join(dstDir, "subdir", "file2.txt"))
		if info.Mode()&0o777 != 0o600 {
			t.Errorf("file2 mode = %o, want %o", info.Mode()&0o777, 0o600)
		}
	})
}

func TestPrepareDataDir(t *testing.T) {
	t.Run("CreatesDirIfNotExists", func(t *testing.T) {
		tmpDir := t.TempDir()
		dataDir := filepath.Join(tmpDir, "data")

		err := prepareDataDir(dataDir)
		if err != nil {
			t.Fatalf("prepareDataDir() error = %v", err)
		}

		// Verify directory was created
		info, err := os.Stat(dataDir)
		if err != nil {
			t.Fatalf("data dir not created: %v", err)
		}
		if !info.IsDir() {
			t.Error("expected directory")
		}
	})

	t.Run("RemovesExistingFiles", func(t *testing.T) {
		tmpDir := t.TempDir()
		dataDir := filepath.Join(tmpDir, "data")
		os.MkdirAll(dataDir, 0o755)

		// Create some files
		os.WriteFile(filepath.Join(dataDir, "old.txt"), []byte("old"), 0o644)
		os.MkdirAll(filepath.Join(dataDir, "olddir"), 0o755)

		err := prepareDataDir(dataDir)
		if err != nil {
			t.Fatalf("prepareDataDir() error = %v", err)
		}

		// Verify files were removed
		if _, err := os.Stat(filepath.Join(dataDir, "old.txt")); !os.IsNotExist(err) {
			t.Error("old.txt should have been removed")
		}
		if _, err := os.Stat(filepath.Join(dataDir, "olddir")); !os.IsNotExist(err) {
			t.Error("olddir should have been removed")
		}
	})

	t.Run("PreservesPrivValidatorState", func(t *testing.T) {
		tmpDir := t.TempDir()
		dataDir := filepath.Join(tmpDir, "data")
		os.MkdirAll(dataDir, 0o755)

		// Create files including priv_validator_state.json
		os.WriteFile(filepath.Join(dataDir, "old.txt"), []byte("old"), 0o644)
		privValState := `{"height":"100"}`
		os.WriteFile(filepath.Join(dataDir, "priv_validator_state.json"), []byte(privValState), 0o600)

		err := prepareDataDir(dataDir)
		if err != nil {
			t.Fatalf("prepareDataDir() error = %v", err)
		}

		// Verify old file was removed
		if _, err := os.Stat(filepath.Join(dataDir, "old.txt")); !os.IsNotExist(err) {
			t.Error("old.txt should have been removed")
		}

		// Verify priv_validator_state.json was preserved
		content, err := os.ReadFile(filepath.Join(dataDir, "priv_validator_state.json"))
		if err != nil {
			t.Fatalf("priv_validator_state.json should have been preserved")
		}
		if string(content) != privValState {
			t.Errorf("priv_validator_state.json content = %q, want %q", string(content), privValState)
		}
	})
}

func TestDownloadFile(t *testing.T) {
	t.Run("Success_SimpleDownload", func(t *testing.T) {
		tmpDir := t.TempDir()
		destPath := filepath.Join(tmpDir, "file.txt")
		content := "test content"

		mock := &mockHTTPDoer{
			responses: map[string]*http.Response{
				"http://example.com/file.txt": makeResponse(
					http.StatusOK,
					content,
					nil,
				),
			},
		}

		svc := &svc{http: mock}
		err := svc.downloadFile(context.Background(), "http://example.com/file.txt", destPath, nil)
		if err != nil {
			t.Fatalf("downloadFile() error = %v", err)
		}

		// Verify file was downloaded
		data, _ := os.ReadFile(destPath)
		if string(data) != content {
			t.Errorf("downloaded content = %q, want %q", string(data), content)
		}
	})

	t.Run("Success_ResumeDownload", func(t *testing.T) {
		tmpDir := t.TempDir()
		destPath := filepath.Join(tmpDir, "file.txt")
		partialPath := destPath + ".partial"

		// Create partial file
		os.WriteFile(partialPath, []byte("test"), 0o644)

		mock := &mockHTTPDoer{
			responses: map[string]*http.Response{
				"http://example.com/file.txt": makeResponse(
					http.StatusPartialContent,
					" content",
					nil,
				),
			},
		}

		svc := &svc{http: mock}
		err := svc.downloadFile(context.Background(), "http://example.com/file.txt", destPath, nil)
		if err != nil {
			t.Fatalf("downloadFile() error = %v", err)
		}

		// Verify file was completed
		data, _ := os.ReadFile(destPath)
		if string(data) != "test content" {
			t.Errorf("resumed content = %q, want %q", string(data), "test content")
		}
	})

	t.Run("Success_RangeNotSatisfiable", func(t *testing.T) {
		tmpDir := t.TempDir()
		destPath := filepath.Join(tmpDir, "file.txt")
		partialPath := destPath + ".partial"

		// Create partial file
		os.WriteFile(partialPath, []byte("corrupted"), 0o644)

		// Create a custom mock that handles multiple calls
		callCount := 0
		customMock := &mockHTTPDoer{}
		customMock.responses = map[string]*http.Response{}

		originalFunc := customMock.Do
		doFunc := func(req *http.Request) (*http.Response, error) {
			callCount++
			if callCount == 1 {
				// First call with Range header returns 416
				return makeResponse(http.StatusRequestedRangeNotSatisfiable, "", nil), nil
			}
			// Second call without Range header succeeds
			return makeResponse(http.StatusOK, "full content", nil), nil
		}
		_ = originalFunc

		// Create new mock with custom behavior
		customMock2 := &customHTTPDoer{doFunc: doFunc}

		svc := &svc{http: customMock2}
		err := svc.downloadFile(context.Background(), "http://example.com/file.txt", destPath, nil)
		if err != nil {
			t.Fatalf("downloadFile() error = %v", err)
		}

		// Verify partial file was removed and full download succeeded
		if _, err := os.Stat(partialPath); !os.IsNotExist(err) {
			t.Error("partial file should have been removed")
		}

		data, _ := os.ReadFile(destPath)
		if string(data) != "full content" {
			t.Errorf("content = %q, want %q", string(data), "full content")
		}
	})
}

// Helper functions for tests
func createTestTarLz4ForExtract(t *testing.T, archivePath string, files map[string]string) {
	t.Helper()
	// Import and use the tar/lz4 creation logic
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("failed to create archive: %v", err)
	}
	defer f.Close()

	lz4Writer := lz4.NewWriter(f)
	defer lz4Writer.Close()

	tarWriter := tar.NewWriter(lz4Writer)
	defer tarWriter.Close()

	for name, content := range files {
		isDir := strings.HasSuffix(name, "/")
		mode := int64(0o644)
		typeflag := byte(tar.TypeReg)

		if isDir {
			mode = 0o755
			typeflag = tar.TypeDir
			content = ""
		}

		header := &tar.Header{
			Name:     name,
			Mode:     mode,
			Size:     int64(len(content)),
			Typeflag: typeflag,
		}

		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatalf("failed to write tar header: %v", err)
		}

		if !isDir && content != "" {
			if _, err := tarWriter.Write([]byte(content)); err != nil {
				t.Fatalf("failed to write tar content: %v", err)
			}
		}
	}
}

func computeSHA256(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

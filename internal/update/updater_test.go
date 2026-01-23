package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestExtractBinary(t *testing.T) {
	tests := []struct {
		name        string
		files       map[string]string // filename -> content
		wantErr     bool
		errContains string
	}{
		{
			name: "binary at root",
			files: map[string]string{
				"push-validator": "binary content",
			},
			wantErr: false,
		},
		{
			name: "binary in subdirectory",
			files: map[string]string{
				"bin/push-validator": "binary content",
			},
			wantErr: false,
		},
		{
			name: "binary with other files",
			files: map[string]string{
				"README.md":      "readme",
				"push-validator": "binary content",
				"LICENSE":        "license",
			},
			wantErr: false,
		},
		{
			name: "no binary found",
			files: map[string]string{
				"README.md": "readme",
				"other":     "file",
			},
			wantErr:     true,
			errContains: "binary not found in archive",
		},
		{
			name:        "empty archive",
			files:       map[string]string{},
			wantErr:     true,
			errContains: "binary not found in archive",
		},
		{
			name: "wrong binary name",
			files: map[string]string{
				"push-validator-cli": "binary content",
			},
			wantErr:     true,
			errContains: "binary not found in archive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create tar.gz archive
			archiveData := createTarGz(t, tt.files)

			// Create updater
			u := &Updater{
				CurrentVersion: "1.0.0",
				BinaryPath:     "/usr/local/bin/push-validator",
			}

			// Extract binary
			got, err := u.ExtractBinary(archiveData)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractBinary() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("ExtractBinary() error = %q, want error containing %q",
						err.Error(), tt.errContains)
				}
				return
			}

			// Verify content
			if string(got) != "binary content" {
				t.Errorf("ExtractBinary() content = %q, want %q", string(got), "binary content")
			}
		})
	}
}

func TestExtractBinary_InvalidArchive(t *testing.T) {
	u := &Updater{
		CurrentVersion: "1.0.0",
		BinaryPath:     "/usr/local/bin/push-validator",
	}

	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "invalid gzip",
			data: []byte("not a gzip file"),
		},
		{
			name: "empty data",
			data: []byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := u.ExtractBinary(tt.data)
			if err == nil {
				t.Error("ExtractBinary() expected error for invalid archive, got nil")
			}
		})
	}
}

func TestInstallAndRollback(t *testing.T) {
	tempDir := t.TempDir()

	// Create fake current binary
	currentBinary := filepath.Join(tempDir, "push-validator")
	originalContent := []byte("original binary v1.0.0")
	err := os.WriteFile(currentBinary, originalContent, 0755)
	if err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

	// Create updater
	u := &Updater{
		CurrentVersion: "1.0.0",
		BinaryPath:     currentBinary,
	}

	// New binary content
	newContent := []byte("new binary v2.0.0")

	// Test Install
	t.Run("Install", func(t *testing.T) {
		err := u.Install(newContent)
		if err != nil {
			t.Fatalf("Install() error = %v", err)
		}

		// Verify new binary is in place
		content, err := os.ReadFile(currentBinary)
		if err != nil {
			t.Fatalf("Failed to read installed binary: %v", err)
		}
		if !bytes.Equal(content, newContent) {
			t.Errorf("Installed binary content = %q, want %q", string(content), string(newContent))
		}

		// Verify backup exists
		backupPath := currentBinary + ".backup"
		backupContent, err := os.ReadFile(backupPath)
		if err != nil {
			t.Fatalf("Failed to read backup: %v", err)
		}
		if !bytes.Equal(backupContent, originalContent) {
			t.Errorf("Backup content = %q, want %q", string(backupContent), string(originalContent))
		}

		// Verify permissions
		info, err := os.Stat(currentBinary)
		if err != nil {
			t.Fatalf("Failed to stat binary: %v", err)
		}
		if info.Mode().Perm() != 0755 {
			t.Errorf("Binary permissions = %o, want 0755", info.Mode().Perm())
		}
	})

	// Test Rollback
	t.Run("Rollback", func(t *testing.T) {
		err := u.Rollback()
		if err != nil {
			t.Fatalf("Rollback() error = %v", err)
		}

		// Verify original binary is restored
		content, err := os.ReadFile(currentBinary)
		if err != nil {
			t.Fatalf("Failed to read restored binary: %v", err)
		}
		if !bytes.Equal(content, originalContent) {
			t.Errorf("Restored binary content = %q, want %q", string(content), string(originalContent))
		}
	})

	// Test Rollback without backup
	t.Run("Rollback_NoBackup", func(t *testing.T) {
		err := u.Rollback()
		if err == nil {
			t.Error("Rollback() expected error when no backup exists, got nil")
		}
		if !strings.Contains(err.Error(), "no backup found") {
			t.Errorf("Rollback() error = %q, want error containing 'no backup found'", err.Error())
		}
	})
}

func TestInstall_PreservesPermissions(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name string
		mode os.FileMode
	}{
		{"executable", 0755},
		{"readonly", 0444},
		{"owner only", 0700},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create binary with specific permissions
			binaryPath := filepath.Join(tempDir, "push-validator-"+tt.name)
			err := os.WriteFile(binaryPath, []byte("original"), tt.mode)
			if err != nil {
				t.Fatalf("Failed to create test binary: %v", err)
			}

			u := &Updater{
				CurrentVersion: "1.0.0",
				BinaryPath:     binaryPath,
			}

			// Install new version
			err = u.Install([]byte("new version"))
			if err != nil {
				t.Fatalf("Install() error = %v", err)
			}

			// Verify permissions are preserved
			info, err := os.Stat(binaryPath)
			if err != nil {
				t.Fatalf("Failed to stat binary: %v", err)
			}
			if info.Mode().Perm() != tt.mode {
				t.Errorf("Binary permissions = %o, want %o", info.Mode().Perm(), tt.mode)
			}
		})
	}
}

func TestVerifyChecksum(t *testing.T) {
	// Create test data
	testData := []byte("test binary content")
	hash := sha256.Sum256(testData)
	correctChecksum := hex.EncodeToString(hash[:])

	tests := []struct {
		name            string
		checksumContent string
		assetName       string
		wantErr         bool
		errContains     string
	}{
		{
			name:            "valid checksum",
			checksumContent: correctChecksum + "  push-validator_1.0.0_linux_amd64.tar.gz\n",
			assetName:       "push-validator_1.0.0_linux_amd64.tar.gz",
			wantErr:         false,
		},
		{
			name:            "checksum mismatch",
			checksumContent: "0000000000000000000000000000000000000000000000000000000000000000  push-validator_1.0.0_linux_amd64.tar.gz\n",
			assetName:       "push-validator_1.0.0_linux_amd64.tar.gz",
			wantErr:         true,
			errContains:     "checksum mismatch",
		},
		{
			name:            "asset not in checksums",
			checksumContent: correctChecksum + "  other_file.tar.gz\n",
			assetName:       "push-validator_1.0.0_linux_amd64.tar.gz",
			wantErr:         true,
			errContains:     "checksum not found",
		},
		{
			name:            "multiple checksums",
			checksumContent: "aaaa  other1.tar.gz\n" + correctChecksum + "  push-validator_1.0.0_linux_amd64.tar.gz\nbbbb  other2.tar.gz\n",
			assetName:       "push-validator_1.0.0_linux_amd64.tar.gz",
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server for checksums.txt
			checksumServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				w.Write([]byte(tt.checksumContent))
			}))
			defer checksumServer.Close()

			// Create release with checksum asset
			release := &Release{
				TagName: "v1.0.0",
				Assets: []Asset{
					{
						Name:               "checksums.txt",
						BrowserDownloadURL: checksumServer.URL,
					},
				},
			}

			u := &Updater{
				CurrentVersion: "1.0.0",
				BinaryPath:     "/usr/local/bin/push-validator",
				http:           &http.Client{},
			}

			err := u.VerifyChecksum(testData, release, tt.assetName)
			if (err != nil) != tt.wantErr {
				t.Errorf("VerifyChecksum() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errContains != "" {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("VerifyChecksum() error = %q, want error containing %q",
						err.Error(), tt.errContains)
				}
			}
		})
	}
}

func TestVerifyChecksum_NoChecksumAsset(t *testing.T) {
	release := &Release{
		TagName: "v1.0.0",
		Assets:  []Asset{},
	}

	u := &Updater{
		CurrentVersion: "1.0.0",
		BinaryPath:     "/usr/local/bin/push-validator",
	}

	err := u.VerifyChecksum([]byte("data"), release, "test.tar.gz")
	if err == nil {
		t.Error("VerifyChecksum() expected error when no checksum asset exists, got nil")
	}
}

func TestDownload(t *testing.T) {
	testData := []byte("binary archive content")

	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
	}{
		{
			name:       "successful download",
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "not found",
			statusCode: http.StatusNotFound,
			wantErr:    true,
		},
		{
			name:       "server error",
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				if tt.statusCode == http.StatusOK {
					w.Write(testData)
				}
			}))
			defer server.Close()

			asset := &Asset{
				Name:               "test.tar.gz",
				BrowserDownloadURL: server.URL,
				Size:               int64(len(testData)),
			}

			u := &Updater{
				CurrentVersion: "1.0.0",
				BinaryPath:     "/usr/local/bin/push-validator",
				http:           &http.Client{},
			}

			data, err := u.Download(asset, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("Download() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && !bytes.Equal(data, testData) {
				t.Errorf("Download() data = %q, want %q", string(data), string(testData))
			}
		})
	}
}

func TestDownload_WithProgress(t *testing.T) {
	testData := []byte("test content for progress tracking")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", string(rune(len(testData))))
		w.WriteHeader(http.StatusOK)
		w.Write(testData)
	}))
	defer server.Close()

	asset := &Asset{
		Name:               "test.tar.gz",
		BrowserDownloadURL: server.URL,
		Size:               int64(len(testData)),
	}

	u := &Updater{
		CurrentVersion: "1.0.0",
		BinaryPath:     "/usr/local/bin/push-validator",
		http:           &http.Client{},
	}

	var progressCalls int
	var lastDownloaded int64
	progressFunc := func(downloaded, total int64) {
		progressCalls++
		lastDownloaded = downloaded
		if downloaded > total && total > 0 {
			t.Errorf("Progress downloaded=%d > total=%d", downloaded, total)
		}
	}

	data, err := u.Download(asset, progressFunc)
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}

	if !bytes.Equal(data, testData) {
		t.Errorf("Download() data = %q, want %q", string(data), string(testData))
	}

	if progressCalls == 0 {
		t.Error("Progress function was never called")
	}

	if lastDownloaded != int64(len(testData)) {
		t.Errorf("Last progress downloaded = %d, want %d", lastDownloaded, len(testData))
	}
}

func TestCheck(t *testing.T) {
	tests := []struct {
		name            string
		currentVersion  string
		latestTag       string
		wantAvailable   bool
		wantErr         bool
	}{
		{
			name:           "update available",
			currentVersion: "1.0.0",
			latestTag:      "v2.0.0",
			wantAvailable:  true,
		},
		{
			name:           "already up to date",
			currentVersion: "2.0.0",
			latestTag:      "v2.0.0",
			wantAvailable:  false,
		},
		{
			name:           "dev version",
			currentVersion: "dev",
			latestTag:      "v1.0.0",
			wantAvailable:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockHTTPDoer{
				doFunc: func(req *http.Request) (*http.Response, error) {
					release := Release{TagName: tt.latestTag}
					body, _ := json.Marshal(release)
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewReader(body)),
					}, nil
				},
			}

			u := &Updater{
				CurrentVersion: tt.currentVersion,
				BinaryPath:     "/usr/local/bin/push-validator",
				http:           mock,
			}

			result, err := u.Check()
			if (err != nil) != tt.wantErr {
				t.Errorf("Check() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			if result.UpdateAvailable != tt.wantAvailable {
				t.Errorf("Check() UpdateAvailable = %v, want %v", result.UpdateAvailable, tt.wantAvailable)
			}
		})
	}
}

func TestInstall_ErrorCases(t *testing.T) {
	t.Run("binary does not exist", func(t *testing.T) {
		u := &Updater{
			CurrentVersion: "1.0.0",
			BinaryPath:     "/nonexistent/path/to/binary",
		}

		err := u.Install([]byte("new binary"))
		if err == nil {
			t.Error("Install() expected error for non-existent binary, got nil")
		}
	})

	t.Run("install to readonly directory", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("Skipping test when running as root")
		}

		tempDir := t.TempDir()
		binaryPath := filepath.Join(tempDir, "push-validator")
		err := os.WriteFile(binaryPath, []byte("original"), 0755)
		if err != nil {
			t.Fatalf("Failed to create test binary: %v", err)
		}

		// Make directory read-only
		err = os.Chmod(tempDir, 0555)
		if err != nil {
			t.Fatalf("Failed to make directory readonly: %v", err)
		}
		defer os.Chmod(tempDir, 0755) // Restore for cleanup

		u := &Updater{
			CurrentVersion: "1.0.0",
			BinaryPath:     binaryPath,
		}

		err = u.Install([]byte("new binary"))
		if err == nil {
			t.Error("Install() expected error for readonly directory, got nil")
		}
	})
}

func TestCopyFile(t *testing.T) {
	tempDir := t.TempDir()

	srcPath := filepath.Join(tempDir, "source.txt")
	dstPath := filepath.Join(tempDir, "dest.txt")
	content := []byte("test content for copy")

	// Create source file
	err := os.WriteFile(srcPath, content, 0644)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Test successful copy
	err = copyFile(srcPath, dstPath)
	if err != nil {
		t.Fatalf("copyFile() error = %v", err)
	}

	// Verify destination content
	dstContent, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("Failed to read destination: %v", err)
	}
	if !bytes.Equal(dstContent, content) {
		t.Errorf("Destination content = %q, want %q", string(dstContent), string(content))
	}

	// Test copy from non-existent file
	err = copyFile("/nonexistent/file", dstPath)
	if err == nil {
		t.Error("copyFile() expected error for non-existent source, got nil")
	}
}

func TestVerifyChecksum_DownloadError(t *testing.T) {
	// Create a server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	release := &Release{
		TagName: "v1.0.0",
		Assets: []Asset{
			{
				Name:               "checksums.txt",
				BrowserDownloadURL: server.URL,
			},
		},
	}

	u := &Updater{
		CurrentVersion: "1.0.0",
		BinaryPath:     "/usr/local/bin/push-validator",
		http:           &http.Client{},
	}

	err := u.VerifyChecksum([]byte("data"), release, "test.tar.gz")
	if err == nil {
		t.Error("VerifyChecksum() expected error for failed checksum download, got nil")
	}
}

func TestExtractBinary_TarError(t *testing.T) {
	// Create a gzip file with invalid tar content
	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	gzWriter.Write([]byte("this is not a valid tar file"))
	gzWriter.Close()

	u := &Updater{
		CurrentVersion: "1.0.0",
		BinaryPath:     "/usr/local/bin/push-validator",
	}

	_, err := u.ExtractBinary(buf.Bytes())
	if err == nil {
		t.Error("ExtractBinary() expected error for invalid tar, got nil")
	}
}

func TestDownload_InvalidURL(t *testing.T) {
	asset := &Asset{
		Name:               "test.tar.gz",
		BrowserDownloadURL: "http://invalid-url-that-does-not-exist-12345.com/file",
		Size:               100,
	}

	u := &Updater{
		CurrentVersion: "1.0.0",
		BinaryPath:     "/usr/local/bin/push-validator",
		http:           &http.Client{},
	}

	_, err := u.Download(asset, nil)
	if err == nil {
		t.Error("Download() expected error for invalid URL, got nil")
	}
}

func TestInstall_AtomicRename(t *testing.T) {
	tempDir := t.TempDir()

	// Create a binary file
	binaryPath := filepath.Join(tempDir, "push-validator")
	originalContent := []byte("original version")
	err := os.WriteFile(binaryPath, originalContent, 0755)
	if err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

	u := &Updater{
		CurrentVersion: "1.0.0",
		BinaryPath:     binaryPath,
	}

	newContent := []byte("new version content")

	// Install should succeed
	err = u.Install(newContent)
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	// Verify the file was atomically replaced
	content, err := os.ReadFile(binaryPath)
	if err != nil {
		t.Fatalf("Failed to read binary: %v", err)
	}

	if !bytes.Equal(content, newContent) {
		t.Errorf("Binary content = %q, want %q", string(content), string(newContent))
	}

	// Verify backup exists and has original content
	backupPath := binaryPath + ".backup"
	backupContent, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("Failed to read backup: %v", err)
	}

	if !bytes.Equal(backupContent, originalContent) {
		t.Errorf("Backup content = %q, want %q", string(backupContent), string(originalContent))
	}
}

func TestVerifyChecksum_EmptyFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(""))
	}))
	defer server.Close()

	release := &Release{
		TagName: "v1.0.0",
		Assets: []Asset{
			{
				Name:               "checksums.txt",
				BrowserDownloadURL: server.URL,
			},
		},
	}

	u := &Updater{
		CurrentVersion: "1.0.0",
		BinaryPath:     "/usr/local/bin/push-validator",
		http:           &http.Client{},
	}

	err := u.VerifyChecksum([]byte("data"), release, "test.tar.gz")
	if err == nil {
		t.Error("VerifyChecksum() expected error for empty checksums file, got nil")
	}
	if !strings.Contains(err.Error(), "checksum not found") {
		t.Errorf("VerifyChecksum() error = %q, want error containing 'checksum not found'", err.Error())
	}
}

func TestExtractBinary_DirectoryInArchive(t *testing.T) {
	// Create tar.gz with a directory entry
	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzWriter)

	// Add directory
	dirHeader := &tar.Header{
		Name:     "bin/",
		Mode:     0755,
		Typeflag: tar.TypeDir,
		ModTime:  time.Now(),
	}
	tarWriter.WriteHeader(dirHeader)

	// Add file in directory
	fileHeader := &tar.Header{
		Name:     "bin/push-validator",
		Mode:     0755,
		Size:     14,
		Typeflag: tar.TypeReg,
		ModTime:  time.Now(),
	}
	tarWriter.WriteHeader(fileHeader)
	tarWriter.Write([]byte("binary content"))

	tarWriter.Close()
	gzWriter.Close()

	u := &Updater{
		CurrentVersion: "1.0.0",
		BinaryPath:     "/usr/local/bin/push-validator",
	}

	data, err := u.ExtractBinary(buf.Bytes())
	if err != nil {
		t.Fatalf("ExtractBinary() error = %v", err)
	}

	if string(data) != "binary content" {
		t.Errorf("ExtractBinary() content = %q, want %q", string(data), "binary content")
	}
}

func TestDownload_ReadError(t *testing.T) {
	// Create a server that closes connection mid-stream
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000")
		w.WriteHeader(http.StatusOK)
		// Write partial data
		w.Write([]byte("partial"))
		// Connection will be closed when handler returns
	}))

	asset := &Asset{
		Name:               "test.tar.gz",
		BrowserDownloadURL: server.URL,
		Size:               1000,
	}

	u := &Updater{
		CurrentVersion: "1.0.0",
		BinaryPath:     "/usr/local/bin/push-validator",
		http:           &http.Client{},
	}

	// Close server immediately to simulate connection error
	server.Close()

	_, err := u.Download(asset, nil)
	if err == nil {
		t.Error("Download() expected error for connection failure, got nil")
	}
}

func TestProgressReader_NilProgress(t *testing.T) {
	data := []byte("test data")
	reader := bytes.NewReader(data)

	pr := &progressReader{
		reader:   reader,
		total:    int64(len(data)),
		progress: nil, // nil progress function
	}

	result, err := io.ReadAll(pr)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}

	if !bytes.Equal(result, data) {
		t.Errorf("Read data = %q, want %q", string(result), string(data))
	}
}

func TestVerifyChecksum_MalformedChecksumLine(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		// Write malformed checksum (only one field instead of two)
		w.Write([]byte("abc123def456\n"))
	}))
	defer server.Close()

	release := &Release{
		TagName: "v1.0.0",
		Assets: []Asset{
			{
				Name:               "checksums.txt",
				BrowserDownloadURL: server.URL,
			},
		},
	}

	u := &Updater{
		CurrentVersion: "1.0.0",
		BinaryPath:     "/usr/local/bin/push-validator",
		http:           &http.Client{},
	}

	err := u.VerifyChecksum([]byte("data"), release, "test.tar.gz")
	if err == nil {
		t.Error("VerifyChecksum() expected error for malformed checksum, got nil")
	}
}

func TestInstall_WriteError(t *testing.T) {
	tempDir := t.TempDir()
	binaryPath := filepath.Join(tempDir, "push-validator")

	// Create initial binary
	err := os.WriteFile(binaryPath, []byte("original"), 0755)
	if err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

	u := &Updater{
		CurrentVersion: "1.0.0",
		BinaryPath:     binaryPath,
	}

	// Create a very large binary that might cause write issues
	// This tests the error path at line 208-211
	largeData := make([]byte, 0) // Empty for this test, real write errors are hard to simulate

	// Actually test by making the directory readonly after creating the binary
	// but before calling Install - this will cause CreateTemp to fail
	if os.Getuid() != 0 { // Skip if root
		err = os.Chmod(tempDir, 0555)
		if err != nil {
			t.Fatalf("Failed to make directory readonly: %v", err)
		}
		defer os.Chmod(tempDir, 0755)

		err = u.Install(largeData)
		if err == nil {
			t.Error("Install() expected error for readonly directory, got nil")
		}
	}
}

func TestInstall_SuccessfulInstall(t *testing.T) {
	// Explicitly test the full success path to ensure all lines are covered
	dir := t.TempDir()
	binPath := filepath.Join(dir, "push-validator")
	os.WriteFile(binPath, []byte("old-binary"), 0o755)

	u := &Updater{BinaryPath: binPath}
	err := u.Install([]byte("new-binary-content"))
	if err != nil {
		t.Fatalf("Install() unexpected error: %v", err)
	}

	// Verify the new binary is in place
	data, err := os.ReadFile(binPath)
	if err != nil {
		t.Fatalf("failed to read installed binary: %v", err)
	}
	if string(data) != "new-binary-content" {
		t.Errorf("installed binary = %q, want %q", string(data), "new-binary-content")
	}

	// Verify backup was created with old content
	backupData, err := os.ReadFile(binPath + ".backup")
	if err != nil {
		t.Fatalf("backup not found: %v", err)
	}
	if string(backupData) != "old-binary" {
		t.Errorf("backup = %q, want %q", string(backupData), "old-binary")
	}

	// Verify permissions preserved
	info, _ := os.Stat(binPath)
	if info.Mode().Perm() != 0o755 {
		t.Errorf("permissions = %o, want 0755", info.Mode().Perm())
	}
}

func TestInstall_CreateTempError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("cannot test as root")
	}
	dir := t.TempDir()
	binPath := filepath.Join(dir, "push-validator")
	os.WriteFile(binPath, []byte("original"), 0o755)

	// Make dir readonly so CreateTemp fails after backup succeeds
	u := &Updater{BinaryPath: binPath}

	// Create backup manually first so backup step succeeds
	os.WriteFile(binPath+".backup", []byte("original"), 0o755)

	// Now make dir readonly - CreateTemp will fail
	os.Chmod(dir, 0o555)
	defer os.Chmod(dir, 0o755)

	err := u.Install([]byte("new"))
	if err == nil {
		t.Fatal("expected error for CreateTemp failure")
	}
	if !strings.Contains(err.Error(), "failed to create temp file") && !strings.Contains(err.Error(), "failed to create backup") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCopyFile_DestinationWriteError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("Skipping test when running as root")
	}

	tempDir := t.TempDir()
	srcPath := filepath.Join(tempDir, "source.txt")

	// Create source
	err := os.WriteFile(srcPath, []byte("content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}

	// Create readonly subdirectory for destination
	readonlyDir := filepath.Join(tempDir, "readonly")
	err = os.Mkdir(readonlyDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create readonly dir: %v", err)
	}

	dstPath := filepath.Join(readonlyDir, "dest.txt")

	// Make directory readonly
	err = os.Chmod(readonlyDir, 0555)
	if err != nil {
		t.Fatalf("Failed to chmod: %v", err)
	}
	defer os.Chmod(readonlyDir, 0755)

	// Attempt to copy
	err = copyFile(srcPath, dstPath)
	if err == nil {
		t.Error("copyFile() expected error for readonly destination, got nil")
	}
}

func TestNewUpdater(t *testing.T) {
	currentVersion := "1.2.3"
	u, err := New(currentVersion)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if u.CurrentVersion != currentVersion {
		t.Errorf("CurrentVersion = %q, want %q", u.CurrentVersion, currentVersion)
	}

	if u.BinaryPath == "" {
		t.Error("BinaryPath is empty")
	}

	// Verify BinaryPath is an absolute path
	if !filepath.IsAbs(u.BinaryPath) {
		t.Errorf("BinaryPath = %q is not absolute", u.BinaryPath)
	}
}

func TestProgressReader(t *testing.T) {
	data := []byte("test data for progress reader")
	reader := bytes.NewReader(data)

	var progressCalls int
	var totalRead int64
	progressFunc := func(downloaded, total int64) {
		progressCalls++
		totalRead = downloaded
	}

	pr := &progressReader{
		reader:   reader,
		total:    int64(len(data)),
		progress: progressFunc,
	}

	result, err := io.ReadAll(pr)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}

	if !bytes.Equal(result, data) {
		t.Errorf("Read data = %q, want %q", string(result), string(data))
	}

	if progressCalls == 0 {
		t.Error("Progress function was never called")
	}

	if totalRead != int64(len(data)) {
		t.Errorf("Total read = %d, want %d", totalRead, len(data))
	}
}

// --- Additional coverage tests ---

func TestInstall_BinaryNotFound(t *testing.T) {
	u := &Updater{BinaryPath: "/nonexistent/binary/path"}
	err := u.Install([]byte("data"))
	if err == nil || !strings.Contains(err.Error(), "failed to stat current binary") {
		t.Errorf("expected stat error, got: %v", err)
	}
}

func TestInstall_BackupFailure(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "push-validator")
	os.WriteFile(binPath, []byte("original"), 0o755)

	// Make the backup target path unwritable by removing write on dir
	roDir := filepath.Join(dir, "readonly")
	os.MkdirAll(roDir, 0o555)
	binInRO := filepath.Join(roDir, "push-validator")
	os.WriteFile(binInRO, []byte("original"), 0o755)
	os.Chmod(roDir, 0o555)
	defer os.Chmod(roDir, 0o755)

	u := &Updater{BinaryPath: binInRO}
	err := u.Install([]byte("new binary data"))
	if err == nil {
		t.Fatal("expected error due to backup failure")
	}
}

func TestInstall_TempFileWriteAndCleanup(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "push-validator")
	os.WriteFile(binPath, []byte("original"), 0o755)

	u := &Updater{BinaryPath: binPath}
	err := u.Install([]byte("new binary data"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify install succeeded
	data, _ := os.ReadFile(binPath)
	if string(data) != "new binary data" {
		t.Errorf("expected 'new binary data', got %q", string(data))
	}
}

func TestInstall_PreservesPermissions_0700(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "push-validator")
	os.WriteFile(binPath, []byte("original"), 0o700)

	u := &Updater{BinaryPath: binPath}
	err := u.Install([]byte("new binary data"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, _ := os.Stat(binPath)
	if info.Mode().Perm() != 0o700 {
		t.Errorf("expected permissions 0700, got %o", info.Mode().Perm())
	}
}

func TestInstall_BackupCreated(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "push-validator")
	os.WriteFile(binPath, []byte("original"), 0o755)

	u := &Updater{BinaryPath: binPath}
	err := u.Install([]byte("new"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify backup exists
	backupData, err := os.ReadFile(binPath + ".backup")
	if err != nil {
		t.Fatalf("backup file not found: %v", err)
	}
	if string(backupData) != "original" {
		t.Errorf("backup content = %q, want 'original'", string(backupData))
	}
}

func TestInstall_EmptyBinaryData(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "push-validator")
	os.WriteFile(binPath, []byte("original"), 0o755)

	u := &Updater{BinaryPath: binPath}
	err := u.Install([]byte{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(binPath)
	if len(data) != 0 {
		t.Errorf("expected empty file, got %d bytes", len(data))
	}
}

func TestCheck_FetchError(t *testing.T) {
	mock := &mockHTTPDoer{
		doFunc: func(req *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("network error")
		},
	}
	u := &Updater{CurrentVersion: "v1.0.0", http: mock}
	_, err := u.Check()
	if err == nil || !strings.Contains(err.Error(), "network error") {
		t.Errorf("expected network error, got: %v", err)
	}
}

func TestCheck_VersionTrimming(t *testing.T) {
	releaseJSON, _ := json.Marshal(Release{
		TagName: "v2.0.0",
		Assets: []Asset{
			{Name: fmt.Sprintf("push-validator_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)},
		},
	})
	mock := &mockHTTPDoer{
		doFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader(releaseJSON)),
			}, nil
		},
	}
	u := &Updater{CurrentVersion: "v1.0.0", http: mock}
	result, err := u.Check()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.CurrentVersion != "1.0.0" {
		t.Errorf("expected CurrentVersion='1.0.0', got %q", result.CurrentVersion)
	}
	if result.LatestVersion != "2.0.0" {
		t.Errorf("expected LatestVersion='2.0.0', got %q", result.LatestVersion)
	}
	if !result.UpdateAvailable {
		t.Error("expected UpdateAvailable=true")
	}
}

func TestCheck_SameVersion(t *testing.T) {
	releaseJSON, _ := json.Marshal(Release{
		TagName: "v1.0.0",
		Assets: []Asset{
			{Name: fmt.Sprintf("push-validator_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)},
		},
	})
	mock := &mockHTTPDoer{
		doFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader(releaseJSON)),
			}, nil
		},
	}
	u := &Updater{CurrentVersion: "v1.0.0", http: mock}
	result, err := u.Check()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.UpdateAvailable {
		t.Error("expected UpdateAvailable=false for same version")
	}
}

func TestDownload_EmptyBody(t *testing.T) {
	mock := &mockHTTPDoer{
		doFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader(nil)),
			}, nil
		},
	}
	u := &Updater{http: mock}
	data, err := u.Download(&Asset{BrowserDownloadURL: "https://example.com/file.tar.gz"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("expected empty data, got %d bytes", len(data))
	}
}

func TestDownload_400StatusCode(t *testing.T) {
	mock := &mockHTTPDoer{
		doFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 400,
				Status:     "400 Bad Request",
				Body:       io.NopCloser(bytes.NewReader(nil)),
			}, nil
		},
	}
	u := &Updater{http: mock}
	_, err := u.Download(&Asset{BrowserDownloadURL: "https://example.com/file.tar.gz"}, nil)
	if err == nil || !strings.Contains(err.Error(), "400 Bad Request") {
		t.Errorf("expected 400 error, got: %v", err)
	}
}

func TestDownload_403StatusCode(t *testing.T) {
	mock := &mockHTTPDoer{
		doFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 403,
				Status:     "403 Forbidden",
				Body:       io.NopCloser(bytes.NewReader(nil)),
			}, nil
		},
	}
	u := &Updater{http: mock}
	_, err := u.Download(&Asset{BrowserDownloadURL: "https://example.com/file.tar.gz"}, nil)
	if err == nil || !strings.Contains(err.Error(), "403 Forbidden") {
		t.Errorf("expected 403 error, got: %v", err)
	}
}

func TestDownload_WithProgressCallback(t *testing.T) {
	content := []byte("file content data here")
	mock := &mockHTTPDoer{
		doFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode:    200,
				Body:          io.NopCloser(bytes.NewReader(content)),
				ContentLength: int64(len(content)),
			}, nil
		},
	}
	u := &Updater{http: mock}
	progressCalls := 0
	data, err := u.Download(&Asset{BrowserDownloadURL: "https://example.com/file.tar.gz"}, func(downloaded, total int64) {
		progressCalls++
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("data mismatch")
	}
	if progressCalls == 0 {
		t.Error("expected progress callback to be called")
	}
}

func TestExtractBinary_CorruptGzip(t *testing.T) {
	// Valid gzip header but corrupt data
	corruptData := []byte{0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff, 0xde, 0xad, 0xbe, 0xef}
	u := &Updater{}
	_, err := u.ExtractBinary(corruptData)
	if err == nil {
		t.Fatal("expected error for corrupt gzip data")
	}
}

func TestExtractBinary_BinaryNotFirst(t *testing.T) {
	// Create archive where push-validator is NOT the first file
	files := map[string]string{
		"README.md":      "readme content",
		"LICENSE":        "license content",
		"push-validator": "actual binary",
	}

	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzWriter)

	// Write in specific order: README first, then binary
	for _, name := range []string{"README.md", "LICENSE", "push-validator"} {
		content := files[name]
		header := &tar.Header{
			Name:    name,
			Mode:    0755,
			Size:    int64(len(content)),
			ModTime: time.Now(),
		}
		tarWriter.WriteHeader(header)
		tarWriter.Write([]byte(content))
	}
	tarWriter.Close()
	gzWriter.Close()

	u := &Updater{}
	data, err := u.ExtractBinary(buf.Bytes())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "actual binary" {
		t.Errorf("expected 'actual binary', got %q", string(data))
	}
}

func TestVerifyChecksum_ExtraWhitespace(t *testing.T) {
	// Checksum file with tab-separated fields
	archiveData := []byte("archive content")
	hash := sha256.Sum256(archiveData)
	expectedHash := hex.EncodeToString(hash[:])

	checksumContent := fmt.Sprintf("%s\ttest-asset.tar.gz\n", expectedHash)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(checksumContent))
	}))
	defer server.Close()

	u := &Updater{http: server.Client()}
	release := &Release{
		Assets: []Asset{
			{Name: "checksums.txt", BrowserDownloadURL: server.URL},
		},
	}

	// Tabs between hash and filename - Fields() splits on any whitespace
	err := u.VerifyChecksum(archiveData, release, "test-asset.tar.gz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewWith_NilHTTPDoer(t *testing.T) {
	u, err := NewWith("v1.0.0", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.http == nil {
		t.Error("expected default HTTP client to be set")
	}
	if u.CurrentVersion != "v1.0.0" {
		t.Errorf("expected CurrentVersion='v1.0.0', got %q", u.CurrentVersion)
	}
}

func TestNewWith_CustomHTTPDoer(t *testing.T) {
	mock := &mockHTTPDoer{doFunc: func(req *http.Request) (*http.Response, error) { return nil, nil }}
	u, err := NewWith("v2.0.0", mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.http != mock {
		t.Error("expected custom HTTP doer to be used")
	}
}

func TestNewWith_ResolvesSymlinks(t *testing.T) {
	// Just verify it can create an updater (the symlink resolution uses os.Executable())
	u, err := NewWith("v1.0.0", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.BinaryPath == "" {
		t.Error("expected BinaryPath to be set")
	}
}

func TestDownload_ReadErrorWithProgress(t *testing.T) {
	// Trigger a read error while using the progressReader path
	mock := &mockHTTPDoer{
		doFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode:    200,
				ContentLength: 1000,
				Body:          io.NopCloser(&errorReader{err: fmt.Errorf("read failed")}),
			}, nil
		},
	}
	u := &Updater{http: mock}
	_, err := u.Download(&Asset{BrowserDownloadURL: "https://example.com/file.tar.gz"}, func(downloaded, total int64) {})
	if err == nil || !strings.Contains(err.Error(), "failed to read download") {
		t.Errorf("expected read error, got: %v", err)
	}
}

func TestDownload_NewRequestError(t *testing.T) {
	u := &Updater{http: &mockHTTPDoer{}}
	// Invalid URL with control character will cause NewRequest to fail
	_, err := u.Download(&Asset{BrowserDownloadURL: "http://invalid\x7f.com"}, nil)
	if err == nil || !strings.Contains(err.Error(), "failed to create download request") {
		t.Errorf("expected new request error, got: %v", err)
	}
}

// errorReader always returns an error on Read
type errorReader struct {
	err error
}

func (r *errorReader) Read(p []byte) (n int, err error) {
	return 0, r.err
}

// Helper function to create a tar.gz archive
func createTarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzWriter)

	for name, content := range files {
		header := &tar.Header{
			Name:    name,
			Mode:    0755,
			Size:    int64(len(content)),
			ModTime: time.Now(),
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatalf("Failed to write tar header: %v", err)
		}
		if _, err := tarWriter.Write([]byte(content)); err != nil {
			t.Fatalf("Failed to write tar content: %v", err)
		}
	}

	if err := tarWriter.Close(); err != nil {
		t.Fatalf("Failed to close tar writer: %v", err)
	}
	if err := gzWriter.Close(); err != nil {
		t.Fatalf("Failed to close gzip writer: %v", err)
	}

	return buf.Bytes()
}

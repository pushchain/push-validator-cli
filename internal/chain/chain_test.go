package chain

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNewInstaller(t *testing.T) {
	homeDir := "/test/home"
	installer := NewInstaller(homeDir)

	if installer.HomeDir != homeDir {
		t.Errorf("Expected HomeDir to be %s, got %s", homeDir, installer.HomeDir)
	}
}

func TestGetAssetForPlatform(t *testing.T) {
	tests := []struct {
		name        string
		release     *Release
		expectError bool
		expectName  string
	}{
		{
			name: "matching asset found",
			release: &Release{
				TagName: "v1.0.0",
				Assets: []Asset{
					{
						Name:               fmt.Sprintf("push-chain_1.0.0_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH),
						BrowserDownloadURL: "https://example.com/download",
					},
				},
			},
			expectError: false,
			expectName:  fmt.Sprintf("push-chain_1.0.0_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH),
		},
		{
			name: "no matching asset",
			release: &Release{
				TagName: "v1.0.0",
				Assets: []Asset{
					{
						Name:               "push-chain_1.0.0_nonexistent_platform.tar.gz",
						BrowserDownloadURL: "https://example.com/download",
					},
				},
			},
			expectError: true,
		},
		{
			name: "multiple assets with correct one",
			release: &Release{
				TagName: "v1.0.0",
				Assets: []Asset{
					{
						Name:               "push-chain_1.0.0_windows_amd64.tar.gz",
						BrowserDownloadURL: "https://example.com/download1",
					},
					{
						Name:               fmt.Sprintf("push-chain_1.0.0_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH),
						BrowserDownloadURL: "https://example.com/download2",
					},
					{
						Name:               "push-chain_1.0.0_linux_arm64.tar.gz",
						BrowserDownloadURL: "https://example.com/download3",
					},
				},
			},
			expectError: false,
			expectName:  fmt.Sprintf("push-chain_1.0.0_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH),
		},
		{
			name: "empty assets",
			release: &Release{
				TagName: "v1.0.0",
				Assets:  []Asset{},
			},
			expectError: true,
		},
		{
			name: "wrong prefix",
			release: &Release{
				TagName: "v1.0.0",
				Assets: []Asset{
					{
						Name:               fmt.Sprintf("wrong-prefix_1.0.0_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH),
						BrowserDownloadURL: "https://example.com/download",
					},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asset, err := GetAssetForPlatform(tt.release)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got nil")
				}
				if asset != nil {
					t.Error("Expected nil asset on error")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if asset == nil {
					t.Fatal("Expected non-nil asset")
				}
				if asset.Name != tt.expectName {
					t.Errorf("Expected asset name %s, got %s", tt.expectName, asset.Name)
				}
			}
		})
	}
}

func TestGetChecksumAsset(t *testing.T) {
	tests := []struct {
		name        string
		release     *Release
		assetName   string
		expectError bool
		expectName  string
	}{
		{
			name: "checksum asset found",
			release: &Release{
				Assets: []Asset{
					{
						Name:               "binary.tar.gz",
						BrowserDownloadURL: "https://example.com/binary.tar.gz",
					},
					{
						Name:               "binary.tar.gz.sha256",
						BrowserDownloadURL: "https://example.com/binary.tar.gz.sha256",
					},
				},
			},
			assetName:   "binary.tar.gz",
			expectError: false,
			expectName:  "binary.tar.gz.sha256",
		},
		{
			name: "checksum asset not found",
			release: &Release{
				Assets: []Asset{
					{
						Name:               "binary.tar.gz",
						BrowserDownloadURL: "https://example.com/binary.tar.gz",
					},
				},
			},
			assetName:   "binary.tar.gz",
			expectError: true,
		},
		{
			name: "multiple assets with correct checksum",
			release: &Release{
				Assets: []Asset{
					{
						Name:               "binary1.tar.gz.sha256",
						BrowserDownloadURL: "https://example.com/binary1.tar.gz.sha256",
					},
					{
						Name:               "binary2.tar.gz.sha256",
						BrowserDownloadURL: "https://example.com/binary2.tar.gz.sha256",
					},
				},
			},
			assetName:   "binary2.tar.gz",
			expectError: false,
			expectName:  "binary2.tar.gz.sha256",
		},
		{
			name: "empty assets",
			release: &Release{
				Assets: []Asset{},
			},
			assetName:   "binary.tar.gz",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asset, err := GetChecksumAsset(tt.release, tt.assetName)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got nil")
				}
				if asset != nil {
					t.Error("Expected nil asset on error")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if asset == nil {
					t.Fatal("Expected non-nil asset")
				}
				if asset.Name != tt.expectName {
					t.Errorf("Expected asset name %s, got %s", tt.expectName, asset.Name)
				}
			}
		})
	}
}

func TestDownload(t *testing.T) {
	testData := []byte("test binary content")

	tests := []struct {
		name            string
		serverResponse  []byte
		serverStatus    int
		expectError     bool
		expectProgress  bool
		verifyData      bool
	}{
		{
			name:           "successful download",
			serverResponse: testData,
			serverStatus:   http.StatusOK,
			expectError:    false,
			expectProgress: false,
			verifyData:     true,
		},
		{
			name:           "successful download with progress",
			serverResponse: testData,
			serverStatus:   http.StatusOK,
			expectError:    false,
			expectProgress: true,
			verifyData:     true,
		},
		{
			name:           "server error",
			serverResponse: nil,
			serverStatus:   http.StatusInternalServerError,
			expectError:    true,
			expectProgress: false,
			verifyData:     false,
		},
		{
			name:           "not found",
			serverResponse: nil,
			serverStatus:   http.StatusNotFound,
			expectError:    true,
			expectProgress: false,
			verifyData:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.serverStatus)
				if tt.serverResponse != nil {
					w.Write(tt.serverResponse)
				}
			}))
			defer server.Close()

			installer := NewInstaller(t.TempDir())
			asset := &Asset{
				Name:               "test.tar.gz",
				BrowserDownloadURL: server.URL,
				Size:               int64(len(testData)),
			}

			var progressCalled bool
			var lastDownloaded int64
			var progress ProgressFunc
			if tt.expectProgress {
				progress = func(downloaded, total int64) {
					progressCalled = true
					lastDownloaded = downloaded
				}
			}

			data, err := installer.Download(asset, progress)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if tt.verifyData {
					if !bytes.Equal(data, testData) {
						t.Errorf("Downloaded data mismatch: expected %v, got %v", testData, data)
					}
				}
				if tt.expectProgress {
					if !progressCalled {
						t.Error("Progress callback was not called")
					}
					if lastDownloaded != int64(len(testData)) {
						t.Errorf("Expected downloaded bytes %d, got %d", len(testData), lastDownloaded)
					}
				}
			}
		})
	}
}

func TestVerifyChecksum(t *testing.T) {
	testData := []byte("test binary content")
	hash := sha256.Sum256(testData)
	correctChecksum := hex.EncodeToString(hash[:])
	incorrectChecksum := "0000000000000000000000000000000000000000000000000000000000000000"

	tests := []struct {
		name               string
		data               []byte
		checksumContent    string
		checksumStatus     int
		hasChecksumAsset   bool
		expectVerified     bool
		expectError        bool
	}{
		{
			name:             "checksum matches",
			data:             testData,
			checksumContent:  correctChecksum,
			checksumStatus:   http.StatusOK,
			hasChecksumAsset: true,
			expectVerified:   true,
			expectError:      false,
		},
		{
			name:             "checksum matches with filename",
			data:             testData,
			checksumContent:  fmt.Sprintf("%s  binary.tar.gz", correctChecksum),
			checksumStatus:   http.StatusOK,
			hasChecksumAsset: true,
			expectVerified:   true,
			expectError:      false,
		},
		{
			name:             "checksum mismatch",
			data:             testData,
			checksumContent:  incorrectChecksum,
			checksumStatus:   http.StatusOK,
			hasChecksumAsset: true,
			expectVerified:   false,
			expectError:      true,
		},
		{
			name:             "checksum asset not in release",
			data:             testData,
			checksumContent:  "",
			checksumStatus:   http.StatusOK,
			hasChecksumAsset: false,
			expectVerified:   false,
			expectError:      false,
		},
		{
			name:             "checksum file not found (404)",
			data:             testData,
			checksumContent:  "",
			checksumStatus:   http.StatusNotFound,
			hasChecksumAsset: true,
			expectVerified:   false,
			expectError:      false,
		},
		{
			name:             "empty checksum file",
			data:             testData,
			checksumContent:  "",
			checksumStatus:   http.StatusOK,
			hasChecksumAsset: true,
			expectVerified:   false,
			expectError:      true,
		},
		{
			name:             "checksum with whitespace and newlines",
			data:             testData,
			checksumContent:  fmt.Sprintf("\n%s  binary.tar.gz\n", correctChecksum),
			checksumStatus:   http.StatusOK,
			hasChecksumAsset: true,
			expectVerified:   true,
			expectError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server for checksum file
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.checksumStatus)
				if tt.checksumContent != "" {
					w.Write([]byte(tt.checksumContent))
				}
			}))
			defer server.Close()

			installer := NewInstaller(t.TempDir())

			// Create release with or without checksum asset
			assetName := "binary.tar.gz"
			release := &Release{
				Assets: []Asset{
					{
						Name:               assetName,
						BrowserDownloadURL: "https://example.com/binary.tar.gz",
					},
				},
			}

			if tt.hasChecksumAsset {
				release.Assets = append(release.Assets, Asset{
					Name:               assetName + ".sha256",
					BrowserDownloadURL: server.URL,
				})
			}

			verified, err := installer.VerifyChecksum(tt.data, release, assetName)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}

			if verified != tt.expectVerified {
				t.Errorf("Expected verified=%v, got verified=%v", tt.expectVerified, verified)
			}
		})
	}
}

func TestExtractAndInstall(t *testing.T) {
	tests := []struct {
		name        string
		createArchive func() []byte
		expectError bool
		expectBinary bool
		expectLib   bool
	}{
		{
			name: "valid archive with pchaind",
			createArchive: func() []byte {
				return createTarGz(t, map[string][]byte{
					"pchaind": []byte("fake pchaind binary"),
				})
			},
			expectError:  false,
			expectBinary: true,
			expectLib:    false,
		},
		{
			name: "valid archive with pchaind and libwasmvm",
			createArchive: func() []byte {
				return createTarGz(t, map[string][]byte{
					"pchaind":        []byte("fake pchaind binary"),
					"libwasmvm.dylib": []byte("fake wasm library"),
				})
			},
			expectError:  false,
			expectBinary: true,
			expectLib:    true,
		},
		{
			name: "archive with nested path",
			createArchive: func() []byte {
				return createTarGz(t, map[string][]byte{
					"some/nested/path/pchaind": []byte("fake pchaind binary"),
				})
			},
			expectError:  false,
			expectBinary: true,
			expectLib:    false,
		},
		{
			name: "archive without pchaind",
			createArchive: func() []byte {
				return createTarGz(t, map[string][]byte{
					"otherfile": []byte("some content"),
				})
			},
			expectError:  true,
			expectBinary: false,
			expectLib:    false,
		},
		{
			name: "invalid gzip data",
			createArchive: func() []byte {
				return []byte("not a gzip file")
			},
			expectError:  true,
			expectBinary: false,
			expectLib:    false,
		},
		{
			name: "empty archive",
			createArchive: func() []byte {
				return createTarGz(t, map[string][]byte{})
			},
			expectError:  true,
			expectBinary: false,
			expectLib:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			homeDir := t.TempDir()
			installer := NewInstaller(homeDir)

			archiveData := tt.createArchive()
			path, err := installer.ExtractAndInstall(archiveData)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got nil")
				}
				if path != "" {
					t.Errorf("Expected empty path on error, got %s", path)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				// Verify directory structure
				cosmovisorBin := filepath.Join(homeDir, "cosmovisor", "genesis", "bin")
				upgradesDir := filepath.Join(homeDir, "cosmovisor", "upgrades")

				if _, err := os.Stat(cosmovisorBin); os.IsNotExist(err) {
					t.Errorf("Cosmovisor bin directory not created: %s", cosmovisorBin)
				}
				if _, err := os.Stat(upgradesDir); os.IsNotExist(err) {
					t.Errorf("Upgrades directory not created: %s", upgradesDir)
				}

				if tt.expectBinary {
					pchaindPath := filepath.Join(cosmovisorBin, "pchaind")
					if path != pchaindPath {
						t.Errorf("Expected path %s, got %s", pchaindPath, path)
					}

					// Verify file exists and is executable
					info, err := os.Stat(pchaindPath)
					if os.IsNotExist(err) {
						t.Errorf("Binary not created at %s", pchaindPath)
					} else if err != nil {
						t.Errorf("Error checking binary: %v", err)
					} else {
						mode := info.Mode()
						if mode&0o111 == 0 {
							t.Error("Binary is not executable")
						}
					}

					// Verify content
					content, err := os.ReadFile(pchaindPath)
					if err != nil {
						t.Errorf("Error reading binary: %v", err)
					} else if !strings.Contains(string(content), "fake pchaind binary") {
						t.Error("Binary content mismatch")
					}
				}

				if tt.expectLib {
					libPath := filepath.Join(cosmovisorBin, "libwasmvm.dylib")
					info, err := os.Stat(libPath)
					if os.IsNotExist(err) {
						t.Errorf("Library not created at %s", libPath)
					} else if err != nil {
						t.Errorf("Error checking library: %v", err)
					} else {
						mode := info.Mode()
						// Library should be readable but not necessarily executable
						if mode&0o400 == 0 {
							t.Error("Library is not readable")
						}
					}
				}
			}
		})
	}
}

func TestGetInstalledVersion(t *testing.T) {
	tests := []struct {
		name           string
		setupBinary    bool
		expectVersion  string
	}{
		{
			name:          "binary exists",
			setupBinary:   true,
			expectVersion: "installed",
		},
		{
			name:          "binary does not exist",
			setupBinary:   false,
			expectVersion: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			homeDir := t.TempDir()
			installer := NewInstaller(homeDir)

			if tt.setupBinary {
				// Create the binary file
				binPath := filepath.Join(homeDir, "cosmovisor", "genesis", "bin", "pchaind")
				err := os.MkdirAll(filepath.Dir(binPath), 0o755)
				if err != nil {
					t.Fatalf("Failed to create directory: %v", err)
				}
				err = os.WriteFile(binPath, []byte("fake binary"), 0o755)
				if err != nil {
					t.Fatalf("Failed to create binary: %v", err)
				}
			}

			version := installer.GetInstalledVersion()

			if version != tt.expectVersion {
				t.Errorf("Expected version %s, got %s", tt.expectVersion, version)
			}
		})
	}
}

// Helper function to create a tar.gz archive with given files
func createTarGz(t *testing.T, files map[string][]byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzWriter)

	for name, content := range files {
		header := &tar.Header{
			Name:     name,
			Mode:     0o644,
			Size:     int64(len(content)),
			Typeflag: tar.TypeReg,
		}

		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatalf("Failed to write tar header: %v", err)
		}

		if _, err := tarWriter.Write(content); err != nil {
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

// Test FetchLatestRelease with mock server
func TestFetchLatestRelease(t *testing.T) {
	tests := []struct {
		name           string
		responseBody   string
		responseStatus int
		expectError    bool
		checkTag       string
	}{
		{
			name: "successful fetch",
			responseBody: `{
				"tag_name": "v1.2.3",
				"name": "Release 1.2.3",
				"body": "Release notes",
				"html_url": "https://github.com/test/repo/releases/tag/v1.2.3",
				"prerelease": false,
				"assets": [
					{
						"name": "binary.tar.gz",
						"size": 1024,
						"browser_download_url": "https://example.com/binary.tar.gz"
					}
				]
			}`,
			responseStatus: http.StatusOK,
			expectError:    false,
			checkTag:       "v1.2.3",
		},
		{
			name:           "not found",
			responseBody:   "",
			responseStatus: http.StatusNotFound,
			expectError:    true,
		},
		{
			name:           "server error",
			responseBody:   "",
			responseStatus: http.StatusInternalServerError,
			expectError:    true,
		},
		{
			name:           "invalid JSON",
			responseBody:   "not json",
			responseStatus: http.StatusOK,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify headers
				if r.Header.Get("Accept") != "application/vnd.github.v3+json" {
					t.Errorf("Expected Accept header application/vnd.github.v3+json, got %s", r.Header.Get("Accept"))
				}
				if r.Header.Get("User-Agent") != "push-validator-cli" {
					t.Errorf("Expected User-Agent push-validator-cli, got %s", r.Header.Get("User-Agent"))
				}

				w.WriteHeader(tt.responseStatus)
				if tt.responseBody != "" {
					w.Write([]byte(tt.responseBody))
				}
			}))
			defer server.Close()

			// Override httpClient to use our mock server
			originalClient := httpClient
			httpClient = &http.Client{
				Transport: &urlRewritingTransport{
					originalURL: latestReleaseURL,
					newURL:      server.URL,
					transport:   http.DefaultTransport,
				},
			}
			defer func() { httpClient = originalClient }()

			release, err := FetchLatestRelease()

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got nil")
				}
				if release != nil {
					t.Error("Expected nil release on error")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if release == nil {
					t.Fatal("Expected non-nil release")
				}
				if release.TagName != tt.checkTag {
					t.Errorf("Expected tag %s, got %s", tt.checkTag, release.TagName)
				}
			}
		})
	}
}

// Test FetchReleaseByTag with mock server
func TestFetchReleaseByTag(t *testing.T) {
	tests := []struct {
		name           string
		inputTag       string
		expectedURL    string
		responseBody   string
		responseStatus int
		expectError    bool
		checkTag       string
	}{
		{
			name:        "tag with v prefix",
			inputTag:    "v1.0.0",
			expectedURL: "/repos/pushchain/push-chain-node/releases/tags/v1.0.0",
			responseBody: `{
				"tag_name": "v1.0.0",
				"name": "Release 1.0.0",
				"body": "Release notes",
				"html_url": "https://github.com/test/repo/releases/tag/v1.0.0",
				"prerelease": false,
				"assets": []
			}`,
			responseStatus: http.StatusOK,
			expectError:    false,
			checkTag:       "v1.0.0",
		},
		{
			name:        "tag without v prefix (should be added)",
			inputTag:    "2.0.0",
			expectedURL: "/repos/pushchain/push-chain-node/releases/tags/v2.0.0",
			responseBody: `{
				"tag_name": "v2.0.0",
				"name": "Release 2.0.0",
				"body": "Release notes",
				"html_url": "https://github.com/test/repo/releases/tag/v2.0.0",
				"prerelease": false,
				"assets": []
			}`,
			responseStatus: http.StatusOK,
			expectError:    false,
			checkTag:       "v2.0.0",
		},
		{
			name:           "release not found",
			inputTag:       "v99.99.99",
			expectedURL:    "/repos/pushchain/push-chain-node/releases/tags/v99.99.99",
			responseBody:   "",
			responseStatus: http.StatusNotFound,
			expectError:    true,
		},
		{
			name:           "server error",
			inputTag:       "v1.0.0",
			expectedURL:    "/repos/pushchain/push-chain-node/releases/tags/v1.0.0",
			responseBody:   "",
			responseStatus: http.StatusInternalServerError,
			expectError:    true,
		},
		{
			name:           "invalid JSON response",
			inputTag:       "v1.0.0",
			expectedURL:    "/repos/pushchain/push-chain-node/releases/tags/v1.0.0",
			responseBody:   "not json",
			responseStatus: http.StatusOK,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify headers
				if r.Header.Get("Accept") != "application/vnd.github.v3+json" {
					t.Errorf("Expected Accept header application/vnd.github.v3+json, got %s", r.Header.Get("Accept"))
				}
				if r.Header.Get("User-Agent") != "push-validator-cli" {
					t.Errorf("Expected User-Agent push-validator-cli, got %s", r.Header.Get("User-Agent"))
				}

				w.WriteHeader(tt.responseStatus)
				if tt.responseBody != "" {
					w.Write([]byte(tt.responseBody))
				}
			}))
			defer server.Close()

			// Override httpClient to use our mock server
			originalClient := httpClient
			httpClient = &http.Client{
				Transport: &tagRewritingTransport{
					baseURL:   server.URL,
					transport: http.DefaultTransport,
				},
			}
			defer func() { httpClient = originalClient }()

			release, err := FetchReleaseByTag(tt.inputTag)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got nil")
				}
				if release != nil {
					t.Error("Expected nil release on error")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if release == nil {
					t.Fatal("Expected non-nil release")
				}
				if release.TagName != tt.checkTag {
					t.Errorf("Expected tag %s, got %s", tt.checkTag, release.TagName)
				}
			}
		})
	}
}

// urlRewritingTransport is a custom RoundTripper that rewrites URLs for testing
type urlRewritingTransport struct {
	originalURL string
	newURL      string
	transport   http.RoundTripper
}

func (t *urlRewritingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.String() == t.originalURL {
		newReq := req.Clone(req.Context())
		newURL, _ := http.NewRequest(req.Method, t.newURL, req.Body)
		newReq.URL = newURL.URL
		return t.transport.RoundTrip(newReq)
	}
	return t.transport.RoundTrip(req)
}

// tagRewritingTransport rewrites GitHub API URLs to point to test server
type tagRewritingTransport struct {
	baseURL   string
	transport http.RoundTripper
}

func (t *tagRewritingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Rewrite GitHub API URLs to test server
	if strings.Contains(req.URL.String(), "api.github.com/repos/pushchain/push-chain-node/releases/tags/") {
		newReq := req.Clone(req.Context())
		newURL, _ := http.NewRequest(req.Method, t.baseURL, req.Body)
		newReq.URL = newURL.URL
		return t.transport.RoundTrip(newReq)
	}
	return t.transport.RoundTrip(req)
}

// Test Release JSON parsing and structure
func TestReleaseStructure(t *testing.T) {
	// Test that we can parse a release JSON response
	jsonData := `{
		"tag_name": "v1.0.0",
		"name": "Release 1.0.0",
		"body": "Release notes",
		"html_url": "https://github.com/test/repo/releases/tag/v1.0.0",
		"prerelease": false,
		"assets": [
			{
				"name": "binary.tar.gz",
				"size": 1024,
				"browser_download_url": "https://example.com/binary.tar.gz"
			}
		]
	}`

	var release Release
	err := json.Unmarshal([]byte(jsonData), &release)
	if err != nil {
		t.Fatalf("Failed to unmarshal release: %v", err)
	}

	if release.TagName != "v1.0.0" {
		t.Errorf("Expected TagName v1.0.0, got %s", release.TagName)
	}

	if release.Name != "Release 1.0.0" {
		t.Errorf("Expected Name 'Release 1.0.0', got %s", release.Name)
	}

	if release.Prerelease {
		t.Error("Expected Prerelease to be false")
	}

	if len(release.Assets) != 1 {
		t.Fatalf("Expected 1 asset, got %d", len(release.Assets))
	}

	if release.Assets[0].Name != "binary.tar.gz" {
		t.Errorf("Expected asset name binary.tar.gz, got %s", release.Assets[0].Name)
	}
}

// Test Asset structure
func TestAssetStructure(t *testing.T) {
	asset := Asset{
		Name:               "test.tar.gz",
		Size:               2048,
		BrowserDownloadURL: "https://example.com/test.tar.gz",
	}

	if asset.Name != "test.tar.gz" {
		t.Errorf("Expected Name test.tar.gz, got %s", asset.Name)
	}

	if asset.Size != 2048 {
		t.Errorf("Expected Size 2048, got %d", asset.Size)
	}

	if asset.BrowserDownloadURL != "https://example.com/test.tar.gz" {
		t.Errorf("Expected URL https://example.com/test.tar.gz, got %s", asset.BrowserDownloadURL)
	}
}

// Test extractFile function coverage
func TestExtractFile(t *testing.T) {
	tempDir := t.TempDir()
	destPath := filepath.Join(tempDir, "testfile")

	testContent := []byte("test file content")
	reader := bytes.NewReader(testContent)

	err := extractFile(reader, destPath, 0o644)
	if err != nil {
		t.Fatalf("extractFile failed: %v", err)
	}

	// Verify file exists
	info, err := os.Stat(destPath)
	if err != nil {
		t.Fatalf("File not created: %v", err)
	}

	// Verify permissions
	if info.Mode().Perm() != 0o644 {
		t.Errorf("Expected permissions 0644, got %o", info.Mode().Perm())
	}

	// Verify content
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if !bytes.Equal(content, testContent) {
		t.Errorf("Content mismatch: expected %v, got %v", testContent, content)
	}

	// Test overwriting existing file
	newContent := []byte("new content")
	reader = bytes.NewReader(newContent)
	err = extractFile(reader, destPath, 0o755)
	if err != nil {
		t.Fatalf("extractFile failed on overwrite: %v", err)
	}

	content, err = os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if !bytes.Equal(content, newContent) {
		t.Errorf("Content mismatch after overwrite: expected %v, got %v", newContent, content)
	}
}

// Test Download with nil progress callback
func TestDownloadNilProgress(t *testing.T) {
	testData := []byte("test data without progress")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(testData)
	}))
	defer server.Close()

	installer := NewInstaller(t.TempDir())
	asset := &Asset{
		Name:               "test.tar.gz",
		BrowserDownloadURL: server.URL,
		Size:               int64(len(testData)),
	}

	data, err := installer.Download(asset, nil)
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	if !bytes.Equal(data, testData) {
		t.Errorf("Data mismatch: expected %v, got %v", testData, data)
	}
}

// Test VerifyChecksum with server error
func TestVerifyChecksumServerError(t *testing.T) {
	testData := []byte("test data")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	installer := NewInstaller(t.TempDir())
	release := &Release{
		Assets: []Asset{
			{
				Name:               "binary.tar.gz",
				BrowserDownloadURL: "https://example.com/binary.tar.gz",
			},
			{
				Name:               "binary.tar.gz.sha256",
				BrowserDownloadURL: server.URL,
			},
		},
	}

	verified, err := installer.VerifyChecksum(testData, release, "binary.tar.gz")
	if err == nil {
		t.Error("Expected error from server error")
	}
	if verified {
		t.Error("Should not be verified on server error")
	}
}

// Test ExtractAndInstall with directory creation error
func TestExtractAndInstallDirectoryPermissions(t *testing.T) {
	// Create a tar.gz with pchaind
	archiveData := createTarGz(t, map[string][]byte{
		"pchaind": []byte("test binary"),
	})

	// Use a temporary directory
	homeDir := t.TempDir()
	installer := NewInstaller(homeDir)

	// Test successful extraction
	path, err := installer.ExtractAndInstall(archiveData)
	if err != nil {
		t.Fatalf("ExtractAndInstall failed: %v", err)
	}

	expectedPath := filepath.Join(homeDir, "cosmovisor", "genesis", "bin", "pchaind")
	if path != expectedPath {
		t.Errorf("Expected path %s, got %s", expectedPath, path)
	}
}

// Test ExtractAndInstall with tar read error
func TestExtractAndInstallTarReadError(t *testing.T) {
	// Create a valid gzip but with corrupted tar content
	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	gzWriter.Write([]byte("not a valid tar archive"))
	gzWriter.Close()

	homeDir := t.TempDir()
	installer := NewInstaller(homeDir)

	_, err := installer.ExtractAndInstall(buf.Bytes())
	if err == nil {
		t.Error("Expected error from corrupted tar archive")
	}
}

// Additional test for progressReader to ensure coverage
func TestProgressReader(t *testing.T) {
	testData := []byte("test data for progress reader")
	reader := bytes.NewReader(testData)

	var lastDownloaded, lastTotal int64
	progressCalled := 0

	pr := &progressReader{
		reader: reader,
		total:  int64(len(testData)),
		progress: func(downloaded, total int64) {
			progressCalled++
			lastDownloaded = downloaded
			lastTotal = total
		},
	}

	// Read in small chunks to trigger multiple progress updates
	buf := make([]byte, 10)
	totalRead := 0
	for {
		n, err := pr.Read(buf)
		totalRead += n
		if err != nil {
			break
		}
	}

	if progressCalled == 0 {
		t.Error("Progress callback was never called")
	}

	if lastDownloaded != int64(len(testData)) {
		t.Errorf("Expected final downloaded %d, got %d", len(testData), lastDownloaded)
	}

	if lastTotal != int64(len(testData)) {
		t.Errorf("Expected total %d, got %d", len(testData), lastTotal)
	}

	if totalRead != len(testData) {
		t.Errorf("Expected to read %d bytes, got %d", len(testData), totalRead)
	}
}

// Test ExtractAndInstall with directories in tar
func TestExtractAndInstallWithDirectories(t *testing.T) {
	// Create archive with directory entries (which should be skipped)
	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzWriter)

	// Add a directory entry
	dirHeader := &tar.Header{
		Name:     "somedir/",
		Mode:     0o755,
		Typeflag: tar.TypeDir,
	}
	tarWriter.WriteHeader(dirHeader)

	// Add the binary inside the directory
	fileHeader := &tar.Header{
		Name:     "somedir/pchaind",
		Mode:     0o755,
		Size:     int64(len("binary content")),
		Typeflag: tar.TypeReg,
	}
	tarWriter.WriteHeader(fileHeader)
	tarWriter.Write([]byte("binary content"))

	tarWriter.Close()
	gzWriter.Close()

	homeDir := t.TempDir()
	installer := NewInstaller(homeDir)

	path, err := installer.ExtractAndInstall(buf.Bytes())
	if err != nil {
		t.Fatalf("ExtractAndInstall failed: %v", err)
	}

	expectedPath := filepath.Join(homeDir, "cosmovisor", "genesis", "bin", "pchaind")
	if path != expectedPath {
		t.Errorf("Expected path %s, got %s", expectedPath, path)
	}
}

// Test ExtractAndInstall libwasmvm extraction
func TestExtractAndInstallLibWasmVM(t *testing.T) {
	archiveData := createTarGz(t, map[string][]byte{
		"pchaind":        []byte("pchaind binary"),
		"libwasmvm.dylib": []byte("wasm library"),
	})

	homeDir := t.TempDir()
	installer := NewInstaller(homeDir)

	_, err := installer.ExtractAndInstall(archiveData)
	if err != nil {
		t.Fatalf("ExtractAndInstall failed: %v", err)
	}

	// Verify libwasmvm was extracted
	libPath := filepath.Join(homeDir, "cosmovisor", "genesis", "bin", "libwasmvm.dylib")
	if _, err := os.Stat(libPath); os.IsNotExist(err) {
		t.Error("libwasmvm.dylib was not extracted")
	}
}

// Test Download with connection error
func TestDownloadConnectionError(t *testing.T) {
	installer := NewInstaller(t.TempDir())
	asset := &Asset{
		Name:               "test.tar.gz",
		BrowserDownloadURL: "http://localhost:1", // Invalid port, should fail
		Size:               100,
	}

	_, err := installer.Download(asset, nil)
	if err == nil {
		t.Error("Expected connection error")
	}
}

// Test VerifyChecksum with whitespace-only checksum file
func TestVerifyChecksumWhitespaceOnly(t *testing.T) {
	testData := []byte("test data")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("   \n\n   \n"))
	}))
	defer server.Close()

	installer := NewInstaller(t.TempDir())
	release := &Release{
		Assets: []Asset{
			{
				Name:               "binary.tar.gz",
				BrowserDownloadURL: "https://example.com/binary.tar.gz",
			},
			{
				Name:               "binary.tar.gz.sha256",
				BrowserDownloadURL: server.URL,
			},
		},
	}

	verified, err := installer.VerifyChecksum(testData, release, "binary.tar.gz")
	if err == nil {
		t.Error("Expected error for empty checksum")
	}
	if verified {
		t.Error("Should not be verified with empty checksum")
	}
}

// Test extractFile with write error (permission denied simulation)
func TestExtractFilePermissionError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Permission test not reliable on Windows")
	}

	tempDir := t.TempDir()

	// Create a read-only directory
	readOnlyDir := filepath.Join(tempDir, "readonly")
	err := os.Mkdir(readOnlyDir, 0o555)
	if err != nil {
		t.Fatalf("Failed to create readonly dir: %v", err)
	}

	destPath := filepath.Join(readOnlyDir, "testfile")
	reader := bytes.NewReader([]byte("content"))

	err = extractFile(reader, destPath, 0o644)
	if err == nil {
		t.Error("Expected permission error")
	}
}

package update

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"testing"
	"time"
)

// mockHTTPDoer is a test helper for mocking HTTP calls.
type mockHTTPDoer struct {
	doFunc func(*http.Request) (*http.Response, error)
}

func (m *mockHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	return m.doFunc(req)
}

func TestIsNewerVersion(t *testing.T) {
	tests := []struct {
		name    string
		current string
		latest  string
		want    bool
	}{
		{
			name:    "newer version available",
			current: "v1.0.0",
			latest:  "v1.1.0",
			want:    true,
		},
		{
			name:    "newer version without v prefix",
			current: "1.0.0",
			latest:  "1.1.0",
			want:    true,
		},
		{
			name:    "major version upgrade",
			current: "v1.9.9",
			latest:  "v2.0.0",
			want:    true,
		},
		{
			name:    "same version",
			current: "v1.0.0",
			latest:  "v1.0.0",
			want:    false,
		},
		{
			name:    "current is newer",
			current: "v2.0.0",
			latest:  "v1.9.9",
			want:    false,
		},
		{
			name:    "dev version always upgrades",
			current: "dev",
			latest:  "v1.0.0",
			want:    true,
		},
		{
			name:    "unknown version always upgrades",
			current: "unknown",
			latest:  "v1.0.0",
			want:    true,
		},
		{
			name:    "invalid current version",
			current: "not-a-version",
			latest:  "v1.0.0",
			want:    true,
		},
		{
			name:    "invalid latest version",
			current: "v1.0.0",
			latest:  "not-a-version",
			want:    false,
		},
		{
			name:    "patch version upgrade",
			current: "v1.0.0",
			latest:  "v1.0.1",
			want:    true,
		},
		{
			name:    "mixed prefix - current without v",
			current: "1.0.0",
			latest:  "v1.1.0",
			want:    true,
		},
		{
			name:    "mixed prefix - latest without v",
			current: "v1.0.0",
			latest:  "1.1.0",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsNewerVersion(tt.current, tt.latest)
			if got != tt.want {
				t.Errorf("IsNewerVersion(%q, %q) = %v, want %v",
					tt.current, tt.latest, got, tt.want)
			}
		})
	}
}

func TestGetAssetForPlatform(t *testing.T) {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	tests := []struct {
		name    string
		release *Release
		wantErr bool
		wantNil bool
	}{
		{
			name: "matching asset found",
			release: &Release{
				TagName: "v1.0.0",
				Assets: []Asset{
					{
						Name:               "push-validator_1.0.0_" + osName + "_" + arch + ".tar.gz",
						BrowserDownloadURL: "https://example.com/binary.tar.gz",
					},
				},
			},
			wantErr: false,
			wantNil: false,
		},
		{
			name: "no matching asset - different OS",
			release: &Release{
				TagName: "v1.0.0",
				Assets: []Asset{
					{
						Name:               "push-validator_1.0.0_different_os_amd64.tar.gz",
						BrowserDownloadURL: "https://example.com/binary.tar.gz",
					},
				},
			},
			wantErr: true,
			wantNil: true,
		},
		{
			name: "no matching asset - different arch",
			release: &Release{
				TagName: "v1.0.0",
				Assets: []Asset{
					{
						Name:               "push-validator_1.0.0_" + osName + "_different_arch.tar.gz",
						BrowserDownloadURL: "https://example.com/binary.tar.gz",
					},
				},
			},
			wantErr: true,
			wantNil: true,
		},
		{
			name: "empty assets",
			release: &Release{
				TagName: "v1.0.0",
				Assets:  []Asset{},
			},
			wantErr: true,
			wantNil: true,
		},
		{
			name: "multiple assets with one match",
			release: &Release{
				TagName: "v1.0.0",
				Assets: []Asset{
					{
						Name:               "push-validator_1.0.0_linux_amd64.tar.gz",
						BrowserDownloadURL: "https://example.com/linux.tar.gz",
					},
					{
						Name:               "push-validator_1.0.0_" + osName + "_" + arch + ".tar.gz",
						BrowserDownloadURL: "https://example.com/match.tar.gz",
					},
					{
						Name:               "push-validator_1.0.0_darwin_arm64.tar.gz",
						BrowserDownloadURL: "https://example.com/darwin.tar.gz",
					},
				},
			},
			wantErr: false,
			wantNil: false,
		},
		{
			name: "wrong prefix",
			release: &Release{
				TagName: "v1.0.0",
				Assets: []Asset{
					{
						Name:               "other_binary_1.0.0_" + osName + "_" + arch + ".tar.gz",
						BrowserDownloadURL: "https://example.com/binary.tar.gz",
					},
				},
			},
			wantErr: true,
			wantNil: true,
		},
		{
			name: "wrong extension",
			release: &Release{
				TagName: "v1.0.0",
				Assets: []Asset{
					{
						Name:               "push-validator_1.0.0_" + osName + "_" + arch + ".zip",
						BrowserDownloadURL: "https://example.com/binary.zip",
					},
				},
			},
			wantErr: true,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetAssetForPlatform(tt.release)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetAssetForPlatform() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if (got == nil) != tt.wantNil {
				t.Errorf("GetAssetForPlatform() got = %v, wantNil %v", got, tt.wantNil)
			}
		})
	}
}

func TestGetChecksumAsset(t *testing.T) {
	tests := []struct {
		name    string
		release *Release
		wantErr bool
	}{
		{
			name: "checksums.txt found",
			release: &Release{
				TagName: "v1.0.0",
				Assets: []Asset{
					{
						Name:               "push-validator_1.0.0_linux_amd64.tar.gz",
						BrowserDownloadURL: "https://example.com/binary.tar.gz",
					},
					{
						Name:               "checksums.txt",
						BrowserDownloadURL: "https://example.com/checksums.txt",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "checksums.txt not found",
			release: &Release{
				TagName: "v1.0.0",
				Assets: []Asset{
					{
						Name:               "push-validator_1.0.0_linux_amd64.tar.gz",
						BrowserDownloadURL: "https://example.com/binary.tar.gz",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "empty assets",
			release: &Release{
				TagName: "v1.0.0",
				Assets:  []Asset{},
			},
			wantErr: true,
		},
		{
			name: "wrong checksum filename",
			release: &Release{
				TagName: "v1.0.0",
				Assets: []Asset{
					{
						Name:               "SHA256SUMS",
						BrowserDownloadURL: "https://example.com/sums",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetChecksumAsset(tt.release)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetChecksumAsset() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got == nil {
				t.Error("GetChecksumAsset() got nil, want non-nil")
			}
			if !tt.wantErr && got.Name != "checksums.txt" {
				t.Errorf("GetChecksumAsset() name = %q, want %q", got.Name, "checksums.txt")
			}
		})
	}
}

func TestFetchLatestRelease(t *testing.T) {
	testRelease := Release{
		TagName:     "v1.2.3",
		Name:        "Release 1.2.3",
		Body:        "Release notes",
		Draft:       false,
		Prerelease:  false,
		PublishedAt: time.Now(),
		HTMLURL:     "https://github.com/pushchain/push-validator-cli/releases/tag/v1.2.3",
		Assets: []Asset{
			{
				Name:               "push-validator_1.2.3_linux_amd64.tar.gz",
				BrowserDownloadURL: "https://example.com/binary.tar.gz",
				Size:               1024,
				ContentType:        "application/gzip",
			},
		},
	}

	tests := []struct {
		name       string
		statusCode int
		response   interface{}
		wantErr    bool
	}{
		{
			name:       "successful fetch",
			statusCode: http.StatusOK,
			response:   testRelease,
			wantErr:    false,
		},
		{
			name:       "not found",
			statusCode: http.StatusNotFound,
			response:   map[string]string{"message": "Not Found"},
			wantErr:    true,
		},
		{
			name:       "server error",
			statusCode: http.StatusInternalServerError,
			response:   map[string]string{"message": "Internal Server Error"},
			wantErr:    true,
		},
		{
			name:       "rate limit",
			statusCode: http.StatusForbidden,
			response:   map[string]string{"message": "Rate limit exceeded"},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockHTTPDoer{
				doFunc: func(req *http.Request) (*http.Response, error) {
					// Verify request headers
					if req.Header.Get("Accept") != "application/vnd.github.v3+json" {
						t.Errorf("Accept header = %q, want %q",
							req.Header.Get("Accept"), "application/vnd.github.v3+json")
					}
					if req.Header.Get("User-Agent") != "push-validator-cli" {
						t.Errorf("User-Agent header = %q, want %q",
							req.Header.Get("User-Agent"), "push-validator-cli")
					}

					body, _ := json.Marshal(tt.response)
					return &http.Response{
						StatusCode: tt.statusCode,
						Body:       io.NopCloser(bytes.NewReader(body)),
					}, nil
				},
			}

			u := &Updater{
				CurrentVersion: "1.0.0",
				BinaryPath:     "/tmp/test-binary",
				http:           mock,
			}

			release, err := u.FetchLatestRelease()
			if (err != nil) != tt.wantErr {
				t.Errorf("FetchLatestRelease() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && release != nil {
				if release.TagName != testRelease.TagName {
					t.Errorf("TagName = %q, want %q", release.TagName, testRelease.TagName)
				}
			}
		})
	}
}

func TestFetchLatestRelease_NetworkError(t *testing.T) {
	mock := &mockHTTPDoer{
		doFunc: func(req *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("network unreachable")
		},
	}

	u := &Updater{
		CurrentVersion: "1.0.0",
		BinaryPath:     "/tmp/test-binary",
		http:           mock,
	}

	_, err := u.FetchLatestRelease()
	if err == nil {
		t.Error("FetchLatestRelease() expected error for network failure, got nil")
	}
}

func TestFetchReleaseByTag(t *testing.T) {
	testRelease := Release{
		TagName:     "v1.2.3",
		Name:        "Release 1.2.3",
		Body:        "Release notes",
		Draft:       false,
		Prerelease:  false,
		PublishedAt: time.Now(),
		HTMLURL:     "https://github.com/pushchain/push-validator-cli/releases/tag/v1.2.3",
		Assets:      []Asset{},
	}

	tests := []struct {
		name       string
		tag        string
		statusCode int
		response   interface{}
		wantErr    bool
	}{
		{
			name:       "fetch with v prefix",
			tag:        "v1.2.3",
			statusCode: http.StatusOK,
			response:   testRelease,
			wantErr:    false,
		},
		{
			name:       "fetch without v prefix",
			tag:        "1.2.3",
			statusCode: http.StatusOK,
			response:   testRelease,
			wantErr:    false,
		},
		{
			name:       "release not found",
			tag:        "v9.9.9",
			statusCode: http.StatusNotFound,
			response:   map[string]string{"message": "Not Found"},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockHTTPDoer{
				doFunc: func(req *http.Request) (*http.Response, error) {
					// Verify headers
					if req.Header.Get("Accept") != "application/vnd.github.v3+json" {
						t.Errorf("Accept header = %q, want %q",
							req.Header.Get("Accept"), "application/vnd.github.v3+json")
					}

					body, _ := json.Marshal(tt.response)
					return &http.Response{
						StatusCode: tt.statusCode,
						Body:       io.NopCloser(bytes.NewReader(body)),
					}, nil
				},
			}

			u := &Updater{
				CurrentVersion: "1.0.0",
				BinaryPath:     "/tmp/test-binary",
				http:           mock,
			}

			release, err := u.FetchReleaseByTag(tt.tag)
			if (err != nil) != tt.wantErr {
				t.Errorf("FetchReleaseByTag(%q) error = %v, wantErr %v", tt.tag, err, tt.wantErr)
				return
			}

			if !tt.wantErr && release != nil {
				if release.TagName != testRelease.TagName {
					t.Errorf("TagName = %q, want %q", release.TagName, testRelease.TagName)
				}
			}
		})
	}
}

func TestFetchReleaseByTag_NetworkError(t *testing.T) {
	mock := &mockHTTPDoer{
		doFunc: func(req *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}

	u := &Updater{
		CurrentVersion: "1.0.0",
		BinaryPath:     "/tmp/test-binary",
		http:           mock,
	}

	_, err := u.FetchReleaseByTag("v1.0.0")
	if err == nil {
		t.Error("FetchReleaseByTag() expected error for network failure, got nil")
	}
}

// TestTypesStructs tests the basic type structures
func TestTypesStructs(t *testing.T) {
	// Test Release struct
	release := Release{
		TagName:     "v1.0.0",
		Name:        "Test Release",
		Body:        "Release notes",
		Draft:       false,
		Prerelease:  false,
		PublishedAt: time.Now(),
		HTMLURL:     "https://example.com",
		Assets:      []Asset{},
	}

	if release.TagName != "v1.0.0" {
		t.Errorf("Release.TagName = %q, want %q", release.TagName, "v1.0.0")
	}

	// Test Asset struct
	asset := Asset{
		Name:               "test.tar.gz",
		BrowserDownloadURL: "https://example.com/test.tar.gz",
		Size:               1024,
		ContentType:        "application/gzip",
	}

	if asset.Name != "test.tar.gz" {
		t.Errorf("Asset.Name = %q, want %q", asset.Name, "test.tar.gz")
	}

	// Test CheckResult struct
	result := CheckResult{
		CurrentVersion:  "1.0.0",
		LatestVersion:   "1.1.0",
		UpdateAvailable: true,
		Release:         &release,
	}

	if !result.UpdateAvailable {
		t.Error("CheckResult.UpdateAvailable = false, want true")
	}

	// Test UpdateOptions struct
	opts := UpdateOptions{
		Force:      true,
		SkipVerify: false,
		Version:    "1.2.3",
	}

	if !opts.Force {
		t.Error("UpdateOptions.Force = false, want true")
	}
}

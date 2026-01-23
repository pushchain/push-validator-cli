package cosmovisor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDefaultPlatforms(t *testing.T) {
	platforms := DefaultPlatforms()

	expectedPlatforms := []Platform{
		{OS: "linux", Arch: "amd64"},
		{OS: "linux", Arch: "arm64"},
		{OS: "darwin", Arch: "arm64"},
	}

	if len(platforms) != len(expectedPlatforms) {
		t.Fatalf("DefaultPlatforms() returned %d platforms, want %d", len(platforms), len(expectedPlatforms))
	}

	for i, expected := range expectedPlatforms {
		if platforms[i].OS != expected.OS || platforms[i].Arch != expected.Arch {
			t.Errorf("platforms[%d] = %+v, want %+v", i, platforms[i], expected)
		}
	}
}

func TestFormatUpgradeProposal(t *testing.T) {
	tests := []struct {
		name      string
		info      *UpgradeInfo
		authority string
		deposit   string
		title     string
		summary   string
		wantErr   bool
		validate  func(t *testing.T, data []byte)
	}{
		{
			name: "valid upgrade info",
			info: &UpgradeInfo{
				Name:   "v1.1.0",
				Height: 12345,
				Info:   `{"binaries":{"linux/amd64":"https://example.com/binary?checksum=sha256:abc123"}}`,
			},
			authority: "pchain10d07y265gmmuvt4z0w9aw880jnsr700jux7803",
			deposit:   "10000000upush",
			title:     "Upgrade to v1.1.0",
			summary:   "This upgrade includes bug fixes and improvements",
			wantErr:   false,
			validate: func(t *testing.T, data []byte) {
				var proposal map[string]interface{}
				if err := json.Unmarshal(data, &proposal); err != nil {
					t.Fatalf("failed to unmarshal proposal: %v", err)
				}

				// Check top-level fields
				if proposal["title"] != "Upgrade to v1.1.0" {
					t.Errorf("title = %v, want 'Upgrade to v1.1.0'", proposal["title"])
				}
				if proposal["summary"] != "This upgrade includes bug fixes and improvements" {
					t.Errorf("summary = %v, want expected summary", proposal["summary"])
				}
				if proposal["deposit"] != "10000000upush" {
					t.Errorf("deposit = %v, want '10000000upush'", proposal["deposit"])
				}

				// Check messages
				messages, ok := proposal["messages"].([]interface{})
				if !ok || len(messages) != 1 {
					t.Fatal("messages should be array with 1 element")
				}

				msg, ok := messages[0].(map[string]interface{})
				if !ok {
					t.Fatal("message should be object")
				}

				if msg["@type"] != "/cosmos.upgrade.v1beta1.MsgSoftwareUpgrade" {
					t.Errorf("@type = %v, want MsgSoftwareUpgrade", msg["@type"])
				}

				if msg["authority"] != "pchain10d07y265gmmuvt4z0w9aw880jnsr700jux7803" {
					t.Errorf("authority = %v, want expected authority", msg["authority"])
				}

				// Check plan
				plan, ok := msg["plan"].(map[string]interface{})
				if !ok {
					t.Fatal("plan should be object")
				}

				if plan["name"] != "v1.1.0" {
					t.Errorf("plan.name = %v, want 'v1.1.0'", plan["name"])
				}

				if plan["height"] != "12345" {
					t.Errorf("plan.height = %v, want '12345'", plan["height"])
				}

				if !strings.Contains(plan["info"].(string), "binaries") {
					t.Error("plan.info should contain 'binaries'")
				}
			},
		},
		{
			name: "zero height",
			info: &UpgradeInfo{
				Name:   "v1.0.0",
				Height: 0,
				Info:   `{"binaries":{}}`,
			},
			authority: "pchain10d07y265gmmuvt4z0w9aw880jnsr700jux7803",
			deposit:   "5000000upush",
			title:     "Test Upgrade",
			summary:   "Test",
			wantErr:   false,
			validate: func(t *testing.T, data []byte) {
				var proposal map[string]interface{}
				if err := json.Unmarshal(data, &proposal); err != nil {
					t.Fatalf("failed to unmarshal proposal: %v", err)
				}

				messages := proposal["messages"].([]interface{})
				msg := messages[0].(map[string]interface{})
				plan := msg["plan"].(map[string]interface{})

				if plan["height"] != "0" {
					t.Errorf("plan.height = %v, want '0'", plan["height"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := FormatUpgradeProposal(tt.info, tt.authority, tt.deposit, tt.title, tt.summary)

			if (err != nil) != tt.wantErr {
				t.Errorf("FormatUpgradeProposal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil && tt.validate != nil {
				tt.validate(t, data)
			}
		})
	}
}

func TestGenerateUpgradeInfo(t *testing.T) {
	tests := []struct {
		name      string
		opts      GenerateUpgradeInfoOptions
		setupFunc func() *httptest.Server
		wantErr   bool
		errMsg    string
		validate  func(t *testing.T, info *UpgradeInfo)
	}{
		{
			name: "success - single platform",
			opts: GenerateUpgradeInfoOptions{
				Version:     "v1.1.0",
				ProjectName: "test-project",
				Platforms: []Platform{
					{OS: "linux", Arch: "amd64"},
				},
			},
			setupFunc: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Return fake binary data
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("fake binary content"))
				}))
			},
			wantErr: false,
			validate: func(t *testing.T, info *UpgradeInfo) {
				if info.Name != "v1.1.0" {
					t.Errorf("Name = %q, want 'v1.1.0'", info.Name)
				}

				var binaryInfo BinaryInfo
				if err := json.Unmarshal([]byte(info.Info), &binaryInfo); err != nil {
					t.Fatalf("failed to unmarshal info: %v", err)
				}

				if len(binaryInfo.Binaries) != 1 {
					t.Errorf("binaries count = %d, want 1", len(binaryInfo.Binaries))
				}

				linuxAmd64, ok := binaryInfo.Binaries["linux/amd64"]
				if !ok {
					t.Fatal("missing linux/amd64 binary")
				}

				if !strings.Contains(linuxAmd64, "checksum=sha256:") {
					t.Error("binary URL should contain checksum")
				}

				// Verify checksum is correct
				expectedChecksum := sha256.Sum256([]byte("fake binary content"))
				expectedHex := hex.EncodeToString(expectedChecksum[:])
				if !strings.Contains(linuxAmd64, expectedHex) {
					t.Errorf("checksum mismatch, expected to contain %s", expectedHex)
				}
			},
		},
		{
			name: "success - multiple platforms",
			opts: GenerateUpgradeInfoOptions{
				Version:     "v1.2.0",
				ProjectName: "push-chain",
				Height:      10000,
				Platforms: []Platform{
					{OS: "linux", Arch: "amd64"},
					{OS: "linux", Arch: "arm64"},
					{OS: "darwin", Arch: "arm64"},
				},
			},
			setupFunc: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Return different content based on platform
					content := fmt.Sprintf("binary for %s", r.URL.Path)
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(content))
				}))
			},
			wantErr: false,
			validate: func(t *testing.T, info *UpgradeInfo) {
				if info.Height != 10000 {
					t.Errorf("Height = %d, want 10000", info.Height)
				}

				var binaryInfo BinaryInfo
				if err := json.Unmarshal([]byte(info.Info), &binaryInfo); err != nil {
					t.Fatalf("failed to unmarshal info: %v", err)
				}

				if len(binaryInfo.Binaries) != 3 {
					t.Errorf("binaries count = %d, want 3", len(binaryInfo.Binaries))
				}

				expectedPlatforms := []string{"linux/amd64", "linux/arm64", "darwin/arm64"}
				for _, platform := range expectedPlatforms {
					if _, ok := binaryInfo.Binaries[platform]; !ok {
						t.Errorf("missing %s binary", platform)
					}
				}
			},
		},
		{
			name: "success - default platforms",
			opts: GenerateUpgradeInfoOptions{
				Version: "v1.0.0",
				// No platforms specified, should use defaults
			},
			setupFunc: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("binary"))
				}))
			},
			wantErr: false,
			validate: func(t *testing.T, info *UpgradeInfo) {
				var binaryInfo BinaryInfo
				if err := json.Unmarshal([]byte(info.Info), &binaryInfo); err != nil {
					t.Fatalf("failed to unmarshal info: %v", err)
				}

				// Should have default platforms
				if len(binaryInfo.Binaries) != 3 {
					t.Errorf("binaries count = %d, want 3 (default platforms)", len(binaryInfo.Binaries))
				}
			},
		},
		{
			name: "partial success - some platforms fail",
			opts: GenerateUpgradeInfoOptions{
				Version:     "v1.1.0",
				ProjectName: "test-project",
				Platforms: []Platform{
					{OS: "linux", Arch: "amd64"},
					{OS: "linux", Arch: "arm64"},
				},
			},
			setupFunc: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Fail for arm64, succeed for amd64
					if strings.Contains(r.URL.Path, "arm64") {
						w.WriteHeader(http.StatusNotFound)
						return
					}
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("binary"))
				}))
			},
			wantErr: false,
			validate: func(t *testing.T, info *UpgradeInfo) {
				var binaryInfo BinaryInfo
				if err := json.Unmarshal([]byte(info.Info), &binaryInfo); err != nil {
					t.Fatalf("failed to unmarshal info: %v", err)
				}

				// Should have only amd64
				if len(binaryInfo.Binaries) != 1 {
					t.Errorf("binaries count = %d, want 1", len(binaryInfo.Binaries))
				}

				if _, ok := binaryInfo.Binaries["linux/amd64"]; !ok {
					t.Error("missing linux/amd64 binary")
				}
			},
		},
		{
			name: "error - no version",
			opts: GenerateUpgradeInfoOptions{
				Version: "",
			},
			setupFunc: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
			},
			wantErr: true,
			errMsg:  "version is required",
		},
		{
			name: "error - no base URL",
			opts: GenerateUpgradeInfoOptions{
				Version: "v1.0.0",
				BaseURL: "",
			},
			setupFunc: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
			},
			wantErr: true,
			errMsg:  "base URL is required",
		},
		{
			name: "error - all platforms fail",
			opts: GenerateUpgradeInfoOptions{
				Version:     "v1.0.0",
				ProjectName: "test-project",
				Platforms: []Platform{
					{OS: "linux", Arch: "amd64"},
				},
			},
			setupFunc: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotFound)
				}))
			},
			wantErr: true,
			errMsg:  "no binaries found",
		},
		{
			name: "success - version with v prefix stripped",
			opts: GenerateUpgradeInfoOptions{
				Version:     "v2.0.0",
				ProjectName: "test-project",
				Platforms: []Platform{
					{OS: "linux", Arch: "amd64"},
				},
			},
			setupFunc: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Verify that the version in the URL doesn't have 'v' prefix
					if !strings.Contains(r.URL.Path, "2.0.0") {
						t.Errorf("URL path should contain '2.0.0', got: %s", r.URL.Path)
					}
					if strings.Contains(r.URL.Path, "v2.0.0") {
						t.Error("URL path should not contain 'v2.0.0' (v should be stripped)")
					}
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("binary"))
				}))
			},
			wantErr: false,
			validate: func(t *testing.T, info *UpgradeInfo) {
				// Name should keep the v prefix
				if info.Name != "v2.0.0" {
					t.Errorf("Name = %q, want 'v2.0.0'", info.Name)
				}
			},
		},
		{
			name: "success - with progress callback",
			opts: GenerateUpgradeInfoOptions{
				Version:     "v1.0.0",
				ProjectName: "test-project",
				Platforms: []Platform{
					{OS: "linux", Arch: "amd64"},
				},
				Progress: func(msg string) {
					t.Logf("Progress: %s", msg)
				},
			},
			setupFunc: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("binary"))
				}))
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupFunc()
			defer server.Close()

			// Set BaseURL if not testing error case
			if tt.opts.BaseURL == "" && tt.errMsg != "base URL is required" {
				tt.opts.BaseURL = server.URL
			}

			info, err := GenerateUpgradeInfo(context.Background(), tt.opts)

			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateUpgradeInfo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error = %q, want to contain %q", err.Error(), tt.errMsg)
				}
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, info)
			}
		})
	}
}

func TestFetchAndHash(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func() *httptest.Server
		wantErr   bool
		errMsg    string
		validate  func(t *testing.T, checksum string)
	}{
		{
			name: "success",
			setupFunc: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("test content"))
				}))
			},
			wantErr: false,
			validate: func(t *testing.T, checksum string) {
				expected := sha256.Sum256([]byte("test content"))
				expectedHex := hex.EncodeToString(expected[:])

				if checksum != expectedHex {
					t.Errorf("checksum = %q, want %q", checksum, expectedHex)
				}
			},
		},
		{
			name: "error - 404 not found",
			setupFunc: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotFound)
				}))
			},
			wantErr: true,
			errMsg:  "HTTP 404",
		},
		{
			name: "error - 500 internal error",
			setupFunc: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
				}))
			},
			wantErr: true,
			errMsg:  "HTTP 500",
		},
		{
			name: "success - empty content",
			setupFunc: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					// Empty response
				}))
			},
			wantErr: false,
			validate: func(t *testing.T, checksum string) {
				expected := sha256.Sum256([]byte(""))
				expectedHex := hex.EncodeToString(expected[:])

				if checksum != expectedHex {
					t.Errorf("checksum = %q, want %q", checksum, expectedHex)
				}
			},
		},
		{
			name: "success - large content",
			setupFunc: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					// Write 1MB of data
					data := make([]byte, 1024*1024)
					for i := range data {
						data[i] = byte(i % 256)
					}
					w.Write(data)
				}))
			},
			wantErr: false,
			validate: func(t *testing.T, checksum string) {
				if checksum == "" {
					t.Error("checksum is empty")
				}
				if len(checksum) != 64 { // SHA256 hex is 64 characters
					t.Errorf("checksum length = %d, want 64", len(checksum))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupFunc()
			defer server.Close()

			checksum, err := fetchAndHash(context.Background(), server.URL)

			if (err != nil) != tt.wantErr {
				t.Errorf("fetchAndHash() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error = %q, want to contain %q", err.Error(), tt.errMsg)
				}
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, checksum)
			}
		})
	}
}

func TestFetchAndHashContext(t *testing.T) {
	t.Run("context cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Simulate slow response
			select {
			case <-r.Context().Done():
				return
			}
		}))
		defer server.Close()

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := fetchAndHash(ctx, server.URL)
		if err == nil {
			t.Error("fetchAndHash() with cancelled context should return error")
		}
	})
}

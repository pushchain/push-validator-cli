package cosmovisor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	homeDir := t.TempDir()
	svc := New(homeDir)

	if svc == nil {
		t.Fatal("New() returned nil")
	}

	// Verify service implements Service interface
	var _ Service = svc
}

func TestGenesisDir(t *testing.T) {
	homeDir := t.TempDir()
	svc := New(homeDir).(*service)

	expected := filepath.Join(homeDir, "cosmovisor", "genesis", "bin")
	got := svc.GenesisDir()

	if got != expected {
		t.Errorf("GenesisDir() = %q, want %q", got, expected)
	}
}

func TestCosmovisorBinaryPath(t *testing.T) {
	homeDir := t.TempDir()
	svc := New(homeDir).(*service)

	// The binary path will be empty if cosmovisor isn't in PATH or COSMOVISOR env
	// This test just verifies the method returns the expected value
	path := svc.CosmovisorBinaryPath()

	// Path could be empty or a valid path
	if path != "" && !filepath.IsAbs(path) {
		t.Errorf("CosmovisorBinaryPath() returned non-absolute path: %q", path)
	}
}

func TestCurrentBinaryPath(t *testing.T) {
	homeDir := t.TempDir()
	svc := New(homeDir).(*service)

	tests := []struct {
		name           string
		setupFunc      func()
		expectedSuffix string
	}{
		{
			name: "no setup - returns genesis path",
			setupFunc: func() {
				// No setup
			},
			expectedSuffix: filepath.Join("cosmovisor", "genesis", "bin", "pchaind"),
		},
		{
			name: "with current symlink",
			setupFunc: func() {
				// Create genesis directory
				genesisDir := svc.GenesisDir()
				if err := os.MkdirAll(genesisDir, 0o755); err != nil {
					t.Fatal(err)
				}

				// Create upgrade directory
				upgradeDir := filepath.Join(svc.upgradesDir(), "v1.1.0", "bin")
				if err := os.MkdirAll(upgradeDir, 0o755); err != nil {
					t.Fatal(err)
				}

				// Create symlink
				currentLink := svc.currentLink()
				target := filepath.Join(svc.upgradesDir(), "v1.1.0")
				if err := os.Symlink(target, currentLink); err != nil {
					t.Fatal(err)
				}
			},
			expectedSuffix: filepath.Join("bin", "pchaind"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset for each test
			homeDir = t.TempDir()
			svc = New(homeDir).(*service)

			tt.setupFunc()

			got := svc.CurrentBinaryPath()
			if !strings.HasSuffix(got, tt.expectedSuffix) {
				t.Errorf("CurrentBinaryPath() = %q, expected suffix %q", got, tt.expectedSuffix)
			}
		})
	}
}

func TestIsSetup(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(homeDir string)
		want      bool
	}{
		{
			name: "not setup - no directory",
			setupFunc: func(homeDir string) {
				// No setup
			},
			want: false,
		},
		{
			name: "not setup - directory exists but no binary",
			setupFunc: func(homeDir string) {
				genesisDir := filepath.Join(homeDir, "cosmovisor", "genesis", "bin")
				if err := os.MkdirAll(genesisDir, 0o755); err != nil {
					t.Fatal(err)
				}
			},
			want: false,
		},
		{
			name: "setup complete - binary exists",
			setupFunc: func(homeDir string) {
				genesisDir := filepath.Join(homeDir, "cosmovisor", "genesis", "bin")
				if err := os.MkdirAll(genesisDir, 0o755); err != nil {
					t.Fatal(err)
				}
				binaryPath := filepath.Join(genesisDir, "pchaind")
				if err := os.WriteFile(binaryPath, []byte("fake binary"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			homeDir := t.TempDir()
			tt.setupFunc(homeDir)

			svc := New(homeDir)
			got := svc.IsSetup()

			if got != tt.want {
				t.Errorf("IsSetup() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEnvVars(t *testing.T) {
	homeDir := t.TempDir()
	svc := New(homeDir)

	envVars := svc.EnvVars()

	// Check all expected keys exist
	expectedKeys := []string{
		EnvDaemonName,
		EnvDaemonHome,
		EnvAllowDownloadBinaries,
		EnvRestartAfterUpgrade,
		EnvUnsafeSkipBackup,
	}

	for _, key := range expectedKeys {
		if _, ok := envVars[key]; !ok {
			t.Errorf("EnvVars() missing key: %q", key)
		}
	}

	// Verify specific values
	if envVars[EnvDaemonName] != "pchaind" {
		t.Errorf("DAEMON_NAME = %q, want %q", envVars[EnvDaemonName], "pchaind")
	}

	if envVars[EnvDaemonHome] != homeDir {
		t.Errorf("DAEMON_HOME = %q, want %q", envVars[EnvDaemonHome], homeDir)
	}

	if envVars[EnvAllowDownloadBinaries] != "true" {
		t.Errorf("DAEMON_ALLOW_DOWNLOAD_BINARIES = %q, want %q", envVars[EnvAllowDownloadBinaries], "true")
	}
}

func TestInit(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(t *testing.T) (homeDir, binPath string)
		wantErr   bool
		errMsg    string
	}{
		{
			name: "success - valid binary",
			setupFunc: func(t *testing.T) (string, string) {
				homeDir := t.TempDir()
				binDir := t.TempDir()
				binPath := filepath.Join(binDir, "pchaind")
				if err := os.WriteFile(binPath, []byte("fake binary content"), 0o755); err != nil {
					t.Fatal(err)
				}
				return homeDir, binPath
			},
			wantErr: false,
		},
		{
			name: "error - no binary path",
			setupFunc: func(t *testing.T) (string, string) {
				return t.TempDir(), ""
			},
			wantErr: true,
			errMsg:  "binary path is required",
		},
		{
			name: "error - binary doesn't exist",
			setupFunc: func(t *testing.T) (string, string) {
				return t.TempDir(), "/nonexistent/path/pchaind"
			},
			wantErr: true,
			errMsg:  "binary not found",
		},
		{
			name: "success - binary already in place",
			setupFunc: func(t *testing.T) (string, string) {
				homeDir := t.TempDir()
				genesisDir := filepath.Join(homeDir, "cosmovisor", "genesis", "bin")
				if err := os.MkdirAll(genesisDir, 0o755); err != nil {
					t.Fatal(err)
				}
				binPath := filepath.Join(genesisDir, "pchaind")
				if err := os.WriteFile(binPath, []byte("fake binary"), 0o755); err != nil {
					t.Fatal(err)
				}
				return homeDir, binPath
			},
			wantErr: false,
		},
		{
			name: "success - with libwasmvm.dylib",
			setupFunc: func(t *testing.T) (string, string) {
				homeDir := t.TempDir()
				binDir := t.TempDir()
				binPath := filepath.Join(binDir, "pchaind")
				if err := os.WriteFile(binPath, []byte("fake binary"), 0o755); err != nil {
					t.Fatal(err)
				}
				// Create libwasmvm.dylib in same directory
				libPath := filepath.Join(binDir, "libwasmvm.dylib")
				if err := os.WriteFile(libPath, []byte("fake library"), 0o644); err != nil {
					t.Fatal(err)
				}
				return homeDir, binPath
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			homeDir, binPath := tt.setupFunc(t)

			// Create a service with a mock cosmovisor binary path
			// Since we can't guarantee cosmovisor is in PATH, we'll set it manually
			svc := &service{
				homeDir: homeDir,
				binPath: "/usr/local/bin/cosmovisor", // Mock path
			}

			// If test expects error due to missing cosmovisor, clear binPath
			if tt.name == "error - no cosmovisor" {
				svc.binPath = ""
			}

			var progressMessages []string
			opts := InitOptions{
				HomeDir: homeDir,
				BinPath: binPath,
				Progress: func(msg string) {
					progressMessages = append(progressMessages, msg)
				},
			}

			err := svc.Init(context.Background(), opts)

			if tt.wantErr {
				if err == nil {
					t.Error("Init() expected error, got nil")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Init() error = %q, want to contain %q", err.Error(), tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("Init() unexpected error: %v", err)
				return
			}

			// Verify directory structure was created
			genesisDir := filepath.Join(homeDir, "cosmovisor", "genesis", "bin")
			if _, err := os.Stat(genesisDir); os.IsNotExist(err) {
				t.Errorf("genesis directory not created: %s", genesisDir)
			}

			upgradesDir := filepath.Join(homeDir, "cosmovisor", "upgrades")
			if _, err := os.Stat(upgradesDir); os.IsNotExist(err) {
				t.Errorf("upgrades directory not created: %s", upgradesDir)
			}

			// Verify binary was copied (unless it was already in place)
			if !strings.Contains(tt.name, "already in place") {
				destPath := filepath.Join(genesisDir, "pchaind")
				info, err := os.Stat(destPath)
				if err != nil {
					t.Errorf("binary not copied to genesis directory: %v", err)
				} else {
					// Check permissions
					if info.Mode().Perm() != 0o755 {
						t.Errorf("binary permissions = %o, want 0755", info.Mode().Perm())
					}
				}
			}

			// Verify libwasmvm.dylib was copied if present
			if strings.Contains(tt.name, "libwasmvm.dylib") {
				libDest := filepath.Join(genesisDir, "libwasmvm.dylib")
				if _, err := os.Stat(libDest); os.IsNotExist(err) {
					t.Error("libwasmvm.dylib not copied")
				}
			}

			// Verify progress was reported
			if len(progressMessages) == 0 {
				t.Error("no progress messages reported")
			}
		})
	}
}

func TestStatus(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(homeDir string) *service
		validate  func(t *testing.T, status *Status)
	}{
		{
			name: "cosmovisor not installed",
			setupFunc: func(homeDir string) *service {
				return &service{
					homeDir: homeDir,
					binPath: "", // Not installed
				}
			},
			validate: func(t *testing.T, status *Status) {
				if status.Installed {
					t.Error("Status.Installed = true, want false")
				}
			},
		},
		{
			name: "installed but not setup",
			setupFunc: func(homeDir string) *service {
				return &service{
					homeDir: homeDir,
					binPath: "/usr/local/bin/cosmovisor",
				}
			},
			validate: func(t *testing.T, status *Status) {
				if !status.Installed {
					t.Error("Status.Installed = false, want true")
				}
				if status.GenesisVersion != "" {
					t.Errorf("Status.GenesisVersion = %q, want empty", status.GenesisVersion)
				}
			},
		},
		{
			name: "with genesis binary",
			setupFunc: func(homeDir string) *service {
				svc := &service{
					homeDir: homeDir,
					binPath: "/usr/local/bin/cosmovisor",
				}

				// Create genesis binary (fake script that outputs version)
				genesisDir := svc.GenesisDir()
				if err := os.MkdirAll(genesisDir, 0o755); err != nil {
					t.Fatal(err)
				}

				// Write a fake binary (we can't test version without a real binary)
				binaryPath := filepath.Join(genesisDir, "pchaind")
				if err := os.WriteFile(binaryPath, []byte("fake binary"), 0o755); err != nil {
					t.Fatal(err)
				}

				return svc
			},
			validate: func(t *testing.T, status *Status) {
				if !status.Installed {
					t.Error("Status.Installed = false, want true")
				}
				// GenesisVersion will be "unknown" since fake binary can't run
				// ActiveBinary should point to genesis
				if !strings.Contains(status.ActiveBinary, "genesis/bin/pchaind") {
					t.Errorf("Status.ActiveBinary = %q, want to contain genesis/bin/pchaind", status.ActiveBinary)
				}
			},
		},
		{
			name: "with current symlink and pending upgrades",
			setupFunc: func(homeDir string) *service {
				svc := &service{
					homeDir: homeDir,
					binPath: "/usr/local/bin/cosmovisor",
				}

				// Create genesis binary
				genesisDir := svc.GenesisDir()
				if err := os.MkdirAll(genesisDir, 0o755); err != nil {
					t.Fatal(err)
				}
				binaryPath := filepath.Join(genesisDir, "pchaind")
				if err := os.WriteFile(binaryPath, []byte("fake binary"), 0o755); err != nil {
					t.Fatal(err)
				}

				// Create upgrade directory
				upgradeDir := filepath.Join(svc.upgradesDir(), "v1.1.0", "bin")
				if err := os.MkdirAll(upgradeDir, 0o755); err != nil {
					t.Fatal(err)
				}
				upgradeBinary := filepath.Join(upgradeDir, "pchaind")
				if err := os.WriteFile(upgradeBinary, []byte("fake upgrade binary"), 0o755); err != nil {
					t.Fatal(err)
				}

				// Create current symlink
				currentLink := svc.currentLink()
				target := filepath.Join(svc.upgradesDir(), "v1.1.0")
				if err := os.Symlink(target, currentLink); err != nil {
					t.Fatal(err)
				}

				// Create another pending upgrade
				upgrade2Dir := filepath.Join(svc.upgradesDir(), "v1.2.0", "bin")
				if err := os.MkdirAll(upgrade2Dir, 0o755); err != nil {
					t.Fatal(err)
				}
				upgrade2Binary := filepath.Join(upgrade2Dir, "pchaind")
				if err := os.WriteFile(upgrade2Binary, []byte("fake upgrade binary 2"), 0o755); err != nil {
					t.Fatal(err)
				}

				return svc
			},
			validate: func(t *testing.T, status *Status) {
				if !status.Installed {
					t.Error("Status.Installed = false, want true")
				}

				// ActiveBinary should point to upgrade
				if !strings.Contains(status.ActiveBinary, "v1.1.0/bin/pchaind") {
					t.Errorf("Status.ActiveBinary = %q, want to contain v1.1.0/bin/pchaind", status.ActiveBinary)
				}

				// Should have pending upgrades
				if len(status.PendingUpgrades) != 2 {
					t.Errorf("len(Status.PendingUpgrades) = %d, want 2", len(status.PendingUpgrades))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			homeDir := t.TempDir()
			svc := tt.setupFunc(homeDir)

			status, err := svc.Status(context.Background())
			if err != nil {
				t.Fatalf("Status() unexpected error: %v", err)
			}

			if status == nil {
				t.Fatal("Status() returned nil")
			}

			tt.validate(t, status)
		})
	}
}

func TestCopyFile(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	srcPath := filepath.Join(srcDir, "source.txt")
	dstPath := filepath.Join(dstDir, "dest.txt")

	content := []byte("test content")
	if err := os.WriteFile(srcPath, content, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := copyFile(srcPath, dstPath); err != nil {
		t.Fatalf("copyFile() error: %v", err)
	}

	// Verify content
	got, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("failed to read destination: %v", err)
	}

	if string(got) != string(content) {
		t.Errorf("copyFile() content = %q, want %q", got, content)
	}
}

func TestCopyFileErrors(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		dst     string
		wantErr bool
	}{
		{
			name:    "source doesn't exist",
			src:     "/nonexistent/source.txt",
			dst:     filepath.Join(t.TempDir(), "dest.txt"),
			wantErr: true,
		},
		{
			name:    "destination directory doesn't exist",
			src:     filepath.Join(t.TempDir(), "source.txt"),
			dst:     "/nonexistent/dir/dest.txt",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create source file if it's supposed to exist
			if tt.name != "source doesn't exist" {
				if err := os.WriteFile(tt.src, []byte("test"), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			err := copyFile(tt.src, tt.dst)
			if (err != nil) != tt.wantErr {
				t.Errorf("copyFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

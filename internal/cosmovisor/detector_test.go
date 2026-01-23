package cosmovisor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetect(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(t *testing.T) (homeDir string, cleanup func())
		validate  func(t *testing.T, result DetectionResult)
	}{
		{
			name: "cosmovisor not available",
			setupFunc: func(t *testing.T) (string, func()) {
				homeDir := t.TempDir()
				// Clear COSMOVISOR env to ensure we don't find it
				oldEnv := os.Getenv("COSMOVISOR")
				os.Unsetenv("COSMOVISOR")
				return homeDir, func() {
					if oldEnv != "" {
						os.Setenv("COSMOVISOR", oldEnv)
					}
				}
			},
			validate: func(t *testing.T, result DetectionResult) {
				// Available depends on whether cosmovisor is in PATH
				// We can only test the behavior when it's not available
				if result.Available {
					// If available, check other fields are correct
					if result.BinaryPath == "" {
						t.Error("Available=true but BinaryPath is empty")
					}
					if result.SetupComplete {
						t.Error("SetupComplete should be false without setup")
					}
				} else {
					if result.BinaryPath != "" {
						t.Errorf("BinaryPath = %q, want empty when not available", result.BinaryPath)
					}
					if result.ShouldUse {
						t.Error("ShouldUse = true, want false when not available")
					}
					if result.Reason != "cosmovisor binary not found in PATH" {
						t.Errorf("Reason = %q, want 'cosmovisor binary not found in PATH'", result.Reason)
					}
				}
			},
		},
		{
			name: "cosmovisor available via env but not setup",
			setupFunc: func(t *testing.T) (string, func()) {
				homeDir := t.TempDir()

				// Create a fake cosmovisor binary
				binDir := t.TempDir()
				fakeBinary := filepath.Join(binDir, "cosmovisor")
				if err := os.WriteFile(fakeBinary, []byte("fake"), 0o755); err != nil {
					t.Fatal(err)
				}

				// Set COSMOVISOR env
				oldEnv := os.Getenv("COSMOVISOR")
				os.Setenv("COSMOVISOR", fakeBinary)

				return homeDir, func() {
					if oldEnv != "" {
						os.Setenv("COSMOVISOR", oldEnv)
					} else {
						os.Unsetenv("COSMOVISOR")
					}
				}
			},
			validate: func(t *testing.T, result DetectionResult) {
				if !result.Available {
					t.Error("Available = false, want true")
				}
				if result.BinaryPath == "" {
					t.Error("BinaryPath is empty, want non-empty")
				}
				if result.SetupComplete {
					t.Error("SetupComplete = true, want false")
				}
				if !result.ShouldUse {
					t.Error("ShouldUse = false, want true")
				}
				if result.Reason != "Cosmovisor is available (will auto-initialize on start)" {
					t.Errorf("Reason = %q, want auto-initialize message", result.Reason)
				}
			},
		},
		{
			name: "cosmovisor available and setup complete",
			setupFunc: func(t *testing.T) (string, func()) {
				homeDir := t.TempDir()

				// Create a fake cosmovisor binary
				binDir := t.TempDir()
				fakeBinary := filepath.Join(binDir, "cosmovisor")
				if err := os.WriteFile(fakeBinary, []byte("fake"), 0o755); err != nil {
					t.Fatal(err)
				}

				// Set COSMOVISOR env
				oldEnv := os.Getenv("COSMOVISOR")
				os.Setenv("COSMOVISOR", fakeBinary)

				// Create genesis binary
				genesisPath := filepath.Join(homeDir, "cosmovisor", "genesis", "bin", "pchaind")
				if err := os.MkdirAll(filepath.Dir(genesisPath), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(genesisPath, []byte("fake pchaind"), 0o755); err != nil {
					t.Fatal(err)
				}

				return homeDir, func() {
					if oldEnv != "" {
						os.Setenv("COSMOVISOR", oldEnv)
					} else {
						os.Unsetenv("COSMOVISOR")
					}
				}
			},
			validate: func(t *testing.T, result DetectionResult) {
				if !result.Available {
					t.Error("Available = false, want true")
				}
				if result.BinaryPath == "" {
					t.Error("BinaryPath is empty, want non-empty")
				}
				if !result.SetupComplete {
					t.Error("SetupComplete = false, want true")
				}
				if !result.ShouldUse {
					t.Error("ShouldUse = false, want true")
				}
				if result.Reason != "Cosmovisor is available and properly configured" {
					t.Errorf("Reason = %q, want properly configured message", result.Reason)
				}
			},
		},
		{
			name: "genesis binary exists but cosmovisor not available",
			setupFunc: func(t *testing.T) (string, func()) {
				homeDir := t.TempDir()

				// Create genesis binary
				genesisPath := filepath.Join(homeDir, "cosmovisor", "genesis", "bin", "pchaind")
				if err := os.MkdirAll(filepath.Dir(genesisPath), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(genesisPath, []byte("fake pchaind"), 0o755); err != nil {
					t.Fatal(err)
				}

				// Clear COSMOVISOR env
				oldEnv := os.Getenv("COSMOVISOR")
				os.Unsetenv("COSMOVISOR")

				return homeDir, func() {
					if oldEnv != "" {
						os.Setenv("COSMOVISOR", oldEnv)
					}
				}
			},
			validate: func(t *testing.T, result DetectionResult) {
				// Available depends on PATH, so we check consistency
				if result.Available {
					if !result.SetupComplete {
						t.Error("SetupComplete = false, want true when genesis exists")
					}
				} else {
					if result.ShouldUse {
						t.Error("ShouldUse = true, want false when not available")
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			homeDir, cleanup := tt.setupFunc(t)
			defer cleanup()

			result := Detect(homeDir)

			tt.validate(t, result)
		})
	}
}

func TestFindCosmovisor(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func() func()
		wantEmpty bool
	}{
		{
			name: "found via COSMOVISOR env",
			setupFunc: func() func() {
				binDir := t.TempDir()
				fakeBinary := filepath.Join(binDir, "cosmovisor")
				if err := os.WriteFile(fakeBinary, []byte("fake"), 0o755); err != nil {
					t.Fatal(err)
				}

				oldEnv := os.Getenv("COSMOVISOR")
				os.Setenv("COSMOVISOR", fakeBinary)

				return func() {
					if oldEnv != "" {
						os.Setenv("COSMOVISOR", oldEnv)
					} else {
						os.Unsetenv("COSMOVISOR")
					}
				}
			},
			wantEmpty: false,
		},
		{
			name: "COSMOVISOR env set but file doesn't exist",
			setupFunc: func() func() {
				oldEnv := os.Getenv("COSMOVISOR")
				os.Setenv("COSMOVISOR", "/nonexistent/cosmovisor")

				return func() {
					if oldEnv != "" {
						os.Setenv("COSMOVISOR", oldEnv)
					} else {
						os.Unsetenv("COSMOVISOR")
					}
				}
			},
			wantEmpty: false, // Falls back to PATH lookup, so may find it
		},
		{
			name: "not found in PATH or env",
			setupFunc: func() func() {
				oldEnv := os.Getenv("COSMOVISOR")
				os.Unsetenv("COSMOVISOR")

				return func() {
					if oldEnv != "" {
						os.Setenv("COSMOVISOR", oldEnv)
					}
				}
			},
			wantEmpty: true, // Unless cosmovisor is actually in PATH
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := tt.setupFunc()
			defer cleanup()

			path := findCosmovisor()

			if tt.wantEmpty && tt.name == "not found in PATH or env" {
				// This case is system-dependent; just log the result
				t.Logf("findCosmovisor() = %q", path)
			} else if !tt.wantEmpty {
				if path == "" {
					t.Error("findCosmovisor() = empty, want non-empty")
				}
			}
		})
	}
}

func TestIsAvailable(t *testing.T) {
	// This test depends on the system state (whether cosmovisor is in PATH)
	// We can test with COSMOVISOR env set
	t.Run("with COSMOVISOR env", func(t *testing.T) {
		binDir := t.TempDir()
		fakeBinary := filepath.Join(binDir, "cosmovisor")
		if err := os.WriteFile(fakeBinary, []byte("fake"), 0o755); err != nil {
			t.Fatal(err)
		}

		oldEnv := os.Getenv("COSMOVISOR")
		os.Setenv("COSMOVISOR", fakeBinary)
		defer func() {
			if oldEnv != "" {
				os.Setenv("COSMOVISOR", oldEnv)
			} else {
				os.Unsetenv("COSMOVISOR")
			}
		}()

		if !IsAvailable() {
			t.Error("IsAvailable() = false, want true with COSMOVISOR env set")
		}
	})

	t.Run("without COSMOVISOR env", func(t *testing.T) {
		oldEnv := os.Getenv("COSMOVISOR")
		os.Unsetenv("COSMOVISOR")
		defer func() {
			if oldEnv != "" {
				os.Setenv("COSMOVISOR", oldEnv)
			}
		}()

		// Result depends on whether cosmovisor is in PATH
		available := IsAvailable()
		// We can't assert a specific value, just verify it returns a bool
		t.Logf("IsAvailable() = %v", available)
	})
}

func TestBinaryPath(t *testing.T) {
	t.Run("with COSMOVISOR env", func(t *testing.T) {
		binDir := t.TempDir()
		fakeBinary := filepath.Join(binDir, "cosmovisor")
		if err := os.WriteFile(fakeBinary, []byte("fake"), 0o755); err != nil {
			t.Fatal(err)
		}

		oldEnv := os.Getenv("COSMOVISOR")
		os.Setenv("COSMOVISOR", fakeBinary)
		defer func() {
			if oldEnv != "" {
				os.Setenv("COSMOVISOR", oldEnv)
			} else {
				os.Unsetenv("COSMOVISOR")
			}
		}()

		path := BinaryPath()
		if path == "" {
			t.Error("BinaryPath() = empty, want non-empty with COSMOVISOR env set")
		}
		if path != fakeBinary {
			t.Errorf("BinaryPath() = %q, want %q", path, fakeBinary)
		}
	})

	t.Run("without COSMOVISOR env", func(t *testing.T) {
		oldEnv := os.Getenv("COSMOVISOR")
		os.Unsetenv("COSMOVISOR")
		defer func() {
			if oldEnv != "" {
				os.Setenv("COSMOVISOR", oldEnv)
			}
		}()

		// Result depends on whether cosmovisor is in PATH
		path := BinaryPath()
		// We can't assert a specific value
		t.Logf("BinaryPath() = %q", path)
	})
}

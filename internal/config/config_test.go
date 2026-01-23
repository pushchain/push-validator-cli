package config

import (
	"os"
	"testing"
)

func TestKeyringBackendDefault(t *testing.T) {
	// Clear env var if set
	os.Unsetenv("PUSH_KEYRING_BACKEND")

	cfg := Defaults()
	if cfg.KeyringBackend != "test" {
		t.Errorf("Expected default KeyringBackend to be 'test', got '%s'", cfg.KeyringBackend)
	}
}

func TestKeyringBackendEnvOverride(t *testing.T) {
	// Set env var
	os.Setenv("PUSH_KEYRING_BACKEND", "os")
	defer os.Unsetenv("PUSH_KEYRING_BACKEND")

	cfg := Defaults()
	if cfg.KeyringBackend != "os" {
		t.Errorf("Expected KeyringBackend to be 'os' from env, got '%s'", cfg.KeyringBackend)
	}
}

func TestWarnIfTestKeyring(t *testing.T) {
	// Reset the warnedOnce flag by creating a new config
	cfg := Config{KeyringBackend: "test"}

	// This should trigger the warning (but we can't easily capture stderr in this test)
	// Just verify it doesn't panic
	cfg.WarnIfTestKeyring()

	// Verify no warning for other backends
	cfg2 := Config{KeyringBackend: "os"}
	cfg2.WarnIfTestKeyring() // Should not print anything
}

func TestDefaults_AllFields(t *testing.T) {
	// Clear env var if set
	os.Unsetenv("PUSH_KEYRING_BACKEND")
	t.Cleanup(func() { os.Unsetenv("PUSH_KEYRING_BACKEND") })

	home, _ := os.UserHomeDir()
	cfg := Defaults()

	// Verify all fields match expected defaults
	if cfg.ChainID != "push_42101-1" {
		t.Errorf("Expected ChainID to be 'push_42101-1', got '%s'", cfg.ChainID)
	}

	expectedHomeDir := home + "/.pchain"
	if cfg.HomeDir != expectedHomeDir {
		t.Errorf("Expected HomeDir to be '%s', got '%s'", expectedHomeDir, cfg.HomeDir)
	}

	if cfg.GenesisDomain != "donut.rpc.push.org" {
		t.Errorf("Expected GenesisDomain to be 'donut.rpc.push.org', got '%s'", cfg.GenesisDomain)
	}

	if cfg.KeyringBackend != "test" {
		t.Errorf("Expected KeyringBackend to be 'test', got '%s'", cfg.KeyringBackend)
	}

	if cfg.SnapshotURL != "https://snapshots.donut.push.org" {
		t.Errorf("Expected SnapshotURL to be 'https://snapshots.donut.push.org', got '%s'", cfg.SnapshotURL)
	}

	if cfg.RPCLocal != "http://127.0.0.1:26657" {
		t.Errorf("Expected RPCLocal to be 'http://127.0.0.1:26657', got '%s'", cfg.RPCLocal)
	}

	if cfg.Denom != "upc" {
		t.Errorf("Expected Denom to be 'upc', got '%s'", cfg.Denom)
	}
}

func TestLoad_DefaultHomeDir(t *testing.T) {
	// Clear env var if set
	os.Unsetenv("HOME_DIR")
	t.Cleanup(func() { os.Unsetenv("HOME_DIR") })

	home, _ := os.UserHomeDir()
	cfg := Load()

	expectedHomeDir := home + "/.pchain"
	if cfg.HomeDir != expectedHomeDir {
		t.Errorf("Expected HomeDir to be '%s', got '%s'", expectedHomeDir, cfg.HomeDir)
	}
}

func TestLoad_HomeDirEnvOverride(t *testing.T) {
	// Set HOME_DIR env var
	customHome := "/custom/home/dir"
	os.Setenv("HOME_DIR", customHome)
	t.Cleanup(func() { os.Unsetenv("HOME_DIR") })

	cfg := Load()

	if cfg.HomeDir != customHome {
		t.Errorf("Expected HomeDir to be '%s', got '%s'", customHome, cfg.HomeDir)
	}
}

func TestRemoteRPCURL(t *testing.T) {
	tests := []struct {
		name          string
		genesisDomain string
		expected      string
	}{
		{
			name:          "Standard domain",
			genesisDomain: "donut.rpc.push.org",
			expected:      "https://donut.rpc.push.org:443",
		},
		{
			name:          "Domain with trailing slash",
			genesisDomain: "donut.rpc.push.org/",
			expected:      "https://donut.rpc.push.org:443",
		},
		{
			name:          "Custom domain",
			genesisDomain: "custom.example.com",
			expected:      "https://custom.example.com:443",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{GenesisDomain: tt.genesisDomain}
			result := cfg.RemoteRPCURL()
			if result != tt.expected {
				t.Errorf("Expected RemoteRPCURL() to return '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestWarnIfTestKeyring_NonTestBackend(t *testing.T) {
	backends := []string{"os", "file", "kwallet", "pass", ""}

	for _, backend := range backends {
		t.Run("backend_"+backend, func(t *testing.T) {
			cfg := Config{KeyringBackend: backend}
			// Should not panic and should not print warning
			cfg.WarnIfTestKeyring()
		})
	}
}

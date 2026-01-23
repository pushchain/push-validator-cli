package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Config holds user/system configuration for the manager.
// File-backed configuration and env/flag merging will be added.
type Config struct {
	ChainID        string
	HomeDir        string
	GenesisDomain  string
	KeyringBackend string
	SnapshotURL    string // Base URL for snapshot downloads
	RPCLocal       string // e.g., http://127.0.0.1:26657
	Denom          string // staking denom (e.g., upc)
}

// Defaults sets chain-specific defaults aligned with current scripts.
func Defaults() Config {
	home, _ := os.UserHomeDir()

	// Default to "test" backend, but allow override via PUSH_KEYRING_BACKEND env var
	keyringBackend := "test"
	if v := os.Getenv("PUSH_KEYRING_BACKEND"); v != "" {
		keyringBackend = v
	}

	return Config{
		ChainID:        "push_42101-1",
		HomeDir:        filepath.Join(home, ".pchain"),
		GenesisDomain:  "donut.rpc.push.org",
		KeyringBackend: keyringBackend,
		SnapshotURL:    "https://snapshots.donut.push.org", // Snapshot download server
		RPCLocal:       "http://127.0.0.1:26657",
		Denom:          "upc",
	}
}

// Load returns default config with HOME_DIR override from environment.
// Use flags for other configuration options.
func Load() Config {
	cfg := Defaults()
	// Only support HOME_DIR env var (common pattern for XDG_* style overrides)
	if v := os.Getenv("HOME_DIR"); v != "" {
		cfg.HomeDir = v
	}
	return cfg
}

// RemoteRPCURL returns the full HTTPS RPC URL derived from GenesisDomain.
func (c Config) RemoteRPCURL() string {
	return "https://" + strings.TrimSuffix(c.GenesisDomain, "/") + ":443"
}

// warnedOnce tracks whether we've already warned about test keyring backend
var warnedOnce sync.Once

// WarnIfTestKeyring prints a warning to stderr if the keyring backend is "test".
// This warning is only shown once per process invocation.
func (c Config) WarnIfTestKeyring() {
	if c.KeyringBackend == "test" {
		warnedOnce.Do(func() {
			fmt.Fprintf(os.Stderr, "Warning: Using \"test\" keyring backend. Keys are stored unencrypted. Set PUSH_KEYRING_BACKEND for production use.\n")
		})
	}
}

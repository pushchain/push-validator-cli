package cosmovisor

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Environment variable constants for Cosmovisor.
const (
	EnvDaemonName            = "DAEMON_NAME"
	EnvDaemonHome            = "DAEMON_HOME"
	EnvAllowDownloadBinaries = "DAEMON_ALLOW_DOWNLOAD_BINARIES"
	EnvRestartAfterUpgrade   = "DAEMON_RESTART_AFTER_UPGRADE"
	EnvUnsafeSkipBackup      = "UNSAFE_SKIP_BACKUP"
)

// Service provides Cosmovisor management functionality.
type Service interface {
	// Init sets up the Cosmovisor directory structure and copies genesis binary.
	Init(ctx context.Context, opts InitOptions) error

	// Status returns the current Cosmovisor status.
	Status(ctx context.Context) (*Status, error)

	// IsSetup returns true if Cosmovisor is properly configured.
	IsSetup() bool

	// CosmovisorBinaryPath returns the path to the cosmovisor binary.
	CosmovisorBinaryPath() string

	// CurrentBinaryPath returns the path to the current active pchaind binary.
	CurrentBinaryPath() string

	// EnvVars returns environment variables for running Cosmovisor.
	EnvVars() map[string]string

	// GenesisDir returns the path to the genesis binary directory.
	GenesisDir() string
}

// InitOptions contains options for Cosmovisor initialization.
type InitOptions struct {
	HomeDir  string            // ~/.pchain
	BinPath  string            // Path to pchaind binary to copy as genesis
	Progress func(msg string)  // Optional progress callback
}

// Status represents the current Cosmovisor status.
type Status struct {
	Installed       bool     `json:"installed"`
	GenesisVersion  string   `json:"genesis_version"`
	CurrentVersion  string   `json:"current_version"`
	PendingUpgrades []string `json:"pending_upgrades,omitempty"`
	ActiveBinary    string   `json:"active_binary"`
}

type service struct {
	homeDir string
	binPath string // cosmovisor binary path
}

// New creates a new Cosmovisor service.
func New(homeDir string) Service {
	return &service{
		homeDir: homeDir,
		binPath: findCosmovisor(),
	}
}

// cosmovisorDir returns the cosmovisor root directory.
func (s *service) cosmovisorDir() string {
	return filepath.Join(s.homeDir, "cosmovisor")
}

// GenesisDir returns the path to the genesis binary directory.
func (s *service) GenesisDir() string {
	return filepath.Join(s.cosmovisorDir(), "genesis", "bin")
}

// upgradesDir returns the upgrades directory.
func (s *service) upgradesDir() string {
	return filepath.Join(s.cosmovisorDir(), "upgrades")
}

// currentLink returns the path to the current symlink.
func (s *service) currentLink() string {
	return filepath.Join(s.cosmovisorDir(), "current")
}

// CosmovisorBinaryPath returns the path to the cosmovisor binary.
func (s *service) CosmovisorBinaryPath() string {
	return s.binPath
}

// CurrentBinaryPath returns the path to the current active pchaind binary.
func (s *service) CurrentBinaryPath() string {
	// Check if current symlink exists
	currentPath := s.currentLink()
	if target, err := os.Readlink(currentPath); err == nil {
		return filepath.Join(target, "bin", "pchaind")
	}
	// Fall back to genesis
	return filepath.Join(s.GenesisDir(), "pchaind")
}

// IsSetup returns true if Cosmovisor is properly configured.
func (s *service) IsSetup() bool {
	genesisPath := filepath.Join(s.GenesisDir(), "pchaind")
	_, err := os.Stat(genesisPath)
	return err == nil
}

// EnvVars returns environment variables for running Cosmovisor.
func (s *service) EnvVars() map[string]string {
	return map[string]string{
		EnvDaemonName:            "pchaind",
		EnvDaemonHome:            s.homeDir,
		EnvAllowDownloadBinaries: "true",
		EnvRestartAfterUpgrade:   "true",
		EnvUnsafeSkipBackup:      "false",
	}
}

// Init sets up the Cosmovisor directory structure and copies genesis binary.
func (s *service) Init(ctx context.Context, opts InitOptions) error {
	if opts.HomeDir == "" {
		opts.HomeDir = s.homeDir
	}

	progress := opts.Progress
	if progress == nil {
		progress = func(string) {}
	}

	// Verify cosmovisor binary exists
	if s.binPath == "" {
		return fmt.Errorf("cosmovisor binary not found in PATH. Install with: go install cosmossdk.io/tools/cosmovisor/cmd/cosmovisor@latest")
	}

	// Verify source binary exists
	if opts.BinPath == "" {
		return fmt.Errorf("pchaind binary path is required")
	}
	if _, err := os.Stat(opts.BinPath); os.IsNotExist(err) {
		return fmt.Errorf("pchaind binary not found at: %s", opts.BinPath)
	}

	// Create directory structure
	progress("Creating Cosmovisor directory structure...")
	if err := os.MkdirAll(s.GenesisDir(), 0o755); err != nil {
		return fmt.Errorf("failed to create genesis directory: %w", err)
	}
	if err := os.MkdirAll(s.upgradesDir(), 0o755); err != nil {
		return fmt.Errorf("failed to create upgrades directory: %w", err)
	}

	// Copy binary to genesis directory
	destPath := filepath.Join(s.GenesisDir(), "pchaind")

	// Check if source and destination are the same file (avoid copying to itself)
	srcAbs, _ := filepath.Abs(opts.BinPath)
	destAbs, _ := filepath.Abs(destPath)
	if srcAbs == destAbs {
		// Binary is already in the right place (install.sh downloaded directly to genesis/bin)
		progress("Genesis binary already in place")
	} else {
		// Always overwrite to ensure latest version
		progress("Copying pchaind to cosmovisor genesis directory...")
		if err := copyFile(opts.BinPath, destPath); err != nil {
			return fmt.Errorf("failed to copy binary: %w", err)
		}
		if err := os.Chmod(destPath, 0o755); err != nil {
			return fmt.Errorf("failed to set binary permissions: %w", err)
		}
		progress(fmt.Sprintf("Binary copied to: %s", destPath))
	}

	// Also copy libwasmvm.dylib if present (required on macOS)
	srcDir := filepath.Dir(opts.BinPath)
	wasmLib := filepath.Join(srcDir, "libwasmvm.dylib")
	if _, err := os.Stat(wasmLib); err == nil {
		destLib := filepath.Join(s.GenesisDir(), "libwasmvm.dylib")
		if err := copyFile(wasmLib, destLib); err != nil {
			// Non-fatal: log but continue
			progress(fmt.Sprintf("Warning: failed to copy libwasmvm.dylib: %v", err))
		} else {
			progress("Copied libwasmvm.dylib to genesis directory")
		}
	}

	return nil
}

// Status returns the current Cosmovisor status.
func (s *service) Status(ctx context.Context) (*Status, error) {
	status := &Status{
		Installed: s.binPath != "",
	}

	if !status.Installed {
		return status, nil
	}

	// Get genesis version
	genesisPath := filepath.Join(s.GenesisDir(), "pchaind")
	if _, err := os.Stat(genesisPath); err == nil {
		status.GenesisVersion = getVersion(genesisPath)
	}

	// Get current version (from symlink or genesis)
	currentPath := s.currentLink()
	if target, err := os.Readlink(currentPath); err == nil {
		currentBin := filepath.Join(target, "bin", "pchaind")
		if _, err := os.Stat(currentBin); err == nil {
			status.CurrentVersion = getVersion(currentBin)
			status.ActiveBinary = currentBin
		}
	} else {
		// No current symlink, use genesis
		status.CurrentVersion = status.GenesisVersion
		status.ActiveBinary = genesisPath
	}

	// List pending upgrades
	entries, err := os.ReadDir(s.upgradesDir())
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				upgradeBin := filepath.Join(s.upgradesDir(), entry.Name(), "bin", "pchaind")
				if _, err := os.Stat(upgradeBin); err == nil {
					status.PendingUpgrades = append(status.PendingUpgrades, entry.Name())
				}
			}
		}
	}

	return status, nil
}

// getVersion runs the binary with "version" command and returns the output.
func getVersion(binPath string) string {
	cmd := exec.Command(binPath, "version")
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

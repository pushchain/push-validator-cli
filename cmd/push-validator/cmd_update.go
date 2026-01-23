package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pushchain/push-validator-cli/internal/config"
	"github.com/pushchain/push-validator-cli/internal/update"
	ui "github.com/pushchain/push-validator-cli/internal/ui"
	"github.com/spf13/cobra"
)

// CLIUpdater abstracts update operations for testability.
type CLIUpdater interface {
	FetchLatestRelease() (*update.Release, error)
	FetchReleaseByTag(tag string) (*update.Release, error)
	Download(asset *update.Asset, progress update.ProgressFunc) ([]byte, error)
	VerifyChecksum(data []byte, release *update.Release, assetName string) error
	ExtractBinary(archiveData []byte) ([]byte, error)
	Install(binaryData []byte) error
	Rollback() error
}

type updateCoreOpts struct {
	checkOnly      bool
	force          bool
	version        string
	skipVerify     bool
	currentVersion string
	binaryPath     string
}

// runUpdateCore contains the core update logic, testable with a mocked CLIUpdater.
func runUpdateCore(updater CLIUpdater, cfg config.Config, opts updateCoreOpts, p ui.Printer, prompter Prompter, output io.Writer, verifyBinary func(string) (string, error)) error {

	// Fetch release (latest or specific version)
	var release *update.Release
	var err error
	if opts.version != "" {
		p.Info(fmt.Sprintf("Fetching release %s...", opts.version))
		release, err = updater.FetchReleaseByTag(opts.version)
	} else {
		p.Info("Checking for updates...")
		release, err = updater.FetchLatestRelease()
	}
	if err != nil {
		return fmt.Errorf("failed to fetch release: %w", err)
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	currentVersion := strings.TrimPrefix(opts.currentVersion, "v")

	// Save result to cache
	updateAvailable := update.IsNewerVersion(opts.currentVersion, release.TagName)
	_ = update.SaveCache(cfg.HomeDir, &update.CacheEntry{
		CheckedAt:       time.Now(),
		LatestVersion:   latestVersion,
		UpdateAvailable: updateAvailable,
	})

	// Check if update needed
	if !opts.force && !update.IsNewerVersion(opts.currentVersion, release.TagName) {
		p.Success(fmt.Sprintf("Already up to date (v%s)", currentVersion))
		return nil
	}

	// Show update info
	fmt.Println()
	p.Info(fmt.Sprintf("Update available: v%s → v%s", currentVersion, latestVersion))

	// Show changelog (first 10 lines)
	if release.Body != "" {
		fmt.Println()
		fmt.Println("Changelog:")
		lines := strings.Split(release.Body, "\n")
		maxLines := 10
		if len(lines) < maxLines {
			maxLines = len(lines)
		}
		for _, line := range lines[:maxLines] {
			fmt.Printf("  %s\n", line)
		}
		if len(lines) > 10 {
			fmt.Printf("  ... (see %s for full changelog)\n", release.HTMLURL)
		}
	}
	fmt.Println()

	// Check only mode
	if opts.checkOnly {
		p.Info("Run 'push-validator update' to install")
		return nil
	}

	// Confirm update (skip if --force or --yes flag)
	if !opts.force && !flagYes {
		response, err := prompter.ReadLine("Update now? [Y/n]: ")
		if err != nil {
			p.Warn("Update cancelled")
			return nil
		}
		response = strings.ToLower(response)
		if response != "" && response != "y" && response != "yes" {
			p.Warn("Update cancelled")
			return nil
		}
	}

	// Find binary for current platform
	asset, err := update.GetAssetForPlatform(release)
	if err != nil {
		return err
	}

	// Download with progress bar
	p.Info(fmt.Sprintf("Downloading %s...", asset.Name))
	bar := ui.NewProgressBar(output, asset.Size)
	archiveData, err := updater.Download(asset, func(downloaded, total int64) {
		bar.Update(downloaded)
	})
	bar.Finish()
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	// Verify checksum
	if !opts.skipVerify {
		p.Info("Verifying checksum...")
		if err := updater.VerifyChecksum(archiveData, release, asset.Name); err != nil {
			return fmt.Errorf("checksum verification failed: %w", err)
		}
		p.Success("Checksum verified")
	} else {
		p.Warn("Skipping checksum verification (not recommended)")
	}

	// Extract binary
	p.Info("Extracting binary...")
	binaryData, err := updater.ExtractBinary(archiveData)
	if err != nil {
		return fmt.Errorf("extraction failed: %w", err)
	}

	// Install
	p.Info("Installing...")
	if err := updater.Install(binaryData); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	// Verify new binary
	p.Info("Verifying installation...")
	if verifyBinary != nil {
		if _, verErr := verifyBinary(opts.binaryPath); verErr != nil {
			p.Warn("Verification failed, rolling back...")
			if rbErr := updater.Rollback(); rbErr != nil {
				return fmt.Errorf("rollback failed: %w (original error: %v)", rbErr, verErr)
			}
			return fmt.Errorf("new binary verification failed, rolled back: %w", verErr)
		}
	}

	fmt.Println()
	p.Success(fmt.Sprintf("Updated to v%s", latestVersion))
	fmt.Println()

	// Check if node is running and suggest restart
	if checkNodeRunningInDir(cfg.HomeDir) {
		p.Info("Node is running. Run 'push-validator restart' to use the new version.")
	}

	return nil
}

func init() {
	var (
		checkOnly  bool
		force      bool
		version    string
		skipVerify bool
	)

	updateCmd := &cobra.Command{
		Use:   "update",
		Short: "Update push-validator to the latest version",
		Long: `Check for and install the latest version of push-validator.

The update command downloads pre-built binaries from GitHub Releases,
verifies the checksum, and replaces the current binary.

Examples:
  push-validator update              # Update to latest version
  push-validator update --check      # Check only, don't install
  push-validator update --force      # Skip confirmation
  push-validator update --version v1.2.0  # Install specific version`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Create updater
			updater, err := update.New(Version)
			if err != nil {
				return fmt.Errorf("failed to initialize updater: %w", err)
			}

			cfg := loadCfg()
			opts := updateCoreOpts{
				checkOnly:      checkOnly,
				force:          force,
				version:        version,
				skipVerify:     skipVerify,
				currentVersion: Version,
				binaryPath:     updater.BinaryPath,
			}

			verifyBinary := func(path string) (string, error) {
				verifyCmd := exec.Command(path, "version")
				var stdout bytes.Buffer
				verifyCmd.Stdout = &stdout
				if err := verifyCmd.Run(); err != nil {
					return "", err
				}
				return strings.TrimSpace(stdout.String()), nil
			}

			return runUpdateCore(updater, cfg, opts, getPrinter(), &ttyPrompter{}, os.Stdout, verifyBinary)
		},
	}

	updateCmd.Flags().BoolVar(&checkOnly, "check", false, "Only check for updates, don't install")
	updateCmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")
	updateCmd.Flags().StringVar(&version, "version", "", "Install specific version (e.g., v1.2.0)")
	updateCmd.Flags().BoolVar(&skipVerify, "no-verify", false, "Skip checksum verification (not recommended)")

	rootCmd.AddCommand(updateCmd)
}

// checkNodeRunningInDir checks if the validator node is currently running
// by looking for PID files in the given home directory.
func checkNodeRunningInDir(homeDir string) bool {
	// Check pchaind PID file
	pidFile := filepath.Join(homeDir, "pchaind.pid")
	if _, err := os.Stat(pidFile); err == nil {
		return true
	}

	// Check cosmovisor PID file
	cosmovisorPid := filepath.Join(homeDir, "cosmovisor.pid")
	if _, err := os.Stat(cosmovisorPid); err == nil {
		return true
	}

	return false
}


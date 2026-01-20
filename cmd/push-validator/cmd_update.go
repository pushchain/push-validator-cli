package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pushchain/push-validator-cli/internal/update"
	ui "github.com/pushchain/push-validator-cli/internal/ui"
	"github.com/spf13/cobra"
)

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
			p := ui.NewPrinter(flagOutput)

			// Create updater
			updater, err := update.NewUpdater(Version)
			if err != nil {
				return fmt.Errorf("failed to initialize updater: %w", err)
			}

			// Fetch release (latest or specific version)
			var release *update.Release
			if version != "" {
				p.Info(fmt.Sprintf("Fetching release %s...", version))
				release, err = update.FetchReleaseByTag(version)
			} else {
				p.Info("Checking for updates...")
				release, err = update.FetchLatestRelease()
			}
			if err != nil {
				return fmt.Errorf("failed to fetch release: %w", err)
			}

			latestVersion := strings.TrimPrefix(release.TagName, "v")
			currentVersion := strings.TrimPrefix(Version, "v")

			// Check if update needed
			if !force && !update.IsNewerVersion(Version, release.TagName) {
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
			if checkOnly {
				p.Info("Run 'push-validator update' to install")
				return nil
			}

			// Confirm update (skip if --force or --yes flag)
			if !force && !flagYes {
				fmt.Print("Update now? [Y/n]: ")
				var response string
				fmt.Scanln(&response)
				response = strings.ToLower(strings.TrimSpace(response))
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

			// Download with progress
			p.Info(fmt.Sprintf("Downloading %s...", asset.Name))
			archiveData, err := updater.Download(asset, func(downloaded, total int64) {
				if total > 0 {
					pct := float64(downloaded) / float64(total) * 100
					fmt.Printf("\r  Downloading... %.1f%%", pct)
				}
			})
			if err != nil {
				return fmt.Errorf("download failed: %w", err)
			}
			fmt.Println() // Clear progress line

			// Verify checksum
			if !skipVerify {
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
			verifyCmd := exec.Command(updater.BinaryPath, "version")
			var stdout bytes.Buffer
			verifyCmd.Stdout = &stdout
			if err := verifyCmd.Run(); err != nil {
				p.Warn("Verification failed, rolling back...")
				if rbErr := updater.Rollback(); rbErr != nil {
					return fmt.Errorf("rollback failed: %w (original error: %v)", rbErr, err)
				}
				return fmt.Errorf("new binary verification failed, rolled back: %w", err)
			}

			fmt.Println()
			p.Success(fmt.Sprintf("Updated to v%s", latestVersion))
			fmt.Println()

			// Check if node is running and suggest restart
			if isNodeRunning() {
				p.Info("Node is running. Run 'push-validator restart' to use the new version.")
			}

			return nil
		},
	}

	updateCmd.Flags().BoolVar(&checkOnly, "check", false, "Only check for updates, don't install")
	updateCmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")
	updateCmd.Flags().StringVar(&version, "version", "", "Install specific version (e.g., v1.2.0)")
	updateCmd.Flags().BoolVar(&skipVerify, "no-verify", false, "Skip checksum verification (not recommended)")

	rootCmd.AddCommand(updateCmd)
}

// isNodeRunning checks if the validator node is currently running
func isNodeRunning() bool {
	cfg := loadCfg()

	// Check pchaind PID file
	pidFile := filepath.Join(cfg.HomeDir, "pchaind.pid")
	if _, err := os.Stat(pidFile); err == nil {
		return true
	}

	// Check cosmovisor PID file
	cosmovisorPid := filepath.Join(cfg.HomeDir, "cosmovisor.pid")
	if _, err := os.Stat(cosmovisorPid); err == nil {
		return true
	}

	return false
}

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pushchain/push-validator-cli/internal/chain"
	"github.com/pushchain/push-validator-cli/internal/ui"
	"github.com/spf13/cobra"
)

func init() {
	var (
		version    string
		force      bool
		skipVerify bool
	)

	chainCmd := &cobra.Command{
		Use:   "chain",
		Short: "Chain binary management commands",
		Long:  `Commands for managing the pchaind chain binary, including downloading and installing.`,
	}

	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Download and install pchaind binary",
		Long: `Download and install the pchaind chain binary from GitHub releases.

The binary is installed to the cosmovisor genesis/bin directory for automatic upgrades.

Examples:
  push-validator chain install              # Install latest version
  push-validator chain install --version v0.0.2  # Install specific version
  push-validator chain install --force      # Force reinstall`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := loadCfg()
			p := ui.NewPrinter(flagOutput)

			installer := chain.NewInstaller(cfg.HomeDir)

			// Fetch release (latest or specific version)
			var release *chain.Release
			var err error
			if version != "" {
				if flagOutput != "json" {
					fmt.Printf("  → Fetching release %s\n", version)
				}
				release, err = chain.FetchReleaseByTag(version)
			} else {
				if flagOutput != "json" {
					fmt.Println("  → Fetching latest release version")
				}
				release, err = chain.FetchLatestRelease()
			}
			if err != nil {
				return fmt.Errorf("failed to fetch release: %w", err)
			}

			// Find binary for current platform
			asset, err := chain.GetAssetForPlatform(release)
			if err != nil {
				return err
			}

			// Check if already installed (unless force)
			cosmovisorBin := filepath.Join(cfg.HomeDir, "cosmovisor", "genesis", "bin", "pchaind")
			if !force {
				if _, err := os.Stat(cosmovisorBin); err == nil {
					// Try to get version
					verCmd := exec.Command(cosmovisorBin, "version")
					if out, err := verCmd.Output(); err == nil {
						installedVer := strings.TrimSpace(string(out))
						releaseVer := strings.TrimPrefix(release.TagName, "v")
						if installedVer == releaseVer {
							p.Success(fmt.Sprintf("pchaind %s already installed", release.TagName))
							return nil
						}
					}
				}
			}

			// Download with progress bar
			if flagOutput != "json" {
				fmt.Printf("  → Downloading pchaind %s for %s\n", release.TagName, getOSArch())
			}
			bar := ui.NewProgressBar(os.Stdout, asset.Size)
			archiveData, err := installer.Download(asset, func(downloaded, total int64) {
				bar.Update(downloaded)
			})
			bar.Finish()
			if err != nil {
				return fmt.Errorf("download failed: %w", err)
			}

			// Verify checksum
			if !skipVerify {
				if flagOutput != "json" {
					fmt.Println("  → Verifying checksum")
				}
				if err := installer.VerifyChecksum(archiveData, release, asset.Name); err != nil {
					return fmt.Errorf("checksum verification failed: %w", err)
				}
				fmt.Printf("  %s Checksum verified\n", p.Colors.Success("✓"))
			}

			// Extract and install
			if flagOutput != "json" {
				fmt.Println("  → Extracting binary")
			}
			pchaindPath, err := installer.ExtractAndInstall(archiveData)
			if err != nil {
				return fmt.Errorf("installation failed: %w", err)
			}

			// Verify installation
			verCmd := exec.Command(pchaindPath, "version")
			var installedVer string
			if out, err := verCmd.Output(); err == nil {
				installedVer = strings.TrimSpace(string(out))
			}

			if installedVer != "" {
				fmt.Printf("  %s Installed pchaind (%s)\n", p.Colors.Success("✓"), installedVer)
			} else {
				fmt.Printf("  %s Installed pchaind %s\n", p.Colors.Success("✓"), release.TagName)
			}

			return nil
		},
	}

	installCmd.Flags().StringVar(&version, "version", "", "Install specific version (e.g., v0.0.2)")
	installCmd.Flags().BoolVar(&force, "force", false, "Force reinstall even if already installed")
	installCmd.Flags().BoolVar(&skipVerify, "no-verify", false, "Skip checksum verification")

	chainCmd.AddCommand(installCmd)
	rootCmd.AddCommand(chainCmd)
}

// getOSArch returns a string like "darwin/arm64"
func getOSArch() string {
	return fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
}

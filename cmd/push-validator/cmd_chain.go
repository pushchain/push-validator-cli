package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pushchain/push-validator-cli/internal/chain"
	"github.com/pushchain/push-validator-cli/internal/config"
	"github.com/pushchain/push-validator-cli/internal/ui"
	"github.com/spf13/cobra"
)

// ChainInstaller abstracts chain binary installation for testability.
type ChainInstaller interface {
	Download(asset *chain.Asset, progress chain.ProgressFunc) ([]byte, error)
	VerifyChecksum(data []byte, release *chain.Release, assetName string) (bool, error)
	ExtractAndInstall(data []byte) (string, error)
}

// ChainReleaseFetcher abstracts release fetching for testability.
type ChainReleaseFetcher interface {
	FetchLatest() (*chain.Release, error)
	FetchByTag(tag string) (*chain.Release, error)
}

type chainInstallOpts struct {
	version    string
	force      bool
	skipVerify bool
}

// prodChainFetcher implements ChainReleaseFetcher using the real chain package.
type prodChainFetcher struct{}

func (f *prodChainFetcher) FetchLatest() (*chain.Release, error)       { return chain.FetchLatestRelease() }
func (f *prodChainFetcher) FetchByTag(tag string) (*chain.Release, error) { return chain.FetchReleaseByTag(tag) }

// runChainInstallCore contains the core chain install logic, testable with mocks.
func runChainInstallCore(cfg config.Config, fetcher ChainReleaseFetcher, installer ChainInstaller, opts chainInstallOpts, verifyBinary func(string) (string, error)) error {
	p := getPrinter()

	// Fetch release (latest or specific version)
	var release *chain.Release
	var err error
	if opts.version != "" {
		if flagOutput != "json" {
			fmt.Printf("  → Fetching release %s\n", opts.version)
		}
		release, err = fetcher.FetchByTag(opts.version)
	} else {
		if flagOutput != "json" {
			fmt.Println("  → Fetching latest release version")
		}
		release, err = fetcher.FetchLatest()
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
	if !opts.force {
		if _, statErr := os.Stat(cosmovisorBin); statErr == nil {
			if verifyBinary != nil {
				if installedVer, verErr := verifyBinary(cosmovisorBin); verErr == nil {
					releaseVer := strings.TrimPrefix(release.TagName, "v")
					if installedVer == releaseVer {
						p.Success(fmt.Sprintf("pchaind %s already installed", release.TagName))
						return nil
					}
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
	if !opts.skipVerify {
		if flagOutput != "json" {
			fmt.Println("  → Verifying checksum")
		}
		verified, err := installer.VerifyChecksum(archiveData, release, asset.Name)
		if err != nil {
			return fmt.Errorf("checksum verification failed: %w", err)
		}
		if verified {
			fmt.Printf("  %s Checksum verified\n", p.Colors.Success(p.Colors.Emoji("✓")))
		} else {
			fmt.Printf("  %s Checksum file not available, skipping verification\n", p.Colors.Warning(p.Colors.Emoji("⚠")))
		}
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
	var installedVer string
	if verifyBinary != nil {
		installedVer, _ = verifyBinary(pchaindPath)
	}

	if installedVer != "" {
		fmt.Printf("  %s Installed pchaind (%s)\n", p.Colors.Success(p.Colors.Emoji("✓")), installedVer)
	} else {
		fmt.Printf("  %s Installed pchaind %s\n", p.Colors.Success(p.Colors.Emoji("✓")), release.TagName)
	}

	return nil
}

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
			installer := chain.NewInstaller(cfg.HomeDir)
			fetcher := &prodChainFetcher{}

			verifyBinary := func(path string) (string, error) {
				verCmd := exec.Command(path, "version")
				verCmd.Stdin = nil
				out, err := verCmd.Output()
				if err != nil {
					return "", err
				}
				return strings.TrimSpace(string(out)), nil
			}

			return runChainInstallCore(cfg, fetcher, installer, chainInstallOpts{
				version:    version,
				force:      force,
				skipVerify: skipVerify,
			}, verifyBinary)
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

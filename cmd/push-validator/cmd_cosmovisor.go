package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/pushchain/push-validator-cli/internal/cosmovisor"
)

var cosmovisorCmd = &cobra.Command{
	Use:   "cosmovisor",
	Short: "Manage Cosmovisor for automatic upgrades",
	Long: `Cosmovisor enables automatic binary upgrades for Push Chain validators.

When Cosmovisor is installed, the start/stop/restart commands will automatically
use it for process management. Cosmovisor will be initialized automatically on
first start if the binary is available.

Subcommands:
  status        Show Cosmovisor status and configuration
  upgrade-info  Generate upgrade JSON with binary checksums`,
}

var cosmovisorStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show Cosmovisor status",
	Long: `Shows the current Cosmovisor status including:
- Whether cosmovisor binary is installed
- Whether Cosmovisor is initialized for this node
- Current and genesis binary versions
- Any pending upgrades`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runCosmovisorStatus,
}

var (
	upgradeInfoVersion string
	upgradeInfoURL     string
	upgradeInfoHeight  int64
)

var cosmovisorUpgradeInfoCmd = &cobra.Command{
	Use:   "upgrade-info",
	Short: "Generate upgrade JSON with checksums",
	Long: `Generates upgrade-info.json content with SHA256 checksums for all supported platforms.

This JSON is used in governance proposals to enable automatic binary upgrades via Cosmovisor.

Supported platforms:
  - linux/amd64
  - linux/arm64
  - darwin/arm64

Example:
  push-validator cosmovisor upgrade-info \
    --version v1.1.0 \
    --url https://github.com/push-protocol/push-chain-node/releases/download/v1.1.0`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runCosmovisorUpgradeInfo,
}

func runCosmovisorStatus(cmd *cobra.Command, args []string) error {
	cfg := loadCfg()
	p := getPrinter()
	c := getPrinter().Colors

	detection := cosmovisor.Detect(cfg.HomeDir)
	svc := cosmovisor.New(cfg.HomeDir)

	ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
	defer cancel()

	status, _ := svc.Status(ctx)

	if flagOutput == "json" {
		p.JSON(map[string]any{
			"available":      detection.Available,
			"binary_path":    detection.BinaryPath,
			"setup_complete": detection.SetupComplete,
			"should_use":     detection.ShouldUse,
			"reason":         detection.Reason,
			"status":         status,
		})
		return nil
	}

	fmt.Println(c.Header(" COSMOVISOR STATUS "))
	fmt.Println()

	// Detection status
	printStatusLine := func(name, value string, ok bool) {
		icon := c.Success("✓")
		if !ok {
			icon = c.Error("✗")
		}
		fmt.Printf("  %s %s: %s\n", icon, c.Apply(c.Theme.Header, name), value)
	}

	binStatus := "not found"
	if detection.Available {
		binStatus = detection.BinaryPath
	}
	printStatusLine("Cosmovisor Binary", binStatus, detection.Available)

	setupStatus := "not initialized"
	if detection.SetupComplete {
		setupStatus = "initialized"
	}
	printStatusLine("Setup Status", setupStatus, detection.SetupComplete)

	useStatus := "no"
	if detection.ShouldUse {
		useStatus = "yes (will auto-initialize if needed)"
	}
	printStatusLine("Will Use Cosmovisor", useStatus, detection.ShouldUse)

	// Version info if available
	if status != nil && status.Installed && detection.SetupComplete {
		fmt.Println()
		fmt.Println(c.SubHeader("Version Information"))

		if status.GenesisVersion != "" {
			fmt.Printf("  Genesis Version:  %s\n", status.GenesisVersion)
		}
		if status.CurrentVersion != "" {
			fmt.Printf("  Current Version:  %s\n", status.CurrentVersion)
		}
		if status.ActiveBinary != "" {
			fmt.Printf("  Active Binary:    %s\n", status.ActiveBinary)
		}

		if len(status.PendingUpgrades) > 0 {
			fmt.Println()
			fmt.Println(c.SubHeader("Pending Upgrades"))
			for _, u := range status.PendingUpgrades {
				fmt.Printf("  - %s\n", u)
			}
		}
	}

	// Help text if not available
	if !detection.Available {
		fmt.Println()
		fmt.Println(c.SubHeader("Installation"))
		fmt.Println("  Install Cosmovisor with:")
		fmt.Printf("  %s\n", c.Apply(c.Theme.Command, "go install cosmossdk.io/tools/cosmovisor/cmd/cosmovisor@latest"))
		fmt.Println()
		fmt.Println("  After installation, Cosmovisor will be used automatically on next 'start'.")
	}

	return nil
}

func runCosmovisorUpgradeInfo(cmd *cobra.Command, args []string) error {
	if upgradeInfoVersion == "" {
		return fmt.Errorf("--version is required")
	}
	if upgradeInfoURL == "" {
		return fmt.Errorf("--url is required")
	}

	p := getPrinter()
	c := getPrinter().Colors

	ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
	defer cancel()

	progress := func(msg string) {
		if flagOutput != "json" && !flagQuiet {
			fmt.Printf("  %s %s\n", c.Apply(c.Theme.Pending, "→"), msg)
		}
	}

	if flagOutput != "json" && !flagQuiet {
		fmt.Println(c.Header(" UPGRADE INFO GENERATOR "))
		fmt.Println()
		fmt.Printf("  Version: %s\n", upgradeInfoVersion)
		fmt.Printf("  URL:     %s\n", upgradeInfoURL)
		fmt.Println()
	}

	info, err := cosmovisor.GenerateUpgradeInfo(ctx, cosmovisor.GenerateUpgradeInfoOptions{
		Version:  upgradeInfoVersion,
		BaseURL:  upgradeInfoURL,
		Height:   upgradeInfoHeight,
		Progress: progress,
	})

	if err != nil {
		if flagOutput == "json" {
			p.JSON(map[string]any{"ok": false, "error": err.Error()})
		} else {
			p.Error(fmt.Sprintf("Failed to generate upgrade info: %v", err))
		}
		return err
	}

	if flagOutput != "json" && !flagQuiet {
		fmt.Println()
		fmt.Println(c.SubHeader("Generated Upgrade Info JSON:"))
		fmt.Println()
	}

	// Output as pretty JSON
	output, _ := json.MarshalIndent(info, "", "  ")
	fmt.Println(string(output))

	if flagOutput != "json" && !flagQuiet {
		fmt.Println()
		p.Success("Upgrade info generated successfully!")
		fmt.Println()
		fmt.Println(c.SubHeader("Next Steps:"))
		fmt.Println("  1. Create an upgrade proposal JSON file with this info")
		fmt.Println("  2. Submit the proposal via governance:")
		fmt.Printf("     %s\n", c.Apply(c.Theme.Command, "pchaind tx gov submit-proposal upgrade-proposal.json ..."))
	}

	return nil
}

func init() {
	// Flags for upgrade-info command
	cosmovisorUpgradeInfoCmd.Flags().StringVar(&upgradeInfoVersion, "version", "", "Upgrade version name (required, e.g., v1.1.0)")
	cosmovisorUpgradeInfoCmd.Flags().StringVar(&upgradeInfoURL, "url", "", "Base URL for binary downloads (required)")
	cosmovisorUpgradeInfoCmd.Flags().Int64Var(&upgradeInfoHeight, "height", 0, "Upgrade height (optional, for proposal)")

	_ = cosmovisorUpgradeInfoCmd.MarkFlagRequired("version")
	_ = cosmovisorUpgradeInfoCmd.MarkFlagRequired("url")

	// Add subcommands to cosmovisor command
	cosmovisorCmd.AddCommand(cosmovisorStatusCmd)
	cosmovisorCmd.AddCommand(cosmovisorUpgradeInfoCmd)

	// Add cosmovisor command to root
	rootCmd.AddCommand(cosmovisorCmd)
}

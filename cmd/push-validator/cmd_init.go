package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pushchain/push-validator-cli/internal/bootstrap"
	ui "github.com/pushchain/push-validator-cli/internal/ui"
)

var (
	initMoniker      string
	initChainID      string
	initSnapshotURL  string
	initSkipSnapshot bool
)

var initNodeCmd = &cobra.Command{
	Use:    "init",
	Short:  "Initialize local node home",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := loadCfg()
		p := getPrinter()
		if initMoniker == "" {
			initMoniker = getenvDefault("MONIKER", "push-validator")
		}
		if initChainID == "" {
			initChainID = cfg.ChainID
		}
		if initSnapshotURL == "" {
			initSnapshotURL = cfg.SnapshotURL
		}

		// Create progress callback that shows init steps
		progressCallback := func(msg string) {
			if flagOutput != "json" {
				fmt.Printf("  → %s\n", msg)
			}
		}

		svc := bootstrap.New()
		if err := svc.Init(cmd.Context(), bootstrap.Options{
			HomeDir:          cfg.HomeDir,
			ChainID:          initChainID,
			Moniker:          initMoniker,
			GenesisDomain:    cfg.GenesisDomain,
			BinPath:          findPchaind(),
			SnapshotURL:      initSnapshotURL,
			Progress:         progressCallback,
			SnapshotProgress: createSnapshotProgressCallback(flagOutput),
			SkipSnapshot:     initSkipSnapshot,
		}); err != nil {
			ui.PrintError(ui.ErrorMessage{
				Problem: "Initialization failed",
				Causes: []string{
					"Network issue fetching genesis or status",
					"Incorrect --genesis-domain or RPC unreachable",
					"pchaind binary missing or not executable",
				},
				Actions: []string{
					"Verify connectivity: curl https://<genesis-domain>/status",
					"Set --genesis-domain to a working RPC host",
					"Ensure pchaind is installed and in PATH or pass --bin",
				},
				Hints: []string{"push-validator validators --output json"},
			})
			return err
		}
		// Only show success message when NOT in scripted mode (--skip-snapshot)
		// install.sh calls with --skip-snapshot and handles its own "Node initialized" message
		if flagOutput != "json" && !initSkipSnapshot {
			p.Success("Initialization complete")
		}
		return nil
	},
}

func init() {
	initNodeCmd.Flags().StringVar(&initMoniker, "moniker", "", "Validator moniker")
	initNodeCmd.Flags().StringVar(&initChainID, "chain-id", "", "Chain ID")
	initNodeCmd.Flags().StringVar(&initSnapshotURL, "snapshot-url", "", "Snapshot download base URL")
	initNodeCmd.Flags().BoolVar(&initSkipSnapshot, "skip-snapshot", false, "Skip snapshot download (for separate step)")
	rootCmd.AddCommand(initNodeCmd)
}

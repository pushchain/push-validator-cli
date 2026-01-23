package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/pushchain/push-validator-cli/internal/cosmovisor"
	"github.com/pushchain/push-validator-cli/internal/process"
	ui "github.com/pushchain/push-validator-cli/internal/ui"
)

var (
	restartBin string
)

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart node",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := loadCfg()
		p := getPrinter()
		if restartBin != "" {
			_ = os.Setenv("PCHAIND", restartBin)
		}

		// Verify cosmovisor is available
		detection := cosmovisor.Detect(cfg.HomeDir)
		if !detection.Available {
			return fmt.Errorf("cosmovisor binary not found; install it or ensure it's in PATH")
		}
		sup := newSupervisor(cfg.HomeDir)

		_, err := sup.Restart(process.StartOpts{HomeDir: cfg.HomeDir, Moniker: os.Getenv("MONIKER"), BinPath: findPchaind()})
		if err != nil {
			ui.PrintError(ui.ErrorMessage{
				Problem: "Failed to restart node",
				Causes: []string{
					"Process could not be stopped cleanly",
					"Start preconditions failed (see start command)",
				},
				Actions: []string{
					"Check logs: push-validator logs",
					"Try: push-validator stop; then start",
				},
			})
			return err
		}
		if flagOutput == "json" {
			p.JSON(map[string]any{"ok": true, "action": "restart", "cosmovisor": true})
		} else {
			p.Success("✓ Node restarted with Cosmovisor")
			fmt.Println()
			fmt.Println(p.Colors.Info("Useful commands:"))
			fmt.Println(p.Colors.Apply(p.Colors.Theme.Command, "  push-validator status"))
			fmt.Println(p.Colors.Apply(p.Colors.Theme.Description, "  (check sync progress)"))
			fmt.Println(p.Colors.Apply(p.Colors.Theme.Command, "  push-validator dashboard"))
			fmt.Println(p.Colors.Apply(p.Colors.Theme.Description, "  (live dashboard)"))
			fmt.Println(p.Colors.Apply(p.Colors.Theme.Command, "  push-validator logs"))
			fmt.Println(p.Colors.Apply(p.Colors.Theme.Description, "  (view logs)"))
		}
		return nil
	},
}

func init() {
	restartCmd.Flags().StringVar(&restartBin, "bin", "", "Path to pchaind binary")
	rootCmd.AddCommand(restartCmd)
}

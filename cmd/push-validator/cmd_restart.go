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
	restartBin          string
	restartNoCosmovisor bool
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

		// Determine which supervisor to use
		var sup process.Supervisor
		useCosmovisor := false

		if !restartNoCosmovisor {
			detection := cosmovisor.Detect(cfg.HomeDir)
			if detection.Available {
				useCosmovisor = true
				sup = process.NewCosmovisor(cfg.HomeDir)
			} else {
				sup = process.New(cfg.HomeDir)
			}
		} else {
			sup = process.New(cfg.HomeDir)
		}

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
			p.JSON(map[string]any{"ok": true, "action": "restart", "cosmovisor": useCosmovisor})
		} else {
			if useCosmovisor {
				p.Success("✓ Node restarted with Cosmovisor")
			} else {
				p.Success("✓ Node restarted")
			}
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
	restartCmd.Flags().BoolVar(&restartNoCosmovisor, "no-cosmovisor", false, "Use direct pchaind instead of Cosmovisor")
	rootCmd.AddCommand(restartCmd)
}

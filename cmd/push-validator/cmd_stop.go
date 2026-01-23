package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pushchain/push-validator-cli/internal/process"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop node",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := loadCfg()
		p := getPrinter()

		sup := process.NewCosmovisor(cfg.HomeDir)
		if err := sup.Stop(); err != nil {
			if flagOutput == "json" {
				p.JSON(map[string]any{"ok": false, "error": err.Error()})
			} else {
				p.Error(fmt.Sprintf("stop error: %v", err))
			}
			return err
		}

		if flagOutput == "json" {
			p.JSON(map[string]any{"ok": true, "action": "stop"})
		} else {
			p.Success("Node stopped")
			fmt.Println()
			fmt.Println(p.Colors.Info("Next steps:"))
			fmt.Println(p.Colors.Apply(p.Colors.Theme.Command, "  push-validator start"))
			fmt.Println(p.Colors.Apply(p.Colors.Theme.Description, "  (start the node)"))
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}

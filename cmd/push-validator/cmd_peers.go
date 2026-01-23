package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/pushchain/push-validator-cli/internal/config"
	"github.com/pushchain/push-validator-cli/internal/node"
	ui "github.com/pushchain/push-validator-cli/internal/ui"
)

// runPeersCore contains the core peers logic, testable with a mocked node client.
func runPeersCore(ctx context.Context, cli node.Client) error {
	plist, err := cli.Peers(ctx)
	if err != nil {
		getPrinter().Error(fmt.Sprintf("peers error: %v", err))
		return err
	}
	c := ui.NewColorConfig()
	headers := []string{"ID", "ADDR"}
	rows := make([][]string, 0, len(plist))
	for _, p := range plist {
		rows = append(rows, []string{p.ID, p.Addr})
	}
	fmt.Println(c.Header(" Connected Peers "))
	fmt.Print(ui.Table(c, headers, rows, []int{40, 0}))
	fmt.Printf("Total Peers: %d\n", len(plist))
	return nil
}

// resolveRPCBase determines the RPC base URL from config.
func resolveRPCBase(cfg config.Config) string {
	if cfg.GenesisDomain != "" {
		return "https://" + strings.TrimSuffix(cfg.GenesisDomain, "/")
	}
	if cfg.RPCLocal != "" {
		return cfg.RPCLocal
	}
	return "http://127.0.0.1:26657"
}

func init() {
	peersCmd := &cobra.Command{
		Use:   "peers",
		Short: "List connected peers (from local or remote RPC)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := loadCfg()
			base := resolveRPCBase(cfg)
			cli := node.New(base)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			return runPeersCore(ctx, cli)
		},
	}
	rootCmd.AddCommand(peersCmd)
}

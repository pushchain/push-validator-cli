package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/pushchain/push-validator-cli/internal/config"
	"github.com/pushchain/push-validator-cli/internal/node"
	"github.com/pushchain/push-validator-cli/internal/process"
)

var internalRefreshHome string

// internalRefreshCmd is a hidden command used by the cron job to refresh peers.
// It is not shown in help output.
var internalRefreshCmd = &cobra.Command{
	Use:    "_internal-refresh-peers",
	Short:  "Internal: refresh peers (used by cron)",
	Hidden: true, // Not shown in help
	RunE: func(cmd *cobra.Command, args []string) error {
		homeDir := internalRefreshHome
		if homeDir == "" {
			homeDir = os.ExpandEnv("$HOME/.pchain")
		}

		// Setup logging
		logPath := filepath.Join(homeDir, "logs", "peer-refresh.log")
		_ = os.MkdirAll(filepath.Dir(logPath), 0o755)

		logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err == nil {
			defer logFile.Close()
		}

		log := func(format string, args ...interface{}) {
			msg := fmt.Sprintf("[%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), fmt.Sprintf(format, args...))
			if logFile != nil {
				_, _ = logFile.WriteString(msg)
			}
		}

		cfg := loadCfgFrom(homeDir)
		remoteURL := cfg.RemoteRPCURL()
		if remoteURL == "" {
			log("ERROR: no remote RPC URL configured")
			return nil // Don't fail cron job
		}

		// Get current peers before refresh
		oldPeers, _ := node.GetCurrentPeers(homeDir)

		// Fetch and update peers
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		count, err := node.RefreshPeersFromRemote(ctx, remoteURL, homeDir, 10)
		if err != nil {
			log("ERROR: failed to refresh peers: %v", err)
			return nil // Don't fail cron job
		}

		// Get new peers after refresh
		newPeers, _ := node.GetCurrentPeers(homeDir)

		// Check if peers changed
		if peersEqual(oldPeers, newPeers) {
			log("peers unchanged (%d peers)", count)
			return nil
		}

		log("peers updated: %d -> %d peers", len(oldPeers), len(newPeers))

		// Restart node if running
		sup := process.New(homeDir)
		if !sup.IsRunning() {
			log("node not running, skip restart")
			return nil
		}

		log("restarting node to apply new peers...")
		if err := sup.Stop(); err != nil {
			log("ERROR: failed to stop node: %v", err)
			return nil
		}

		// Brief pause before restart
		time.Sleep(2 * time.Second)

		_, err = sup.Start(process.StartOpts{
			HomeDir: homeDir,
			BinPath: findPchaind(),
		})
		if err != nil {
			log("ERROR: failed to start node: %v", err)
			return nil
		}

		log("node restarted successfully")
		return nil
	},
}

func init() {
	internalRefreshCmd.Flags().StringVar(&internalRefreshHome, "home", "", "Node home directory")
	rootCmd.AddCommand(internalRefreshCmd)
}

// peersEqual compares two peer slices for equality (order-independent).
func peersEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aSet := make(map[string]bool, len(a))
	for _, p := range a {
		aSet[p] = true
	}
	for _, p := range b {
		if !aSet[p] {
			return false
		}
	}
	return true
}

// loadCfgFrom loads config from a specific home directory.
func loadCfgFrom(homeDir string) config.Config {
	cfg := config.Load()
	cfg.HomeDir = homeDir
	return cfg
}

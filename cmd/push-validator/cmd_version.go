package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/pushchain/push-validator-cli/internal/update"
	ui "github.com/pushchain/push-validator-cli/internal/ui"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version",
	Run: func(cmd *cobra.Command, args []string) {
		switch flagOutput {
		case "json":
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(map[string]string{
				"version":    Version,
				"commit":     Commit,
				"build_date": BuildDate,
			})
		case "yaml":
			data, _ := yaml.Marshal(map[string]string{
				"version":    Version,
				"commit":     Commit,
				"build_date": BuildDate,
			})
			fmt.Println(string(data))
		default:
			fmt.Printf("push-validator %s (%s) built %s\n", Version, Commit, BuildDate)
		}
	},
}

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletion(os.Stdout)
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
		default:
			return fmt.Errorf("unknown shell: %s", args[0])
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(completionCmd)
}

// checkForUpdateBackground performs a non-blocking update check.
// Uses cache to avoid checking more than once per 24 hours.
// Stores result in updateCheckResult global for use by PersistentPostRun.
func checkForUpdateBackground() {
	cfg := loadCfg()

	// Check cache first (avoid network calls if recently checked)
	cache, err := update.LoadCache(cfg.HomeDir)
	if err == nil && update.IsCacheValid(cache) {
		// Use cached result, but re-verify in case version changed (e.g., after update)
		if cache.UpdateAvailable && update.IsNewerVersion(Version, cache.LatestVersion) {
			updateCheckMu.Lock()
			updateCheckResult = &update.CheckResult{
				CurrentVersion:  strings.TrimPrefix(Version, "v"),
				LatestVersion:   cache.LatestVersion,
				UpdateAvailable: true,
			}
			updateCheckMu.Unlock()
		}
		return
	}

	// Perform network check with timeout
	updater, err := update.NewUpdater(Version)
	if err != nil {
		return // Silently fail - don't disrupt user's command
	}

	result, err := updater.Check()
	if err != nil {
		return // Silently fail
	}

	// Save to cache
	_ = update.SaveCache(cfg.HomeDir, &update.CacheEntry{
		CheckedAt:       time.Now(),
		LatestVersion:   result.LatestVersion,
		UpdateAvailable: result.UpdateAvailable,
	})

	// Store result for notification
	if result.UpdateAvailable {
		updateCheckMu.Lock()
		updateCheckResult = result
		updateCheckMu.Unlock()
	}
}

// showUpdateNotification displays an update notification after command completes.
func showUpdateNotification(latestVersion string) {
	// Don't show in JSON/YAML output modes
	if flagOutput == "json" || flagOutput == "yaml" {
		return
	}

	// Don't show in quiet mode
	if flagQuiet {
		return
	}

	c := ui.NewColorConfig()
	c.Enabled = c.Enabled && !flagNoColor

	fmt.Println()
	fmt.Println(c.Warning("─────────────────────────────────────────────────────────────"))
	fmt.Printf(c.Warning("  Update available: %s → %s\n"), Version, latestVersion)
	fmt.Println(c.Info("  Run: push-validator update"))
	fmt.Println(c.Warning("─────────────────────────────────────────────────────────────"))
}

// shouldSkipUpdateCheck returns true for commands where update notifications are disruptive
func shouldSkipUpdateCheck(cmd *cobra.Command) bool {
	cmdName := cmd.Name()
	// Skip for update, help, version commands
	if cmdName == "update" || cmdName == "help" || cmdName == "version" {
		return true
	}
	// Skip for installation-related commands (called by install.sh)
	if cmdName == "init" || cmdName == "snapshot" || cmdName == "chain" ||
		cmdName == "start" || cmdName == "sync" {
		return true
	}
	// Skip for subcommands of chain (e.g., "chain install")
	if cmd.Parent() != nil && cmd.Parent().Name() == "chain" {
		return true
	}
	// Skip for subcommands of snapshot (e.g., "snapshot download")
	if cmd.Parent() != nil && cmd.Parent().Name() == "snapshot" {
		return true
	}
	return false
}

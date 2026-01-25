package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/pushchain/push-validator-cli/internal/config"
	"github.com/pushchain/push-validator-cli/internal/exitcodes"
	ui "github.com/pushchain/push-validator-cli/internal/ui"
	"github.com/pushchain/push-validator-cli/internal/update"
)

// Version information - set via -ldflags during build
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

// rootCmd wires the CLI surface using Cobra. Persistent flags are
// applied to a loaded config in loadCfg(). Subcommands implement the
// actual operations (init, start/stop, sync, status, etc.).
// updateCheckResult stores the result of background update check
var (
	updateCheckResult *update.CheckResult
	updateCheckMu     sync.Mutex
)

var rootCmd = &cobra.Command{
	Use:           "push-validator",
	Short:         "Push Validator",
	Long:          "Manage a Push Chain validator node: init, start, status, sync, and admin tasks.",
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Initialize global UI config from flags after parsing but before command execution
		ui.InitGlobal(ui.Config{
			NoColor:        flagNoColor,
			NoEmoji:        flagNoEmoji,
			Yes:            flagYes,
			NonInteractive: flagNonInteractive,
			Verbose:        flagVerbose,
			Quiet:          flagQuiet,
			Debug:          flagDebug,
		})

		// Set NO_COLOR env so lipgloss and other libraries respect the flag
		if flagNoColor {
			os.Setenv("NO_COLOR", "1")
		}

		// Start background update check (non-blocking)
		// Skip for installation-related commands where notifications are disruptive
		if !shouldSkipUpdateCheck(cmd) {
			// Use fresh check (bypass cache) for status/dashboard commands
			// to ensure immediate notification of new versions
			if shouldForceFreshUpdateCheck(cmd) {
				go checkForUpdateFresh()
			} else {
				go checkForUpdateBackground()
			}
		}
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		// Show update notification if available (after command completes)
		// Skip for installation-related commands where notifications are disruptive
		updateCheckMu.Lock()
		result := updateCheckResult
		updateCheckMu.Unlock()
		if !shouldSkipUpdateCheck(cmd) && result != nil && result.UpdateAvailable {
			showUpdateNotification(result.LatestVersion)
		}
	},
}

var (
	flagHome           string
	flagBin            string
	flagRPC            string
	flagGenesis        string
	flagOutput         string
	flagVerbose        bool
	flagQuiet          bool
	flagDebug          bool
	flagNoColor        bool
	flagNoEmoji        bool
	flagYes            bool
	flagNonInteractive bool
)

func init() {
	// Persistent flags to override defaults
	rootCmd.PersistentFlags().StringVar(&flagHome, "home", "", "Node home directory (overrides env)")
	rootCmd.PersistentFlags().StringVar(&flagBin, "bin", "", "Path to pchaind binary (overrides env)")
	rootCmd.PersistentFlags().StringVar(&flagRPC, "rpc", "", "Local RPC base (http[s]://host:port)")
	rootCmd.PersistentFlags().StringVar(&flagGenesis, "genesis-domain", "", "Genesis RPC domain or URL")
	rootCmd.PersistentFlags().StringVarP(&flagOutput, "output", "o", "text", "Output format: json|yaml|text")
	rootCmd.PersistentFlags().BoolVar(&flagVerbose, "verbose", false, "Verbose output")
	rootCmd.PersistentFlags().BoolVarP(&flagQuiet, "quiet", "q", false, "Quiet mode: minimal output (suppresses extras)")
	rootCmd.PersistentFlags().BoolVarP(&flagDebug, "debug", "d", false, "Debug output: extra diagnostic logs")
	rootCmd.PersistentFlags().BoolVar(&flagNoColor, "no-color", false, "Disable ANSI colors")
	rootCmd.PersistentFlags().BoolVar(&flagNoEmoji, "no-emoji", false, "Disable emoji output")
	rootCmd.PersistentFlags().BoolVarP(&flagYes, "yes", "y", false, "Assume yes for all prompts")
	rootCmd.PersistentFlags().BoolVar(&flagNonInteractive, "non-interactive", false, "Fail instead of prompting")

	// Replace root help to present grouped, example-rich output.
	// Only apply custom help to the root command; subcommands use cobra's default help.
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if cmd != rootCmd {
			// For subcommands, print cobra's default usage (includes flags and descriptions)
			fmt.Fprintln(os.Stdout, cmd.UsageString())
			return
		}
		// Help runs before PersistentPreRun, so manually configure colors
		c := ui.NewColorConfig()
		c.Enabled = c.Enabled && !flagNoColor
		c.EmojiEnabled = c.EmojiEnabled && !flagNoEmoji
		w := os.Stdout

		// Fixed column width for command alignment (longest command + buffer)
		const cmdWidth = 36

		// Header
		fmt.Fprintln(w, c.Header(" Push Validator "))
		fmt.Fprintln(w, c.Description("Manage a Push Chain validator node: init, start, status, sync, and admin tasks."))
		fmt.Fprintln(w, c.Separator(50))
		fmt.Fprintln(w)

		// Usage
		fmt.Fprintln(w, c.SubHeader("USAGE"))
		fmt.Fprintf(w, "  %s <command> [flags]\n", "push-validator")
		fmt.Fprintln(w)

		// Quick Start
		fmt.Fprintln(w, c.SubHeader("Quick Start"))
		fmt.Fprintln(w, c.FormatCommandAligned("start", "Start the node process", cmdWidth))
		fmt.Fprintln(w, c.FormatCommandAligned("status", "Show node/rpc/sync status", cmdWidth))
		fmt.Fprintln(w, c.FormatCommandAligned("dashboard", "Live dashboard with metrics", cmdWidth))
		fmt.Fprintln(w)

		// Operations
		fmt.Fprintln(w, c.SubHeader("Operations"))
		fmt.Fprintln(w, c.FormatCommandAligned("stop", "Stop the node process", cmdWidth))
		fmt.Fprintln(w, c.FormatCommandAligned("restart", "Restart the node process", cmdWidth))
		fmt.Fprintln(w, c.FormatCommandAligned("logs", "Tail node logs", cmdWidth))
		fmt.Fprintln(w)

		// Validator
		fmt.Fprintln(w, c.SubHeader("Validator"))
		fmt.Fprintln(w, c.FormatCommandAligned("validators", "List validators", cmdWidth))
		fmt.Fprintln(w, c.FormatCommandAligned("balance [address]", "Check account balance", cmdWidth))
		fmt.Fprintln(w, c.FormatCommandAligned("register-validator", "Register this node as a validator", cmdWidth))
		fmt.Fprintln(w, c.FormatCommandAligned("increase-stake", "Increase validator stake", cmdWidth))
		fmt.Fprintln(w, c.FormatCommandAligned("unjail", "Restore jailed validator to active status", cmdWidth))
		fmt.Fprintln(w, c.FormatCommandAligned("withdraw-rewards", "Withdraw rewards and commission", cmdWidth))
		fmt.Fprintln(w, c.FormatCommandAligned("restake-rewards", "Withdraw and restake all rewards", cmdWidth))
		fmt.Fprintln(w)

		// Maintenance
		fmt.Fprintln(w, c.SubHeader("Maintenance"))
		fmt.Fprintln(w, c.FormatCommandAligned("backup", "Create config/state backup archive", cmdWidth))
		fmt.Fprintln(w, c.FormatCommandAligned("reset", "Reset chain data (keeps addr book)", cmdWidth))
		fmt.Fprintln(w, c.FormatCommandAligned("full-reset", "Complete reset (deletes ALL data)", cmdWidth))
		fmt.Fprintln(w)

		// Utilities
		fmt.Fprintln(w, c.SubHeader("Utilities"))
		fmt.Fprintln(w, c.FormatCommandAligned("doctor", "Run diagnostic checks", cmdWidth))
		fmt.Fprintln(w, c.FormatCommandAligned("peers", "Show connected peer information", cmdWidth))
		fmt.Fprintln(w)

		// Upgrades
		fmt.Fprintln(w, c.SubHeader("Upgrades"))
		fmt.Fprintln(w, c.FormatCommandAligned("update", "Update push-validator to latest version", cmdWidth))
		fmt.Fprintln(w, c.FormatCommandAligned("cosmovisor status", "Show Cosmovisor status", cmdWidth))
		fmt.Fprintln(w, c.FormatCommandAligned("cosmovisor upgrade-info", "Generate upgrade JSON", cmdWidth))
		fmt.Fprintln(w)
	})

	// status command (uses root --output)
	var statusStrict bool
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show node status",
		RunE: func(cmd *cobra.Command, args []string) error {
			d := newDeps()
			res := computeStatus(d)

			// Strict mode: exit non-zero if issues detected
			if statusStrict && (res.Error != "" || !res.Running || res.CatchingUp || res.Peers == 0) {
				// Still output the status before exiting
				switch flagOutput {
				case "json":
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					_ = enc.Encode(res)
				case "yaml":
					data, _ := yaml.Marshal(res)
					fmt.Println(string(data))
				case "text", "":
					if !flagQuiet {
						printStatusText(res)
					}
				}
				return exitcodes.ValidationErr("node has issues")
			}

			switch flagOutput {
			case "json":
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(res)
			case "yaml":
				data, err := yaml.Marshal(res)
				if err != nil {
					return err
				}
				fmt.Println(string(data))
				return nil
			case "text", "":
				if flagQuiet {
					fmt.Printf("running=%v rpc=%v catching_up=%v height=%d\n", res.Running, res.RPCListening, res.CatchingUp, res.Height)
				} else {
					printStatusText(res)
				}
				return nil
			default:
				return fmt.Errorf("invalid --output: %s (use json|yaml|text)", flagOutput)
			}
		},
	}
	statusCmd.Flags().BoolVar(&statusStrict, "strict", false, "Exit non-zero if node has issues (not running, catching up, no peers, or errors)")
	rootCmd.AddCommand(statusCmd)

	// dashboard - interactive TUI for monitoring
	rootCmd.AddCommand(createDashboardCmd())

	rootCmd.AddCommand(&cobra.Command{Use: "logs", Short: "Tail node logs", RunE: func(cmd *cobra.Command, args []string) error {
		cfg := loadCfg()
		sup := newSupervisor(cfg.HomeDir)
		return handleLogs(sup)
	}})

	rootCmd.AddCommand(&cobra.Command{Use: "reset", Short: "Reset chain data", RunE: func(cmd *cobra.Command, args []string) error {
		cfg := loadCfg()
		sup := newSupervisor(cfg.HomeDir)
		return handleReset(cfg, sup)
	}})
	rootCmd.AddCommand(&cobra.Command{Use: "full-reset", Short: "Complete reset (deletes all keys and data)", RunE: func(cmd *cobra.Command, args []string) error {
		cfg := loadCfg()
		sup := newSupervisor(cfg.HomeDir)
		return handleFullReset(cfg, sup)
	}})
	rootCmd.AddCommand(&cobra.Command{Use: "backup", Short: "Backup config and validator state", RunE: func(cmd *cobra.Command, args []string) error { return handleBackup(newDeps()) }})
	validatorsCmd := &cobra.Command{Use: "validators", Short: "List validators", RunE: func(cmd *cobra.Command, args []string) error {
		return handleValidatorsWithFormat(newDeps(), flagOutput == "json")
	}}
	rootCmd.AddCommand(validatorsCmd)
	var balAddr string
	balanceCmd := &cobra.Command{Use: "balance [address]", Short: "Show balance", Args: cobra.RangeArgs(0, 1), RunE: func(cmd *cobra.Command, args []string) error {
		if balAddr != "" {
			args = []string{balAddr}
		}
		return handleBalance(newDeps(), args)
	}}
	balanceCmd.Flags().StringVar(&balAddr, "address", "", "Account address")
	rootCmd.AddCommand(balanceCmd)
	// register-validator: interactive flow with optional flag overrides
	regCmd := &cobra.Command{Use: "register-validator", Short: "Register this node as validator", RunE: func(cmd *cobra.Command, args []string) error {
		return handleRegisterValidator(newDeps())
	}}
	regCmd.Flags().BoolVar(&flagRegisterCheckOnly, "check-only", false, "Exit after reporting validator registration status")
	rootCmd.AddCommand(regCmd)

	// unjail command
	unjailCmd := &cobra.Command{
		Use:   "unjail",
		Short: "Restore jailed validator to active status",
		Long:  "Unjail a validator that was temporarily jailed for downtime, restoring it to the active validator set",
		RunE: func(cmd *cobra.Command, args []string) error {
			return handleUnjail(newDeps())
		},
	}
	rootCmd.AddCommand(unjailCmd)

	// withdraw-rewards command
	withdrawRewardsCmd := &cobra.Command{
		Use:     "withdraw-rewards",
		Aliases: []string{"withdraw", "claim-rewards"},
		Short:   "Withdraw validator rewards and commission",
		Long:    "Withdraw accumulated delegation rewards and optionally withdraw validator commission",
		RunE: func(cmd *cobra.Command, args []string) error {
			return handleWithdrawRewards(newDeps())
		},
	}
	rootCmd.AddCommand(withdrawRewardsCmd)

	// increase-stake command
	increaseStakeCmd := &cobra.Command{
		Use:   "increase-stake",
		Short: "Increase validator stake",
		Long:  "Delegate additional tokens to increase your validator's stake and voting power",
		RunE: func(cmd *cobra.Command, args []string) error {
			return handleIncreaseStake(newDeps())
		},
	}
	rootCmd.AddCommand(increaseStakeCmd)

	// restake-rewards command
	restakeRewardsCmd := &cobra.Command{
		Use:     "restake-rewards",
		Aliases: []string{"restake"},
		Short:   "Withdraw all rewards and restake them",
		Long:    "Automatically withdraw all rewards (commission and outstanding) and restake them to increase validator power",
		RunE: func(cmd *cobra.Command, args []string) error {
			return handleRestakeRewardsAll(newDeps())
		},
	}
	rootCmd.AddCommand(restakeRewardsCmd)

}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		var se silentErr
		if !errors.As(err, &se) {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(exitcodes.CodeForError(err))
	}
}

// loadCfg reads defaults + env via internal/config.Load() and then
// applies overrides from persistent flags (home, bin, rpc, domain).
func loadCfg() config.Config {
	cfg := config.Load()
	if flagHome != "" {
		cfg.HomeDir = flagHome
	}
	if flagRPC != "" {
		cfg.RPCLocal = flagRPC
	}
	if flagGenesis != "" {
		cfg.GenesisDomain = flagGenesis
	}

	return cfg
}

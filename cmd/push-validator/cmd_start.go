package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/pushchain/push-validator-cli/internal/admin"
	"github.com/pushchain/push-validator-cli/internal/bootstrap"
	"github.com/pushchain/push-validator-cli/internal/config"
	"github.com/pushchain/push-validator-cli/internal/cosmovisor"
	"github.com/pushchain/push-validator-cli/internal/dashboard"
	"github.com/pushchain/push-validator-cli/internal/metrics"
	"github.com/pushchain/push-validator-cli/internal/process"
	"github.com/pushchain/push-validator-cli/internal/snapshot"
	syncmon "github.com/pushchain/push-validator-cli/internal/sync"
	ui "github.com/pushchain/push-validator-cli/internal/ui"
	"github.com/pushchain/push-validator-cli/internal/validator"
)

var (
	startBin          string
	startNoPrompt     bool
	startNoCosmovisor bool
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start node",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := loadCfg()
		p := getPrinter()

		// Check if initialization is needed (genesis.json or validator keys missing)
		genesisPath := filepath.Join(cfg.HomeDir, "config", "genesis.json")
		privValKeyPath := filepath.Join(cfg.HomeDir, "config", "priv_validator_key.json")
		nodeKeyPath := filepath.Join(cfg.HomeDir, "config", "node_key.json")

		// Initialize if genesis OR validator keys are missing
		// (needed for first-time setup and post-full-reset scenarios)
		needsInit := false
		if _, err := os.Stat(genesisPath); os.IsNotExist(err) {
			needsInit = true
		}
		if _, err := os.Stat(privValKeyPath); os.IsNotExist(err) {
			needsInit = true
		}
		if _, err := os.Stat(nodeKeyPath); os.IsNotExist(err) {
			needsInit = true
		}

		if needsInit {
			// Auto-initialize on first start
			if flagOutput != "json" {
				p.Info("Initializing node (first time)...")
				fmt.Println()
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
				ChainID:          cfg.ChainID,
				Moniker:          getenvDefault("MONIKER", "push-validator"),
				GenesisDomain:    cfg.GenesisDomain,
				BinPath:          findPchaind(),
				SnapshotURL:      cfg.SnapshotURL,
				Progress:         progressCallback,
				SnapshotProgress: createSnapshotProgressCallback(flagOutput),
			}); err != nil {
				ui.PrintError(ui.ErrorMessage{
					Problem: "Initialization failed",
					Causes: []string{
						"Network issue fetching genesis or status",
						"Incorrect genesis domain configuration",
						"pchaind binary missing or not executable",
					},
					Actions: []string{
						"Verify connectivity: curl https://<genesis-domain>/status",
						"Check genesis domain in config",
						"Ensure pchaind is installed and in PATH",
					},
				})
				return err
			}

			if flagOutput != "json" {
				fmt.Println()
				p.Success("Initialization complete")
			}
		}

		// Determine which supervisor to use (Cosmovisor or direct pchaind)
		var sup process.Supervisor
		useCosmovisor := false

		if !startNoCosmovisor {
			detection := cosmovisor.Detect(cfg.HomeDir)
			if detection.Available {
				useCosmovisor = true
				if flagOutput != "json" && !detection.SetupComplete {
					p.Info("Initializing Cosmovisor...")
				}
			}
			sup = newSupervisor(cfg.HomeDir)
		} else {
			sup = process.New(cfg.HomeDir)
		}

		// Check if node is already running
		isAlreadyRunning := sup.IsRunning()

		if flagOutput != "json" {
			if isAlreadyRunning {
				if pid, ok := sup.PID(); ok {
					p.Success(fmt.Sprintf("Node is running (PID: %d)", pid))
				} else {
					p.Success("Node is running")
				}
			} else {
				if useCosmovisor {
					fmt.Println("→ Starting node with Cosmovisor...")
				} else {
					fmt.Println("→ Starting node...")
				}
			}
		}

		// Continue with normal start
		if startBin != "" {
			_ = os.Setenv("PCHAIND", startBin)
		}
		_, err := sup.Start(process.StartOpts{HomeDir: cfg.HomeDir, Moniker: os.Getenv("MONIKER"), BinPath: findPchaind()})
		if err != nil {
			ui.PrintError(ui.ErrorMessage{
				Problem: "Failed to start node",
				Causes: []string{
					"Invalid home directory or permissions",
					"pchaind not found or incompatible",
					"Port already in use",
				},
				Actions: []string{
					"Check: ls <home>/config/genesis.json",
					"Confirm pchaind version matches network",
					"Verify ports 26656/26657 are available",
				},
			})
			return err
		}
		if flagOutput == "json" {
			p.JSON(map[string]any{"ok": true, "action": "start", "already_running": isAlreadyRunning, "cosmovisor": useCosmovisor})
		} else {
			if !isAlreadyRunning {
				if useCosmovisor {
					p.Success("Node started with Cosmovisor")
				} else {
					p.Success("Node started successfully")
				}
			}

			// Check validator status and show appropriate next steps (skip if --no-prompt)
			if !startNoPrompt {
				fmt.Println()
				if !handlePostStartFlow(cfg, &p) {
					// If post-start flow fails, just continue (node is already started)
					return nil
				}
			}
		}
		return nil
	},
}

func init() {
	startCmd.Flags().StringVar(&startBin, "bin", "", "Path to pchaind binary")
	startCmd.Flags().BoolVar(&startNoPrompt, "no-prompt", false, "Skip post-start prompts (for use in scripts)")
	startCmd.Flags().BoolVar(&startNoCosmovisor, "no-cosmovisor", false, "Use direct pchaind instead of Cosmovisor")
	rootCmd.AddCommand(startCmd)
}

// handlePostStartFlow manages the post-start flow based on validator status.
// Returns false if an error occurred (non-fatal), true if flow completed successfully.
func handlePostStartFlow(cfg config.Config, p *ui.Printer) bool {
	// First, check if the node is still syncing using comprehensive sync check
	// (same logic as dashboard/status to ensure accuracy)
	fmt.Println(p.Colors.Info("▸ Checking Sync Status"))

	collector := metrics.NewWithoutCPU()
	syncCtx, syncCancel := context.WithTimeout(context.Background(), 5*time.Second)
	snap := collector.Collect(syncCtx, "http://127.0.0.1:26657", cfg.GenesisDomain)
	syncCancel()

	// Consider synced only if:
	// 1. CatchingUp is false AND
	// 2. Local height is within 5 blocks of remote height (or remote height unavailable)
	const syncTolerance = 5
	isSyncing := snap.Chain.CatchingUp ||
		(snap.Chain.RemoteHeight > 0 && snap.Chain.LocalHeight < snap.Chain.RemoteHeight-syncTolerance)

	// DEBUG: Log sync status if verbose
	if flagVerbose {
		fmt.Printf("[DEBUG] Sync Check: CatchingUp=%v, LocalHeight=%d, RemoteHeight=%d, IsSyncing=%v\n",
			snap.Chain.CatchingUp, snap.Chain.LocalHeight, snap.Chain.RemoteHeight, isSyncing)
	}

	if isSyncing {
		// Node is still syncing - wait for sync to complete before validator checks
		fmt.Println(p.Colors.Info("  ▸ Node is syncing with the network..."))
		fmt.Println(p.Colors.Apply(p.Colors.Theme.Description, "    Waiting for sync to complete...\n"))
		fmt.Println(p.Colors.Info("▸ Monitoring Sync Progress"))

		// Wait for sync to complete using sync monitor
		// Use correct supervisor based on whether Cosmovisor is available (determines log path)
		sup := newSupervisor(cfg.HomeDir)
		remoteURL := cfg.RemoteRPCURL()

		// Create reset function for retry logic
		resetFunc := func() error {
			fmt.Println(p.Colors.Info("    Stopping node..."))
			if err := sup.Stop(); err != nil {
				// Ignore stop errors - node might not be running
			}
			time.Sleep(2 * time.Second)

			fmt.Println(p.Colors.Info("    Clearing data..."))
			if err := admin.Reset(admin.ResetOptions{
				HomeDir:      cfg.HomeDir,
				BinPath:      findPchaind(),
				KeepAddrBook: true,
			}); err != nil {
				return fmt.Errorf("reset failed: %w", err)
			}

			// Recreate priv_validator_state.json
			pvs := filepath.Join(cfg.HomeDir, "data", "priv_validator_state.json")
			if err := os.WriteFile(pvs, []byte("{\n  \"height\": \"0\",\n  \"round\": 0,\n  \"step\": 0\n}\n"), 0o644); err != nil {
				return fmt.Errorf("failed to create priv_validator_state.json: %w", err)
			}

			fmt.Println(p.Colors.Info("    Restarting node..."))
			_, err := sup.Start(process.StartOpts{
				HomeDir: cfg.HomeDir,
				Moniker: os.Getenv("MONIKER"),
				BinPath: findPchaind(),
			})
			if err != nil {
				return fmt.Errorf("restart failed: %w", err)
			}
			time.Sleep(5 * time.Second) // Give node time to initialize
			return nil
		}

		// Note: With snapshot download, we don't need state sync reconfigure.
		// The node already has data from the snapshot and just needs to catch up via block sync.

		if err := syncmon.RunWithRetry(context.Background(), syncmon.RetryOptions{
			Options: syncmon.Options{
				LocalRPC:     "http://127.0.0.1:26657",
				RemoteRPC:    remoteURL,
				LogPath:      sup.LogPath(),
				Window:       30,
				Compact:      false,
				Out:          os.Stdout,
				Interval:     120 * time.Millisecond,
				Quiet:        flagQuiet,
				Debug:        flagDebug,
				StuckTimeout: 30 * time.Minute, // Detect stuck sync
			},
			MaxRetries: 3,
			ResetFunc:  resetFunc,
		}); err != nil {
			// Sync failed after retries - show warning and dashboard
			fmt.Println()
			fmt.Println(p.Colors.Warning("  " + p.Colors.Emoji("⚠") + " Sync failed after retries"))
			fmt.Println(p.Colors.Apply(p.Colors.Theme.Description, "    Try: push-validator reset && push-validator start"))
			showDashboardPrompt(cfg, p)
			return false
		}

		// Sync complete - fall through to validator checks
		fmt.Println()
	} else {
		// Node is already synced - show success message
		fmt.Println(p.Colors.Success("  " + p.Colors.Emoji("✓") + " Node is synced"))
	}

	// Node is synced (or sync check failed) - proceed with validator checks
	// Check if already a validator
	v := validator.NewWith(validator.Options{
		BinPath:       findPchaind(),
		HomeDir:       cfg.HomeDir,
		ChainID:       cfg.ChainID,
		Keyring:       cfg.KeyringBackend,
		GenesisDomain: cfg.GenesisDomain,
		Denom:         cfg.Denom,
	})

	// Show status checking message
	fmt.Println(p.Colors.Info("▸ Checking Validator Status"))

	// Give the node a moment to fully initialize RPC after sync check
	time.Sleep(2 * time.Second)

	valResult := checkValidatorRegistration(v, 2)

	if valResult.Error != nil {
		if flagVerbose {
			fmt.Printf("  [DEBUG] IsValidator error: %v\n", valResult.Error)
		}
		fmt.Println(p.Colors.Warning("  " + p.Colors.Emoji("⚠") + " Could not verify validator status (will retry in dashboard)"))
		showDashboardPrompt(cfg, p)
		return false
	}

	decision := computePostStartDecision(valResult, isTerminalInteractive())

	switch decision {
	case actionShowDashboard:
		if valResult.IsValidator {
			fmt.Println(p.Colors.Success("  " + p.Colors.Emoji("✓") + " Registered as validator"))
		}
		showDashboardPrompt(cfg, p)
		return true

	case actionShowSteps:
		fmt.Println(p.Colors.Warning("  " + p.Colors.Emoji("⚠") + " Not registered as validator"))
		fmt.Println()
		fmt.Println("Next steps to register as validator:")
		fmt.Println("1. Get test tokens: https://faucet.push.org")
		fmt.Println("2. Check balance: push-validator balance")
		fmt.Println("3. Register: push-validator register-validator")
		fmt.Println()
		showDashboardPrompt(cfg, p)
		return true

	case actionPromptRegister:
		fmt.Println(p.Colors.Warning("  " + p.Colors.Emoji("⚠") + " Not registered as validator"))
		fmt.Println()
		prompter := &ttyPrompter{}
		response, err := prompter.ReadLine("Register as validator now? (y/N) ")
		if err != nil {
			showDashboardPrompt(cfg, p)
			return false
		}
		response = strings.ToLower(response)

		if response == "y" || response == "yes" {
			fmt.Println()
			if err := handleRegisterValidator(newDeps()); err != nil {
				return false
			}
			fmt.Println()
		} else {
			fmt.Println()
			fmt.Println("Next steps to register as validator:")
			fmt.Println("1. Get test tokens: https://faucet.push.org")
			fmt.Println("2. Check balance: push-validator balance")
			fmt.Println("3. Register: push-validator register-validator")
			fmt.Println()
		}
		showDashboardPrompt(cfg, p)
		return true
	}

	return true
}

// handleDashboard launches the interactive dashboard
func handleDashboard(cfg config.Config) error {
	opts := dashboard.Options{
		Config:          cfg,
		RefreshInterval: 3 * time.Second,
		RPCTimeout:      5 * time.Second,
		NoColor:         flagNoColor,
		NoEmoji:         flagNoEmoji,
		CLIVersion:      Version,
		Debug:           false,
	}
	return runDashboardInteractive(opts)
}

// showDashboardPrompt displays a prompt asking user to press ENTER to launch dashboard.
func showDashboardPrompt(cfg config.Config, p *ui.Printer) {
	showDashboardPromptWith(cfg, p, &ttyPrompter{})
}

// isTerminalInteractive checks if we're running in an interactive terminal
func isTerminalInteractive() bool {
	return isTerminalInteractiveWith(os.Stdin.Fd(), os.Stdout.Fd())
}

// isTerminalInteractiveWith is a testable version that accepts file descriptors.
func isTerminalInteractiveWith(stdinFd, stdoutFd uintptr) bool {
	if !term.IsTerminal(int(stdinFd)) {
		return false
	}
	if !term.IsTerminal(int(stdoutFd)) {
		return false
	}
	return true
}

// valCheckResult holds the result of a validator registration check.
type valCheckResult struct {
	IsValidator bool
	Error       error
}

// checkValidatorRegistration checks if the node is registered as a validator with retries.
func checkValidatorRegistration(v validator.Service, maxRetries int) valCheckResult {
	var isValidator bool
	var err error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		statusCtx, statusCancel := context.WithTimeout(context.Background(), 30*time.Second)
		isValidator, err = v.IsValidator(statusCtx, "")
		statusCancel()

		if err == nil {
			break
		}

		if attempt < maxRetries {
			time.Sleep(2 * time.Second)
		}
	}
	return valCheckResult{IsValidator: isValidator, Error: err}
}

// postStartAction represents what action to take after the post-start checks.
type postStartAction string

const (
	actionShowDashboard   postStartAction = "show_dashboard"
	actionPromptRegister  postStartAction = "prompt_register"
	actionShowSteps       postStartAction = "show_steps"
)

// computePostStartDecision determines what to do based on validator check results and interactivity.
func computePostStartDecision(valResult valCheckResult, isInteractive bool) postStartAction {
	if valResult.Error != nil {
		return actionShowDashboard
	}
	if valResult.IsValidator {
		return actionShowDashboard
	}
	if !isInteractive {
		return actionShowSteps
	}
	return actionPromptRegister
}

// showDashboardPromptWith is a testable version that uses a Prompter.
func showDashboardPromptWith(cfg config.Config, p *ui.Printer, prompter Prompter) {
	fmt.Println()
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║                      DASHBOARD AVAILABLE                      ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("  The node is running in the background.")
	fmt.Println("  Press ENTER to open the interactive dashboard (or Ctrl+C to skip)")
	fmt.Println("  Note: The node will continue running in the background.")
	fmt.Println()
	fmt.Println("─────────────────────────────────────────────────────────────")

	if !prompter.IsInteractive() {
		fmt.Println("  Dashboard is available - run: push-validator dashboard")
		fmt.Println()
		return
	}

	_, err := prompter.ReadLine("Press ENTER to continue to the dashboard... ")
	if err != nil {
		fmt.Println()
		fmt.Println("  Dashboard skipped. Node is running in background.")
		fmt.Println()
		return
	}

	fmt.Println()
	_ = handleDashboard(cfg)

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println(p.Colors.Success(p.Colors.Emoji("✓") + " Dashboard closed. Node is still running in background."))
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()
}

// createSnapshotProgressCallback creates a progress callback for snapshot downloads
// that displays a visual progress bar during download.
func createSnapshotProgressCallback(output string) snapshot.ProgressFunc {
	var bar *ui.ProgressBar
	return func(phase snapshot.ProgressPhase, current, total int64, message string) {
		if output == "json" {
			return
		}
		switch phase {
		case snapshot.PhaseDownload:
			if bar == nil && total > 0 {
				bar = ui.NewProgressBar(os.Stdout, total)
			}
			if bar != nil {
				bar.Update(current)
			}
		case snapshot.PhaseVerify:
			if bar != nil {
				bar.Finish()
				bar = nil
			}
			if message != "" {
				fmt.Printf("  → %s\n", message)
			}
		case snapshot.PhaseExtract:
			if message != "" {
				// Truncate long filenames to fit on one line
				if len(message) > 60 {
					message = message[:57] + "..."
				}
				fmt.Printf("\r  → Extracting: %-60s", message)
			}
		}
	}
}

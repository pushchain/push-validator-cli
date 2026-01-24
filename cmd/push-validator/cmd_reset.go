package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/pushchain/push-validator-cli/internal/admin"
	"github.com/pushchain/push-validator-cli/internal/config"
	"github.com/pushchain/push-validator-cli/internal/process"
	ui "github.com/pushchain/push-validator-cli/internal/ui"
)

// handleReset stops the node (best-effort), clears chain data while
// preserving the address book, and restarts the node. It emits JSON or text depending on --output.
func handleReset(cfg config.Config, sup process.Supervisor, prompters ...Prompter) error {
	var prompter Prompter
	if len(prompters) > 0 {
		prompter = prompters[0]
	} else {
		prompter = &ttyPrompter{}
	}
	return handleResetWith(cfg, sup, prompter,
		func() bool { return term.IsTerminal(int(os.Stdout.Fd())) },
		func(opts admin.ResetOptions) error { return admin.Reset(opts) },
	)
}

// handleResetWith is the testable core of handleReset with injectable dependencies.
func handleResetWith(cfg config.Config, sup process.Supervisor, prompter Prompter, isTTY func() bool, resetFn func(admin.ResetOptions) error) error {
	p := getPrinter()

	// Require confirmation for destructive operation
	if flagOutput != "json" && !flagYes {
		if flagNonInteractive {
			return fmt.Errorf("reset requires confirmation: use --yes to confirm in non-interactive mode")
		}
		fmt.Println(p.Colors.Warning(p.Colors.Emoji("⚠️") + "  This will reset all chain data (address book will be kept)"))
		fmt.Println()
		response, err := prompter.ReadLine("Confirm reset? (y/N): ")
		if err != nil || strings.ToLower(strings.TrimSpace(response)) != "y" {
			fmt.Println(p.Colors.Info("Reset cancelled"))
			return nil
		}
	}

	wasRunning := sup.IsRunning()

	// Stop node first and verify it stopped
	if wasRunning {
		if flagOutput != "json" {
			fmt.Println(p.Colors.Info("Stopping node..."))
		}
		if err := sup.Stop(); err != nil {
			if flagOutput == "json" {
				p.JSON(map[string]any{"ok": false, "error": fmt.Sprintf("failed to stop node: %v", err)})
			} else {
				p.Warn(p.Colors.Emoji("⚠") + fmt.Sprintf(" Could not stop node gracefully: %v", err))
				p.Info("Proceeding with reset (node may need manual cleanup)")
			}
		} else if flagOutput != "json" {
			p.Success("✓ Node stopped")
		}
	}

	showSpinner := flagOutput != "json" && isTTY()
	var (
		spinnerStop   chan struct{}
		spinnerTicker *time.Ticker
	)
	if showSpinner {
		c := ui.NewColorConfig()
		prefix := c.Info("Resetting chain data")
		sp := ui.NewSpinner(os.Stdout, prefix)
		spinnerStop = make(chan struct{})
		spinnerTicker = time.NewTicker(120 * time.Millisecond)
		go func() {
			for {
				select {
				case <-spinnerStop:
					return
				case <-spinnerTicker.C:
					sp.Tick()
				}
			}
		}()
	}

	err := resetFn(admin.ResetOptions{
		HomeDir:      cfg.HomeDir,
		BinPath:      findPchaind(),
		KeepAddrBook: true,
	})

	if showSpinner {
		spinnerTicker.Stop()
		close(spinnerStop)
		fmt.Fprint(os.Stdout, "\r\033[K")
	}

	if err != nil {
		if flagOutput == "json" {
			getPrinter().JSON(map[string]any{"ok": false, "error": err.Error()})
		} else {
			getPrinter().Error(fmt.Sprintf("reset error: %v", err))
		}
		return err
	}

	if flagOutput == "json" {
		getPrinter().JSON(map[string]any{"ok": true, "action": "reset"})
	} else {
		p := getPrinter()
		p.Success("✓ Chain data reset (addr book kept)")
		fmt.Println()
		fmt.Println(p.Colors.Info("Next steps:"))
		fmt.Println(p.Colors.Apply(p.Colors.Theme.Command, "  push-validator start"))
		fmt.Println(p.Colors.Apply(p.Colors.Theme.Description, "  (will resume node from genesis with existing peers)\n"))
	}

	return nil
}

// handleFullReset performs a complete reset, deleting ALL data including validator keys.
// Requires explicit confirmation unless --yes flag is used.
func handleFullReset(cfg config.Config, sup process.Supervisor, prompters ...Prompter) error {
	p := getPrinter()
	var prompter Prompter
	if len(prompters) > 0 {
		prompter = prompters[0]
	} else {
		prompter = &ttyPrompter{}
	}

	// Require confirmation before stopping or modifying anything
	if flagOutput != "json" {
		fmt.Println()
		fmt.Println(p.Colors.Warning(p.Colors.Emoji("⚠️") + "  FULL RESET - This will delete EVERYTHING"))
		fmt.Println()
		fmt.Println("This operation will permanently delete:")
		fmt.Println(p.Colors.Error("  • All blockchain data"))
		fmt.Println(p.Colors.Error("  • Validator consensus keys (priv_validator_key.json)"))
		fmt.Println(p.Colors.Error("  • All keyring accounts and keys"))
		fmt.Println(p.Colors.Error("  • Node identity (node_key.json)"))
		fmt.Println(p.Colors.Error("  • Address book and peer connections"))
		fmt.Println()
		fmt.Println(p.Colors.Warning("This will create a NEW validator identity - you cannot recover the old one!"))
		fmt.Println()

		// Require explicit confirmation
		if !flagYes {
			if flagNonInteractive {
				return fmt.Errorf("full-reset requires confirmation: use --yes to confirm in non-interactive mode")
			}
			response, pErr := prompter.ReadLine("Type 'yes' to confirm full reset: ")
			if pErr != nil || strings.TrimSpace(strings.ToLower(response)) != "yes" {
				fmt.Println(p.Colors.Info("Full reset cancelled"))
				return nil
			}
		}
	}

	// Stop node after confirmation
	if sup.IsRunning() {
		if flagOutput != "json" {
			fmt.Println(p.Colors.Info("Stopping node..."))
		}
		if err := sup.Stop(); err != nil {
			if flagOutput == "json" {
				p.JSON(map[string]any{"ok": false, "error": fmt.Sprintf("failed to stop node: %v", err)})
				return err
			} else {
				p.Warn(p.Colors.Emoji("⚠") + fmt.Sprintf(" Could not stop node: %v", err))
				response, pErr := prompter.ReadLine("Continue with full reset anyway? (y/N): ")
				if pErr != nil || strings.ToLower(strings.TrimSpace(response)) != "y" {
					p.Info("Full reset cancelled")
					return nil
				}
			}
		} else if flagOutput != "json" {
			p.Success("✓ Node stopped")
		}
	}

	// Perform full reset
	err := admin.FullReset(admin.FullResetOptions{
		HomeDir: cfg.HomeDir,
		BinPath: findPchaind(),
	})

	if err != nil {
		if flagOutput == "json" {
			getPrinter().JSON(map[string]any{"ok": false, "error": err.Error()})
		} else {
			getPrinter().Error(fmt.Sprintf("full reset error: %v", err))
		}
		return err
	}

	if flagOutput == "json" {
		getPrinter().JSON(map[string]any{"ok": true, "action": "full-reset"})
	} else {
		p := getPrinter()
		p.Success("✓ Full reset complete")
		fmt.Println()
		fmt.Println(p.Colors.Info("Next steps:"))
		fmt.Println(p.Colors.Apply(p.Colors.Theme.Command, "  push-validator start"))
		fmt.Println(p.Colors.Apply(p.Colors.Theme.Description, "  (will auto-initialize with new validator keys)"))
	}

	return nil
}

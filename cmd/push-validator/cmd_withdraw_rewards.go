package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pushchain/push-validator-cli/internal/dashboard"
)

// handleWithdrawRewards orchestrates the withdraw rewards flow:
// - verify node is synced
// - verify validator is registered
// - display current rewards
// - prompt for key name
// - ask about including commission
// - submit withdraw transaction
// - display results
func handleWithdrawRewards(d *Deps) error {
	if err := checkNodeRunning(d.Sup); err != nil {
		return err
	}

	p := getPrinter()
	cfg := d.Cfg

	// Step 1: Check sync status
	if flagOutput != "json" {
		fmt.Println()
		fmt.Print(p.Colors.Apply(p.Colors.Theme.Prompt, p.Colors.Emoji("🔍")+" Checking node sync status..."))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	stLocal, err1 := d.Node.Status(ctx)
	_, err2 := d.RemoteNode.RemoteStatus(ctx, cfg.RemoteRPCURL())
	cancel()

	if err1 != nil || err2 != nil {
		if flagOutput == "json" {
			getPrinter().JSON(map[string]any{"ok": false, "error": "failed to check sync status"})
		} else {
			fmt.Println()
			fmt.Println(p.Colors.Error(p.Colors.Emoji("❌") + " Failed to check sync status"))
			fmt.Println()
			fmt.Println(p.Colors.Info("Please verify your node is running and properly configured."))
			fmt.Println()
		}
		return fmt.Errorf("failed to check sync status")
	}

	if stLocal.CatchingUp {
		if flagOutput == "json" {
			getPrinter().JSON(map[string]any{"ok": false, "error": "node is still syncing"})
		} else {
			fmt.Println()
			fmt.Println(p.Colors.Warning(p.Colors.Emoji("⚠️") + " Node is still syncing to latest block"))
			fmt.Println()
			fmt.Println(p.Colors.Info("Please wait for sync to complete before withdrawing rewards."))
			fmt.Println(p.Colors.Apply(p.Colors.Theme.Command, "  push-validator sync"))
			fmt.Println()
		}
		return fmt.Errorf("node is still syncing")
	}

	if flagOutput != "json" {
		fmt.Println(" " + p.Colors.Success(p.Colors.Emoji("✓")))
	}

	// Step 2: Check validator registration
	if flagOutput != "json" {
		fmt.Print(p.Colors.Apply(p.Colors.Theme.Prompt, p.Colors.Emoji("🔍")+" Checking validator status..."))
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
	myVal, statusErr := d.Fetcher.GetMyValidator(ctx2, cfg)
	cancel2()

	if statusErr != nil {
		if flagOutput == "json" {
			getPrinter().JSON(map[string]any{"ok": false, "error": "failed to check validator status"})
		} else {
			fmt.Println()
			fmt.Println(p.Colors.Error(p.Colors.Emoji("❌") + " Failed to check validator status"))
			fmt.Println()
		}
		return fmt.Errorf("failed to check validator status")
	}

	if !myVal.IsValidator {
		if flagOutput == "json" {
			getPrinter().JSON(map[string]any{"ok": false, "error": "node is not registered as validator"})
		} else {
			fmt.Println()
			fmt.Println(p.Colors.Warning(p.Colors.Emoji("⚠️") + " This node is not registered as a validator"))
			fmt.Println()
			fmt.Println(p.Colors.Info("Register first using:"))
			fmt.Println(p.Colors.Apply(p.Colors.Theme.Command, "  push-validator register-validator"))
			fmt.Println()
		}
		return fmt.Errorf("node is not registered as validator")
	}

	if flagOutput != "json" {
		fmt.Println(" " + p.Colors.Success(p.Colors.Emoji("✓")))
	}

	// Step 3: Display current rewards
	if flagOutput != "json" {
		fmt.Print(p.Colors.Apply(p.Colors.Theme.Prompt, p.Colors.Emoji("💰")+" Fetching current rewards..."))
	}

	ctx3, cancel3 := context.WithTimeout(context.Background(), 5*time.Second)
	commission, outstanding, rewardsErr := d.Fetcher.GetRewards(ctx3, cfg, myVal.Address)
	cancel3()

	if flagOutput != "json" {
		fmt.Println(" " + p.Colors.Success(p.Colors.Emoji("✓")))
	}

	if flagOutput == "json" {
		getPrinter().JSON(map[string]any{
			"ok":                  true,
			"commission_rewards":  commission,
			"outstanding_rewards": outstanding,
		})
		return nil
	}

	// Display rewards summary and validate
	fmt.Println()
	p.Header("Current Rewards")
	if rewardsErr == nil {
		p.KeyValueLine("Commission Rewards", dashboard.FormatSmartNumber(commission)+" PC", "green")
		p.KeyValueLine("Outstanding Rewards", dashboard.FormatSmartNumber(outstanding)+" PC", "green")
	} else {
		fmt.Println(p.Colors.Warning(p.Colors.Emoji("⚠️") + " Could not fetch rewards, but proceeding with withdrawal"))
	}
	fmt.Println()

	// Parse rewards to check if any are available
	commissionFloat, _ := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(commission, "PC")), 64)
	outstandingFloat, _ := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(outstanding, "PC")), 64)
	const rewardThreshold = 0.01 // Minimum 0.01 PC to be worthwhile
	hasSignificantRewards := commissionFloat >= rewardThreshold || outstandingFloat >= rewardThreshold

	// Warn if rewards are minimal
	if !hasSignificantRewards && rewardsErr == nil {
		fmt.Println(p.Colors.Warning(p.Colors.Emoji("⚠️") + " No significant rewards available (less than 0.01 PC)"))
		if d.Prompter.IsInteractive() {
			input, err := d.Prompter.ReadLine("Continue with withdrawal anyway? (y/N): ")
			if err != nil {
				fmt.Println()
				fmt.Println(p.Colors.Info("Withdrawal cancelled."))
				fmt.Println()
				return nil
			}
			input = strings.ToLower(input)
			if input != "y" && input != "yes" {
				fmt.Println()
				fmt.Println(p.Colors.Info("Withdrawal cancelled."))
				fmt.Println()
				return nil
			}
			fmt.Println()
		} else {
			// Non-interactive: abort if no rewards
			fmt.Println(p.Colors.Error(p.Colors.Emoji("❌") + " No significant rewards to withdraw. Aborting."))
			fmt.Println()
			return fmt.Errorf("no significant rewards to withdraw")
		}
	}

	// Step 4: Auto-detect key name from validator
	defaultKeyName := getenvDefault("KEY_NAME", "validator-key")
	var keyName string

	// Try to auto-derive the key name from the validator's address
	if myVal.Address != "" {
		// Convert validator address to account address
		addrCtx, addrCancel := context.WithTimeout(context.Background(), 10*time.Second)
		accountAddr, convErr := convertValidatorToAccountAddress(addrCtx, myVal.Address, d.Runner)
		addrCancel()
		if convErr == nil {
			// Try to find the key in the keyring
			keyCtx, keyCancel := context.WithTimeout(context.Background(), 10*time.Second)
			foundKey, findErr := findKeyNameByAddress(keyCtx, cfg, accountAddr, d.Runner)
			keyCancel()
			if findErr == nil {
				keyName = foundKey
				if flagOutput != "json" {
					fmt.Println()
					fmt.Printf("%s Using key: %s\n", p.Colors.Emoji("🔑"), keyName)
				}
			} else {
				// Fall back to default if key not found
				keyName = defaultKeyName
			}
		} else {
			// Fall back to default if address conversion failed
			keyName = defaultKeyName
		}
	} else {
		keyName = defaultKeyName
	}

	// Only prompt if explicitly requested via env or interactive mode AND key derivation failed
	if flagOutput != "json" && d.Prompter.IsInteractive() && keyName == defaultKeyName && getenvDefault("KEY_NAME", "") == "" {
		input, err := d.Prompter.ReadLine(fmt.Sprintf("\nEnter key name for withdrawal [%s]: ", defaultKeyName))
		if err == nil && input != "" {
			keyName = input
		}
		fmt.Println()
	}

	// Step 5: Check balance for gas fees
	if flagOutput != "json" {
		fmt.Print(p.Colors.Apply(p.Colors.Theme.Prompt, p.Colors.Emoji("💰")+" Checking wallet balance for gas fees..."))
	}

	// Convert validator address to account address for balance check
	balAddrCtx, balAddrCancel := context.WithTimeout(context.Background(), 10*time.Second)
	accountAddr, addrErr := convertValidatorToAccountAddress(balAddrCtx, myVal.Address, d.Runner)
	balAddrCancel()
	if addrErr != nil {
		if flagOutput == "json" {
			getPrinter().JSON(map[string]any{"ok": false, "error": "failed to derive account address"})
		} else {
			fmt.Println()
			fmt.Println(p.Colors.Error(p.Colors.Emoji("❌") + " Failed to derive account address"))
			fmt.Println()
		}
		return fmt.Errorf("failed to derive account address")
	}

	// Get EVM address for display
	evmCtx2, evmCancel2 := context.WithTimeout(context.Background(), 10*time.Second)
	evmAddr, evmErr := getEVMAddress(evmCtx2, accountAddr, d.Runner)
	evmCancel2()
	if evmErr != nil {
		evmAddr = "" // Not critical, we can proceed without EVM address
	}

	if flagOutput != "json" {
		fmt.Println(" " + p.Colors.Success(p.Colors.Emoji("✓")))
	}

	// Wait for sufficient balance (only in interactive mode)
	if flagOutput != "json" && !flagNonInteractive {
		const requiredForGasFees = "150000000000000000" // 0.15 PC in micro-units, enough for gas (actual: ~0.1037 PC + 1.45x buffer)
		if !waitForSufficientBalance(cfg, accountAddr, evmAddr, requiredForGasFees, "withdraw") {
			return fmt.Errorf("insufficient balance for gas fees")
		}
	}

	// Step 7: Ask about commission
	var includeCommission bool
	if d.Prompter.IsInteractive() {
		input, err := d.Prompter.ReadLine("Include commission rewards in withdrawal? (y/n) [n]: ")
		if err == nil {
			input = strings.ToLower(input)
			includeCommission = input == "y" || input == "yes"
		}
		fmt.Println()
	}

	// Step 8: Submit withdraw rewards transaction
	if flagOutput != "json" {
		fmt.Print(p.Colors.Apply(p.Colors.Theme.Prompt, p.Colors.Emoji("📤")+" Submitting withdrawal transaction..."))
	}

	ctx5, cancel5 := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel5()

	txHash, err := d.Validator.WithdrawRewards(ctx5, myVal.Address, keyName, includeCommission)
	if err != nil {
		if flagOutput == "json" {
			getPrinter().JSON(map[string]any{"ok": false, "error": err.Error()})
		} else {
			fmt.Println()
			fmt.Println(p.Colors.Error(p.Colors.Emoji("❌") + " Withdrawal transaction failed"))
			fmt.Println()
			fmt.Printf("Error: %v\n", err)
			fmt.Println()
		}
		return fmt.Errorf("withdrawal transaction failed: %w", err)
	}

	if flagOutput != "json" {
		fmt.Println(" " + p.Colors.Success(p.Colors.Emoji("✓")))
	}

	// Success output
	if flagOutput == "json" {
		getPrinter().JSON(map[string]any{"ok": true, "txhash": txHash})
	} else {
		fmt.Println()
		p.Success(p.Colors.Emoji("✅") + " Rewards successfully withdrawn!")
		fmt.Println()

		// Display transaction hash
		p.KeyValueLine("Transaction Hash", txHash, "green")
		fmt.Println()

		// Show helpful next steps
		fmt.Println(p.Colors.SubHeader("Next Steps"))
		fmt.Println(p.Colors.Separator(40))
		fmt.Println()
		fmt.Println(p.Colors.Info("  1. Check validator status:"))
		fmt.Println(p.Colors.Apply(p.Colors.Theme.Command, "     push-validator validators"))
		fmt.Println()
		fmt.Println(p.Colors.Info("  2. View account balance:"))
		fmt.Println(p.Colors.Apply(p.Colors.Theme.Command, "     push-validator balance"))
		fmt.Println()
		fmt.Println(p.Colors.Info("  3. Live dashboard:"))
		fmt.Println(p.Colors.Apply(p.Colors.Theme.Command, "     push-validator dashboard"))
		fmt.Println()
		fmt.Println(p.Colors.Apply(p.Colors.Theme.Description, "  Your rewards have been transferred to your account."))
		fmt.Println()
	}
	return nil
}

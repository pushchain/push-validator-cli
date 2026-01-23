package main

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/pushchain/push-validator-cli/internal/dashboard"
	"github.com/pushchain/push-validator-cli/internal/validator"
)

// handleRestakeRewardsAll orchestrates the restake-rewards-all flow:
// - verify node is synced
// - verify validator is registered
// - display current rewards
// - automatically withdraw all rewards (commission + outstanding)
// - ask for confirmation to restake rewards with edit/cancel options
// - submit delegation transaction
// - display results
func handleRestakeRewardsAll(d *Deps) error {
	p := getPrinter()
	cfg := d.Cfg

	if flagOutput != "json" {
		fmt.Println()
		p.Header("Push Validator Manager - Restake Rewards")
		fmt.Println()
	}

	// Step 1: Check sync status
	if flagOutput != "json" {
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
			fmt.Println(p.Colors.Info("Please wait for sync to complete before restaking rewards."))
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
		return fmt.Errorf("failed to check validator status: %w", statusErr)
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

	// Step 3: Fetch current rewards
	if flagOutput != "json" {
		fmt.Print(p.Colors.Apply(p.Colors.Theme.Prompt, p.Colors.Emoji("💰")+" Fetching current rewards..."))
	}

	ctx3, cancel3 := context.WithTimeout(context.Background(), 5*time.Second)
	commission, outstanding, rewardsErr := d.Fetcher.GetRewards(ctx3, cfg, myVal.Address)
	cancel3()

	if flagOutput != "json" {
		fmt.Println(" " + p.Colors.Success(p.Colors.Emoji("✓")))
	}

	if rewardsErr != nil {
		if flagOutput == "json" {
			getPrinter().JSON(map[string]any{"ok": false, "error": "failed to fetch rewards"})
		} else {
			fmt.Println()
			fmt.Println(p.Colors.Error(p.Colors.Emoji("❌") + " Failed to fetch rewards"))
			fmt.Println()
			fmt.Printf("Error: %v\n", rewardsErr)
			fmt.Println()
		}
		return fmt.Errorf("failed to fetch rewards: %w", rewardsErr)
	}

	// Display rewards summary
	if flagOutput != "json" {
		fmt.Println()
		p.Section("Current Rewards")
		p.KeyValueLine("Commission Rewards", dashboard.FormatSmartNumber(commission)+" PC", "green")
		p.KeyValueLine("Outstanding Rewards", dashboard.FormatSmartNumber(outstanding)+" PC", "green")
		fmt.Println()
	}

	// Parse rewards to check if any are available
	commissionFloat, _ := strconv.ParseFloat(strings.TrimSpace(commission), 64)
	outstandingFloat, _ := strconv.ParseFloat(strings.TrimSpace(outstanding), 64)
	totalRewards := commissionFloat + outstandingFloat
	const rewardThreshold = 0.01 // Minimum 0.01 PC to be worthwhile

	if totalRewards < rewardThreshold {
		if flagOutput == "json" {
			getPrinter().JSON(map[string]any{"ok": false, "error": "no significant rewards available"})
		} else {
			fmt.Println(p.Colors.Warning(p.Colors.Emoji("⚠️") + " No significant rewards available (less than 0.01 PC)"))
			fmt.Println()
			fmt.Println(p.Colors.Info("Nothing to restake. Continue earning rewards and try again later."))
			fmt.Println()
		}
		return fmt.Errorf("no significant rewards available")
	}

	// Step 4: Auto-detect key name from validator
	defaultKeyName := getenvDefault("KEY_NAME", "validator-key")
	var keyName string

	if myVal.Address != "" {
		ctx4, cancel4 := context.WithTimeout(context.Background(), 5*time.Second)
		accountAddr, convErr := convertValidatorToAccountAddress(ctx4, myVal.Address, d.Runner)
		cancel4()
		if convErr == nil {
			ctx4b, cancel4b := context.WithTimeout(context.Background(), 5*time.Second)
			foundKey, findErr := findKeyNameByAddress(ctx4b, cfg, accountAddr, d.Runner)
			cancel4b()
			if findErr == nil {
				keyName = foundKey
				if flagOutput != "json" {
					fmt.Printf("%s Using key: %s\n", p.Colors.Emoji("🔑"), keyName)
					fmt.Println()
				}
			} else {
				keyName = defaultKeyName
			}
		} else {
			keyName = defaultKeyName
		}
	} else {
		keyName = defaultKeyName
	}

	// Step 5: Submit withdraw rewards transaction (always include commission for restaking)
	if flagOutput != "json" {
		fmt.Print(p.Colors.Apply(p.Colors.Theme.Prompt, "💸 Withdrawing all rewards..."))
	}

	ctx5, cancel5 := context.WithTimeout(context.Background(), 90*time.Second)
	txHash, withdrawErr := d.Validator.WithdrawRewards(ctx5, myVal.Address, keyName, true) // Always include commission
	cancel5()

	if withdrawErr != nil {
		if flagOutput == "json" {
			getPrinter().JSON(map[string]any{"ok": false, "error": withdrawErr.Error(), "step": "withdraw"})
		} else {
			fmt.Println()
			fmt.Println(p.Colors.Error(p.Colors.Emoji("❌") + " Withdrawal transaction failed"))
			fmt.Println()
			fmt.Printf("Error: %v\n", withdrawErr)
			fmt.Println()
		}
		return fmt.Errorf("withdrawal transaction failed: %w", withdrawErr)
	}

	if flagOutput != "json" {
		fmt.Println(" " + p.Colors.Success(p.Colors.Emoji("✓")))
		fmt.Println()
		p.KeyValueLine("Transaction Hash", txHash, "green")
		fmt.Printf(p.Colors.Success(p.Colors.Emoji("✓") + " Successfully withdrew %.6f PC\n"), totalRewards)
		fmt.Println()
	}

	// Step 6: Calculate available amount for restaking
	const feeReserve = 0.15 // Reserve 0.15 PC for gas fees
	maxRestakeable := totalRewards - feeReserve

	if maxRestakeable <= 0 {
		if flagOutput == "json" {
			getPrinter().JSON(map[string]any{
				"ok":              true,
				"withdraw_txhash": txHash,
				"withdrawn":       fmt.Sprintf("%.6f", totalRewards),
				"restaked":        "0",
				"message":         "insufficient balance for restaking after gas reserve",
			})
		} else {
			fmt.Println(p.Colors.Warning(p.Colors.Emoji("⚠️") + " Insufficient balance for restaking after gas reserve"))
			fmt.Println()
			fmt.Println("Funds have been withdrawn to your wallet but are too small to restake.")
			fmt.Println()
		}
		return fmt.Errorf("insufficient balance for restaking after gas reserve")
	}

	// Step 7: Display restaking options
	if flagOutput != "json" {
		p.Section("Available for Restaking")
		p.KeyValueLine("Withdrawn Amount", dashboard.FormatSmartNumber(fmt.Sprintf("%.6f", totalRewards))+" PC", "blue")
		p.KeyValueLine("Gas Reserve", dashboard.FormatSmartNumber(fmt.Sprintf("%.2f", feeReserve))+" PC", "dim")
		p.KeyValueLine("Available to Stake", dashboard.FormatSmartNumber(fmt.Sprintf("%.6f", maxRestakeable))+" PC", "blue")
		fmt.Println()
	}

	// Step 8: Interactive confirmation with edit/cancel option
	restakeAmount := maxRestakeable
	restakeAmountWei := ""

	if d.Prompter.IsInteractive() && !flagYes && flagOutput != "json" {
		for {
			input, err := d.Prompter.ReadLine(fmt.Sprintf("Restake %.6f PC? (y/n/edit) [y]: ", restakeAmount))
			if err != nil {
				break // treat read error as confirm
			}
			input = strings.ToLower(input)

			if input == "" || input == "y" || input == "yes" {
				break
			} else if input == "n" || input == "no" {
				fmt.Println()
				fmt.Println(p.Colors.Info("Restaking cancelled. Funds remain in your wallet."))
				fmt.Println()
				return nil
			} else if input == "edit" || input == "e" {
				fmt.Println()
				for {
					amountInput, amtErr := d.Prompter.ReadLine(fmt.Sprintf("Enter amount to restake (0.01 - %.6f PC): ", maxRestakeable))
					if amtErr != nil {
						break
					}

					if amountInput == "" {
						fmt.Println(p.Colors.Error(p.Colors.Emoji("⚠") + " Amount is required. Try again."))
						continue
					}

					customAmount, parseErr := strconv.ParseFloat(amountInput, 64)
					if parseErr != nil {
						fmt.Println(p.Colors.Error(p.Colors.Emoji("⚠") + " Invalid amount. Enter a number. Try again."))
						continue
					}

					if customAmount < 0.01 {
						fmt.Println(p.Colors.Error(p.Colors.Emoji("⚠") + " Amount too low. Minimum restake is 0.01 PC. Try again."))
						continue
					}
					if customAmount > maxRestakeable {
						fmt.Printf(p.Colors.Error(p.Colors.Emoji("⚠")+" Insufficient balance. Maximum: %.6f PC. Try again.\n"), maxRestakeable)
						continue
					}

					restakeAmount = customAmount
					fmt.Printf(p.Colors.Success(p.Colors.Emoji("✓")+" Will restake %.6f PC\n"), restakeAmount)
					fmt.Println()
					break
				}
				break
			} else {
				fmt.Println()
				fmt.Println(p.Colors.Info("Invalid input. Restaking cancelled."))
				fmt.Println()
				return fmt.Errorf("restaking cancelled by user")
			}
		}
	}

	// Convert to wei
	restakeWei := new(big.Float).Mul(new(big.Float).SetFloat64(restakeAmount), new(big.Float).SetFloat64(1e18))
	restakeAmountWei = restakeWei.Text('f', 0)

	// Step 9: Submit delegation transaction
	if flagOutput != "json" {
		fmt.Print(p.Colors.Apply(p.Colors.Theme.Prompt, p.Colors.Emoji("📤")+" Restaking funds..."))
	}

	ctx6, cancel6 := context.WithTimeout(context.Background(), 90*time.Second)
	delegateTxHash, delegateErr := d.Validator.Delegate(ctx6, validator.DelegateArgs{
		ValidatorAddress: myVal.Address,
		Amount:           restakeAmountWei,
		KeyName:          keyName,
	})
	cancel6()

	if delegateErr != nil {
		if flagOutput == "json" {
			getPrinter().JSON(map[string]any{
				"ok":              false,
				"withdraw_txhash": txHash,
				"withdrawn":       fmt.Sprintf("%.6f", totalRewards),
				"restake_error":   delegateErr.Error(),
				"step":            "restake",
			})
		} else {
			fmt.Println()
			fmt.Println(p.Colors.Error(p.Colors.Emoji("❌") + " Restaking transaction failed"))
			fmt.Println()
			fmt.Printf("Error: %v\n", delegateErr)
			fmt.Println()
			fmt.Println(p.Colors.Warning("Note: Rewards were successfully withdrawn. Funds are in your wallet."))
			fmt.Println(p.Colors.Info("You can manually delegate using: push-validator increase-stake"))
			fmt.Println()
		}
		return fmt.Errorf("restaking transaction failed: %w", delegateErr)
	}

	if flagOutput != "json" {
		fmt.Println(" " + p.Colors.Success(p.Colors.Emoji("✓")))
	}

	// Success output
	if flagOutput == "json" {
		getPrinter().JSON(map[string]any{
			"ok":                true,
			"withdraw_txhash":   txHash,
			"restake_txhash":    delegateTxHash,
			"withdrawn":         fmt.Sprintf("%.6f", totalRewards),
			"restaked":          fmt.Sprintf("%.6f", restakeAmount),
		})
	} else {
		fmt.Println()
		p.Success(p.Colors.Emoji("✅") + " Successfully restaked rewards!")
		fmt.Println()

		// Display transaction details
		p.KeyValueLine("Withdrawal TxHash", txHash, "green")
		p.KeyValueLine("Restake TxHash", delegateTxHash, "green")
		p.KeyValueLine("Amount Restaked", fmt.Sprintf("%.6f PC", restakeAmount), "yellow")
		fmt.Println()

		// Show helpful next steps
		fmt.Println(p.Colors.SubHeader("Next Steps"))
		fmt.Println(p.Colors.Separator(40))
		fmt.Println()
		fmt.Println(p.Colors.Info("  1. Check your increased stake:"))
		fmt.Println(p.Colors.Apply(p.Colors.Theme.Command, "     push-validator status"))
		fmt.Println()
		fmt.Println(p.Colors.Info("  2. Monitor validator performance:"))
		fmt.Println(p.Colors.Apply(p.Colors.Theme.Command, "     push-validator dashboard"))
		fmt.Println()
		fmt.Println(p.Colors.Apply(p.Colors.Theme.Description, "  Your validator power has been increased!"))
		fmt.Println()
	}
	return nil
}

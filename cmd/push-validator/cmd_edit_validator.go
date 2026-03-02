package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/pushchain/push-validator-cli/internal/validator"
)

// handleEditValidator orchestrates editing a validator's profile fields:
// - verify node is running and validator is registered
// - auto-derive key name
// - prompt for fields to update
// - submit edit-validator transaction
func handleEditValidator(d *Deps) error {
	if err := checkNodeRunning(d.Sup); err != nil {
		return err
	}

	p := getPrinter()
	cfg := d.Cfg

	// Step 1: Check validator status
	if flagOutput != "json" {
		fmt.Println()
		fmt.Print(p.Colors.Apply(p.Colors.Theme.Prompt, p.Colors.Emoji("🔍")+" Checking validator status..."))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	myVal, statusErr := d.Fetcher.GetMyValidator(ctx, cfg)
	cancel()

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

	// Step 2: Auto-derive key name
	defaultKeyName := getenvDefault("KEY_NAME", "validator-key")
	keyName := defaultKeyName

	if myVal.Address != "" {
		addrCtx, addrCancel := context.WithTimeout(context.Background(), 10*time.Second)
		accountAddr, convErr := convertValidatorToAccountAddress(addrCtx, myVal.Address, d.Runner)
		addrCancel()
		if convErr == nil {
			keyCtx, keyCancel := context.WithTimeout(context.Background(), 10*time.Second)
			foundKey, findErr := findKeyNameByAddress(keyCtx, cfg, accountAddr, d.Runner)
			keyCancel()
			if findErr == nil {
				keyName = foundKey
				if flagOutput != "json" {
					fmt.Printf("%s Using key: %s\n", p.Colors.Emoji("🔑"), keyName)
				}
			}
		}
	}

	// Step 3: Prompt for fields
	if flagOutput != "json" {
		fmt.Println()
		p.Section("Edit Validator Profile")
		fmt.Println()
		if myVal.Moniker != "" {
			fmt.Printf("  Current moniker:          %s\n", p.Colors.Apply(p.Colors.Theme.Value, myVal.Moniker))
		}
		if myVal.Website != "" {
			fmt.Printf("  Current website:          %s\n", p.Colors.Apply(p.Colors.Theme.Value, myVal.Website))
		}
		if myVal.Details != "" {
			fmt.Printf("  Current details:          %s\n", p.Colors.Apply(p.Colors.Theme.Value, myVal.Details))
		}
		if myVal.SecurityContact != "" {
			fmt.Printf("  Current security contact: %s\n", p.Colors.Apply(p.Colors.Theme.Value, myVal.SecurityContact))
		}
		if myVal.Identity != "" {
			fmt.Printf("  Current identity:         %s\n", p.Colors.Apply(p.Colors.Theme.Value, myVal.Identity))
		}
		fmt.Println()
	}

	prompter := d.Prompter
	var args validator.EditValidatorArgs
	args.KeyName = keyName

	// Read from env vars first, then prompt interactively
	args.Moniker = os.Getenv("VALIDATOR_MONIKER")
	args.Website = os.Getenv("VALIDATOR_WEBSITE")
	args.Details = os.Getenv("VALIDATOR_DETAILS")
	args.Security = os.Getenv("VALIDATOR_SECURITY")
	args.Identity = os.Getenv("VALIDATOR_IDENTITY")

	if prompter.IsInteractive() && flagOutput != "json" {
		monikerPrompt := "Enter new moniker (press ENTER to keep current): "
		if myVal.Moniker != "" {
			monikerPrompt = fmt.Sprintf("Enter new moniker (current: %s, press ENTER to keep): ", myVal.Moniker)
		}
		if moniker, err := prompter.ReadLine(monikerPrompt); err == nil && moniker != "" {
			args.Moniker = moniker
		}

		websitePrompt := "Enter website URL (press ENTER to skip): "
		if myVal.Website != "" {
			websitePrompt = fmt.Sprintf("Enter website URL (current: %s, press ENTER to keep): ", myVal.Website)
		}
		if website, err := prompter.ReadLine(websitePrompt); err == nil && website != "" {
			args.Website = website
		}

		detailsPrompt := "Enter description (press ENTER to skip): "
		if myVal.Details != "" {
			detailsPrompt = fmt.Sprintf("Enter description (current: %s, press ENTER to keep): ", myVal.Details)
		}
		if details, err := prompter.ReadLine(detailsPrompt); err == nil && details != "" {
			args.Details = details
		}

		securityPrompt := "Enter security contact email (press ENTER to skip): "
		if myVal.SecurityContact != "" {
			securityPrompt = fmt.Sprintf("Enter security contact email (current: %s, press ENTER to keep): ", myVal.SecurityContact)
		}
		if security, err := prompter.ReadLine(securityPrompt); err == nil && security != "" {
			args.Security = security
		}

		identityPrompt := "Enter Keybase identity for logo (16-digit ID, press ENTER to skip): "
		if myVal.Identity != "" {
			identityPrompt = fmt.Sprintf("Enter Keybase identity for logo (current: %s, press ENTER to keep): ", myVal.Identity)
		}
		if identity, err := prompter.ReadLine(identityPrompt); err == nil && identity != "" {
			args.Identity = identity
		}
	}

	// Check if anything was provided
	if args.Moniker == "" && args.Website == "" && args.Details == "" && args.Security == "" && args.Identity == "" {
		if flagOutput == "json" {
			getPrinter().JSON(map[string]any{"ok": true, "message": "no changes to make"})
		} else {
			fmt.Println(p.Colors.Info("No changes to make."))
			fmt.Println()
		}
		return nil
	}

	// Step 4: Submit transaction
	if flagOutput != "json" {
		fmt.Println()
		fmt.Print(p.Colors.Apply(p.Colors.Theme.Prompt, p.Colors.Emoji("📤")+" Submitting edit-validator transaction..."))
	}

	txCtx, txCancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer txCancel()

	txHash, err := d.Validator.EditValidator(txCtx, args)
	if err != nil {
		if flagOutput == "json" {
			getPrinter().JSON(map[string]any{"ok": false, "error": err.Error()})
		} else {
			fmt.Println()
			fmt.Println(p.Colors.Error(p.Colors.Emoji("❌") + " Edit validator failed"))
			fmt.Println()
			fmt.Printf("Error: %v\n", err)
			fmt.Println()
		}
		return fmt.Errorf("edit validator failed: %w", err)
	}

	if flagOutput != "json" {
		fmt.Println(" " + p.Colors.Success(p.Colors.Emoji("✓")))
	}

	// Success output
	if flagOutput == "json" {
		getPrinter().JSON(map[string]any{"ok": true, "txhash": txHash})
	} else {
		fmt.Println()
		p.Success(p.Colors.Emoji("✅") + " Validator profile updated successfully!")
		fmt.Println()
		p.KeyValueLine("Transaction Hash", txHash, "green")
		if args.Moniker != "" {
			p.KeyValueLine("New Moniker", args.Moniker, "blue")
		}
		if args.Website != "" {
			p.KeyValueLine("Website", args.Website, "blue")
		}
		if args.Details != "" {
			p.KeyValueLine("Details", args.Details, "dim")
		}
		if args.Security != "" {
			p.KeyValueLine("Security Contact", args.Security, "dim")
		}
		if args.Identity != "" {
			p.KeyValueLine("Identity", args.Identity, "dim")
		}
		fmt.Println()
	}
	return nil
}

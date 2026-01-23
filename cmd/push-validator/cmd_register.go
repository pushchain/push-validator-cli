package main

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/pushchain/push-validator-cli/internal/config"
	"github.com/pushchain/push-validator-cli/internal/node"
	"github.com/pushchain/push-validator-cli/internal/validator"
)

const (
	// registrationRequiredBalance is the minimum balance needed to register (1.6 PC in wei)
	registrationRequiredBalance = "1600000000000000000"

	// registrationMinStake is the minimum self-delegation amount (1.5 PC in wei)
	registrationMinStake = "1500000000000000000"

	// registrationFeeReserve is the amount reserved for transaction fees (0.1 PC in wei)
	registrationFeeReserve = "100000000000000000"

	// defaultCommissionRate is the default validator commission rate (10%)
	defaultCommissionRate = "0.10"

	// defaultMinSelfDelegation is the default minimum self-delegation value
	defaultMinSelfDelegation = "1"
)

// registrationInputs holds the collected registration parameters.
type registrationInputs struct {
	Moniker        string
	KeyName        string
	ImportMnemonic string
	CommissionRate string
	StakeAmount    string
}

// collectRegistrationInputs prompts for registration parameters interactively.
// It uses the Prompter interface for testable I/O.
func collectRegistrationInputs(d *Deps, defaults registrationInputs) (registrationInputs, error) {
	result := defaults
	prompter := d.Prompter

	// Moniker prompt
	if result.Moniker == "" || result.Moniker == "push-validator" {
		input, err := prompter.ReadLine(fmt.Sprintf("Enter validator name (moniker) [%s]: ", defaults.Moniker))
		if err == nil && input != "" {
			result.Moniker = input
		}
	}

	// Key name prompt
	input, err := prompter.ReadLine(fmt.Sprintf("Enter key name for validator (default: %s): ", defaults.KeyName))
	if err == nil && input != "" {
		result.KeyName = input
	}

	// Check if key already exists
	if keyExistsWithRunner(d.Cfg, result.KeyName, d.Runner) {
		p := d.Printer
		fmt.Println()
		fmt.Println(p.Colors.Warning(p.Colors.Emoji("⚠") + fmt.Sprintf(" Key '%s' already exists.", result.KeyName)))
		fmt.Println()
		fmt.Println(p.Colors.Info("You can use this existing key or create a new one."))
		fmt.Println(p.Colors.Apply(p.Colors.Theme.Description, "Note: Recovery mnemonics are only shown when creating new keys."))

		newName, err := prompter.ReadLine("\nEnter a different key name (or press ENTER to use existing key): ")
		if err == nil && newName != "" {
			result.KeyName = newName
			if !keyExistsWithRunner(d.Cfg, result.KeyName, d.Runner) {
				result.ImportMnemonic = promptWalletChoiceWith(prompter)
			}
		} else {
			fmt.Println()
			fmt.Println(p.Colors.Success(p.Colors.Emoji("✓") + " Proceeding with existing key"))
			fmt.Println()
		}
	} else {
		result.ImportMnemonic = promptWalletChoiceWith(prompter)
	}

	// Commission rate prompt
	result.CommissionRate = promptCommissionRate(prompter, defaults.CommissionRate)

	return result, nil
}

// promptCommissionRate prompts for the commission rate and validates it.
func promptCommissionRate(prompter Prompter, defaultRate string) string {
	input, err := prompter.ReadLine("Enter commission rate (1-100%) [10]: ")
	if err != nil || input == "" {
		return defaultRate
	}

	rate, err := strconv.ParseFloat(input, 64)
	if err != nil || rate < 1 || rate > 100 {
		return defaultRate
	}
	return fmt.Sprintf("%.2f", rate/100)
}

// promptWalletChoiceWith prompts the user to choose between creating a new wallet or importing.
// Returns the mnemonic if user chooses to import, empty string otherwise.
func promptWalletChoiceWith(prompter Prompter) string {
	fmt.Println()
	fmt.Println("Wallet Setup")
	fmt.Println("  [1] Create new wallet (generates new recovery phrase)")
	fmt.Println("  [2] Import existing wallet (use your recovery phrase)")
	fmt.Println()

	choice, _ := prompter.ReadLine("Choose option [1]: ")

	if choice != "2" {
		return ""
	}

	fmt.Println()
	fmt.Println("Enter your recovery mnemonic phrase (12 or 24 words):")

	mnemonic, err := prompter.ReadLine("> ")
	if err != nil {
		return ""
	}

	// Normalize the mnemonic
	mnemonic = strings.TrimSpace(mnemonic)
	mnemonic = strings.Join(strings.Fields(mnemonic), " ")
	mnemonic = strings.ToLower(mnemonic)

	if err := validator.ValidateMnemonic(mnemonic); err != nil {
		fmt.Printf("Invalid mnemonic: %v\n", err)
		return ""
	}

	fmt.Println("Mnemonic format validated")
	return mnemonic
}

// selectStakeAmount prompts for and validates the stake amount.
// Returns the stake in wei. If prompter is non-interactive or balance is empty, returns minStake.
func selectStakeAmount(prompter Prompter, balance string) (string, error) {
	if balance == "" {
		return registrationMinStake, nil
	}

	balInt := new(big.Int)
	balInt.SetString(balance, 10)
	feeInt := new(big.Int)
	feeInt.SetString(registrationFeeReserve, 10)
	maxStakeable := new(big.Int).Sub(balInt, feeInt)

	minStakeInt := new(big.Int)
	minStakeInt.SetString(registrationMinStake, 10)

	if !prompter.IsInteractive() {
		return maxStakeable.String(), nil
	}

	divisor := new(big.Float).SetFloat64(1e18)
	maxStakeFloat, _ := new(big.Float).SetString(maxStakeable.String())
	maxPC := new(big.Float).Quo(maxStakeFloat, divisor)

	for {
		minStakePC := 1.5
		maxStakePC, _ := strconv.ParseFloat(fmt.Sprintf("%.6f", maxPC), 64)

		input, err := prompter.ReadLine(fmt.Sprintf("Enter stake amount (%.1f - %.1f PC) [%.1f]: ", minStakePC, maxStakePC, maxStakePC))
		if err != nil || input == "" {
			return maxStakeable.String(), nil
		}

		stakeAmount, err := strconv.ParseFloat(input, 64)
		if err != nil {
			fmt.Println("Invalid amount. Enter a number. Try again.")
			continue
		}

		if stakeAmount < minStakePC {
			fmt.Printf("Amount too low. Minimum stake is %.1f PC. Try again.\n", minStakePC)
			continue
		}
		if stakeAmount > maxStakePC {
			fmt.Printf("Insufficient balance. Maximum: %.1f PC. Try again.\n", maxStakePC)
			continue
		}

		stakeWei := new(big.Float).Mul(new(big.Float).SetFloat64(stakeAmount), new(big.Float).SetFloat64(1e18))
		return stakeWei.Text('f', 0), nil
	}
}

// waitForFunding polls the validator's balance until it meets the required amount.
// Returns the final balance in wei, or error if max retries exceeded.
func waitForFunding(v validator.Service, prompter Prompter, address string, maxRetries int) (string, error) {
	for tries := 0; tries < maxRetries; {
		balCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		bal, err := v.Balance(balCtx, address)
		cancel()
		if err != nil {
			fmt.Printf("Balance check failed: %v\n", err)
			tries++
			time.Sleep(2 * time.Second)
			continue
		}

		balInt := new(big.Int)
		balInt.SetString(bal, 10)
		reqInt := new(big.Int)
		reqInt.SetString(registrationRequiredBalance, 10)
		if balInt.Cmp(reqInt) >= 0 {
			return bal, nil
		}

		// Display balance info
		pcAmount := "0.000000"
		if bal != "0" {
			balFloat, _ := new(big.Float).SetString(bal)
			divisor := new(big.Float).SetFloat64(1e18)
			result := new(big.Float).Quo(balFloat, divisor)
			pcAmount = fmt.Sprintf("%.6f", result)
		}

		fmt.Printf("Current Balance: %s PC (need 1.6 PC)\n", pcAmount)
		fmt.Println("Please send at least 1.6 PC to the EVM address shown above.")

		if prompter.IsInteractive() {
			_, _ = prompter.ReadLine("Press ENTER after funding...")
		} else {
			tries++
			time.Sleep(2 * time.Second)
		}
	}
	return "", fmt.Errorf("insufficient balance after %d retries", maxRetries)
}

var flagRegisterCheckOnly bool

// handleRegisterValidator is the main entry point for registration.
// It prompts interactively for moniker and key name if not set via env vars.
func handleRegisterValidator(d *Deps) error {
	cfg := d.Cfg
	// Get defaults from env or use hardcoded fallbacks
	defaultMoniker := getenvDefault("MONIKER", "push-validator")
	defaultKeyName := getenvDefault("KEY_NAME", "validator-key")
	defaultAmount := getenvDefault("STAKE_AMOUNT", registrationMinStake)

	moniker := defaultMoniker
	keyName := defaultKeyName

	statusCtx, statusCancel := context.WithTimeout(context.Background(), 20*time.Second)
	isValAlready, statusErr := d.Validator.IsValidator(statusCtx, "")
	statusCancel()
	if statusErr != nil {
		if flagOutput == "json" {
			getPrinter().JSON(map[string]any{"ok": false, "error": statusErr.Error()})
		} else {
			p := getPrinter()
			fmt.Println()
			fmt.Println(p.Colors.Error(p.Colors.Emoji("⚠️") + " Failed to verify validator status"))
			fmt.Printf("Error: %v\n\n", statusErr)
			fmt.Println("Please check your network connection and genesis domain configuration.")
		}
		return fmt.Errorf("failed to verify validator status: %w", statusErr)
	}
	if flagRegisterCheckOnly {
		if flagOutput == "json" {
			getPrinter().JSON(map[string]any{"ok": true, "registered": isValAlready})
		} else {
			p := getPrinter()
			fmt.Println()
			if isValAlready {
				fmt.Println(p.Colors.Success(p.Colors.Emoji("✓") + " This node is already registered as a validator"))
			} else {
				fmt.Println(p.Colors.Info("Validator registration required"))
			}
		}
		return nil
	}
	if isValAlready {
		if flagOutput == "json" {
			getPrinter().JSON(map[string]any{"ok": false, "error": "validator already registered"})
		} else {
			p := getPrinter()
			fmt.Println()
			fmt.Println(p.Colors.Success(p.Colors.Emoji("✓") + " This node is already registered as a validator"))
			fmt.Println()
			fmt.Println("Your validator is active on the network.")
			fmt.Println()
			p.Section("Validator Status")
			fmt.Println()
			fmt.Println(p.Colors.Info("  Check your validator:"))
			fmt.Println(p.Colors.Apply(p.Colors.Theme.Command, "     push-validator validators"))
			fmt.Println()
			fmt.Println(p.Colors.Info("  Monitor node status:"))
			fmt.Println(p.Colors.Apply(p.Colors.Theme.Command, "     push-validator status"))
			fmt.Println()
		}
		return fmt.Errorf("validator already registered")
	}

	// Check for moniker conflicts before prompting for registration
	monikerCheckCtx, monikerCheckCancel := context.WithTimeout(context.Background(), 10*time.Second)
	myValInfo, monikerErr := d.Fetcher.GetMyValidator(monikerCheckCtx, cfg)
	monikerCheckCancel()
	if monikerErr == nil && myValInfo.ValidatorExistsWithSameMoniker {
		if flagOutput == "json" {
			getPrinter().JSON(map[string]any{
				"ok":                false,
				"error":             "moniker conflict",
				"conflicting_moniker": myValInfo.ConflictingMoniker,
				"message":           fmt.Sprintf("A different validator is already using moniker '%s'. Choose a different moniker to register.", myValInfo.ConflictingMoniker),
			})
			return fmt.Errorf("moniker conflict: %s", myValInfo.ConflictingMoniker)
		} else {
			p := getPrinter()
			fmt.Println()
			fmt.Println(p.Colors.Warning(p.Colors.Emoji("⚠️") + " Moniker Conflict Detected"))
			fmt.Println()
			fmt.Printf("A different validator is already using the moniker '%s'.\n", p.Colors.Apply(p.Colors.Theme.Value, myValInfo.ConflictingMoniker))
			fmt.Println()
			fmt.Println(p.Colors.Info("Please choose a different moniker when registering your validator."))
			fmt.Println(p.Colors.Apply(p.Colors.Theme.Description, "Each validator must have a unique identifier on the network."))
			fmt.Println()
		}
		// Don't return - allow registration with a different moniker in interactive mode
	}

	// Interactive prompts (skip in JSON mode or if env vars are explicitly set)
	if flagOutput != "json" && d.Prompter.IsInteractive() {
		defaults := registrationInputs{
			Moniker:        moniker,
			KeyName:        keyName,
			CommissionRate: defaultCommissionRate,
		}
		inputs, err := collectRegistrationInputs(d, defaults)
		if err != nil {
			return err
		}
		// Interactive mode - let user choose stake amount
		// Pass empty string to trigger the interactive stake selection prompt
		return runRegisterValidatorWithDeps(d, cfg, inputs.Moniker, inputs.KeyName, "", inputs.CommissionRate, inputs.ImportMnemonic)
	}
	// JSON mode or non-interactive - use default/env amount
	commissionRate := getenvDefault("COMMISSION_RATE", defaultCommissionRate)
	return runRegisterValidatorWithDeps(d, cfg, moniker, keyName, defaultAmount, commissionRate, "")
}

// keyExistsWithRunner checks if a key with the given name already exists in the keyring
// using the provided CommandRunner for testability.
func keyExistsWithRunner(cfg config.Config, keyName string, runner CommandRunner) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := runner.Run(ctx, findPchaind(), "keys", "show", keyName, "-a",
		"--keyring-backend", cfg.KeyringBackend, "--home", cfg.HomeDir)
	return err == nil
}


// runRegisterValidatorWithDeps is the testable version that accepts
// injected dependencies. If d is nil, production dependencies are created.
func runRegisterValidatorWithDeps(d *Deps, cfg config.Config, moniker, keyName, amount, commissionRate, importMnemonic string) error {
	// Use injected deps or create production ones
	var nodeClient node.Client
	var remoteClient node.Client
	var v validator.Service
	var prompter Prompter

	if d != nil {
		nodeClient = d.Node
		remoteClient = d.RemoteNode
		v = d.Validator
		prompter = d.Prompter
	} else {
		local := strings.TrimRight(cfg.RPCLocal, "/")
		if local == "" {
			local = "http://127.0.0.1:26657"
		}
		remoteHTTP := cfg.RemoteRPCURL()
		nodeClient = node.New(local)
		remoteClient = node.New(remoteHTTP)
		v = validator.NewWith(validator.Options{BinPath: findPchaind(), HomeDir: cfg.HomeDir, ChainID: cfg.ChainID, Keyring: cfg.KeyringBackend, GenesisDomain: cfg.GenesisDomain, Denom: cfg.Denom})
		prompter = &ttyPrompter{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	stLocal, err1 := nodeClient.Status(ctx)
	_, err2 := remoteClient.RemoteStatus(ctx, cfg.RemoteRPCURL())
	if err1 == nil && err2 == nil {
		if stLocal.CatchingUp {
			if flagOutput == "json" {
				getPrinter().JSON(map[string]any{"ok": false, "error": "node is still syncing"})
			} else {
				fmt.Println("node is still syncing. Run 'push-validator sync' first")
			}
			return fmt.Errorf("node is still syncing")
		}
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel2()

	// Handle key creation or import based on importMnemonic
	var keyInfo validator.KeyInfo
	var err error
	var wasImported bool

	if importMnemonic != "" {
		// Import key from mnemonic
		keyInfo, err = v.ImportKey(ctx2, keyName, importMnemonic)
		wasImported = true
		if err != nil {
			if flagOutput == "json" {
				getPrinter().JSON(map[string]any{"ok": false, "error": err.Error()})
			} else {
				p := getPrinter()
				fmt.Println()
				fmt.Println(p.Colors.Error("Failed to import wallet"))
				fmt.Printf("Error: %v\n\n", err)
				fmt.Println(p.Colors.Info("Please verify your mnemonic phrase and try again."))
				fmt.Println()
			}
			return fmt.Errorf("failed to import wallet: %w", err)
		}
	} else {
		// Create new key or use existing (original behavior)
		keyInfo, err = v.EnsureKey(ctx2, keyName)
		if err != nil {
			if flagOutput == "json" {
				getPrinter().JSON(map[string]any{"ok": false, "error": err.Error()})
			} else {
				fmt.Printf("key error: %v\n", err)
			}
			return fmt.Errorf("key error: %w", err)
		}
	}

	evmAddr, err := v.GetEVMAddress(ctx2, keyInfo.Address)
	if err != nil {
		evmAddr = ""
	}

	p := getPrinter()

	if flagOutput != "json" {
		// Display appropriate message based on key creation method
		if keyInfo.Mnemonic != "" {
			// New key was created - display mnemonic in prominent box
			p.MnemonicBox(keyInfo.Mnemonic)
			fmt.Println()

			// Warning message in yellow
			fmt.Println(p.Colors.Warning("**Important** Write this mnemonic phrase in a safe place."))
			fmt.Println(p.Colors.Warning("It is the only way to recover your account if you ever forget your password."))
			fmt.Println()
		} else if wasImported {
			// Key was imported from mnemonic - show success message
			fmt.Println(p.Colors.Success(p.Colors.Emoji("✓") + fmt.Sprintf(" Wallet imported successfully: %s", keyInfo.Name)))
			fmt.Println(p.Colors.Apply(p.Colors.Theme.Description, "  (Keep your recovery phrase safe - it controls this wallet)"))
			fmt.Println()
		} else {
			// Existing key - show clear status with reminder
			fmt.Println(p.Colors.Success(p.Colors.Emoji("✓") + fmt.Sprintf(" Using existing key: %s", keyInfo.Name)))
			fmt.Println(p.Colors.Apply(p.Colors.Theme.Description, "  (Recovery mnemonic was displayed when this key was first created)"))
			fmt.Println()
		}

		// Always display Account Info section (whether new or existing key)
		p.Section("Account Info")
		p.KeyValueLine("EVM Address", evmAddr, "blue")
		p.KeyValueLine("Cosmos Address", keyInfo.Address, "dim")
		fmt.Println()
	}

	// Wait for funding
	finalBalance, fundingErr := waitForFunding(v, prompter, keyInfo.Address, 10)
	if fundingErr != nil {
		if flagOutput == "json" {
			getPrinter().JSON(map[string]any{"ok": false, "error": fundingErr.Error()})
		}
		return fundingErr
	}
	fmt.Println(p.Colors.Success(p.Colors.Emoji("✅") + " Sufficient balance"))

	// Interactive stake amount selection
	stake := amount
	if stake == "" {
		var stakeErr error
		stake, stakeErr = selectStakeAmount(prompter, finalBalance)
		if stakeErr != nil {
			return stakeErr
		}
	}
	// Create fresh context for registration transaction (independent of earlier operations)
	regCtx, regCancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer regCancel()
	txHash, err := v.Register(regCtx, validator.RegisterArgs{Moniker: moniker, Amount: stake, KeyName: keyName, CommissionRate: commissionRate, MinSelfDelegation: defaultMinSelfDelegation})
	if err != nil {
		errMsg := err.Error()
		if flagOutput == "json" {
			getPrinter().JSON(map[string]any{"ok": false, "error": errMsg})
		} else {
			// Check if this is a "validator already exists" error
			if strings.Contains(errMsg, "validator already exist") {
				p := getPrinter()
				fmt.Println()
				fmt.Println(p.Colors.Error(p.Colors.Emoji("❌") + " Validator registration failed: Validator pubkey already exists"))
				fmt.Println()
				fmt.Println("This validator consensus key is already registered on the network.")
				fmt.Println()
				p.Section("Resolution Options")
				fmt.Println()
				fmt.Println(p.Colors.Info("  1. Check existing validators:"))
				fmt.Println(p.Colors.Apply(p.Colors.Theme.Command, "     push-validator validators"))
				fmt.Println()
				fmt.Println(p.Colors.Info("  2. To register a new validator, reset node data:"))
				fmt.Println(p.Colors.Apply(p.Colors.Theme.Command, "     push-validator reset"))
				fmt.Println(p.Colors.Apply(p.Colors.Theme.Description, "     (This will generate new validator keys)"))
				fmt.Println()
				fmt.Println(p.Colors.Apply(p.Colors.Theme.Description, "  Note: Resetting will create a new validator identity."))
				fmt.Println()
			} else {
				fmt.Printf("register error: %v\n", err)
			}
		}
		return fmt.Errorf("validator registration failed: %w", err)
	}

	// Success output
	if flagOutput == "json" {
		getPrinter().JSON(map[string]any{"ok": true, "txhash": txHash, "moniker": moniker, "key_name": keyName, "commission_rate": commissionRate, "stake_amount": stake})
	} else {
		fmt.Println()
		p := getPrinter()
		p.Success(p.Colors.Emoji("✅") + " Validator registration successful!")
		fmt.Println()

		// Display registration details
		p.KeyValueLine("Transaction Hash", txHash, "green")
		p.KeyValueLine("Validator Name", moniker, "blue")

		// Convert stake amount from wei to PC for display
		stakeFloat, _ := new(big.Float).SetString(stake)
		divisor := new(big.Float).SetFloat64(1e18)
		stakePC := new(big.Float).Quo(stakeFloat, divisor)
		p.KeyValueLine("Staked Amount", fmt.Sprintf("%.6f", stakePC)+" PC", "yellow")

		// Convert commission rate back to percentage for display
		commRate, _ := strconv.ParseFloat(commissionRate, 64)
		p.KeyValueLine("Commission Rate", fmt.Sprintf("%.0f%%", commRate*100), "dim")
		fmt.Println()

		// Show helpful next steps
		fmt.Println(p.Colors.SubHeader("Next Steps"))
		fmt.Println(p.Colors.Separator(40))
		fmt.Println()
		fmt.Println(p.Colors.Info("  1. Check validator status:"))
		fmt.Println(p.Colors.Apply(p.Colors.Theme.Command, "     push-validator validators"))
		fmt.Println()
		fmt.Println(p.Colors.Info("  2. Live dashboard:"))
		fmt.Println(p.Colors.Apply(p.Colors.Theme.Command, "     push-validator dashboard"))
		fmt.Println()
		fmt.Println(p.Colors.Info("  3. Monitor node status:"))
		fmt.Println(p.Colors.Apply(p.Colors.Theme.Command, "     push-validator status"))
		fmt.Println()
		fmt.Println(p.Colors.Info("  4. View node logs:"))
		fmt.Println(p.Colors.Apply(p.Colors.Theme.Command, "     push-validator logs"))
		fmt.Println()
		fmt.Println(p.Colors.Apply(p.Colors.Theme.Description, "  Your validator will appear in the active set after the next epoch."))
		fmt.Println()
	}
	return nil
}

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pushchain/push-validator-cli/internal/config"
	"github.com/pushchain/push-validator-cli/internal/process"
	ui "github.com/pushchain/push-validator-cli/internal/ui"
	"github.com/pushchain/push-validator-cli/internal/validator"
)

// newSupervisor creates a Cosmovisor-based process supervisor.
func newSupervisor(homeDir string) process.Supervisor {
	return process.NewCosmovisor(homeDir)
}

// findPchaind returns the path to the pchaind binary, resolving
// either --bin flag, PCHAIND or PCHAIN_BIN environment variables, checking the
// cosmovisor genesis directory, or falling back to PATH lookup.
func findPchaind() string {
	if flagBin != "" {
		return flagBin
	}
	if v := os.Getenv("PCHAIND"); v != "" {
		return v
	}
	if v := os.Getenv("PCHAIN_BIN"); v != "" {
		return v
	}

	// Check cosmovisor genesis directory (primary location after install.sh)
	// Priority: --home flag > HOME_DIR env > default ~/.pchain
	homeDir := flagHome
	if homeDir == "" {
		homeDir = os.Getenv("HOME_DIR")
	}
	if homeDir == "" {
		if home, err := os.UserHomeDir(); err == nil {
			homeDir = filepath.Join(home, ".pchain")
		}
	}
	if homeDir != "" {
		cosmovisorPath := filepath.Join(homeDir, "cosmovisor", "genesis", "bin", "pchaind")
		if _, err := os.Stat(cosmovisorPath); err == nil {
			return cosmovisorPath
		}
	}

	return "pchaind"
}

// getenvDefault returns the environment value for k, or default d
// when k is not set.
func getenvDefault(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

// getPrinter returns a UI printer bound to the current --output flag.
func getPrinter() ui.Printer { return ui.NewPrinter(flagOutput) }

// parseDebugAddrField extracts a named field from pchaind debug addr output.
// The output format is lines like "Bech32 Acc: push1...", "Address (hex): 6AD3...".
// fieldPrefix should be e.g. "Bech32 Acc:" or "Address (hex):".
func parseDebugAddrField(output []byte, fieldPrefix string) (string, error) {
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, fieldPrefix) {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				return parts[len(parts)-1], nil
			}
		}
	}
	return "", fmt.Errorf("could not find %s in debug output", fieldPrefix)
}

// parseKeysListJSON parses the JSON output from pchaind keys list and finds the key name
// for the given target address.
func parseKeysListJSON(output []byte, targetAddress string) (string, error) {
	var keys []struct {
		Name    string `json:"name"`
		Address string `json:"address"`
	}
	if err := json.Unmarshal(output, &keys); err != nil {
		return "", fmt.Errorf("failed to parse keys: %w", err)
	}
	for _, key := range keys {
		if key.Address == targetAddress {
			return key.Name, nil
		}
	}
	return "", fmt.Errorf("no key found for address %s", targetAddress)
}

// convertValidatorToAccountAddress converts a validator operator address (pushvaloper...)
// to its corresponding account address (push...) using pchaind debug addr
func convertValidatorToAccountAddress(ctx context.Context, validatorAddress string, runners ...CommandRunner) (string, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
	}
	var runner CommandRunner
	if len(runners) > 0 && runners[0] != nil {
		runner = runners[0]
	} else {
		runner = &execRunner{}
	}
	bin := findPchaind()
	output, err := runner.Run(ctx, bin, "debug", "addr", validatorAddress)
	if err != nil {
		return "", fmt.Errorf("failed to convert address: %w", err)
	}
	return parseDebugAddrField(output, "Bech32 Acc:")
}

// getEVMAddress converts a bech32 address (push...) to EVM hex format (0x...)
// using pchaind debug addr command
func getEVMAddress(ctx context.Context, address string, runners ...CommandRunner) (string, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
	}
	var runner CommandRunner
	if len(runners) > 0 && runners[0] != nil {
		runner = runners[0]
	} else {
		runner = &execRunner{}
	}
	bin := findPchaind()
	output, err := runner.Run(ctx, bin, "debug", "addr", address)
	if err != nil {
		return "", fmt.Errorf("failed to convert address to EVM format: %w", err)
	}

	hexAddr, parseErr := parseDebugAddrField(output, "Address (hex):")
	if parseErr != nil {
		return "", parseErr
	}
	if !strings.HasPrefix(hexAddr, "0x") {
		hexAddr = "0x" + hexAddr
	}
	return hexAddr, nil
}

// hexToBech32Address converts a hex address (0x... or just hex bytes) to bech32 format (push1...)
// using pchaind debug addr command
func hexToBech32Address(ctx context.Context, hexAddr string, runners ...CommandRunner) (string, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
	}
	// Remove 0x prefix if present
	if strings.HasPrefix(hexAddr, "0x") || strings.HasPrefix(hexAddr, "0X") {
		hexAddr = hexAddr[2:]
	}

	var runner CommandRunner
	if len(runners) > 0 && runners[0] != nil {
		runner = runners[0]
	} else {
		runner = &execRunner{}
	}
	bin := findPchaind()
	output, err := runner.Run(ctx, bin, "debug", "addr", hexAddr)
	if err != nil {
		return "", fmt.Errorf("failed to convert hex address to bech32: %w", err)
	}
	return parseDebugAddrField(output, "Bech32 Acc:")
}

// findKeyNameByAddress finds the key name in the keyring that corresponds to the given address
func findKeyNameByAddress(ctx context.Context, cfg config.Config, accountAddress string, runners ...CommandRunner) (string, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
	}
	var runner CommandRunner
	if len(runners) > 0 && runners[0] != nil {
		runner = runners[0]
	} else {
		runner = &execRunner{}
	}
	bin := findPchaind()
	output, err := runner.Run(ctx, bin, "keys", "list", "--keyring-backend", cfg.KeyringBackend, "--home", cfg.HomeDir, "--output", "json")
	if err != nil {
		return "", fmt.Errorf("failed to list keys: %w", err)
	}
	return parseKeysListJSON(output, accountAddress)
}

// waitForSufficientBalance checks if the account has enough balance to pay gas fees
// If not, prompts user to fund the wallet and waits for them to press Enter
// requiredBalance is in micro-units (upc)
// Returns true if balance is sufficient, false if check failed
func waitForSufficientBalance(cfg config.Config, accountAddr string, evmAddr string, requiredBalance string, operationName string) bool {
	v := validator.NewWith(validator.Options{
		BinPath:       findPchaind(),
		HomeDir:       cfg.HomeDir,
		ChainID:       cfg.ChainID,
		Keyring:       cfg.KeyringBackend,
		GenesisDomain: cfg.GenesisDomain,
		Denom:         cfg.Denom,
	})
	return waitForSufficientBalanceWith(v, getPrinter(), &ttyPrompter{}, accountAddr, evmAddr, requiredBalance, operationName)
}

// waitForSufficientBalanceWith is the testable version that accepts injected dependencies.
func waitForSufficientBalanceWith(v validator.Service, p ui.Printer, prompter Prompter, accountAddr string, evmAddr string, requiredBalance string, operationName string) bool {
	maxRetries := 10
	for tries := 0; tries < maxRetries; tries++ {
		balCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		bal, err := v.Balance(balCtx, accountAddr)
		cancel()

		if err != nil {
			fmt.Printf("%s Balance check failed: %v\n", p.Colors.Emoji("⚠️"), err)
			time.Sleep(2 * time.Second)
			continue
		}

		balInt := new(big.Int)
		balInt.SetString(bal, 10)
		reqInt := new(big.Int)
		reqInt.SetString(requiredBalance, 10)

		if balInt.Cmp(reqInt) >= 0 {
			fmt.Println(p.Colors.Success(p.Colors.Emoji("✅") + " Sufficient balance"))
			fmt.Println()
			return true
		}

		// Convert balance to PC for display (1 PC = 1e18 upc)
		pcAmount := "0.000000"
		if bal != "0" {
			balFloat, _ := new(big.Float).SetString(bal)
			divisor := new(big.Float).SetFloat64(1e18)
			result := new(big.Float).Quo(balFloat, divisor)
			pcAmount = fmt.Sprintf("%.6f", result)
		}

		// Convert required to PC for display
		reqFloat, _ := new(big.Float).SetString(requiredBalance)
		divisor := new(big.Float).SetFloat64(1e18)
		reqPC := new(big.Float).Quo(reqFloat, divisor)
		reqPCStr := fmt.Sprintf("%.6f", reqPC)

		// Display funding information with address
		fmt.Println()
		p.KeyValueLine("Current Balance", pcAmount+" PC", "yellow")
		p.KeyValueLine("Required for "+operationName, reqPCStr+" PC", "yellow")
		fmt.Println()
		if evmAddr != "" {
			p.KeyValueLine("Send funds to", evmAddr, "blue")
			fmt.Println()
		}
		fmt.Printf("Please send at least %s to your account for %s.\n\n", p.Colors.Warning(reqPCStr+" PC"), operationName)
		fmt.Printf("Use faucet at %s for testnet validators\n", p.Colors.Info("https://faucet.push.org"))
		fmt.Printf("or contact us at %s\n\n", p.Colors.Info("push.org/support"))

		// Wait for user to press Enter
		if prompter.IsInteractive() {
			_, _ = prompter.ReadLine(p.Colors.Apply(p.Colors.Theme.Prompt, "Press ENTER after funding..."))
			fmt.Println()
		}
	}

	// After max retries, give up
	fmt.Println(p.Colors.Error(p.Colors.Emoji("❌") + " Unable to verify sufficient balance after multiple attempts"))
	fmt.Println()
	return false
}

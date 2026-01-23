package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/pushchain/push-validator-cli/internal/validator"
)

func stakeDeps(overrides ...func(*Deps)) *Deps {
	d := &Deps{
		Cfg:        testCfg(),
		Sup:        &mockSupervisor{running: true},
		Node:       &mockNodeClient{},
		RemoteNode: &mockNodeClient{},
		Fetcher:    &mockFetcher{},
		Validator:  &mockValidator{},
		Runner:     newMockRunner(),
		RPCCheck:   func(string, time.Duration) bool { return true },
		Prompter:   &nonInteractivePrompter{},
	}
	for _, fn := range overrides {
		fn(d)
	}
	return d
}

func TestHandleIncreaseStake_FetchValidatorError(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	d := stakeDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{myValidatorErr: fmt.Errorf("timeout")}
	})

	err := handleIncreaseStake(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "failed to retrieve validator information") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleIncreaseStake_NotValidator(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	d := stakeDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{myValidator: validator.MyValidatorInfo{IsValidator: false}}
	})

	err := handleIncreaseStake(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "not a registered validator") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleIncreaseStake_AddressConversionFails(t *testing.T) {
	origOutput := flagOutput
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagOutput = origOutput
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagOutput = "json"
	flagNoColor = true
	flagNoEmoji = true

	runner := newMockRunner()
	binPath := findPchaind()
	runner.errors[binPath+" debug addr pushvaloper1test"] = fmt.Errorf("binary not found")

	d := stakeDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{
			myValidator: validator.MyValidatorInfo{
				IsValidator: true,
				Address:     "pushvaloper1test",
				Moniker:     "test-val",
				VotingPower: 1000000,
			},
		}
		d.Runner = runner
	})

	err := handleIncreaseStake(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "failed to convert validator address") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleIncreaseStake_BalanceFetchFails(t *testing.T) {
	origOutput := flagOutput
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagOutput = origOutput
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagOutput = "json"
	flagNoColor = true
	flagNoEmoji = true

	runner := newMockRunner()
	binPath := findPchaind()
	runner.outputs[binPath+" debug addr pushvaloper1test"] = []byte("Bech32 Acc: push1account\nAddress (hex): AABB\n")

	d := stakeDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{
			myValidator: validator.MyValidatorInfo{
				IsValidator: true,
				Address:     "pushvaloper1test",
				Moniker:     "test-val",
			},
		}
		d.Validator = &mockValidator{balanceErr: fmt.Errorf("rpc error")}
		d.Runner = runner
	})

	err := handleIncreaseStake(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "failed to retrieve balance") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleIncreaseStake_InsufficientBalance(t *testing.T) {
	origOutput := flagOutput
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagOutput = origOutput
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagOutput = "json"
	flagNoColor = true
	flagNoEmoji = true

	runner := newMockRunner()
	binPath := findPchaind()
	runner.outputs[binPath+" debug addr pushvaloper1test"] = []byte("Bech32 Acc: push1account\nAddress (hex): AABB\n")

	d := stakeDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{
			myValidator: validator.MyValidatorInfo{
				IsValidator: true,
				Address:     "pushvaloper1test",
				Moniker:     "test-val",
			},
		}
		// Balance of 50000000000000000 wei = 0.05 PC, which is less than the 0.1 PC fee reserve
		d.Validator = &mockValidator{balanceResult: "50000000000000000"}
		d.Runner = runner
	})

	err := handleIncreaseStake(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "insufficient balance") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleIncreaseStake_TextOutput_NotValidator(t *testing.T) {
	origOutput := flagOutput
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagOutput = origOutput
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagOutput = "text"
	flagNoColor = true
	flagNoEmoji = true

	d := stakeDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{myValidator: validator.MyValidatorInfo{IsValidator: false}}
	})

	err := handleIncreaseStake(d)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestHandleIncreaseStake_NonInteractive_Success(t *testing.T) {
	origOutput := flagOutput
	origNonInteractive := flagNonInteractive
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	origYes := flagYes
	defer func() {
		flagOutput = origOutput
		flagNonInteractive = origNonInteractive
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
		flagYes = origYes
	}()
	flagOutput = "json"
	flagNonInteractive = true
	flagNoColor = true
	flagNoEmoji = true
	flagYes = true

	runner := newMockRunner()
	binPath := findPchaind()
	cfg := testCfg()
	runner.outputs[binPath+" debug addr pushvaloper1test"] = []byte("Bech32 Acc: push1account\nAddress (hex): AABB\n")
	runner.outputs[binPath+" keys list --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir+" --output json"] = []byte(`[{"name":"validator-key","address":"push1account"}]`)

	d := stakeDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{
			myValidator: validator.MyValidatorInfo{
				IsValidator: true,
				Address:     "pushvaloper1test",
				Moniker:     "test-val",
				VotingPower: 1000000,
			},
		}
		// 5 PC balance = 5000000000000000000 wei
		d.Validator = &mockValidator{
			balanceResult:  "5000000000000000000",
			delegateResult: "TX_DELEGATE_HASH",
		}
		d.Runner = runner
	})

	err := handleIncreaseStake(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleIncreaseStake_NonInteractive_DelegationFails(t *testing.T) {
	origOutput := flagOutput
	origNonInteractive := flagNonInteractive
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	origYes := flagYes
	defer func() {
		flagOutput = origOutput
		flagNonInteractive = origNonInteractive
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
		flagYes = origYes
	}()
	flagOutput = "json"
	flagNonInteractive = true
	flagNoColor = true
	flagNoEmoji = true
	flagYes = true

	runner := newMockRunner()
	binPath := findPchaind()
	cfg := testCfg()
	runner.outputs[binPath+" debug addr pushvaloper1test"] = []byte("Bech32 Acc: push1account\nAddress (hex): AABB\n")
	runner.outputs[binPath+" keys list --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir+" --output json"] = []byte(`[{"name":"validator-key","address":"push1account"}]`)

	d := stakeDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{
			myValidator: validator.MyValidatorInfo{
				IsValidator: true,
				Address:     "pushvaloper1test",
				Moniker:     "test-val",
				VotingPower: 1000000,
			},
		}
		d.Validator = &mockValidator{
			balanceResult: "5000000000000000000",
			delegateErr:   fmt.Errorf("insufficient gas"),
		}
		d.Runner = runner
	})

	err := handleIncreaseStake(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "delegation transaction failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleIncreaseStake_NonInteractive_TextSuccess(t *testing.T) {
	origOutput := flagOutput
	origNonInteractive := flagNonInteractive
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	origYes := flagYes
	defer func() {
		flagOutput = origOutput
		flagNonInteractive = origNonInteractive
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
		flagYes = origYes
	}()
	flagOutput = "text"
	flagNonInteractive = true
	flagNoColor = true
	flagNoEmoji = true
	flagYes = true

	runner := newMockRunner()
	binPath := findPchaind()
	cfg := testCfg()
	runner.outputs[binPath+" debug addr pushvaloper1test"] = []byte("Bech32 Acc: push1account\nAddress (hex): AABB\n")
	runner.outputs[binPath+" keys list --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir+" --output json"] = []byte(`[{"name":"validator-key","address":"push1account"}]`)

	d := stakeDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{
			myValidator: validator.MyValidatorInfo{
				IsValidator: true,
				Address:     "pushvaloper1test",
				Moniker:     "test-val",
				VotingPower: 2000000,
			},
		}
		d.Validator = &mockValidator{
			balanceResult:  "2000000000000000000",
			delegateResult: "TX_DELEGATE_TEXT",
		}
		d.Runner = runner
	})

	err := handleIncreaseStake(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleIncreaseStake_NonInteractive_KeyDerivationFallback(t *testing.T) {
	origOutput := flagOutput
	origNonInteractive := flagNonInteractive
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	origYes := flagYes
	defer func() {
		flagOutput = origOutput
		flagNonInteractive = origNonInteractive
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
		flagYes = origYes
	}()
	flagOutput = "json"
	flagNonInteractive = true
	flagNoColor = true
	flagNoEmoji = true
	flagYes = true

	runner := newMockRunner()
	binPath := findPchaind()
	// First call for balance check succeeds
	runner.outputs[binPath+" debug addr pushvaloper1test"] = []byte("Bech32 Acc: push1account\nAddress (hex): AABB\n")
	// Key list returns no matching key - falls back to default
	cfg := testCfg()
	runner.outputs[binPath+" keys list --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir+" --output json"] = []byte(`[{"name":"other-key","address":"push1other"}]`)

	d := stakeDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{
			myValidator: validator.MyValidatorInfo{
				IsValidator: true,
				Address:     "pushvaloper1test",
				Moniker:     "test-val",
				VotingPower: 1000000,
			},
		}
		d.Validator = &mockValidator{
			balanceResult:  "5000000000000000000",
			delegateResult: "TX_DELEGATE_FALLBACK",
		}
		d.Runner = runner
	})

	err := handleIncreaseStake(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleIncreaseStake_Interactive_ValidAmount(t *testing.T) {
	origOutput := flagOutput
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagOutput = origOutput
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagOutput = "text"
	flagNoColor = true
	flagNoEmoji = true

	runner := newMockRunner()
	binPath := findPchaind()
	cfg := testCfg()
	runner.outputs[binPath+" debug addr pushvaloper1test"] = []byte("Bech32 Acc: push1account\nAddress (hex): AABB\n")
	runner.outputs[binPath+" keys list --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir+" --output json"] = []byte(`[{"name":"validator-key","address":"push1account"}]`)

	d := stakeDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{
			myValidator: validator.MyValidatorInfo{
				IsValidator: true,
				Address:     "pushvaloper1test",
				Moniker:     "test-val",
				VotingPower: 1000000,
			},
		}
		d.Validator = &mockValidator{
			balanceResult:  "5000000000000000000", // 5 PC
			delegateResult: "TX_INTERACTIVE",
		}
		d.Runner = runner
		d.Prompter = &mockPrompter{interactive: true, responses: []string{"2.0"}}
	})

	err := handleIncreaseStake(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleIncreaseStake_Interactive_InvalidThenValid(t *testing.T) {
	origOutput := flagOutput
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagOutput = origOutput
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagOutput = "text"
	flagNoColor = true
	flagNoEmoji = true

	runner := newMockRunner()
	binPath := findPchaind()
	cfg := testCfg()
	runner.outputs[binPath+" debug addr pushvaloper1test"] = []byte("Bech32 Acc: push1account\nAddress (hex): AABB\n")
	runner.outputs[binPath+" keys list --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir+" --output json"] = []byte(`[{"name":"validator-key","address":"push1account"}]`)

	d := stakeDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{
			myValidator: validator.MyValidatorInfo{
				IsValidator: true,
				Address:     "pushvaloper1test",
				Moniker:     "test-val",
				VotingPower: 1000000,
			},
		}
		d.Validator = &mockValidator{
			balanceResult:  "5000000000000000000",
			delegateResult: "TX_INTERACTIVE_2",
		}
		d.Runner = runner
		// Empty, then invalid, then too low, then too high, then valid
		d.Prompter = &mockPrompter{interactive: true, responses: []string{"", "abc", "0.01", "999", "1.5"}}
	})

	err := handleIncreaseStake(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleIncreaseStake_TextOutput_FetchError(t *testing.T) {
	origOutput := flagOutput
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagOutput = origOutput
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagOutput = "text"
	flagNoColor = true
	flagNoEmoji = true

	d := stakeDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{myValidatorErr: fmt.Errorf("network timeout")}
	})

	err := handleIncreaseStake(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "failed to retrieve validator information") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleIncreaseStake_TextOutput_BalanceFetchFails(t *testing.T) {
	origOutput := flagOutput
	origNonInteractive := flagNonInteractive
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagOutput = origOutput
		flagNonInteractive = origNonInteractive
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagOutput = "text"
	flagNonInteractive = true
	flagNoColor = true
	flagNoEmoji = true

	runner := newMockRunner()
	binPath := findPchaind()
	runner.outputs[binPath+" debug addr pushvaloper1test"] = []byte("Bech32 Acc: push1account\nAddress (hex): AABB\n")

	d := stakeDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{
			myValidator: validator.MyValidatorInfo{
				IsValidator: true,
				Address:     "pushvaloper1test",
				Moniker:     "test-val",
			},
		}
		d.Validator = &mockValidator{balanceErr: fmt.Errorf("rpc timeout")}
		d.Runner = runner
	})

	err := handleIncreaseStake(d)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestHandleIncreaseStake_TextOutput_InsufficientBalance(t *testing.T) {
	origOutput := flagOutput
	origNonInteractive := flagNonInteractive
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagOutput = origOutput
		flagNonInteractive = origNonInteractive
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagOutput = "text"
	flagNonInteractive = true
	flagNoColor = true
	flagNoEmoji = true

	runner := newMockRunner()
	binPath := findPchaind()
	runner.outputs[binPath+" debug addr pushvaloper1test"] = []byte("Bech32 Acc: push1account\nAddress (hex): AABB\n")

	d := stakeDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{
			myValidator: validator.MyValidatorInfo{
				IsValidator: true,
				Address:     "pushvaloper1test",
				Moniker:     "test-val",
			},
		}
		d.Validator = &mockValidator{balanceResult: "50000000000000000"}
		d.Runner = runner
	})

	err := handleIncreaseStake(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "insufficient balance") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleIncreaseStake_TextOutput_AddressConversionFails(t *testing.T) {
	origOutput := flagOutput
	origNonInteractive := flagNonInteractive
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagOutput = origOutput
		flagNonInteractive = origNonInteractive
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagOutput = "text"
	flagNonInteractive = true
	flagNoColor = true
	flagNoEmoji = true

	runner := newMockRunner()
	binPath := findPchaind()
	runner.errors[binPath+" debug addr pushvaloper1test"] = fmt.Errorf("binary not found")

	d := stakeDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{
			myValidator: validator.MyValidatorInfo{
				IsValidator: true,
				Address:     "pushvaloper1test",
				Moniker:     "test-val",
			},
		}
		d.Runner = runner
	})

	err := handleIncreaseStake(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "failed to convert validator address") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleIncreaseStake_NonInteractive_DelegationFails_Text(t *testing.T) {
	origOutput := flagOutput
	origNonInteractive := flagNonInteractive
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	origYes := flagYes
	defer func() {
		flagOutput = origOutput
		flagNonInteractive = origNonInteractive
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
		flagYes = origYes
	}()
	flagOutput = "text"
	flagNonInteractive = true
	flagNoColor = true
	flagNoEmoji = true
	flagYes = true

	runner := newMockRunner()
	binPath := findPchaind()
	cfg := testCfg()
	runner.outputs[binPath+" debug addr pushvaloper1test"] = []byte("Bech32 Acc: push1account\nAddress (hex): AABB\n")
	runner.outputs[binPath+" keys list --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir+" --output json"] = []byte(`[{"name":"validator-key","address":"push1account"}]`)

	d := stakeDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{
			myValidator: validator.MyValidatorInfo{
				IsValidator: true,
				Address:     "pushvaloper1test",
				Moniker:     "test-val",
			},
		}
		d.Validator = &mockValidator{
			balanceResult: "5000000000000000000",
			delegateErr:   fmt.Errorf("out of gas"),
		}
		d.Runner = runner
	})

	err := handleIncreaseStake(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "delegation transaction failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

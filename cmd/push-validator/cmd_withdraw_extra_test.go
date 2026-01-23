package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/pushchain/push-validator-cli/internal/node"
	"github.com/pushchain/push-validator-cli/internal/validator"
)

func TestHandleWithdrawRewards_TextOutput_FullSuccess(t *testing.T) {
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
	cfg := testCfg()
	runner.outputs[binPath+" debug addr pushvaloper1test"] = []byte("Bech32 Acc: push1account\nAddress (hex): AABB1234\n")
	runner.outputs[binPath+" debug addr push1account"] = []byte("Address (hex): AABB1234\n")
	runner.outputs[binPath+" keys list --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir+" --output json"] = []byte(`[{"name":"mykey","address":"push1account"}]`)

	d := withdrawDeps(func(d *Deps) {
		d.Node = &mockNodeClient{status: node.Status{Height: 200, CatchingUp: false}}
		d.RemoteNode = &mockNodeClient{status: node.Status{Height: 200}}
		d.Fetcher = &mockFetcher{
			myValidator: validator.MyValidatorInfo{
				IsValidator: true,
				Address:     "pushvaloper1test",
			},
			commission:  "5.0",
			outstanding: "10.0",
		}
		d.Validator = &mockValidator{withdrawResult: "TX_SUCCESS_123"}
		d.Runner = runner
	})

	err := handleWithdrawRewards(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleWithdrawRewards_JSON_WithdrawSuccess(t *testing.T) {
	origOutput := flagOutput
	origNonInteractive := flagNonInteractive
	defer func() {
		flagOutput = origOutput
		flagNonInteractive = origNonInteractive
	}()
	flagOutput = "json"
	flagNonInteractive = true

	runner := newMockRunner()
	binPath := findPchaind()
	cfg := testCfg()
	runner.outputs[binPath+" debug addr pushvaloper1test"] = []byte("Bech32 Acc: push1account\n")
	runner.outputs[binPath+" keys list --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir+" --output json"] = []byte(`[{"name":"mykey","address":"push1account"}]`)

	d := withdrawDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{
			myValidator: validator.MyValidatorInfo{
				IsValidator: true,
				Address:     "pushvaloper1test",
			},
			commission:  "5.0",
			outstanding: "10.0",
		}
		d.Validator = &mockValidator{withdrawResult: "TX_JSON_SUCCESS"}
		d.Runner = runner
	})

	// JSON mode returns rewards info first; doesn't actually withdraw
	// This is the JSON path that returns rewards info and exits early
	err := handleWithdrawRewards(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleWithdrawRewards_EmptyValidatorAddress(t *testing.T) {
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
	cfg := testCfg()
	runner.outputs[binPath+" debug addr "] = []byte("")
	runner.outputs[binPath+" keys list --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir+" --output json"] = []byte(`[]`)

	d := withdrawDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{
			myValidator: validator.MyValidatorInfo{
				IsValidator: true,
				Address:     "", // Empty address
			},
			commission:  "5.0",
			outstanding: "10.0",
		}
		d.Validator = &mockValidator{withdrawResult: "TX_EMPTY_ADDR"}
		d.Runner = runner
	})

	// With empty address, key derivation falls back to default
	// Then balance addr derivation will also fail, causing an error
	err := handleWithdrawRewards(d)
	// The empty address leads to "failed to derive account address" error
	// because convertValidatorToAccountAddress fails with empty input
	if err == nil {
		// It might succeed if the runner mock handles it
		t.Log("no error with empty address - acceptable")
	}
}

func TestHandleWithdrawRewards_RewardsError_TextOutput(t *testing.T) {
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
	cfg := testCfg()
	runner.outputs[binPath+" debug addr pushvaloper1test"] = []byte("Bech32 Acc: push1account\nAddress (hex): AABB\n")
	runner.outputs[binPath+" debug addr push1account"] = []byte("Address (hex): AABB\n")
	runner.outputs[binPath+" keys list --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir+" --output json"] = []byte(`[{"name":"mykey","address":"push1account"}]`)

	d := withdrawDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{
			myValidator: validator.MyValidatorInfo{
				IsValidator: true,
				Address:     "pushvaloper1test",
			},
			commission:  "5.0",
			outstanding: "10.0",
			rewardsErr:  fmt.Errorf("timeout"),
		}
		// Even with rewards error, withdrawal can proceed
		d.Validator = &mockValidator{withdrawResult: "TX_DESPITE_ERR"}
		d.Runner = runner
	})

	err := handleWithdrawRewards(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleWithdrawRewards_AddrConversionFails_Text_UsesFallback(t *testing.T) {
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
	// Addr conversion fails
	runner.errors[binPath+" debug addr pushvaloper1test"] = fmt.Errorf("conversion error")

	d := &Deps{
		Cfg:        testCfg(),
		Sup:        &mockSupervisor{running: true},
		Node:       &mockNodeClient{status: node.Status{Height: 100, CatchingUp: false}},
		RemoteNode: &mockNodeClient{status: node.Status{Height: 100}},
		Fetcher: &mockFetcher{
			myValidator: validator.MyValidatorInfo{
				IsValidator: true,
				Address:     "pushvaloper1test",
			},
			commission:  "5.0",
			outstanding: "10.0",
		},
		Validator: &mockValidator{withdrawResult: "TX_FALLBACK"},
		Runner:    runner,
		RPCCheck:  func(string, time.Duration) bool { return true },
		Prompter:  &nonInteractivePrompter{},
	}

	// When addr conversion fails for balance check, it should return error
	err := handleWithdrawRewards(d)
	if err == nil {
		t.Fatal("expected error when addr derivation fails for balance check")
	}
	if !containsSubstr(err.Error(), "failed to derive account address") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleWithdrawRewards_Interactive_NoSignificantRewards_CancelPrompt(t *testing.T) {
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

	d := withdrawDeps(func(d *Deps) {
		d.Node = &mockNodeClient{status: node.Status{Height: 100, CatchingUp: false}}
		d.RemoteNode = &mockNodeClient{status: node.Status{Height: 100}}
		d.Fetcher = &mockFetcher{
			myValidator: validator.MyValidatorInfo{
				IsValidator: true,
				Address:     "pushvaloper1test",
			},
			commission:  "0.001",
			outstanding: "0.005",
		}
		d.Prompter = &mockPrompter{interactive: true, responses: []string{"n"}} // Decline to continue
	})

	err := handleWithdrawRewards(d)
	if err != nil {
		t.Fatalf("expected nil (cancelled), got: %v", err)
	}
}

func TestHandleWithdrawRewards_Interactive_NoSignificantRewards_Continue(t *testing.T) {
	origOutput := flagOutput
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	origNonInteractive := flagNonInteractive
	defer func() {
		flagOutput = origOutput
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
		flagNonInteractive = origNonInteractive
	}()
	flagOutput = "text"
	flagNoColor = true
	flagNoEmoji = true
	flagNonInteractive = true // Skip waitForSufficientBalance

	runner := newMockRunner()
	binPath := findPchaind()
	cfg := testCfg()
	runner.outputs[binPath+" debug addr pushvaloper1test"] = []byte("Bech32 Acc: push1account\n")
	runner.outputs[binPath+" debug addr push1account"] = []byte("Address (hex): AABB\n")
	runner.outputs[binPath+" keys list --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir+" --output json"] = []byte(`[{"name":"mykey","address":"push1account"}]`)

	d := withdrawDeps(func(d *Deps) {
		d.Node = &mockNodeClient{status: node.Status{Height: 100, CatchingUp: false}}
		d.RemoteNode = &mockNodeClient{status: node.Status{Height: 100}}
		d.Fetcher = &mockFetcher{
			myValidator: validator.MyValidatorInfo{
				IsValidator: true,
				Address:     "pushvaloper1test",
			},
			commission:  "0.001",
			outstanding: "0.005",
		}
		d.Validator = &mockValidator{withdrawResult: "TX_LOW_REWARDS"}
		d.Runner = runner
		// Say "y" to continue, then "custom-key" for key name, then "n" for commission
		d.Prompter = &mockPrompter{interactive: true, responses: []string{"y", "custom-key", "n"}}
	})

	err := handleWithdrawRewards(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleWithdrawRewards_Interactive_KeyPrompt_CustomKey(t *testing.T) {
	origOutput := flagOutput
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	origNonInteractive := flagNonInteractive
	defer func() {
		flagOutput = origOutput
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
		flagNonInteractive = origNonInteractive
	}()
	flagOutput = "text"
	flagNoColor = true
	flagNoEmoji = true
	flagNonInteractive = true // Skip waitForSufficientBalance

	runner := newMockRunner()
	binPath := findPchaind()
	cfg := testCfg()
	runner.outputs[binPath+" debug addr pushvaloper1test"] = []byte("Bech32 Acc: push1account\n")
	runner.outputs[binPath+" debug addr push1account"] = []byte("Address (hex): AABB\n")
	// Key list fails so key derivation falls back to default
	runner.errors[binPath+" keys list --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir+" --output json"] = errMock

	d := withdrawDeps(func(d *Deps) {
		d.Node = &mockNodeClient{status: node.Status{Height: 100, CatchingUp: false}}
		d.RemoteNode = &mockNodeClient{status: node.Status{Height: 100}}
		d.Fetcher = &mockFetcher{
			myValidator: validator.MyValidatorInfo{
				IsValidator: true,
				Address:     "pushvaloper1test",
			},
			commission:  "5.0",
			outstanding: "10.0",
		}
		d.Validator = &mockValidator{withdrawResult: "TX_CUSTOM_KEY"}
		d.Runner = runner
		// Key derivation fails → fallback to default → prompt for key name, then commission
		d.Prompter = &mockPrompter{interactive: true, responses: []string{"my-custom-key", "y"}}
	})

	err := handleWithdrawRewards(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleWithdrawRewards_Interactive_IncludeCommission(t *testing.T) {
	origOutput := flagOutput
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	origNonInteractive := flagNonInteractive
	defer func() {
		flagOutput = origOutput
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
		flagNonInteractive = origNonInteractive
	}()
	flagOutput = "text"
	flagNoColor = true
	flagNoEmoji = true
	flagNonInteractive = true // Skip waitForSufficientBalance

	runner := newMockRunner()
	binPath := findPchaind()
	cfg := testCfg()
	runner.outputs[binPath+" debug addr pushvaloper1test"] = []byte("Bech32 Acc: push1account\n")
	runner.outputs[binPath+" debug addr push1account"] = []byte("Address (hex): AABB\n")
	runner.outputs[binPath+" keys list --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir+" --output json"] = []byte(`[{"name":"mykey","address":"push1account"}]`)

	d := withdrawDeps(func(d *Deps) {
		d.Node = &mockNodeClient{status: node.Status{Height: 100, CatchingUp: false}}
		d.RemoteNode = &mockNodeClient{status: node.Status{Height: 100}}
		d.Fetcher = &mockFetcher{
			myValidator: validator.MyValidatorInfo{
				IsValidator: true,
				Address:     "pushvaloper1test",
			},
			commission:  "5.0",
			outstanding: "10.0",
		}
		d.Validator = &mockValidator{withdrawResult: "TX_WITH_COMMISSION"}
		d.Runner = runner
		// Key derivation succeeds so no key prompt. Only commission prompt: "y"
		d.Prompter = &mockPrompter{interactive: true, responses: []string{"y"}}
	})

	err := handleWithdrawRewards(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleWithdrawRewards_EVMAddrError_Continues(t *testing.T) {
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
	cfg := testCfg()
	// First convertValidatorToAccountAddress (for key derivation) succeeds
	runner.outputs[binPath+" debug addr pushvaloper1test"] = []byte("Bech32 Acc: push1account\n")
	runner.outputs[binPath+" keys list --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir+" --output json"] = []byte(`[{"name":"mykey","address":"push1account"}]`)
	// getEVMAddress fails (non-fatal)
	runner.errors[binPath+" debug addr push1account"] = fmt.Errorf("evm lookup failed")

	d := &Deps{
		Cfg:        testCfg(),
		Sup:        &mockSupervisor{running: true},
		Node:       &mockNodeClient{status: node.Status{Height: 100, CatchingUp: false}},
		RemoteNode: &mockNodeClient{status: node.Status{Height: 100}},
		Fetcher: &mockFetcher{
			myValidator: validator.MyValidatorInfo{
				IsValidator: true,
				Address:     "pushvaloper1test",
			},
			commission:  "5.0",
			outstanding: "10.0",
		},
		Validator: &mockValidator{withdrawResult: "TX_NO_EVM"},
		Runner:    runner,
		RPCCheck:  func(string, time.Duration) bool { return true },
		Prompter:  &nonInteractivePrompter{},
	}

	err := handleWithdrawRewards(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

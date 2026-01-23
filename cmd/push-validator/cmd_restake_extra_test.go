package main

import (
	"testing"
	"time"

	"github.com/pushchain/push-validator-cli/internal/node"
	"github.com/pushchain/push-validator-cli/internal/validator"
)

func TestHandleRestakeRewardsAll_FlagYes_FullSuccess(t *testing.T) {
	origOutput := flagOutput
	origNonInteractive := flagNonInteractive
	origYes := flagYes
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagOutput = origOutput
		flagNonInteractive = origNonInteractive
		flagYes = origYes
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagOutput = "text"
	flagNonInteractive = false // Not non-interactive
	flagYes = true             // But --yes is set - should skip prompts
	flagNoColor = true
	flagNoEmoji = true

	runner := newMockRunner()
	binPath := findPchaind()
	cfg := testCfg()
	runner.outputs[binPath+" debug addr pushvaloper1test"] = []byte("Bech32 Acc: push1account\nAddress (hex): AABB\n")
	runner.outputs[binPath+" keys list --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir+" --output json"] = []byte(`[{"name":"validator-key","address":"push1account"}]`)

	d := restakeDeps(func(d *Deps) {
		d.Node = &mockNodeClient{status: node.Status{Height: 100, CatchingUp: false}}
		d.RemoteNode = &mockNodeClient{status: node.Status{Height: 100}}
		d.Fetcher = &mockFetcher{
			myValidator: validator.MyValidatorInfo{
				IsValidator: true,
				Address:     "pushvaloper1test",
				Moniker:     "test",
			},
			commission:  "5.0",
			outstanding: "10.0",
		}
		d.Validator = &mockValidator{
			withdrawResult: "WITHDRAW_TX_YES",
			delegateResult: "DELEGATE_TX_YES",
		}
		d.Runner = runner
	})

	err := handleRestakeRewardsAll(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleRestakeRewardsAll_JSON_FullSuccess(t *testing.T) {
	origOutput := flagOutput
	origNonInteractive := flagNonInteractive
	origYes := flagYes
	defer func() {
		flagOutput = origOutput
		flagNonInteractive = origNonInteractive
		flagYes = origYes
	}()
	flagOutput = "json"
	flagNonInteractive = true
	flagYes = true

	runner := newMockRunner()
	binPath := findPchaind()
	cfg := testCfg()
	runner.outputs[binPath+" debug addr pushvaloper1test"] = []byte("Bech32 Acc: push1account\n")
	runner.outputs[binPath+" keys list --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir+" --output json"] = []byte(`[{"name":"mykey","address":"push1account"}]`)

	d := restakeDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{
			myValidator: validator.MyValidatorInfo{
				IsValidator: true,
				Address:     "pushvaloper1test",
			},
			commission:  "5.0",
			outstanding: "10.0",
		}
		d.Validator = &mockValidator{
			withdrawResult: "W_TX",
			delegateResult: "D_TX",
		}
		d.Runner = runner
	})

	err := handleRestakeRewardsAll(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleRestakeRewardsAll_JSON_InsufficientAfterGasReserve(t *testing.T) {
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

	d := restakeDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{
			myValidator: validator.MyValidatorInfo{
				IsValidator: true,
				Address:     "pushvaloper1test",
			},
			commission:  "0.05",
			outstanding: "0.08", // Total: 0.13 PC, below gas reserve of 0.15
		}
		d.Validator = &mockValidator{
			withdrawResult: "W_TX",
		}
		d.Runner = runner
	})

	err := handleRestakeRewardsAll(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "insufficient balance for restaking") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleRestakeRewardsAll_JSON_DelegationFails(t *testing.T) {
	origOutput := flagOutput
	origNonInteractive := flagNonInteractive
	origYes := flagYes
	defer func() {
		flagOutput = origOutput
		flagNonInteractive = origNonInteractive
		flagYes = origYes
	}()
	flagOutput = "json"
	flagNonInteractive = true
	flagYes = true

	runner := newMockRunner()
	binPath := findPchaind()
	cfg := testCfg()
	runner.outputs[binPath+" debug addr pushvaloper1test"] = []byte("Bech32 Acc: push1account\n")
	runner.outputs[binPath+" keys list --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir+" --output json"] = []byte(`[{"name":"mykey","address":"push1account"}]`)

	d := restakeDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{
			myValidator: validator.MyValidatorInfo{
				IsValidator: true,
				Address:     "pushvaloper1test",
			},
			commission:  "5.0",
			outstanding: "10.0",
		}
		d.Validator = &mockValidator{
			withdrawResult: "W_TX",
			delegateErr:    errMock,
		}
		d.Runner = runner
	})

	err := handleRestakeRewardsAll(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "restaking transaction failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleRestakeRewardsAll_JSON_NoSignificantRewards(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	d := restakeDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{
			myValidator: validator.MyValidatorInfo{
				IsValidator: true,
				Address:     "pushvaloper1test",
			},
			commission:  "0.001",
			outstanding: "0.002",
		}
	})

	err := handleRestakeRewardsAll(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleRestakeRewardsAll_KeyDerivationFallback(t *testing.T) {
	origOutput := flagOutput
	origNonInteractive := flagNonInteractive
	origYes := flagYes
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagOutput = origOutput
		flagNonInteractive = origNonInteractive
		flagYes = origYes
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagOutput = "text"
	flagNonInteractive = true
	flagYes = true
	flagNoColor = true
	flagNoEmoji = true

	runner := newMockRunner()
	binPath := findPchaind()
	// Addr conversion fails - should fallback to default key
	runner.errors[binPath+" debug addr pushvaloper1test"] = errMock

	d := restakeDeps(func(d *Deps) {
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
		d.Validator = &mockValidator{
			withdrawResult: "W_TX",
			delegateResult: "D_TX",
		}
		d.Runner = runner
	})

	err := handleRestakeRewardsAll(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleRestakeRewardsAll_EmptyAddress_FallbackToDefault(t *testing.T) {
	origOutput := flagOutput
	origNonInteractive := flagNonInteractive
	origYes := flagYes
	defer func() {
		flagOutput = origOutput
		flagNonInteractive = origNonInteractive
		flagYes = origYes
	}()
	flagOutput = "json"
	flagNonInteractive = true
	flagYes = true

	d := restakeDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{
			myValidator: validator.MyValidatorInfo{
				IsValidator: true,
				Address:     "", // Empty address
			},
			commission:  "5.0",
			outstanding: "10.0",
		}
		d.Validator = &mockValidator{
			withdrawResult: "W_TX",
			delegateResult: "D_TX",
		}
	})

	err := handleRestakeRewardsAll(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleRestakeRewardsAll_Interactive_ConfirmYes(t *testing.T) {
	origOutput := flagOutput
	origYes := flagYes
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagOutput = origOutput
		flagYes = origYes
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagOutput = "text"
	flagYes = false
	flagNoColor = true
	flagNoEmoji = true

	runner := newMockRunner()
	binPath := findPchaind()
	cfg := testCfg()
	runner.outputs[binPath+" debug addr pushvaloper1test"] = []byte("Bech32 Acc: push1account\n")
	runner.outputs[binPath+" keys list --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir+" --output json"] = []byte(`[{"name":"mykey","address":"push1account"}]`)

	d := restakeDeps(func(d *Deps) {
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
		d.Validator = &mockValidator{
			withdrawResult: "W_TX",
			delegateResult: "D_TX",
		}
		d.Runner = runner
		// Interactive mode: confirm with "y"
		d.Prompter = &mockPrompter{interactive: true, responses: []string{"y"}}
	})

	err := handleRestakeRewardsAll(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleRestakeRewardsAll_Interactive_Cancel(t *testing.T) {
	origOutput := flagOutput
	origYes := flagYes
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagOutput = origOutput
		flagYes = origYes
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagOutput = "text"
	flagYes = false
	flagNoColor = true
	flagNoEmoji = true

	runner := newMockRunner()
	binPath := findPchaind()
	cfg := testCfg()
	runner.outputs[binPath+" debug addr pushvaloper1test"] = []byte("Bech32 Acc: push1account\n")
	runner.outputs[binPath+" keys list --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir+" --output json"] = []byte(`[{"name":"mykey","address":"push1account"}]`)

	d := restakeDeps(func(d *Deps) {
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
		d.Validator = &mockValidator{withdrawResult: "W_TX"}
		d.Runner = runner
		// Interactive mode: cancel with "n"
		d.Prompter = &mockPrompter{interactive: true, responses: []string{"n"}}
	})

	err := handleRestakeRewardsAll(d)
	if err != nil {
		t.Fatalf("expected nil (cancelled), got: %v", err)
	}
}

func TestHandleRestakeRewardsAll_Interactive_EditAmount(t *testing.T) {
	origOutput := flagOutput
	origYes := flagYes
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagOutput = origOutput
		flagYes = origYes
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagOutput = "text"
	flagYes = false
	flagNoColor = true
	flagNoEmoji = true

	runner := newMockRunner()
	binPath := findPchaind()
	cfg := testCfg()
	runner.outputs[binPath+" debug addr pushvaloper1test"] = []byte("Bech32 Acc: push1account\n")
	runner.outputs[binPath+" keys list --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir+" --output json"] = []byte(`[{"name":"mykey","address":"push1account"}]`)

	d := restakeDeps(func(d *Deps) {
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
		d.Validator = &mockValidator{
			withdrawResult: "W_TX",
			delegateResult: "D_TX",
		}
		d.Runner = runner
		// Interactive mode: choose "edit", then enter custom amount "5.0"
		d.Prompter = &mockPrompter{interactive: true, responses: []string{"edit", "5.0"}}
	})

	err := handleRestakeRewardsAll(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleRestakeRewardsAll_Interactive_EditAmount_TooLow(t *testing.T) {
	origOutput := flagOutput
	origYes := flagYes
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagOutput = origOutput
		flagYes = origYes
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagOutput = "text"
	flagYes = false
	flagNoColor = true
	flagNoEmoji = true

	runner := newMockRunner()
	binPath := findPchaind()
	cfg := testCfg()
	runner.outputs[binPath+" debug addr pushvaloper1test"] = []byte("Bech32 Acc: push1account\n")
	runner.outputs[binPath+" keys list --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir+" --output json"] = []byte(`[{"name":"mykey","address":"push1account"}]`)

	d := restakeDeps(func(d *Deps) {
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
		d.Validator = &mockValidator{
			withdrawResult: "W_TX",
			delegateResult: "D_TX",
		}
		d.Runner = runner
		// Enter edit, too low amount, then valid amount
		d.Prompter = &mockPrompter{interactive: true, responses: []string{"edit", "0.001", "5.0"}}
	})

	err := handleRestakeRewardsAll(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleRestakeRewardsAll_Interactive_EditAmount_TooHigh(t *testing.T) {
	origOutput := flagOutput
	origYes := flagYes
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagOutput = origOutput
		flagYes = origYes
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagOutput = "text"
	flagYes = false
	flagNoColor = true
	flagNoEmoji = true

	runner := newMockRunner()
	binPath := findPchaind()
	cfg := testCfg()
	runner.outputs[binPath+" debug addr pushvaloper1test"] = []byte("Bech32 Acc: push1account\n")
	runner.outputs[binPath+" keys list --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir+" --output json"] = []byte(`[{"name":"mykey","address":"push1account"}]`)

	d := restakeDeps(func(d *Deps) {
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
		d.Validator = &mockValidator{
			withdrawResult: "W_TX",
			delegateResult: "D_TX",
		}
		d.Runner = runner
		// Enter edit, too high amount, then valid amount
		d.Prompter = &mockPrompter{interactive: true, responses: []string{"edit", "999.0", "5.0"}}
	})

	err := handleRestakeRewardsAll(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleRestakeRewardsAll_Interactive_EditAmount_InvalidInput(t *testing.T) {
	origOutput := flagOutput
	origYes := flagYes
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagOutput = origOutput
		flagYes = origYes
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagOutput = "text"
	flagYes = false
	flagNoColor = true
	flagNoEmoji = true

	runner := newMockRunner()
	binPath := findPchaind()
	cfg := testCfg()
	runner.outputs[binPath+" debug addr pushvaloper1test"] = []byte("Bech32 Acc: push1account\n")
	runner.outputs[binPath+" keys list --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir+" --output json"] = []byte(`[{"name":"mykey","address":"push1account"}]`)

	d := restakeDeps(func(d *Deps) {
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
		d.Validator = &mockValidator{
			withdrawResult: "W_TX",
			delegateResult: "D_TX",
		}
		d.Runner = runner
		// Enter edit, invalid input, empty input, then valid
		d.Prompter = &mockPrompter{interactive: true, responses: []string{"edit", "abc", "", "5.0"}}
	})

	err := handleRestakeRewardsAll(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleRestakeRewardsAll_Interactive_InvalidChoice(t *testing.T) {
	origOutput := flagOutput
	origYes := flagYes
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagOutput = origOutput
		flagYes = origYes
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagOutput = "text"
	flagYes = false
	flagNoColor = true
	flagNoEmoji = true

	runner := newMockRunner()
	binPath := findPchaind()
	cfg := testCfg()
	runner.outputs[binPath+" debug addr pushvaloper1test"] = []byte("Bech32 Acc: push1account\n")
	runner.outputs[binPath+" keys list --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir+" --output json"] = []byte(`[{"name":"mykey","address":"push1account"}]`)

	d := restakeDeps(func(d *Deps) {
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
		d.Validator = &mockValidator{withdrawResult: "W_TX"}
		d.Runner = runner
		// Invalid input (not y/n/edit) → cancelled
		d.Prompter = &mockPrompter{interactive: true, responses: []string{"xyz"}}
	})

	err := handleRestakeRewardsAll(d)
	if err == nil {
		t.Fatal("expected error for invalid input")
	}
	if !containsSubstr(err.Error(), "restaking cancelled by user") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleRestakeRewardsAll_Interactive_EmptyConfirm(t *testing.T) {
	origOutput := flagOutput
	origYes := flagYes
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagOutput = origOutput
		flagYes = origYes
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagOutput = "text"
	flagYes = false
	flagNoColor = true
	flagNoEmoji = true

	runner := newMockRunner()
	binPath := findPchaind()
	cfg := testCfg()
	runner.outputs[binPath+" debug addr pushvaloper1test"] = []byte("Bech32 Acc: push1account\n")
	runner.outputs[binPath+" keys list --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir+" --output json"] = []byte(`[{"name":"mykey","address":"push1account"}]`)

	d := restakeDeps(func(d *Deps) {
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
		d.Validator = &mockValidator{
			withdrawResult: "W_TX",
			delegateResult: "D_TX",
		}
		d.Runner = runner
		// Empty string → defaults to yes
		d.Prompter = &mockPrompter{interactive: true, responses: []string{""}}
	})

	err := handleRestakeRewardsAll(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleRestakeRewardsAll_KeyListFails_FallbackToDefault(t *testing.T) {
	origOutput := flagOutput
	origNonInteractive := flagNonInteractive
	origYes := flagYes
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagOutput = origOutput
		flagNonInteractive = origNonInteractive
		flagYes = origYes
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagOutput = "text"
	flagNonInteractive = true
	flagYes = true
	flagNoColor = true
	flagNoEmoji = true

	runner := newMockRunner()
	binPath := findPchaind()
	cfg := testCfg()
	runner.outputs[binPath+" debug addr pushvaloper1test"] = []byte("Bech32 Acc: push1account\n")
	// Keys list fails
	runner.errors[binPath+" keys list --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir+" --output json"] = errMock

	d := &Deps{
		Cfg:        cfg,
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
		Validator: &mockValidator{
			withdrawResult: "W_TX",
			delegateResult: "D_TX",
		},
		Runner:   runner,
		RPCCheck: func(string, time.Duration) bool { return true },
		Prompter: &nonInteractivePrompter{},
	}

	err := handleRestakeRewardsAll(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

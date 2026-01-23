package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/pushchain/push-validator-cli/internal/node"
	"github.com/pushchain/push-validator-cli/internal/validator"
)

func restakeDeps(overrides ...func(*Deps)) *Deps {
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

func TestHandleRestakeRewardsAll_SyncError(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	d := restakeDeps(func(d *Deps) {
		d.Node = &mockNodeClient{statusErr: fmt.Errorf("refused")}
	})

	err := handleRestakeRewardsAll(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "failed to check sync status") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleRestakeRewardsAll_StillSyncing(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	d := restakeDeps(func(d *Deps) {
		d.Node = &mockNodeClient{status: node.Status{CatchingUp: true}}
	})

	err := handleRestakeRewardsAll(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "node is still syncing") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleRestakeRewardsAll_NotValidator(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	d := restakeDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{myValidator: validator.MyValidatorInfo{IsValidator: false}}
	})

	err := handleRestakeRewardsAll(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "not registered as validator") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleRestakeRewardsAll_RewardsFetchError(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	d := restakeDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{
			myValidator: validator.MyValidatorInfo{IsValidator: true, Address: "pushvaloper1test"},
			rewardsErr:  fmt.Errorf("rpc error"),
		}
	})

	err := handleRestakeRewardsAll(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "failed to fetch rewards") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleRestakeRewardsAll_NoSignificantRewards(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	d := restakeDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{
			myValidator: validator.MyValidatorInfo{IsValidator: true, Address: "pushvaloper1test"},
			commission:  "0.001",
			outstanding: "0.005",
		}
	})

	err := handleRestakeRewardsAll(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleRestakeRewardsAll_WithdrawFails(t *testing.T) {
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
	flagOutput = "json"
	flagNonInteractive = true
	flagNoColor = true
	flagNoEmoji = true

	runner := newMockRunner()
	binPath := findPchaind()
	cfg := testCfg()
	runner.outputs[binPath+" debug addr pushvaloper1test"] = []byte("Bech32 Acc: push1account\n")
	runner.outputs[binPath+" keys list --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir+" --output json"] = []byte(`[{"name":"mykey","address":"push1account"}]`)

	d := restakeDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{
			myValidator: validator.MyValidatorInfo{IsValidator: true, Address: "pushvaloper1test"},
			commission:  "5.0",
			outstanding: "10.0",
		}
		d.Validator = &mockValidator{withdrawErr: fmt.Errorf("out of gas")}
		d.Runner = runner
	})

	err := handleRestakeRewardsAll(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "withdrawal transaction failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleRestakeRewardsAll_InsufficientAfterGasReserve(t *testing.T) {
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
	flagOutput = "json"
	flagNonInteractive = true
	flagYes = true
	flagNoColor = true
	flagNoEmoji = true

	runner := newMockRunner()
	binPath := findPchaind()
	cfg := testCfg()
	runner.outputs[binPath+" debug addr pushvaloper1test"] = []byte("Bech32 Acc: push1account\n")
	runner.outputs[binPath+" keys list --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir+" --output json"] = []byte(`[{"name":"mykey","address":"push1account"}]`)

	d := restakeDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{
			myValidator: validator.MyValidatorInfo{IsValidator: true, Address: "pushvaloper1test"},
			commission:  "0.05",  // total = 0.1 which is < 0.15 gas reserve
			outstanding: "0.05",
		}
		d.Validator = &mockValidator{withdrawResult: "TX_WITHDRAW"}
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

func TestHandleRestakeRewardsAll_NonInteractive_Success(t *testing.T) {
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
	runner.outputs[binPath+" keys list --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir+" --output json"] = []byte(`[{"name":"mykey","address":"push1account"}]`)

	d := restakeDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{
			myValidator: validator.MyValidatorInfo{IsValidator: true, Address: "pushvaloper1test"},
			commission:  "5.0",
			outstanding: "10.0",
		}
		d.Validator = &mockValidator{
			withdrawResult: "TX_WITHDRAW_123",
			delegateResult: "TX_DELEGATE_456",
		}
		d.Runner = runner
	})

	err := handleRestakeRewardsAll(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleRestakeRewardsAll_DelegationFails(t *testing.T) {
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
	flagOutput = "json"
	flagNonInteractive = true
	flagYes = true
	flagNoColor = true
	flagNoEmoji = true

	runner := newMockRunner()
	binPath := findPchaind()
	cfg := testCfg()
	runner.outputs[binPath+" debug addr pushvaloper1test"] = []byte("Bech32 Acc: push1account\n")
	runner.outputs[binPath+" keys list --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir+" --output json"] = []byte(`[{"name":"mykey","address":"push1account"}]`)

	d := restakeDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{
			myValidator: validator.MyValidatorInfo{IsValidator: true, Address: "pushvaloper1test"},
			commission:  "5.0",
			outstanding: "10.0",
		}
		d.Validator = &mockValidator{
			withdrawResult: "TX_WITHDRAW",
			delegateErr:    fmt.Errorf("delegation failed"),
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

func TestHandleRestakeRewardsAll_FetcherError(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	d := restakeDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{myValidatorErr: fmt.Errorf("timeout")}
	})

	err := handleRestakeRewardsAll(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "failed to check validator status") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleRestakeRewardsAll_TextOutput_SyncError(t *testing.T) {
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

	d := restakeDeps(func(d *Deps) {
		d.Node = &mockNodeClient{statusErr: fmt.Errorf("refused")}
	})

	err := handleRestakeRewardsAll(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "failed to check sync status") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleRestakeRewardsAll_TextOutput_StillSyncing(t *testing.T) {
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

	d := restakeDeps(func(d *Deps) {
		d.Node = &mockNodeClient{status: node.Status{CatchingUp: true}}
	})

	err := handleRestakeRewardsAll(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "node is still syncing") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleRestakeRewardsAll_TextOutput_NotValidator(t *testing.T) {
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

	d := restakeDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{myValidator: validator.MyValidatorInfo{IsValidator: false}}
	})

	err := handleRestakeRewardsAll(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "not registered as validator") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleRestakeRewardsAll_TextOutput_FetcherError(t *testing.T) {
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

	d := restakeDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{myValidatorErr: fmt.Errorf("timeout")}
	})

	err := handleRestakeRewardsAll(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "failed to check validator status") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleRestakeRewardsAll_TextOutput_RewardsFetchError(t *testing.T) {
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

	d := restakeDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{
			myValidator: validator.MyValidatorInfo{IsValidator: true, Address: "pushvaloper1test"},
			rewardsErr:  fmt.Errorf("rpc error"),
		}
	})

	err := handleRestakeRewardsAll(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "failed to fetch rewards") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleRestakeRewardsAll_TextOutput_NoSignificantRewards(t *testing.T) {
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

	d := restakeDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{
			myValidator: validator.MyValidatorInfo{IsValidator: true, Address: "pushvaloper1test"},
			commission:  "0.001",
			outstanding: "0.005",
		}
	})

	err := handleRestakeRewardsAll(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleRestakeRewardsAll_TextOutput_WithdrawFails(t *testing.T) {
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
	runner.outputs[binPath+" debug addr pushvaloper1test"] = []byte("Bech32 Acc: push1account\n")
	runner.outputs[binPath+" keys list --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir+" --output json"] = []byte(`[{"name":"mykey","address":"push1account"}]`)

	d := restakeDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{
			myValidator: validator.MyValidatorInfo{IsValidator: true, Address: "pushvaloper1test"},
			commission:  "5.0",
			outstanding: "10.0",
		}
		d.Validator = &mockValidator{withdrawErr: fmt.Errorf("out of gas")}
		d.Runner = runner
	})

	err := handleRestakeRewardsAll(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "withdrawal transaction failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleRestakeRewardsAll_TextOutput_DelegationFails(t *testing.T) {
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
	runner.outputs[binPath+" keys list --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir+" --output json"] = []byte(`[{"name":"mykey","address":"push1account"}]`)

	d := restakeDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{
			myValidator: validator.MyValidatorInfo{IsValidator: true, Address: "pushvaloper1test"},
			commission:  "5.0",
			outstanding: "10.0",
		}
		d.Validator = &mockValidator{
			withdrawResult: "TX_WITHDRAW",
			delegateErr:    fmt.Errorf("delegation failed"),
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

func TestHandleRestakeRewardsAll_TextOutput_InsufficientAfterGasReserve(t *testing.T) {
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
	runner.outputs[binPath+" keys list --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir+" --output json"] = []byte(`[{"name":"mykey","address":"push1account"}]`)

	d := restakeDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{
			myValidator: validator.MyValidatorInfo{IsValidator: true, Address: "pushvaloper1test"},
			commission:  "0.05",
			outstanding: "0.05",
		}
		d.Validator = &mockValidator{withdrawResult: "TX_WITHDRAW"}
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

func TestHandleRestakeRewardsAll_NonInteractive_FullSuccess(t *testing.T) {
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
			withdrawResult: "WITHDRAW_TX",
			delegateResult: "DELEGATE_TX",
			balanceResult:  "5000000000000000000",
		}
		d.Runner = runner
	})

	err := handleRestakeRewardsAll(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleRestakeRewardsAll_Text_FullSuccess(t *testing.T) {
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
			withdrawResult: "WITHDRAW_TX",
			delegateResult: "DELEGATE_TX",
			balanceResult:  "5000000000000000000",
		}
		d.Runner = runner
	})

	err := handleRestakeRewardsAll(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

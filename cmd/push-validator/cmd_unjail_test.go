package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/pushchain/push-validator-cli/internal/node"
	"github.com/pushchain/push-validator-cli/internal/validator"
)

func unjailDeps(overrides ...func(*Deps)) *Deps {
	d := &Deps{
		Cfg:        testCfg(),
		Sup:        &mockSupervisor{running: true},
		Node:       &mockNodeClient{},
		RemoteNode: &mockNodeClient{},
		Fetcher:    &mockFetcher{},
		Validator:  &mockValidator{},
		Runner:     newMockRunner(),
		RPCCheck:   func(string, time.Duration) bool { return true },
	}
	for _, fn := range overrides {
		fn(d)
	}
	return d
}

func TestHandleUnjail_SyncError(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	d := unjailDeps(func(d *Deps) {
		d.Node = &mockNodeClient{statusErr: fmt.Errorf("connection refused")}
	})

	err := handleUnjail(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "failed to check sync status") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleUnjail_StillSyncing(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	d := unjailDeps(func(d *Deps) {
		d.Node = &mockNodeClient{status: node.Status{CatchingUp: true}}
	})

	err := handleUnjail(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "node is still syncing") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleUnjail_FetcherError(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	d := unjailDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{myValidatorErr: fmt.Errorf("timeout")}
	})

	err := handleUnjail(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "failed to check validator status") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleUnjail_NotValidator(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	d := unjailDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{myValidator: validator.MyValidatorInfo{IsValidator: false}}
	})

	err := handleUnjail(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "not registered as validator") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleUnjail_NotJailed(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	d := unjailDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{myValidator: validator.MyValidatorInfo{
			IsValidator: true,
			Jailed:      false,
			Status:      "BONDED",
		}}
	})

	err := handleUnjail(d)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestHandleUnjail_JailPeriodNotExpired(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	futureTime := time.Now().Add(1 * time.Hour).Format(time.RFC3339Nano)
	d := unjailDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{myValidator: validator.MyValidatorInfo{
			IsValidator: true,
			Jailed:      true,
			SlashingInfo: validator.SlashingInfo{
				JailedUntil: futureTime,
			},
		}}
	})

	err := handleUnjail(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "jail period has not expired") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleUnjail_EmptyJailedUntil(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	d := unjailDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{myValidator: validator.MyValidatorInfo{
			IsValidator: true,
			Jailed:      true,
			SlashingInfo: validator.SlashingInfo{
				JailedUntil: "",
			},
		}}
	})

	err := handleUnjail(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "could not determine jail period") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleUnjail_NonInteractive_Success(t *testing.T) {
	origOutput := flagOutput
	origNonInteractive := flagNonInteractive
	defer func() {
		flagOutput = origOutput
		flagNonInteractive = origNonInteractive
	}()
	flagOutput = "json"
	flagNonInteractive = true

	pastTime := time.Now().Add(-1 * time.Hour).Format(time.RFC3339Nano)
	runner := newMockRunner()
	binPath := findPchaind()
	// Mock convertValidatorToAccountAddress
	runner.outputs[binPath+" debug addr pushvaloper1test"] = []byte("Bech32 Acc: push1account\n")
	// Mock getEVMAddress
	runner.outputs[binPath+" debug addr push1account"] = []byte("Address (hex): AABB1234\n")
	// Mock findKeyNameByAddress
	cfg := testCfg()
	runner.outputs[binPath+" keys list --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir+" --output json"] = []byte(`[{"name":"mykey","address":"push1account"}]`)

	d := unjailDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{myValidator: validator.MyValidatorInfo{
			IsValidator: true,
			Address:     "pushvaloper1test",
			Jailed:      true,
			SlashingInfo: validator.SlashingInfo{
				JailedUntil: pastTime,
			},
		}}
		d.Validator = &mockValidator{unjailResult: "TX_HASH_123"}
		d.Runner = runner
	})

	err := handleUnjail(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleUnjail_NonInteractive_UnjailFails(t *testing.T) {
	origOutput := flagOutput
	origNonInteractive := flagNonInteractive
	defer func() {
		flagOutput = origOutput
		flagNonInteractive = origNonInteractive
	}()
	flagOutput = "json"
	flagNonInteractive = true

	pastTime := time.Now().Add(-1 * time.Hour).Format(time.RFC3339Nano)
	runner := newMockRunner()
	binPath := findPchaind()
	runner.outputs[binPath+" debug addr pushvaloper1test"] = []byte("Bech32 Acc: push1account\n")
	runner.outputs[binPath+" debug addr push1account"] = []byte("Address (hex): AABB\n")
	cfg := testCfg()
	runner.outputs[binPath+" keys list --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir+" --output json"] = []byte(`[{"name":"mykey","address":"push1account"}]`)

	d := unjailDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{myValidator: validator.MyValidatorInfo{
			IsValidator: true,
			Address:     "pushvaloper1test",
			Jailed:      true,
			SlashingInfo: validator.SlashingInfo{
				JailedUntil: pastTime,
			},
		}}
		d.Validator = &mockValidator{unjailErr: fmt.Errorf("insufficient gas")}
		d.Runner = runner
	})

	err := handleUnjail(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "unjail transaction failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleUnjail_AddressConversionFails(t *testing.T) {
	origOutput := flagOutput
	origNonInteractive := flagNonInteractive
	defer func() {
		flagOutput = origOutput
		flagNonInteractive = origNonInteractive
	}()
	flagOutput = "json"
	flagNonInteractive = true

	pastTime := time.Now().Add(-1 * time.Hour).Format(time.RFC3339Nano)
	runner := newMockRunner()
	binPath := findPchaind()
	// convertValidatorToAccountAddress for key derivation fails (falls back to default)
	runner.errors[binPath+" debug addr pushvaloper1test"] = fmt.Errorf("binary not found")

	d := unjailDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{myValidator: validator.MyValidatorInfo{
			IsValidator: true,
			Address:     "pushvaloper1test",
			Jailed:      true,
			SlashingInfo: validator.SlashingInfo{
				JailedUntil: pastTime,
			},
		}}
		d.Runner = runner
	})

	err := handleUnjail(d)
	if err == nil {
		t.Fatal("expected error")
	}
	// Should fail at the balance check step (second convertValidatorToAccountAddress call)
	if !containsSubstr(err.Error(), "failed to derive account address") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleUnjail_TextOutput_SyncError(t *testing.T) {
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

	d := unjailDeps(func(d *Deps) {
		d.Node = &mockNodeClient{statusErr: fmt.Errorf("connection refused")}
	})

	err := handleUnjail(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "failed to check sync status") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleUnjail_TextOutput_StillSyncing(t *testing.T) {
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

	d := unjailDeps(func(d *Deps) {
		d.Node = &mockNodeClient{status: node.Status{CatchingUp: true}}
	})

	err := handleUnjail(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "node is still syncing") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleUnjail_TextOutput_FetcherError(t *testing.T) {
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

	d := unjailDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{myValidatorErr: fmt.Errorf("timeout")}
	})

	err := handleUnjail(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "failed to check validator status") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleUnjail_TextOutput_NotValidator(t *testing.T) {
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

	d := unjailDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{myValidator: validator.MyValidatorInfo{IsValidator: false}}
	})

	err := handleUnjail(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "not registered as validator") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleUnjail_TextOutput_JailPeriodNotExpired(t *testing.T) {
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

	futureTime := time.Now().Add(1 * time.Hour).Format(time.RFC3339Nano)
	d := unjailDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{myValidator: validator.MyValidatorInfo{
			IsValidator: true,
			Jailed:      true,
			SlashingInfo: validator.SlashingInfo{
				JailedUntil: futureTime,
			},
		}}
	})

	err := handleUnjail(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "jail period has not expired") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleUnjail_TextOutput_NotJailed(t *testing.T) {
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

	d := unjailDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{myValidator: validator.MyValidatorInfo{
			IsValidator: true,
			Jailed:      false,
			Status:      "BONDED",
		}}
	})

	err := handleUnjail(d)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestHandleUnjail_NonInteractive_SuccessfulUnjail_Text(t *testing.T) {
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

	d := unjailDeps(func(d *Deps) {
		d.Node = &mockNodeClient{status: node.Status{Height: 100, CatchingUp: false}}
		d.RemoteNode = &mockNodeClient{status: node.Status{Height: 100}}
		d.Fetcher = &mockFetcher{myValidator: validator.MyValidatorInfo{
			IsValidator: true,
			Address:     "pushvaloper1test",
			Jailed:      true,
			SlashingInfo: validator.SlashingInfo{
				JailedUntil: "2020-01-01T00:00:00Z",
			},
		}}
		d.Validator = &mockValidator{unjailResult: "TXHASH_UNJAIL"}
		d.Runner = runner
	})

	err := handleUnjail(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleUnjail_TextOutput_UnjailFails(t *testing.T) {
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

	d := unjailDeps(func(d *Deps) {
		d.Node = &mockNodeClient{status: node.Status{Height: 100, CatchingUp: false}}
		d.RemoteNode = &mockNodeClient{status: node.Status{Height: 100}}
		d.Fetcher = &mockFetcher{myValidator: validator.MyValidatorInfo{
			IsValidator: true,
			Address:     "pushvaloper1test",
			Jailed:      true,
			SlashingInfo: validator.SlashingInfo{
				JailedUntil: "2020-01-01T00:00:00Z",
			},
		}}
		d.Validator = &mockValidator{unjailErr: fmt.Errorf("insufficient gas")}
		d.Runner = runner
	})

	err := handleUnjail(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "unjail transaction failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleUnjail_TextOutput_EmptyJailedUntil(t *testing.T) {
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

	d := unjailDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{myValidator: validator.MyValidatorInfo{
			IsValidator: true,
			Jailed:      true,
			SlashingInfo: validator.SlashingInfo{
				JailedUntil: "",
			},
		}}
	})

	err := handleUnjail(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "could not determine jail period") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleUnjail_TextOutput_JailedButPeriodExpired(t *testing.T) {
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

	d := unjailDeps(func(d *Deps) {
		d.Node = &mockNodeClient{status: node.Status{Height: 100, CatchingUp: false}}
		d.RemoteNode = &mockNodeClient{status: node.Status{Height: 100}}
		d.Fetcher = &mockFetcher{myValidator: validator.MyValidatorInfo{
			IsValidator: true,
			Address:     "pushvaloper1test",
			Jailed:      true,
			SlashingInfo: validator.SlashingInfo{
				JailedUntil: "2020-01-01T00:00:00Z",
			},
		}}
		d.Validator = &mockValidator{unjailResult: "TXHASH_UNJAIL"}
		d.Runner = runner
	})

	err := handleUnjail(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}


package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/pushchain/push-validator-cli/internal/node"
	"github.com/pushchain/push-validator-cli/internal/validator"
)

// Tests for runRegisterValidatorWithDeps covering the full registration flow.

func TestHandleRegisterValidator_NodeSyncing_JSON(t *testing.T) {
	origOutput := flagOutput
	origNonInteractive := flagNonInteractive
	defer func() {
		flagOutput = origOutput
		flagNonInteractive = origNonInteractive
	}()
	flagOutput = "json"
	flagNonInteractive = true

	d := registerDeps(func(d *Deps) {
		d.Node = &mockNodeClient{status: node.Status{CatchingUp: true}}
		d.RemoteNode = &mockNodeClient{}
	})

	err := handleRegisterValidator(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "node is still syncing") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleRegisterValidator_NodeSyncing_Text(t *testing.T) {
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

	d := registerDeps(func(d *Deps) {
		d.Node = &mockNodeClient{status: node.Status{CatchingUp: true}}
		d.RemoteNode = &mockNodeClient{}
	})

	err := handleRegisterValidator(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "node is still syncing") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunRegisterValidatorWithDeps_EnsureKeyError_JSON(t *testing.T) {
	origOutput := flagOutput
	origNonInteractive := flagNonInteractive
	defer func() {
		flagOutput = origOutput
		flagNonInteractive = origNonInteractive
	}()
	flagOutput = "json"
	flagNonInteractive = true

	d := registerDeps(func(d *Deps) {
		d.Node = &mockNodeClient{status: node.Status{CatchingUp: false}}
		d.RemoteNode = &mockNodeClient{}
		d.Validator = &mockValidator{ensureKeyErr: fmt.Errorf("keyring locked")}
	})

	err := runRegisterValidatorWithDeps(d, d.Cfg, "myval", "mykey", "1500000000000000000", "0.10", "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "key error") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunRegisterValidatorWithDeps_EnsureKeyError_Text(t *testing.T) {
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

	d := registerDeps(func(d *Deps) {
		d.Node = &mockNodeClient{status: node.Status{CatchingUp: false}}
		d.RemoteNode = &mockNodeClient{}
		d.Validator = &mockValidator{ensureKeyErr: fmt.Errorf("keyring locked")}
	})

	err := runRegisterValidatorWithDeps(d, d.Cfg, "myval", "mykey", "1500000000000000000", "0.10", "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "key error") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunRegisterValidatorWithDeps_ImportKeyError_JSON(t *testing.T) {
	origOutput := flagOutput
	origNonInteractive := flagNonInteractive
	defer func() {
		flagOutput = origOutput
		flagNonInteractive = origNonInteractive
	}()
	flagOutput = "json"
	flagNonInteractive = true

	d := registerDeps(func(d *Deps) {
		d.Node = &mockNodeClient{status: node.Status{CatchingUp: false}}
		d.RemoteNode = &mockNodeClient{}
		d.Validator = &mockValidator{importKeyErr: fmt.Errorf("invalid mnemonic")}
	})

	err := runRegisterValidatorWithDeps(d, d.Cfg, "myval", "mykey", "1500000000000000000", "0.10", "word1 word2 word3 word4 word5 word6 word7 word8 word9 word10 word11 word12")
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "failed to import wallet") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunRegisterValidatorWithDeps_ImportKeyError_Text(t *testing.T) {
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

	d := registerDeps(func(d *Deps) {
		d.Node = &mockNodeClient{status: node.Status{CatchingUp: false}}
		d.RemoteNode = &mockNodeClient{}
		d.Validator = &mockValidator{importKeyErr: fmt.Errorf("bad mnemonic format")}
	})

	err := runRegisterValidatorWithDeps(d, d.Cfg, "myval", "mykey", "1500000000000000000", "0.10", "some bad mnemonic")
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "failed to import wallet") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunRegisterValidatorWithDeps_RegistrationSuccess_JSON(t *testing.T) {
	origOutput := flagOutput
	origNonInteractive := flagNonInteractive
	defer func() {
		flagOutput = origOutput
		flagNonInteractive = origNonInteractive
	}()
	flagOutput = "json"
	flagNonInteractive = true

	d := registerDeps(func(d *Deps) {
		d.Node = &mockNodeClient{status: node.Status{CatchingUp: false}}
		d.RemoteNode = &mockNodeClient{}
		d.Validator = &mockValidator{
			ensureKeyResult: validator.KeyInfo{Name: "mykey", Address: "push1test"},
			balanceResult:   "2000000000000000000", // 2 PC (enough)
			registerResult:  "TXHASH_REGISTER_123",
			evmAddrResult:   "0xABCD1234",
		}
	})

	err := runRegisterValidatorWithDeps(d, d.Cfg, "myval", "mykey", "1500000000000000000", "0.10", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunRegisterValidatorWithDeps_RegistrationSuccess_Text(t *testing.T) {
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

	d := registerDeps(func(d *Deps) {
		d.Node = &mockNodeClient{status: node.Status{CatchingUp: false}}
		d.RemoteNode = &mockNodeClient{}
		d.Validator = &mockValidator{
			ensureKeyResult: validator.KeyInfo{Name: "mykey", Address: "push1test"},
			balanceResult:   "2000000000000000000",
			registerResult:  "TXHASH_REG",
			evmAddrResult:   "0x1234",
		}
	})

	err := runRegisterValidatorWithDeps(d, d.Cfg, "myval", "mykey", "1500000000000000000", "0.10", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunRegisterValidatorWithDeps_RegistrationError_JSON(t *testing.T) {
	origOutput := flagOutput
	origNonInteractive := flagNonInteractive
	defer func() {
		flagOutput = origOutput
		flagNonInteractive = origNonInteractive
	}()
	flagOutput = "json"
	flagNonInteractive = true

	d := registerDeps(func(d *Deps) {
		d.Node = &mockNodeClient{status: node.Status{CatchingUp: false}}
		d.RemoteNode = &mockNodeClient{}
		d.Validator = &mockValidator{
			ensureKeyResult: validator.KeyInfo{Name: "mykey", Address: "push1test"},
			balanceResult:   "2000000000000000000",
			registerErr:     fmt.Errorf("out of gas"),
			evmAddrResult:   "0xABC",
		}
	})

	err := runRegisterValidatorWithDeps(d, d.Cfg, "myval", "mykey", "1500000000000000000", "0.10", "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "validator registration failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunRegisterValidatorWithDeps_RegistrationError_ValidatorAlreadyExists(t *testing.T) {
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

	d := registerDeps(func(d *Deps) {
		d.Node = &mockNodeClient{status: node.Status{CatchingUp: false}}
		d.RemoteNode = &mockNodeClient{}
		d.Validator = &mockValidator{
			ensureKeyResult: validator.KeyInfo{Name: "mykey", Address: "push1test"},
			balanceResult:   "2000000000000000000",
			registerErr:     fmt.Errorf("validator already exist for this pubkey"),
			evmAddrResult:   "0xABC",
		}
	})

	// "validator already exist" is now treated as success (validator is already registered)
	err := runRegisterValidatorWithDeps(d, d.Cfg, "myval", "mykey", "1500000000000000000", "0.10", "")
	if err != nil {
		t.Fatalf("expected nil (validator already registered treated as success), got: %v", err)
	}
}

func TestRunRegisterValidatorWithDeps_RegistrationError_Text(t *testing.T) {
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

	d := registerDeps(func(d *Deps) {
		d.Node = &mockNodeClient{status: node.Status{CatchingUp: false}}
		d.RemoteNode = &mockNodeClient{}
		d.Validator = &mockValidator{
			ensureKeyResult: validator.KeyInfo{Name: "mykey", Address: "push1test"},
			balanceResult:   "2000000000000000000",
			registerErr:     fmt.Errorf("insufficient funds"),
			evmAddrResult:   "0xABC",
		}
	})

	err := runRegisterValidatorWithDeps(d, d.Cfg, "myval", "mykey", "1500000000000000000", "0.10", "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRunRegisterValidatorWithDeps_ImportKeySuccess_JSON(t *testing.T) {
	origOutput := flagOutput
	origNonInteractive := flagNonInteractive
	defer func() {
		flagOutput = origOutput
		flagNonInteractive = origNonInteractive
	}()
	flagOutput = "json"
	flagNonInteractive = true

	d := registerDeps(func(d *Deps) {
		d.Node = &mockNodeClient{status: node.Status{CatchingUp: false}}
		d.RemoteNode = &mockNodeClient{}
		d.Validator = &mockValidator{
			importKeyResult: validator.KeyInfo{Name: "imported-key", Address: "push1imported"},
			balanceResult:   "2000000000000000000",
			registerResult:  "TXHASH_IMPORT_REG",
			evmAddrResult:   "0xIMPORTED",
		}
	})

	err := runRegisterValidatorWithDeps(d, d.Cfg, "myval", "imported-key", "1500000000000000000", "0.10", "word1 word2 word3 word4 word5 word6 word7 word8 word9 word10 word11 word12")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunRegisterValidatorWithDeps_ImportKeySuccess_Text_WithMnemonic(t *testing.T) {
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

	d := registerDeps(func(d *Deps) {
		d.Node = &mockNodeClient{status: node.Status{CatchingUp: false}}
		d.RemoteNode = &mockNodeClient{}
		d.Validator = &mockValidator{
			importKeyResult: validator.KeyInfo{Name: "imported-key", Address: "push1imported"},
			balanceResult:   "2000000000000000000",
			registerResult:  "TXHASH_IMPORT",
			evmAddrResult:   "0xIMPORTED",
		}
	})

	err := runRegisterValidatorWithDeps(d, d.Cfg, "myval", "imported-key", "1500000000000000000", "0.10", "word1 word2 word3 word4 word5 word6 word7 word8 word9 word10 word11 word12")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunRegisterValidatorWithDeps_NewKeyWithMnemonic_Text(t *testing.T) {
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

	d := registerDeps(func(d *Deps) {
		d.Node = &mockNodeClient{status: node.Status{CatchingUp: false}}
		d.RemoteNode = &mockNodeClient{}
		d.Validator = &mockValidator{
			ensureKeyResult: validator.KeyInfo{
				Name:     "newkey",
				Address:  "push1new",
				Mnemonic: "word1 word2 word3 word4 word5 word6 word7 word8 word9 word10 word11 word12",
			},
			balanceResult:  "2000000000000000000",
			registerResult: "TXHASH_NEW",
			evmAddrResult:  "0xNEW",
		}
	})

	err := runRegisterValidatorWithDeps(d, d.Cfg, "myval", "newkey", "1500000000000000000", "0.10", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunRegisterValidatorWithDeps_ExistingKey_Text(t *testing.T) {
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

	d := registerDeps(func(d *Deps) {
		d.Node = &mockNodeClient{status: node.Status{CatchingUp: false}}
		d.RemoteNode = &mockNodeClient{}
		d.Validator = &mockValidator{
			ensureKeyResult: validator.KeyInfo{Name: "existing", Address: "push1existing"},
			balanceResult:   "2000000000000000000",
			registerResult:  "TXHASH_EXISTING",
			evmAddrResult:   "0xEXISTING",
		}
	})

	err := runRegisterValidatorWithDeps(d, d.Cfg, "myval", "existing", "1500000000000000000", "0.10", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunRegisterValidatorWithDeps_NodeErrors_SkipsSyncCheck(t *testing.T) {
	origOutput := flagOutput
	origNonInteractive := flagNonInteractive
	defer func() {
		flagOutput = origOutput
		flagNonInteractive = origNonInteractive
	}()
	flagOutput = "json"
	flagNonInteractive = true

	// When both node status calls fail, the sync check is skipped
	d := registerDeps(func(d *Deps) {
		d.Node = &mockNodeClient{statusErr: fmt.Errorf("connection refused")}
		d.RemoteNode = &mockNodeClient{statusErr: fmt.Errorf("connection refused")}
		d.Validator = &mockValidator{
			ensureKeyResult: validator.KeyInfo{Name: "mykey", Address: "push1test"},
			balanceResult:   "2000000000000000000",
			registerResult:  "TXHASH_SKIP_SYNC",
			evmAddrResult:   "0x123",
		}
	})

	err := runRegisterValidatorWithDeps(d, d.Cfg, "myval", "mykey", "1500000000000000000", "0.10", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunRegisterValidatorWithDeps_EVMAddrError(t *testing.T) {
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

	// EVM address error is non-fatal
	d := registerDeps(func(d *Deps) {
		d.Node = &mockNodeClient{status: node.Status{CatchingUp: false}}
		d.RemoteNode = &mockNodeClient{}
		d.Validator = &mockValidator{
			ensureKeyResult: validator.KeyInfo{Name: "mykey", Address: "push1test"},
			balanceResult:   "2000000000000000000",
			registerResult:  "TXHASH_NO_EVM",
			evmAddrErr:      fmt.Errorf("evm lookup failed"),
		}
	})

	err := runRegisterValidatorWithDeps(d, d.Cfg, "myval", "mykey", "1500000000000000000", "0.10", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunRegisterValidatorWithDeps_DefaultStake_JSON(t *testing.T) {
	origOutput := flagOutput
	origNonInteractive := flagNonInteractive
	defer func() {
		flagOutput = origOutput
		flagNonInteractive = origNonInteractive
	}()
	flagOutput = "json"
	flagNonInteractive = true

	d := registerDeps(func(d *Deps) {
		d.Node = &mockNodeClient{status: node.Status{CatchingUp: false}}
		d.RemoteNode = &mockNodeClient{}
		d.Validator = &mockValidator{
			ensureKeyResult: validator.KeyInfo{Name: "mykey", Address: "push1test"},
			balanceResult:   "5000000000000000000", // 5 PC
			registerResult:  "TXHASH_DEFAULT_STAKE",
			evmAddrResult:   "0xDEF",
		}
	})

	// Empty stake amount should default to registrationMinStake when in JSON mode
	err := runRegisterValidatorWithDeps(d, d.Cfg, "myval", "mykey", "", "0.10", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestKeyExistsWithRunner_KeyFound(t *testing.T) {
	cfg := testCfg()
	runner := newMockRunner()
	binPath := findPchaind()

	key := binPath + " keys show test-key -a --keyring-backend " + cfg.KeyringBackend + " --home " + cfg.HomeDir
	runner.outputs[key] = []byte("push1testaddr\n")

	result := keyExistsWithRunner(cfg, "test-key", runner)
	if !result {
		t.Error("expected key to exist")
	}
}

func TestKeyExistsWithRunner_KeyNotFound(t *testing.T) {
	cfg := testCfg()
	runner := newMockRunner()
	binPath := findPchaind()

	key := binPath + " keys show test-key -a --keyring-backend " + cfg.KeyringBackend + " --home " + cfg.HomeDir
	runner.errors[key] = fmt.Errorf("key not found")

	result := keyExistsWithRunner(cfg, "test-key", runner)
	if result {
		t.Error("expected key to not exist")
	}
}


func TestRunRegisterValidatorWithDeps_BalanceCheckRetries(t *testing.T) {
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

	// Balance starts insufficient, then becomes sufficient
	callCount := 0
	mv := &mockValidator{
		ensureKeyResult: validator.KeyInfo{Name: "mykey", Address: "push1test"},
		registerResult:  "TXHASH_RETRY",
		evmAddrResult:   "0xABC",
	}
	// Override Balance to return different values on subsequent calls
	d := &Deps{
		Cfg:        testCfg(),
		Sup:        &mockSupervisor{running: true},
		Node:       &mockNodeClient{status: node.Status{CatchingUp: false}},
		RemoteNode: &mockNodeClient{},
		Fetcher:    &mockFetcher{},
		Validator: &balanceRetryMockValidator{
			inner:     mv,
			callCount: &callCount,
		},
		Runner:   newMockRunner(),
		Prompter: &nonInteractivePrompter{},
		RPCCheck: func(string, time.Duration) bool { return true },
	}

	err := runRegisterValidatorWithDeps(d, d.Cfg, "myval", "mykey", "1500000000000000000", "0.10", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// balanceRetryMockValidator returns insufficient balance on first call, then sufficient.
type balanceRetryMockValidator struct {
	inner     *mockValidator
	callCount *int
}

func (m *balanceRetryMockValidator) Balance(ctx context.Context, addr string) (string, error) {
	*m.callCount++
	if *m.callCount <= 1 {
		return "100000000000000000", nil // 0.1 PC - insufficient
	}
	return "2000000000000000000", nil // 2 PC - sufficient
}

func (m *balanceRetryMockValidator) IsValidator(ctx context.Context, addr string) (bool, error) {
	return m.inner.IsValidator(ctx, addr)
}

func (m *balanceRetryMockValidator) Register(ctx context.Context, args validator.RegisterArgs) (string, error) {
	return m.inner.Register(ctx, args)
}

func (m *balanceRetryMockValidator) Unjail(ctx context.Context, keyName string) (string, error) {
	return m.inner.Unjail(ctx, keyName)
}

func (m *balanceRetryMockValidator) WithdrawRewards(ctx context.Context, validatorAddr string, keyName string, includeCommission bool) (string, error) {
	return m.inner.WithdrawRewards(ctx, validatorAddr, keyName, includeCommission)
}

func (m *balanceRetryMockValidator) Delegate(ctx context.Context, args validator.DelegateArgs) (string, error) {
	return m.inner.Delegate(ctx, args)
}

func (m *balanceRetryMockValidator) EnsureKey(ctx context.Context, name string) (validator.KeyInfo, error) {
	return m.inner.EnsureKey(ctx, name)
}

func (m *balanceRetryMockValidator) ImportKey(ctx context.Context, name string, mnemonic string) (validator.KeyInfo, error) {
	return m.inner.ImportKey(ctx, name, mnemonic)
}

func (m *balanceRetryMockValidator) GetEVMAddress(ctx context.Context, addr string) (string, error) {
	return m.inner.GetEVMAddress(ctx, addr)
}

func (m *balanceRetryMockValidator) IsAddressValidator(ctx context.Context, cosmosAddr string) (bool, error) {
	return m.inner.IsAddressValidator(ctx, cosmosAddr)
}

func TestRunRegisterValidatorWithDeps_ValidatorAlreadyExists_ReturnsSuccess(t *testing.T) {
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

	d := registerDeps(func(d *Deps) {
		d.Node = &mockNodeClient{status: node.Status{CatchingUp: false}}
		d.RemoteNode = &mockNodeClient{}
		d.Validator = &mockValidator{
			importKeyResult: validator.KeyInfo{Name: "mykey", Address: "push1test"},
			balanceResult:   "2000000000000000000",
			registerErr:     fmt.Errorf("validator already exist"),
			evmAddrResult:   "0xABCD",
		}
	})

	err := runRegisterValidatorWithDeps(d, d.Cfg, "myval", "mykey", "1500000000000000000", "0.10", "word1 word2 word3 word4 word5 word6 word7 word8 word9 word10 word11 word12")
	if err != nil {
		t.Fatalf("expected nil (validator already registered treated as success), got: %v", err)
	}
}

func TestRunRegisterValidatorWithDeps_ValidatorAlreadyExists_JSON(t *testing.T) {
	origOutput := flagOutput
	origNonInteractive := flagNonInteractive
	defer func() {
		flagOutput = origOutput
		flagNonInteractive = origNonInteractive
	}()
	flagOutput = "json"
	flagNonInteractive = true

	d := registerDeps(func(d *Deps) {
		d.Node = &mockNodeClient{status: node.Status{CatchingUp: false}}
		d.RemoteNode = &mockNodeClient{}
		d.Validator = &mockValidator{
			importKeyResult: validator.KeyInfo{Name: "mykey", Address: "push1test"},
			balanceResult:   "2000000000000000000",
			registerErr:     fmt.Errorf("validator already exist"),
			evmAddrResult:   "0xABCD",
		}
	})

	err := runRegisterValidatorWithDeps(d, d.Cfg, "myval", "mykey", "1500000000000000000", "0.10", "word1 word2 word3 word4 word5 word6 word7 word8 word9 word10 word11 word12")
	if err != nil {
		t.Fatalf("expected nil (validator already registered treated as success), got: %v", err)
	}
}

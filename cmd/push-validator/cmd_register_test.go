package main

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/pushchain/push-validator-cli/internal/validator"
)

func registerDeps(overrides ...func(*Deps)) *Deps {
	d := &Deps{
		Cfg:        testCfg(),
		Sup:        &mockSupervisor{running: true},
		Node:       &mockNodeClient{},
		RemoteNode: &mockNodeClient{},
		Fetcher:    &mockFetcher{},
		Validator:  &mockValidator{},
		Runner:     newMockRunner(),
		Prompter:   &nonInteractivePrompter{},
		Output:     &bytes.Buffer{},
		Printer:    testPrinter(),
		RPCCheck:   func(string, time.Duration) bool { return true },
	}
	for _, fn := range overrides {
		fn(d)
	}
	return d
}

func TestHandleRegisterValidator_StatusError(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	d := registerDeps(func(d *Deps) {
		d.Validator = &mockValidator{isValidatorErr: fmt.Errorf("connection refused")}
	})

	err := handleRegisterValidator(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "failed to verify validator status") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleRegisterValidator_CheckOnly_Registered(t *testing.T) {
	origOutput := flagOutput
	origCheckOnly := flagRegisterCheckOnly
	defer func() {
		flagOutput = origOutput
		flagRegisterCheckOnly = origCheckOnly
	}()
	flagOutput = "json"
	flagRegisterCheckOnly = true

	d := registerDeps(func(d *Deps) {
		d.Validator = &mockValidator{isValidatorRes: true}
	})

	err := handleRegisterValidator(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleRegisterValidator_CheckOnly_NotRegistered(t *testing.T) {
	origOutput := flagOutput
	origCheckOnly := flagRegisterCheckOnly
	defer func() {
		flagOutput = origOutput
		flagRegisterCheckOnly = origCheckOnly
	}()
	flagOutput = "json"
	flagRegisterCheckOnly = true

	d := registerDeps(func(d *Deps) {
		d.Validator = &mockValidator{isValidatorRes: false}
	})

	err := handleRegisterValidator(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleRegisterValidator_AlreadyRegistered_JSON(t *testing.T) {
	origOutput := flagOutput
	origCheckOnly := flagRegisterCheckOnly
	defer func() {
		flagOutput = origOutput
		flagRegisterCheckOnly = origCheckOnly
	}()
	flagOutput = "json"
	flagRegisterCheckOnly = false

	d := registerDeps(func(d *Deps) {
		d.Validator = &mockValidator{isValidatorRes: true}
	})

	err := handleRegisterValidator(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleRegisterValidator_AlreadyRegistered_Text(t *testing.T) {
	origOutput := flagOutput
	origCheckOnly := flagRegisterCheckOnly
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagOutput = origOutput
		flagRegisterCheckOnly = origCheckOnly
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagOutput = "text"
	flagRegisterCheckOnly = false
	flagNoColor = true
	flagNoEmoji = true

	d := registerDeps(func(d *Deps) {
		d.Validator = &mockValidator{isValidatorRes: true}
	})

	err := handleRegisterValidator(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleRegisterValidator_MonikerConflict_JSON(t *testing.T) {
	origOutput := flagOutput
	origCheckOnly := flagRegisterCheckOnly
	defer func() {
		flagOutput = origOutput
		flagRegisterCheckOnly = origCheckOnly
	}()
	flagOutput = "json"
	flagRegisterCheckOnly = false

	d := registerDeps(func(d *Deps) {
		d.Validator = &mockValidator{isValidatorRes: false}
		d.Fetcher = &mockFetcher{
			myValidator: validator.MyValidatorInfo{
				ValidatorExistsWithSameMoniker: true,
				ConflictingMoniker:             "existing-val",
			},
		}
	})

	err := handleRegisterValidator(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "moniker conflict") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleRegisterValidator_StatusError_Text(t *testing.T) {
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

	d := registerDeps(func(d *Deps) {
		d.Validator = &mockValidator{isValidatorErr: fmt.Errorf("network error")}
	})

	err := handleRegisterValidator(d)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestHandleRegisterValidator_CheckOnly_Text_NotRegistered(t *testing.T) {
	origOutput := flagOutput
	origCheckOnly := flagRegisterCheckOnly
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagOutput = origOutput
		flagRegisterCheckOnly = origCheckOnly
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagOutput = "text"
	flagRegisterCheckOnly = true
	flagNoColor = true
	flagNoEmoji = true

	d := registerDeps(func(d *Deps) {
		d.Validator = &mockValidator{isValidatorRes: false}
	})

	err := handleRegisterValidator(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleRegisterValidator_MonikerConflict_Text(t *testing.T) {
	origOutput := flagOutput
	origCheckOnly := flagRegisterCheckOnly
	origNonInteractive := flagNonInteractive
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagOutput = origOutput
		flagRegisterCheckOnly = origCheckOnly
		flagNonInteractive = origNonInteractive
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagOutput = "json"
	flagRegisterCheckOnly = false
	flagNonInteractive = true
	flagNoColor = true
	flagNoEmoji = true

	d := registerDeps(func(d *Deps) {
		d.Validator = &mockValidator{isValidatorRes: false}
		d.Fetcher = &mockFetcher{
			myValidator: validator.MyValidatorInfo{
				ValidatorExistsWithSameMoniker: true,
				ConflictingMoniker:             "existing-val",
			},
		}
	})

	err := handleRegisterValidator(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "moniker conflict") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleRegisterValidator_JSON_FullFlow(t *testing.T) {
	origOutput := flagOutput
	origCheckOnly := flagRegisterCheckOnly
	origNonInteractive := flagNonInteractive
	defer func() {
		flagOutput = origOutput
		flagRegisterCheckOnly = origCheckOnly
		flagNonInteractive = origNonInteractive
	}()
	flagOutput = "json"
	flagRegisterCheckOnly = false
	flagNonInteractive = true

	runner := newMockRunner()
	binPath := findPchaind()
	cfg := testCfg()
	runner.outputs[binPath+" keys list --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir+" --output json"] = []byte(`[{"name":"validator-key","address":"push1account"}]`)

	d := registerDeps(func(d *Deps) {
		d.Validator = &mockValidator{
			isValidatorRes: false,
			registerResult: "TX_REGISTER_SUCCESS",
			balanceResult:  "2000000000000000000", // 2 PC - sufficient
		}
		d.Fetcher = &mockFetcher{
			myValidator: validator.MyValidatorInfo{
				IsValidator: false,
			},
		}
		d.Runner = runner
	})

	err := handleRegisterValidator(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleRegisterValidator_JSON_MonikerCheckError(t *testing.T) {
	origOutput := flagOutput
	origCheckOnly := flagRegisterCheckOnly
	origNonInteractive := flagNonInteractive
	defer func() {
		flagOutput = origOutput
		flagRegisterCheckOnly = origCheckOnly
		flagNonInteractive = origNonInteractive
	}()
	flagOutput = "json"
	flagRegisterCheckOnly = false
	flagNonInteractive = true

	runner := newMockRunner()
	binPath := findPchaind()
	cfg := testCfg()
	runner.outputs[binPath+" keys list --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir+" --output json"] = []byte(`[{"name":"validator-key","address":"push1account"}]`)

	d := registerDeps(func(d *Deps) {
		d.Validator = &mockValidator{
			isValidatorRes: false,
			registerResult: "TX_REG",
			balanceResult:  "2000000000000000000",
		}
		d.Fetcher = &mockFetcher{
			myValidatorErr: fmt.Errorf("rpc error"), // moniker check fails - should continue
		}
		d.Runner = runner
	})

	err := handleRegisterValidator(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleRegisterValidator_CheckOnly_Text_Registered(t *testing.T) {
	origOutput := flagOutput
	origCheckOnly := flagRegisterCheckOnly
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagOutput = origOutput
		flagRegisterCheckOnly = origCheckOnly
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagOutput = "text"
	flagRegisterCheckOnly = true
	flagNoColor = true
	flagNoEmoji = true

	d := registerDeps(func(d *Deps) {
		d.Validator = &mockValidator{isValidatorRes: true}
	})

	err := handleRegisterValidator(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

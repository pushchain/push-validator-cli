package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/pushchain/push-validator-cli/internal/validator"
)

func editValidatorDeps(overrides ...func(*Deps)) *Deps {
	d := &Deps{
		Cfg:        testCfg(),
		Sup:        &mockSupervisor{running: true},
		Node:       &mockNodeClient{},
		RemoteNode: &mockNodeClient{},
		Fetcher: &mockFetcher{myValidator: validator.MyValidatorInfo{
			IsValidator: true,
			Address:     "pushvaloper1test",
			Moniker:     "test-validator",
		}},
		Validator: &mockValidator{editValResult: "TXHASH_EDIT"},
		Runner:    newMockRunner(),
		Prompter:  &nonInteractivePrompter{},
		RPCCheck:  func(string, time.Duration) bool { return true },
	}
	for _, fn := range overrides {
		fn(d)
	}
	return d
}

func TestHandleEditValidator_NodeNotRunning(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	d := editValidatorDeps(func(d *Deps) {
		d.Sup = &mockSupervisor{running: false}
	})

	err := handleEditValidator(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "node is not running") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleEditValidator_FetcherError(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	d := editValidatorDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{myValidatorErr: fmt.Errorf("timeout")}
	})

	err := handleEditValidator(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "failed to check validator status") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleEditValidator_NotValidator(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	d := editValidatorDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{myValidator: validator.MyValidatorInfo{IsValidator: false}}
	})

	err := handleEditValidator(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "not registered as validator") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleEditValidator_NoChanges(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	d := editValidatorDeps(func(d *Deps) {
		d.Prompter = &mockPrompter{
			responses:   []string{"", "", "", "", ""},
			interactive: true,
		}
	})

	err := handleEditValidator(d)
	if err != nil {
		t.Fatalf("expected nil error for no changes, got: %v", err)
	}
}

func TestHandleEditValidator_Success_JSON(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	t.Setenv("VALIDATOR_MONIKER", "json-moniker")

	d := editValidatorDeps()

	err := handleEditValidator(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleEditValidator_Success_Text(t *testing.T) {
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

	d := editValidatorDeps(func(d *Deps) {
		d.Prompter = &mockPrompter{
			responses:   []string{"new-moniker", "", "", "", ""},
			interactive: true,
		}
	})

	err := handleEditValidator(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleEditValidator_EditValidatorFails(t *testing.T) {
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

	d := editValidatorDeps(func(d *Deps) {
		d.Validator = &mockValidator{editValErr: fmt.Errorf("insufficient gas")}
		d.Prompter = &mockPrompter{
			responses:   []string{"new-moniker", "", "", "", ""},
			interactive: true,
		}
	})

	err := handleEditValidator(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "update details failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleEditValidator_EditValidatorFails_JSON(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	t.Setenv("VALIDATOR_MONIKER", "fail-moniker")

	d := editValidatorDeps(func(d *Deps) {
		d.Validator = &mockValidator{editValErr: fmt.Errorf("insufficient gas")}
	})

	err := handleEditValidator(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "update details failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleEditValidator_EditValidatorFails_Text(t *testing.T) {
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

	d := editValidatorDeps(func(d *Deps) {
		d.Validator = &mockValidator{editValErr: fmt.Errorf("insufficient gas")}
		d.Prompter = &mockPrompter{
			responses:   []string{"new-moniker", "", "", "", ""},
			interactive: true,
		}
	})

	err := handleEditValidator(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "update details failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleEditValidator_NonInteractive_EnvVars(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	t.Setenv("VALIDATOR_MONIKER", "env-moniker")
	t.Setenv("VALIDATOR_WEBSITE", "https://example.com")
	t.Setenv("VALIDATOR_DETAILS", "my validator")
	t.Setenv("VALIDATOR_SECURITY", "sec@example.com")
	t.Setenv("VALIDATOR_IDENTITY", "ABCD1234ABCD1234")

	d := editValidatorDeps() // non-interactive prompter by default

	err := handleEditValidator(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleEditValidator_Interactive_AllFields(t *testing.T) {
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

	d := editValidatorDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{myValidator: validator.MyValidatorInfo{
			IsValidator:     true,
			Address:         "pushvaloper1test",
			Moniker:         "old-moniker",
			Website:         "https://old.example.com",
			Details:         "old details",
			SecurityContact: "old@example.com",
			Identity:        "OLD_IDENTITY",
		}}
		d.Prompter = &mockPrompter{
			responses:   []string{"new-moniker", "https://new.example.com", "new details", "new@example.com", "NEW_IDENTITY"},
			interactive: true,
		}
	})

	err := handleEditValidator(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleEditValidator_KeyDerivation_Success(t *testing.T) {
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
	// Mock address conversion
	runner.outputs[binPath+" debug addr pushvaloper1test"] = []byte("Bech32 Acc: push1account\n")
	// Mock key list lookup
	runner.outputs[binPath+" keys list --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir+" --output json"] = []byte(`[{"name":"my-derived-key","address":"push1account"}]`)

	d := editValidatorDeps(func(d *Deps) {
		d.Runner = runner
		d.Prompter = &mockPrompter{
			responses:   []string{"new-moniker", "", "", "", ""},
			interactive: true,
		}
	})

	err := handleEditValidator(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleEditValidator_KeyDerivation_Fallback(t *testing.T) {
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
	// Address conversion fails — should fall back to default key name
	runner.errors[binPath+" debug addr pushvaloper1test"] = fmt.Errorf("binary not found")

	d := editValidatorDeps(func(d *Deps) {
		d.Runner = runner
		d.Prompter = &mockPrompter{
			responses:   []string{"new-moniker", "", "", "", ""},
			interactive: true,
		}
	})

	err := handleEditValidator(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleEditValidator_NotValidator_Text(t *testing.T) {
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

	d := editValidatorDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{myValidator: validator.MyValidatorInfo{IsValidator: false}}
	})

	err := handleEditValidator(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "not registered as validator") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleEditValidator_FetcherError_Text(t *testing.T) {
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

	d := editValidatorDeps(func(d *Deps) {
		d.Fetcher = &mockFetcher{myValidatorErr: fmt.Errorf("timeout")}
	})

	err := handleEditValidator(d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "failed to check validator status") {
		t.Errorf("unexpected error: %v", err)
	}
}

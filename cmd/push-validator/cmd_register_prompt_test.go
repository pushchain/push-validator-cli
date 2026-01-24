package main

import (
	"testing"

	"github.com/pushchain/push-validator-cli/internal/validator"
)

func TestPromptCommissionRate_Default(t *testing.T) {
	p := &mockPrompter{responses: []string{""}}
	rate := promptCommissionRate(p, "0.10")
	if rate != "0.10" {
		t.Errorf("expected 0.10, got %s", rate)
	}
}

func TestPromptCommissionRate_ValidInput(t *testing.T) {
	p := &mockPrompter{responses: []string{"15"}}
	rate := promptCommissionRate(p, "0.10")
	if rate != "0.15" {
		t.Errorf("expected 0.15, got %s", rate)
	}
}

func TestPromptCommissionRate_InvalidInput(t *testing.T) {
	// Invalid input, then empty → falls back to default
	p := &mockPrompter{responses: []string{"abc", ""}, interactive: true}
	rate := promptCommissionRate(p, "0.10")
	if rate != "0.10" {
		t.Errorf("expected default 0.10, got %s", rate)
	}
}

func TestPromptCommissionRate_TooHigh(t *testing.T) {
	// Too high, then empty → falls back to default
	p := &mockPrompter{responses: []string{"150", ""}, interactive: true}
	rate := promptCommissionRate(p, "0.10")
	if rate != "0.10" {
		t.Errorf("expected default 0.10 for >100, got %s", rate)
	}
}

func TestPromptCommissionRate_TooLow(t *testing.T) {
	// Too low, then empty → falls back to default
	p := &mockPrompter{responses: []string{"4", ""}, interactive: true}
	rate := promptCommissionRate(p, "0.10")
	if rate != "0.10" {
		t.Errorf("expected default 0.10 for <5, got %s", rate)
	}
}

func TestPromptCommissionRate_BoundaryValues(t *testing.T) {
	p := &mockPrompter{responses: []string{"5"}}
	rate := promptCommissionRate(p, "0.10")
	if rate != "0.05" {
		t.Errorf("expected 0.05 for 5%%, got %s", rate)
	}

	p = &mockPrompter{responses: []string{"100"}}
	rate = promptCommissionRate(p, "0.10")
	if rate != "1.00" {
		t.Errorf("expected 1.00 for 100%%, got %s", rate)
	}
}

func TestPromptWalletChoiceWith_CreateNew(t *testing.T) {
	p := &mockPrompter{responses: []string{"1"}}
	mnemonic := promptWalletChoiceWith(p)
	if mnemonic != "" {
		t.Errorf("expected empty mnemonic for create new, got %s", mnemonic)
	}
}

func TestPromptWalletChoiceWith_DefaultOption(t *testing.T) {
	p := &mockPrompter{responses: []string{""}}
	mnemonic := promptWalletChoiceWith(p)
	if mnemonic != "" {
		t.Errorf("expected empty mnemonic for default, got %s", mnemonic)
	}
}

func TestPromptWalletChoiceWith_ImportInvalidMnemonic(t *testing.T) {
	p := &mockPrompter{responses: []string{"2", "not a valid mnemonic"}}
	mnemonic := promptWalletChoiceWith(p)
	if mnemonic != "" {
		t.Errorf("expected empty mnemonic for invalid, got %s", mnemonic)
	}
}

func TestPromptWalletChoiceWith_ImportValidMnemonic(t *testing.T) {
	// A valid 12-word mnemonic
	validMnemonic := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
	if err := validator.ValidateMnemonic(validMnemonic); err != nil {
		t.Skipf("ValidateMnemonic rejects test mnemonic: %v", err)
	}
	p := &mockPrompter{responses: []string{"2", validMnemonic}}
	mnemonic := promptWalletChoiceWith(p)
	if mnemonic != validMnemonic {
		t.Errorf("expected valid mnemonic back, got %s", mnemonic)
	}
}

func TestSelectStakeAmount_EmptyBalance(t *testing.T) {
	p := &mockPrompter{interactive: false}
	stake, err := selectStakeAmount(p, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stake != registrationMinStake {
		t.Errorf("expected minStake for empty balance, got %s", stake)
	}
}

func TestSelectStakeAmount_NonInteractive(t *testing.T) {
	p := &mockPrompter{interactive: false}
	// 5 PC balance
	stake, err := selectStakeAmount(p, "5000000000000000000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should return max stakeable (balance - fee reserve = 5 - 0.1 = 4.9 PC)
	expected := "4900000000000000000"
	if stake != expected {
		t.Errorf("expected %s, got %s", expected, stake)
	}
}

func TestSelectStakeAmount_Interactive_Default(t *testing.T) {
	// Empty input = default to max
	p := &mockPrompter{interactive: true, responses: []string{""}}
	stake, err := selectStakeAmount(p, "5000000000000000000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "4900000000000000000"
	if stake != expected {
		t.Errorf("expected %s, got %s", expected, stake)
	}
}

func TestSelectStakeAmount_Interactive_CustomAmount(t *testing.T) {
	// User enters 2.0 PC
	p := &mockPrompter{interactive: true, responses: []string{"2.0"}}
	stake, err := selectStakeAmount(p, "5000000000000000000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 2.0 * 1e18 = 2000000000000000000
	expected := "2000000000000000000"
	if stake != expected {
		t.Errorf("expected %s, got %s", expected, stake)
	}
}

func TestSelectStakeAmount_Interactive_TooLow_ThenValid(t *testing.T) {
	// First input too low, second valid
	p := &mockPrompter{interactive: true, responses: []string{"0.5", "2.0"}}
	stake, err := selectStakeAmount(p, "5000000000000000000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "2000000000000000000"
	if stake != expected {
		t.Errorf("expected %s, got %s", expected, stake)
	}
}

func TestSelectStakeAmount_Interactive_TooHigh_ThenValid(t *testing.T) {
	// First input too high, second valid
	p := &mockPrompter{interactive: true, responses: []string{"10.0", "2.0"}}
	stake, err := selectStakeAmount(p, "5000000000000000000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "2000000000000000000"
	if stake != expected {
		t.Errorf("expected %s, got %s", expected, stake)
	}
}

func TestSelectStakeAmount_Interactive_InvalidInput_ThenValid(t *testing.T) {
	p := &mockPrompter{interactive: true, responses: []string{"abc", "1.5"}}
	stake, err := selectStakeAmount(p, "5000000000000000000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "1500000000000000000"
	if stake != expected {
		t.Errorf("expected %s, got %s", expected, stake)
	}
}

func TestWaitForFunding_SufficientImmediately(t *testing.T) {
	v := &mockValidator{balanceResult: "2000000000000000000"} // 2 PC
	p := &nonInteractivePrompter{}
	bal, err := waitForFunding(v, p, "push1test", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bal != "2000000000000000000" {
		t.Errorf("expected 2 PC balance, got %s", bal)
	}
}

func TestWaitForFunding_InsufficientNonInteractive(t *testing.T) {
	v := &mockValidator{balanceResult: "100000000000000000"} // 0.1 PC
	p := &nonInteractivePrompter{}
	_, err := waitForFunding(v, p, "push1test", 3)
	if err == nil {
		t.Fatal("expected error for insufficient balance")
	}
}

func TestWaitForFunding_BalanceError(t *testing.T) {
	v := &mockValidator{balanceErr: errMock}
	p := &nonInteractivePrompter{}
	_, err := waitForFunding(v, p, "push1test", 2)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCollectRegistrationInputs_DefaultValues(t *testing.T) {
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagNoColor = true
	flagNoEmoji = true

	d := registerDeps()
	defaults := registrationInputs{
		Moniker:        "my-validator",
		KeyName:        "my-key",
		CommissionRate: "0.10",
	}

	// Response 0: wallet choice → "" (default = create new)
	// Response 1: key name prompt → "" (use default)
	// Moniker prompt is skipped (not "" or "push-validator")
	d.Prompter = &mockPrompter{
		responses:   []string{"", ""},
		interactive: true,
	}

	result, err := collectRegistrationInputs(d, defaults)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Moniker != "my-validator" {
		t.Errorf("expected moniker my-validator, got %s", result.Moniker)
	}
	if result.KeyName != "my-key" {
		t.Errorf("expected keyName my-key, got %s", result.KeyName)
	}
}

func TestCollectRegistrationInputs_CustomValues(t *testing.T) {
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagNoColor = true
	flagNoEmoji = true

	d := registerDeps()
	defaults := registrationInputs{
		Moniker:        "push-validator",
		KeyName:        "validator-key",
		CommissionRate: "0.10",
	}

	// Response 0: wallet choice → "" (default = create new)
	// Response 1: key name → "custom-key"
	// Moniker IS "push-validator" so moniker prompt fires
	// Response 2: moniker → "custom-moniker"
	d.Prompter = &mockPrompter{
		responses:   []string{"", "custom-key", "custom-moniker"},
		interactive: true,
	}

	result, err := collectRegistrationInputs(d, defaults)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Moniker != "custom-moniker" {
		t.Errorf("expected custom-moniker, got %s", result.Moniker)
	}
	if result.KeyName != "custom-key" {
		t.Errorf("expected custom-key, got %s", result.KeyName)
	}
}

func TestCollectRegistrationInputs_KeyExists_UseExisting(t *testing.T) {
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagNoColor = true
	flagNoEmoji = true

	d := registerDeps()
	defaults := registrationInputs{
		Moniker:        "my-validator",
		KeyName:        "my-key",
		CommissionRate: "0.10",
	}

	// Make key exist: mockRunner returns success for "keys show my-key ..."
	cfg := d.Cfg
	binPath := findPchaind()
	runner := newMockRunner()
	runner.outputs[binPath+" keys show my-key -a --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir] = []byte("push1existing\n")
	d.Runner = runner

	// Response 0: wallet choice → "" (create new)
	// Response 1: key name → "" (use default "my-key" which exists)
	// Key exists branch: Response 2: enter different name → "" (use existing)
	d.Prompter = &mockPrompter{
		responses:   []string{"", "", ""},
		interactive: true,
	}

	result, err := collectRegistrationInputs(d, defaults)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.KeyName != "my-key" {
		t.Errorf("expected my-key, got %s", result.KeyName)
	}
	if result.ImportMnemonic != "" {
		t.Errorf("expected empty mnemonic for existing key, got %s", result.ImportMnemonic)
	}
}

func TestCollectRegistrationInputs_KeyExists_EnterNewName(t *testing.T) {
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagNoColor = true
	flagNoEmoji = true

	d := registerDeps()
	defaults := registrationInputs{
		Moniker:        "my-validator",
		KeyName:        "existing-key",
		CommissionRate: "0.10",
	}

	cfg := d.Cfg
	binPath := findPchaind()
	runner := newMockRunner()
	// existing-key exists
	runner.outputs[binPath+" keys show existing-key -a --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir] = []byte("push1existing\n")
	// new-key does NOT exist (returns error)
	runner.errors[binPath+" keys show new-key -a --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir] = errMock
	d.Runner = runner

	// Response 0: wallet choice → "" (create new)
	// Response 1: key name → "" (use default "existing-key" which exists)
	// Key exists: Response 2: enter different name → "new-key"
	d.Prompter = &mockPrompter{
		responses:   []string{"", "", "new-key"},
		interactive: true,
	}

	result, err := collectRegistrationInputs(d, defaults)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.KeyName != "new-key" {
		t.Errorf("expected new-key, got %s", result.KeyName)
	}
}

func TestCollectRegistrationInputs_KeyExists_NewNameAlsoExists(t *testing.T) {
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagNoColor = true
	flagNoEmoji = true

	d := registerDeps()
	defaults := registrationInputs{
		Moniker:        "my-validator",
		KeyName:        "existing-key",
		CommissionRate: "0.10",
	}

	cfg := d.Cfg
	binPath := findPchaind()
	runner := newMockRunner()
	// Both keys exist
	runner.outputs[binPath+" keys show existing-key -a --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir] = []byte("push1a\n")
	runner.outputs[binPath+" keys show also-exists -a --keyring-backend "+cfg.KeyringBackend+" --home "+cfg.HomeDir] = []byte("push1b\n")
	d.Runner = runner

	// Response 0: wallet choice → "" (create new)
	// Response 1: key name → "" (use default which exists)
	// Key exists: Response 2: enter different name → "also-exists" (also exists)
	d.Prompter = &mockPrompter{
		responses:   []string{"", "", "also-exists"},
		interactive: true,
	}

	result, err := collectRegistrationInputs(d, defaults)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.KeyName != "also-exists" {
		t.Errorf("expected also-exists, got %s", result.KeyName)
	}
	if result.ImportMnemonic != "" {
		t.Errorf("expected empty mnemonic, got %s", result.ImportMnemonic)
	}
}

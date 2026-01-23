package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/pushchain/push-validator-cli/internal/cosmovisor"
	ui "github.com/pushchain/push-validator-cli/internal/ui"
	"github.com/pushchain/push-validator-cli/internal/validator"
)

// nonInteractivePrompter is a test prompter that is non-interactive.
type nonInteractivePrompter struct{}

func (p *nonInteractivePrompter) ReadLine(prompt string) (string, error) {
	return "", fmt.Errorf("non-interactive")
}
func (p *nonInteractivePrompter) IsInteractive() bool { return false }

func testPrinter() ui.Printer { return ui.NewPrinter("text") }

func TestWaitForSufficientBalanceWith_SufficientImmediately(t *testing.T) {
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


	v := &mockValidator{balanceResult: "500000000000000000"} // 0.5 PC

	result := waitForSufficientBalanceWith(v, testPrinter(), &nonInteractivePrompter{}, "push1test", "0xABC", "150000000000000000", "test-op")
	if !result {
		t.Error("expected true when balance is sufficient")
	}
}

func TestWaitForSufficientBalanceWith_Insufficient_NonInteractive(t *testing.T) {
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


	v := &mockValidator{balanceResult: "50000000000000000"} // 0.05 PC - insufficient

	result := waitForSufficientBalanceWith(v, testPrinter(), &nonInteractivePrompter{}, "push1test", "0xABC", "150000000000000000", "test-op")
	if result {
		t.Error("expected false when balance is insufficient")
	}
}

func TestWaitForSufficientBalanceWith_BalanceError(t *testing.T) {
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


	v := &mockValidator{balanceErr: fmt.Errorf("rpc error")}

	result := waitForSufficientBalanceWith(v, testPrinter(), &nonInteractivePrompter{}, "push1test", "0xABC", "150000000000000000", "test-op")
	if result {
		t.Error("expected false when balance check fails")
	}
}

func TestWaitForSufficientBalanceWith_ZeroBalance(t *testing.T) {
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


	v := &mockValidator{balanceResult: "0"}

	result := waitForSufficientBalanceWith(v, testPrinter(), &nonInteractivePrompter{}, "push1test", "0xABC", "150000000000000000", "test-op")
	if result {
		t.Error("expected false with zero balance")
	}
}

func TestWaitForSufficientBalanceWith_NoEVMAddr(t *testing.T) {
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


	v := &mockValidator{balanceResult: "50000000000000000"} // insufficient

	// Test with empty EVM address (should still work, just no EVM display)
	result := waitForSufficientBalanceWith(v, testPrinter(), &nonInteractivePrompter{}, "push1test", "", "150000000000000000", "withdraw")
	if result {
		t.Error("expected false when balance is insufficient")
	}
}

func TestWaitForSufficientBalanceWith_BecomeSufficient(t *testing.T) {
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


	callCount := 0
	v := &balanceIncrementingValidator{callCount: &callCount}

	result := waitForSufficientBalanceWith(v, testPrinter(), &nonInteractivePrompter{}, "push1test", "0xABC", "150000000000000000", "test-op")
	if !result {
		t.Error("expected true when balance becomes sufficient on retry")
	}
}

// balanceIncrementingValidator returns increasing balance on each call.
type balanceIncrementingValidator struct {
	callCount *int
}

func (m *balanceIncrementingValidator) Balance(ctx context.Context, addr string) (string, error) {
	*m.callCount++
	if *m.callCount <= 2 {
		return "50000000000000000", nil // 0.05 PC - insufficient
	}
	return "500000000000000000", nil // 0.5 PC - sufficient
}

func (m *balanceIncrementingValidator) IsValidator(ctx context.Context, addr string) (bool, error) {
	return false, nil
}

func (m *balanceIncrementingValidator) Register(ctx context.Context, args validator.RegisterArgs) (string, error) {
	return "", nil
}

func (m *balanceIncrementingValidator) Unjail(ctx context.Context, keyName string) (string, error) {
	return "", nil
}

func (m *balanceIncrementingValidator) WithdrawRewards(ctx context.Context, validatorAddr string, keyName string, includeCommission bool) (string, error) {
	return "", nil
}

func (m *balanceIncrementingValidator) Delegate(ctx context.Context, args validator.DelegateArgs) (string, error) {
	return "", nil
}

func (m *balanceIncrementingValidator) EnsureKey(ctx context.Context, name string) (validator.KeyInfo, error) {
	return validator.KeyInfo{}, nil
}

func (m *balanceIncrementingValidator) ImportKey(ctx context.Context, name string, mnemonic string) (validator.KeyInfo, error) {
	return validator.KeyInfo{}, nil
}

func (m *balanceIncrementingValidator) GetEVMAddress(ctx context.Context, addr string) (string, error) {
	return "", nil
}

func TestNewSupervisorWith_CosmovisorDetected(t *testing.T) {
	detect := func(homeDir string) cosmovisor.DetectionResult {
		return cosmovisor.DetectionResult{Available: true, SetupComplete: true}
	}
	sup := newSupervisorWith("/tmp/test", detect)
	if sup == nil {
		t.Fatal("expected non-nil supervisor")
	}
}

func TestNewSupervisorWith_CosmovisorNotDetected(t *testing.T) {
	detect := func(homeDir string) cosmovisor.DetectionResult {
		return cosmovisor.DetectionResult{Available: false, SetupComplete: false}
	}
	sup := newSupervisorWith("/tmp/test", detect)
	if sup == nil {
		t.Fatal("expected non-nil supervisor")
	}
}

func TestNewSupervisorWith_AvailableButNotSetup(t *testing.T) {
	detect := func(homeDir string) cosmovisor.DetectionResult {
		return cosmovisor.DetectionResult{Available: true, SetupComplete: false}
	}
	sup := newSupervisorWith("/tmp/test", detect)
	if sup == nil {
		t.Fatal("expected non-nil supervisor")
	}
}

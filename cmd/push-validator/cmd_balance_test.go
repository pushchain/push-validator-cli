package main

import (
	"fmt"
	"os"
	"testing"
)

func TestHandleBalance_NoAddress_NoKeyName(t *testing.T) {
	os.Unsetenv("KEY_NAME")
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()

	d := &Deps{
		Cfg:       testCfg(),
		Printer:   getPrinter(),
		Validator: &mockValidator{},
		Runner:    newMockRunner(),
	}

	err := handleBalance(d, nil)
	if err == nil {
		t.Fatal("expected error when no address and no KEY_NAME")
	}
	if err.Error() != "address not provided" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleBalance_NoAddress_NoKeyName_JSON(t *testing.T) {
	os.Unsetenv("KEY_NAME")
	origOutput := flagOutput
	flagOutput = "json"
	defer func() { flagOutput = origOutput }()

	d := &Deps{
		Cfg:       testCfg(),
		Printer:   getPrinter(),
		Validator: &mockValidator{},
		Runner:    newMockRunner(),
	}

	err := handleBalance(d, nil)
	if err == nil {
		t.Fatal("expected error when no address and no KEY_NAME (json)")
	}
}

func TestHandleBalance_DirectAddress_Success(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "text"

	d := &Deps{
		Cfg:       testCfg(),
		Printer:   getPrinter(),
		Validator: &mockValidator{balanceResult: "1000000"},
		Runner:    newMockRunner(),
	}

	err := handleBalance(d, []string{"push1abc123"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleBalance_DirectAddress_JSON(t *testing.T) {
	origOutput := flagOutput
	flagOutput = "json"
	defer func() { flagOutput = origOutput }()

	d := &Deps{
		Cfg:       testCfg(),
		Printer:   getPrinter(),
		Validator: &mockValidator{balanceResult: "5000000"},
		Runner:    newMockRunner(),
	}

	err := handleBalance(d, []string{"push1xyz789"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleBalance_BalanceError(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "text"

	d := &Deps{
		Cfg:       testCfg(),
		Printer:   getPrinter(),
		Validator: &mockValidator{balanceErr: fmt.Errorf("node unreachable")},
		Runner:    newMockRunner(),
	}

	err := handleBalance(d, []string{"push1abc123"})
	if err == nil {
		t.Fatal("expected error from Balance")
	}
	if err.Error() != "node unreachable" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleBalance_BalanceError_JSON(t *testing.T) {
	origOutput := flagOutput
	flagOutput = "json"
	defer func() { flagOutput = origOutput }()

	d := &Deps{
		Cfg:       testCfg(),
		Printer:   getPrinter(),
		Validator: &mockValidator{balanceErr: fmt.Errorf("timeout")},
		Runner:    newMockRunner(),
	}

	err := handleBalance(d, []string{"push1abc123"})
	if err == nil {
		t.Fatal("expected error from Balance (json)")
	}
}

func TestHandleBalance_KeyNameResolution_Success(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "text"

	os.Setenv("KEY_NAME", "mykey")
	defer os.Unsetenv("KEY_NAME")

	runner := newMockRunner()
	cfg := testCfg()
	// The runner key needs to match exactly what handleBalance calls
	binPath := findPchaind()
	runnerKey := binPath + " keys show mykey -a --keyring-backend " + cfg.KeyringBackend + " --home " + cfg.HomeDir
	runner.outputs[runnerKey] = []byte("push1resolved\n")

	d := &Deps{
		Cfg:       cfg,
		Printer:   getPrinter(),
		Validator: &mockValidator{balanceResult: "999"},
		Runner:    runner,
	}

	err := handleBalance(d, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleBalance_KeyNameResolution_RunnerError(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "text"

	os.Setenv("KEY_NAME", "badkey")
	defer os.Unsetenv("KEY_NAME")

	runner := newMockRunner()
	cfg := testCfg()
	binPath := findPchaind()
	runnerKey := binPath + " keys show badkey -a --keyring-backend " + cfg.KeyringBackend + " --home " + cfg.HomeDir
	runner.errors[runnerKey] = fmt.Errorf("key not found")

	d := &Deps{
		Cfg:     cfg,
		Printer: getPrinter(),
		Runner:  runner,
	}

	err := handleBalance(d, nil)
	if err == nil {
		t.Fatal("expected error from runner")
	}
	if !containsSubstr(err.Error(), "resolve address") {
		t.Errorf("expected 'resolve address' in error, got: %v", err)
	}
}

func TestHandleBalance_HexAddress_Success(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "text"

	runner := newMockRunner()
	cfg := testCfg()
	binPath := findPchaind()
	// hexToBech32Address strips 0x prefix and calls: debug addr <hex>
	runnerKey := binPath + " debug addr 1234abcd"
	runner.outputs[runnerKey] = []byte("Address: 1234ABCD\nBech32 Acc: push1converted\n")

	d := &Deps{
		Cfg:       cfg,
		Printer:   getPrinter(),
		Validator: &mockValidator{balanceResult: "5000"},
		Runner:    runner,
	}

	err := handleBalance(d, []string{"0x1234abcd"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleBalance_HexAddress_ConversionError(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "text"

	runner := newMockRunner()
	cfg := testCfg()
	binPath := findPchaind()
	runnerKey := binPath + " debug addr deadbeef"
	runner.errors[runnerKey] = fmt.Errorf("invalid hex")

	d := &Deps{
		Cfg:       cfg,
		Printer:   getPrinter(),
		Validator: &mockValidator{},
		Runner:    runner,
	}

	err := handleBalance(d, []string{"0xdeadbeef"})
	if err == nil {
		t.Fatal("expected error from hex conversion")
	}
}

func TestHandleBalance_HexAddress_ConversionError_JSON(t *testing.T) {
	origOutput := flagOutput
	flagOutput = "json"
	defer func() { flagOutput = origOutput }()

	runner := newMockRunner()
	cfg := testCfg()
	binPath := findPchaind()
	runnerKey := binPath + " debug addr deadbeef"
	runner.errors[runnerKey] = fmt.Errorf("invalid hex")

	d := &Deps{
		Cfg:       cfg,
		Printer:   getPrinter(),
		Validator: &mockValidator{},
		Runner:    runner,
	}

	err := handleBalance(d, []string{"0Xdeadbeef"})
	if err == nil {
		t.Fatal("expected error from hex conversion (json)")
	}
}

func TestHandleBalance_KeyNameResolution_RunnerError_JSON(t *testing.T) {
	origOutput := flagOutput
	flagOutput = "json"
	defer func() { flagOutput = origOutput }()

	os.Setenv("KEY_NAME", "badkey")
	defer os.Unsetenv("KEY_NAME")

	runner := newMockRunner()
	cfg := testCfg()
	binPath := findPchaind()
	runnerKey := binPath + " keys show badkey -a --keyring-backend " + cfg.KeyringBackend + " --home " + cfg.HomeDir
	runner.errors[runnerKey] = fmt.Errorf("key not found")

	d := &Deps{
		Cfg:     cfg,
		Printer: getPrinter(),
		Runner:  runner,
	}

	err := handleBalance(d, nil)
	if err == nil {
		t.Fatal("expected error from runner (json)")
	}
}


package main

import (
	"fmt"
	"testing"

	"github.com/pushchain/push-validator-cli/internal/validator"
)

func TestTruncateAddress_ShortAddress(t *testing.T) {
	addr := "push1abc"
	result := truncateAddress(addr, 30)
	if result != addr {
		t.Errorf("truncateAddress(%q, 30) = %q, want %q", addr, result, addr)
	}
}

func TestTruncateAddress_LongPushvaloper(t *testing.T) {
	addr := "pushvaloper1abcdefghijklmnopqrstuvwxyz123456"
	result := truncateAddress(addr, 24)
	if result == addr {
		t.Error("expected address to be truncated")
	}
	// Should keep pushvaloper prefix (14 chars) + ... + 8 char suffix
	if len(result) > 25 {
		t.Errorf("truncated result too long: %q (%d chars)", result, len(result))
	}
	if result[:14] != "pushvaloper1ab" {
		t.Errorf("expected prefix 'pushvaloper1ab', got %q", result[:14])
	}
}

func TestTruncateAddress_LongHexAddress(t *testing.T) {
	addr := "0x1234567890abcdef1234567890abcdef12345678"
	result := truncateAddress(addr, 16)
	if result == addr {
		t.Error("expected hex address to be truncated")
	}
	if result[:6] != "0x1234" {
		t.Errorf("expected prefix '0x1234', got %q", result[:6])
	}
}

func TestTruncateAddress_ExactWidth(t *testing.T) {
	addr := "push1abc123"
	result := truncateAddress(addr, len(addr))
	if result != addr {
		t.Errorf("address at exact width should not be truncated: %q", result)
	}
}

func TestTruncateAddress_UnknownPrefix(t *testing.T) {
	addr := "someotherverylongaddressthatexceedsmaxwidthfortesting"
	result := truncateAddress(addr, 10)
	// Unknown prefix - returns original (no truncation logic for unknown prefixes)
	if result != addr {
		t.Errorf("unknown prefix should return original: got %q", result)
	}
}

func TestTruncateAddress_UppercaseHex(t *testing.T) {
	addr := "0X1234567890ABCDEF1234567890ABCDEF12345678"
	result := truncateAddress(addr, 16)
	if result == addr {
		t.Error("expected uppercase hex address to be truncated")
	}
}

func TestHandleValidatorsWithFormat_JSONOutput_Success(t *testing.T) {
	runner := newMockRunner()
	binPath := findPchaind()
	cfg := testCfg()
	remote := fmt.Sprintf("https://%s", cfg.GenesisDomain)
	key := binPath + " query staking validators --node " + remote + " -o json"
	runner.outputs[key] = []byte(`{"validators":[]}`)

	d := &Deps{
		Cfg:     cfg,
		Runner:  runner,
		Fetcher: &mockFetcher{},
		Printer: getPrinter(),
	}

	err := handleValidatorsWithFormat(d, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleValidatorsWithFormat_JSONOutput_Error(t *testing.T) {
	runner := newMockRunner()
	binPath := findPchaind()
	cfg := testCfg()
	remote := fmt.Sprintf("https://%s", cfg.GenesisDomain)
	key := binPath + " query staking validators --node " + remote + " -o json"
	runner.errors[key] = fmt.Errorf("connection refused")

	d := &Deps{
		Cfg:     cfg,
		Runner:  runner,
		Fetcher: &mockFetcher{},
		Printer: getPrinter(),
	}

	err := handleValidatorsWithFormat(d, true)
	if err == nil {
		t.Fatal("expected error from runner")
	}
}

func TestHandleValidatorsWithFormat_TableOutput_EmptyList(t *testing.T) {
	d := &Deps{
		Cfg: testCfg(),
		Fetcher: &mockFetcher{
			allValidators: validator.ValidatorList{Total: 0, Validators: nil},
		},
		Runner:  newMockRunner(),
		Printer: getPrinter(),
	}

	err := handleValidatorsWithFormat(d, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleValidatorsWithFormat_TableOutput_FetchError(t *testing.T) {
	d := &Deps{
		Cfg: testCfg(),
		Fetcher: &mockFetcher{
			allValidatorsErr: fmt.Errorf("network timeout"),
		},
		Runner:  newMockRunner(),
		Printer: getPrinter(),
	}

	err := handleValidatorsWithFormat(d, false)
	if err == nil {
		t.Fatal("expected error from fetcher")
	}
	if !containsSubstr(err.Error(), "validators:") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleValidatorsWithFormat_TableOutput_WithValidators(t *testing.T) {
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagNoColor = true
	flagNoEmoji = true

	d := &Deps{
		Cfg: testCfg(),
		Fetcher: &mockFetcher{
			allValidators: validator.ValidatorList{
				Total: 3,
				Validators: []validator.ValidatorInfo{
					{OperatorAddress: "pushvaloper1aaa", Moniker: "val-1", Status: "BONDED", Tokens: "1000000000000000000", Commission: "10%"},
					{OperatorAddress: "pushvaloper1bbb", Moniker: "val-2", Status: "UNBONDED", Tokens: "500000000000000000", Commission: "5%", Jailed: true},
					{OperatorAddress: "pushvaloper1ccc", Moniker: "", Status: "UNBONDING", Tokens: "0", Commission: "0%"},
				},
			},
			myValidator: validator.MyValidatorInfo{
				IsValidator: true,
				Address:     "pushvaloper1aaa",
			},
		},
		Runner:  newMockRunner(),
		Printer: getPrinter(),
	}

	err := handleValidatorsWithFormat(d, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleValidatorsWithFormat_TableOutput_NoMyValidator(t *testing.T) {
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagNoColor = true
	flagNoEmoji = true

	d := &Deps{
		Cfg: testCfg(),
		Fetcher: &mockFetcher{
			allValidators: validator.ValidatorList{
				Total: 1,
				Validators: []validator.ValidatorInfo{
					{OperatorAddress: "pushvaloper1xyz", Moniker: "other-val", Status: "BONDED", Tokens: "2000000000000000000", Commission: "15%"},
				},
			},
			myValidatorErr: fmt.Errorf("not registered"),
		},
		Runner:  newMockRunner(),
		Printer: getPrinter(),
	}

	err := handleValidatorsWithFormat(d, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

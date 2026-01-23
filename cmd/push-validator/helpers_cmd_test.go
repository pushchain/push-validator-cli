package main

import (
	"context"
	"fmt"
	"testing"
)

func TestConvertValidatorToAccountAddress_Success(t *testing.T) {
	runner := newMockRunner()
	binPath := findPchaind()
	key := binPath + " debug addr pushvaloper1abc123"
	runner.outputs[key] = []byte("Address: [106 211 ...]\nAddress (hex): 6AD36CEE5A91\nBech32 Acc: push1dtfkemne22yusl\nBech32 Val: pushvaloper1abc123\n")

	ctx := context.Background()
	result, err := convertValidatorToAccountAddress(ctx, "pushvaloper1abc123", runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "push1dtfkemne22yusl" {
		t.Errorf("result = %q, want push1dtfkemne22yusl", result)
	}
}

func TestConvertValidatorToAccountAddress_RunnerError(t *testing.T) {
	runner := newMockRunner()
	binPath := findPchaind()
	key := binPath + " debug addr pushvaloper1bad"
	runner.errors[key] = fmt.Errorf("exit status 1")

	ctx := context.Background()
	_, err := convertValidatorToAccountAddress(ctx, "pushvaloper1bad", runner)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "failed to convert address") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConvertValidatorToAccountAddress_NilContext(t *testing.T) {
	runner := newMockRunner()
	binPath := findPchaind()
	key := binPath + " debug addr pushvaloper1test"
	runner.outputs[key] = []byte("Address (hex): AABB\nBech32 Acc: push1resolved\n")

	result, err := convertValidatorToAccountAddress(nil, "pushvaloper1test", runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "push1resolved" {
		t.Errorf("result = %q", result)
	}
}

func TestGetEVMAddress_Success(t *testing.T) {
	runner := newMockRunner()
	binPath := findPchaind()
	key := binPath + " debug addr push1abc"
	runner.outputs[key] = []byte("Address (hex): 6AD36CEE5A9113907D\nBech32 Acc: push1abc\n")

	ctx := context.Background()
	result, err := getEVMAddress(ctx, "push1abc", runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "0x6AD36CEE5A9113907D" {
		t.Errorf("result = %q", result)
	}
}

func TestGetEVMAddress_AlreadyHasPrefix(t *testing.T) {
	runner := newMockRunner()
	binPath := findPchaind()
	key := binPath + " debug addr push1xyz"
	runner.outputs[key] = []byte("Address (hex): 0xABCDEF\nBech32 Acc: push1xyz\n")

	ctx := context.Background()
	result, err := getEVMAddress(ctx, "push1xyz", runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Already has 0x prefix, should not double-prefix
	if result != "0xABCDEF" {
		t.Errorf("result = %q", result)
	}
}

func TestGetEVMAddress_RunnerError(t *testing.T) {
	runner := newMockRunner()
	binPath := findPchaind()
	key := binPath + " debug addr push1bad"
	runner.errors[key] = fmt.Errorf("connection error")

	ctx := context.Background()
	_, err := getEVMAddress(ctx, "push1bad", runner)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetEVMAddress_ParseError(t *testing.T) {
	runner := newMockRunner()
	binPath := findPchaind()
	key := binPath + " debug addr push1nohex"
	runner.outputs[key] = []byte("Bech32 Acc: push1nohex\n") // No "Address (hex):" line

	ctx := context.Background()
	_, err := getEVMAddress(ctx, "push1nohex", runner)
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestGetEVMAddress_NilContext(t *testing.T) {
	runner := newMockRunner()
	binPath := findPchaind()
	key := binPath + " debug addr push1nilctx"
	runner.outputs[key] = []byte("Address (hex): DEADBEEF\nBech32 Acc: push1nilctx\n")

	result, err := getEVMAddress(nil, "push1nilctx", runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "0xDEADBEEF" {
		t.Errorf("result = %q, want 0xDEADBEEF", result)
	}
}

func TestGetEVMAddress_NilRunner(t *testing.T) {
	// With nil runner, it falls back to execRunner which will fail in test
	ctx := context.Background()
	_, err := getEVMAddress(ctx, "push1test")
	if err == nil {
		t.Fatal("expected error with nil/default runner")
	}
}

func TestHexToBech32Address_NilContext(t *testing.T) {
	runner := newMockRunner()
	binPath := findPchaind()
	key := binPath + " debug addr CAFE"
	runner.outputs[key] = []byte("Bech32 Acc: push1nilctx\n")

	result, err := hexToBech32Address(nil, "0xCAFE", runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "push1nilctx" {
		t.Errorf("result = %q", result)
	}
}

func TestHexToBech32Address_ParseError(t *testing.T) {
	runner := newMockRunner()
	binPath := findPchaind()
	key := binPath + " debug addr NOFIELD"
	runner.outputs[key] = []byte("Address (hex): NOFIELD\n") // No "Bech32 Acc:" line

	ctx := context.Background()
	_, err := hexToBech32Address(ctx, "NOFIELD", runner)
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestHexToBech32Address_Success(t *testing.T) {
	runner := newMockRunner()
	binPath := findPchaind()
	// hex prefix is stripped before calling runner
	key := binPath + " debug addr 6AD36CEE5A91"
	runner.outputs[key] = []byte("Address (hex): 6AD36CEE5A91\nBech32 Acc: push1converted\n")

	ctx := context.Background()
	result, err := hexToBech32Address(ctx, "0x6AD36CEE5A91", runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "push1converted" {
		t.Errorf("result = %q", result)
	}
}

func TestHexToBech32Address_UppercasePrefix(t *testing.T) {
	runner := newMockRunner()
	binPath := findPchaind()
	key := binPath + " debug addr ABCDEF"
	runner.outputs[key] = []byte("Bech32 Acc: push1upper\n")

	ctx := context.Background()
	result, err := hexToBech32Address(ctx, "0XABCDEF", runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "push1upper" {
		t.Errorf("result = %q", result)
	}
}

func TestHexToBech32Address_NoPrefix(t *testing.T) {
	runner := newMockRunner()
	binPath := findPchaind()
	key := binPath + " debug addr DEADBEEF"
	runner.outputs[key] = []byte("Bech32 Acc: push1noprefix\n")

	ctx := context.Background()
	result, err := hexToBech32Address(ctx, "DEADBEEF", runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "push1noprefix" {
		t.Errorf("result = %q", result)
	}
}

func TestHexToBech32Address_RunnerError(t *testing.T) {
	runner := newMockRunner()
	binPath := findPchaind()
	key := binPath + " debug addr BAD"
	runner.errors[key] = fmt.Errorf("binary not found")

	ctx := context.Background()
	_, err := hexToBech32Address(ctx, "0xBAD", runner)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFindKeyNameByAddress_Success(t *testing.T) {
	runner := newMockRunner()
	binPath := findPchaind()
	cfg := testCfg()
	key := binPath + " keys list --keyring-backend " + cfg.KeyringBackend + " --home " + cfg.HomeDir + " --output json"
	runner.outputs[key] = []byte(`[{"name":"mykey","address":"push1target"},{"name":"other","address":"push1other"}]`)

	ctx := context.Background()
	result, err := findKeyNameByAddress(ctx, cfg, "push1target", runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "mykey" {
		t.Errorf("result = %q, want mykey", result)
	}
}

func TestFindKeyNameByAddress_NotFound(t *testing.T) {
	runner := newMockRunner()
	binPath := findPchaind()
	cfg := testCfg()
	key := binPath + " keys list --keyring-backend " + cfg.KeyringBackend + " --home " + cfg.HomeDir + " --output json"
	runner.outputs[key] = []byte(`[{"name":"mykey","address":"push1other"}]`)

	ctx := context.Background()
	_, err := findKeyNameByAddress(ctx, cfg, "push1target", runner)
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestFindKeyNameByAddress_RunnerError(t *testing.T) {
	runner := newMockRunner()
	binPath := findPchaind()
	cfg := testCfg()
	key := binPath + " keys list --keyring-backend " + cfg.KeyringBackend + " --home " + cfg.HomeDir + " --output json"
	runner.errors[key] = fmt.Errorf("keyring locked")

	ctx := context.Background()
	_, err := findKeyNameByAddress(ctx, cfg, "push1any", runner)
	if err == nil {
		t.Fatal("expected runner error")
	}
	if !containsSubstr(err.Error(), "failed to list keys") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFindKeyNameByAddress_NilContext(t *testing.T) {
	runner := newMockRunner()
	binPath := findPchaind()
	cfg := testCfg()
	key := binPath + " keys list --keyring-backend " + cfg.KeyringBackend + " --home " + cfg.HomeDir + " --output json"
	runner.outputs[key] = []byte(`[{"name":"found","address":"push1addr"}]`)

	result, err := findKeyNameByAddress(nil, cfg, "push1addr", runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "found" {
		t.Errorf("result = %q", result)
	}
}

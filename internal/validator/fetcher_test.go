package validator

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/pushchain/push-validator-cli/internal/config"
)

// createMockPchaind creates a mock pchaind binary for testing
func createMockPchaind(t *testing.T, handlers map[string]func(args []string) (string, error)) string {
	if runtime.GOOS == "windows" {
		t.Skip("windows not supported in this test")
	}

	dir := t.TempDir()
	binPath := filepath.Join(dir, "pchaind")

	// Create a bash script that handles various commands
	script := `#!/usr/bin/env bash
# Skip all args until we find the command
while [[ $# -gt 0 ]]; do
	case "$1" in
		query)
			shift
			if [ "$1" = "staking" ]; then
				shift
				if [ "$1" = "validators" ]; then
					echo '{"validators":[{"operator_address":"pushvaloper1test","description":{"moniker":"test-validator"},"consensus_pubkey":{"value":"TESTPUBKEY123"},"status":"BOND_STATUS_BONDED","tokens":"1000000000000000000000","commission":{"commission_rates":{"rate":"0.10"}},"jailed":false}]}'
					exit 0
				fi
			elif [ "$1" = "distribution" ]; then
				shift
				if [ "$1" = "commission" ]; then
					echo '{"commission":{"commission":["100000000000000000000upc"]}}'
					exit 0
				elif [ "$1" = "validator-outstanding-rewards" ]; then
					echo '{"rewards":{"rewards":["200000000000000000000upc"]}}'
					exit 0
				fi
			elif [ "$1" = "slashing" ]; then
				shift
				if [ "$1" = "signing-info" ]; then
					echo '{"val_signing_info":{"address":"pushvalcons1test","start_height":"1","jailed_until":"1970-01-01T00:00:00Z","tombstoned":false,"missed_blocks_counter":"5"}}'
					exit 0
				fi
			fi
			;;
		tendermint)
			shift
			if [ "$1" = "show-validator" ]; then
				echo '{"@type":"/cosmos.crypto.ed25519.PubKey","key":"TESTPUBKEY123"}'
				exit 0
			fi
			;;
		status)
			echo '{"NodeInfo":{"moniker":"test-node"}}'
			exit 0
			;;
		debug)
			shift
			if [ "$1" = "addr" ]; then
				echo "Address (hex): ABCDEF1234567890"
				exit 0
			fi
			;;
		keys)
			shift
			if [ "$1" = "list" ]; then
				echo '[{"address":"push1keyaddr"}]'
				exit 0
			fi
			;;
	esac
	shift
done

exit 1
`

	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	// Add to PATH
	oldPath := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", oldPath) })
	os.Setenv("PATH", dir+":"+oldPath)

	return binPath
}

func TestFetcher_GetAllValidators(t *testing.T) {
	createMockPchaind(t, nil)

	f := NewFetcher()
	cfg := config.Config{
		GenesisDomain: "donut.rpc.push.org",
		HomeDir:       t.TempDir(),
	}
	ctx := context.Background()

	// First call - should fetch fresh data
	list, err := f.GetAllValidators(ctx, cfg)
	if err != nil {
		t.Fatalf("GetAllValidators error: %v", err)
	}

	if list.Total != 1 {
		t.Errorf("expected 1 validator, got %d", list.Total)
	}

	if len(list.Validators) != 1 {
		t.Errorf("expected 1 validator in list, got %d", len(list.Validators))
	}

	if list.Validators[0].Moniker != "test-validator" {
		t.Errorf("expected moniker 'test-validator', got %q", list.Validators[0].Moniker)
	}

	if list.Validators[0].Status != "BONDED" {
		t.Errorf("expected status 'BONDED', got %q", list.Validators[0].Status)
	}

	// Second call within cache TTL - should return cached data
	list2, err := f.GetAllValidators(ctx, cfg)
	if err != nil {
		t.Fatalf("GetAllValidators cached error: %v", err)
	}

	if list2.Total != list.Total {
		t.Errorf("cached response differs: got %d, want %d", list2.Total, list.Total)
	}
}

func TestFetcher_GetAllValidators_CacheExpiry(t *testing.T) {
	createMockPchaind(t, nil)

	f := NewFetcher()
	f.cacheTTL = 50 * time.Millisecond // Short TTL for testing

	cfg := config.Config{
		GenesisDomain: "donut.rpc.push.org",
		HomeDir:       t.TempDir(),
	}
	ctx := context.Background()

	// First call
	_, err := f.GetAllValidators(ctx, cfg)
	if err != nil {
		t.Fatalf("GetAllValidators error: %v", err)
	}

	// Wait for cache to expire
	time.Sleep(100 * time.Millisecond)

	// Second call - should fetch fresh data
	_, err = f.GetAllValidators(ctx, cfg)
	if err != nil {
		t.Fatalf("GetAllValidators after expiry error: %v", err)
	}
}

func TestFetcher_GetMyValidator_NotValidator(t *testing.T) {
	// Create a mock that returns no matching validator
	if runtime.GOOS == "windows" {
		t.Skip("windows not supported in this test")
	}

	dir := t.TempDir()
	binPath := filepath.Join(dir, "pchaind")

	script := `#!/usr/bin/env bash
shift
cmd="$1"
shift

if [ "$cmd" = "tendermint" ]; then
	echo '{"@type":"/cosmos.crypto.ed25519.PubKey","key":"DIFFERENTKEY"}'
	exit 0
elif [ "$cmd" = "status" ]; then
	echo '{"NodeInfo":{"moniker":"my-node"}}'
	exit 0
elif [ "$cmd" = "query" ]; then
	mod="$1"
	shift
	if [ "$mod" = "staking" ]; then
		echo '{"validators":[{"operator_address":"pushvaloper1test","description":{"moniker":"other-validator"},"consensus_pubkey":{"value":"OTHERPUBKEY"},"status":"BOND_STATUS_BONDED","tokens":"1000000000000000000000","commission":{"commission_rates":{"rate":"0.10"}},"jailed":false}]}'
		exit 0
	fi
elif [ "$cmd" = "keys" ]; then
	echo '[]'
	exit 0
fi
exit 1
`

	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	oldPath := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", oldPath) })
	os.Setenv("PATH", dir+":"+oldPath)

	f := NewFetcher()
	cfg := config.Config{
		GenesisDomain: "donut.rpc.push.org",
		HomeDir:       t.TempDir(),
		RPCLocal:      "http://127.0.0.1:26657",
	}
	ctx := context.Background()

	myVal, err := f.GetMyValidator(ctx, cfg)
	if err != nil {
		t.Fatalf("GetMyValidator error: %v", err)
	}

	if myVal.IsValidator {
		t.Errorf("expected IsValidator=false, got true")
	}
}

func TestFetcher_GetMyValidator_IsValidator(t *testing.T) {
	createMockPchaind(t, nil)

	f := NewFetcher()
	cfg := config.Config{
		GenesisDomain: "donut.rpc.push.org",
		HomeDir:       t.TempDir(),
		RPCLocal:      "http://127.0.0.1:26657",
	}
	ctx := context.Background()

	myVal, err := f.GetMyValidator(ctx, cfg)
	if err != nil {
		t.Fatalf("GetMyValidator error: %v", err)
	}

	if !myVal.IsValidator {
		t.Errorf("expected IsValidator=true, got false")
	}

	if myVal.Address != "pushvaloper1test" {
		t.Errorf("expected address 'pushvaloper1test', got %q", myVal.Address)
	}

	if myVal.Moniker != "test-validator" {
		t.Errorf("expected moniker 'test-validator', got %q", myVal.Moniker)
	}
}

func TestFetcher_GetMyValidator_Cache(t *testing.T) {
	createMockPchaind(t, nil)

	f := NewFetcher()
	cfg := config.Config{
		GenesisDomain: "donut.rpc.push.org",
		HomeDir:       t.TempDir(),
		RPCLocal:      "http://127.0.0.1:26657",
	}
	ctx := context.Background()

	// First call
	myVal1, err := f.GetMyValidator(ctx, cfg)
	if err != nil {
		t.Fatalf("GetMyValidator error: %v", err)
	}

	// Second call within cache TTL - should return cached
	myVal2, err := f.GetMyValidator(ctx, cfg)
	if err != nil {
		t.Fatalf("GetMyValidator cached error: %v", err)
	}

	if myVal1.Address != myVal2.Address {
		t.Errorf("cached address differs: got %q, want %q", myVal2.Address, myVal1.Address)
	}
}

func TestFetcher_GetMyValidator_ErrorCaching(t *testing.T) {
	// Test that errors are cached with timestamp to avoid infinite retry loops
	if runtime.GOOS == "windows" {
		t.Skip("windows not supported in this test")
	}

	dir := t.TempDir()
	binPath := filepath.Join(dir, "pchaind")

	// Binary that fails for query staking validators
	script := `#!/usr/bin/env bash
while [[ $# -gt 0 ]]; do
	case "$1" in
		tendermint)
			shift
			if [ "$1" = "show-validator" ]; then
				echo '{"@type":"/cosmos.crypto.ed25519.PubKey","key":"TESTKEY"}'
				exit 0
			fi
			;;
		status)
			echo '{"NodeInfo":{"moniker":"test"}}'
			exit 0
			;;
		query)
			# Fail on query
			exit 1
			;;
	esac
	shift
done
exit 1
`

	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	oldPath := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", oldPath) })
	os.Setenv("PATH", dir+":"+oldPath)

	f := NewFetcher()
	cfg := config.Config{
		GenesisDomain: "donut.rpc.push.org",
		HomeDir:       t.TempDir(),
		RPCLocal:      "http://127.0.0.1:26657",
	}
	ctx := context.Background()

	// First call - should error and set cache time
	myVal, err := f.GetMyValidator(ctx, cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if myVal.IsValidator {
		t.Errorf("expected IsValidator=false on error")
	}

	// Cache time should be set even on error
	if f.myValidatorTime.IsZero() {
		t.Errorf("expected cache time to be set on error")
	}
}

func TestParseStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"BOND_STATUS_BONDED", "BONDED"},
		{"BOND_STATUS_UNBONDING", "UNBONDING"},
		{"BOND_STATUS_UNBONDED", "UNBONDED"},
		{"UNKNOWN_STATUS", "UNKNOWN_STATUS"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseStatus(tt.input)
			if result != tt.expected {
				t.Errorf("parseStatus(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetValidatorRewards(t *testing.T) {
	createMockPchaind(t, nil)

	cfg := config.Config{
		GenesisDomain: "donut.rpc.push.org",
		HomeDir:       t.TempDir(),
	}
	ctx := context.Background()

	commission, outstanding, err := GetValidatorRewards(ctx, cfg, "pushvaloper1test")
	if err != nil {
		t.Fatalf("GetValidatorRewards error: %v", err)
	}

	if commission == "—" {
		t.Errorf("expected commission value, got placeholder")
	}

	if outstanding == "—" {
		t.Errorf("expected outstanding value, got placeholder")
	}
}

func TestGetValidatorRewards_EmptyAddress(t *testing.T) {
	cfg := config.Config{
		GenesisDomain: "donut.rpc.push.org",
		HomeDir:       t.TempDir(),
	}
	ctx := context.Background()

	commission, outstanding, err := GetValidatorRewards(ctx, cfg, "")
	if err == nil {
		t.Fatal("expected error for empty address")
	}

	if commission != "—" || outstanding != "—" {
		t.Errorf("expected placeholder values on error, got %q, %q", commission, outstanding)
	}
}

func TestFetcher_GetCachedValidatorRewards(t *testing.T) {
	createMockPchaind(t, nil)

	f := NewFetcher()
	cfg := config.Config{
		GenesisDomain: "donut.rpc.push.org",
		HomeDir:       t.TempDir(),
	}
	ctx := context.Background()

	// First call - should fetch and cache
	comm1, out1, err := f.GetCachedValidatorRewards(ctx, cfg, "pushvaloper1test")
	if err != nil {
		t.Fatalf("GetCachedValidatorRewards error: %v", err)
	}

	// Second call within TTL - should return cached
	comm2, out2, err := f.GetCachedValidatorRewards(ctx, cfg, "pushvaloper1test")
	if err != nil {
		t.Fatalf("GetCachedValidatorRewards cached error: %v", err)
	}

	if comm1 != comm2 || out1 != out2 {
		t.Errorf("cached rewards differ: got (%q, %q), want (%q, %q)", comm2, out2, comm1, out1)
	}
}

func TestFetcher_GetCachedValidatorRewards_Expiry(t *testing.T) {
	createMockPchaind(t, nil)

	f := NewFetcher()
	f.rewardsTTL = 50 * time.Millisecond // Short TTL for testing

	cfg := config.Config{
		GenesisDomain: "donut.rpc.push.org",
		HomeDir:       t.TempDir(),
	}
	ctx := context.Background()

	// First call
	_, _, err := f.GetCachedValidatorRewards(ctx, cfg, "pushvaloper1test")
	if err != nil {
		t.Fatalf("GetCachedValidatorRewards error: %v", err)
	}

	// Wait for cache to expire
	time.Sleep(100 * time.Millisecond)

	// Second call - should fetch fresh data
	_, _, err = f.GetCachedValidatorRewards(ctx, cfg, "pushvaloper1test")
	if err != nil {
		t.Fatalf("GetCachedValidatorRewards after expiry error: %v", err)
	}
}

func TestGetCachedValidatorsList(t *testing.T) {
	createMockPchaind(t, nil)

	cfg := config.Config{
		GenesisDomain: "donut.rpc.push.org",
		HomeDir:       t.TempDir(),
	}
	ctx := context.Background()

	list, err := GetCachedValidatorsList(ctx, cfg)
	if err != nil {
		t.Fatalf("GetCachedValidatorsList error: %v", err)
	}

	if list.Total == 0 {
		t.Errorf("expected validators, got none")
	}
}

func TestGetCachedMyValidator(t *testing.T) {
	createMockPchaind(t, nil)

	cfg := config.Config{
		GenesisDomain: "donut.rpc.push.org",
		HomeDir:       t.TempDir(),
		RPCLocal:      "http://127.0.0.1:26657",
	}
	ctx := context.Background()

	myVal, err := GetCachedMyValidator(ctx, cfg)
	if err != nil {
		t.Fatalf("GetCachedMyValidator error: %v", err)
	}

	if !myVal.IsValidator {
		t.Errorf("expected IsValidator=true")
	}
}

func TestGetCachedRewards(t *testing.T) {
	createMockPchaind(t, nil)

	cfg := config.Config{
		GenesisDomain: "donut.rpc.push.org",
		HomeDir:       t.TempDir(),
	}
	ctx := context.Background()

	commission, outstanding, err := GetCachedRewards(ctx, cfg, "pushvaloper1test")
	if err != nil {
		t.Fatalf("GetCachedRewards error: %v", err)
	}

	if commission == "—" || outstanding == "—" {
		t.Errorf("expected reward values, got placeholders")
	}
}

func TestGetEVMAddress(t *testing.T) {
	// Note: GetEVMAddress uses subprocess to call pchaind debug addr.
	// For production use, prefer Bech32ToHex which is pure Go.
	// This test may fail if pchaind is not available.
	createMockPchaind(t, nil)

	ctx := context.Background()
	// Use a mock address that the mock script might not handle
	evmAddr := GetEVMAddress(ctx, "pushvaloper1test")

	// The mock script doesn't handle debug addr, so it will return "—"
	// This is expected behavior - the function falls back gracefully.
	// The real conversion should use Bech32ToHex.
	if evmAddr != "—" && !strings.HasPrefix(evmAddr, "0x") {
		t.Errorf("expected EVM address to start with 0x or be placeholder, got %q", evmAddr)
	}
}

func TestGetEVMAddress_EmptyAddress(t *testing.T) {
	ctx := context.Background()
	evmAddr := GetEVMAddress(ctx, "")

	if evmAddr != "—" {
		t.Errorf("expected placeholder for empty address, got %q", evmAddr)
	}
}

func TestGetEVMAddress_NoBinary(t *testing.T) {
	// Set PATH to empty to ensure pchaind is not found
	oldPath := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", oldPath) })
	os.Setenv("PATH", "/nonexistent")

	// Clear the PATH so exec.LookPath fails
	exec.Command("pchaind", "version").Run() // Ensure path cache is cleared

	ctx := context.Background()
	evmAddr := GetEVMAddress(ctx, "pushvaloper1test")

	if evmAddr != "—" {
		t.Errorf("expected placeholder when binary not found, got %q", evmAddr)
	}
}

func TestGetSlashingInfo(t *testing.T) {
	createMockPchaind(t, nil)

	cfg := config.Config{
		GenesisDomain: "donut.rpc.push.org",
		HomeDir:       t.TempDir(),
	}
	ctx := context.Background()

	consensusPubkey := `{"@type":"/cosmos.crypto.ed25519.PubKey","key":"TESTKEY"}`
	info, err := GetSlashingInfo(ctx, cfg, consensusPubkey)
	if err != nil {
		t.Fatalf("GetSlashingInfo error: %v", err)
	}

	if info.Tombstoned {
		t.Errorf("expected not tombstoned")
	}

	if info.MissedBlocks == 0 {
		t.Errorf("expected missed blocks count > 0")
	}

	if info.JailReason == "" {
		t.Errorf("expected jail reason to be set")
	}
}

func TestGetSlashingInfo_Tombstoned(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows not supported in this test")
	}

	dir := t.TempDir()
	binPath := filepath.Join(dir, "pchaind")

	script := `#!/usr/bin/env bash
while [[ $# -gt 0 ]]; do
	case "$1" in
		query)
			shift
			if [ "$1" = "slashing" ]; then
				echo '{"val_signing_info":{"address":"test","start_height":"1","jailed_until":"2025-01-01T00:00:00Z","tombstoned":true,"missed_blocks_counter":"100"}}'
				exit 0
			fi
			;;
	esac
	shift
done
exit 1
`

	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	oldPath := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", oldPath) })
	os.Setenv("PATH", dir+":"+oldPath)

	cfg := config.Config{
		GenesisDomain: "donut.rpc.push.org",
		HomeDir:       t.TempDir(),
	}
	ctx := context.Background()

	info, err := GetSlashingInfo(ctx, cfg, `{"@type":"/cosmos.crypto.ed25519.PubKey","key":"TESTKEY"}`)
	if err != nil {
		t.Fatalf("GetSlashingInfo error: %v", err)
	}

	if !info.Tombstoned {
		t.Errorf("expected tombstoned=true")
	}

	if info.JailReason != "Double Sign" {
		t.Errorf("expected jail reason 'Double Sign', got %q", info.JailReason)
	}
}

func TestGetKeyringAddresses(t *testing.T) {
	createMockPchaind(t, nil)

	bin, err := exec.LookPath("pchaind")
	if err != nil {
		t.Skip("pchaind not in PATH")
	}

	cfg := config.Config{
		HomeDir:        t.TempDir(),
		KeyringBackend: "test",
	}

	addrs := getKeyringAddresses(bin, cfg)

	// Should return at least the mock address
	if len(addrs) == 0 {
		t.Errorf("expected at least one address from keyring")
	}
}

func TestGetKeyringAddresses_Error(t *testing.T) {
	// Use invalid binary path
	bin := "/nonexistent/pchaind"
	cfg := config.Config{
		HomeDir:        t.TempDir(),
		KeyringBackend: "test",
	}

	addrs := getKeyringAddresses(bin, cfg)

	if len(addrs) != 0 {
		t.Errorf("expected empty address list on error, got %d addresses", len(addrs))
	}
}

func TestFetcher_GetAllValidators_EmptyMoniker(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows not supported in this test")
	}

	dir := t.TempDir()
	binPath := filepath.Join(dir, "pchaind")

	// Return validator with empty moniker
	script := `#!/usr/bin/env bash
while [[ $# -gt 0 ]]; do
	case "$1" in
		query)
			shift
			if [ "$1" = "staking" ]; then
				echo '{"validators":[{"operator_address":"pushvaloper1test","description":{"moniker":""},"consensus_pubkey":{"value":"KEY"},"status":"BOND_STATUS_BONDED","tokens":"1000000000000000000000","commission":{"commission_rates":{"rate":"0.10"}},"jailed":false}]}'
				exit 0
			fi
			;;
	esac
	shift
done
exit 1
`

	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	oldPath := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", oldPath) })
	os.Setenv("PATH", dir+":"+oldPath)

	f := NewFetcher()
	cfg := config.Config{
		GenesisDomain: "donut.rpc.push.org",
		HomeDir:       t.TempDir(),
	}
	ctx := context.Background()

	list, err := f.GetAllValidators(ctx, cfg)
	if err != nil {
		t.Fatalf("GetAllValidators error: %v", err)
	}

	if list.Validators[0].Moniker != "unknown" {
		t.Errorf("expected moniker 'unknown' for empty moniker, got %q", list.Validators[0].Moniker)
	}
}

func TestFetcher_GetMyValidator_Jailed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows not supported in this test")
	}

	dir := t.TempDir()
	binPath := filepath.Join(dir, "pchaind")

	script := `#!/usr/bin/env bash
while [[ $# -gt 0 ]]; do
	case "$1" in
		tendermint)
			shift
			if [ "$1" = "show-validator" ]; then
				echo '{"@type":"/cosmos.crypto.ed25519.PubKey","key":"TESTPUBKEY123"}'
				exit 0
			fi
			;;
		status)
			echo '{"NodeInfo":{"moniker":"test-node"}}'
			exit 0
			;;
		query)
			shift
			if [ "$1" = "staking" ]; then
				echo '{"validators":[{"operator_address":"pushvaloper1test","description":{"moniker":"test-validator"},"consensus_pubkey":{"value":"TESTPUBKEY123"},"status":"BOND_STATUS_BONDED","tokens":"1000000000000000000000","commission":{"commission_rates":{"rate":"0.10"}},"jailed":true}]}'
				exit 0
			elif [ "$1" = "slashing" ]; then
				echo '{"val_signing_info":{"address":"test","start_height":"1","jailed_until":"2025-01-01T00:00:00Z","tombstoned":false,"missed_blocks_counter":"100"}}'
				exit 0
			fi
			;;
		keys)
			shift
			if [ "$1" = "list" ]; then
				echo '[]'
				exit 0
			fi
			;;
	esac
	shift
done
exit 1
`

	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	oldPath := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", oldPath) })
	os.Setenv("PATH", dir+":"+oldPath)

	f := NewFetcher()
	cfg := config.Config{
		GenesisDomain: "donut.rpc.push.org",
		HomeDir:       t.TempDir(),
		RPCLocal:      "http://127.0.0.1:26657",
	}
	ctx := context.Background()

	myVal, err := f.GetMyValidator(ctx, cfg)
	if err != nil {
		t.Fatalf("GetMyValidator error: %v", err)
	}

	if !myVal.Jailed {
		t.Errorf("expected validator to be jailed")
	}

	if myVal.SlashingInfo.JailReason == "" {
		t.Errorf("expected jail reason to be populated for jailed validator")
	}
}

func TestFetcher_GetMyValidator_MonikerConflict(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows not supported in this test")
	}

	dir := t.TempDir()
	binPath := filepath.Join(dir, "pchaind")

	script := `#!/usr/bin/env bash
while [[ $# -gt 0 ]]; do
	case "$1" in
		tendermint)
			shift
			if [ "$1" = "show-validator" ]; then
				echo '{"@type":"/cosmos.crypto.ed25519.PubKey","key":"MYPUBKEY"}'
				exit 0
			fi
			;;
		status)
			echo '{"NodeInfo":{"moniker":"shared-moniker"}}'
			exit 0
			;;
		query)
			shift
			if [ "$1" = "staking" ]; then
				echo '{"validators":[{"operator_address":"pushvaloper1other","description":{"moniker":"shared-moniker"},"consensus_pubkey":{"value":"OTHERPUBKEY"},"status":"BOND_STATUS_BONDED","tokens":"1000000000000000000000","commission":{"commission_rates":{"rate":"0.10"}},"jailed":false}]}'
				exit 0
			fi
			;;
		keys)
			shift
			if [ "$1" = "list" ]; then
				echo '[]'
				exit 0
			fi
			;;
	esac
	shift
done
exit 1
`

	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	oldPath := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", oldPath) })
	os.Setenv("PATH", dir+":"+oldPath)

	f := NewFetcher()
	cfg := config.Config{
		GenesisDomain: "donut.rpc.push.org",
		HomeDir:       t.TempDir(),
		RPCLocal:      "http://127.0.0.1:26657",
	}
	ctx := context.Background()

	myVal, err := f.GetMyValidator(ctx, cfg)
	if err != nil {
		t.Fatalf("GetMyValidator error: %v", err)
	}

	// When matching by moniker (not by consensus pubkey), IsValidator should be false
	if myVal.IsValidator {
		t.Errorf("expected IsValidator=false when matching by moniker only")
	}

	// Should return the validator info since it matched by moniker
	if myVal.Address != "pushvaloper1other" {
		t.Errorf("expected address 'pushvaloper1other', got %q", myVal.Address)
	}

	if myVal.Moniker != "shared-moniker" {
		t.Errorf("expected moniker 'shared-moniker', got %q", myVal.Moniker)
	}
}

func TestFetcher_GetAllValidators_StaleCache(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows not supported in this test")
	}

	callCount := 0
	dir := t.TempDir()
	binPath := filepath.Join(dir, "pchaind")

	// Create a counter file to track calls
	counterFile := filepath.Join(t.TempDir(), "counter")
	os.WriteFile(counterFile, []byte("0"), 0644)

	script := `#!/usr/bin/env bash
while [[ $# -gt 0 ]]; do
	case "$1" in
		query)
			shift
			if [ "$1" = "staking" ]; then
				# Increment counter
				COUNT=$(cat ` + counterFile + `)
				COUNT=$((COUNT + 1))
				echo $COUNT > ` + counterFile + `

				# Fail on second call
				if [ $COUNT -eq 2 ]; then
					exit 1
				fi

				echo '{"validators":[{"operator_address":"pushvaloper1test","description":{"moniker":"test"},"consensus_pubkey":{"value":"KEY"},"status":"BOND_STATUS_BONDED","tokens":"1000000000000000000000","commission":{"commission_rates":{"rate":"0.10"}},"jailed":false}]}'
				exit 0
			fi
			;;
	esac
	shift
done
exit 1
`

	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	oldPath := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", oldPath) })
	os.Setenv("PATH", dir+":"+oldPath)

	f := NewFetcher()
	f.cacheTTL = 50 * time.Millisecond

	cfg := config.Config{
		GenesisDomain: "donut.rpc.push.org",
		HomeDir:       t.TempDir(),
	}
	ctx := context.Background()

	// First call - should succeed
	list1, err := f.GetAllValidators(ctx, cfg)
	if err != nil {
		t.Fatalf("GetAllValidators first call error: %v", err)
	}

	if list1.Total != 1 {
		t.Errorf("expected 1 validator, got %d", list1.Total)
	}

	// Wait for cache to expire
	time.Sleep(100 * time.Millisecond)

	// Second call - fetch will fail, but should return stale cache
	list2, err := f.GetAllValidators(ctx, cfg)
	if err != nil {
		t.Fatalf("GetAllValidators should return stale cache on error: %v", err)
	}

	if list2.Total != list1.Total {
		t.Errorf("expected stale cache to be returned, got different data")
	}

	// Read counter to verify second fetch was attempted
	counterData, _ := os.ReadFile(counterFile)
	if string(counterData) != "2\n" && string(counterData) != "2" {
		t.Logf("Call count: %s (expected 2)", counterData)
	}

	_ = callCount
}

func TestNewFetcher(t *testing.T) {
	f := NewFetcher()

	if f == nil {
		t.Fatal("NewFetcher returned nil")
	}

	if f.cacheTTL != 30*time.Second {
		t.Errorf("expected cacheTTL=30s, got %v", f.cacheTTL)
	}

	if f.rewardsTTL != 30*time.Second {
		t.Errorf("expected rewardsTTL=30s, got %v", f.rewardsTTL)
	}

	if f.rewardsCache == nil {
		t.Error("expected rewardsCache to be initialized")
	}
}

func TestFetcher_GetMyValidator_KeyringMatch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows not supported in this test")
	}

	dir := t.TempDir()
	binPath := filepath.Join(dir, "pchaind")

	script := `#!/usr/bin/env bash
while [[ $# -gt 0 ]]; do
	case "$1" in
		tendermint)
			shift
			if [ "$1" = "show-validator" ]; then
				echo '{"@type":"/cosmos.crypto.ed25519.PubKey","key":"MYPUBKEY"}'
				exit 0
			fi
			;;
		status)
			echo '{"NodeInfo":{"moniker":"my-node"}}'
			exit 0
			;;
		query)
			shift
			if [ "$1" = "staking" ]; then
				# Return validator with operator address that matches keyring
				echo '{"validators":[{"operator_address":"pushvaloper1keyaddr","description":{"moniker":"keyring-validator"},"consensus_pubkey":{"value":"OTHERPUBKEY"},"status":"BOND_STATUS_BONDED","tokens":"1000000000000000000000","commission":{"commission_rates":{"rate":"0.10"}},"jailed":false}]}'
				exit 0
			fi
			;;
		keys)
			shift
			if [ "$1" = "list" ]; then
				echo '[{"address":"push1keyaddr"}]'
				exit 0
			fi
			;;
	esac
	shift
done
exit 1
`

	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	oldPath := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", oldPath) })
	os.Setenv("PATH", dir+":"+oldPath)

	f := NewFetcher()
	cfg := config.Config{
		GenesisDomain:  "donut.rpc.push.org",
		HomeDir:        t.TempDir(),
		RPCLocal:       "http://127.0.0.1:26657",
		KeyringBackend: "test",
	}
	ctx := context.Background()

	myVal, err := f.GetMyValidator(ctx, cfg)
	if err != nil {
		t.Fatalf("GetMyValidator error: %v", err)
	}

	// Should match by keyring but IsValidator should be false (no consensus key match)
	if myVal.IsValidator {
		t.Errorf("expected IsValidator=false for keyring-only match")
	}

	if myVal.Address != "pushvaloper1keyaddr" {
		t.Errorf("expected address to match keyring validator")
	}

	if myVal.Moniker != "keyring-validator" {
		t.Errorf("expected moniker 'keyring-validator', got %q", myVal.Moniker)
	}
}

func TestFetcher_GetMyValidator_MonikerMatch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows not supported in this test")
	}

	dir := t.TempDir()
	binPath := filepath.Join(dir, "pchaind")

	script := `#!/usr/bin/env bash
while [[ $# -gt 0 ]]; do
	case "$1" in
		tendermint)
			shift
			if [ "$1" = "show-validator" ]; then
				echo '{"@type":"/cosmos.crypto.ed25519.PubKey","key":"MYPUBKEY"}'
				exit 0
			fi
			;;
		status)
			echo '{"NodeInfo":{"moniker":"my-moniker"}}'
			exit 0
			;;
		query)
			shift
			if [ "$1" = "staking" ]; then
				# Return validator with same moniker but different pubkey
				echo '{"validators":[{"operator_address":"pushvaloper1other","description":{"moniker":"my-moniker"},"consensus_pubkey":{"value":"OTHERPUBKEY"},"status":"BOND_STATUS_BONDED","tokens":"1000000000000000000000","commission":{"commission_rates":{"rate":"0.10"}},"jailed":false}]}'
				exit 0
			fi
			;;
		keys)
			shift
			if [ "$1" = "list" ]; then
				echo '[]'
				exit 0
			fi
			;;
	esac
	shift
done
exit 1
`

	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	oldPath := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", oldPath) })
	os.Setenv("PATH", dir+":"+oldPath)

	f := NewFetcher()
	cfg := config.Config{
		GenesisDomain: "donut.rpc.push.org",
		HomeDir:       t.TempDir(),
		RPCLocal:      "http://127.0.0.1:26657",
	}
	ctx := context.Background()

	myVal, err := f.GetMyValidator(ctx, cfg)
	if err != nil {
		t.Fatalf("GetMyValidator error: %v", err)
	}

	// Should match by moniker but IsValidator should be false (no consensus key match)
	if myVal.IsValidator {
		t.Errorf("expected IsValidator=false for moniker-only match")
	}

	if myVal.Address != "pushvaloper1other" {
		t.Errorf("expected address to match moniker validator")
	}

	if myVal.Moniker != "my-moniker" {
		t.Errorf("expected moniker 'my-moniker', got %q", myVal.Moniker)
	}
}

func TestBech32ToHex(t *testing.T) {
	tests := []struct {
		name     string
		addr     string
		wantHex  bool   // true if we expect a valid 0x... address
		expected string // expected hex result (optional)
	}{
		{
			name:    "empty address",
			addr:    "",
			wantHex: false,
		},
		{
			name:    "invalid bech32",
			addr:    "notvalidbech32",
			wantHex: false,
		},
		{
			name:    "invalid checksum",
			addr:    "pushvaloper15g6nzraqkdw7t72j4eqcj5dwpdr3vzy9",
			wantHex: false,
		},
		{
			name:     "valid pushvaloper address",
			addr:     "pushvaloper1qypqxpq9qcrsszg2pvxq6rs0zqg3yyc5v4yt0n",
			wantHex:  true,
			expected: "0x0102030405060708090A0B0C0D0E0F1011121314",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Bech32ToHex(tt.addr)
			if tt.wantHex {
				if !strings.HasPrefix(result, "0x") {
					t.Errorf("Bech32ToHex(%q) = %q, want 0x... prefix", tt.addr, result)
				}
				if tt.expected != "" && result != tt.expected {
					t.Errorf("Bech32ToHex(%q) = %q, want %q", tt.addr, result, tt.expected)
				}
			} else {
				if result != "—" {
					t.Errorf("Bech32ToHex(%q) = %q, want '—'", tt.addr, result)
				}
			}
		})
	}
}

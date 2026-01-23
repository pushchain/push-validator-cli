package validator

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Creates a fake pchaind executable that responds to the minimal subset of commands
// used by the validator service.
func makeFakePchaind(t *testing.T) string {
	dir := t.TempDir()
	bin := filepath.Join(dir, "pchaind")
	script := "#!/usr/bin/env sh\n" +
		"cmd=\"$1\"; shift\n" +
		"if [ \"$cmd\" = \"tendermint\" ]; then sub=\"$1\"; shift; if [ \"$sub\" = \"show-validator\" ]; then echo '{\"type\":\"tendermint/PubKeyEd25519\",\"key\":\"PUBKEYBASE64\"}'; exit 0; fi; fi\n" +
		"if [ \"$cmd\" = \"keys\" ]; then sub=\"$1\"; shift\n" +
		"  if [ \"$sub\" = \"show\" ]; then\n" +
		"    if [ \"$1\" = \"-o\" ] && [ \"$2\" = \"json\" ]; then\n" +
		"      echo '{\"name\":\"test-key\",\"type\":\"local\",\"address\":\"push1addrxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx\",\"pubkey\":{\"@type\":\"/cosmos.crypto.secp256k1.PubKey\",\"key\":\"AAAA\"}}'\n" +
		"    else\n" +
		"      echo 'push1addrxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx'\n" +
		"    fi\n" +
		"    exit 0\n" +
		"  fi\n" +
		"  if [ \"$sub\" = \"add\" ]; then exit 0; fi\n" +
		"fi\n" +
		"if [ \"$cmd\" = \"query\" ]; then mod=\"$1\"; shift; if [ \"$mod\" = \"bank\" ]; then echo '{\"balances\":[{\"denom\":\"upc\",\"amount\":\"999\"}]}' ; exit 0; fi; if [ \"$mod\" = \"staking\" ]; then echo '{\"validators\":[]}' ; exit 0; fi; fi\n" +
		"if [ \"$cmd\" = \"tx\" ]; then mod=\"$1\"; shift; if [ \"$mod\" = \"staking\" ]; then echo 'txhash: 0xABCD'; exit 0; fi; fi\n" +
		"echo 'unknown'; exit 1\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		t.Skip("windows not supported in this test")
	}
	return bin
}

func TestValidator_RegisterHappyPath(t *testing.T) {
	bin := makeFakePchaind(t)
	home := t.TempDir()
	s := NewWith(Options{
		BinPath:       bin,
		HomeDir:       home,
		ChainID:       "push_42101-1",
		Keyring:       "test",
		GenesisDomain: "donut.rpc.push.org",
		Denom:         "upc",
	})
	ctx := context.Background()
	// EnsureKey should return the fake key info
	keyInfo, err := s.EnsureKey(ctx, "validator-key")
	if err != nil {
		t.Fatal(err)
	}
	if keyInfo.Address == "" {
		t.Fatal("empty address")
	}
	// IsValidator should be false (no validators in fake output)
	ok, err := s.IsValidator(ctx, keyInfo.Address)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected not a validator")
	}
	// Balance should parse
	bal, err := s.Balance(ctx, keyInfo.Address)
	if err != nil {
		t.Fatal(err)
	}
	if bal != "999" {
		t.Fatalf("balance got %s", bal)
	}
	// Register should return txhash
	tx, err := s.Register(ctx, RegisterArgs{Moniker: "m", Amount: "1500000000000000000", KeyName: "validator-key"})
	if err != nil {
		t.Fatal(err)
	}
	if tx == "" {
		t.Fatal("empty txhash")
	}
}

func TestValidator_EnsureKey_EmptyName(t *testing.T) {
	bin := makeFakePchaind(t)
	home := t.TempDir()
	s := NewWith(Options{
		BinPath: bin,
		HomeDir: home,
		Keyring: "test",
	})
	ctx := context.Background()

	_, err := s.EnsureKey(ctx, "")
	if err == nil {
		t.Fatal("EnsureKey with empty name should return error")
	}
}

func TestValidator_GetEVMAddress(t *testing.T) {
	bin := makeFakePchaind(t)
	home := t.TempDir()

	// Update fake script to handle debug addr command
	script := "#!/usr/bin/env sh\n" +
		"cmd=\"$1\"; shift\n" +
		"if [ \"$cmd\" = \"debug\" ]; then sub=\"$1\"; shift\n" +
		"  if [ \"$sub\" = \"addr\" ]; then echo 'Address (hex): ABCDEF1234567890'; exit 0; fi\n" +
		"fi\n" +
		"echo 'unknown'; exit 1\n"
	bin = filepath.Join(t.TempDir(), "pchaind")
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	s := NewWith(Options{
		BinPath: bin,
		HomeDir: home,
	})
	ctx := context.Background()

	evmAddr, err := s.GetEVMAddress(ctx, "push1test")
	if err != nil {
		t.Fatalf("GetEVMAddress error: %v", err)
	}

	if evmAddr != "0xABCDEF1234567890" {
		t.Errorf("GetEVMAddress = %q, want %q", evmAddr, "0xABCDEF1234567890")
	}
}

func TestValidator_GetEVMAddress_EmptyAddr(t *testing.T) {
	bin := makeFakePchaind(t)
	home := t.TempDir()
	s := NewWith(Options{
		BinPath: bin,
		HomeDir: home,
	})
	ctx := context.Background()

	_, err := s.GetEVMAddress(ctx, "")
	if err == nil {
		t.Fatal("GetEVMAddress with empty address should return error")
	}
}

func TestValidator_Balance_ZeroBalance(t *testing.T) {
	bin := makeFakePchaind(t)
	home := t.TempDir()

	// Update script to return empty balances
	script := "#!/usr/bin/env sh\n" +
		"cmd=\"$1\"; shift\n" +
		"if [ \"$cmd\" = \"query\" ]; then mod=\"$1\"; shift\n" +
		"  if [ \"$mod\" = \"bank\" ]; then echo '{\"balances\":[]}'; exit 0; fi\n" +
		"fi\n" +
		"echo 'unknown'; exit 1\n"
	bin = filepath.Join(t.TempDir(), "pchaind")
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	s := NewWith(Options{
		BinPath:       bin,
		HomeDir:       home,
		GenesisDomain: "donut.rpc.push.org",
		Denom:         "upc",
	})
	ctx := context.Background()

	bal, err := s.Balance(ctx, "push1test")
	if err != nil {
		t.Fatalf("Balance error: %v", err)
	}

	if bal != "0" {
		t.Errorf("Balance = %q, want %q", bal, "0")
	}
}

func TestValidator_ValidateMnemonic(t *testing.T) {
	tests := []struct {
		name     string
		mnemonic string
		wantErr  bool
	}{
		{
			name:     "valid 12 words",
			mnemonic: "word word word word word word word word word word word word",
			wantErr:  false,
		},
		{
			name:     "valid 24 words",
			mnemonic: "word word word word word word word word word word word word word word word word word word word word word word word word",
			wantErr:  false,
		},
		{
			name:     "invalid word count",
			mnemonic: "word word word",
			wantErr:  true,
		},
		{
			name:     "word too short",
			mnemonic: "ab word word word word word word word word word word word",
			wantErr:  true,
		},
		{
			name:     "word too long",
			mnemonic: "toolongword word word word word word word word word word word word",
			wantErr:  true,
		},
		{
			name:     "invalid characters",
			mnemonic: "Word word word word word word word word word word word word",
			wantErr:  true,
		},
		{
			name:     "numbers in word",
			mnemonic: "word1 word word word word word word word word word word word",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMnemonic(tt.mnemonic)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateMnemonic() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidator_ImportKey(t *testing.T) {
	bin := makeFakePchaind(t)
	home := t.TempDir()
	s := NewWith(Options{
		BinPath: bin,
		HomeDir: home,
		Keyring: "test",
	})
	ctx := context.Background()

	// The fake binary doesn't handle --recover properly, but we can test the error paths
	_, err := s.ImportKey(ctx, "", "word word word word word word word word word word word word")
	if err == nil {
		t.Fatal("ImportKey with empty name should return error")
	}

	_, err = s.ImportKey(ctx, "test-key", "")
	if err == nil {
		t.Fatal("ImportKey with empty mnemonic should return error")
	}
}

func TestValidator_Unjail(t *testing.T) {
	bin := makeFakePchaind(t)
	home := t.TempDir()

	// Update script to handle unjail command
	script := "#!/usr/bin/env sh\n" +
		"cmd=\"$1\"; shift\n" +
		"if [ \"$cmd\" = \"tx\" ]; then mod=\"$1\"; shift\n" +
		"  if [ \"$mod\" = \"slashing\" ]; then echo 'txhash: 0xUNJAIL'; exit 0; fi\n" +
		"fi\n" +
		"echo 'unknown'; exit 1\n"
	bin = filepath.Join(t.TempDir(), "pchaind")
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	s := NewWith(Options{
		BinPath:       bin,
		HomeDir:       home,
		ChainID:       "push_42101-1",
		Keyring:       "test",
		GenesisDomain: "donut.rpc.push.org",
		Denom:         "upc",
	})
	ctx := context.Background()

	tx, err := s.Unjail(ctx, "validator-key")
	if err != nil {
		t.Fatalf("Unjail error: %v", err)
	}

	if tx != "0xUNJAIL" {
		t.Errorf("Unjail txhash = %q, want %q", tx, "0xUNJAIL")
	}
}

func TestValidator_Unjail_EmptyKeyName(t *testing.T) {
	bin := makeFakePchaind(t)
	home := t.TempDir()
	s := NewWith(Options{
		BinPath: bin,
		HomeDir: home,
	})
	ctx := context.Background()

	_, err := s.Unjail(ctx, "")
	if err == nil {
		t.Fatal("Unjail with empty key name should return error")
	}
}

func TestValidator_WithdrawRewards(t *testing.T) {
	bin := makeFakePchaind(t)
	home := t.TempDir()

	// Update script to handle withdraw rewards command
	script := "#!/usr/bin/env sh\n" +
		"cmd=\"$1\"; shift\n" +
		"if [ \"$cmd\" = \"tx\" ]; then mod=\"$1\"; shift\n" +
		"  if [ \"$mod\" = \"distribution\" ]; then echo 'txhash: 0xREWARDS'; exit 0; fi\n" +
		"fi\n" +
		"echo 'unknown'; exit 1\n"
	bin = filepath.Join(t.TempDir(), "pchaind")
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	s := NewWith(Options{
		BinPath:       bin,
		HomeDir:       home,
		ChainID:       "push_42101-1",
		Keyring:       "test",
		GenesisDomain: "donut.rpc.push.org",
		Denom:         "upc",
	})
	ctx := context.Background()

	tx, err := s.WithdrawRewards(ctx, "pushvaloper1test", "validator-key", false)
	if err != nil {
		t.Fatalf("WithdrawRewards error: %v", err)
	}

	if tx != "0xREWARDS" {
		t.Errorf("WithdrawRewards txhash = %q, want %q", tx, "0xREWARDS")
	}
}

func TestValidator_WithdrawRewards_WithCommission(t *testing.T) {
	bin := makeFakePchaind(t)
	home := t.TempDir()

	script := "#!/usr/bin/env sh\n" +
		"cmd=\"$1\"; shift\n" +
		"if [ \"$cmd\" = \"tx\" ]; then mod=\"$1\"; shift\n" +
		"  if [ \"$mod\" = \"distribution\" ]; then echo 'txhash: 0xCOMMISSION'; exit 0; fi\n" +
		"fi\n" +
		"echo 'unknown'; exit 1\n"
	bin = filepath.Join(t.TempDir(), "pchaind")
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	s := NewWith(Options{
		BinPath:       bin,
		HomeDir:       home,
		ChainID:       "push_42101-1",
		Keyring:       "test",
		GenesisDomain: "donut.rpc.push.org",
		Denom:         "upc",
	})
	ctx := context.Background()

	tx, err := s.WithdrawRewards(ctx, "pushvaloper1test", "validator-key", true)
	if err != nil {
		t.Fatalf("WithdrawRewards with commission error: %v", err)
	}

	if tx != "0xCOMMISSION" {
		t.Errorf("WithdrawRewards txhash = %q, want %q", tx, "0xCOMMISSION")
	}
}

func TestValidator_WithdrawRewards_EmptyValidatorAddr(t *testing.T) {
	bin := makeFakePchaind(t)
	home := t.TempDir()
	s := NewWith(Options{
		BinPath: bin,
		HomeDir: home,
	})
	ctx := context.Background()

	_, err := s.WithdrawRewards(ctx, "", "validator-key", false)
	if err == nil {
		t.Fatal("WithdrawRewards with empty validator address should return error")
	}
}

func TestValidator_WithdrawRewards_EmptyKeyName(t *testing.T) {
	bin := makeFakePchaind(t)
	home := t.TempDir()
	s := NewWith(Options{
		BinPath: bin,
		HomeDir: home,
	})
	ctx := context.Background()

	_, err := s.WithdrawRewards(ctx, "pushvaloper1test", "", false)
	if err == nil {
		t.Fatal("WithdrawRewards with empty key name should return error")
	}
}

func TestValidator_Delegate(t *testing.T) {
	bin := makeFakePchaind(t)
	home := t.TempDir()

	// Update script to handle delegate command
	script := "#!/usr/bin/env sh\n" +
		"cmd=\"$1\"; shift\n" +
		"if [ \"$cmd\" = \"tx\" ]; then mod=\"$1\"; shift\n" +
		"  if [ \"$mod\" = \"staking\" ]; then sub=\"$1\"; shift\n" +
		"    if [ \"$sub\" = \"delegate\" ]; then echo 'txhash: 0xDELEGATE'; exit 0; fi\n" +
		"  fi\n" +
		"fi\n" +
		"echo 'unknown'; exit 1\n"
	bin = filepath.Join(t.TempDir(), "pchaind")
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	s := NewWith(Options{
		BinPath:       bin,
		HomeDir:       home,
		ChainID:       "push_42101-1",
		Keyring:       "test",
		GenesisDomain: "donut.rpc.push.org",
		Denom:         "upc",
	})
	ctx := context.Background()

	tx, err := s.Delegate(ctx, DelegateArgs{
		ValidatorAddress: "pushvaloper1test",
		Amount:           "1000000",
		KeyName:          "validator-key",
	})
	if err != nil {
		t.Fatalf("Delegate error: %v", err)
	}

	if tx != "0xDELEGATE" {
		t.Errorf("Delegate txhash = %q, want %q", tx, "0xDELEGATE")
	}
}

func TestValidator_Delegate_EmptyValidatorAddress(t *testing.T) {
	bin := makeFakePchaind(t)
	home := t.TempDir()
	s := NewWith(Options{
		BinPath: bin,
		HomeDir: home,
	})
	ctx := context.Background()

	_, err := s.Delegate(ctx, DelegateArgs{
		ValidatorAddress: "",
		Amount:           "1000000",
		KeyName:          "validator-key",
	})
	if err == nil {
		t.Fatal("Delegate with empty validator address should return error")
	}
}

func TestValidator_Delegate_EmptyAmount(t *testing.T) {
	bin := makeFakePchaind(t)
	home := t.TempDir()
	s := NewWith(Options{
		BinPath: bin,
		HomeDir: home,
	})
	ctx := context.Background()

	_, err := s.Delegate(ctx, DelegateArgs{
		ValidatorAddress: "pushvaloper1test",
		Amount:           "",
		KeyName:          "validator-key",
	})
	if err == nil {
		t.Fatal("Delegate with empty amount should return error")
	}
}

func TestImproveRewardErrorMessage(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no delegation info",
			input: "error: no delegation distribution info",
			want:  "No rewards to withdraw. This is normal for new validators that haven't earned any rewards yet.",
		},
		{
			name:  "insufficient fees",
			input: "insufficient balance for fee",
			want:  "Insufficient balance to pay transaction fees. Check your account balance.",
		},
		{
			name:  "invalid coins",
			input: "invalid coins",
			want:  "No rewards available to withdraw.",
		},
		{
			name:  "unauthorized",
			input: "unauthorized to sign",
			want:  "Transaction signing failed. Check that the key exists and is accessible.",
		},
		{
			name:  "unknown error",
			input: "some random error",
			want:  "some random error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := improveRewardErrorMessage(tt.input)
			if got != tt.want {
				t.Errorf("improveRewardErrorMessage(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractMnemonic(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   string
	}{
		{
			name: "standard output",
			output: `**Important** write this mnemonic phrase in a safe place.

word one two three four five six seven eight nine ten eleven twelve`,
			want: "word one two three four five six seven eight nine ten eleven twelve",
		},
		{
			name: "with extra warnings",
			output: `**Important** write this mnemonic phrase in a safe place.
It is the only way to recover your account if you ever forget your password.

word one two three four five six seven eight nine ten eleven twelve`,
			want: "word one two three four five six seven eight nine ten eleven twelve",
		},
		{
			name:   "no mnemonic",
			output: "some other output",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractMnemonic(tt.output)
			if got != tt.want {
				t.Errorf("extractMnemonic() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidator_EnsureKey_ExistingKey(t *testing.T) {
	bin := makeFakePchaind(t)
	home := t.TempDir()
	s := NewWith(Options{
		BinPath: bin,
		HomeDir: home,
		Keyring: "test",
	})
	ctx := context.Background()

	// First call creates the key
	keyInfo1, err := s.EnsureKey(ctx, "existing-key")
	if err != nil {
		t.Fatalf("EnsureKey first call error: %v", err)
	}

	if keyInfo1.Address == "" {
		t.Error("expected address to be set")
	}

	// Second call should return existing key without creating new one
	keyInfo2, err := s.EnsureKey(ctx, "existing-key")
	if err != nil {
		t.Fatalf("EnsureKey second call error: %v", err)
	}

	if keyInfo2.Address != keyInfo1.Address {
		t.Errorf("expected same address on second call, got %q vs %q", keyInfo2.Address, keyInfo1.Address)
	}

	// Mnemonic should only be set on creation (first call), not on subsequent calls
	if keyInfo2.Mnemonic != "" {
		t.Error("expected mnemonic to be empty for existing key")
	}
}

func TestValidator_EnsureKey_DefaultBinPath(t *testing.T) {
	bin := makeFakePchaind(t)
	home := t.TempDir()

	// Set empty BinPath to test default
	s := NewWith(Options{
		BinPath: "", // Should default to "pchaind"
		HomeDir: home,
		Keyring: "test",
	})

	// Add our fake binary to PATH
	dir := filepath.Dir(bin)
	oldPath := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", oldPath) })
	os.Setenv("PATH", dir+":"+oldPath)

	// Rename binary to pchaind
	pchainPath := filepath.Join(dir, "pchaind")
	if err := os.Rename(bin, pchainPath); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	_, err := s.EnsureKey(ctx, "test-key")
	if err != nil {
		t.Fatalf("EnsureKey with default bin path error: %v", err)
	}
}

func TestValidator_ImportKey_ExistingKey(t *testing.T) {
	bin := makeFakePchaind(t)
	home := t.TempDir()
	s := NewWith(Options{
		BinPath: bin,
		HomeDir: home,
		Keyring: "test",
	})
	ctx := context.Background()

	// Create a key first
	_, err := s.EnsureKey(ctx, "existing-key")
	if err != nil {
		t.Fatalf("EnsureKey error: %v", err)
	}

	// Try to import with same name - should fail
	_, err = s.ImportKey(ctx, "existing-key", "word word word word word word word word word word word word")
	if err == nil {
		t.Fatal("ImportKey should fail for existing key name")
	}

	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

func TestValidator_ImportKey_DefaultBinPath(t *testing.T) {
	bin := makeFakePchaind(t)
	home := t.TempDir()

	s := NewWith(Options{
		BinPath: "", // Should default to "pchaind"
		HomeDir: home,
		Keyring: "test",
	})

	// Add our fake binary to PATH
	dir := filepath.Dir(bin)
	oldPath := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", oldPath) })
	os.Setenv("PATH", dir+":"+oldPath)

	pchainPath := filepath.Join(dir, "pchaind")
	if err := os.Rename(bin, pchainPath); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// Should not error about missing binary
	_, err := s.ImportKey(ctx, "", "test mnemonic")
	if err == nil {
		t.Fatal("expected error for empty key name")
	}

	if !strings.Contains(err.Error(), "key name required") {
		t.Errorf("expected 'key name required' error, got: %v", err)
	}
}

func TestExtractErrorLine(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "rpc error",
			input:    "some output\nrpc error: code = Unknown desc = error\nmore output",
			expected: "rpc error: code = Unknown desc = error",
		},
		{
			name:     "failed to execute message",
			input:    "output\nfailed to execute message: insufficient funds\n",
			expected: "failed to execute message: insufficient funds",
		},
		{
			name:     "insufficient",
			input:    "Error: insufficient fees\n",
			expected: "Error: insufficient fees",
		},
		{
			name:     "unauthorized",
			input:    "unauthorized: signature verification failed\n",
			expected: "unauthorized: signature verification failed",
		},
		{
			name:     "no error line",
			input:    "normal output\nmore output\n",
			expected: "",
		},
		{
			name:     "empty input",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractErrorLine(tt.input)
			if result != tt.expected {
				t.Errorf("extractErrorLine() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestValidator_ImportKey_InvalidMnemonic(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows not supported in this test")
	}

	dir := t.TempDir()
	binPath := filepath.Join(dir, "pchaind")

	// Script that simulates invalid mnemonic error
	script := `#!/usr/bin/env sh
cmd="$1"; shift
if [ "$cmd" = "keys" ]; then
	sub="$1"; shift
	if [ "$sub" = "show" ]; then
		exit 1
	fi
	if [ "$sub" = "add" ]; then
		echo "Error: invalid mnemonic"
		exit 1
	fi
fi
exit 1
`

	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	s := NewWith(Options{
		BinPath: binPath,
		HomeDir: t.TempDir(),
		Keyring: "test",
	})
	ctx := context.Background()

	_, err := s.ImportKey(ctx, "test-key", "invalid mnemonic phrase")
	if err == nil {
		t.Fatal("expected error for invalid mnemonic")
	}

	if !strings.Contains(err.Error(), "invalid mnemonic") {
		t.Errorf("expected 'invalid mnemonic' error, got: %v", err)
	}
}

func TestValidator_EnsureKey_CreationFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows not supported in this test")
	}

	dir := t.TempDir()
	binPath := filepath.Join(dir, "pchaind")

	// Script that fails on key creation
	script := `#!/usr/bin/env sh
cmd="$1"; shift
if [ "$cmd" = "keys" ]; then
	sub="$1"; shift
	if [ "$sub" = "show" ]; then
		exit 1
	fi
	if [ "$sub" = "add" ]; then
		echo "Error: key creation failed"
		exit 1
	fi
fi
exit 1
`

	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	s := NewWith(Options{
		BinPath: binPath,
		HomeDir: t.TempDir(),
		Keyring: "test",
	})
	ctx := context.Background()

	_, err := s.EnsureKey(ctx, "test-key")
	if err == nil {
		t.Fatal("expected error when key creation fails")
	}
}

func TestValidator_GetEVMAddress_NoHexAddress(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows not supported in this test")
	}

	dir := t.TempDir()
	binPath := filepath.Join(dir, "pchaind")

	// Script that returns output without hex address
	script := `#!/usr/bin/env sh
cmd="$1"; shift
if [ "$cmd" = "debug" ]; then
	echo "Some other output"
	echo "No hex address here"
	exit 0
fi
exit 1
`

	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	s := NewWith(Options{
		BinPath: binPath,
		HomeDir: t.TempDir(),
	})
	ctx := context.Background()

	_, err := s.GetEVMAddress(ctx, "push1test")
	if err == nil {
		t.Fatal("expected error when hex address not found")
	}

	if !strings.Contains(err.Error(), "could not extract EVM address") {
		t.Errorf("expected 'could not extract' error, got: %v", err)
	}
}

func TestValidator_EnsureKey_ShowAddressFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows not supported in this test")
	}

	dir := t.TempDir()
	binPath := filepath.Join(dir, "pchaind")

	// Script that succeeds in creation but fails to show address
	script := `#!/usr/bin/env sh
cmd="$1"; shift
if [ "$cmd" = "keys" ]; then
	sub="$1"; shift
	if [ "$sub" = "show" ]; then
		exit 1
	fi
	if [ "$sub" = "add" ]; then
		echo "**Important** write this mnemonic phrase"
		echo ""
		echo "word one two three four five six seven eight nine ten eleven twelve"
		exit 0
	fi
fi
exit 1
`

	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	s := NewWith(Options{
		BinPath: binPath,
		HomeDir: t.TempDir(),
		Keyring: "test",
	})
	ctx := context.Background()

	_, err := s.EnsureKey(ctx, "test-key")
	if err == nil {
		t.Fatal("expected error when show address fails after creation")
	}

	if !strings.Contains(err.Error(), "keys show") {
		t.Errorf("expected 'keys show' error, got: %v", err)
	}
}

func TestValidator_ImportKey_ShowAddressFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows not supported in this test")
	}

	dir := t.TempDir()
	binPath := filepath.Join(dir, "pchaind")

	callCount := 0
	counterFile := filepath.Join(t.TempDir(), "counter")
	os.WriteFile(counterFile, []byte("0"), 0644)

	// Script that succeeds in import but fails to show address
	script := `#!/usr/bin/env sh
cmd="$1"; shift
if [ "$cmd" = "keys" ]; then
	sub="$1"; shift
	if [ "$sub" = "show" ]; then
		# First call checks if key exists (should fail)
		# Second call tries to get address after import (should fail)
		COUNT=$(cat ` + counterFile + `)
		COUNT=$((COUNT + 1))
		echo $COUNT > ` + counterFile + `
		exit 1
	fi
	if [ "$sub" = "add" ]; then
		exit 0
	fi
fi
exit 1
`

	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	s := NewWith(Options{
		BinPath: binPath,
		HomeDir: t.TempDir(),
		Keyring: "test",
	})
	ctx := context.Background()

	_, err := s.ImportKey(ctx, "test-key", "word word word word word word word word word word word word")
	if err == nil {
		t.Fatal("expected error when show address fails after import")
	}

	if !strings.Contains(err.Error(), "failed to get imported key address") {
		t.Errorf("expected 'failed to get imported key address' error, got: %v", err)
	}

	_ = callCount
}

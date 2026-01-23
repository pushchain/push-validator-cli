package main

import (
	"os"
	"testing"
)

func TestAllSubcommandsRegistered(t *testing.T) {
	expectedCmds := []string{
		"status",
		"logs",
		"reset",
		"full-reset",
		"backup",
		"validators",
		"balance",
		"register-validator",
		"unjail",
		"withdraw-rewards",
		"increase-stake",
		"restake-rewards",
		"version",
		"completion",
		"stop",
		"restart",
		"doctor",
		"dashboard",
	}

	registered := map[string]bool{}
	for _, cmd := range rootCmd.Commands() {
		registered[cmd.Name()] = true
	}

	for _, name := range expectedCmds {
		if !registered[name] {
			t.Errorf("expected subcommand %q not registered on rootCmd", name)
		}
	}
}

func TestLoadCfg_Defaults(t *testing.T) {
	// Save and restore flags
	origHome := flagHome
	origRPC := flagRPC
	origGenesis := flagGenesis
	defer func() {
		flagHome = origHome
		flagRPC = origRPC
		flagGenesis = origGenesis
	}()

	flagHome = ""
	flagRPC = ""
	flagGenesis = ""

	cfg := loadCfg()

	if cfg.ChainID == "" {
		t.Error("loadCfg() returned empty ChainID")
	}
	if cfg.Denom == "" {
		t.Error("loadCfg() returned empty Denom")
	}
	if cfg.HomeDir == "" {
		t.Error("loadCfg() returned empty HomeDir")
	}
}

func TestLoadCfg_FlagOverrides(t *testing.T) {
	origHome := flagHome
	origRPC := flagRPC
	origGenesis := flagGenesis
	defer func() {
		flagHome = origHome
		flagRPC = origRPC
		flagGenesis = origGenesis
	}()

	flagHome = "/custom/home"
	flagRPC = "http://custom:26657"
	flagGenesis = "custom.domain.org"

	cfg := loadCfg()

	if cfg.HomeDir != "/custom/home" {
		t.Errorf("loadCfg() HomeDir = %q, want %q", cfg.HomeDir, "/custom/home")
	}
	if cfg.RPCLocal != "http://custom:26657" {
		t.Errorf("loadCfg() RPCLocal = %q, want %q", cfg.RPCLocal, "http://custom:26657")
	}
	if cfg.GenesisDomain != "custom.domain.org" {
		t.Errorf("loadCfg() GenesisDomain = %q, want %q", cfg.GenesisDomain, "custom.domain.org")
	}
}

func TestFindPchaind_FlagOverride(t *testing.T) {
	origBin := flagBin
	defer func() { flagBin = origBin }()

	flagBin = "/custom/path/pchaind"
	result := findPchaind()
	if result != "/custom/path/pchaind" {
		t.Errorf("findPchaind() = %q, want %q", result, "/custom/path/pchaind")
	}
}

func TestFindPchaind_EnvPCHAIND(t *testing.T) {
	origBin := flagBin
	defer func() { flagBin = origBin }()
	flagBin = ""

	os.Setenv("PCHAIND", "/env/pchaind")
	defer os.Unsetenv("PCHAIND")
	os.Unsetenv("PCHAIN_BIN")

	result := findPchaind()
	if result != "/env/pchaind" {
		t.Errorf("findPchaind() = %q, want %q", result, "/env/pchaind")
	}
}

func TestFindPchaind_EnvPCHAIN_BIN(t *testing.T) {
	origBin := flagBin
	defer func() { flagBin = origBin }()
	flagBin = ""

	os.Unsetenv("PCHAIND")
	os.Setenv("PCHAIN_BIN", "/env/pchain_bin")
	defer os.Unsetenv("PCHAIN_BIN")

	result := findPchaind()
	if result != "/env/pchain_bin" {
		t.Errorf("findPchaind() = %q, want %q", result, "/env/pchain_bin")
	}
}

func TestFindPchaind_Fallback(t *testing.T) {
	origBin := flagBin
	origHome := flagHome
	defer func() {
		flagBin = origBin
		flagHome = origHome
	}()
	flagBin = ""
	flagHome = "/nonexistent/path"

	os.Unsetenv("PCHAIND")
	os.Unsetenv("PCHAIN_BIN")
	os.Unsetenv("HOME_DIR")

	result := findPchaind()
	// Should fall through to "pchaind" default when cosmovisor path doesn't exist
	if result != "pchaind" {
		t.Errorf("findPchaind() = %q, want %q", result, "pchaind")
	}
}

func TestPersistentFlags(t *testing.T) {
	flags := []string{"home", "bin", "rpc", "genesis-domain", "output", "verbose", "quiet", "debug", "no-color", "no-emoji", "yes", "non-interactive"}

	for _, flag := range flags {
		if rootCmd.PersistentFlags().Lookup(flag) == nil {
			t.Errorf("persistent flag %q not registered on rootCmd", flag)
		}
	}
}

func TestRootCmdProperties(t *testing.T) {
	if rootCmd.Use != "push-validator" {
		t.Errorf("rootCmd.Use = %q, want %q", rootCmd.Use, "push-validator")
	}
	if rootCmd.Short == "" {
		t.Error("rootCmd.Short should not be empty")
	}
}

func TestRootCmd_HelpFunction(t *testing.T) {
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagNoColor = true
	flagNoEmoji = true

	// Call the help function directly to exercise custom help formatting
	rootCmd.SetArgs([]string{"--help"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("help returned error: %v", err)
	}
}

func TestRootCmd_SubcommandHelp(t *testing.T) {
	// Test that subcommand help uses cobra default (not custom)
	rootCmd.SetArgs([]string{"status", "--help"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("subcommand help returned error: %v", err)
	}
}

func TestRootCmd_PersistentPreRun(t *testing.T) {
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	origYes := flagYes
	origNonInteractive := flagNonInteractive
	origVerbose := flagVerbose
	origQuiet := flagQuiet
	origDebug := flagDebug
	defer func() {
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
		flagYes = origYes
		flagNonInteractive = origNonInteractive
		flagVerbose = origVerbose
		flagQuiet = origQuiet
		flagDebug = origDebug
	}()

	flagNoColor = true
	flagNoEmoji = true
	flagYes = true
	flagNonInteractive = true
	flagVerbose = false
	flagQuiet = true
	flagDebug = false

	// Call PersistentPreRun directly
	rootCmd.PersistentPreRun(rootCmd, nil)

	// Verify NO_COLOR was set
	if os.Getenv("NO_COLOR") != "1" {
		t.Error("expected NO_COLOR=1 after PersistentPreRun with flagNoColor=true")
	}
	os.Unsetenv("NO_COLOR") // cleanup
}

func TestRootCmd_StatusCommand_JSON(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()

	rootCmd.SetArgs([]string{"status", "--output", "json"})
	// This will fail to connect to a node, but exercises the command path
	_ = rootCmd.Execute()
}

func TestRootCmd_StatusCommand_YAML(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()

	rootCmd.SetArgs([]string{"status", "--output", "yaml"})
	_ = rootCmd.Execute()
}

func TestRootCmd_StatusCommand_Quiet(t *testing.T) {
	origOutput := flagOutput
	origQuiet := flagQuiet
	defer func() {
		flagOutput = origOutput
		flagQuiet = origQuiet
	}()
	flagQuiet = true

	rootCmd.SetArgs([]string{"status", "--quiet"})
	_ = rootCmd.Execute()
}

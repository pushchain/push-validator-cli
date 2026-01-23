package main

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestShouldSkipUpdateCheck(t *testing.T) {
	tests := []struct {
		name     string
		cmdName  string
		parent   string
		expected bool
	}{
		{"update command", "update", "", true},
		{"help command", "help", "", true},
		{"version command", "version", "", true},
		{"init command", "init", "", true},
		{"snapshot command", "snapshot", "", true},
		{"chain command", "chain", "", true},
		{"start command", "start", "", true},
		{"sync command", "sync", "", true},
		{"status command", "status", "", false},
		{"balance command", "balance", "", false},
		{"register-validator command", "register-validator", "", false},
		{"unjail command", "unjail", "", false},
		{"dashboard command", "dashboard", "", false},
		{"doctor command", "doctor", "", false},
		{"validators command", "validators", "", false},
		{"withdraw-rewards command", "withdraw-rewards", "", false},
		{"restake-rewards command", "restake-rewards", "", false},
		{"chain subcommand - install", "install", "chain", true},
		{"chain subcommand - download", "download", "chain", true},
		{"snapshot subcommand - download", "download", "snapshot", true},
		{"snapshot subcommand - list", "list", "snapshot", true},
		{"non-skip parent subcommand", "sub", "status", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: tt.cmdName}
			if tt.parent != "" {
				parent := &cobra.Command{Use: tt.parent}
				parent.AddCommand(cmd)
			}
			got := shouldSkipUpdateCheck(cmd)
			if got != tt.expected {
				t.Errorf("shouldSkipUpdateCheck(%q, parent=%q) = %v, want %v", tt.cmdName, tt.parent, got, tt.expected)
			}
		})
	}
}

func TestShowUpdateNotification_JSONSuppressed(t *testing.T) {
	// Save and restore flags
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()

	flagOutput = "json"
	// Should not panic - just silently return
	showUpdateNotification("v1.2.3")

	flagOutput = "yaml"
	showUpdateNotification("v1.2.3")
}

func TestShowUpdateNotification_QuietSuppressed(t *testing.T) {
	origOutput := flagOutput
	origQuiet := flagQuiet
	defer func() {
		flagOutput = origOutput
		flagQuiet = origQuiet
	}()

	flagOutput = "text"
	flagQuiet = true
	// Should not panic - just silently return
	showUpdateNotification("v1.2.3")
}

func TestShowUpdateNotification_TextOutput(t *testing.T) {
	origOutput := flagOutput
	origQuiet := flagQuiet
	origNoColor := flagNoColor
	defer func() {
		flagOutput = origOutput
		flagQuiet = origQuiet
		flagNoColor = origNoColor
	}()

	flagOutput = "text"
	flagQuiet = false
	flagNoColor = true
	// Should print notification without panic
	showUpdateNotification("v2.0.0")
}

func TestGetOSArch(t *testing.T) {
	result := getOSArch()
	if result == "" {
		t.Error("getOSArch() returned empty string")
	}
	// Should contain a slash separating OS and arch
	if !containsSubstr(result, "/") {
		t.Errorf("getOSArch() = %q, expected format 'os/arch'", result)
	}
}

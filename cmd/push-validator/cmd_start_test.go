package main

import (
	"testing"

	"github.com/pushchain/push-validator-cli/internal/snapshot"
)

func TestCreateSnapshotProgressCallback_JSONMode(t *testing.T) {
	cb := createSnapshotProgressCallback("json")
	// In JSON mode, callback should be a no-op (no panic)
	cb(snapshot.PhaseDownload, 100, 1000, "downloading")
	cb(snapshot.PhaseVerify, 0, 0, "verifying")
	cb(snapshot.PhaseExtract, 0, 0, "extracting file.tar")
}

func TestCreateSnapshotProgressCallback_Download_InitBar(t *testing.T) {
	cb := createSnapshotProgressCallback("text")
	// Test download phase with total > 0 creates and updates progress bar
	cb(snapshot.PhaseDownload, 0, 1000, "")
	cb(snapshot.PhaseDownload, 500, 1000, "")
	cb(snapshot.PhaseDownload, 1000, 1000, "")
}

func TestCreateSnapshotProgressCallback_Download_NoTotal(t *testing.T) {
	cb := createSnapshotProgressCallback("text")
	// Test download phase with total <= 0 doesn't create bar (no panic)
	cb(snapshot.PhaseDownload, 100, 0, "")
	cb(snapshot.PhaseDownload, 200, -1, "")
}

func TestCreateSnapshotProgressCallback_Verify_WithMessage(t *testing.T) {
	cb := createSnapshotProgressCallback("text")
	// Test verify phase prints message
	cb(snapshot.PhaseVerify, 0, 0, "Checksum verified")
}

func TestCreateSnapshotProgressCallback_Verify_NoMessage(t *testing.T) {
	cb := createSnapshotProgressCallback("text")
	// Test verify phase with empty message (no panic)
	cb(snapshot.PhaseVerify, 0, 0, "")
}

func TestCreateSnapshotProgressCallback_Extract_ShortMessage(t *testing.T) {
	cb := createSnapshotProgressCallback("text")
	// Test extract with short message (< 60 chars)
	cb(snapshot.PhaseExtract, 0, 0, "data/application.db")
}

func TestCreateSnapshotProgressCallback_Extract_LongMessage(t *testing.T) {
	cb := createSnapshotProgressCallback("text")
	// Test extract with message > 60 chars (should be truncated to 57 + "...")
	longMessage := "data/very/long/path/to/some/deeply/nested/directory/structure/file.db"
	cb(snapshot.PhaseExtract, 0, 0, longMessage)
}

func TestCreateSnapshotProgressCallback_Extract_NoMessage(t *testing.T) {
	cb := createSnapshotProgressCallback("text")
	// Test extract with empty message (no panic)
	cb(snapshot.PhaseExtract, 0, 0, "")
}

func TestCreateSnapshotProgressCallback_Download_ThenVerify(t *testing.T) {
	cb := createSnapshotProgressCallback("text")
	// Test download phase followed by verify (bar should be finished)
	cb(snapshot.PhaseDownload, 0, 1000, "")
	cb(snapshot.PhaseDownload, 500, 1000, "")
	cb(snapshot.PhaseDownload, 1000, 1000, "")
	cb(snapshot.PhaseVerify, 0, 0, "Verification complete")
}

func TestCreateSnapshotProgressCallback_FullSequence(t *testing.T) {
	cb := createSnapshotProgressCallback("text")
	// Test full sequence: download -> verify -> extract
	cb(snapshot.PhaseDownload, 0, 1000, "")
	cb(snapshot.PhaseDownload, 500, 1000, "")
	cb(snapshot.PhaseDownload, 1000, 1000, "")
	cb(snapshot.PhaseVerify, 0, 0, "Checksum verified")
	cb(snapshot.PhaseExtract, 0, 0, "data/application.db")
	cb(snapshot.PhaseExtract, 0, 0, "data/blockstore.db")
}

func TestIsTerminalInteractive(t *testing.T) {
	// In test environment, stdin/stdout are typically not terminals
	result := isTerminalInteractive()
	if result {
		t.Log("isTerminalInteractive() returned true - running in a real terminal")
	}
	// Just verify it doesn't panic and returns a boolean
}

func TestIsTerminalInteractiveWith_NonTerminal(t *testing.T) {
	// fd 0 in tests is typically not a terminal
	result := isTerminalInteractiveWith(0, 0)
	if result {
		t.Error("expected false for non-terminal fds in test environment")
	}
}

func TestIsTerminalInteractiveWith_InvalidFd(t *testing.T) {
	// Large invalid fd should return false
	result := isTerminalInteractiveWith(99999, 99999)
	if result {
		t.Error("expected false for invalid fds")
	}
}

func TestCheckValidatorRegistration_Immediate(t *testing.T) {
	v := &mockValidator{isValidatorRes: true}
	result := checkValidatorRegistration(v, 0)
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !result.IsValidator {
		t.Error("expected IsValidator=true")
	}
}

func TestCheckValidatorRegistration_NotValidator(t *testing.T) {
	v := &mockValidator{isValidatorRes: false}
	result := checkValidatorRegistration(v, 0)
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if result.IsValidator {
		t.Error("expected IsValidator=false")
	}
}

func TestCheckValidatorRegistration_Error(t *testing.T) {
	v := &mockValidator{isValidatorErr: errMock}
	result := checkValidatorRegistration(v, 0)
	if result.Error == nil {
		t.Fatal("expected error")
	}
}

func TestComputePostStartDecision_ErrorShowsDashboard(t *testing.T) {
	result := computePostStartDecision(valCheckResult{Error: errMock}, true)
	if result != actionShowDashboard {
		t.Errorf("expected actionShowDashboard, got %s", result)
	}
}

func TestComputePostStartDecision_ValidatorShowsDashboard(t *testing.T) {
	result := computePostStartDecision(valCheckResult{IsValidator: true}, true)
	if result != actionShowDashboard {
		t.Errorf("expected actionShowDashboard, got %s", result)
	}
}

func TestComputePostStartDecision_NotValidator_Interactive(t *testing.T) {
	result := computePostStartDecision(valCheckResult{IsValidator: false}, true)
	if result != actionPromptRegister {
		t.Errorf("expected actionPromptRegister, got %s", result)
	}
}

func TestComputePostStartDecision_NotValidator_NonInteractive(t *testing.T) {
	result := computePostStartDecision(valCheckResult{IsValidator: false}, false)
	if result != actionShowSteps {
		t.Errorf("expected actionShowSteps, got %s", result)
	}
}

func TestShowDashboardPromptWith_NonInteractive(t *testing.T) {
	p := testPrinter()
	prompter := &mockPrompter{interactive: false}
	cfg := testCfg()
	// Should not panic and should print non-interactive message
	showDashboardPromptWith(cfg, &p, prompter)
}

func TestShowDashboardPromptWith_Interactive_Error(t *testing.T) {
	p := testPrinter()
	// Prompter with no responses configured will return error
	prompter := &mockPrompter{interactive: true, responses: []string{}}
	cfg := testCfg()
	// Should handle ReadLine error gracefully
	showDashboardPromptWith(cfg, &p, prompter)
}

func TestShowDashboardPromptWith_Interactive_Success(t *testing.T) {
	p := testPrinter()
	// Prompter responds with Enter (empty string)
	prompter := &mockPrompter{interactive: true, responses: []string{""}}
	cfg := testCfg()
	// This will call handleDashboard which will fail gracefully (no node running)
	// but the point is testing the prompt success path
	showDashboardPromptWith(cfg, &p, prompter)
}

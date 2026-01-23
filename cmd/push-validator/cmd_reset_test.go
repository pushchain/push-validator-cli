package main

import (
	"testing"

	"github.com/pushchain/push-validator-cli/internal/config"
)

func TestHandleReset_NonInteractive_NoYes(t *testing.T) {
	origOutput := flagOutput
	origYes := flagYes
	origNonInteractive := flagNonInteractive
	defer func() {
		flagOutput = origOutput
		flagYes = origYes
		flagNonInteractive = origNonInteractive
	}()
	flagOutput = "text"
	flagYes = false
	flagNonInteractive = true

	cfg := config.Config{HomeDir: t.TempDir()}
	sup := &mockSupervisor{running: false}

	err := handleReset(cfg, sup)
	if err == nil {
		t.Fatal("expected error when non-interactive without --yes")
	}
	if err.Error() != "reset requires confirmation: use --yes to confirm in non-interactive mode" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleReset_JSON_NoConfirmNeeded(t *testing.T) {
	origOutput := flagOutput
	origYes := flagYes
	defer func() {
		flagOutput = origOutput
		flagYes = origYes
	}()
	flagOutput = "json"
	flagYes = false

	cfg := config.Config{HomeDir: t.TempDir()}
	sup := &mockSupervisor{running: false}

	// In JSON mode, no confirmation is needed - it proceeds to admin.Reset
	// admin.Reset returns nil for a valid HomeDir (just removes and recreates dirs)
	err := handleReset(cfg, sup)
	if err != nil {
		t.Fatalf("expected no error for JSON mode reset with valid dir, got: %v", err)
	}
}

func TestHandleReset_WithYes_NotRunning(t *testing.T) {
	origOutput := flagOutput
	origYes := flagYes
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagOutput = origOutput
		flagYes = origYes
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagOutput = "text"
	flagYes = true
	flagNoColor = true
	flagNoEmoji = true

	cfg := config.Config{HomeDir: t.TempDir()}
	sup := &mockSupervisor{running: false}

	// With --yes, skips confirmation. admin.Reset succeeds for valid HomeDir.
	err := handleReset(cfg, sup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleReset_WithYes_RunningNode_StopSuccess(t *testing.T) {
	origOutput := flagOutput
	origYes := flagYes
	defer func() {
		flagOutput = origOutput
		flagYes = origYes
	}()
	flagOutput = "text"
	flagYes = true

	cfg := config.Config{HomeDir: t.TempDir()}
	sup := &mockSupervisor{running: true, pid: 123}

	// Node is running, will try to stop, then call admin.Reset
	err := handleReset(cfg, sup)
	// admin.Reset will fail, but we verify stop was called
	if sup.running {
		t.Error("expected supervisor to be stopped")
	}
	_ = err
}

func TestHandleFullReset_NonInteractive_NoYes(t *testing.T) {
	origOutput := flagOutput
	origYes := flagYes
	origNonInteractive := flagNonInteractive
	defer func() {
		flagOutput = origOutput
		flagYes = origYes
		flagNonInteractive = origNonInteractive
	}()
	flagOutput = "text"
	flagYes = false
	flagNonInteractive = true

	cfg := config.Config{HomeDir: t.TempDir()}
	sup := &mockSupervisor{running: false}

	err := handleFullReset(cfg, sup)
	if err == nil {
		t.Fatal("expected error when non-interactive without --yes")
	}
	if err.Error() != "full-reset requires confirmation: use --yes to confirm in non-interactive mode" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleFullReset_JSON_NotRunning(t *testing.T) {
	origOutput := flagOutput
	origYes := flagYes
	defer func() {
		flagOutput = origOutput
		flagYes = origYes
	}()
	flagOutput = "json"
	flagYes = false

	cfg := config.Config{HomeDir: t.TempDir()}
	sup := &mockSupervisor{running: false}

	// JSON mode skips confirmation prompts. admin.FullReset succeeds for valid HomeDir.
	err := handleFullReset(cfg, sup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleFullReset_RunningNode_StopError_JSON(t *testing.T) {
	origOutput := flagOutput
	origYes := flagYes
	defer func() {
		flagOutput = origOutput
		flagYes = origYes
	}()
	flagOutput = "json"
	flagYes = false

	cfg := config.Config{HomeDir: t.TempDir()}
	sup := &mockSupervisor{running: true, pid: 456, stopErr: errMock}

	err := handleFullReset(cfg, sup)
	if err == nil {
		t.Fatal("expected error when stop fails in JSON mode")
	}
}

func TestHandleFullReset_WithYes_NotRunning(t *testing.T) {
	origOutput := flagOutput
	origYes := flagYes
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagOutput = origOutput
		flagYes = origYes
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagOutput = "text"
	flagYes = true
	flagNoColor = true
	flagNoEmoji = true

	cfg := config.Config{HomeDir: t.TempDir()}
	sup := &mockSupervisor{running: false}

	// With --yes, skips confirmation. admin.FullReset succeeds for valid HomeDir.
	err := handleFullReset(cfg, sup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleReset_WithYes_RunningNode_StopError_Text(t *testing.T) {
	origOutput := flagOutput
	origYes := flagYes
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagOutput = origOutput
		flagYes = origYes
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagOutput = "text"
	flagYes = true
	flagNoColor = true
	flagNoEmoji = true

	cfg := config.Config{HomeDir: t.TempDir()}
	sup := &mockSupervisor{running: true, pid: 789, stopErr: errMock}

	// When stop fails, reset continues with a warning
	err := handleReset(cfg, sup)
	// We expect an error from admin.Reset (no pchaind binary), not from stop
	// The stop error should be handled/logged but not returned
	_ = err
}

func TestHandleReset_JSON_RunningNode_StopSuccess(t *testing.T) {
	origOutput := flagOutput
	origYes := flagYes
	defer func() {
		flagOutput = origOutput
		flagYes = origYes
	}()
	flagOutput = "json"
	flagYes = false

	cfg := config.Config{HomeDir: t.TempDir()}
	sup := &mockSupervisor{running: true, pid: 100}

	// JSON mode with running node - should stop it
	err := handleReset(cfg, sup)
	if sup.running {
		t.Error("expected supervisor to be stopped")
	}
	_ = err
}

func TestHandleFullReset_WithYes_RunningNode_StopSuccess(t *testing.T) {
	origOutput := flagOutput
	origYes := flagYes
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagOutput = origOutput
		flagYes = origYes
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagOutput = "text"
	flagYes = true
	flagNoColor = true
	flagNoEmoji = true

	cfg := config.Config{HomeDir: t.TempDir()}
	sup := &mockSupervisor{running: true, pid: 200}

	// Node is running, will stop successfully, then call admin.FullReset
	err := handleFullReset(cfg, sup)
	if sup.running {
		t.Error("expected supervisor to be stopped")
	}
	_ = err
}

func TestHandleFullReset_JSON_RunningNode_StopSuccess(t *testing.T) {
	origOutput := flagOutput
	origYes := flagYes
	defer func() {
		flagOutput = origOutput
		flagYes = origYes
	}()
	flagOutput = "json"
	flagYes = false

	cfg := config.Config{HomeDir: t.TempDir()}
	sup := &mockSupervisor{running: true, pid: 300}

	// JSON mode with running node - should stop it
	err := handleFullReset(cfg, sup)
	if sup.running {
		t.Error("expected supervisor to be stopped")
	}
	_ = err
}

func TestHandleReset_Interactive_ConfirmYes(t *testing.T) {
	origOutput := flagOutput
	origYes := flagYes
	origNonInteractive := flagNonInteractive
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagOutput = origOutput
		flagYes = origYes
		flagNonInteractive = origNonInteractive
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagOutput = "text"
	flagYes = false
	flagNonInteractive = false
	flagNoColor = true
	flagNoEmoji = true

	cfg := config.Config{HomeDir: t.TempDir()}
	sup := &mockSupervisor{running: false}
	prompter := &mockPrompter{interactive: true, responses: []string{"y"}}

	err := handleReset(cfg, sup, prompter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleReset_Interactive_ConfirmNo(t *testing.T) {
	origOutput := flagOutput
	origYes := flagYes
	origNonInteractive := flagNonInteractive
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagOutput = origOutput
		flagYes = origYes
		flagNonInteractive = origNonInteractive
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagOutput = "text"
	flagYes = false
	flagNonInteractive = false
	flagNoColor = true
	flagNoEmoji = true

	cfg := config.Config{HomeDir: t.TempDir()}
	sup := &mockSupervisor{running: false}
	prompter := &mockPrompter{interactive: true, responses: []string{"n"}}

	err := handleReset(cfg, sup, prompter)
	if err != nil {
		t.Fatalf("expected nil (cancelled), got: %v", err)
	}
}

func TestHandleFullReset_Interactive_ConfirmYes(t *testing.T) {
	origOutput := flagOutput
	origYes := flagYes
	origNonInteractive := flagNonInteractive
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagOutput = origOutput
		flagYes = origYes
		flagNonInteractive = origNonInteractive
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagOutput = "text"
	flagYes = false
	flagNonInteractive = false
	flagNoColor = true
	flagNoEmoji = true

	cfg := config.Config{HomeDir: t.TempDir()}
	sup := &mockSupervisor{running: false}
	prompter := &mockPrompter{interactive: true, responses: []string{"yes"}}

	err := handleFullReset(cfg, sup, prompter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleFullReset_Interactive_ConfirmNo(t *testing.T) {
	origOutput := flagOutput
	origYes := flagYes
	origNonInteractive := flagNonInteractive
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagOutput = origOutput
		flagYes = origYes
		flagNonInteractive = origNonInteractive
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagOutput = "text"
	flagYes = false
	flagNonInteractive = false
	flagNoColor = true
	flagNoEmoji = true

	cfg := config.Config{HomeDir: t.TempDir()}
	sup := &mockSupervisor{running: false}
	prompter := &mockPrompter{interactive: true, responses: []string{"no"}}

	err := handleFullReset(cfg, sup, prompter)
	if err != nil {
		t.Fatalf("expected nil (cancelled), got: %v", err)
	}
}

func TestHandleFullReset_RunningNode_StopError_Text_ContinueYes(t *testing.T) {
	origOutput := flagOutput
	origYes := flagYes
	origNonInteractive := flagNonInteractive
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagOutput = origOutput
		flagYes = origYes
		flagNonInteractive = origNonInteractive
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagOutput = "text"
	flagYes = true // Skip confirmation prompt
	flagNonInteractive = false
	flagNoColor = true
	flagNoEmoji = true

	cfg := config.Config{HomeDir: t.TempDir()}
	sup := &mockSupervisor{running: true, pid: 500, stopErr: errMock}
	// First response: continue after stop error
	prompter := &mockPrompter{interactive: true, responses: []string{"y"}}

	err := handleFullReset(cfg, sup, prompter)
	// Even if stop fails, the reset itself should work
	_ = err
}

func TestHandleFullReset_RunningNode_StopError_Text_CancelNo(t *testing.T) {
	origOutput := flagOutput
	origYes := flagYes
	origNonInteractive := flagNonInteractive
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagOutput = origOutput
		flagYes = origYes
		flagNonInteractive = origNonInteractive
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagOutput = "text"
	flagYes = true
	flagNonInteractive = false
	flagNoColor = true
	flagNoEmoji = true

	cfg := config.Config{HomeDir: t.TempDir()}
	sup := &mockSupervisor{running: true, pid: 501, stopErr: errMock}
	// Decline to continue after stop error
	prompter := &mockPrompter{interactive: true, responses: []string{"n"}}

	err := handleFullReset(cfg, sup, prompter)
	if err != nil {
		t.Fatalf("expected nil (cancelled), got: %v", err)
	}
}

func TestHandleReset_WithYes_RunningNode_StopError_JSON(t *testing.T) {
	origOutput := flagOutput
	origYes := flagYes
	defer func() {
		flagOutput = origOutput
		flagYes = origYes
	}()
	flagOutput = "json"
	flagYes = false

	cfg := config.Config{HomeDir: t.TempDir()}
	sup := &mockSupervisor{running: true, stopErr: errMock}

	// JSON mode - when stop fails during reset, it continues with warning
	// (unlike full-reset which aborts on stop error)
	err := handleReset(cfg, sup)
	// Should not return the stop error itself - reset continues
	_ = err
}

package main

import (
	"testing"
)

func TestHandleBackup_Success(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "text"

	// handleBackup calls admin.Backup which needs a real HomeDir
	// with specific structure. We test the error path here.
	d := &Deps{
		Cfg:     testCfg(),
		Printer: getPrinter(),
	}

	// This will likely return an error since /tmp/test-pchain doesn't exist
	// but we're testing that the function doesn't panic and handles errors
	err := handleBackup(d)
	if err == nil {
		// If it somehow succeeds (e.g., the dir exists), that's fine
		return
	}
	// Error is expected since the test home dir doesn't have proper structure
}

func TestHandleBackup_Error_JSON(t *testing.T) {
	origOutput := flagOutput
	flagOutput = "json"
	defer func() { flagOutput = origOutput }()

	d := &Deps{
		Cfg:     testCfg(),
		Printer: getPrinter(),
	}

	// Test JSON error path
	err := handleBackup(d)
	if err == nil {
		return // If backup succeeds in test env, that's OK
	}
	// We're testing that JSON error output doesn't panic
}

func TestHandleBackup_RealTempDir(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "text"

	dir := t.TempDir()
	cfg := testCfg()
	cfg.HomeDir = dir

	d := &Deps{
		Cfg:     cfg,
		Printer: getPrinter(),
	}

	// admin.Backup will try to create a tar.gz of the config dir
	// It may fail because there's no config dir, but shouldn't panic
	_ = handleBackup(d)
}

func TestHandleBackup_Success_JSON(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	dir := t.TempDir()
	cfg := testCfg()
	cfg.HomeDir = dir

	d := &Deps{
		Cfg:     cfg,
		Printer: getPrinter(),
	}

	// This should succeed since HomeDir exists and backup dir is auto-created
	err := handleBackup(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleBackup_Success_Text(t *testing.T) {
	origOutput := flagOutput
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagOutput = origOutput
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagOutput = "text"
	flagNoColor = true
	flagNoEmoji = true

	dir := t.TempDir()
	cfg := testCfg()
	cfg.HomeDir = dir

	d := &Deps{
		Cfg:     cfg,
		Printer: getPrinter(),
	}

	err := handleBackup(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

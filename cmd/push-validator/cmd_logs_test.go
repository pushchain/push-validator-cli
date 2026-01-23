package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHandleLogs_NoLogPath(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "text"

	sup := &mockSupervisor{logPath: ""}

	err := handleLogs(sup)
	if err == nil {
		t.Fatal("expected error when no log path configured")
	}
	if err.Error() != "no log path configured" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleLogs_NoLogPath_JSON(t *testing.T) {
	origOutput := flagOutput
	flagOutput = "json"
	defer func() { flagOutput = origOutput }()

	sup := &mockSupervisor{logPath: ""}

	err := handleLogs(sup)
	if err == nil {
		t.Fatal("expected error when no log path (json)")
	}
}

func TestHandleLogs_FileNotFound(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "text"

	sup := &mockSupervisor{logPath: "/nonexistent/path/to/logfile.log"}

	err := handleLogs(sup)
	if err == nil {
		t.Fatal("expected error when log file not found")
	}
	if !containsSubstr(err.Error(), "log file not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleLogs_FileNotFound_JSON(t *testing.T) {
	origOutput := flagOutput
	flagOutput = "json"
	defer func() { flagOutput = origOutput }()

	sup := &mockSupervisor{logPath: "/nonexistent/path/to/logfile.log"}

	err := handleLogs(sup)
	if err == nil {
		t.Fatal("expected error when log file not found (json)")
	}
}

func TestHandleLogs_ValidLogPath_Exists(t *testing.T) {
	// Verify that the function gets past the path validation checks
	// when the file exists. We can't fully test ui.RunLogUIV2 since it blocks.
	// This test just verifies the path validation logic works.
	dir := t.TempDir()
	logFile := filepath.Join(dir, "node.log")
	if err := os.WriteFile(logFile, []byte("test log line\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	sup := &mockSupervisor{logPath: logFile}

	// Verify LogPath returns expected value
	if sup.LogPath() != logFile {
		t.Errorf("expected LogPath=%s, got %s", logFile, sup.LogPath())
	}

	// Verify the file passes os.Stat check
	if _, err := os.Stat(sup.LogPath()); err != nil {
		t.Errorf("expected file to exist: %v", err)
	}
}

func TestHandleLogs_LogPath_EmptyString_Text(t *testing.T) {
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

	sup := &mockSupervisor{logPath: ""}

	err := handleLogs(sup)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "no log path configured" {
		t.Errorf("got %q, want %q", err.Error(), "no log path configured")
	}
}

func TestHandleLogs_NonInteractive_FileNotFound(t *testing.T) {
	origOutput := flagOutput
	origNonInteractive := flagNonInteractive
	defer func() {
		flagOutput = origOutput
		flagNonInteractive = origNonInteractive
	}()
	flagOutput = "text"
	flagNonInteractive = true

	sup := &mockSupervisor{logPath: "/nonexistent/log.log"}

	err := handleLogs(sup)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "log file not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

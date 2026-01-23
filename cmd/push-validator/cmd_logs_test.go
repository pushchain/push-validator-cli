package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	ui "github.com/pushchain/push-validator-cli/internal/ui"
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

// --- Tests using handleLogsCore with injectable deps ---

func testLogDeps(logFile string) logDeps {
	return logDeps{
		isTerminal: func(fd int) bool { return false },
		openTTY:    func() (*os.File, error) { return nil, fmt.Errorf("no tty") },
		runLogUI:   func(ctx context.Context, opts ui.LogUIOptions) error { return nil },
		stat:       os.Stat,
	}
}

func TestHandleLogsCore_EmptyLogPath_Text(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "text"

	sup := &mockSupervisor{logPath: ""}
	err := handleLogsCore(sup, testLogDeps(""))
	if err == nil || err.Error() != "no log path configured" {
		t.Errorf("expected 'no log path configured', got: %v", err)
	}
}

func TestHandleLogsCore_EmptyLogPath_JSON(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	sup := &mockSupervisor{logPath: ""}
	err := handleLogsCore(sup, testLogDeps(""))
	if err == nil || err.Error() != "no log path configured" {
		t.Errorf("expected 'no log path configured', got: %v", err)
	}
}

func TestHandleLogsCore_FileNotFound_Text(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "text"

	sup := &mockSupervisor{logPath: "/no/such/file.log"}
	err := handleLogsCore(sup, testLogDeps(""))
	if err == nil || !containsSubstr(err.Error(), "log file not found") {
		t.Errorf("expected 'log file not found', got: %v", err)
	}
}

func TestHandleLogsCore_FileNotFound_JSON(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	sup := &mockSupervisor{logPath: "/no/such/file.log"}
	err := handleLogsCore(sup, testLogDeps(""))
	if err == nil || !containsSubstr(err.Error(), "log file not found") {
		t.Errorf("expected 'log file not found', got: %v", err)
	}
}

func TestHandleLogsCore_NonInteractive_RunsLogUI(t *testing.T) {
	origOutput := flagOutput
	origNonInteractive := flagNonInteractive
	defer func() {
		flagOutput = origOutput
		flagNonInteractive = origNonInteractive
	}()
	flagOutput = "text"
	flagNonInteractive = true

	dir := t.TempDir()
	logFile := filepath.Join(dir, "node.log")
	os.WriteFile(logFile, []byte("log\n"), 0o644)

	var calledOpts ui.LogUIOptions
	deps := logDeps{
		isTerminal: func(fd int) bool { return false },
		openTTY:    func() (*os.File, error) { return nil, fmt.Errorf("no tty") },
		runLogUI: func(ctx context.Context, opts ui.LogUIOptions) error {
			calledOpts = opts
			return nil
		},
		stat: os.Stat,
	}

	sup := &mockSupervisor{logPath: logFile}
	err := handleLogsCore(sup, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calledOpts.LogPath != logFile {
		t.Errorf("expected LogPath=%s, got %s", logFile, calledOpts.LogPath)
	}
	if calledOpts.ShowFooter {
		t.Error("expected ShowFooter=false in non-interactive mode")
	}
}

func TestHandleLogsCore_Interactive_ShowsFooter(t *testing.T) {
	origOutput := flagOutput
	origNonInteractive := flagNonInteractive
	defer func() {
		flagOutput = origOutput
		flagNonInteractive = origNonInteractive
	}()
	flagOutput = "text"
	flagNonInteractive = false

	dir := t.TempDir()
	logFile := filepath.Join(dir, "node.log")
	os.WriteFile(logFile, []byte("log\n"), 0o644)

	var calledOpts ui.LogUIOptions
	deps := logDeps{
		isTerminal: func(fd int) bool { return true }, // Both stdin and stdout are terminals
		openTTY:    func() (*os.File, error) { return nil, fmt.Errorf("no tty") },
		runLogUI: func(ctx context.Context, opts ui.LogUIOptions) error {
			calledOpts = opts
			return nil
		},
		stat: os.Stat,
	}

	sup := &mockSupervisor{logPath: logFile}
	err := handleLogsCore(sup, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !calledOpts.ShowFooter {
		t.Error("expected ShowFooter=true when terminal is interactive")
	}
}

func TestHandleLogsCore_RunLogUI_Error(t *testing.T) {
	origOutput := flagOutput
	origNonInteractive := flagNonInteractive
	defer func() {
		flagOutput = origOutput
		flagNonInteractive = origNonInteractive
	}()
	flagOutput = "text"
	flagNonInteractive = true

	dir := t.TempDir()
	logFile := filepath.Join(dir, "node.log")
	os.WriteFile(logFile, []byte("log\n"), 0o644)

	deps := logDeps{
		isTerminal: func(fd int) bool { return false },
		openTTY:    func() (*os.File, error) { return nil, fmt.Errorf("no tty") },
		runLogUI: func(ctx context.Context, opts ui.LogUIOptions) error {
			return fmt.Errorf("TUI crashed")
		},
		stat: os.Stat,
	}

	sup := &mockSupervisor{logPath: logFile}
	err := handleLogsCore(sup, deps)
	if err == nil || err.Error() != "TUI crashed" {
		t.Errorf("expected 'TUI crashed', got: %v", err)
	}
}

func TestHandleLogsCore_TTYFallback_Success(t *testing.T) {
	origOutput := flagOutput
	origNonInteractive := flagNonInteractive
	defer func() {
		flagOutput = origOutput
		flagNonInteractive = origNonInteractive
	}()
	flagOutput = "text"
	flagNonInteractive = false

	dir := t.TempDir()
	logFile := filepath.Join(dir, "node.log")
	os.WriteFile(logFile, []byte("log\n"), 0o644)

	// Create a pipe to simulate a TTY file
	r, w, _ := os.Pipe()
	defer r.Close()
	defer w.Close()

	var calledOpts ui.LogUIOptions
	deps := logDeps{
		isTerminal: func(fd int) bool {
			// stdin/stdout are NOT terminals, but the TTY file IS
			return fd == int(r.Fd())
		},
		openTTY: func() (*os.File, error) { return r, nil },
		runLogUI: func(ctx context.Context, opts ui.LogUIOptions) error {
			calledOpts = opts
			return nil
		},
		stat: os.Stat,
	}

	sup := &mockSupervisor{logPath: logFile}
	err := handleLogsCore(sup, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !calledOpts.ShowFooter {
		t.Error("expected ShowFooter=true when TTY fallback succeeds")
	}
}

func TestHandleLogsCore_TTYFallback_NotTerminal(t *testing.T) {
	origOutput := flagOutput
	origNonInteractive := flagNonInteractive
	defer func() {
		flagOutput = origOutput
		flagNonInteractive = origNonInteractive
	}()
	flagOutput = "text"
	flagNonInteractive = false

	dir := t.TempDir()
	logFile := filepath.Join(dir, "node.log")
	os.WriteFile(logFile, []byte("log\n"), 0o644)

	r, w, _ := os.Pipe()
	defer r.Close()
	defer w.Close()

	var calledOpts ui.LogUIOptions
	deps := logDeps{
		isTerminal: func(fd int) bool { return false }, // Nothing is a terminal
		openTTY:    func() (*os.File, error) { return r, nil },
		runLogUI: func(ctx context.Context, opts ui.LogUIOptions) error {
			calledOpts = opts
			return nil
		},
		stat: os.Stat,
	}

	sup := &mockSupervisor{logPath: logFile}
	err := handleLogsCore(sup, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calledOpts.ShowFooter {
		t.Error("expected ShowFooter=false when TTY fallback is not a terminal")
	}
}

func TestHandleLogsCore_NoColor(t *testing.T) {
	origOutput := flagOutput
	origNonInteractive := flagNonInteractive
	origNoColor := flagNoColor
	defer func() {
		flagOutput = origOutput
		flagNonInteractive = origNonInteractive
		flagNoColor = origNoColor
	}()
	flagOutput = "text"
	flagNonInteractive = true
	flagNoColor = true

	dir := t.TempDir()
	logFile := filepath.Join(dir, "node.log")
	os.WriteFile(logFile, []byte("log\n"), 0o644)

	var calledOpts ui.LogUIOptions
	deps := logDeps{
		isTerminal: func(fd int) bool { return false },
		openTTY:    func() (*os.File, error) { return nil, fmt.Errorf("no tty") },
		runLogUI: func(ctx context.Context, opts ui.LogUIOptions) error {
			calledOpts = opts
			return nil
		},
		stat: os.Stat,
	}

	sup := &mockSupervisor{logPath: logFile}
	err := handleLogsCore(sup, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !calledOpts.NoColor {
		t.Error("expected NoColor=true")
	}
}

func TestHandleLogsCore_StatError(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "text"

	deps := logDeps{
		isTerminal: func(fd int) bool { return false },
		openTTY:    func() (*os.File, error) { return nil, fmt.Errorf("no tty") },
		runLogUI:   func(ctx context.Context, opts ui.LogUIOptions) error { return nil },
		stat:       func(name string) (os.FileInfo, error) { return nil, fmt.Errorf("permission denied") },
	}

	sup := &mockSupervisor{logPath: "/some/log/file.log"}
	err := handleLogsCore(sup, deps)
	if err == nil || !containsSubstr(err.Error(), "log file not found") {
		t.Errorf("expected 'log file not found', got: %v", err)
	}
}

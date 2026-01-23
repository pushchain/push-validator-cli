package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/term"

	"github.com/pushchain/push-validator-cli/internal/process"
	ui "github.com/pushchain/push-validator-cli/internal/ui"
)

// logDeps holds injectable dependencies for handleLogsCore.
type logDeps struct {
	isTerminal func(fd int) bool
	openTTY    func() (*os.File, error)
	runLogUI   func(ctx context.Context, opts ui.LogUIOptions) error
	stat       func(name string) (os.FileInfo, error)
}

// handleLogs tails the node log file until interrupted. It validates
// the log path and prints structured JSON errors when --output=json.
func handleLogs(sup process.Supervisor) error {
	return handleLogsCore(sup, logDeps{
		isTerminal: func(fd int) bool { return term.IsTerminal(fd) },
		openTTY:    func() (*os.File, error) { return os.OpenFile("/dev/tty", os.O_RDWR, 0) },
		runLogUI:   ui.RunLogUIV2,
		stat:       os.Stat,
	})
}

// handleLogsCore contains the testable core logic for handleLogs.
func handleLogsCore(sup process.Supervisor, deps logDeps) error {
	lp := sup.LogPath()
	if lp == "" {
		if flagOutput == "json" {
			getPrinter().JSON(map[string]any{"ok": false, "error": "no log path configured"})
		} else {
			getPrinter().Error("no log path configured")
		}
		return fmt.Errorf("no log path configured")
	}
	if _, err := deps.stat(lp); err != nil {
		if flagOutput == "json" {
			getPrinter().JSON(map[string]any{"ok": false, "error": "log file not found", "path": lp})
		} else {
			getPrinter().Error(fmt.Sprintf("log file not found: %s", lp))
		}
		return fmt.Errorf("log file not found: %s", lp)
	}
	interactive := deps.isTerminal(int(os.Stdin.Fd())) && deps.isTerminal(int(os.Stdout.Fd())) && !flagNonInteractive
	var tty *os.File
	if !interactive && !flagNonInteractive {
		if t, err := deps.openTTY(); err == nil {
			if deps.isTerminal(int(t.Fd())) {
				interactive = true
				tty = t
			} else {
				_ = t.Close()
			}
		}
	}
	// Setup context for signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle Ctrl+C
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		cancel()
	}()

	var (
		origIn  = os.Stdin
		origOut = os.Stdout
	)
	if tty != nil {
		os.Stdin = tty
		os.Stdout = tty
		defer func() {
			_ = tty.Close()
			os.Stdin = origIn
			os.Stdout = origOut
		}()
	}

	return deps.runLogUI(ctx, ui.LogUIOptions{
		LogPath:    lp,
		BgKey:      'b',
		ShowFooter: interactive,
		NoColor:    flagNoColor,
	})
}

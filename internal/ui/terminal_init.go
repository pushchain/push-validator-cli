package ui

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"golang.org/x/term"
)

var terminalInitialized bool

// InitTerminal configures the terminal to prevent escape sequence pollution.
// This MUST be called before any charmbracelet library (lipgloss, bubbletea) usage
// to prevent OSC 11 background color queries from polluting the output.
//
// The issue: muesli/termenv (used by lipgloss) queries terminal background color
// via OSC 11, and the terminal response (\033]11;rgb:...\033\\) gets mixed into stdout.
// Setting COLORFGBG tells termenv the background color, skipping the query.
func InitTerminal() {
	if terminalInitialized {
		return
	}
	terminalInitialized = true

	// 1. Prevent OSC 11 background color query by pre-setting COLORFGBG
	// Format: "foreground;background" where values indicate color indices
	// Setting "0;15" indicates dark foreground on light background area
	// This prevents termenv from sending OSC 11 query to detect background
	if os.Getenv("COLORFGBG") == "" {
		os.Setenv("COLORFGBG", "0;15")
	}

	// 2. For TTY output, disable focus reporting and flush stale responses
	// iTerm2 and other terminals can send focus in/out events (^[[I/^[[O])
	// which pollute the output stream
	if term.IsTerminal(int(os.Stdout.Fd())) {
		// Disable focus reporting (CSI ? 1004 l)
		fmt.Fprint(os.Stdout, "\033[?1004l")
		// Small delay to allow any pending responses to arrive
		time.Sleep(20 * time.Millisecond)
		// Flush any pending terminal responses from stdin
		// Increased timeout to 150ms to catch slower terminals
		FlushStdinWithTimeout(150 * time.Millisecond)
	}
}

// ResetTerminalAfterTUI cleans up terminal state after a TUI (like bubbletea) exits.
// This prevents escape sequences from asynchronous terminal responses (cursor position
// reports, OSC responses) from appearing in the output after the TUI closes.
//
// Should be called after any bubbletea program exits, especially when using alternate screen.
func ResetTerminalAfterTUI() {
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return
	}

	// 1. Disable terminal query modes that may have been enabled
	// These prevent the terminal from sending unsolicited responses
	fmt.Fprint(os.Stdout, "\033[?1004l")  // Disable focus reporting
	fmt.Fprint(os.Stdout, "\033[?1003l")  // Disable all mouse tracking
	fmt.Fprint(os.Stdout, "\033[?1000l")  // Disable X10 mouse tracking
	fmt.Fprint(os.Stdout, "\033[?1006l")  // Disable SGR mouse mode
	fmt.Fprint(os.Stdout, "\033[?25h")    // Show cursor (ensure it's visible)

	// 2. Send a carriage return to ensure we're at the start of a line
	// This prevents partial escape sequences from appearing mid-line
	fmt.Fprint(os.Stdout, "\r")

	// 3. Allow time for terminal to process the reset commands
	time.Sleep(30 * time.Millisecond)

	// 4. Flush stdin to catch any async responses that arrived during/after TUI exit
	// Use a longer timeout (150ms) to catch delayed responses from slow terminals
	// Common delayed responses:
	//   - Cursor position reports (CPR): ^[[row;colR
	//   - OSC responses: ^[]11;rgb:xxxx/xxxx/xxxx^[\
	//   - Focus events: ^[[I or ^[[O
	FlushStdinWithTimeout(150 * time.Millisecond)
}

// FlushStdinWithTimeout reads and discards stdin for the specified duration.
// This catches asynchronous terminal responses (cursor position reports,
// OSC responses, focus events) that arrive after queries are sent.
// Only flushes if stdin is a terminal — never reads from pipes or /dev/null
// to avoid consuming piped script content (e.g., curl | bash).
func FlushStdinWithTimeout(timeout time.Duration) {
	fd := int(os.Stdin.Fd())

	// Never read from stdin if it's not a terminal — this would consume
	// piped input (e.g., the install script when run via "curl | bash")
	if !term.IsTerminal(fd) {
		return
	}

	// Set non-blocking mode to read without waiting
	if err := syscall.SetNonblock(fd, true); err != nil {
		return
	}
	defer syscall.SetNonblock(fd, false)

	buf := make([]byte, 256)
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		n, _ := os.Stdin.Read(buf)
		if n <= 0 {
			// No data available, wait briefly before checking again
			time.Sleep(5 * time.Millisecond)
		}
		// If we read data, continue the loop to catch more
	}
}

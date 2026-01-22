package main

import "github.com/pushchain/push-validator-cli/internal/ui"

func main() {
	// Initialize terminal FIRST, before any charmbracelet imports are used.
	// This prevents OSC 11 background color queries and focus events from
	// polluting the output stream.
	ui.InitTerminal()

	Execute()
}

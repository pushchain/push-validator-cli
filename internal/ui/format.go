package ui

import (
	"fmt"
	"os"
	"strings"
)

// FormatNumber formats an integer with thousands separators
// Example: 1234567 -> "1,234,567"
func FormatNumber(n int64) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}

	// Insert commas from right to left
	var result strings.Builder
	for i, c := range reverse(s) {
		if i > 0 && i%3 == 0 {
			result.WriteRune(',')
		}
		result.WriteRune(c)
	}
	return reverse(result.String())
}

func reverse(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// FormatBytes formats a byte count as a human-readable string.
// Example: 1234567890 -> "1.1GB"
func FormatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1fGB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1fMB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1fKB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}

// FormatSpeed formats a bytes-per-second rate as a human-readable string.
// Example: 2097152.0 -> "2.0MB/s"
func FormatSpeed(bytesPerSec float64) string {
	const (
		KB = 1024.0
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case bytesPerSec >= GB:
		return fmt.Sprintf("%.1fGB/s", bytesPerSec/GB)
	case bytesPerSec >= MB:
		return fmt.Sprintf("%.1fMB/s", bytesPerSec/MB)
	case bytesPerSec >= KB:
		return fmt.Sprintf("%.1fKB/s", bytesPerSec/KB)
	default:
		return fmt.Sprintf("%.0fB/s", bytesPerSec)
	}
}

// ShortenPath replaces the home directory with ~ for cleaner display.
// Example: /Users/john/.pchain/data -> ~/.pchain/data
func ShortenPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}

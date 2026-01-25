package node

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	cronMarker      = "# push-validator-peer-refresh"
	cronInterval    = "* * * * *" // Every 1 minute
	launchdPlistName = "com.push.validator.peerrefresh.plist"
)

// InstallPeerRefreshCron installs a cron job (or launchd on macOS) that
// runs every minute to refresh peers and restart the node if needed.
// This is idempotent - safe to call multiple times.
func InstallPeerRefreshCron(homeDir string) error {
	// Find the push-validator binary path
	binPath, err := os.Executable()
	if err != nil {
		binPath = "push-validator" // fallback
	}

	if runtime.GOOS == "darwin" {
		return installLaunchd(binPath, homeDir)
	}
	return installCron(binPath, homeDir)
}

// UninstallPeerRefreshCron removes the peer refresh cron job.
func UninstallPeerRefreshCron(homeDir string) error {
	if runtime.GOOS == "darwin" {
		return uninstallLaunchd()
	}
	return uninstallCron()
}

// IsPeerRefreshCronInstalled checks if the cron job is installed.
func IsPeerRefreshCronInstalled(homeDir string) bool {
	if runtime.GOOS == "darwin" {
		return isLaunchdInstalled()
	}
	return isCronInstalled()
}

// --- Cron (Linux) ---

func installCron(binPath, homeDir string) error {
	// Check if already installed
	if isCronInstalled() {
		return nil
	}

	// Get current crontab
	cmd := exec.Command("crontab", "-l")
	existing, _ := cmd.Output() // ignore error (no crontab yet)

	// Build new cron entry
	cronLine := fmt.Sprintf("%s %s _internal-refresh-peers --home %s %s",
		cronInterval, binPath, homeDir, cronMarker)

	// Append to existing crontab
	newCrontab := string(existing)
	if !strings.HasSuffix(newCrontab, "\n") && len(newCrontab) > 0 {
		newCrontab += "\n"
	}
	newCrontab += cronLine + "\n"

	// Install new crontab
	cmd = exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(newCrontab)
	return cmd.Run()
}

func uninstallCron() error {
	// Get current crontab
	cmd := exec.Command("crontab", "-l")
	existing, err := cmd.Output()
	if err != nil {
		return nil // no crontab
	}

	// Filter out our marker line
	lines := strings.Split(string(existing), "\n")
	var filtered []string
	for _, line := range lines {
		if !strings.Contains(line, cronMarker) {
			filtered = append(filtered, line)
		}
	}

	newCrontab := strings.Join(filtered, "\n")

	// Install filtered crontab
	cmd = exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(newCrontab)
	return cmd.Run()
}

func isCronInstalled() bool {
	cmd := exec.Command("crontab", "-l")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), cronMarker)
}

// --- Launchd (macOS) ---

func launchdPlistPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", launchdPlistName)
}

func installLaunchd(binPath, homeDir string) error {
	// Check if already installed
	if isLaunchdInstalled() {
		return nil
	}

	plistPath := launchdPlistPath()

	// Ensure LaunchAgents directory exists
	if err := os.MkdirAll(filepath.Dir(plistPath), 0o755); err != nil {
		return err
	}

	// Create plist content - runs every 60 seconds
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.push.validator.peerrefresh</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>_internal-refresh-peers</string>
        <string>--home</string>
        <string>%s</string>
    </array>
    <key>StartInterval</key>
    <integer>60</integer>
    <key>RunAtLoad</key>
    <true/>
    <key>StandardOutPath</key>
    <string>%s/logs/peer-refresh.log</string>
    <key>StandardErrorPath</key>
    <string>%s/logs/peer-refresh.log</string>
</dict>
</plist>
`, binPath, homeDir, homeDir, homeDir)

	// Write plist file
	if err := os.WriteFile(plistPath, []byte(plist), 0o644); err != nil {
		return err
	}

	// Load the agent
	cmd := exec.Command("launchctl", "load", plistPath)
	return cmd.Run()
}

func uninstallLaunchd() error {
	plistPath := launchdPlistPath()

	// Unload the agent (ignore error if not loaded)
	cmd := exec.Command("launchctl", "unload", plistPath)
	_ = cmd.Run()

	// Remove the plist file
	return os.Remove(plistPath)
}

func isLaunchdInstalled() bool {
	_, err := os.Stat(launchdPlistPath())
	return err == nil
}

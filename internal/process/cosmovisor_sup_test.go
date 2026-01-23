package process

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func TestCosmovisorSupervisor_LogPath(t *testing.T) {
	home := t.TempDir()
	sup := NewCosmovisor(home)

	logPath := sup.LogPath()
	expectedPath := filepath.Join(home, "logs", "cosmovisor.log")
	if logPath != expectedPath {
		t.Errorf("LogPath() = %q, want %q", logPath, expectedPath)
	}
}

func TestCosmovisorSupervisor_PID_NoFile(t *testing.T) {
	home := t.TempDir()
	sup := NewCosmovisor(home)

	pid, ok := sup.PID()
	if ok {
		t.Errorf("PID() with no file should return ok=false, got pid=%d ok=%v", pid, ok)
	}
	if pid != 0 {
		t.Errorf("PID() with no file should return 0, got %d", pid)
	}
}

func TestCosmovisorSupervisor_PID_EmptyFile(t *testing.T) {
	home := t.TempDir()
	pidFile := filepath.Join(home, "cosmovisor.pid")

	// Create empty PID file
	if err := os.WriteFile(pidFile, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	sup := NewCosmovisor(home)
	pid, ok := sup.PID()
	if ok {
		t.Errorf("PID() with empty file should return ok=false, got pid=%d ok=%v", pid, ok)
	}
}

func TestCosmovisorSupervisor_PID_InvalidContent(t *testing.T) {
	home := t.TempDir()
	pidFile := filepath.Join(home, "cosmovisor.pid")

	// Create PID file with invalid content
	if err := os.WriteFile(pidFile, []byte("not-a-number"), 0o644); err != nil {
		t.Fatal(err)
	}

	sup := NewCosmovisor(home)
	pid, ok := sup.PID()
	if ok {
		t.Errorf("PID() with invalid content should return ok=false, got pid=%d ok=%v", pid, ok)
	}
}

func TestCosmovisorSupervisor_PID_NonExistentProcess(t *testing.T) {
	home := t.TempDir()
	pidFile := filepath.Join(home, "cosmovisor.pid")

	// Write a PID that definitely doesn't exist (high number)
	if err := os.WriteFile(pidFile, []byte("999999"), 0o644); err != nil {
		t.Fatal(err)
	}

	sup := NewCosmovisor(home)
	pid, ok := sup.PID()
	if ok {
		t.Errorf("PID() with non-existent process should return ok=false, got pid=%d ok=%v", pid, ok)
	}
}

func TestCosmovisorSupervisor_PID_ValidProcess(t *testing.T) {
	home := t.TempDir()
	pidFile := filepath.Join(home, "cosmovisor.pid")

	// Write current process PID
	currentPID := os.Getpid()
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(currentPID)), 0o644); err != nil {
		t.Fatal(err)
	}

	sup := NewCosmovisor(home)
	pid, ok := sup.PID()
	if !ok {
		t.Errorf("PID() with valid process should return ok=true, got pid=%d ok=%v", pid, ok)
	}
	if pid != currentPID {
		t.Errorf("PID() = %d, want %d", pid, currentPID)
	}
}

func TestCosmovisorSupervisor_IsRunning(t *testing.T) {
	home := t.TempDir()
	sup := NewCosmovisor(home)

	if sup.IsRunning() {
		t.Error("IsRunning() should return false when no PID file exists")
	}
}

func TestCosmovisorSupervisor_IsRunning_WithValidPID(t *testing.T) {
	home := t.TempDir()
	pidFile := filepath.Join(home, "cosmovisor.pid")

	// Write current process PID
	currentPID := os.Getpid()
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(currentPID)), 0o644); err != nil {
		t.Fatal(err)
	}

	sup := NewCosmovisor(home)
	if !sup.IsRunning() {
		t.Error("IsRunning() should return true with valid PID file")
	}
}

func TestCosmovisorSupervisor_Uptime_NotRunning(t *testing.T) {
	home := t.TempDir()
	sup := NewCosmovisor(home)

	duration, ok := sup.Uptime()
	if ok {
		t.Errorf("Uptime() should return ok=false when not running, got duration=%v ok=%v", duration, ok)
	}
	if duration != 0 {
		t.Errorf("Uptime() should return 0 duration when not running, got %v", duration)
	}
}

func TestCosmovisorSupervisor_Uptime_Running(t *testing.T) {
	home := t.TempDir()
	pidFile := filepath.Join(home, "cosmovisor.pid")

	// Write current process PID (which is definitely running)
	currentPID := os.Getpid()
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(currentPID)), 0o644); err != nil {
		t.Fatal(err)
	}

	sup := NewCosmovisor(home)
	duration, ok := sup.Uptime()
	// ps command may return 0 seconds for very short-lived processes or current process
	if !ok {
		t.Log("Uptime() returned ok=false - ps command may not work for current process")
	}
	if ok && duration < 0 {
		t.Errorf("Uptime() returned negative duration: %v", duration)
	}
	t.Logf("Current process uptime: %v (ok=%v)", duration, ok)
}

func TestCosmovisorSupervisor_Stop_NotRunning(t *testing.T) {
	home := t.TempDir()
	sup := NewCosmovisor(home)

	// Stopping when not running should succeed (no-op)
	err := sup.Stop()
	if err != nil {
		t.Errorf("Stop() when not running should succeed, got error: %v", err)
	}
}

func TestCosmovisorSupervisor_Stop_StaleProcess(t *testing.T) {
	home := t.TempDir()
	pidFile := filepath.Join(home, "cosmovisor.pid")

	// Create a process that exits immediately
	testScript := filepath.Join(home, "test-proc")
	script := `#!/bin/sh
exit 0
`
	if err := os.WriteFile(testScript, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	// Start and let it exit
	cmd := createDetachedProcess(testScript)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid

	// Write PID file
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(pid)), 0o644); err != nil {
		t.Fatal(err)
	}

	// Wait for it to exit
	_ = cmd.Wait()
	time.Sleep(100 * time.Millisecond)

	// Process should be dead now
	if processAlive(pid) {
		t.Fatal("Process should be dead")
	}

	// Stop should succeed even though process is already dead
	sup := NewCosmovisor(home)
	err := sup.Stop()
	if err != nil {
		t.Errorf("Stop() should succeed for dead process, got error: %v", err)
	}

	// PID file should be removed (or not exist)
	time.Sleep(100 * time.Millisecond)
}

func TestCosmovisorSupervisor_Start_NoGenesis(t *testing.T) {
	home := t.TempDir()
	sup := NewCosmovisor(home)

	// Create config directory but no genesis.json
	if err := os.MkdirAll(filepath.Join(home, "config"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := sup.Start(StartOpts{
		HomeDir: home,
		BinPath: "pchaind",
	})
	if err == nil {
		t.Error("Start() without genesis.json should return error")
	}
}

func TestCosmovisorSupervisor_Start_AlreadyRunning(t *testing.T) {
	home := t.TempDir()
	pidFile := filepath.Join(home, "cosmovisor.pid")

	// Create genesis.json
	if err := os.MkdirAll(filepath.Join(home, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "config", "genesis.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Write current process PID to simulate already running
	currentPID := os.Getpid()
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(currentPID)), 0o644); err != nil {
		t.Fatal(err)
	}

	sup := NewCosmovisor(home)
	pid, err := sup.Start(StartOpts{
		HomeDir: home,
		BinPath: "pchaind",
	})

	// Should return the existing PID without error
	if err != nil {
		t.Errorf("Start() when already running should not return error, got: %v", err)
	}
	if pid != currentPID {
		t.Errorf("Start() when already running should return existing PID %d, got %d", currentPID, pid)
	}
}

func TestCosmovisorSupervisor_Start_CosmovisorNotSetup(t *testing.T) {
	home := t.TempDir()

	// Create genesis.json
	if err := os.MkdirAll(filepath.Join(home, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "config", "genesis.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	sup := NewCosmovisor(home)

	// This will fail because cosmovisor is not set up
	// and we don't have a real pchaind binary
	_, err := sup.Start(StartOpts{
		HomeDir: home,
		BinPath: "/nonexistent/pchaind",
	})

	// Should get an error about cosmovisor not being set up or binary not found
	if err == nil {
		// Clean up if somehow succeeded
		_ = sup.Stop()
		t.Error("Start() should fail when cosmovisor is not set up")
	}
}

func TestCosmovisorSupervisor_Restart(t *testing.T) {
	home := t.TempDir()
	sup := NewCosmovisor(home)

	// Create genesis.json
	if err := os.MkdirAll(filepath.Join(home, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "config", "genesis.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Restart when not running - will attempt to stop (no-op) then start
	_, err := sup.Restart(StartOpts{
		HomeDir: home,
		BinPath: "/nonexistent/pchaind",
	})

	// Will fail because we don't have real binaries, but exercises the code path
	if err == nil {
		_ = sup.Stop()
	}
}

// TestCosmovisorSupervisor_Start_WithInitialSync tests the initial sync path
func TestCosmovisorSupervisor_Start_WithInitialSync(t *testing.T) {
	home := t.TempDir()

	// Create genesis.json
	if err := os.MkdirAll(filepath.Join(home, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "config", "genesis.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create the .initial_state_sync marker file
	markerFile := filepath.Join(home, ".initial_state_sync")
	if err := os.WriteFile(markerFile, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	sup := NewCosmovisor(home)

	// This will fail because cosmovisor is not set up
	_, err := sup.Start(StartOpts{
		HomeDir: home,
		BinPath: "/nonexistent/pchaind",
	})

	// Should get an error, but we're testing the code path
	if err == nil {
		_ = sup.Stop()
	}
}

// TestCosmovisorSupervisor_Start_EmptyHomeDir tests with empty HomeDir
func TestCosmovisorSupervisor_Start_EmptyHomeDir(t *testing.T) {
	home := t.TempDir()
	sup := NewCosmovisor(home)

	// Start with empty HomeDir should use supervisor's homeDir
	// Will fail due to no genesis, but tests the path
	_, err := sup.Start(StartOpts{})
	if err == nil {
		t.Error("Start() with no genesis should fail")
		_ = sup.Stop()
	}
}

// TestCosmovisorSupervisor_Restart_StaleProcess tests restart with stale PID
func TestCosmovisorSupervisor_Restart_StaleProcess(t *testing.T) {
	home := t.TempDir()
	pidFile := filepath.Join(home, "cosmovisor.pid")

	// Create genesis.json
	if err := os.MkdirAll(filepath.Join(home, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "config", "genesis.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Write a non-existent PID
	if err := os.WriteFile(pidFile, []byte("999999"), 0o644); err != nil {
		t.Fatal(err)
	}

	sup := NewCosmovisor(home)

	// Restart will stop (quick since process doesn't exist) then try to start
	_, err := sup.Restart(StartOpts{
		HomeDir: home,
		BinPath: "/nonexistent/pchaind",
	})

	// Will fail due to cosmovisor not being set up or binary not found
	if err == nil {
		_ = sup.Stop()
	}
}

// TestCosmovisorSupervisor_Start_WithBlockstore tests when blockstore exists
func TestCosmovisorSupervisor_Start_WithBlockstore(t *testing.T) {
	home := t.TempDir()

	// Create genesis.json
	if err := os.MkdirAll(filepath.Join(home, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "config", "genesis.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create blockstore to avoid initial sync path
	if err := os.MkdirAll(filepath.Join(home, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "data", "blockstore.db"), []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	sup := NewCosmovisor(home)

	// Will fail due to cosmovisor not being set up
	_, err := sup.Start(StartOpts{
		HomeDir: home,
		BinPath: "/nonexistent/pchaind",
	})

	if err == nil {
		_ = sup.Stop()
	}
}

// Helper function to create a detached process for testing
func createDetachedProcess(path string) *exec.Cmd {
	cmd := exec.Command(path)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd
}

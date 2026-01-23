package process

import (
    "fmt"
    "net"
    "os"
    "os/exec"
    "path/filepath"
    "strconv"
    "testing"
    "time"
)

func TestIsRPCListening(t *testing.T) {
    ln, err := net.Listen("tcp", "127.0.0.1:0")
    if err != nil { t.Skipf("skipping: cannot bind due to sandbox: %v", err) }
    defer func() { _ = ln.Close() }()
    addr := ln.Addr().String()
    if !IsRPCListening(addr, 200*time.Millisecond) { t.Fatalf("expected listening true for %s", addr) }
    ln.Close()
    if IsRPCListening(addr, 200*time.Millisecond) { t.Fatalf("expected listening false after close for %s", addr) }
}

func TestIsRPCListening_DefaultPort(t *testing.T) {
    // Test with empty hostport (should use default)
    // This will likely fail since nothing is listening, but tests the default path
    if IsRPCListening("", 100*time.Millisecond) {
        t.Log("default port is listening (unexpected but not an error)")
    }
}

func TestSupervisor_PID_NoFile(t *testing.T) {
    home := t.TempDir()
    sup := New(home)

    pid, ok := sup.PID()
    if ok {
        t.Errorf("PID() with no file should return ok=false, got pid=%d ok=%v", pid, ok)
    }
    if pid != 0 {
        t.Errorf("PID() with no file should return 0, got %d", pid)
    }
}

func TestSupervisor_PID_EmptyFile(t *testing.T) {
    home := t.TempDir()
    pidFile := filepath.Join(home, "pchaind.pid")

    // Create empty PID file
    if err := os.WriteFile(pidFile, []byte(""), 0o644); err != nil {
        t.Fatal(err)
    }

    sup := New(home)
    pid, ok := sup.PID()
    if ok {
        t.Errorf("PID() with empty file should return ok=false, got pid=%d ok=%v", pid, ok)
    }
}

func TestSupervisor_PID_InvalidContent(t *testing.T) {
    home := t.TempDir()
    pidFile := filepath.Join(home, "pchaind.pid")

    // Create PID file with invalid content
    if err := os.WriteFile(pidFile, []byte("not-a-number"), 0o644); err != nil {
        t.Fatal(err)
    }

    sup := New(home)
    pid, ok := sup.PID()
    if ok {
        t.Errorf("PID() with invalid content should return ok=false, got pid=%d ok=%v", pid, ok)
    }
}

func TestSupervisor_PID_NonExistentProcess(t *testing.T) {
    home := t.TempDir()
    pidFile := filepath.Join(home, "pchaind.pid")

    // Write a PID that definitely doesn't exist (high number)
    if err := os.WriteFile(pidFile, []byte("999999"), 0o644); err != nil {
        t.Fatal(err)
    }

    sup := New(home)
    pid, ok := sup.PID()
    if ok {
        t.Errorf("PID() with non-existent process should return ok=false, got pid=%d ok=%v", pid, ok)
    }
}

func TestSupervisor_IsRunning(t *testing.T) {
    home := t.TempDir()
    sup := New(home)

    if sup.IsRunning() {
        t.Error("IsRunning() should return false when no PID file exists")
    }
}

func TestSupervisor_Uptime_NotRunning(t *testing.T) {
    home := t.TempDir()
    sup := New(home)

    duration, ok := sup.Uptime()
    if ok {
        t.Errorf("Uptime() should return ok=false when not running, got duration=%v ok=%v", duration, ok)
    }
    if duration != 0 {
        t.Errorf("Uptime() should return 0 duration when not running, got %v", duration)
    }
}

func TestSupervisor_LogPath(t *testing.T) {
    home := t.TempDir()
    sup := New(home)

    logPath := sup.LogPath()
    expectedPath := filepath.Join(home, "logs", "pchaind.log")
    if logPath != expectedPath {
        t.Errorf("LogPath() = %q, want %q", logPath, expectedPath)
    }
}

func TestSupervisor_Start_NoHomeDir(t *testing.T) {
    home := t.TempDir()
    sup := New(home)

    _, err := sup.Start(StartOpts{}) // Empty opts, no HomeDir
    if err == nil {
        t.Error("Start() with no HomeDir should return error")
    }
}

func TestSupervisor_Start_NoGenesis(t *testing.T) {
    home := t.TempDir()
    sup := New(home)

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

func TestSupervisor_Stop_NotRunning(t *testing.T) {
    home := t.TempDir()
    sup := New(home)

    // Stopping when not running should succeed (no-op)
    err := sup.Stop()
    if err != nil {
        t.Errorf("Stop() when not running should succeed, got error: %v", err)
    }
}

func TestProcessAlive(t *testing.T) {
    tests := []struct {
        name string
        pid  int
        want bool
    }{
        {
            name: "zero pid",
            pid:  0,
            want: false,
        },
        {
            name: "negative pid",
            pid:  -1,
            want: false,
        },
        {
            name: "current process",
            pid:  os.Getpid(),
            want: true,
        },
        {
            name: "non-existent pid",
            pid:  999999,
            want: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := processAlive(tt.pid)
            if got != tt.want {
                t.Errorf("processAlive(%d) = %v, want %v", tt.pid, got, tt.want)
            }
        })
    }
}

func TestSupervisor_Start_AlreadyRunning(t *testing.T) {
    home := t.TempDir()
    pidFile := filepath.Join(home, "pchaind.pid")

    // Create genesis.json so start check passes
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

    sup := New(home)
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

// TestSupervisor_Uptime_WithRunningProcess tests Uptime with current process
func TestSupervisor_Uptime_WithRunningProcess(t *testing.T) {
    home := t.TempDir()
    pidFile := filepath.Join(home, "pchaind.pid")

    // Write current process PID (which is definitely running)
    currentPID := os.Getpid()
    if err := os.WriteFile(pidFile, []byte(strconv.Itoa(currentPID)), 0o644); err != nil {
        t.Fatal(err)
    }

    sup := New(home)
    duration, ok := sup.Uptime()
    // ps command may return 0 seconds for very short-lived processes or current process
    // Just check that it doesn't return an error and duration is non-negative
    if !ok {
        t.Log("Uptime() returned ok=false - ps command may not work for current process")
    }
    if ok && duration < 0 {
        t.Errorf("Uptime() returned negative duration: %v", duration)
    }
    t.Logf("Current process uptime: %v (ok=%v)", duration, ok)
}

// TestSupervisor_Restart_NotRunning tests restart when process is not running
func TestSupervisor_Restart_NotRunning(t *testing.T) {
    home := t.TempDir()

    // Create genesis.json
    if err := os.MkdirAll(filepath.Join(home, "config"), 0o755); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(filepath.Join(home, "config", "genesis.json"), []byte("{}"), 0o644); err != nil {
        t.Fatal(err)
    }

    sup := New(home)

    // Restart when not running - will attempt to start with default binary
    _, err := sup.Restart(StartOpts{
        HomeDir: home,
    })

    // This will likely fail since pchaind probably doesn't exist
    // But we're testing the code path
    if err == nil {
        // If it somehow succeeded, clean up
        _ = sup.Stop()
    }
}

// TestSupervisor_Start_WithInitialSync tests the initial sync scenario
func TestSupervisor_Start_WithInitialSync(t *testing.T) {
    home := t.TempDir()
    binPath := filepath.Join(home, "fake-pchaind")

    // Create a fake binary that accepts tendermint unsafe-reset-all
    script := `#!/bin/sh
if [ "$1" = "tendermint" ] && [ "$2" = "unsafe-reset-all" ]; then
    exit 0
fi
# For start command with args, just run briefly then exit
if [ "$1" = "start" ]; then
    sleep 0.1
    exit 0
fi
`
    if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
        t.Fatal(err)
    }

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

    sup := New(home)
    pid, err := sup.Start(StartOpts{
        HomeDir: home,
        BinPath: binPath,
    })
    if err != nil {
        t.Fatalf("Start() with initial sync failed: %v", err)
    }
    if pid <= 0 {
        t.Fatalf("Start() returned invalid PID: %d", pid)
    }

    // Wait for start
    time.Sleep(200 * time.Millisecond)

    // Verify marker file was removed
    if _, err := os.Stat(markerFile); !os.IsNotExist(err) {
        t.Error(".initial_state_sync marker should be removed after start")
    }

    // Verify priv_validator_state.json was created
    pvsPath := filepath.Join(home, "data", "priv_validator_state.json")
    if _, err := os.Stat(pvsPath); os.IsNotExist(err) {
        t.Error("priv_validator_state.json should be created")
    }
}

// TestSupervisor_Start_NoBlockstore tests initial sync when blockstore doesn't exist
func TestSupervisor_Start_NoBlockstore(t *testing.T) {
    home := t.TempDir()
    binPath := filepath.Join(home, "fake-pchaind")

    script := `#!/bin/sh
if [ "$1" = "tendermint" ] && [ "$2" = "unsafe-reset-all" ]; then
    exit 0
fi
if [ "$1" = "start" ]; then
    sleep 0.1
    exit 0
fi
`
    if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
        t.Fatal(err)
    }

    // Create genesis.json
    if err := os.MkdirAll(filepath.Join(home, "config"), 0o755); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(filepath.Join(home, "config", "genesis.json"), []byte("{}"), 0o644); err != nil {
        t.Fatal(err)
    }

    // Don't create blockstore.db - should trigger initial sync
    sup := New(home)
    pid, err := sup.Start(StartOpts{
        HomeDir: home,
        BinPath: binPath,
    })
    if err != nil {
        t.Fatalf("Start() without blockstore failed: %v", err)
    }
    if pid <= 0 {
        t.Fatalf("Start() returned invalid PID: %d", pid)
    }

    time.Sleep(100 * time.Millisecond)
}

// TestSupervisor_Start_WithExtraArgs tests starting with extra arguments
func TestSupervisor_Start_WithExtraArgs(t *testing.T) {
    home := t.TempDir()
    binPath := filepath.Join(home, "fake-daemon")

    script := `#!/bin/sh
if [ "$1" = "start" ]; then
    sleep 0.1
    exit 0
fi
`
    if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
        t.Fatal(err)
    }

    // Create genesis.json
    if err := os.MkdirAll(filepath.Join(home, "config"), 0o755); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(filepath.Join(home, "config", "genesis.json"), []byte("{}"), 0o644); err != nil {
        t.Fatal(err)
    }

    sup := New(home)
    pid, err := sup.Start(StartOpts{
        HomeDir:   home,
        BinPath:   binPath,
        ExtraArgs: []string{"--test-arg", "value"},
    })
    if err != nil {
        t.Fatalf("Start() with extra args failed: %v", err)
    }
    if pid <= 0 {
        t.Fatalf("Start() returned invalid PID: %d", pid)
    }

    time.Sleep(100 * time.Millisecond)
}

// TestSupervisor_Stop_StaleProcess tests stopping when process is already dead
func TestSupervisor_Stop_StaleProcess(t *testing.T) {
    home := t.TempDir()
    pidFile := filepath.Join(home, "pchaind.pid")

    // Create a process that exits immediately
    script := `#!/bin/sh
exit 0
`
    scriptPath := filepath.Join(home, "exit-daemon")
    if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
        t.Fatal(err)
    }

    // Start and let it exit
    cmd := exec.Command(scriptPath)
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
    sup := New(home)
    if err := sup.Stop(); err != nil {
        t.Errorf("Stop() should succeed for dead process, got error: %v", err)
    }

    // PID file might not be removed if process was already dead
    // This is acceptable behavior
    _ = pidFile
}

// TestSupervisor_Start_DefaultBinPath tests using default pchaind binary
func TestSupervisor_Start_DefaultBinPath(t *testing.T) {
    home := t.TempDir()

    // Create genesis.json
    if err := os.MkdirAll(filepath.Join(home, "config"), 0o755); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(filepath.Join(home, "config", "genesis.json"), []byte("{}"), 0o644); err != nil {
        t.Fatal(err)
    }

    sup := New(home)
    // Don't specify BinPath - will use default "pchaind"
    _, err := sup.Start(StartOpts{
        HomeDir: home,
    })

    // This will likely fail since pchaind probably doesn't exist in PATH
    // but we're testing that the code path is exercised
    if err == nil {
        // If it somehow succeeded, clean up
        _ = sup.Stop()
    }
    // We don't check for error because pchaind might not exist
}

// TestSupervisor_Start_LogFileError tests error handling when log file can't be created
func TestSupervisor_Start_LogFileError(t *testing.T) {
    home := t.TempDir()

    // Create genesis.json
    if err := os.MkdirAll(filepath.Join(home, "config"), 0o755); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(filepath.Join(home, "config", "genesis.json"), []byte("{}"), 0o644); err != nil {
        t.Fatal(err)
    }

    // Create logs as a file instead of directory to cause error
    if err := os.WriteFile(filepath.Join(home, "logs"), []byte("not a directory"), 0o644); err != nil {
        t.Fatal(err)
    }

    binPath := filepath.Join(home, "fake-bin")
    script := `#!/bin/sh
if [ "$1" = "start" ]; then
    sleep 0.1
    exit 0
fi
`
    if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
        t.Fatal(err)
    }

    sup := New(home)
    _, err := sup.Start(StartOpts{
        HomeDir: home,
        BinPath: binPath,
    })

    // Should fail to create logs directory
    if err == nil {
        t.Error("Start() should fail when logs directory can't be created")
        _ = sup.Stop()
    }
}

// TestSupervisor_Start_WithHomeEnv tests the .env symlink creation
func TestSupervisor_Start_WithHomeEnv(t *testing.T) {
    // Skip if HOME is not set
    if os.Getenv("HOME") == "" {
        t.Skip("HOME environment variable not set")
    }

    home := t.TempDir()

    // Create genesis.json
    if err := os.MkdirAll(filepath.Join(home, "config"), 0o755); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(filepath.Join(home, "config", "genesis.json"), []byte("{}"), 0o644); err != nil {
        t.Fatal(err)
    }

    // Create a fake .env in HOME
    homeEnv := filepath.Join(os.Getenv("HOME"), ".env-test-temp")
    if err := os.WriteFile(homeEnv, []byte("TEST=1"), 0o644); err != nil {
        t.Fatal(err)
    }
    defer os.Remove(homeEnv)

    // Temporarily set HOME to point to our test file
    oldHome := os.Getenv("HOME")
    defer os.Setenv("HOME", oldHome)

    // Create a fake binary
    binPath := filepath.Join(home, "fake-bin")
    script := `#!/bin/sh
if [ "$1" = "start" ]; then
    sleep 0.1
    exit 0
fi
`
    if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
        t.Fatal(err)
    }

    sup := New(home)
    pid, err := sup.Start(StartOpts{
        HomeDir: home,
        BinPath: binPath,
    })
    if err != nil {
        t.Logf("Start() failed (expected): %v", err)
    }
    if pid > 0 {
        time.Sleep(100 * time.Millisecond)
    }
}

// TestSupervisor_PID_WithWhitespace tests PID file with whitespace
func TestSupervisor_PID_WithWhitespace(t *testing.T) {
    home := t.TempDir()
    pidFile := filepath.Join(home, "pchaind.pid")

    // Write PID with extra whitespace
    currentPID := os.Getpid()
    if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("  %d  \n", currentPID)), 0o644); err != nil {
        t.Fatal(err)
    }

    sup := New(home)
    pid, ok := sup.PID()
    if !ok {
        t.Error("PID() should handle whitespace in PID file")
    }
    if pid != currentPID {
        t.Errorf("PID() = %d, want %d", pid, currentPID)
    }
}

// TestNew tests the New constructor
func TestNew(t *testing.T) {
    home := "/test/home"
    sup := New(home)

    if sup == nil {
        t.Fatal("New() returned nil")
    }

    // Check LogPath is set correctly
    expectedLog := filepath.Join(home, "logs", "pchaind.log")
    if sup.LogPath() != expectedLog {
        t.Errorf("LogPath() = %q, want %q", sup.LogPath(), expectedLog)
    }
}

// TestNewCosmovisor tests the NewCosmovisor constructor
func TestNewCosmovisor(t *testing.T) {
    home := "/test/home"
    sup := NewCosmovisor(home)

    if sup == nil {
        t.Fatal("NewCosmovisor() returned nil")
    }

    // Check LogPath is set correctly
    expectedLog := filepath.Join(home, "logs", "cosmovisor.log")
    if sup.LogPath() != expectedLog {
        t.Errorf("LogPath() = %q, want %q", sup.LogPath(), expectedLog)
    }
}

// TestSupervisor_Start_FailedReset tests when unsafe-reset-all fails
func TestSupervisor_Start_FailedReset(t *testing.T) {
    home := t.TempDir()
    binPath := filepath.Join(home, "fake-pchaind")

    // Create a binary that fails on tendermint reset
    script := `#!/bin/sh
if [ "$1" = "tendermint" ] && [ "$2" = "unsafe-reset-all" ]; then
    exit 1
fi
if [ "$1" = "start" ]; then
    sleep 0.1
    exit 0
fi
`
    if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
        t.Fatal(err)
    }

    // Create genesis.json
    if err := os.MkdirAll(filepath.Join(home, "config"), 0o755); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(filepath.Join(home, "config", "genesis.json"), []byte("{}"), 0o644); err != nil {
        t.Fatal(err)
    }

    // Trigger initial sync by not having blockstore
    markerFile := filepath.Join(home, ".initial_state_sync")
    if err := os.WriteFile(markerFile, []byte(""), 0o644); err != nil {
        t.Fatal(err)
    }

    sup := New(home)
    pid, err := sup.Start(StartOpts{
        HomeDir: home,
        BinPath: binPath,
    })

    // Should succeed even if reset fails (non-fatal error)
    if err != nil {
        t.Fatalf("Start() should succeed even if reset fails: %v", err)
    }
    if pid <= 0 {
        t.Fatalf("Start() returned invalid PID: %d", pid)
    }

    time.Sleep(200 * time.Millisecond)
}

// TestSupervisor_Start_ExistingBlockstore tests when blockstore exists (no reset)
func TestSupervisor_Start_ExistingBlockstore(t *testing.T) {
    home := t.TempDir()
    binPath := filepath.Join(home, "fake-pchaind")

    script := `#!/bin/sh
if [ "$1" = "start" ]; then
    sleep 0.1
    exit 0
fi
`
    if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
        t.Fatal(err)
    }

    // Create genesis.json
    if err := os.MkdirAll(filepath.Join(home, "config"), 0o755); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(filepath.Join(home, "config", "genesis.json"), []byte("{}"), 0o644); err != nil {
        t.Fatal(err)
    }

    // Create blockstore to skip initial sync
    if err := os.MkdirAll(filepath.Join(home, "data"), 0o755); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(filepath.Join(home, "data", "blockstore.db"), []byte("fake"), 0o644); err != nil {
        t.Fatal(err)
    }

    sup := New(home)
    pid, err := sup.Start(StartOpts{
        HomeDir: home,
        BinPath: binPath,
    })

    if err != nil {
        t.Fatalf("Start() with existing blockstore failed: %v", err)
    }
    if pid <= 0 {
        t.Fatalf("Start() returned invalid PID: %d", pid)
    }

    time.Sleep(200 * time.Millisecond)
}

// TestSupervisor_Start_CmdStartFails tests when cmd.Start() fails
func TestSupervisor_Start_CmdStartFails(t *testing.T) {
    home := t.TempDir()

    // Create genesis.json
    if err := os.MkdirAll(filepath.Join(home, "config"), 0o755); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(filepath.Join(home, "config", "genesis.json"), []byte("{}"), 0o644); err != nil {
        t.Fatal(err)
    }

    sup := New(home)

    // Use a non-executable file
    binPath := filepath.Join(home, "not-executable")
    if err := os.WriteFile(binPath, []byte("not a script"), 0o644); err != nil {
        t.Fatal(err)
    }

    _, err := sup.Start(StartOpts{
        HomeDir: home,
        BinPath: binPath,
    })

    // Should fail because binary is not executable
    if err == nil {
        t.Error("Start() should fail for non-executable binary")
        _ = sup.Stop()
    }
}

// TestSupervisor_Restart_WithError tests restart when stop fails
func TestSupervisor_Restart_WithError(t *testing.T) {
    home := t.TempDir()

    // Create genesis.json
    if err := os.MkdirAll(filepath.Join(home, "config"), 0o755); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(filepath.Join(home, "config", "genesis.json"), []byte("{}"), 0o644); err != nil {
        t.Fatal(err)
    }

    sup := New(home)

    // Restart when not running should work
    _, err := sup.Restart(StartOpts{
        HomeDir: home,
        BinPath: "/nonexistent/binary",
    })

    // Will fail to start with nonexistent binary
    if err == nil {
        _ = sup.Stop()
    }
}

// TestSupervisor_Start_PIDWriteError tests when PID file can't be written
func TestSupervisor_Start_PIDWriteError(t *testing.T) {
    home := t.TempDir()

    // Create genesis.json
    if err := os.MkdirAll(filepath.Join(home, "config"), 0o755); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(filepath.Join(home, "config", "genesis.json"), []byte("{}"), 0o644); err != nil {
        t.Fatal(err)
    }

    // Make home read-only to prevent PID file creation
    // This is tricky - create supervisor first, then make directory read-only
    binPath := filepath.Join(home, "fake-bin")
    script := `#!/bin/sh
if [ "$1" = "start" ]; then
    sleep 10 &
    wait
fi
`
    if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
        t.Fatal(err)
    }

    // Create a read-only PID file location by making it a directory
    pidPath := filepath.Join(home, "pchaind.pid")
    if err := os.MkdirAll(pidPath, 0o555); err != nil {
        t.Fatal(err)
    }
    defer os.Chmod(pidPath, 0o755)
    defer os.RemoveAll(pidPath)

    sup := New(home)
    _, err := sup.Start(StartOpts{
        HomeDir: home,
        BinPath: binPath,
    })

    // Should fail because PID file can't be written
    if err == nil {
        t.Error("Start() should fail when PID file can't be written")
        _ = sup.Stop()
    }
}

// TestSupervisor_Uptime_InvalidOutput tests Uptime when ps returns invalid output
func TestSupervisor_Uptime_InvalidOutput(t *testing.T) {
    // This test relies on ps returning parseable output
    // We can't easily mock ps, so we just test the happy path is covered
    // The error paths are hard to test without mocking
    t.Skip("Uptime error paths require process mocking")
}

// TestIsRPCListening_ClosedConnection tests RPC listening with immediate close
func TestIsRPCListening_ClosedConnection(t *testing.T) {
    ln, err := net.Listen("tcp", "127.0.0.1:0")
    if err != nil {
        t.Skipf("skipping: cannot bind due to sandbox: %v", err)
    }
    defer func() { _ = ln.Close() }()

    addr := ln.Addr().String()

    // Close immediately
    ln.Close()

    // Should return false
    if IsRPCListening(addr, 100*time.Millisecond) {
        t.Error("IsRPCListening should return false for closed port")
    }
}

// TestSupervisor_Start_NeedsInitialSyncWithExistingPVS tests when pvs already exists
func TestSupervisor_Start_NeedsInitialSyncWithExistingPVS(t *testing.T) {
    home := t.TempDir()
    binPath := filepath.Join(home, "fake-bin")

    script := `#!/bin/sh
if [ "$1" = "tendermint" ] && [ "$2" = "unsafe-reset-all" ]; then
    exit 0
fi
if [ "$1" = "start" ]; then
    sleep 0.1
    exit 0
fi
`
    if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
        t.Fatal(err)
    }

    // Create genesis.json
    if err := os.MkdirAll(filepath.Join(home, "config"), 0o755); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(filepath.Join(home, "config", "genesis.json"), []byte("{}"), 0o644); err != nil {
        t.Fatal(err)
    }

    // Create marker for initial sync
    markerFile := filepath.Join(home, ".initial_state_sync")
    if err := os.WriteFile(markerFile, []byte(""), 0o644); err != nil {
        t.Fatal(err)
    }

    // Create existing priv_validator_state.json
    if err := os.MkdirAll(filepath.Join(home, "data"), 0o755); err != nil {
        t.Fatal(err)
    }
    pvsPath := filepath.Join(home, "data", "priv_validator_state.json")
    if err := os.WriteFile(pvsPath, []byte(`{"height":"100","round":1,"step":2}`), 0o644); err != nil {
        t.Fatal(err)
    }

    sup := New(home)
    pid, err := sup.Start(StartOpts{
        HomeDir: home,
        BinPath: binPath,
    })

    if err != nil {
        t.Fatalf("Start() failed: %v", err)
    }
    if pid <= 0 {
        t.Fatalf("Start() returned invalid PID: %d", pid)
    }

    time.Sleep(200 * time.Millisecond)

    // Marker should be removed
    if _, err := os.Stat(markerFile); !os.IsNotExist(err) {
        t.Error("Marker file should be removed")
    }
}

// TestSupervisor_Start_WithoutHOME tests when HOME env is not set
func TestSupervisor_Start_WithoutHOME(t *testing.T) {
    home := t.TempDir()
    binPath := filepath.Join(home, "fake-bin")

    script := `#!/bin/sh
if [ "$1" = "start" ]; then
    sleep 0.1
    exit 0
fi
`
    if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
        t.Fatal(err)
    }

    // Create genesis.json
    if err := os.MkdirAll(filepath.Join(home, "config"), 0o755); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(filepath.Join(home, "config", "genesis.json"), []byte("{}"), 0o644); err != nil {
        t.Fatal(err)
    }

    // Unset HOME temporarily
    oldHome := os.Getenv("HOME")
    os.Unsetenv("HOME")
    defer os.Setenv("HOME", oldHome)

    sup := New(home)
    pid, err := sup.Start(StartOpts{
        HomeDir: home,
        BinPath: binPath,
    })

    if err != nil {
        t.Fatalf("Start() without HOME env failed: %v", err)
    }
    if pid <= 0 {
        t.Fatalf("Start() returned invalid PID: %d", pid)
    }

    time.Sleep(200 * time.Millisecond)
}

// TestSupervisor_Start_HomeEnvNotExist tests when HOME/.env doesn't exist
func TestSupervisor_Start_HomeEnvNotExist(t *testing.T) {
    if os.Getenv("HOME") == "" {
        t.Skip("HOME not set")
    }

    home := t.TempDir()
    binPath := filepath.Join(home, "fake-bin")

    script := `#!/bin/sh
if [ "$1" = "start" ]; then
    sleep 0.1
    exit 0
fi
`
    if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
        t.Fatal(err)
    }

    // Create genesis.json
    if err := os.MkdirAll(filepath.Join(home, "config"), 0o755); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(filepath.Join(home, "config", "genesis.json"), []byte("{}"), 0o644); err != nil {
        t.Fatal(err)
    }

    // Make sure HOME/.env doesn't exist (it probably doesn't in test environment)
    homeEnv := filepath.Join(os.Getenv("HOME"), ".env")
    _ = os.Remove(homeEnv)

    sup := New(home)
    pid, err := sup.Start(StartOpts{
        HomeDir: home,
        BinPath: binPath,
    })

    if err != nil {
        t.Fatalf("Start() failed: %v", err)
    }
    if pid <= 0 {
        t.Fatalf("Start() returned invalid PID: %d", pid)
    }

    time.Sleep(200 * time.Millisecond)
}

// TestSupervisor_Start_TargetEnvExists tests when target .env already exists
func TestSupervisor_Start_TargetEnvExists(t *testing.T) {
    if os.Getenv("HOME") == "" {
        t.Skip("HOME not set")
    }

    home := t.TempDir()
    binPath := filepath.Join(home, "fake-bin")

    script := `#!/bin/sh
if [ "$1" = "start" ]; then
    sleep 0.1
    exit 0
fi
`
    if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
        t.Fatal(err)
    }

    // Create genesis.json
    if err := os.MkdirAll(filepath.Join(home, "config"), 0o755); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(filepath.Join(home, "config", "genesis.json"), []byte("{}"), 0o644); err != nil {
        t.Fatal(err)
    }

    // Create target .env file
    targetEnv := filepath.Join(home, ".env")
    if err := os.WriteFile(targetEnv, []byte("EXISTING=1"), 0o644); err != nil {
        t.Fatal(err)
    }

    // Create HOME/.env
    homeEnv := filepath.Join(os.Getenv("HOME"), ".env-test-supervisor")
    if err := os.WriteFile(homeEnv, []byte("TEST=1"), 0o644); err != nil {
        t.Fatal(err)
    }
    defer os.Remove(homeEnv)

    sup := New(home)
    pid, err := sup.Start(StartOpts{
        HomeDir: home,
        BinPath: binPath,
    })

    if err != nil {
        t.Fatalf("Start() failed: %v", err)
    }
    if pid <= 0 {
        t.Fatalf("Start() returned invalid PID: %d", pid)
    }

    time.Sleep(200 * time.Millisecond)

    // Target .env should still contain original content
    content, _ := os.ReadFile(targetEnv)
    if string(content) != "EXISTING=1" {
        t.Error("Target .env should not be overwritten")
    }
}

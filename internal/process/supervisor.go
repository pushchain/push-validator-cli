package process

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Supervisor controls the pchaind process: start/stop/restart and status.
// Implementation handles detached exec, PID files, and log paths.
type Supervisor interface {
	Start(opts StartOpts) (int, error) // returns PID
	Stop() error
	Restart(opts StartOpts) (int, error)
	IsRunning() bool
	PID() (int, bool)
	Uptime() (time.Duration, bool) // returns uptime duration and whether process is running
	LogPath() string
}

// StartOpts captures settings for launching the daemon.
type StartOpts struct {
	HomeDir   string
	Moniker   string
	BinPath   string   // path to pchaind (defaults to "pchaind" if empty)
	ExtraArgs []string // additional args to append after defaults
}

type supervisor struct {
	pidFile string
	logFile string
	mu      sync.Mutex
}

// New returns a process supervisor bound to the given home dir.
func New(home string) Supervisor {
	return &supervisor{
		pidFile: filepath.Join(home, "pchaind.pid"),
		logFile: filepath.Join(home, "logs", "pchaind.log"),
	}
}

func (s *supervisor) LogPath() string { return s.logFile }

func (s *supervisor) PID() (int, bool) {
	b, err := os.ReadFile(s.pidFile)
	if err != nil {
		return 0, false
	}
	txt := strings.TrimSpace(string(b))
	if txt == "" {
		return 0, false
	}
	pid, err := strconv.Atoi(txt)
	if err != nil {
		return 0, false
	}
	if processAlive(pid) {
		return pid, true
	}
	// Process is dead - clean up stale PID file
	_ = os.Remove(s.pidFile)
	return 0, false
}

func (s *supervisor) IsRunning() bool {
	_, ok := s.PID()
	return ok
}

func (s *supervisor) Uptime() (time.Duration, bool) {
	pid, ok := s.PID()
	if !ok {
		return 0, false
	}

	// Use ps to get elapsed time in seconds (works on Linux and macOS)
	cmd := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "etimes=")
	out, err := cmd.Output()
	if err != nil {
		return 0, false
	}

	// Parse elapsed seconds
	elapsed := strings.TrimSpace(string(out))
	seconds, err := strconv.ParseInt(elapsed, 10, 64)
	if err != nil {
		return 0, false
	}

	return time.Duration(seconds) * time.Second, true
}

func (s *supervisor) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	pid, ok := s.PID()
	if !ok {
		return nil
	}
	// Try graceful TERM to process group first, fall back to individual PID
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
		_ = syscall.Kill(pid, syscall.SIGTERM)
	}
	// Wait up to 15 seconds
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			_ = os.Remove(s.pidFile)
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	// Force kill process group, fall back to individual PID
	if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
	// Poll for process death after SIGKILL (up to 5 seconds)
	killDeadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(killDeadline) {
		if !processAlive(pid) {
			_ = os.Remove(s.pidFile)
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	_ = os.Remove(s.pidFile)
	if processAlive(pid) {
		return errors.New("failed to stop pchaind")
	}
	return nil
}

func (s *supervisor) Restart(opts StartOpts) (int, error) {
	if err := s.Stop(); err != nil {
		return 0, err
	}
	return s.Start(opts)
}

func (s *supervisor) Start(opts StartOpts) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if opts.HomeDir == "" {
		return 0, errors.New("HomeDir required")
	}
	if s.IsRunning() {
		pid, _ := s.PID()
		return pid, nil
	}

	// Check if genesis.json exists before starting
	genesisPath := filepath.Join(opts.HomeDir, "config", "genesis.json")
	if _, err := os.Stat(genesisPath); os.IsNotExist(err) {
		return 0, fmt.Errorf("genesis.json not found at %s. Please run 'init' first", genesisPath)
	}

	// Check if this node needs initial sync (fresh start or marked for sync)
	needsInitialSyncPath := filepath.Join(opts.HomeDir, ".initial_state_sync")
	blockstorePath := filepath.Join(opts.HomeDir, "data", "blockstore.db")

	needsInitialSync := false
	if _, err := os.Stat(needsInitialSyncPath); err == nil {
		needsInitialSync = true
	} else if _, err := os.Stat(blockstorePath); os.IsNotExist(err) {
		needsInitialSync = true
	}

	// If initial sync is needed, reset data right before starting
	if needsInitialSync {
		bin := opts.BinPath
		if bin == "" {
			bin = "pchaind"
		}

		// Run tendermint unsafe-reset-all to clear data for sync
		cmd := exec.Command(bin, "tendermint", "unsafe-reset-all", "--home", opts.HomeDir, "--keep-addr-book")
		if err := cmd.Run(); err != nil {
			// Non-fatal: continue anyway as node might work
			_ = err
		}

		// Remove the marker file after processing
		_ = os.Remove(needsInitialSyncPath)
	}

	// Always ensure priv_validator_state.json exists before starting
	pvsPath := filepath.Join(opts.HomeDir, "data", "priv_validator_state.json")
	if _, err := os.Stat(pvsPath); os.IsNotExist(err) {
		_ = os.MkdirAll(filepath.Join(opts.HomeDir, "data"), 0o755)
		_ = os.WriteFile(pvsPath, []byte(`{"height":"0","round":0,"step":0}`), 0o644)
	}

	if err := os.MkdirAll(filepath.Join(opts.HomeDir, "logs"), 0o755); err != nil {
		return 0, err
	}
	bin := opts.BinPath
	if bin == "" {
		bin = "pchaind"
	}

	// Build args: pchaind start --home <home>
	args := []string{"start", "--home", opts.HomeDir, "--log_level", "statesync:debug,*:info"}
	// if RPC port env set, leave default
	if len(opts.ExtraArgs) > 0 {
		args = append(args, opts.ExtraArgs...)
	}

	// Auto-symlink ~/.env to HomeDir/.env if it exists and target doesn't
	if home := os.Getenv("HOME"); home != "" {
		homeEnv := filepath.Join(home, ".env")
		targetEnv := filepath.Join(opts.HomeDir, ".env")
		if _, err := os.Stat(homeEnv); err == nil {
			if _, err := os.Stat(targetEnv); os.IsNotExist(err) {
				_ = os.Symlink(homeEnv, targetEnv)
			}
		}
	}

	// Open/append log file
	lf, err := os.OpenFile(s.logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return 0, err
	}

	cmd := exec.Command(bin, args...)
	cmd.Dir = opts.HomeDir // Set working directory so pchaind finds .env
	cmd.Stdout = lf
	cmd.Stderr = lf
	cmd.Stdin = nil
	// Detach from this session/process group
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		_ = lf.Close()
		return 0, fmt.Errorf("start pchaind: %w", err)
	}
	// Write PID file
	pid := cmd.Process.Pid
	if err := os.WriteFile(s.pidFile, []byte(strconv.Itoa(pid)), 0o644); err != nil {
		// Best effort stop if we can't persist PID
		_ = syscall.Kill(pid, syscall.SIGTERM)
		_ = lf.Close()
		return 0, err
	}
	// We do not wait; keep log file open a bit to avoid losing early bytes
	go func(f *os.File) {
		// Flush quickly and close after a small delay
		time.Sleep(500 * time.Millisecond)
		_ = f.Sync()
		_ = f.Close()
	}(lf)
	return pid, nil
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	// signal 0 tests for existence without sending a signal
	err := syscall.Kill(pid, 0)
	return err == nil
}

// IsRPCListening returns true if TCP connection to the RPC port succeeds.
func IsRPCListening(hostport string, timeout time.Duration) bool {
	if hostport == "" {
		hostport = "127.0.0.1:26657"
	}
	d := net.Dialer{Timeout: timeout}
	conn, err := d.Dial("tcp", hostport)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}


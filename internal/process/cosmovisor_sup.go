package process

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/pushchain/push-validator-cli/internal/cosmovisor"
)

// CosmovisorSupervisor manages pchaind through Cosmovisor.
type CosmovisorSupervisor struct {
	homeDir  string
	pidFile  string
	logFile  string
	cosmoSvc cosmovisor.Service
	mu       sync.Mutex
}

// NewCosmovisor returns a Cosmovisor-aware supervisor.
func NewCosmovisor(home string) Supervisor {
	return &CosmovisorSupervisor{
		homeDir:  home,
		pidFile:  filepath.Join(home, "cosmovisor.pid"),
		logFile:  filepath.Join(home, "logs", "cosmovisor.log"),
		cosmoSvc: cosmovisor.New(home),
	}
}

func (s *CosmovisorSupervisor) LogPath() string { return s.logFile }

func (s *CosmovisorSupervisor) PID() (int, bool) {
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
	return 0, false
}

func (s *CosmovisorSupervisor) IsRunning() bool {
	_, ok := s.PID()
	return ok
}

func (s *CosmovisorSupervisor) Uptime() (time.Duration, bool) {
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

func (s *CosmovisorSupervisor) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pid, ok := s.PID()
	if !ok {
		// Try pkill fallback for cosmovisor processes
		_ = exec.Command("pkill", "-f", "cosmovisor run").Run()
		_ = exec.Command("pkill", "-f", "pchaind start").Run()
		return nil
	}

	// Try graceful TERM
	_ = syscall.Kill(pid, syscall.SIGTERM)

	// Wait up to 15 seconds
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			_ = os.Remove(s.pidFile)
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}

	// Force kill
	_ = syscall.Kill(pid, syscall.SIGKILL)
	time.Sleep(500 * time.Millisecond)

	// Also kill any orphaned pchaind processes
	_ = exec.Command("pkill", "-f", "pchaind start").Run()

	_ = os.Remove(s.pidFile)
	if processAlive(pid) {
		return errors.New("failed to stop cosmovisor")
	}
	return nil
}

func (s *CosmovisorSupervisor) Restart(opts StartOpts) (int, error) {
	if err := s.Stop(); err != nil {
		return 0, err
	}
	return s.Start(opts)
}

func (s *CosmovisorSupervisor) Start(opts StartOpts) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if opts.HomeDir == "" {
		opts.HomeDir = s.homeDir
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

	// Auto-initialize Cosmovisor if not set up
	if !s.cosmoSvc.IsSetup() {
		binPath := opts.BinPath
		if binPath == "" {
			binPath = "pchaind"
		}
		// Resolve binary path if it's just the name
		if !filepath.IsAbs(binPath) {
			if resolved, err := exec.LookPath(binPath); err == nil {
				binPath = resolved
			}
		}

		err := s.cosmoSvc.Init(context.Background(), cosmovisor.InitOptions{
			HomeDir: opts.HomeDir,
			BinPath: binPath,
		})
		if err != nil {
			return 0, fmt.Errorf("failed to initialize cosmovisor: %w", err)
		}
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
		// Use the genesis binary for reset
		bin := filepath.Join(s.cosmoSvc.GenesisDir(), "pchaind")
		if _, err := os.Stat(bin); os.IsNotExist(err) {
			bin = "pchaind" // Fall back to PATH
		}

		// Run tendermint unsafe-reset-all to clear data for sync
		cmd := exec.Command(bin, "tendermint", "unsafe-reset-all", "--home", opts.HomeDir, "--keep-addr-book")
		if err := cmd.Run(); err != nil {
			// Non-fatal: continue anyway as node might work
			_ = err
		}

		// Ensure priv_validator_state.json exists after reset
		pvsPath := filepath.Join(opts.HomeDir, "data", "priv_validator_state.json")
		if _, err := os.Stat(pvsPath); os.IsNotExist(err) {
			_ = os.MkdirAll(filepath.Join(opts.HomeDir, "data"), 0o755)
			_ = os.WriteFile(pvsPath, []byte(`{"height":"0","round":0,"step":0}`), 0o644)
		}

		// Remove the marker file after processing
		_ = os.Remove(needsInitialSyncPath)
	}

	// Ensure logs directory exists
	if err := os.MkdirAll(filepath.Join(opts.HomeDir, "logs"), 0o755); err != nil {
		return 0, err
	}

	// Build Cosmovisor command: cosmovisor run start [args]
	args := []string{
		"run", "start",
		"--home", opts.HomeDir,
		"--pruning=everything",
		"--minimum-gas-prices=1000000000upc",
		"--rpc.laddr=tcp://0.0.0.0:26657",
		"--json-rpc.address=0.0.0.0:8545",
		"--json-rpc.ws-address=0.0.0.0:8546",
		"--json-rpc.api=eth,txpool,personal,net,debug,web3",
		"--chain-id=push_42101-1",
		"--log_level", "statesync:debug,*:info",
	}

	// Add extra args if provided
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

	cosmovisorBin := s.cosmoSvc.CosmovisorBinaryPath()
	if cosmovisorBin == "" {
		_ = lf.Close()
		return 0, errors.New("cosmovisor binary not found")
	}

	cmd := exec.Command(cosmovisorBin, args...)
	cmd.Dir = opts.HomeDir
	cmd.Stdout = lf
	cmd.Stderr = lf
	cmd.Stdin = nil

	// Set Cosmovisor environment variables
	cmd.Env = os.Environ()
	for k, v := range s.cosmoSvc.EnvVars() {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Detach from this session/process group
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		_ = lf.Close()
		return 0, fmt.Errorf("start cosmovisor: %w", err)
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
		time.Sleep(500 * time.Millisecond)
		_ = f.Sync()
		_ = f.Close()
	}(lf)

	return pid, nil
}

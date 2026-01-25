package node

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pushchain/push-validator-cli/internal/files"
)

// PeerRefreshService manages automatic peer discovery and config updates.
type PeerRefreshService struct {
	remoteRPC string
	homeDir   string
	interval  time.Duration
	minPeers  int
	maxPeers  int
	logPath   string

	mu       sync.RWMutex
	running  bool
	cancel   context.CancelFunc
	lastRun  time.Time
	lastErr  error
	logger   *log.Logger
}

// PeerRefreshOptions configures the peer refresh service.
type PeerRefreshOptions struct {
	RemoteRPC string        // Remote RPC URL to fetch peers from (e.g., https://donut.rpc.push.org)
	HomeDir   string        // Node home directory (e.g., ~/.pchain)
	Interval  time.Duration // How often to refresh peers (default: 5 minutes)
	MinPeers  int           // Minimum peers to maintain (default: 3)
	MaxPeers  int           // Maximum peers to configure (default: 10)
	LogPath   string        // Path to write refresh logs (optional)
}

// NewPeerRefreshService creates a new peer refresh service.
func NewPeerRefreshService(opts PeerRefreshOptions) *PeerRefreshService {
	if opts.Interval == 0 {
		opts.Interval = 5 * time.Minute
	}
	if opts.MinPeers == 0 {
		opts.MinPeers = 3
	}
	if opts.MaxPeers == 0 {
		opts.MaxPeers = 10
	}
	if opts.LogPath == "" {
		opts.LogPath = filepath.Join(opts.HomeDir, "logs", "peer-refresh.log")
	}

	return &PeerRefreshService{
		remoteRPC: opts.RemoteRPC,
		homeDir:   opts.HomeDir,
		interval:  opts.Interval,
		minPeers:  opts.MinPeers,
		maxPeers:  opts.MaxPeers,
		logPath:   opts.LogPath,
	}
}

// Start begins the background peer refresh loop.
func (s *PeerRefreshService) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}

	// Setup logger
	if err := os.MkdirAll(filepath.Dir(s.logPath), 0o755); err == nil {
		f, err := os.OpenFile(s.logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err == nil {
			s.logger = log.New(f, "", log.LstdFlags)
		}
	}
	if s.logger == nil {
		s.logger = log.New(os.Stderr, "[peer-refresh] ", log.LstdFlags)
	}

	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.running = true
	s.mu.Unlock()

	// Do initial refresh
	if err := s.RefreshOnce(ctx); err != nil {
		s.log("initial refresh failed: %v", err)
	}

	// Start background loop
	go s.loop(ctx)

	return nil
}

// Stop halts the background refresh loop.
func (s *PeerRefreshService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancel != nil {
		s.cancel()
	}
	s.running = false
}

// IsRunning returns whether the service is active.
func (s *PeerRefreshService) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// LastRefresh returns the time of the last successful refresh.
func (s *PeerRefreshService) LastRefresh() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastRun
}

// LastError returns the last error encountered, if any.
func (s *PeerRefreshService) LastError() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastErr
}

// RefreshOnce performs a single peer refresh operation.
func (s *PeerRefreshService) RefreshOnce(ctx context.Context) error {
	s.log("fetching peers from %s", s.remoteRPC)

	// Fetch peers from remote RPC
	peers, err := FetchRemotePeers(ctx, s.remoteRPC, s.maxPeers)
	if err != nil {
		s.mu.Lock()
		s.lastErr = err
		s.mu.Unlock()
		return fmt.Errorf("fetch peers: %w", err)
	}

	if len(peers) < s.minPeers {
		err := fmt.Errorf("only found %d peers (minimum: %d)", len(peers), s.minPeers)
		s.mu.Lock()
		s.lastErr = err
		s.mu.Unlock()
		return err
	}

	s.log("found %d peers, updating config", len(peers))

	// Update config.toml
	cfgs := files.New(s.homeDir)
	if err := cfgs.SetPersistentPeers(peers); err != nil {
		s.mu.Lock()
		s.lastErr = err
		s.mu.Unlock()
		return fmt.Errorf("update config: %w", err)
	}

	s.mu.Lock()
	s.lastRun = time.Now()
	s.lastErr = nil
	s.mu.Unlock()

	s.log("successfully updated %d peers in config.toml", len(peers))
	return nil
}

func (s *PeerRefreshService) loop(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.log("stopping peer refresh loop")
			return
		case <-ticker.C:
			if err := s.RefreshOnce(ctx); err != nil {
				s.log("refresh failed: %v", err)
			}
		}
	}
}

func (s *PeerRefreshService) log(format string, args ...interface{}) {
	if s.logger != nil {
		s.logger.Printf(format, args...)
	}
}

// FetchRemotePeers queries the remote RPC for active peers and returns them
// in the format "nodeID@ip:port".
func FetchRemotePeers(ctx context.Context, remoteRPC string, maxPeers int) ([]string, error) {
	if maxPeers == 0 {
		maxPeers = 10
	}

	cli := New(remoteRPC)
	peers, err := cli.Peers(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, len(peers))
	for _, p := range peers {
		if p.ID == "" || p.Addr == "" {
			continue
		}
		// Format: nodeID@ip:port
		peerStr := fmt.Sprintf("%s@%s", p.ID, p.Addr)
		result = append(result, peerStr)
		if len(result) >= maxPeers {
			break
		}
	}

	return result, nil
}

// RefreshPeersFromRemote is a convenience function that fetches peers from
// remote RPC and updates the local config.toml in one call.
func RefreshPeersFromRemote(ctx context.Context, remoteRPC, homeDir string, maxPeers int) (int, error) {
	peers, err := FetchRemotePeers(ctx, remoteRPC, maxPeers)
	if err != nil {
		return 0, fmt.Errorf("fetch peers: %w", err)
	}

	if len(peers) == 0 {
		return 0, fmt.Errorf("no peers found from remote RPC")
	}

	cfgs := files.New(homeDir)
	if err := cfgs.SetPersistentPeers(peers); err != nil {
		return 0, fmt.Errorf("update config: %w", err)
	}

	return len(peers), nil
}

// GetCurrentPeers reads the current persistent_peers from config.toml.
func GetCurrentPeers(homeDir string) ([]string, error) {
	configPath := filepath.Join(homeDir, "config", "config.toml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	// Parse persistent_peers from config
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "persistent_peers") {
			// Extract value between quotes
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			val := strings.TrimSpace(parts[1])
			val = strings.Trim(val, "\"")
			if val == "" {
				return []string{}, nil
			}
			return strings.Split(val, ","), nil
		}
	}

	return []string{}, nil
}

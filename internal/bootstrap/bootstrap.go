package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pushchain/push-validator-cli/internal/files"
	"github.com/pushchain/push-validator-cli/internal/snapshot"
)

// Hardcoded fullnode peers for P2P connectivity
var fullnodePeers = []string{
	"6751a6539368608a65512d1a4b7ede4a9cd5004f@136.112.142.137:26656",
	"374573900e4365bea5d946dd69c7343e56e4f375@34.72.243.200:26656",
	"deda68a955b352bb201ab54422de1ab35db46652@136.113.195.0:26656",
}

// Options configures the bootstrap process.
type Options struct {
	HomeDir          string                  // Node home directory (e.g., ~/.pchain)
	ChainID          string                  // Chain ID (e.g., push_42101-1)
	Moniker          string                  // Node moniker
	Denom            string                  // Staking denom (e.g., upc)
	GenesisDomain    string                  // Genesis RPC domain (e.g., donut.rpc.push.org)
	BinPath          string                  // Path to pchaind binary
	SnapshotURL      string                  // Base URL for snapshot downloads
	Progress         func(string)            // Progress message callback
	SnapshotProgress snapshot.ProgressFunc   // Detailed snapshot progress callback
	SkipSnapshot     bool                    // Skip snapshot download (for separate step)
}

// Service bootstraps a new node with snapshot download.
type Service interface {
	Init(ctx context.Context, opts Options) error
}

// HTTPDoer matches http.Client's Do.
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// Runner runs commands; used for pchaind calls in init flow.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) error
}

type svc struct {
	http     HTTPDoer
	run      Runner
	snapshot snapshot.Service
}

// New builds a default service with real http client and runner.
func New() Service {
	return &svc{
		http:     &http.Client{Timeout: 15 * time.Second},
		run:      defaultRunner{},
		snapshot: snapshot.New(),
	}
}

// NewWith allows injecting dependencies for testing.
func NewWith(h HTTPDoer, r Runner, s snapshot.Service) Service {
	if h == nil {
		h = &http.Client{Timeout: 15 * time.Second}
	}
	if r == nil {
		r = defaultRunner{}
	}
	if s == nil {
		s = snapshot.New()
	}
	return &svc{http: h, run: r, snapshot: s}
}

type defaultRunner struct{}

func (defaultRunner) Run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

// Init initializes a new node by downloading a snapshot.
func (s *svc) Init(ctx context.Context, opts Options) error {
	if opts.HomeDir == "" || opts.ChainID == "" {
		return errors.New("HomeDir and ChainID required")
	}
	if opts.Moniker == "" {
		opts.Moniker = "push-validator"
	}
	if opts.Denom == "" {
		opts.Denom = "upc"
	}
	if opts.BinPath == "" {
		opts.BinPath = "pchaind"
	}
	if opts.GenesisDomain == "" {
		return errors.New("GenesisDomain required")
	}
	if opts.SnapshotURL == "" {
		opts.SnapshotURL = snapshot.DefaultSnapshotURL
	}

	progress := opts.Progress
	if progress == nil {
		progress = func(string) {} // no-op if not provided
	}

	// Step 1: Ensure base directories
	progress("Setting up node directories...")
	if err := os.MkdirAll(filepath.Join(opts.HomeDir, "config"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(opts.HomeDir, "logs"), 0o755); err != nil {
		return err
	}

	// Step 2: Run `pchaind init` if config is missing
	cfgPath := filepath.Join(opts.HomeDir, "config", "config.toml")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		progress("Running pchaind init...")
		if err := s.run.Run(ctx, opts.BinPath, "init", opts.Moniker, "--chain-id", opts.ChainID, "--default-denom", opts.Denom, "--home", opts.HomeDir, "--overwrite"); err != nil {
			return fmt.Errorf("pchaind init: %w", err)
		}
		// In test environments where the runner is a noop, ensure the file exists
		if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
			if mkerr := os.MkdirAll(filepath.Dir(cfgPath), 0o755); mkerr == nil {
				_ = os.WriteFile(cfgPath, []byte(""), 0o644)
			}
		}
	}

	// Step 3: Fetch genesis from remote
	progress("Fetching genesis from network...")
	base := baseURL(opts.GenesisDomain)
	genesisURL := base + "/genesis"
	gen, err := s.getGenesis(ctx, genesisURL)
	if err != nil {
		return fmt.Errorf("fetch genesis: %w", err)
	}
	genPath := filepath.Join(opts.HomeDir, "config", "genesis.json")
	if err := os.WriteFile(genPath, gen, 0o644); err != nil {
		return err
	}

	// Step 4: Configure persistent peers
	progress("Configuring persistent peers...")
	cfgs := files.New(opts.HomeDir)
	if err := cfgs.SetPersistentPeers(fullnodePeers); err != nil {
		return err
	}

	// Step 5: Backup config before modifications
	progress("Backing up configuration...")
	_, _ = cfgs.Backup() // best-effort

	// Step 6: Disable state sync (we're using snapshot download instead)
	progress("Configuring node for snapshot sync...")
	if err := cfgs.DisableStateSync(); err != nil {
		return err
	}

	// Step 7: Write priv_validator_state.json if missing
	pvs := filepath.Join(opts.HomeDir, "data", "priv_validator_state.json")
	if _, err := os.Stat(pvs); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(pvs), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(pvs, []byte("{\n  \"height\": \"0\",\n  \"round\": 0,\n  \"step\": 0\n}\n"), 0o644); err != nil {
			return err
		}
	}

	// Step 8: Download and extract snapshot (unless skipped or already present)
	if opts.SkipSnapshot {
		progress("Skipping snapshot download (handled separately)")
	} else if snapshot.IsSnapshotPresent(opts.HomeDir) {
		progress("Snapshot already exists, skipping download")
	} else {
		progress("Downloading blockchain snapshot...")
		if err := s.snapshot.Download(ctx, snapshot.Options{
			SnapshotURL: opts.SnapshotURL,
			HomeDir:     opts.HomeDir,
			Progress:    opts.SnapshotProgress,
		}); err != nil {
			return fmt.Errorf("download snapshot: %w", err)
		}

		progress("Extracting snapshot...")
		if err := s.snapshot.Extract(ctx, snapshot.ExtractOptions{
			HomeDir:   opts.HomeDir,
			TargetDir: filepath.Join(opts.HomeDir, "data"),
			Progress:  opts.SnapshotProgress,
		}); err != nil {
			return fmt.Errorf("extract snapshot: %w", err)
		}

		// Mark successful snapshot download
		_ = os.WriteFile(filepath.Join(opts.HomeDir, ".snapshot_downloaded"), []byte(time.Now().Format(time.RFC3339)), 0o644)

		progress("Snapshot downloaded and extracted successfully")
	}
	return nil
}

// ---- helpers ----

func (s *svc) getGenesis(ctx context.Context, url string) ([]byte, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	var payload struct {
		Result struct {
			Genesis json.RawMessage `json:"genesis"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if len(payload.Result.Genesis) == 0 {
		return nil, errors.New("empty genesis")
	}
	return payload.Result.Genesis, nil
}

func baseURL(genesisDomain string) string {
	d := strings.TrimSpace(genesisDomain)
	if strings.HasPrefix(d, "http://") || strings.HasPrefix(d, "https://") {
		return d
	}
	if d == "" {
		return "https://donut.rpc.push.org"
	}
	return "https://" + d
}

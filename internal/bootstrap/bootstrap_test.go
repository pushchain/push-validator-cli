package bootstrap

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pushchain/push-validator-cli/internal/snapshot"
)

// fakeRunner just records invocations without executing anything.
type fakeRunner struct{ calls [][]string }

func (f *fakeRunner) Run(ctx context.Context, name string, args ...string) error {
	call := append([]string{name}, args...)
	f.calls = append(f.calls, call)
	return nil
}

// fakeSnapshot is a mock snapshot service for testing.
type fakeSnapshot struct{}

func (fakeSnapshot) Download(ctx context.Context, opts snapshot.Options) error {
	// Create a minimal data directory structure to simulate snapshot extraction
	dataDir := filepath.Join(opts.HomeDir, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	// Create a marker file to indicate snapshot was "extracted"
	return os.WriteFile(filepath.Join(dataDir, ".snapshot_extracted"), []byte("test"), 0o644)
}

func (fakeSnapshot) Extract(ctx context.Context, opts snapshot.ExtractOptions) error {
	return nil
}

func (fakeSnapshot) IsCacheValid(ctx context.Context, opts snapshot.Options) (bool, error) {
	return true, nil
}

func TestBootstrap_Init_FullFlow(t *testing.T) {
	// Skip if sandbox disallows binding
	if ln, err := net.Listen("tcp", "127.0.0.1:0"); err != nil {
		t.Skip("binding disabled in sandbox")
	} else {
		ln.Close()
	}

	mux := http.NewServeMux()
	// JSON-RPC POST status handler at root for light client probes
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":{"node_info":{"id":"test"},"sync_info":{"latest_block_height":"5000","catching_up":true}}}`))
			return
		}
		http.NotFound(w, r)
	})
	mux.HandleFunc("/genesis", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{"result": map[string]any{"genesis": map[string]any{"chain_id": "push_42101-1"}}}
		_ = json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/net_info", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{"result": map[string]any{"peers": []map[string]any{
			{"node_info": map[string]any{"id": "id1", "listen_addr": "tcp://0.0.0.0:26656"}, "remote_ip": "10.0.0.1"},
			{"node_info": map[string]any{"id": "id2", "listen_addr": "tcp://1.2.3.4:26656"}, "remote_ip": "1.2.3.4"},
		}}}
		_ = json.NewEncoder(w).Encode(resp)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	home := t.TempDir()
	r := &fakeRunner{}
	svc := NewWith(srv.Client(), r, fakeSnapshot{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := svc.Init(ctx, Options{
		HomeDir:       home,
		ChainID:       "push_42101-1",
		Moniker:       "testnode",
		GenesisDomain: srv.URL, // full URL supported
		BinPath:       "pchaind",
		SnapshotURL:   srv.URL, // uses fake snapshot service anyway
	})
	if err != nil {
		t.Fatalf("init error: %v", err)
	}

	// Verify files written
	if _, err := os.Stat(filepath.Join(home, "config", "genesis.json")); err != nil {
		t.Fatalf("missing genesis.json: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(home, "config", "config.toml"))
	if err != nil {
		t.Fatalf("missing config.toml: %v", err)
	}
	s := string(b)
	if !containsAll(s, []string{"[p2p]", "persistent_peers", "addr_book_strict = false"}) {
		t.Fatalf("p2p peers not configured: %s", s)
	}
	// State sync should be DISABLED (we use snapshot download instead)
	if !strings.Contains(s, "[statesync]") || !strings.Contains(s, "enable = false") {
		t.Fatalf("statesync should be disabled: %s", s)
	}
	// Verify snapshot was "extracted"
	if _, err := os.Stat(filepath.Join(home, "data", ".snapshot_extracted")); err != nil {
		t.Fatalf("snapshot not extracted: %v", err)
	}
	// Verify runner was invoked for init
	if len(r.calls) == 0 {
		t.Fatalf("runner not called")
	}
}

func containsAll(s string, subs []string) bool {
	for _, sub := range subs {
		if !contains(s, sub) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }

func TestBootstrap_Init_MissingHomeDir(t *testing.T) {
	svc := New()
	ctx := context.Background()

	err := svc.Init(ctx, Options{
		ChainID: "push_42101-1",
		// HomeDir is missing
	})

	if err == nil {
		t.Fatal("Init() with missing HomeDir should return error")
	}
}

func TestBootstrap_Init_MissingChainID(t *testing.T) {
	svc := New()
	ctx := context.Background()

	err := svc.Init(ctx, Options{
		HomeDir: t.TempDir(),
		// ChainID is missing
	})

	if err == nil {
		t.Fatal("Init() with missing ChainID should return error")
	}
}

func TestBootstrap_Init_MissingGenesisDomain(t *testing.T) {
	svc := New()
	ctx := context.Background()

	err := svc.Init(ctx, Options{
		HomeDir: t.TempDir(),
		ChainID: "push_42101-1",
		// GenesisDomain is missing
	})

	if err == nil {
		t.Fatal("Init() with missing GenesisDomain should return error")
	}
}

func TestBootstrap_Init_GenesisDownloadError(t *testing.T) {
	if ln, err := net.Listen("tcp", "127.0.0.1:0"); err != nil {
		t.Skip("binding disabled in sandbox")
	} else {
		ln.Close()
	}

	// Server returns error status
	mux := http.NewServeMux()
	mux.HandleFunc("/genesis", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	home := t.TempDir()
	r := &fakeRunner{}
	svc := NewWith(srv.Client(), r, fakeSnapshot{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := svc.Init(ctx, Options{
		HomeDir:       home,
		ChainID:       "push_42101-1",
		GenesisDomain: srv.URL,
		BinPath:       "pchaind",
	})

	if err == nil {
		t.Fatal("Init() with genesis download error should return error")
	}
	if !strings.Contains(err.Error(), "fetch genesis") {
		t.Errorf("expected 'fetch genesis' error, got: %v", err)
	}
}

func TestBootstrap_Init_InvalidGenesisJSON(t *testing.T) {
	if ln, err := net.Listen("tcp", "127.0.0.1:0"); err != nil {
		t.Skip("binding disabled in sandbox")
	} else {
		ln.Close()
	}

	// Server returns invalid JSON
	mux := http.NewServeMux()
	mux.HandleFunc("/genesis", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not valid json`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	home := t.TempDir()
	r := &fakeRunner{}
	svc := NewWith(srv.Client(), r, fakeSnapshot{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := svc.Init(ctx, Options{
		HomeDir:       home,
		ChainID:       "push_42101-1",
		GenesisDomain: srv.URL,
		BinPath:       "pchaind",
	})

	if err == nil {
		t.Fatal("Init() with invalid genesis JSON should return error")
	}
}

func TestBootstrap_Init_EmptyGenesis(t *testing.T) {
	if ln, err := net.Listen("tcp", "127.0.0.1:0"); err != nil {
		t.Skip("binding disabled in sandbox")
	} else {
		ln.Close()
	}

	// Server returns response with missing genesis field (will decode to empty RawMessage)
	mux := http.NewServeMux()
	mux.HandleFunc("/genesis", func(w http.ResponseWriter, r *http.Request) {
		// Send a response where genesis is completely missing or empty object
		_, _ = w.Write([]byte(`{"result":{}}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	home := t.TempDir()
	r := &fakeRunner{}
	svc := NewWith(srv.Client(), r, fakeSnapshot{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := svc.Init(ctx, Options{
		HomeDir:       home,
		ChainID:       "push_42101-1",
		GenesisDomain: srv.URL,
		BinPath:       "pchaind",
	})

	if err == nil {
		t.Fatal("Init() with empty genesis should return error")
	}
	if !strings.Contains(err.Error(), "empty genesis") {
		t.Errorf("expected 'empty genesis' error, got: %v", err)
	}
}

func TestBootstrap_Init_WithDefaults(t *testing.T) {
	if ln, err := net.Listen("tcp", "127.0.0.1:0"); err != nil {
		t.Skip("binding disabled in sandbox")
	} else {
		ln.Close()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/genesis", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{"result": map[string]any{"genesis": map[string]any{"chain_id": "push_42101-1"}}}
		_ = json.NewEncoder(w).Encode(resp)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	home := t.TempDir()
	r := &fakeRunner{}
	svc := NewWith(srv.Client(), r, fakeSnapshot{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := svc.Init(ctx, Options{
		HomeDir:       home,
		ChainID:       "push_42101-1",
		GenesisDomain: srv.URL,
		// Moniker, Denom, BinPath not specified - should use defaults
	})

	if err != nil {
		t.Fatalf("Init() with defaults should succeed, got error: %v", err)
	}

	// Verify runner was called with default values
	if len(r.calls) == 0 {
		t.Fatal("runner not called")
	}

	// Check that runner was invoked with expected defaults
	call := r.calls[0]
	if call[0] != "pchaind" {
		t.Errorf("expected binary 'pchaind', got %q", call[0])
	}

	// Verify default moniker and denom were used in the call
	foundMoniker := false
	foundDenom := false
	for i, arg := range call {
		if arg == "--default-denom" && i+1 < len(call) {
			if call[i+1] == "upc" {
				foundDenom = true
			}
		}
		if arg == "push-validator" {
			foundMoniker = true
		}
	}

	if !foundDenom {
		t.Error("expected default denom 'upc' not found in runner call")
	}
	if !foundMoniker {
		t.Error("expected default moniker 'push-validator' not found in runner call")
	}
}

func TestBootstrap_Init_SkipSnapshot(t *testing.T) {
	if ln, err := net.Listen("tcp", "127.0.0.1:0"); err != nil {
		t.Skip("binding disabled in sandbox")
	} else {
		ln.Close()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/genesis", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{"result": map[string]any{"genesis": map[string]any{"chain_id": "push_42101-1"}}}
		_ = json.NewEncoder(w).Encode(resp)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	home := t.TempDir()
	r := &fakeRunner{}
	svc := NewWith(srv.Client(), r, fakeSnapshot{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := svc.Init(ctx, Options{
		HomeDir:       home,
		ChainID:       "push_42101-1",
		GenesisDomain: srv.URL,
		SkipSnapshot:  true, // Skip snapshot download
	})

	if err != nil {
		t.Fatalf("Init() with SkipSnapshot should succeed, got error: %v", err)
	}

	// Verify snapshot was NOT extracted (no marker file)
	if _, err := os.Stat(filepath.Join(home, "data", ".snapshot_extracted")); !os.IsNotExist(err) {
		t.Error("snapshot should not have been extracted when SkipSnapshot=true")
	}
}

func TestBootstrap_Init_SnapshotAlreadyPresent(t *testing.T) {
	if ln, err := net.Listen("tcp", "127.0.0.1:0"); err != nil {
		t.Skip("binding disabled in sandbox")
	} else {
		ln.Close()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/genesis", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{"result": map[string]any{"genesis": map[string]any{"chain_id": "push_42101-1"}}}
		_ = json.NewEncoder(w).Encode(resp)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	home := t.TempDir()

	// Create blockstore.db directory structure to simulate snapshot already present
	blockstoreDir := filepath.Join(home, "data", "blockstore.db")
	if err := os.MkdirAll(blockstoreDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(blockstoreDir, "CURRENT"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := &fakeRunner{}
	svc := NewWith(srv.Client(), r, fakeSnapshot{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := svc.Init(ctx, Options{
		HomeDir:       home,
		ChainID:       "push_42101-1",
		GenesisDomain: srv.URL,
	})

	if err != nil {
		t.Fatalf("Init() with existing snapshot should succeed, got error: %v", err)
	}

	// Snapshot extraction should have been skipped (no new marker file from fakeSnapshot)
	// The existing blockstore.db should still be there
	if _, err := os.Stat(filepath.Join(blockstoreDir, "CURRENT")); err != nil {
		t.Error("existing blockstore.db should still be present")
	}
}

func TestBootstrap_Init_ConfigAlreadyExists(t *testing.T) {
	if ln, err := net.Listen("tcp", "127.0.0.1:0"); err != nil {
		t.Skip("binding disabled in sandbox")
	} else {
		ln.Close()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/genesis", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{"result": map[string]any{"genesis": map[string]any{"chain_id": "push_42101-1"}}}
		_ = json.NewEncoder(w).Encode(resp)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	home := t.TempDir()

	// Create config.toml to simulate already initialized
	if err := os.MkdirAll(filepath.Join(home, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "config", "config.toml"), []byte("existing config"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := &fakeRunner{}
	svc := NewWith(srv.Client(), r, fakeSnapshot{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := svc.Init(ctx, Options{
		HomeDir:       home,
		ChainID:       "push_42101-1",
		GenesisDomain: srv.URL,
	})

	if err != nil {
		t.Fatalf("Init() with existing config should succeed, got error: %v", err)
	}

	// pchaind init should have been skipped (runner calls should be minimal or none)
	// Verify the existing config.toml is modified for peers
	content, err := os.ReadFile(filepath.Join(home, "config", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(content), "persistent_peers") {
		t.Error("config.toml should be updated with persistent_peers")
	}
}

func TestBaseURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain domain",
			input: "donut.rpc.push.org",
			want:  "https://donut.rpc.push.org",
		},
		{
			name:  "http URL",
			input: "http://localhost:26657",
			want:  "http://localhost:26657",
		},
		{
			name:  "https URL",
			input: "https://rpc.example.com",
			want:  "https://rpc.example.com",
		},
		{
			name:  "empty string",
			input: "",
			want:  "https://donut.rpc.push.org",
		},
		{
			name:  "whitespace",
			input: "  ",
			want:  "https://donut.rpc.push.org",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := baseURL(tt.input)
			if got != tt.want {
				t.Errorf("baseURL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

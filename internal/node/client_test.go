package node

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClient_Status(t *testing.T) {
	// Skip if sandbox disallows binding
	if ln, err := net.Listen("tcp", "127.0.0.1:0"); err != nil {
		t.Skip("skipping due to sandbox")
	} else {
		ln.Close()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"result": map[string]interface{}{
				"node_info": map[string]interface{}{
					"id":      "test-node-id",
					"moniker": "test-moniker",
					"network": "push_42101-1",
				},
				"sync_info": map[string]interface{}{
					"catching_up":         true,
					"latest_block_height": "12345",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := New(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	status, err := client.Status(ctx)
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}

	if status.NodeID != "test-node-id" {
		t.Errorf("NodeID = %q, want %q", status.NodeID, "test-node-id")
	}
	if status.Moniker != "test-moniker" {
		t.Errorf("Moniker = %q, want %q", status.Moniker, "test-moniker")
	}
	if status.Network != "push_42101-1" {
		t.Errorf("Network = %q, want %q", status.Network, "push_42101-1")
	}
	if !status.CatchingUp {
		t.Error("CatchingUp = false, want true")
	}
	if status.Height != 12345 {
		t.Errorf("Height = %d, want 12345", status.Height)
	}
}

func TestClient_RemoteStatus(t *testing.T) {
	if ln, err := net.Listen("tcp", "127.0.0.1:0"); err != nil {
		t.Skip("skipping due to sandbox")
	} else {
		ln.Close()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"result": map[string]interface{}{
				"node_info": map[string]interface{}{
					"id":      "remote-id",
					"moniker": "remote-moniker",
					"network": "push_42101-1",
				},
				"sync_info": map[string]interface{}{
					"catching_up":         false,
					"latest_block_height": "99999",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := New("http://localhost:26657") // doesn't matter for RemoteStatus
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	status, err := client.RemoteStatus(ctx, srv.URL)
	if err != nil {
		t.Fatalf("RemoteStatus() error: %v", err)
	}

	if status.NodeID != "remote-id" {
		t.Errorf("NodeID = %q, want %q", status.NodeID, "remote-id")
	}
	if status.CatchingUp {
		t.Error("CatchingUp = true, want false")
	}
	if status.Height != 99999 {
		t.Errorf("Height = %d, want 99999", status.Height)
	}
}

func TestClient_Peers(t *testing.T) {
	if ln, err := net.Listen("tcp", "127.0.0.1:0"); err != nil {
		t.Skip("skipping due to sandbox")
	} else {
		ln.Close()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/net_info", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"result": map[string]interface{}{
				"peers": []map[string]interface{}{
					{
						"node_info": map[string]interface{}{
							"id":          "peer1-id",
							"listen_addr": "tcp://0.0.0.0:26656",
						},
						"remote_ip": "192.168.1.10",
					},
					{
						"node_info": map[string]interface{}{
							"id":          "peer2-id",
							"listen_addr": "tcp://0.0.0.0:26656",
						},
						"remote_ip": "192.168.1.20",
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := New(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	peers, err := client.Peers(ctx)
	if err != nil {
		t.Fatalf("Peers() error: %v", err)
	}

	if len(peers) != 2 {
		t.Fatalf("len(peers) = %d, want 2", len(peers))
	}

	if peers[0].ID != "peer1-id" {
		t.Errorf("peers[0].ID = %q, want %q", peers[0].ID, "peer1-id")
	}
	if peers[0].Addr != "192.168.1.10:26656" {
		t.Errorf("peers[0].Addr = %q, want %q", peers[0].Addr, "192.168.1.10:26656")
	}

	if peers[1].ID != "peer2-id" {
		t.Errorf("peers[1].ID = %q, want %q", peers[1].ID, "peer2-id")
	}
}

func TestClient_Status_BadJSON(t *testing.T) {
	if ln, err := net.Listen("tcp", "127.0.0.1:0"); err != nil {
		t.Skip("skipping due to sandbox")
	} else {
		ln.Close()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := New(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := client.Status(ctx)
	if err == nil {
		t.Fatal("Status() expected error, got nil")
	}
}

func TestClient_Status_ConnectionRefused(t *testing.T) {
	if ln, err := net.Listen("tcp", "127.0.0.1:0"); err != nil {
		t.Skip("skipping due to sandbox")
	} else {
		ln.Close()
	}

	// Use a port that's not listening
	client := New("http://127.0.0.1:19999")
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err := client.Status(ctx)
	if err == nil {
		t.Fatal("Status() expected error, got nil")
	}
}

func TestClient_Peers_EmptyList(t *testing.T) {
	if ln, err := net.Listen("tcp", "127.0.0.1:0"); err != nil {
		t.Skip("skipping due to sandbox")
	} else {
		ln.Close()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/net_info", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"result": map[string]interface{}{
				"peers": []map[string]interface{}{},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := New(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	peers, err := client.Peers(ctx)
	if err != nil {
		t.Fatalf("Peers() error: %v", err)
	}

	if len(peers) != 0 {
		t.Errorf("len(peers) = %d, want 0", len(peers))
	}
}

func TestClient_Peers_FilterInvalidEntries(t *testing.T) {
	if ln, err := net.Listen("tcp", "127.0.0.1:0"); err != nil {
		t.Skip("skipping due to sandbox")
	} else {
		ln.Close()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/net_info", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"result": map[string]interface{}{
				"peers": []map[string]interface{}{
					{
						"node_info": map[string]interface{}{
							"id":          "valid-peer",
							"listen_addr": "tcp://0.0.0.0:26656",
						},
						"remote_ip": "192.168.1.10",
					},
					{
						"node_info": map[string]interface{}{
							"id":          "", // empty ID should be filtered
							"listen_addr": "tcp://0.0.0.0:26656",
						},
						"remote_ip": "192.168.1.20",
					},
					{
						"node_info": map[string]interface{}{
							"id":          "no-ip-peer",
							"listen_addr": "tcp://0.0.0.0:26656",
						},
						"remote_ip": "", // empty IP should be filtered
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := New(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	peers, err := client.Peers(ctx)
	if err != nil {
		t.Fatalf("Peers() error: %v", err)
	}

	if len(peers) != 1 {
		t.Fatalf("len(peers) = %d, want 1 (invalid entries filtered)", len(peers))
	}

	if peers[0].ID != "valid-peer" {
		t.Errorf("peers[0].ID = %q, want %q", peers[0].ID, "valid-peer")
	}
}

func TestDeriveWS(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "http URL",
			input: "http://localhost:26657",
			want:  "ws://localhost:26657/websocket",
		},
		{
			name:  "https URL",
			input: "https://rpc.example.com",
			want:  "wss://rpc.example.com/websocket",
		},
		{
			name:  "plain host",
			input: "localhost:26657",
			want:  "ws://localhost:26657/websocket",
		},
		{
			name:  "with trailing slash",
			input: "http://localhost:26657/",
			want:  "ws://localhost:26657//websocket", // deriveWS doesn't strip trailing slash
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deriveWS(tt.input)
			if got != tt.want {
				t.Errorf("deriveWS(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStrconvParseInt(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		{
			name:  "positive number",
			input: "12345",
			want:  12345,
		},
		{
			name:  "negative number",
			input: "-999",
			want:  -999,
		},
		{
			name:  "zero",
			input: "0",
			want:  0,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "invalid chars",
			input:   "123abc",
			wantErr: true,
		},
		{
			name:    "non-numeric",
			input:   "abc",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := strconvParseInt(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("strconvParseInt(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("strconvParseInt(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseHeaderHeight(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantHeight int64
		wantOK     bool
	}{
		{
			name:       "valid header",
			input:      `{"result":{"data":{"value":{"header":{"height":"42","time":"2024-01-01T00:00:00Z"}}}}}`,
			wantHeight: 42,
			wantOK:     true,
		},
		{
			name:   "invalid JSON",
			input:  `{invalid}`,
			wantOK: false,
		},
		{
			name:   "missing height",
			input:  `{"result":{"data":{"value":{"header":{"time":"2024-01-01T00:00:00Z"}}}}}`,
			wantOK: false,
		},
		{
			name:   "empty height",
			input:  `{"result":{"data":{"value":{"header":{"height":"","time":"2024-01-01T00:00:00Z"}}}}}`,
			wantOK: false,
		},
		{
			name:   "invalid height format",
			input:  `{"result":{"data":{"value":{"header":{"height":"abc","time":"2024-01-01T00:00:00Z"}}}}}`,
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseHeaderHeight([]byte(tt.input))
			if ok != tt.wantOK {
				t.Errorf("parseHeaderHeight() ok = %v, want %v", ok, tt.wantOK)
			}
			if tt.wantOK && got.Height != tt.wantHeight {
				t.Errorf("parseHeaderHeight() height = %d, want %d", got.Height, tt.wantHeight)
			}
		})
	}
}

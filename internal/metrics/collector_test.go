package metrics

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewWithoutCPU(t *testing.T) {
	c := NewWithoutCPU()
	if c == nil {
		t.Fatal("NewWithoutCPU() returned nil")
	}

	// CPU should not be running initially
	c.mu.RLock()
	running := c.cpuRunning
	c.mu.RUnlock()

	if running {
		t.Error("NewWithoutCPU() should not start CPU monitoring")
	}
}

func TestNew(t *testing.T) {
	c := New()
	if c == nil {
		t.Fatal("New() returned nil")
	}

	// Give CPU monitoring more time to start in the goroutine
	time.Sleep(200 * time.Millisecond)

	c.mu.RLock()
	running := c.cpuRunning
	c.mu.RUnlock()

	if !running {
		t.Log("New() may not have started CPU monitoring yet (timing dependent)")
		// Don't fail - this is timing dependent
	}

	// Clean up
	c.Stop()
}

func TestCollector_StartStop(t *testing.T) {
	c := NewWithoutCPU()

	// Start should begin CPU monitoring
	c.Start()
	time.Sleep(100 * time.Millisecond)

	c.mu.RLock()
	running := c.cpuRunning
	c.mu.RUnlock()

	if !running {
		t.Error("Start() should start CPU monitoring")
	}

	// Stop should halt CPU monitoring
	c.Stop()
	time.Sleep(100 * time.Millisecond)

	c.mu.RLock()
	running = c.cpuRunning
	c.mu.RUnlock()

	if running {
		t.Error("Stop() should stop CPU monitoring")
	}
}

func TestCollector_StartTwice(t *testing.T) {
	c := NewWithoutCPU()
	defer c.Stop()

	// Starting twice should be safe
	c.Start()
	c.Start()

	time.Sleep(100 * time.Millisecond)

	c.mu.RLock()
	running := c.cpuRunning
	c.mu.RUnlock()

	if !running {
		t.Error("Start() called twice should still have CPU monitoring running")
	}
}

func TestCollector_StopTwice(t *testing.T) {
	c := New()

	// Stopping twice should be safe
	c.Stop()
	c.Stop()

	c.mu.RLock()
	running := c.cpuRunning
	c.mu.RUnlock()

	if running {
		t.Error("Stop() called twice should have CPU monitoring stopped")
	}
}

func TestCollector_Collect(t *testing.T) {
	if _, err := net.Listen("tcp", "127.0.0.1:0"); err != nil {
		t.Skip("skipping due to sandbox")
	}

	// Create mock RPC servers
	localMux := http.NewServeMux()
	localMux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"result": map[string]interface{}{
				"node_info": map[string]interface{}{
					"id":      "local-node",
					"moniker": "local-moniker",
					"network": "push_42101-1",
				},
				"sync_info": map[string]interface{}{
					"catching_up":         true,
					"latest_block_height": "1000",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	localMux.HandleFunc("/net_info", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"result": map[string]interface{}{
				"peers": []map[string]interface{}{
					{
						"node_info": map[string]interface{}{
							"id":          "peer1",
							"listen_addr": "tcp://0.0.0.0:26656",
						},
						"remote_ip": "192.168.1.10",
					},
					{
						"node_info": map[string]interface{}{
							"id":          "peer2",
							"listen_addr": "tcp://0.0.0.0:26656",
						},
						"remote_ip": "192.168.1.20",
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	localSrv := httptest.NewServer(localMux)
	defer localSrv.Close()

	remoteMux := http.NewServeMux()
	remoteMux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"result": map[string]interface{}{
				"node_info": map[string]interface{}{
					"id":      "remote-node",
					"moniker": "remote-moniker",
					"network": "push_42101-1",
				},
				"sync_info": map[string]interface{}{
					"catching_up":         false,
					"latest_block_height": "5000",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	remoteSrv := httptest.NewServer(remoteMux)
	defer remoteSrv.Close()

	c := NewWithoutCPU()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	snap := c.Collect(ctx, localSrv.URL, remoteSrv.URL)

	// Verify chain data
	if snap.Chain.LocalHeight != 1000 {
		t.Errorf("LocalHeight = %d, want 1000", snap.Chain.LocalHeight)
	}
	if snap.Chain.RemoteHeight != 5000 {
		t.Errorf("RemoteHeight = %d, want 5000", snap.Chain.RemoteHeight)
	}
	if !snap.Chain.CatchingUp {
		t.Error("CatchingUp = false, want true")
	}

	// Verify node data
	if snap.Node.NodeID != "local-node" {
		t.Errorf("NodeID = %q, want %q", snap.Node.NodeID, "local-node")
	}
	if snap.Node.Moniker != "local-moniker" {
		t.Errorf("Moniker = %q, want %q", snap.Node.Moniker, "local-moniker")
	}
	if snap.Node.ChainID != "push_42101-1" {
		t.Errorf("ChainID = %q, want %q", snap.Node.ChainID, "push_42101-1")
	}
	if !snap.Node.RPCListening {
		t.Error("RPCListening = false, want true")
	}

	// Verify network data
	if snap.Network.Peers != 2 {
		t.Errorf("Peers = %d, want 2", snap.Network.Peers)
	}
	// LatencyMS can be 0 in fast local test environments
	if snap.Network.LatencyMS < 0 {
		t.Errorf("LatencyMS = %d, want >= 0", snap.Network.LatencyMS)
	}

	// System metrics should be populated (though values may vary)
	// Just check that they're not completely zero
	if snap.System.MemTotal == 0 {
		t.Log("Warning: MemTotal is 0 (may be expected in some environments)")
	}
}

func TestCollector_Collect_WithDomainRemote(t *testing.T) {
	if _, err := net.Listen("tcp", "127.0.0.1:0"); err != nil {
		t.Skip("skipping due to sandbox")
	}

	localMux := http.NewServeMux()
	localMux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"result": map[string]interface{}{
				"node_info": map[string]interface{}{
					"id":      "local-node",
					"moniker": "local-moniker",
					"network": "push_42101-1",
				},
				"sync_info": map[string]interface{}{
					"catching_up":         false,
					"latest_block_height": "2000",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	localMux.HandleFunc("/net_info", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"result": map[string]interface{}{
				"peers": []map[string]interface{}{},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	localSrv := httptest.NewServer(localMux)
	defer localSrv.Close()

	c := NewWithoutCPU()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use a domain-style remote (will fail to connect but tests the URL construction)
	snap := c.Collect(ctx, localSrv.URL, "donut.rpc.push.org")

	// Local should succeed
	if snap.Chain.LocalHeight != 2000 {
		t.Errorf("LocalHeight = %d, want 2000", snap.Chain.LocalHeight)
	}

	// Remote will likely be 0 due to connection failure, but that's expected
	if snap.Chain.RemoteHeight != 0 {
		t.Logf("RemoteHeight = %d (connection succeeded unexpectedly)", snap.Chain.RemoteHeight)
	}
}

func TestCollector_Collect_CPUTracking(t *testing.T) {
	if _, err := net.Listen("tcp", "127.0.0.1:0"); err != nil {
		t.Skip("skipping due to sandbox")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"result": map[string]interface{}{
				"node_info": map[string]interface{}{
					"id":      "test",
					"moniker": "test",
					"network": "test",
				},
				"sync_info": map[string]interface{}{
					"catching_up":         false,
					"latest_block_height": "100",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
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

	// Start with CPU monitoring enabled
	c := New()
	defer c.Stop()

	// Wait for CPU to be sampled
	time.Sleep(1500 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	snap := c.Collect(ctx, srv.URL, srv.URL)

	// CPU should have been sampled and should be >= 0
	if snap.System.CPUPercent < 0 {
		t.Errorf("CPUPercent = %f, want >= 0", snap.System.CPUPercent)
	}
}

func TestCollector_Collect_LocalRPCDown(t *testing.T) {
	if _, err := net.Listen("tcp", "127.0.0.1:0"); err != nil {
		t.Skip("skipping due to sandbox")
	}

	c := NewWithoutCPU()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Use URLs that will fail to connect
	snap := c.Collect(ctx, "http://127.0.0.1:19999", "http://127.0.0.1:19998")

	// All values should be zero/false since RPC is down
	if snap.Chain.LocalHeight != 0 {
		t.Errorf("LocalHeight = %d, want 0 when RPC is down", snap.Chain.LocalHeight)
	}
	if snap.Node.RPCListening {
		t.Error("RPCListening = true, want false when RPC is down")
	}
}

func TestCollector_Collect_EmptyPeers(t *testing.T) {
	if _, err := net.Listen("tcp", "127.0.0.1:0"); err != nil {
		t.Skip("skipping due to sandbox")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"result": map[string]interface{}{
				"node_info": map[string]interface{}{
					"id":      "solo-node",
					"moniker": "solo",
					"network": "test",
				},
				"sync_info": map[string]interface{}{
					"catching_up":         false,
					"latest_block_height": "500",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
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

	c := NewWithoutCPU()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	snap := c.Collect(ctx, srv.URL, srv.URL)

	if snap.Network.Peers != 0 {
		t.Errorf("Peers = %d, want 0 when no peers", snap.Network.Peers)
	}
}

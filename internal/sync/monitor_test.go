package syncmon

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRenderProgress_FloorClamp(t *testing.T) {
	// 99.97% should floor to 99.97 (floor2), not 100.0
	cur, remote := int64(9997), int64(10000)
	percent := float64(cur) / float64(remote) * 100
	percent = floor2(percent)
	if percent >= 100.0 {
		t.Fatalf("percent should not be 100, got %.2f", percent)
	}
}

func TestIsSyncedQuick(t *testing.T) {
	// skip in sandbox environments that restrict binding
	if _, err := net.Listen("tcp", "127.0.0.1:0"); err != nil {
		t.Skip("skipping due to sandbox")
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"sync_info":{"catching_up":false}}}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	if !isSyncedQuick(srv.URL) {
		t.Fatal("expected synced true")
	}
}

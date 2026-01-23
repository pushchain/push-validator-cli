package main

import (
	"testing"
	"time"

	"github.com/pushchain/push-validator-cli/internal/dashboard"
)

func TestNormalizeDashboardOptions_Defaults(t *testing.T) {
	opts := normalizeDashboardOptions(dashboard.Options{})

	if opts.RefreshInterval != 2*time.Second {
		t.Errorf("RefreshInterval = %v, want 2s", opts.RefreshInterval)
	}
	// With 2s refresh, timeout should be min(15s, 2*2s=4s) = 4s
	if opts.RPCTimeout != 4*time.Second {
		t.Errorf("RPCTimeout = %v, want 4s", opts.RPCTimeout)
	}
}

func TestNormalizeDashboardOptions_CustomRefresh(t *testing.T) {
	opts := normalizeDashboardOptions(dashboard.Options{
		RefreshInterval: 10 * time.Second,
	})

	if opts.RefreshInterval != 10*time.Second {
		t.Errorf("RefreshInterval = %v, want 10s", opts.RefreshInterval)
	}
	// With 10s refresh, timeout should be min(15s, 2*10s=20s) = 15s
	if opts.RPCTimeout != 15*time.Second {
		t.Errorf("RPCTimeout = %v, want 15s", opts.RPCTimeout)
	}
}

func TestNormalizeDashboardOptions_CustomTimeout(t *testing.T) {
	opts := normalizeDashboardOptions(dashboard.Options{
		RefreshInterval: 5 * time.Second,
		RPCTimeout:      3 * time.Second,
	})

	if opts.RefreshInterval != 5*time.Second {
		t.Errorf("RefreshInterval = %v, want 5s", opts.RefreshInterval)
	}
	// Custom timeout should be kept
	if opts.RPCTimeout != 3*time.Second {
		t.Errorf("RPCTimeout = %v, want 3s (custom)", opts.RPCTimeout)
	}
}

func TestNormalizeDashboardOptions_NegativeRefresh(t *testing.T) {
	opts := normalizeDashboardOptions(dashboard.Options{
		RefreshInterval: -1 * time.Second,
	})

	if opts.RefreshInterval != 2*time.Second {
		t.Errorf("RefreshInterval = %v, want 2s (corrected from negative)", opts.RefreshInterval)
	}
}

func TestNormalizeDashboardOptions_ZeroRefresh(t *testing.T) {
	opts := normalizeDashboardOptions(dashboard.Options{
		RefreshInterval: 0,
	})

	if opts.RefreshInterval != 2*time.Second {
		t.Errorf("RefreshInterval = %v, want 2s (corrected from zero)", opts.RefreshInterval)
	}
}

func TestNormalizeDashboardOptions_SmallRefresh(t *testing.T) {
	opts := normalizeDashboardOptions(dashboard.Options{
		RefreshInterval: 1 * time.Second,
	})

	// With 1s refresh, timeout should be min(15s, 2*1s=2s) = 2s
	if opts.RPCTimeout != 2*time.Second {
		t.Errorf("RPCTimeout = %v, want 2s", opts.RPCTimeout)
	}
}

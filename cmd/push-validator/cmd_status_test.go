package main

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/pushchain/push-validator-cli/internal/node"
	"github.com/pushchain/push-validator-cli/internal/validator"
	"gopkg.in/yaml.v3"
)

func TestStatusResult_JSONMarshal(t *testing.T) {
	res := statusResult{
		Running:      true,
		PID:          12345,
		RPCListening: true,
		RPCURL:       "http://127.0.0.1:26657",
		CatchingUp:   false,
		Height:       50000,
		RemoteHeight: 50000,
		SyncProgress: 100.0,
		IsValidator:  true,
		Peers:        5,
		NodeID:       "node123",
		Moniker:      "my-validator",
		Network:      "push_42101-1",
	}

	data, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded statusResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.Running != true {
		t.Error("decoded.Running should be true")
	}
	if decoded.PID != 12345 {
		t.Errorf("decoded.PID = %d, want 12345", decoded.PID)
	}
	if decoded.Height != 50000 {
		t.Errorf("decoded.Height = %d, want 50000", decoded.Height)
	}
	if decoded.IsValidator != true {
		t.Error("decoded.IsValidator should be true")
	}
}

func TestStatusResult_YAMLMarshal(t *testing.T) {
	res := statusResult{
		Running:      true,
		Height:       1000,
		CatchingUp:   true,
		RPCListening: false,
	}

	data, err := yaml.Marshal(res)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}

	if len(data) == 0 {
		t.Error("yaml.Marshal() produced empty output")
	}

	var decoded map[string]interface{}
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	if decoded["running"] != true {
		t.Error("decoded[running] should be true")
	}
}

func TestStatusResult_OmitEmpty(t *testing.T) {
	res := statusResult{
		Running:      false,
		RPCListening: false,
	}

	data, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	// Fields with omitempty should not be present when zero
	if _, ok := decoded["pid"]; ok {
		t.Error("pid should be omitted when zero")
	}
	if _, ok := decoded["node_id"]; ok {
		t.Error("node_id should be omitted when empty")
	}
	if _, ok := decoded["peers"]; ok {
		t.Error("peers should be omitted when zero")
	}
}

func TestStatusResult_ValidatorFields(t *testing.T) {
	res := statusResult{
		IsValidator:        true,
		ValidatorStatus:    "BONDED",
		ValidatorMoniker:   "test-val",
		VotingPower:        1000000,
		VotingPct:          0.05,
		Commission:         "10%",
		CommissionRewards:  "123.456",
		OutstandingRewards: "789.012",
		IsJailed:           true,
		JailReason:         "Downtime",
		JailedUntil:        "2025-01-15T14:30:00Z",
		MissedBlocks:       500,
		Tombstoned:         false,
	}

	data, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded["validator_status"] != "BONDED" {
		t.Errorf("validator_status = %v, want BONDED", decoded["validator_status"])
	}
	if decoded["is_jailed"] != true {
		t.Error("is_jailed should be true")
	}
	if decoded["jail_reason"] != "Downtime" {
		t.Errorf("jail_reason = %v, want Downtime", decoded["jail_reason"])
	}
}

func TestIsJailPeriodExpired(t *testing.T) {
	tests := []struct {
		name        string
		jailedUntil string
		want        bool
	}{
		{
			name:        "empty string",
			jailedUntil: "",
			want:        true,
		},
		{
			name:        "epoch time (1970)",
			jailedUntil: "1970-01-01T00:00:00Z",
			want:        true,
		},
		{
			name:        "past time",
			jailedUntil: "2020-01-01T00:00:00Z",
			want:        true,
		},
		{
			name:        "far future",
			jailedUntil: "2099-12-31T23:59:59Z",
			want:        false,
		},
		{
			name:        "invalid timestamp",
			jailedUntil: "not-a-time",
			want:        false,
		},
		{
			name:        "recent past with nano",
			jailedUntil: time.Now().Add(-1 * time.Hour).Format(time.RFC3339Nano),
			want:        true,
		},
		{
			name:        "near future",
			jailedUntil: time.Now().Add(1 * time.Hour).Format(time.RFC3339Nano),
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isJailPeriodExpired(tt.jailedUntil)
			if got != tt.want {
				t.Errorf("isJailPeriodExpired(%q) = %v, want %v", tt.jailedUntil, got, tt.want)
			}
		})
	}
}

func TestPrintStatusText_NotPanics(t *testing.T) {
	// Save and restore flags
	origOutput := flagOutput
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagOutput = origOutput
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()

	flagOutput = "text"
	flagNoColor = true
	flagNoEmoji = true

	// Test various status scenarios don't panic
	cases := []statusResult{
		{},
		{Running: true, PID: 100},
		{Running: true, RPCListening: true, Height: 5000},
		{Running: true, RPCListening: true, CatchingUp: true, Height: 1000, RemoteHeight: 5000},
		{Running: true, RPCListening: true, IsValidator: true, ValidatorMoniker: "test"},
		{Running: true, RPCListening: true, IsValidator: true, IsJailed: true, JailReason: "Downtime", JailedUntil: "2025-01-01T00:00:00Z"},
		{Running: true, Peers: 1},
		{Running: true, Peers: 5, PeerList: []string{"peer1", "peer2", "peer3", "peer4", "peer5"}},
		{Error: "test error"},
		{Running: true, RPCListening: true, MemoryPct: 55.5, DiskPct: 32.1, BinaryVer: "v1.0.0"},
		{Running: true, RPCListening: true, IsValidator: true, CommissionRewards: "100.5", OutstandingRewards: "200.3"},
		// Jailed validator with all detail fields populated
		{
			Running: true, RPCListening: true, IsValidator: true, IsJailed: true,
			ValidatorMoniker: "jailed-val", ValidatorStatus: "BOND_STATUS_UNBONDING",
			VotingPower: 500, VotingPct: 2.5, Commission: "0.10",
			CommissionRewards: "50.5", OutstandingRewards: "120.3",
			JailReason: "double_sign", JailedUntil: "2099-01-01T00:00:00Z",
			MissedBlocks: 100, Tombstoned: true,
		},
		// Jailed with expired jail time (remaining=0s)
		{
			Running: true, RPCListening: true, IsValidator: true, IsJailed: true,
			ValidatorMoniker: "expired-jail", ValidatorStatus: "BOND_STATUS_UNBONDING",
			JailedUntil: "2020-01-01T00:00:00Z",
		},
		// Validator with VotingPct but no commission/rewards
		{
			Running: true, RPCListening: true, IsValidator: true,
			ValidatorMoniker: "active-val", ValidatorStatus: "BOND_STATUS_BONDED",
			VotingPower: 10000, VotingPct: 15.5,
		},
		// Node with network info, latency, and zero peers hint
		{
			Running: true, RPCListening: true, Peers: 0,
			Network: "push_42101-1", NodeID: "abc123", Moniker: "my-node",
			LatencyMS: 42,
		},
		// Running without PID
		{Running: true, PID: 0, RPCListening: true},
	}

	for i, c := range cases {
		t.Run(fmt.Sprintf("case_%d", i), func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("printStatusText panicked: %v", r)
				}
			}()
			printStatusText(c)
		})
	}
}

func TestComputeStatus_ProcessNotRunning(t *testing.T) {
	d := &Deps{
		Cfg:      testCfg(),
		Sup:      &mockSupervisor{running: false},
		Node:     &mockNodeClient{},
		Fetcher:  &mockFetcher{},
		RPCCheck: func(string, time.Duration) bool { return false },
		Runner:   newMockRunner(),
	}

	res := computeStatus(d)
	if res.Running {
		t.Error("expected Running=false")
	}
	if res.RPCListening {
		t.Error("expected RPCListening=false")
	}
}

func TestComputeStatus_ProcessRunning_RPCDown(t *testing.T) {
	d := &Deps{
		Cfg:      testCfg(),
		Sup:      &mockSupervisor{running: true, pid: 42},
		Node:     &mockNodeClient{},
		Fetcher:  &mockFetcher{},
		RPCCheck: func(string, time.Duration) bool { return false },
		Runner:   newMockRunner(),
	}

	res := computeStatus(d)
	if !res.Running {
		t.Error("expected Running=true")
	}
	if res.PID != 42 {
		t.Errorf("PID = %d, want 42", res.PID)
	}
	if res.RPCListening {
		t.Error("expected RPCListening=false when RPC down")
	}
}

func TestComputeStatus_RPCUp_StatusError(t *testing.T) {
	d := &Deps{
		Cfg:      testCfg(),
		Sup:      &mockSupervisor{running: true, pid: 100},
		Node:     &mockNodeClient{statusErr: fmt.Errorf("connection refused")},
		Fetcher:  &mockFetcher{},
		RPCCheck: func(string, time.Duration) bool { return true },
		Runner:   newMockRunner(),
	}

	res := computeStatus(d)
	if !res.RPCListening {
		t.Error("expected RPCListening=true")
	}
	if res.Error == "" {
		t.Error("expected Error to be set when Status fails")
	}
}

func TestComputeStatus_RPCUp_StatusSuccess_NotValidator(t *testing.T) {
	d := &Deps{
		Cfg: testCfg(),
		Sup: &mockSupervisor{running: true, pid: 200},
		Node: &mockNodeClient{
			status: node.Status{
				Height:     50000,
				CatchingUp: false,
				NodeID:     "node123",
				Moniker:    "my-node",
				Network:    "push_42101-1",
			},
			peers: []node.Peer{
				{ID: "peer1", Addr: "1.2.3.4:26656"},
				{ID: "peer2", Addr: "5.6.7.8:26656"},
			},
		},
		Fetcher: &mockFetcher{
			myValidator: validator.MyValidatorInfo{IsValidator: false},
		},
		RPCCheck: func(string, time.Duration) bool { return true },
		Runner:   newMockRunner(),
	}

	res := computeStatus(d)
	if !res.RPCListening {
		t.Error("expected RPCListening=true")
	}
	if res.Height != 50000 {
		t.Errorf("Height = %d, want 50000", res.Height)
	}
	if res.CatchingUp {
		t.Error("expected CatchingUp=false")
	}
	if res.NodeID != "node123" {
		t.Errorf("NodeID = %q, want node123", res.NodeID)
	}
	if res.IsValidator {
		t.Error("expected IsValidator=false")
	}
	if len(res.PeerList) != 2 {
		t.Errorf("PeerList = %d, want 2", len(res.PeerList))
	}
}

func TestComputeStatus_RPCUp_IsValidator(t *testing.T) {
	d := &Deps{
		Cfg: testCfg(),
		Sup: &mockSupervisor{running: true, pid: 300},
		Node: &mockNodeClient{
			status: node.Status{
				Height:     100000,
				CatchingUp: true,
			},
		},
		Fetcher: &mockFetcher{
			myValidator: validator.MyValidatorInfo{
				IsValidator: true,
				Address:     "pushvaloper1test",
				Moniker:     "test-validator",
				VotingPower: 5000,
				VotingPct:   0.1,
				Commission:  "10%",
				Status:      "BONDED",
				Jailed:      true,
				SlashingInfo: validator.SlashingInfo{
					JailReason:  "Downtime",
					JailedUntil: "2025-06-01T00:00:00Z",
					MissedBlocks: 100,
					Tombstoned:  false,
				},
			},
			commission:  "50.5",
			outstanding: "120.3",
		},
		RPCCheck: func(string, time.Duration) bool { return true },
		Runner:   newMockRunner(),
	}

	res := computeStatus(d)
	if !res.IsValidator {
		t.Error("expected IsValidator=true")
	}
	if res.ValidatorMoniker != "test-validator" {
		t.Errorf("ValidatorMoniker = %q", res.ValidatorMoniker)
	}
	if res.VotingPower != 5000 {
		t.Errorf("VotingPower = %d", res.VotingPower)
	}
	if res.Commission != "10%" {
		t.Errorf("Commission = %q", res.Commission)
	}
	if !res.IsJailed {
		t.Error("expected IsJailed=true")
	}
	if res.JailReason != "Downtime" {
		t.Errorf("JailReason = %q", res.JailReason)
	}
	if res.JailedUntil != "2025-06-01T00:00:00Z" {
		t.Errorf("JailedUntil = %q", res.JailedUntil)
	}
	if res.MissedBlocks != 100 {
		t.Errorf("MissedBlocks = %d", res.MissedBlocks)
	}
	if res.CommissionRewards != "50.5" {
		t.Errorf("CommissionRewards = %q", res.CommissionRewards)
	}
	if res.OutstandingRewards != "120.3" {
		t.Errorf("OutstandingRewards = %q", res.OutstandingRewards)
	}
	if res.CatchingUp != true {
		t.Error("expected CatchingUp=true")
	}
}

func TestRenderSyncProgressDashboard_EdgeCases(t *testing.T) {
	origNoEmoji := flagNoEmoji
	defer func() { flagNoEmoji = origNoEmoji }()
	flagNoEmoji = true

	tests := []struct {
		name       string
		local      int64
		remote     int64
		catchingUp bool
		wantEmpty  bool
	}{
		{"remote zero", 100, 0, false, true},
		{"remote negative", 100, -1, false, true},
		{"in sync", 5000, 5000, false, false},
		{"catching up", 1000, 5000, true, false},
		{"local greater than remote", 5001, 5000, false, false},
		{"local zero", 0, 5000, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderSyncProgressDashboard(tt.local, tt.remote, tt.catchingUp)
			if tt.wantEmpty && result != "" {
				t.Errorf("expected empty result, got %q", result)
			}
			if !tt.wantEmpty && result == "" {
				t.Error("expected non-empty result")
			}
		})
	}
}

func TestComputeStatus_ValidatorDetails(t *testing.T) {
	d := &Deps{
		Cfg: testCfg(),
		Sup: &mockSupervisor{running: true, pid: 500},
		Node: &mockNodeClient{
			status: node.Status{
				Height:     90000,
				CatchingUp: false,
				NodeID:     "abc123",
				Moniker:    "my-node",
				Network:    "push_42101-1",
			},
		},
		Fetcher: &mockFetcher{
			myValidator: validator.MyValidatorInfo{
				IsValidator: true,
				Moniker:     "my-validator",
				VotingPower: 1000,
				VotingPct:   5.5,
				Commission:  "0.10",
				Status:      "BOND_STATUS_BONDED",
				Jailed:      true,
				SlashingInfo: validator.SlashingInfo{
					JailReason:   "double_sign",
					JailedUntil:  "2025-01-01T00:00:00Z",
					MissedBlocks: 50,
					Tombstoned:   true,
				},
			},
			commission: "1.5",
			outstanding: "2.3",
		},
		RPCCheck: func(string, time.Duration) bool { return true },
		Runner:   newMockRunner(),
	}

	res := computeStatus(d)
	if res.Height != 90000 {
		t.Errorf("Height = %d, want 90000", res.Height)
	}
	if !res.IsValidator {
		t.Error("expected IsValidator=true")
	}
	if res.ValidatorMoniker != "my-validator" {
		t.Errorf("ValidatorMoniker = %q, want %q", res.ValidatorMoniker, "my-validator")
	}
	if res.VotingPower != 1000 {
		t.Errorf("VotingPower = %d, want 1000", res.VotingPower)
	}
	if res.Commission != "0.10" {
		t.Errorf("Commission = %q, want %q", res.Commission, "0.10")
	}
	if !res.IsJailed {
		t.Error("expected IsJailed=true")
	}
	if res.JailReason != "double_sign" {
		t.Errorf("JailReason = %q, want %q", res.JailReason, "double_sign")
	}
	if res.JailedUntil != "2025-01-01T00:00:00Z" {
		t.Errorf("JailedUntil = %q", res.JailedUntil)
	}
	if res.MissedBlocks != 50 {
		t.Errorf("MissedBlocks = %d, want 50", res.MissedBlocks)
	}
	if !res.Tombstoned {
		t.Error("expected Tombstoned=true")
	}
	if res.CommissionRewards != "1.5" {
		t.Errorf("CommissionRewards = %q, want %q", res.CommissionRewards, "1.5")
	}
	if res.OutstandingRewards != "2.3" {
		t.Errorf("OutstandingRewards = %q, want %q", res.OutstandingRewards, "2.3")
	}
}

func TestRenderSyncProgressDashboard_WithEmoji(t *testing.T) {
	origNoEmoji := flagNoEmoji
	defer func() { flagNoEmoji = origNoEmoji }()
	flagNoEmoji = false

	// Test catching up with emoji enabled
	result := renderSyncProgressDashboard(1000, 5000, true)
	if result == "" {
		t.Error("expected non-empty result")
	}

	// Test in sync with emoji enabled
	result2 := renderSyncProgressDashboard(5000, 5000, false)
	if result2 == "" {
		t.Error("expected non-empty result for in-sync")
	}
}

func TestRenderSyncProgressDashboard_NegativePercent(t *testing.T) {
	origNoEmoji := flagNoEmoji
	defer func() { flagNoEmoji = origNoEmoji }()
	flagNoEmoji = true

	// local > remote should cap at 100%
	result := renderSyncProgressDashboard(10000, 5000, false)
	if result == "" {
		t.Error("expected non-empty result")
	}
	if !containsSubstr(result, "100.00%") {
		t.Errorf("expected 100%% for local > remote, got %q", result)
	}
}

func TestRenderSyncProgressDashboard_VeryLargeBlocks(t *testing.T) {
	origNoEmoji := flagNoEmoji
	defer func() { flagNoEmoji = origNoEmoji }()
	flagNoEmoji = true

	// Test with large block numbers that would have long ETA
	result := renderSyncProgressDashboard(0, 1000000, true)
	if result == "" {
		t.Error("expected non-empty result")
	}
	if !containsSubstr(result, "ETA:") {
		t.Errorf("expected ETA in result, got %q", result)
	}
}

func TestComputeStatus_RPCURLDefault(t *testing.T) {
	cfg := testCfg()
	cfg.RPCLocal = ""

	d := &Deps{
		Cfg:      cfg,
		Sup:      &mockSupervisor{running: false},
		Node:     &mockNodeClient{},
		Fetcher:  &mockFetcher{},
		RPCCheck: func(string, time.Duration) bool { return false },
		Runner:   newMockRunner(),
	}

	res := computeStatus(d)
	if res.RPCURL != "http://127.0.0.1:26657" {
		t.Errorf("RPCURL = %q, want default", res.RPCURL)
	}
}

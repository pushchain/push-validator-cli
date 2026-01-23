package dashboard

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pushchain/push-validator-cli/internal/config"
	"github.com/pushchain/push-validator-cli/internal/metrics"
)

// Helper function to create test dashboard data
func createTestData() DashboardData {
	return DashboardData{
		Metrics: metrics.Snapshot{
			Chain: metrics.Chain{
				LocalHeight:  100000,
				RemoteHeight: 100100,
				CatchingUp:   true,
			},
			Node: metrics.Node{
				RPCListening: true,
				ChainID:      "pushchain-1",
				Moniker:      "test-node",
				NodeID:       "abc123def456",
			},
			Network: metrics.Network{
				Peers:     10,
				LatencyMS: 50,
			},
			System: metrics.System{
				MemUsed:  1024 * 1024 * 1024,
				MemTotal: 4096 * 1024 * 1024,
				DiskUsed: 10 * 1024 * 1024 * 1024,
				DiskTotal: 100 * 1024 * 1024 * 1024,
			},
		},
		NodeInfo: struct {
			Running   bool
			PID       int
			Uptime    time.Duration
			BinaryVer string
		}{
			Running:   true,
			PID:       12345,
			Uptime:    2 * time.Hour,
			BinaryVer: "v1.0.0",
		},
		MyValidator: struct {
			IsValidator                  bool
			Address                      string
			Moniker                      string
			Status                       string
			VotingPower                  int64
			VotingPct                    float64
			Commission                   string
			CommissionRewards            string
			OutstandingRewards           string
			Jailed                       bool
			SlashingInfo                 struct {
				JailReason  string
				JailedUntil string
				Tombstoned  bool
				MissedBlocks int64
			}
			SlashingInfoError              string
			ValidatorExistsWithSameMoniker bool
			ConflictingMoniker            string
		}{
			IsValidator: true,
			Address:     "pushvaloper1abc123",
			Moniker:     "my-validator",
			Status:      "BONDED",
			VotingPower: 1000000,
			VotingPct:   0.05,
			Commission:  "10.00",
		},
		CLIVersion: "1.0.0",
		LastUpdate: time.Now(),
	}
}

func TestNewHeader(t *testing.T) {
	header := NewHeader()
	if header == nil {
		t.Fatal("NewHeader returned nil")
	}
	if header.ID() != "header" {
		t.Errorf("ID() = %s, want 'header'", header.ID())
	}
	if header.Title() != "PUSH VALIDATOR DASHBOARD" {
		t.Errorf("Title() = %s, want 'PUSH VALIDATOR DASHBOARD'", header.Title())
	}
	if header.MinWidth() != 40 {
		t.Errorf("MinWidth() = %d, want 40", header.MinWidth())
	}
	if header.MinHeight() != 3 {
		t.Errorf("MinHeight() = %d, want 3", header.MinHeight())
	}
}

func TestHeaderView(t *testing.T) {
	header := NewHeader()
	data := createTestData()

	// Update with data
	updated, _ := header.Update(tea.Msg(nil), data)
	header = updated.(*Header)

	// Test View
	view := header.View(100, 10)
	if view == "" {
		t.Error("View returned empty string")
	}
	if !strings.Contains(view, "PUSH VALIDATOR DASHBOARD") {
		t.Error("View should contain dashboard title")
	}

	// Test with invalid dimensions
	view = header.View(0, 0)
	if view != "" {
		t.Error("View should return empty string for invalid dimensions")
	}

	view = header.View(-1, 10)
	if view != "" {
		t.Error("View should return empty string for negative width")
	}
}

func TestHeaderViewWithError(t *testing.T) {
	header := NewHeader()
	data := createTestData()
	data.Err = &testError{msg: "test error"}

	updated, _ := header.Update(tea.Msg(nil), data)
	header = updated.(*Header)

	view := header.View(100, 10)
	if !strings.Contains(view, "test error") {
		t.Error("View should display error message")
	}
}

func TestHeaderViewWithUpdate(t *testing.T) {
	header := NewHeader()
	data := createTestData()
	data.UpdateInfo.Available = true
	data.UpdateInfo.LatestVersion = "1.1.0"

	updated, _ := header.Update(tea.Msg(nil), data)
	header = updated.(*Header)

	view := header.View(100, 10)
	if !strings.Contains(view, "1.1.0") {
		t.Error("View should display update version")
	}
}

func TestNewChainStatus(t *testing.T) {
	comp := NewChainStatus(true)
	if comp == nil {
		t.Fatal("NewChainStatus returned nil")
	}
	if comp.ID() != "chain_status" {
		t.Errorf("ID() = %s, want 'chain_status'", comp.ID())
	}
	if comp.Title() != "Chain Status" {
		t.Errorf("Title() = %s, want 'Chain Status'", comp.Title())
	}
	if comp.MinWidth() != 30 {
		t.Errorf("MinWidth() = %d, want 30", comp.MinWidth())
	}
	if comp.MinHeight() != 10 {
		t.Errorf("MinHeight() = %d, want 10", comp.MinHeight())
	}
}

func TestChainStatusView(t *testing.T) {
	comp := NewChainStatus(true)
	data := createTestData()

	updated, _ := comp.Update(tea.Msg(nil), data)
	comp = updated.(*ChainStatus)

	view := comp.View(80, 12)
	if view == "" {
		t.Error("View returned empty string")
	}
	// Title is uppercased and styled with ANSI codes
	viewUpper := strings.ToUpper(view)
	if !strings.Contains(viewUpper, "CHAIN STATUS") {
		t.Errorf("View should contain title, got: %s", view)
	}

	// Test with invalid dimensions
	view = comp.View(-1, 12)
	if view == "" {
		t.Error("View should handle negative width gracefully")
	}
}

func TestNewNodeStatus(t *testing.T) {
	comp := NewNodeStatus(true)
	if comp == nil {
		t.Fatal("NewNodeStatus returned nil")
	}
	if comp.ID() != "node_status" {
		t.Errorf("ID() = %s, want 'node_status'", comp.ID())
	}
	if comp.Title() != "Node Status" {
		t.Errorf("Title() = %s, want 'Node Status'", comp.Title())
	}
	if comp.MinWidth() != 25 {
		t.Errorf("MinWidth() = %d, want 25", comp.MinWidth())
	}
	if comp.MinHeight() != 8 {
		t.Errorf("MinHeight() = %d, want 8", comp.MinHeight())
	}
}

func TestNodeStatusView(t *testing.T) {
	comp := NewNodeStatus(true)
	data := createTestData()

	updated, _ := comp.Update(tea.Msg(nil), data)
	comp = updated.(*NodeStatus)

	view := comp.View(60, 10)
	if view == "" {
		t.Error("View returned empty string")
	}
	viewUpper := strings.ToUpper(view)
	if !strings.Contains(viewUpper, "NODE STATUS") {
		t.Errorf("View should contain title, got: %s", view)
	}

	// Test when node is stopped
	data.NodeInfo.Running = false
	updated, _ = comp.Update(tea.Msg(nil), data)
	comp = updated.(*NodeStatus)
	view = comp.View(60, 10)
	if !strings.Contains(view, "Stopped") {
		t.Error("View should show Stopped status")
	}
}

func TestNewNetworkStatus(t *testing.T) {
	comp := NewNetworkStatus(true)
	if comp == nil {
		t.Fatal("NewNetworkStatus returned nil")
	}
	if comp.ID() != "network_status" {
		t.Errorf("ID() = %s, want 'network_status'", comp.ID())
	}
	if comp.Title() != "Network Status" {
		t.Errorf("Title() = %s, want 'Network Status'", comp.Title())
	}
	if comp.MinWidth() != 25 {
		t.Errorf("MinWidth() = %d, want 25", comp.MinWidth())
	}
	if comp.MinHeight() != 8 {
		t.Errorf("MinHeight() = %d, want 8", comp.MinHeight())
	}
}

func TestNetworkStatusView(t *testing.T) {
	comp := NewNetworkStatus(true)
	data := createTestData()
	data.PeerList = []struct {
		ID   string
		Addr string
	}{
		{ID: "peer1", Addr: "127.0.0.1:26656"},
		{ID: "peer2", Addr: "127.0.0.1:26657"},
	}

	updated, _ := comp.Update(tea.Msg(nil), data)
	comp = updated.(*NetworkStatus)

	view := comp.View(70, 10)
	if view == "" {
		t.Error("View returned empty string")
	}
	viewUpper := strings.ToUpper(view)
	if !strings.Contains(viewUpper, "NETWORK STATUS") {
		t.Errorf("View should contain title, got: %s", view)
	}
	if !strings.Contains(view, "2 peers") {
		t.Error("View should show peer count")
	}
}

func TestNewValidatorInfo(t *testing.T) {
	comp := NewValidatorInfo(true)
	if comp == nil {
		t.Fatal("NewValidatorInfo returned nil")
	}
	if comp.ID() != "validator_info" {
		t.Errorf("ID() = %s, want 'validator_info'", comp.ID())
	}
	if comp.Title() != "My Validator Status" {
		t.Errorf("Title() = %s, want 'My Validator Status'", comp.Title())
	}
	if comp.MinWidth() != 30 {
		t.Errorf("MinWidth() = %d, want 30", comp.MinWidth())
	}
	if comp.MinHeight() != 10 {
		t.Errorf("MinHeight() = %d, want 10", comp.MinHeight())
	}
}

func TestValidatorInfoView(t *testing.T) {
	comp := NewValidatorInfo(true)
	data := createTestData()

	updated, _ := comp.Update(tea.Msg(nil), data)
	comp = updated.(*ValidatorInfo)

	view := comp.View(70, 15)
	if view == "" {
		t.Error("View returned empty string")
	}
	viewUpper := strings.ToUpper(view)
	if !strings.Contains(viewUpper, "MY VALIDATOR STATUS") {
		t.Errorf("View should contain title, got: %s", view)
	}

	// Test not registered (clear all validator data)
	data.MyValidator.IsValidator = false
	data.MyValidator.Moniker = ""
	data.MyValidator.Status = ""
	updated, _ = comp.Update(tea.Msg(nil), data)
	comp = updated.(*ValidatorInfo)
	view = comp.View(70, 15)
	if !strings.Contains(strings.ToLower(view), "not registered") {
		t.Errorf("View should show not registered message, got: %s", view)
	}
}

func TestValidatorInfoViewJailed(t *testing.T) {
	comp := NewValidatorInfo(true)
	data := createTestData()
	data.MyValidator.Jailed = true
	data.MyValidator.SlashingInfo.JailReason = "Downtime"

	updated, _ := comp.Update(tea.Msg(nil), data)
	comp = updated.(*ValidatorInfo)

	view := comp.View(100, 20)
	if !strings.Contains(view, "Downtime") {
		t.Error("View should show jail reason")
	}
}

func TestNewValidatorsList(t *testing.T) {
	cfg := config.Config{
		HomeDir:   "/tmp/test",
		RPCLocal:  "http://localhost:26657",
	}
	comp := NewValidatorsList(true, cfg)
	if comp == nil {
		t.Fatal("NewValidatorsList returned nil")
	}
	if comp.ID() != "validators_list" {
		t.Errorf("ID() = %s, want 'validators_list'", comp.ID())
	}
	if comp.MinWidth() != 30 {
		t.Errorf("MinWidth() = %d, want 30", comp.MinWidth())
	}
	if comp.MinHeight() != 16 {
		t.Errorf("MinHeight() = %d, want 16", comp.MinHeight())
	}
}

func TestValidatorsListView(t *testing.T) {
	cfg := config.Config{
		HomeDir:   "/tmp/test",
		RPCLocal:  "http://localhost:26657",
	}
	comp := NewValidatorsList(true, cfg)
	data := createTestData()
	data.NetworkValidators.Total = 2
	data.NetworkValidators.Validators = []struct {
		Moniker              string
		Status               string
		VotingPower          int64
		Commission           string
		CommissionRewards    string
		OutstandingRewards   string
		Address              string
		EVMAddress           string
		Jailed               bool
	}{
		{
			Moniker:     "validator1",
			Status:      "BONDED",
			VotingPower: 1000000,
			Commission:  "10.00",
			Address:     "pushvaloper1abc",
		},
		{
			Moniker:     "validator2",
			Status:      "BONDED",
			VotingPower: 500000,
			Commission:  "5.00",
			Address:     "pushvaloper1def",
		},
	}

	updated, _ := comp.Update(tea.Msg(nil), data)
	comp = updated.(*ValidatorsList)

	view := comp.View(150, 20)
	if view == "" {
		t.Error("View returned empty string")
	}
	viewUpper := strings.ToUpper(view)
	if !strings.Contains(viewUpper, "NETWORK VALIDATORS") {
		t.Errorf("View should contain title, got: %s", view)
	}
}

func TestValidatorsListPagination(t *testing.T) {
	cfg := config.Config{
		HomeDir:   "/tmp/test",
		RPCLocal:  "http://localhost:26657",
	}
	comp := NewValidatorsList(true, cfg)
	data := createTestData()
	data.NetworkValidators.Total = 10

	// Create 10 validators
	validators := make([]struct {
		Moniker              string
		Status               string
		VotingPower          int64
		Commission           string
		CommissionRewards    string
		OutstandingRewards   string
		Address              string
		EVMAddress           string
		Jailed               bool
	}, 10)
	for i := 0; i < 10; i++ {
		validators[i].Moniker = "validator" + string(rune('0'+i))
		validators[i].Status = "BONDED"
		validators[i].VotingPower = int64(1000000 - i*1000)
		validators[i].Commission = "10.00"
		validators[i].Address = "pushvaloper" + string(rune('a'+i))
	}
	data.NetworkValidators.Validators = validators

	updated, _ := comp.Update(tea.Msg(nil), data)
	comp = updated.(*ValidatorsList)

	// Test pagination keys
	updated, _ = comp.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}, data)
	comp = updated.(*ValidatorsList)

	view := comp.View(150, 25)
	// The title changes when there are multiple pages
	if !strings.Contains(strings.ToUpper(view), "PAGE 2") {
		t.Errorf("View should show Page 2 after next page key, got: %s", view)
	}
}

func TestComponentCaching(t *testing.T) {
	comp := NewNodeStatus(true)
	data := createTestData()

	updated, _ := comp.Update(tea.Msg(nil), data)
	comp = updated.(*NodeStatus)

	// First render
	view1 := comp.View(60, 10)
	if view1 == "" {
		t.Fatal("First view is empty")
	}

	// Second render with same data should use cache
	view2 := comp.View(60, 10)
	if view2 != view1 {
		t.Error("Cached view should be identical")
	}

	// Render with different dimensions should not use cache
	view3 := comp.View(80, 10)
	if view3 == "" {
		t.Error("View with different width should render")
	}
}

// testError implements error interface for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

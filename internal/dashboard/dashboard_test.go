package dashboard

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pushchain/push-validator-cli/internal/config"
)

// Test tickCmd
func TestTickCmd(t *testing.T) {
	interval := 100 * time.Millisecond
	cmd := tickCmd(interval)

	if cmd == nil {
		t.Fatal("tickCmd returned nil")
	}

	// Execute the command
	msg := cmd()

	// Should return a tickMsg
	if _, ok := msg.(tickMsg); !ok {
		t.Errorf("Expected tickMsg, got %T", msg)
	}
}

// Test getCommandHelpText
func TestGetCommandHelpText(t *testing.T) {
	helpText := getCommandHelpText()

	if helpText == "" {
		t.Error("getCommandHelpText returned empty string")
	}

	// Check for expected sections
	expectedSections := []string{
		"Push Validator Manager",
		"USAGE",
		"Quick Start",
		"Operations",
		"Validator",
		"Maintenance",
		"Utilities",
		"push-validator start",
		"push-validator status",
		"push-validator dashboard",
	}

	for _, section := range expectedSections {
		if !strings.Contains(helpText, section) {
			t.Errorf("Help text should contain %q", section)
		}
	}
}

// Test newKeyMap
func TestNewKeyMap(t *testing.T) {
	keys := newKeyMap()

	// Check that all keys are initialized
	if keys.Quit.Keys() == nil {
		t.Error("Quit key not initialized")
	}
	if keys.Refresh.Keys() == nil {
		t.Error("Refresh key not initialized")
	}
	if keys.Help.Keys() == nil {
		t.Error("Help key not initialized")
	}
	if keys.Up.Keys() == nil {
		t.Error("Up key not initialized")
	}
	if keys.Down.Keys() == nil {
		t.Error("Down key not initialized")
	}
	if keys.Left.Keys() == nil {
		t.Error("Left key not initialized")
	}
	if keys.Right.Keys() == nil {
		t.Error("Right key not initialized")
	}
	if keys.Search.Keys() == nil {
		t.Error("Search key not initialized")
	}
	if keys.Follow.Keys() == nil {
		t.Error("Follow key not initialized")
	}
	if keys.Home.Keys() == nil {
		t.Error("Home key not initialized")
	}
	if keys.End.Keys() == nil {
		t.Error("End key not initialized")
	}
}

// Test keyMap ShortHelp
func TestKeyMapShortHelp(t *testing.T) {
	keys := newKeyMap()
	shortHelp := keys.ShortHelp()

	if len(shortHelp) != 3 {
		t.Errorf("Expected 3 short help bindings, got %d", len(shortHelp))
	}
}

// Test keyMap FullHelp
func TestKeyMapFullHelp(t *testing.T) {
	keys := newKeyMap()
	fullHelp := keys.FullHelp()

	if len(fullHelp) != 3 {
		t.Errorf("Expected 3 groups of help bindings, got %d", len(fullHelp))
	}

	// Each group should have bindings
	for i, group := range fullHelp {
		if len(group) == 0 {
			t.Errorf("Group %d has no bindings", i)
		}
	}
}

// Test New
func TestNew(t *testing.T) {
	opts := Options{
		Config: config.Config{
			HomeDir:   "/tmp/test",
			RPCLocal:  "http://localhost:26657",
		},
		RefreshInterval: 1 * time.Second,
		RPCTimeout:      5 * time.Second,
		CLIVersion:      "1.0.0",
		NoEmoji:         true,
	}

	dashboard := New(opts)

	if dashboard == nil {
		t.Fatal("New returned nil")
	}

	if dashboard.registry == nil {
		t.Error("Registry not initialized")
	}

	if dashboard.layout == nil {
		t.Error("Layout not initialized")
	}

	if dashboard.collector == nil {
		t.Error("Collector not initialized")
	}

	if !dashboard.loading {
		t.Error("Dashboard should start in loading state")
	}

	if dashboard.showHelp {
		t.Error("Dashboard should not show help initially")
	}
}

// Test New with zero RefreshInterval defaults
func TestNewWithDefaults(t *testing.T) {
	opts := Options{
		Config: config.Config{
			HomeDir:   "/tmp/test",
			RPCLocal:  "http://localhost:26657",
		},
		CLIVersion: "1.0.0",
	}

	dashboard := New(opts)

	if dashboard.opts.RefreshInterval <= 0 {
		t.Error("RefreshInterval should be set to default")
	}

	if dashboard.opts.RPCTimeout <= 0 {
		t.Error("RPCTimeout should be set to default")
	}
}

// Test Init
func TestDashboardInit(t *testing.T) {
	opts := Options{
		Config: config.Config{
			HomeDir:   "/tmp/test",
			RPCLocal:  "http://localhost:26657",
		},
		RefreshInterval: 1 * time.Second,
		CLIVersion:      "1.0.0",
		NoEmoji:         true,
	}

	dashboard := New(opts)
	cmd := dashboard.Init()

	if cmd == nil {
		t.Error("Init should return a command")
	}
}

// Test View when loading
func TestDashboardViewLoading(t *testing.T) {
	opts := Options{
		Config: config.Config{
			HomeDir:   "/tmp/test",
			RPCLocal:  "http://localhost:26657",
		},
		RefreshInterval: 1 * time.Second,
		CLIVersion:      "1.0.0",
		NoEmoji:         true,
	}

	dashboard := New(opts)
	dashboard.width = 100
	dashboard.height = 30

	view := dashboard.View()

	if !strings.Contains(view, "CONNECTING TO RPC") {
		t.Error("Loading view should show connection message")
	}
}

// Test View with zero dimensions
func TestDashboardViewZeroDimensions(t *testing.T) {
	opts := Options{
		Config: config.Config{
			HomeDir:   "/tmp/test",
			RPCLocal:  "http://localhost:26657",
		},
		RefreshInterval: 1 * time.Second,
		CLIVersion:      "1.0.0",
		NoEmoji:         true,
	}

	dashboard := New(opts)
	view := dashboard.View()

	if view != "" {
		t.Error("View should return empty string with zero dimensions")
	}
}

// Test handleKey with quit
func TestHandleKeyQuit(t *testing.T) {
	opts := Options{
		Config: config.Config{
			HomeDir:   "/tmp/test",
			RPCLocal:  "http://localhost:26657",
		},
		RefreshInterval: 1 * time.Second,
		CLIVersion:      "1.0.0",
		NoEmoji:         true,
	}

	dashboard := New(opts)

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	_, cmd := dashboard.handleKey(msg)

	// Quit should return tea.Quit command
	if cmd == nil {
		t.Error("Quit should return a command")
	}
}

// Test handleKey with refresh
func TestHandleKeyRefresh(t *testing.T) {
	opts := Options{
		Config: config.Config{
			HomeDir:   "/tmp/test",
			RPCLocal:  "http://localhost:26657",
		},
		RefreshInterval: 1 * time.Second,
		CLIVersion:      "1.0.0",
		NoEmoji:         true,
	}

	dashboard := New(opts)

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}
	_, cmd := dashboard.handleKey(msg)

	if cmd == nil {
		t.Error("Refresh should return a command")
	}

	// Execute command and verify it returns forceRefreshMsg
	result := cmd()
	if _, ok := result.(forceRefreshMsg); !ok {
		t.Errorf("Expected forceRefreshMsg, got %T", result)
	}
}

// Test handleKey with help toggle
func TestHandleKeyHelp(t *testing.T) {
	opts := Options{
		Config: config.Config{
			HomeDir:   "/tmp/test",
			RPCLocal:  "http://localhost:26657",
		},
		RefreshInterval: 1 * time.Second,
		CLIVersion:      "1.0.0",
		NoEmoji:         true,
	}

	dashboard := New(opts)

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}}
	_, cmd := dashboard.handleKey(msg)

	if cmd == nil {
		t.Error("Help toggle should return a command")
	}

	// Execute command and verify it returns toggleHelpMsg
	result := cmd()
	if _, ok := result.(toggleHelpMsg); !ok {
		t.Errorf("Expected toggleHelpMsg, got %T", result)
	}
}

// Test handleKey when help is showing
func TestHandleKeyHelpShowing(t *testing.T) {
	opts := Options{
		Config: config.Config{
			HomeDir:   "/tmp/test",
			RPCLocal:  "http://localhost:26657",
		},
		RefreshInterval: 1 * time.Second,
		CLIVersion:      "1.0.0",
		NoEmoji:         true,
	}

	dashboard := New(opts)
	dashboard.showHelp = true

	// Press 'q' to close help
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	_, cmd := dashboard.handleKey(msg)

	if cmd == nil {
		t.Error("Should return command to close help")
	}

	// Execute command
	result := cmd()
	if _, ok := result.(toggleHelpMsg); !ok {
		t.Errorf("Expected toggleHelpMsg, got %T", result)
	}
}

// Test Update with WindowSizeMsg
func TestDashboardUpdateWindowSize(t *testing.T) {
	opts := Options{
		Config: config.Config{
			HomeDir:   "/tmp/test",
			RPCLocal:  "http://localhost:26657",
		},
		RefreshInterval: 1 * time.Second,
		CLIVersion:      "1.0.0",
		NoEmoji:         true,
	}

	dashboard := New(opts)

	msg := tea.WindowSizeMsg{Width: 120, Height: 40}
	model, _ := dashboard.Update(msg)

	d := model.(*Dashboard)
	if d.width != 120 {
		t.Errorf("Width should be 120, got %d", d.width)
	}
	if d.height != 40 {
		t.Errorf("Height should be 40, got %d", d.height)
	}
}

// Test Update with toggleHelpMsg
func TestDashboardUpdateToggleHelp(t *testing.T) {
	opts := Options{
		Config: config.Config{
			HomeDir:   "/tmp/test",
			RPCLocal:  "http://localhost:26657",
		},
		RefreshInterval: 1 * time.Second,
		CLIVersion:      "1.0.0",
		NoEmoji:         true,
	}

	dashboard := New(opts)

	initialShowHelp := dashboard.showHelp

	msg := toggleHelpMsg{}
	model, _ := dashboard.Update(msg)

	d := model.(*Dashboard)
	if d.showHelp == initialShowHelp {
		t.Error("showHelp should be toggled")
	}
}

// Test RenderStatic
func TestRenderStatic(t *testing.T) {
	opts := Options{
		Config: config.Config{
			HomeDir:   "/tmp/test",
			RPCLocal:  "http://localhost:26657",
		},
		RefreshInterval: 1 * time.Second,
		CLIVersion:      "1.0.0",
		NoEmoji:         true,
	}

	dashboard := New(opts)
	data := createTestData()

	rendered := dashboard.RenderStatic(data)

	if rendered == "" {
		t.Error("RenderStatic returned empty string")
	}

	// Check for expected content
	expectedContent := []string{
		"PUSH VALIDATOR STATUS",
		"NODE STATUS",
		"CHAIN STATUS",
		"NETWORK STATUS",
	}

	for _, content := range expectedContent {
		if !strings.Contains(rendered, content) {
			t.Errorf("RenderStatic should contain %q", content)
		}
	}
}

// Test RenderStatic with stopped node
func TestRenderStaticStoppedNode(t *testing.T) {
	opts := Options{
		Config: config.Config{
			HomeDir:   "/tmp/test",
			RPCLocal:  "http://localhost:26657",
		},
		RefreshInterval: 1 * time.Second,
		CLIVersion:      "1.0.0",
		NoEmoji:         true,
	}

	dashboard := New(opts)
	data := createTestData()
	data.NodeInfo.Running = false

	rendered := dashboard.RenderStatic(data)

	if !strings.Contains(rendered, "Stopped") {
		t.Error("RenderStatic should show Stopped status")
	}
}

// Test getCachedVersion when node is stopped - should still attempt version fetch
func TestGetCachedVersionStopped(t *testing.T) {
	opts := Options{
		Config: config.Config{
			HomeDir:  "/tmp/test",
			RPCLocal: "http://localhost:26657",
		},
		RefreshInterval: 1 * time.Second,
		CLIVersion:      "1.0.0",
		NoEmoji:         true,
	}

	dashboard := New(opts)
	ctx := context.Background()

	// Even when stopped, version fetch is attempted (binary exists on disk)
	version := dashboard.getCachedVersion(ctx, false, 0)

	// Without a valid BinPath, falls back to "pchaind" which won't be in PATH during tests
	if version != "pchaind not found" {
		t.Errorf("Version for stopped node without BinPath should be 'pchaind not found', got %q", version)
	}
}

// Test getCachedVersion with PID change
func TestGetCachedVersionPIDChange(t *testing.T) {
	opts := Options{
		Config: config.Config{
			HomeDir:   "/tmp/test",
			RPCLocal:  "http://localhost:26657",
		},
		RefreshInterval: 1 * time.Second,
		CLIVersion:      "1.0.0",
		NoEmoji:         true,
		Debug:           true,
	}

	dashboard := New(opts)
	ctx := context.Background()

	// Set initial cached version
	dashboard.cachedVersion = "v1.0.0"
	dashboard.cachedVersionPID = 1234
	dashboard.cachedVersionAt = time.Now()

	// Call with different PID (simulating restart)
	dashboard.getCachedVersion(ctx, true, 5678)

	// Cache should be invalidated
	if dashboard.cachedVersionPID != 5678 {
		t.Errorf("PID should be updated to 5678, got %d", dashboard.cachedVersionPID)
	}
}

// Test Update with dataMsg
func TestDashboardUpdateDataMsg(t *testing.T) {
	opts := Options{
		Config: config.Config{
			HomeDir:   "/tmp/test",
			RPCLocal:  "http://localhost:26657",
		},
		RefreshInterval: 1 * time.Second,
		CLIVersion:      "1.0.0",
		NoEmoji:         true,
	}

	dashboard := New(opts)
	dashboard.loading = true

	data := createTestData()
	msg := dataMsg(data)

	model, _ := dashboard.Update(msg)
	d := model.(*Dashboard)

	if d.loading {
		t.Error("Loading should be false after dataMsg")
	}

	if d.err != nil {
		t.Error("Error should be nil after successful dataMsg")
	}

	if d.stale {
		t.Error("Stale should be false after dataMsg")
	}
}

// Test Update with dataErrMsg
func TestDashboardUpdateDataErrMsg(t *testing.T) {
	opts := Options{
		Config: config.Config{
			HomeDir:   "/tmp/test",
			RPCLocal:  "http://localhost:26657",
		},
		RefreshInterval: 1 * time.Second,
		CLIVersion:      "1.0.0",
		NoEmoji:         true,
	}

	dashboard := New(opts)
	dashboard.loading = true
	dashboard.lastOK = time.Now().Add(-11 * time.Second)

	msg := dataErrMsg{err: &testError{msg: "test error"}}

	model, _ := dashboard.Update(msg)
	d := model.(*Dashboard)

	if d.loading {
		t.Error("Loading should be false after dataErrMsg")
	}

	if d.err == nil {
		t.Error("Error should be set after dataErrMsg")
	}

	if !d.stale {
		t.Error("Stale should be true after long time without success")
	}
}

// Test validators_list getSortedValidators
func TestGetSortedValidators(t *testing.T) {
	cfg := config.Config{
		HomeDir:  "/tmp/test",
		RPCLocal: "http://localhost:26657",
	}

	comp := NewValidatorsList(true, cfg)
	data := createTestData()

	// Create test validators with different statuses and voting power
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
		{Moniker: "val1", Status: "BONDED", VotingPower: 1000, Address: "addr1"},
		{Moniker: "val2", Status: "UNBONDING", VotingPower: 2000, Address: "addr2"},
		{Moniker: "val3", Status: "BONDED", VotingPower: 3000, Address: "addr3"},
		{Moniker: "my-val", Status: "BONDED", VotingPower: 500, Address: "pushvaloper1abc123"},
	}
	data.MyValidator.Address = "pushvaloper1abc123"

	updated, _ := comp.Update(tea.Msg(nil), data)
	comp = updated.(*ValidatorsList)

	sorted := comp.getSortedValidators()

	// My validator should be first
	if sorted[0].Moniker != "my-val" {
		t.Errorf("My validator should be first, got %s", sorted[0].Moniker)
	}

	// BONDED validators should come before UNBONDING
	for i, v := range sorted {
		if v.Status == "UNBONDING" {
			// Check that all previous validators are BONDED or our validator
			for j := 0; j < i; j++ {
				if sorted[j].Address != "pushvaloper1abc123" && sorted[j].Status != "BONDED" {
					t.Errorf("BONDED validators should come before UNBONDING")
				}
			}
			break
		}
	}
}

// Test validators_list handleKey with pagination
func TestValidatorsListHandleKeyPagination(t *testing.T) {
	cfg := config.Config{
		HomeDir:  "/tmp/test",
		RPCLocal: "http://localhost:26657",
	}

	comp := NewValidatorsList(true, cfg)
	data := createTestData()

	// Create enough validators for multiple pages (pageSize is 5)
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
	}, 12)

	for i := 0; i < 12; i++ {
		validators[i] = struct {
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
			Moniker:     "val" + string(rune('A'+i)),
			Status:      "BONDED",
			VotingPower: int64(1000 * (12 - i)),
			Commission:  "10.00",
			Address:     "addr" + string(rune('A'+i)),
		}
	}

	data.NetworkValidators.Total = 12
	data.NetworkValidators.Validators = validators

	updated, _ := comp.Update(tea.Msg(nil), data)
	comp = updated.(*ValidatorsList)

	// Initially on page 0
	if comp.currentPage != 0 {
		t.Errorf("Initial page should be 0, got %d", comp.currentPage)
	}

	// Press 'n' for next page
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}
	updated, _ = comp.handleKey(msg)
	comp = updated.(*ValidatorsList)

	if comp.currentPage != 1 {
		t.Errorf("After next, page should be 1, got %d", comp.currentPage)
	}

	// Press 'p' for previous page
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}
	updated, _ = comp.handleKey(msg)
	comp = updated.(*ValidatorsList)

	if comp.currentPage != 0 {
		t.Errorf("After previous, page should be 0, got %d", comp.currentPage)
	}

	// Press 'p' again (should stay at 0)
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}
	updated, _ = comp.handleKey(msg)
	comp = updated.(*ValidatorsList)

	if comp.currentPage != 0 {
		t.Errorf("Should not go below page 0, got %d", comp.currentPage)
	}
}

// Test log_viewer styleLogLine
func TestStyleLogLine(t *testing.T) {
	lv := NewLogViewer(true, "/tmp/test/logs/pchaind.log")
	defer lv.Close()

	tests := []struct {
		name     string
		line     string
		noEmoji  bool
		validate func(t *testing.T, result string)
	}{
		{
			name:    "error line",
			line:    "2024-01-01 ERROR: something went wrong",
			noEmoji: true,
			validate: func(t *testing.T, result string) {
				if result == "" {
					t.Error("Result should not be empty")
				}
			},
		},
		{
			name:    "warning line",
			line:    "2024-01-01 WARN: potential issue",
			noEmoji: false,
			validate: func(t *testing.T, result string) {
				if result == "" {
					t.Error("Result should not be empty")
				}
			},
		},
		{
			name:    "info line",
			line:    "2024-01-01 INFO: normal operation",
			noEmoji: false,
			validate: func(t *testing.T, result string) {
				if result == "" {
					t.Error("Result should not be empty")
				}
			},
		},
		{
			name:    "debug line",
			line:    "2024-01-01 DEBUG: detailed trace",
			noEmoji: false,
			validate: func(t *testing.T, result string) {
				if result == "" {
					t.Error("Result should not be empty")
				}
			},
		},
		{
			name:    "regular line no emoji",
			line:    "regular log line",
			noEmoji: true,
			validate: func(t *testing.T, result string) {
				if result != "regular log line" {
					t.Errorf("Expected unchanged line, got %q", result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lv.noEmoji = tt.noEmoji
			result := lv.styleLogLine(tt.line, 100)
			tt.validate(t, result)
		})
	}
}

// Test log_viewer renderFooter
func TestRenderFooter(t *testing.T) {
	lv := NewLogViewer(true, "/tmp/test/logs/pchaind.log")
	defer lv.Close()

	// Test follow mode footer
	lv.followMode = true
	lv.searchMode = false
	footer := lv.renderFooter()

	if footer == "" {
		t.Error("Footer should not be empty")
	}

	if !strings.Contains(footer, "scroll") {
		t.Error("Footer should contain scroll hint")
	}

	// Test search mode footer
	lv.searchMode = true
	footer = lv.renderFooter()

	if !strings.Contains(footer, "Enter") && !strings.Contains(footer, "Esc") {
		t.Error("Search mode footer should show Enter and Esc hints")
	}

	// Test paused mode footer
	lv.searchMode = false
	lv.followMode = false
	footer = lv.renderFooter()

	if !strings.Contains(footer, "live") {
		t.Error("Paused mode footer should show 'live' option")
	}
}

// Test log_viewer Title
func TestLogViewerTitle(t *testing.T) {
	lv := NewLogViewer(true, "/tmp/test/logs/pchaind.log")
	defer lv.Close()

	// Test normal mode
	lv.searchMode = false
	lv.followMode = true
	title := lv.Title()

	if title == "" {
		t.Error("Title should not be empty")
	}

	// Test search mode
	lv.searchMode = true
	lv.searchTerm = "test"
	title = lv.Title()

	if !strings.Contains(title, "Search") && !strings.Contains(title, "test") {
		t.Error("Title should show search mode and term")
	}

	// Test paused mode
	lv.searchMode = false
	lv.followMode = false
	title = lv.Title()

	if !strings.Contains(title, "Paused") {
		t.Error("Title should show paused state")
	}
}

// Test log_viewer handleKey
func TestLogViewerHandleKey(t *testing.T) {
	lv := NewLogViewer(true, "/tmp/test/logs/pchaind.log")
	defer lv.Close()

	// Add some test data to buffer
	lv.buffer.Add("line1")
	lv.buffer.Add("line2")
	lv.buffer.Add("line3")

	// Test follow mode toggle
	lv.followMode = true
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}}
	updated, _ := lv.handleKey(msg)
	lv = updated.(*LogViewer)

	if lv.followMode {
		t.Error("Follow mode should be toggled off")
	}

	// Test up key (scroll up)
	lv.followMode = true
	lv.scrollPos = 0
	msg = tea.KeyMsg{Type: tea.KeyUp}
	updated, _ = lv.handleKey(msg)
	lv = updated.(*LogViewer)

	if lv.scrollPos != 1 {
		t.Errorf("Scroll position should be 1, got %d", lv.scrollPos)
	}

	if lv.followMode {
		t.Error("Up key should disable follow mode")
	}

	// Test down key (scroll down)
	lv.scrollPos = 2
	msg = tea.KeyMsg{Type: tea.KeyDown}
	updated, _ = lv.handleKey(msg)
	lv = updated.(*LogViewer)

	if lv.scrollPos != 1 {
		t.Errorf("Scroll position should decrease to 1, got %d", lv.scrollPos)
	}

	// Test down to bottom (re-enables follow mode)
	lv.scrollPos = 1
	lv.followMode = false
	msg = tea.KeyMsg{Type: tea.KeyDown}
	updated, _ = lv.handleKey(msg)
	lv = updated.(*LogViewer)

	if lv.scrollPos != 0 {
		t.Errorf("Scroll position should be 0, got %d", lv.scrollPos)
	}

	if !lv.followMode {
		t.Error("Scrolling to bottom should re-enable follow mode")
	}

	// Test search mode activation
	lv.searchMode = false
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}
	updated, _ = lv.handleKey(msg)
	lv = updated.(*LogViewer)

	if !lv.searchMode {
		t.Error("/ should activate search mode")
	}

	// Test search term input
	lv.searchTerm = ""
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}}
	updated, _ = lv.handleKey(msg)
	lv = updated.(*LogViewer)

	if lv.searchTerm != "t" {
		t.Errorf("Search term should be 't', got %q", lv.searchTerm)
	}

	// Test backspace in search
	msg = tea.KeyMsg{Type: tea.KeyBackspace}
	updated, _ = lv.handleKey(msg)
	lv = updated.(*LogViewer)

	if lv.searchTerm != "" {
		t.Errorf("Search term should be empty after backspace, got %q", lv.searchTerm)
	}

	// Test escape from search
	lv.searchMode = true
	lv.searchTerm = "test"
	msg = tea.KeyMsg{Type: tea.KeyEsc}
	updated, _ = lv.handleKey(msg)
	lv = updated.(*LogViewer)

	if lv.searchMode {
		t.Error("Escape should exit search mode")
	}

	if lv.searchTerm != "" {
		t.Error("Escape should clear search term")
	}

	// Test enter from search
	lv.searchMode = true
	lv.searchTerm = "test"
	msg = tea.KeyMsg{Type: tea.KeyEnter}
	updated, _ = lv.handleKey(msg)
	lv = updated.(*LogViewer)

	if lv.searchMode {
		t.Error("Enter should exit search mode")
	}

	// Test 't' key (jump to top/oldest)
	lv.searchMode = false
	lv.followMode = true
	lv.scrollPos = 0
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}}
	updated, _ = lv.handleKey(msg)
	lv = updated.(*LogViewer)

	if lv.followMode {
		t.Error("'t' should disable follow mode")
	}

	if lv.scrollPos != lv.buffer.Count() {
		t.Errorf("'t' should set scroll pos to buffer count, got %d", lv.scrollPos)
	}

	// Test 'l' key (jump to latest)
	lv.followMode = false
	lv.scrollPos = 10
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}}
	updated, _ = lv.handleKey(msg)
	lv = updated.(*LogViewer)

	if !lv.followMode {
		t.Error("'l' should enable follow mode")
	}

	if lv.scrollPos != 0 {
		t.Errorf("'l' should reset scroll pos to 0, got %d", lv.scrollPos)
	}
}

// Test log_viewer View and renderContent
func TestLogViewerView(t *testing.T) {
	lv := NewLogViewer(true, "/tmp/test/logs/pchaind.log")
	defer lv.Close()

	// Add test lines
	lv.buffer.Add("line1")
	lv.buffer.Add("line2")
	lv.buffer.Add("line3")

	view := lv.View(80, 20)

	if view == "" {
		t.Error("View should not be empty")
	}

	// The view contains styled/formatted content, so just check it's not empty
	// Title is part of the rendered view with styling
	if len(view) < 10 {
		t.Errorf("View too short, got length %d", len(view))
	}
}

// Test log_viewer MinWidth and MinHeight
func TestLogViewerMinDimensions(t *testing.T) {
	lv := NewLogViewer(true, "/tmp/test/logs/pchaind.log")
	defer lv.Close()

	if lv.MinWidth() != 40 {
		t.Errorf("MinWidth should be 40, got %d", lv.MinWidth())
	}

	if lv.MinHeight() != 13 {
		t.Errorf("MinHeight should be 13, got %d", lv.MinHeight())
	}
}

// Test log_viewer Update method
func TestLogViewerUpdate(t *testing.T) {
	lv := NewLogViewer(true, "/tmp/test/logs/pchaind.log")
	defer lv.Close()

	data := createTestData()

	// Test non-KeyMsg
	updated, cmd := lv.Update(tea.WindowSizeMsg{Width: 100, Height: 50}, data)

	if updated == nil {
		t.Error("Update should return component")
	}

	if cmd != nil {
		t.Error("Update should return nil command for non-KeyMsg")
	}

	// Test KeyMsg
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}}
	updated, _ = lv.Update(msg, data)

	if updated == nil {
		t.Error("Update should return component")
	}
}

// Test validators_list Title variations
func TestValidatorsListTitle(t *testing.T) {
	cfg := config.Config{
		HomeDir:  "/tmp/test",
		RPCLocal: "http://localhost:26657",
	}

	comp := NewValidatorsList(true, cfg)

	// Test with no validators
	title := comp.Title()
	if title != "Network Validators" {
		t.Errorf("Expected 'Network Validators', got %q", title)
	}

	// Test with single page
	data := createTestData()
	data.NetworkValidators.Total = 3
	data.NetworkValidators.Validators = make([]struct {
		Moniker              string
		Status               string
		VotingPower          int64
		Commission           string
		CommissionRewards    string
		OutstandingRewards   string
		Address              string
		EVMAddress           string
		Jailed               bool
	}, 3)

	updated, _ := comp.Update(tea.Msg(nil), data)
	comp = updated.(*ValidatorsList)

	title = comp.Title()
	if title != "Network Validators" {
		t.Errorf("Expected 'Network Validators' for single page, got %q", title)
	}

	// Test with multiple pages
	data.NetworkValidators.Total = 12
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
	}, 12)
	data.NetworkValidators.Validators = validators

	updated, _ = comp.Update(tea.Msg(nil), data)
	comp = updated.(*ValidatorsList)

	title = comp.Title()
	if !strings.Contains(title, "Page") {
		t.Errorf("Title should contain page number for multiple pages, got %q", title)
	}
}

// Test validators_list handleKey with EVM toggle
func TestValidatorsListHandleKeyEVMToggle(t *testing.T) {
	cfg := config.Config{
		HomeDir:  "/tmp/test",
		RPCLocal: "http://localhost:26657",
	}

	comp := NewValidatorsList(true, cfg)

	// Set initial state
	comp.showEVMAddress = true

	// Press 'e' to toggle
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}}
	updated, _ := comp.handleKey(msg)
	comp = updated.(*ValidatorsList)

	if comp.showEVMAddress {
		t.Error("EVM address display should be toggled off")
	}

	// Toggle back
	updated, _ = comp.handleKey(msg)
	comp = updated.(*ValidatorsList)

	if !comp.showEVMAddress {
		t.Error("EVM address display should be toggled back on")
	}
}

// Test validators_list getEVMAddressFromCache
func TestGetEVMAddressFromCache(t *testing.T) {
	cfg := config.Config{
		HomeDir:  "/tmp/test",
		RPCLocal: "http://localhost:26657",
	}

	comp := NewValidatorsList(true, cfg)

	// Test empty cache
	addr := comp.getEVMAddressFromCache("test-address")
	if addr != "" {
		t.Error("Should return empty string for uncached address")
	}

	// Add to cache
	comp.evmAddressCache["test-address"] = "0x1234567890abcdef"

	// Test cached address
	addr = comp.getEVMAddressFromCache("test-address")
	if addr != "0x1234567890abcdef" {
		t.Errorf("Expected cached address, got %q", addr)
	}
}

// Test Dashboard Update with tickMsg
func TestDashboardUpdateTickMsg(t *testing.T) {
	opts := Options{
		Config: config.Config{
			HomeDir:  "/tmp/test",
			RPCLocal: "http://localhost:26657",
		},
		RefreshInterval: 1 * time.Second,
		CLIVersion:      "1.0.0",
		NoEmoji:         true,
	}

	dashboard := New(opts)

	msg := tickMsg(time.Now())
	model, cmd := dashboard.Update(msg)

	if model == nil {
		t.Error("Update should return model")
	}

	if cmd == nil {
		t.Error("Update with tickMsg should return command")
	}
}

// Test Dashboard Update with forceRefreshMsg
func TestDashboardUpdateForceRefresh(t *testing.T) {
	opts := Options{
		Config: config.Config{
			HomeDir:  "/tmp/test",
			RPCLocal: "http://localhost:26657",
		},
		RefreshInterval: 1 * time.Second,
		CLIVersion:      "1.0.0",
		NoEmoji:         true,
	}

	dashboard := New(opts)

	msg := forceRefreshMsg{}
	model, cmd := dashboard.Update(msg)

	if model == nil {
		t.Error("Update should return model")
	}

	if cmd == nil {
		t.Error("Update with forceRefreshMsg should return command")
	}
}

// Test Dashboard handleKey with arrow keys
func TestHandleKeyArrowKeys(t *testing.T) {
	opts := Options{
		Config: config.Config{
			HomeDir:  "/tmp/test",
			RPCLocal: "http://localhost:26657",
		},
		RefreshInterval: 1 * time.Second,
		CLIVersion:      "1.0.0",
		NoEmoji:         true,
	}

	dashboard := New(opts)

	// Test up key
	msg := tea.KeyMsg{Type: tea.KeyUp}
	model, _ := dashboard.handleKey(msg)

	if model == nil {
		t.Error("handleKey should return model")
	}

	// Test down key
	msg = tea.KeyMsg{Type: tea.KeyDown}
	model, _ = dashboard.handleKey(msg)

	if model == nil {
		t.Error("handleKey should return model")
	}

	// Test left key
	msg = tea.KeyMsg{Type: tea.KeyLeft}
	model, _ = dashboard.handleKey(msg)

	if model == nil {
		t.Error("handleKey should return model")
	}

	// Test right key
	msg = tea.KeyMsg{Type: tea.KeyRight}
	model, _ = dashboard.handleKey(msg)

	if model == nil {
		t.Error("handleKey should return model")
	}
}

// Test Dashboard View with help overlay
func TestDashboardViewHelp(t *testing.T) {
	opts := Options{
		Config: config.Config{
			HomeDir:  "/tmp/test",
			RPCLocal: "http://localhost:26657",
		},
		RefreshInterval: 1 * time.Second,
		CLIVersion:      "1.0.0",
		NoEmoji:         true,
	}

	dashboard := New(opts)
	dashboard.width = 120
	dashboard.height = 40
	dashboard.loading = false
	dashboard.showHelp = true

	view := dashboard.View()

	if !strings.Contains(view, "Push Validator Manager") {
		t.Error("Help view should contain title")
	}

	if !strings.Contains(view, "USAGE") {
		t.Error("Help view should contain USAGE section")
	}
}

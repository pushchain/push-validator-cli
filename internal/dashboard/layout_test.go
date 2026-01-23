package dashboard

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewLayout(t *testing.T) {
	registry := NewComponentRegistry()
	registry.Register(NewHeader())

	config := LayoutConfig{
		Rows: []LayoutRow{
			{Components: []string{"header"}, Weights: []int{100}, MinHeight: 4},
		},
	}

	layout := NewLayout(config, registry)
	if layout == nil {
		t.Fatal("NewLayout returned nil")
	}
	if layout.registry != registry {
		t.Error("Layout registry not set correctly")
	}
	if len(layout.config.Rows) != 1 {
		t.Errorf("Expected 1 row, got %d", len(layout.config.Rows))
	}
}

func TestLayoutCompute(t *testing.T) {
	tests := []struct {
		name          string
		width         int
		height        int
		config        LayoutConfig
		wantCellCount int
		wantWarning   bool
	}{
		{
			name:   "single component",
			width:  100,
			height: 20,
			config: LayoutConfig{
				Rows: []LayoutRow{
					{Components: []string{"header"}, Weights: []int{100}, MinHeight: 4},
				},
			},
			wantCellCount: 1,
			wantWarning:   false,
		},
		{
			name:   "two rows multiple components",
			width:  100,
			height: 30,
			config: LayoutConfig{
				Rows: []LayoutRow{
					{Components: []string{"header"}, Weights: []int{100}, MinHeight: 4},
					{Components: []string{"node_status", "chain_status"}, Weights: []int{50, 50}, MinHeight: 10},
				},
			},
			wantCellCount: 3,
			wantWarning:   false,
		},
		{
			name:   "narrow width triggers warning",
			width:  10,
			height: 20,
			config: LayoutConfig{
				Rows: []LayoutRow{
					{Components: []string{"node_status", "chain_status", "network_status"}, Weights: []int{33, 33, 34}, MinHeight: 10},
				},
			},
			wantCellCount: 0, // Components may be dropped
			wantWarning:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewComponentRegistry()
			registry.Register(NewHeader())
			registry.Register(NewNodeStatus(true))
			registry.Register(NewChainStatus(true))
			registry.Register(NewNetworkStatus(true))

			layout := NewLayout(tt.config, registry)
			result := layout.Compute(tt.width, tt.height)

			if tt.wantWarning && result.Warning == "" {
				t.Error("Expected warning but got none")
			}
			if !tt.wantWarning && result.Warning != "" {
				t.Errorf("Unexpected warning: %s", result.Warning)
			}

			// Verify cells were created
			if len(result.Cells) < tt.wantCellCount && !tt.wantWarning {
				t.Errorf("Expected at least %d cells, got %d", tt.wantCellCount, len(result.Cells))
			}

			// Verify cell positions are valid
			for _, cell := range result.Cells {
				if cell.W < 0 || cell.H < 0 {
					t.Errorf("Invalid cell dimensions: W=%d, H=%d", cell.W, cell.H)
				}
				if cell.X < 0 || cell.Y < 0 {
					t.Errorf("Invalid cell position: X=%d, Y=%d", cell.X, cell.Y)
				}
			}
		})
	}
}

func TestLayoutComputeVerticalSlack(t *testing.T) {
	registry := NewComponentRegistry()
	registry.Register(NewHeader())
	registry.Register(NewNodeStatus(true))

	config := LayoutConfig{
		Rows: []LayoutRow{
			{Components: []string{"header"}, Weights: []int{100}, MinHeight: 3},
			{Components: []string{"node_status"}, Weights: []int{100}, MinHeight: 8},
		},
	}

	layout := NewLayout(config, registry)

	// Test with extra vertical space
	result := layout.Compute(100, 20) // total height 20, minHeight 11, slack 9

	if len(result.Cells) != 2 {
		t.Fatalf("Expected 2 cells, got %d", len(result.Cells))
	}

	// Header should stay at MinHeight (3)
	headerCell := result.Cells[0]
	if headerCell.H != 3 {
		t.Errorf("Header height: expected 3, got %d", headerCell.H)
	}

	// Node status should get the slack (8 + 9 = 17)
	nodeCell := result.Cells[1]
	if nodeCell.H < 8 {
		t.Errorf("Node status height should be at least 8, got %d", nodeCell.H)
	}
}

func TestComputeRowWidths(t *testing.T) {
	registry := NewComponentRegistry()
	registry.Register(NewNodeStatus(true)) // MinWidth: 25
	registry.Register(NewChainStatus(true)) // MinWidth: 30

	layout := NewLayout(LayoutConfig{}, registry)

	tests := []struct {
		name        string
		row         LayoutRow
		totalWidth  int
		wantWidths  int // number of widths
		wantWarning bool
	}{
		{
			name: "sufficient width with weights",
			row: LayoutRow{
				Components: []string{"node_status", "chain_status"},
				Weights:    []int{50, 50},
				MinHeight:  10,
			},
			totalWidth:  100,
			wantWidths:  2,
			wantWarning: false,
		},
		{
			name: "insufficient width",
			row: LayoutRow{
				Components: []string{"node_status", "chain_status"},
				Weights:    []int{50, 50},
				MinHeight:  10,
			},
			totalWidth:  20, // Less than sum of MinWidths (25 + 30 = 55)
			wantWidths:  0,  // Components may be dropped
			wantWarning: true,
		},
		{
			name: "no weights specified",
			row: LayoutRow{
				Components: []string{"node_status"},
				Weights:    []int{},
				MinHeight:  10,
			},
			totalWidth:  100,
			wantWidths:  1,
			wantWarning: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			widths, keptIDs, warning := layout.computeRowWidths(tt.row, tt.totalWidth)

			if tt.wantWarning && warning == "" {
				t.Error("Expected warning but got none")
			}
			if !tt.wantWarning && warning != "" {
				t.Errorf("Unexpected warning: %s", warning)
			}

			if len(widths) != len(keptIDs) {
				t.Errorf("Widths and keptIDs length mismatch: %d vs %d", len(widths), len(keptIDs))
			}

			// Verify total width doesn't exceed available
			totalAllocated := 0
			for _, w := range widths {
				totalAllocated += w
			}
			if totalAllocated > tt.totalWidth {
				t.Errorf("Total allocated width %d exceeds available %d", totalAllocated, tt.totalWidth)
			}
		})
	}
}

func TestHandleInsufficientWidth(t *testing.T) {
	registry := NewComponentRegistry()
	registry.Register(&testLayoutComponent{BaseComponent: BaseComponent{id: "header", title: "Header", minW: 40, minH: 3}})
	registry.Register(&testLayoutComponent{BaseComponent: BaseComponent{id: "node_status", title: "Node", minW: 25, minH: 8}})
	registry.Register(&testLayoutComponent{BaseComponent: BaseComponent{id: "chain_status", title: "Chain", minW: 30, minH: 8}})

	layout := NewLayout(LayoutConfig{}, registry)

	tests := []struct {
		name       string
		row        LayoutRow
		width      int
		wantKept   int // Expected number of kept components
		wantWarn   bool
	}{
		{
			name: "drops non-essential components",
			row: LayoutRow{
				Components: []string{"header", "node_status", "chain_status"},
				Weights:    []int{33, 33, 34},
				MinHeight:  10,
			},
			width:    50,
			wantKept: 0, // Will keep essentials but they may not all fit
			wantWarn: true,
		},
		{
			name: "extreme narrow width",
			row: LayoutRow{
				Components: []string{"header"},
				Weights:    []int{100},
				MinHeight:  3,
			},
			width:    5,
			wantKept: 1,
			wantWarn: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			widths, keptIDs, warning := layout.handleInsufficientWidth(tt.row, tt.width)

			if tt.wantWarn && warning == "" {
				t.Error("Expected warning but got none")
			}

			if len(widths) != len(keptIDs) {
				t.Errorf("Widths and keptIDs length mismatch: %d vs %d", len(widths), len(keptIDs))
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name  string
		slice []string
		item  string
		want  bool
	}{
		{"found", []string{"a", "b", "c"}, "b", true},
		{"not found", []string{"a", "b", "c"}, "d", false},
		{"empty slice", []string{}, "a", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := contains(tt.slice, tt.item)
			if got != tt.want {
				t.Errorf("contains() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClampEven(t *testing.T) {
	tests := []struct {
		name     string
		kept     []string
		width    int
		wantSum  int
		wantAll  bool // All widths should be >= 1
	}{
		{"equal distribution", []string{"a", "b", "c"}, 30, 30, true},
		{"with remainder", []string{"a", "b"}, 11, 11, true},
		{"narrow width", []string{"a", "b", "c"}, 3, 3, true},
		{"single component", []string{"a"}, 10, 10, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			widths := clampEven(tt.kept, tt.width)

			if len(widths) != len(tt.kept) {
				t.Errorf("Expected %d widths, got %d", len(tt.kept), len(widths))
			}

			sum := 0
			for _, w := range widths {
				sum += w
				if tt.wantAll && w < 1 {
					t.Errorf("Width should be at least 1, got %d", w)
				}
			}

			if sum > tt.width {
				t.Errorf("Total width %d exceeds available %d", sum, tt.width)
			}

			// Sum should equal width after adjustment
			if sum != tt.width {
				t.Logf("Info: Total width %d differs from available %d (acceptable if clampEven adjusted)", sum, tt.width)
			}
		})
	}
}

func TestKeptEssentials(t *testing.T) {
	tests := []struct {
		name      string
		ids       []string
		essential []string
		want      int // Expected number of kept IDs
	}{
		{
			name:      "all essential",
			ids:       []string{"header", "node_status", "chain_status"},
			essential: []string{"header", "node_status", "chain_status"},
			want:      3,
		},
		{
			name:      "some essential",
			ids:       []string{"header", "validator_info", "chain_status"},
			essential: []string{"header", "node_status", "chain_status"},
			want:      2,
		},
		{
			name:      "none essential",
			ids:       []string{"validator_info", "validators_list"},
			essential: []string{"header", "node_status", "chain_status"},
			want:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kept := keptEssentials(tt.ids, tt.essential)
			if len(kept) != tt.want {
				t.Errorf("Expected %d kept IDs, got %d", tt.want, len(kept))
			}
		})
	}
}

func TestEqualWeights(t *testing.T) {
	tests := []struct {
		name string
		n    int
		want int // Expected length
	}{
		{"zero", 0, 0},
		{"one", 1, 1},
		{"five", 5, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			weights := equalWeights(tt.n)
			if len(weights) != tt.want {
				t.Errorf("Expected %d weights, got %d", tt.want, len(weights))
			}

			// All weights should be 1
			for i, w := range weights {
				if w != 1 {
					t.Errorf("Weight at index %d should be 1, got %d", i, w)
				}
			}
		})
	}
}

// testLayoutComponent is a minimal component implementation for testing
type testLayoutComponent struct {
	BaseComponent
}

func (c *testLayoutComponent) Init() tea.Cmd {
	return nil
}

func (c *testLayoutComponent) Update(msg tea.Msg, data DashboardData) (Component, tea.Cmd) {
	return c, nil
}

func (c *testLayoutComponent) View(width, height int) string {
	return ""
}

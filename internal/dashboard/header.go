package dashboard

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Header component shows dashboard title, timestamp, and status
type Header struct {
	BaseComponent
	data DashboardData // Dashboard data with error info
}

// NewHeader creates a new header component
func NewHeader() *Header {
	return &Header{
		BaseComponent: BaseComponent{},
	}
}

// ID returns component identifier
func (c *Header) ID() string {
	return "header"
}

// Title returns component title
func (c *Header) Title() string {
	return "PUSH VALIDATOR DASHBOARD"
}

// MinWidth returns minimum width
func (c *Header) MinWidth() int {
	return 40
}

// MinHeight returns minimum height
func (c *Header) MinHeight() int {
	return 3
}

// Update receives dashboard data and updates internal state
func (c *Header) Update(msg tea.Msg, data DashboardData) (Component, tea.Cmd) {
	// Store entire data for access in View
	c.data = data
	return c, nil
}

// View renders the header matching canonical signature View(width, height int)
func (c *Header) View(w, h int) string {
	// Guard against invalid dimensions
	if w <= 0 || h <= 0 {
		return ""
	}

	// Build plain text content
	// Calculate interior width for centering
	inner := w - 4 // Account for border (2) + padding (2)
	if inner < 0 {
		inner = 0
	}

	// Apply bold + cyan highlighting to title
	titleStyled := FormatTitle(c.Title(), inner)

	// Add version on a new line (more visible cyan color)
	var versionLine string
	if c.data.CLIVersion != "" {
		versionStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("81")). // Bright cyan - more visible
			Bold(false)
		versionLine = versionStyle.Render("v" + strings.TrimPrefix(c.data.CLIVersion, "v"))
	}

	// Title line is just the styled title (centered)
	titleLine := titleStyled

	var lines []string
	lines = append(lines, titleLine)

	// Add version line (centered below title)
	if versionLine != "" {
		lines = append(lines, versionLine)
	}

	// Add update notification if available (on its own line, more prominent)
	if c.data.UpdateInfo.Available && c.data.UpdateInfo.LatestVersion != "" {
		updateStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("226")). // Yellow/gold
			Bold(true)
		updateLine := updateStyle.Render(fmt.Sprintf("⬆ Update v%s available - run: push-validator update", c.data.UpdateInfo.LatestVersion))
		lines = append(lines, updateLine)
	}

	if c.data.Err != nil {
		errLine := fmt.Sprintf("⚠ %s", c.data.Err.Error())
		lines = append(lines, errLine)
	}

	content := strings.Join(lines, "\n")

	// Match the exact styling pattern of data components for full compatibility
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(0, 1).
		Align(lipgloss.Center)

	// Account for border width (2 chars: left + right) to prevent overflow
	borderWidth := 2
	contentWidth := w - borderWidth
	if contentWidth < 0 {
		contentWidth = 0
	}

	return style.Width(contentWidth).Render(content)
}

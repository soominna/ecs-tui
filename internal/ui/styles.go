package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	colorPrimary   = lipgloss.Color("#7D56F4")
	colorSecondary = lipgloss.Color("#3C3C3C")
	colorAccent    = lipgloss.Color("#04B575")
	colorError     = lipgloss.Color("#FF4444")
	colorWarning   = lipgloss.Color("#FFAA00")
	colorMuted     = lipgloss.Color("#555555")
	colorWhite     = lipgloss.Color("#FFFFFF")
	colorDimText   = lipgloss.Color("#888888")

	// Header — full-width bar with background fill (clearly non-interactive)
	headerStyle = lipgloss.NewStyle().
			Background(colorPrimary).
			Foreground(colorWhite).
			Bold(true).
			Padding(0, 1)

	// Footer
	footerStyle = lipgloss.NewStyle().
			Background(colorSecondary).
			Foreground(colorWhite).
			Padding(0, 1)

	shortcutKeyStyle = lipgloss.NewStyle().
				Foreground(colorAccent).
				Bold(true)

	shortcutDescStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#AAAAAA"))

	// Status styles
	errorStyle = lipgloss.NewStyle().
			Foreground(colorError).
			Bold(true)

	statusStyle = lipgloss.NewStyle().
			Foreground(colorAccent)

	// Table status colors
	StatusRunning = lipgloss.NewStyle().Foreground(colorAccent)
	StatusStopped = lipgloss.NewStyle().Foreground(colorError)
	StatusPending = lipgloss.NewStyle().Foreground(colorWarning)
	StatusActive  = lipgloss.NewStyle().Foreground(colorAccent)

	// List/section title — dim, non-interactive, clearly a label
	// Uses muted color to distinguish from selectable items
	titleStyle = lipgloss.NewStyle().
			Foreground(colorDimText).
			Bold(true)

	// Section header in detail/content views — with separator line
	sectionTitleStyle = lipgloss.NewStyle().
				Foreground(colorDimText).
				Bold(true).
				BorderStyle(lipgloss.NormalBorder()).
				BorderBottom(true).
				BorderForeground(lipgloss.Color("#444444")).
				Padding(0, 0, 0, 1).
				MarginBottom(1)

	// Breadcrumb navigation
	breadcrumbStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	breadcrumbActiveStyle = lipgloss.NewStyle().
				Foreground(colorWhite).
				Bold(true)

	breadcrumbSepStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#444444"))

	// Loading
	loadingStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Italic(true)
)

// responsiveColumn defines a table column with flexible width.
type responsiveColumn struct {
	Title    string
	MinWidth int // minimum width
	Flex     int // flex weight (0 = fixed at MinWidth)
}

// calcColumnWidths distributes available width across columns.
// Fixed columns (Flex=0) get their MinWidth. Remaining space is distributed
// proportionally to flex columns based on their Flex weight.
func calcColumnWidths(cols []responsiveColumn, totalWidth int) []int {
	// Account for table borders/padding (~2 per column + 1)
	available := totalWidth - len(cols)*2 - 1
	if available < 0 {
		available = 0
	}

	widths := make([]int, len(cols))
	totalFlex := 0
	fixedUsed := 0

	for i, c := range cols {
		widths[i] = c.MinWidth
		fixedUsed += c.MinWidth
		totalFlex += c.Flex
	}

	extra := available - fixedUsed
	if extra > 0 && totalFlex > 0 {
		for i, c := range cols {
			if c.Flex > 0 {
				widths[i] += extra * c.Flex / totalFlex
			}
		}
	}

	return widths
}

// RenderShortcutBar renders the footer shortcut bar.
func RenderShortcutBar(shortcuts []Shortcut, width int) string {
	var parts []string
	for _, s := range shortcuts {
		parts = append(parts,
			shortcutKeyStyle.Render("<"+s.Key+">")+
				" "+
				shortcutDescStyle.Render(s.Desc))
	}
	bar := strings.Join(parts, "  ")
	return footerStyle.Width(width).Render(bar)
}

// RenderHeader renders the header bar.
func RenderHeader(content string, width int) string {
	return headerStyle.Width(width).Render(content)
}

// StatusColor returns styled text based on status.
func StatusColor(status string) string {
	switch strings.ToUpper(status) {
	case "RUNNING", "ACTIVE":
		return StatusRunning.Render(status)
	case "STOPPED", "INACTIVE", "DRAINING":
		return StatusStopped.Render(status)
	case "PENDING", "PROVISIONING":
		return StatusPending.Render(status)
	default:
		return status
	}
}

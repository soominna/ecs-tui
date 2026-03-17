package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// ─── Catppuccin Palette (set by ApplyTheme in theme.go) ─────────────────────

var (
	colorCrust    lipgloss.Color
	colorMantle   lipgloss.Color
	colorBase     lipgloss.Color
	colorSurface0 lipgloss.Color
	colorSurface1 lipgloss.Color
	colorSurface2 lipgloss.Color
	colorOverlay0 lipgloss.Color
	colorOverlay1 lipgloss.Color
	colorSubtext0 lipgloss.Color
	colorSubtext1 lipgloss.Color
	colorText     lipgloss.Color
	colorBlue     lipgloss.Color
	colorLavender lipgloss.Color
	colorSapphire lipgloss.Color
	colorGreen    lipgloss.Color
	colorTeal     lipgloss.Color
	colorRed      lipgloss.Color
	colorMaroon   lipgloss.Color
	colorPeach    lipgloss.Color
	colorYellow   lipgloss.Color
	colorMauve    lipgloss.Color
	colorRosewater lipgloss.Color
)

// ─── Semantic aliases (set by ApplyTheme) ───────────────────────────────────

var (
	colorPrimary lipgloss.Color
	colorAccent  lipgloss.Color
	colorError   lipgloss.Color
	colorWarning lipgloss.Color
	colorMuted   lipgloss.Color
	colorWhite   lipgloss.Color
	colorDimText lipgloss.Color
)

// ─── Shared Styles (set by ApplyTheme) ──────────────────────────────────────

var (
	headerStyle       lipgloss.Style
	footerStyle       lipgloss.Style
	shortcutKeyStyle  lipgloss.Style
	shortcutDescStyle lipgloss.Style
	errorStyle        lipgloss.Style
	statusStyle       lipgloss.Style
	StatusRunning     lipgloss.Style
	StatusStopped     lipgloss.Style
	StatusPending     lipgloss.Style
	StatusActive      lipgloss.Style
	titleStyle        lipgloss.Style
	sectionTitleStyle lipgloss.Style
	breadcrumbStyle   lipgloss.Style
	breadcrumbActiveStyle lipgloss.Style
	breadcrumbSepStyle    lipgloss.Style
	loadingStyle      lipgloss.Style
)

// ─── Table Styles (reusable) ────────────────────────────────────────────────

// TableStyles returns consistent table header & selected row styles.
func TableStyles() (header, selected lipgloss.Style) {
	header = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colorSurface1).
		BorderBottom(true).
		Foreground(colorSubtext1).
		Bold(true).
		Padding(0, 1)
	selected = lipgloss.NewStyle().
		Foreground(colorText).
		Background(colorSurface0).
		Bold(false).
		Padding(0, 1)
	return
}

// ─── Responsive Columns ─────────────────────────────────────────────────────

// responsiveColumn defines a table column with flexible width.
type responsiveColumn struct {
	Title    string
	MinWidth int // minimum width
	Flex     int // flex weight (0 = fixed at MinWidth)
}

// calcColumnWidths distributes available width across columns.
func calcColumnWidths(cols []responsiveColumn, totalWidth int) []int {
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

// ─── Render Helpers ─────────────────────────────────────────────────────────

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

// RenderOverlay composites a centered modal box on top of the background content.
func RenderOverlay(bg string, box string, width, height int) string {
	bgLines := strings.Split(bg, "\n")
	boxLines := strings.Split(box, "\n")

	// Pad background to fill height
	for len(bgLines) < height {
		bgLines = append(bgLines, strings.Repeat(" ", width))
	}

	boxW := lipgloss.Width(box)
	boxH := len(boxLines)

	// Center position
	startX := (width - boxW) / 2
	startY := (height - boxH) / 2
	if startX < 0 {
		startX = 0
	}
	if startY < 0 {
		startY = 0
	}

	for i, boxLine := range boxLines {
		y := startY + i
		if y >= len(bgLines) {
			break
		}

		bgLine := bgLines[y]
		bgLineW := lipgloss.Width(bgLine)
		boxLineW := lipgloss.Width(boxLine)

		// Pad background line if shorter than needed
		if bgLineW < startX+boxLineW {
			bgLine += strings.Repeat(" ", startX+boxLineW-bgLineW)
		}

		// left part (up to startX) + overlay line + right part (after overlay)
		left := ansi.Truncate(bgLine, startX, "")
		right := ansi.TruncateLeft(bgLine, startX+boxLineW, "")

		bgLines[y] = left + boxLine + right
	}

	return strings.Join(bgLines[:height], "\n")
}

// OverlayBoxStyle returns a styled modal box.
func OverlayBoxStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBlue).
		Padding(1, 3).
		Width(44)
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

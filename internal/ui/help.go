package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Style vars set by ApplyTheme in theme.go.
var (
	helpTitleStyle lipgloss.Style
	helpKeyStyle   lipgloss.Style
	helpDescStyle  lipgloss.Style
	helpBoxStyle   lipgloss.Style
)

func renderHelpOverlay(current View, width, height int) string {
	var sb strings.Builder

	sb.WriteString(helpTitleStyle.Render("Keyboard Shortcuts"))
	sb.WriteString("\n\n")

	// View-specific shortcuts
	sectionLabel := lipgloss.NewStyle().Bold(true).Foreground(colorLavender).Background(colorBase)
	sb.WriteString(sectionLabel.Render(fmt.Sprintf("── %s ──", current.Title())))
	sb.WriteString("\n")
	for _, s := range current.ShortcutHelp() {
		sb.WriteString(helpKeyStyle.Render("<"+s.Key+">") + helpDescStyle.Render(s.Desc) + "\n")
	}

	sb.WriteString("\n")
	sb.WriteString(sectionLabel.Render("── Global ──"))
	sb.WriteString("\n")

	globalShortcuts := []Shortcut{
		{Key: "P", Desc: "Change AWS Profile/Region"},
		{Key: "?", Desc: "Toggle this help"},
		{Key: "Ctrl+C", Desc: "Quit"},
	}
	for _, s := range globalShortcuts {
		sb.WriteString(helpKeyStyle.Render("<"+s.Key+">") + helpDescStyle.Render(s.Desc) + "\n")
	}

	boxWidth := width - 4
	if boxWidth > 60 {
		boxWidth = 60
	}
	content := helpBoxStyle.Width(boxWidth).Render(sb.String())

	// Center the box
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
}

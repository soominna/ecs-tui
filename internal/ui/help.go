package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	helpTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			MarginBottom(1)

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true).
			Width(15)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#CCCCCC"))

	helpBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(1, 2)
)

func renderHelpOverlay(current View, width, height int) string {
	var sb strings.Builder

	sb.WriteString(helpTitleStyle.Render("Keyboard Shortcuts"))
	sb.WriteString("\n\n")

	// View-specific shortcuts
	sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(colorWhite).Render(fmt.Sprintf("── %s ──", current.Title())))
	sb.WriteString("\n")
	for _, s := range current.ShortcutHelp() {
		sb.WriteString(helpKeyStyle.Render("<"+s.Key+">") + helpDescStyle.Render(s.Desc) + "\n")
	}

	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(colorWhite).Render("── Global ──"))
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

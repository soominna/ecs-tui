package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ExecHintView shows exec error details with actionable fix instructions.
type ExecHintView struct {
	errMsg   string
	hint     string
	viewport viewport.Model
	width    int
	height   int
	ready    bool
}

func NewExecHintView(errMsg, hint string) *ExecHintView {
	return &ExecHintView{
		errMsg: errMsg,
		hint:   hint,
	}
}

func (v *ExecHintView) Title() string { return "Exec Error" }

func (v *ExecHintView) ShortcutHelp() []Shortcut {
	return []Shortcut{
		{Key: "j/k", Desc: "Scroll"},
		{Key: "Esc", Desc: "Back"},
	}
}

func (v *ExecHintView) Init() tea.Cmd { return nil }

func (v *ExecHintView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		if !v.ready {
			v.viewport = viewport.New(v.width, v.height)
			v.ready = true
		} else {
			v.viewport.Width = v.width
			v.viewport.Height = v.height
		}
		v.viewport.SetContent(v.renderContent())
		return v, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return v, func() tea.Msg { return PopViewMsg{} }
		}
	}

	if v.ready {
		var cmd tea.Cmd
		v.viewport, cmd = v.viewport.Update(msg)
		return v, cmd
	}
	return v, nil
}

func (v *ExecHintView) View() string {
	if !v.ready {
		return ""
	}
	return v.viewport.View()
}

func (v *ExecHintView) renderContent() string {
	errLabel := lipgloss.NewStyle().Bold(true).Foreground(colorError).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(lipgloss.Color("#444444")).
			Padding(0, 0, 0, 1).
			MarginBottom(1)
	hintLabel := lipgloss.NewStyle().Bold(true).Foreground(colorAccent).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(lipgloss.Color("#444444")).
			Padding(0, 0, 0, 1).
			MarginBottom(1)
	cmdStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#50FA7B"))
	dimStyle := lipgloss.NewStyle().Foreground(colorMuted)

	var sb strings.Builder

	// Error section
	sb.WriteString(errLabel.Render("Error"))
	sb.WriteString("\n")

	for _, line := range strings.Split(v.errMsg, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			sb.WriteString(fmt.Sprintf("  %s\n", dimStyle.Render(line)))
		}
	}

	if v.hint != "" {
		sb.WriteString("\n")
		sb.WriteString(hintLabel.Render("How to Fix"))
		sb.WriteString("\n")

		for _, line := range strings.Split(v.hint, "\n") {
			// Highlight command lines (starting with spaces + "aws " or "brew ")
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "aws ") || strings.HasPrefix(trimmed, "brew ") ||
				strings.HasPrefix(trimmed, "--") {
				sb.WriteString(fmt.Sprintf("  %s\n", cmdStyle.Render(line)))
			} else {
				sb.WriteString(fmt.Sprintf("  %s\n", line))
			}
		}
	}

	return sb.String()
}

package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	awsclient "github.com/soominna/ecs-tui/internal/aws"
)

type TaskDefDiffView struct {
	client       awsclient.ECSAPI
	oldARN       string
	newARN       string
	oldShort     string
	newShort     string
	diffs        []awsclient.DiffEntry
	viewport     viewport.Model
	width        int
	height       int
	ready        bool
	loaded       bool
}

type diffLoadedMsg struct {
	diffs []awsclient.DiffEntry
}

func NewTaskDefDiffView(client awsclient.ECSAPI, oldARN, newARN, oldShort, newShort string) *TaskDefDiffView {
	return &TaskDefDiffView{
		client:   client,
		oldARN:   oldARN,
		newARN:   newARN,
		oldShort: oldShort,
		newShort: newShort,
	}
}

func (v *TaskDefDiffView) Title() string {
	return fmt.Sprintf("Diff (%s → %s)", v.oldShort, v.newShort)
}

func (v *TaskDefDiffView) ShortcutHelp() []Shortcut {
	return []Shortcut{
		{Key: "j/k", Desc: "Scroll"},
		{Key: "Esc", Desc: "Back"},
	}
}

func (v *TaskDefDiffView) Init() tea.Cmd {
	client := v.client
	oldARN := v.oldARN
	newARN := v.newARN
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
		defer cancel()
		oldDetail, err := client.DescribeTaskDefinitionDetail(ctx, oldARN)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("fetching old task def: %w", err)}
		}
		newDetail, err := client.DescribeTaskDefinitionDetail(ctx, newARN)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("fetching new task def: %w", err)}
		}
		diffs := awsclient.DiffTaskDefinitions(oldDetail, newDetail)
		return diffLoadedMsg{diffs: diffs}
	}
}

func (v *TaskDefDiffView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

	case diffLoadedMsg:
		v.diffs = msg.diffs
		v.loaded = true
		if v.ready {
			v.viewport.SetContent(v.renderContent())
		}
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

func (v *TaskDefDiffView) View() string {
	if !v.ready {
		return loadingStyle.Render("  Loading diff...")
	}
	return v.viewport.View()
}

func (v *TaskDefDiffView) renderContent() string {
	if !v.loaded {
		return loadingStyle.Render("  Loading task definition diff...")
	}

	removedStyle := lipgloss.NewStyle().Foreground(colorRed)
	addedStyle := lipgloss.NewStyle().Foreground(colorGreen)
	changedStyle := lipgloss.NewStyle().Foreground(colorYellow)
	labelStyle := lipgloss.NewStyle().Foreground(colorBlue).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(colorSubtext0)

	var sb strings.Builder

	sb.WriteString(sectionTitleStyle.Render("Task Definition Changes"))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("  %s → %s\n\n",
		dimStyle.Render(v.oldShort),
		labelStyle.Render(v.newShort)))

	if len(v.diffs) == 0 {
		sb.WriteString("  No differences found.\n")
		return sb.String()
	}

	for _, d := range v.diffs {
		sb.WriteString(fmt.Sprintf("  %s\n", changedStyle.Render(d.Field+":")))
		switch d.Kind {
		case "changed":
			sb.WriteString(fmt.Sprintf("    %s\n", removedStyle.Render("- "+d.OldValue)))
			sb.WriteString(fmt.Sprintf("    %s\n", addedStyle.Render("+ "+d.NewValue)))
		case "added":
			sb.WriteString(fmt.Sprintf("    %s  %s\n",
				addedStyle.Render("+ "+d.NewValue),
				dimStyle.Render("[added]")))
		case "removed":
			sb.WriteString(fmt.Sprintf("    %s  %s\n",
				removedStyle.Render("- "+d.OldValue),
				dimStyle.Render("[removed]")))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
